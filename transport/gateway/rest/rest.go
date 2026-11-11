// Copyright 2022 The codesjoy Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package rest provides HTTP server for the framework.
package rest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/protobuf/proto"

	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
	"github.com/codesjoy/yggdrasil/v3/rpc/status"
	transportmeta "github.com/codesjoy/yggdrasil/v3/transport/internal/httpmeta"
	"github.com/codesjoy/yggdrasil/v3/transport/support/listenaddr"
	"github.com/codesjoy/yggdrasil/v3/transport/support/marshaler"
	"github.com/codesjoy/yggdrasil/v3/transport/support/peer"
)

// Config is the configuration for the server.
type Config struct {
	Host              string        `mapstructure:"host"`
	Port              int           `mapstructure:"port"`
	AdvertiseAddress  string        `mapstructure:"advertise_address"`
	ReadHeaderTimeout time.Duration `mapstructure:"read_header_timeout" default:"5s"`
	ReadTimeout       time.Duration `mapstructure:"read_timeout"        default:"15s"`
	WriteTimeout      time.Duration `mapstructure:"write_timeout"       default:"30s"`
	IdleTimeout       time.Duration `mapstructure:"idle_timeout"        default:"1m"`
	ShutdownTimeout   time.Duration `mapstructure:"shutdown_timeout"    default:"5s"`
	AcceptHeader      []string      `mapstructure:"accept_header"`
	OutHeader         []string      `mapstructure:"out_header"`
	OutTrailer        []string      `mapstructure:"out_trailer"`
	TrustedProxyCIDRs []string      `mapstructure:"trusted_proxy_cidrs"`
	Middleware        struct {
		RPC []string `mapstructure:"rpc"`
		Web []string `mapstructure:"web"`
		All []string `mapstructure:"all"`
	} `mapstructure:"middleware"`
	Marshaler struct {
		Support []string `mapstructure:"support"`
		Config  struct {
			JSONPB *marshaler.JSONPbConfig `mapstructure:"jsonpb"`
		} `mapstructure:"config"`
	} `mapstructure:"marshaler"`
}

type serverInfo struct {
	address          string
	advertiseAddress string
	attributes       map[string]string
}

func (s *serverInfo) GetAttributes() map[string]string {
	return cloneStringMap(s.attributes)
}

func (s *serverInfo) GetAddress() string {
	return s.address
}

func (s *serverInfo) GetAdvertiseAddress() string {
	return s.advertiseAddress
}

// ServeMux is a request multiplexer for RPC-gateway.
// It matches http requests to patterns and invokes the corresponding handler.
type ServeMux struct {
	chi.Router
	rpcRouter chi.Router
	webRouter chi.Router
	svr       *http.Server
	mu        sync.Mutex
	listener  net.Listener
	stopped   bool
	started   bool

	cfg *Config

	info *serverInfo

	acceptHeaders  []string
	outHeaders     []string
	outTrailers    []string
	trustedProxies []netip.Prefix

	marshalerRegistry marshaler.Registry
	middlewareMap     map[string]Provider
	strictMiddleware  bool
}

// Option is the option for the server.
type Option func(*ServeMux)

// WithMarshalerRegistry sets the marshaler registry.
func WithMarshalerRegistry(registry marshaler.Registry) Option {
	return func(s *ServeMux) {
		s.marshalerRegistry = registry
	}
}

// WithMiddlewareProviders sets the provider map used to build REST middleware chains.
func WithMiddlewareProviders(providers map[string]Provider) Option {
	return func(s *ServeMux) {
		s.middlewareMap = providers
	}
}

// WithStrictMiddlewareProviders makes missing REST middleware providers a
// server construction error.
func WithStrictMiddlewareProviders() Option {
	return func(s *ServeMux) {
		s.strictMiddleware = true
	}
}

// NewServer creates a new ServeMux from explicit config.
func NewServer(cfg *Config, opts ...Option) (Server, error) {
	if cfg == nil {
		cfg = &Config{}
	}

	host := listenaddr.ResolveBindHost(cfg.Host)
	address := net.JoinHostPort(host, strconv.Itoa(cfg.Port))
	trustedProxies, err := parseTrustedProxyCIDRs(cfg.TrustedProxyCIDRs)
	if err != nil {
		return nil, err
	}

	s := &ServeMux{
		cfg: cfg,
		info: &serverInfo{
			address:    address,
			attributes: map[string]string{},
		},
		acceptHeaders:  cfg.AcceptHeader,
		outHeaders:     cfg.OutHeader,
		outTrailers:    cfg.OutTrailer,
		trustedProxies: trustedProxies,
	}

	for _, opt := range opts {
		opt(s)
	}

	router, rpcRouter, webRouter, err := s.buildRouters()
	if err != nil {
		return nil, err
	}
	s.Router = router
	s.rpcRouter = rpcRouter
	s.webRouter = webRouter
	return s, nil
}

func (s *ServeMux) buildRouters() (chi.Router, chi.Router, chi.Router, error) {
	r := chi.NewMux()
	allMiddlewares := dedupStableStrings(s.cfg.Middleware.All)
	allChain, err := buildMiddlewareChain(
		s.middlewareMap,
		s.strictMiddleware,
		allMiddlewares...,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	r.Use(allChain...)

	rpcMiddlewaresRaw := s.cfg.Middleware.RPC
	if s.marshalerRegistry == nil {
		rpcMiddlewaresRaw = append([]string{"marshaler"}, rpcMiddlewaresRaw...)
	}
	rpcMiddlewares := dedupStableStrings(rpcMiddlewaresRaw)
	rpcChain, err := buildMiddlewareChain(
		s.middlewareMap,
		s.strictMiddleware,
		rpcMiddlewares...,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	rpcRouter := r.Group(func(r chi.Router) {
		if s.marshalerRegistry != nil {
			r.Use(NewMarshalerMiddleware(s.marshalerRegistry))
		}
		r.Use(rpcChain...)
	})

	webMiddlewares := dedupStableStrings(s.cfg.Middleware.Web)
	webChain, err := buildMiddlewareChain(
		s.middlewareMap,
		s.strictMiddleware,
		webMiddlewares...,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	webRouter := r.Group(func(r chi.Router) {
		r.Use(webChain...)
	})

	return r, rpcRouter, webRouter, nil
}

func buildMiddlewareChain(
	providerMap map[string]Provider,
	strict bool,
	names ...string,
) (chi.Middlewares, error) {
	if providerMap == nil {
		if strict {
			return BuildE(names...)
		}
		return Build(names...), nil
	}
	if strict {
		return BuildWithProvidersE(providerMap, names...)
	}
	return BuildWithProviders(providerMap, names...), nil
}

// RPCHandle registers a new RPC handler.
func (s *ServeMux) RPCHandle(meth, path string, f HandlerFunc) {
	s.rpcRouter.MethodFunc(meth, path, func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctx = metadata.WithStreamContext(ctx)
		ctx = metadata.WithInContext(ctx, s.extractInMetadata(r))
		ctx = peer.WithContext(ctx, s.getPeer(r))
		r = r.WithContext(ctx)
		res, err := f(w, r)
		if err != nil {
			s.errorHandler(w, r, err)
			return
		}
		resp, ok := res.(proto.Message)
		if !ok || isNilProtoMessage(resp) {
			s.errorHandler(
				w,
				r,
				status.New(code.Code_INTERNAL, "RPC handler returned an invalid response").Err(),
			)
			return
		}
		s.successHandler(w, r, resp)
	})
}

func isNilProtoMessage(message proto.Message) bool {
	if message == nil {
		return true
	}
	value := reflect.ValueOf(message)
	return value.Kind() == reflect.Pointer && value.IsNil()
}

// RawHandle registers a new raw handler.
func (s *ServeMux) RawHandle(meth, path string, h http.HandlerFunc) {
	s.webRouter.MethodFunc(meth, path, h)
}

// Start starts the server.
func (s *ServeMux) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return errors.New("server had already stopped")
	}
	if s.started {
		return errors.New("server had already serve")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	lis, err := (&net.ListenConfig{}).Listen(ctx, "tcp", s.info.address)
	if err != nil {
		s.started = false
		s.listener = nil
		s.svr = nil
		return err
	}
	boundAddress := lis.Addr().String()
	advertiseAddress, err := listenaddr.ResolveAdvertiseAddress(
		"tcp",
		boundAddress,
		s.cfg.AdvertiseAddress,
	)
	if err != nil {
		_ = lis.Close()
		s.started = false
		s.listener = nil
		return err
	}
	s.info.address = boundAddress
	s.info.advertiseAddress = advertiseAddress
	s.listener = lis
	s.svr = &http.Server{
		Handler:           s,
		ReadHeaderTimeout: s.cfg.ReadHeaderTimeout,
		ReadTimeout:       s.cfg.ReadTimeout,
		WriteTimeout:      s.cfg.WriteTimeout,
		IdleTimeout:       s.cfg.IdleTimeout,
	}
	s.started = true
	return nil
}

func dedupStableStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	seen := make(map[string]struct{}, len(values))
	i := 0
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		values[i] = value
		i++
	}
	return values[:i]
}

// Serve serves the server.
func (s *ServeMux) Serve() error {
	if !s.started || s.svr == nil {
		return errors.New("server is not initialized")
	}
	return s.svr.Serve(s.listener)
}

// Stop stops the server.
func (s *ServeMux) Stop(ctx context.Context) error {
	s.mu.Lock()
	svr := s.svr
	timeout := time.Duration(0)
	if s.cfg != nil {
		timeout = s.cfg.ShutdownTimeout
	}
	s.stopped = true
	s.mu.Unlock()
	if svr == nil {
		return nil
	}
	if ctx == nil {
		if timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(context.Background(), timeout)
			defer cancel()
		} else {
			ctx = context.Background()
		}
	}
	return svr.Shutdown(ctx)
}

// Info returns the server info.
func (s *ServeMux) Info() ServerInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.info == nil {
		return &serverInfo{attributes: map[string]string{}}
	}
	return &serverInfo{
		address:          s.info.address,
		advertiseAddress: s.info.advertiseAddress,
		attributes:       cloneStringMap(s.info.attributes),
	}
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func (s *ServeMux) extractInMetadata(r *http.Request) metadata.MD {
	md := transportmeta.ExtractMetadata(r.Header, MetadataHeaderPrefix)
	for _, item := range s.acceptHeaders {
		vals := r.Header.Values(item)
		if vals == nil {
			continue
		}
		md.Append(item, vals...)
	}
	return md
}

func (s *ServeMux) getPeer(r *http.Request) *peer.Peer {
	remoteIP, port := transportmeta.ParseRemoteAddr(r.RemoteAddr)
	clientIP := remoteIP
	if s.isTrustedProxy(remoteIP) {
		if forwarded, ok := s.forwardedClientIP(r, remoteIP); ok {
			clientIP = forwarded
			port = 0
		}
	}
	var localAddr net.Addr
	s.mu.Lock()
	if s.listener != nil {
		localAddr = s.listener.Addr()
	}
	s.mu.Unlock()
	var tcpIP net.IP
	if clientIP.IsValid() {
		tcpIP = net.IP(clientIP.Unmap().AsSlice())
	}
	return &peer.Peer{
		Addr: &net.TCPAddr{
			IP:   tcpIP,
			Port: port,
		},
		LocalAddr: localAddr,
		Protocol:  "http",
	}
}

func parseTrustedProxyCIDRs(values []string) ([]netip.Prefix, error) {
	out := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if strings.Contains(value, "/") {
			prefix, err := netip.ParsePrefix(value)
			if err != nil {
				return nil, fmt.Errorf("invalid trusted proxy CIDR %q: %w", value, err)
			}
			out = append(out, prefix.Masked())
			continue
		}
		address, err := netip.ParseAddr(value)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy address %q: %w", value, err)
		}
		address = address.Unmap()
		bits := 128
		if address.Is4() {
			bits = 32
		}
		out = append(out, netip.PrefixFrom(address, bits))
	}
	return out, nil
}

func (s *ServeMux) isTrustedProxy(address netip.Addr) bool {
	if !address.IsValid() || len(s.trustedProxies) == 0 {
		return false
	}
	address = address.Unmap()
	for _, prefix := range s.trustedProxies {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func (s *ServeMux) forwardedClientIP(
	request *http.Request,
	remoteIP netip.Addr,
) (netip.Addr, bool) {
	if values := request.Header.Values("X-Forwarded-For"); len(values) > 0 {
		chain, ok := parseForwardedChain(values)
		if !ok {
			return netip.Addr{}, false
		}
		chain = append(chain, remoteIP)
		for index := range slices.Backward(chain) {
			if !s.isTrustedProxy(chain[index]) {
				return chain[index], true
			}
		}
		if len(chain) > 0 {
			return chain[0], true
		}
		return netip.Addr{}, false
	}
	value := strings.TrimSpace(request.Header.Get("X-Real-Ip"))
	if value == "" {
		return netip.Addr{}, false
	}
	address, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Addr{}, false
	}
	return address.Unmap(), true
}

func parseForwardedChain(values []string) ([]netip.Addr, bool) {
	out := make([]netip.Addr, 0, len(values))
	for _, value := range values {
		for item := range strings.SplitSeq(value, ",") {
			item = strings.TrimSpace(item)
			if item == "" {
				return nil, false
			}
			address, err := netip.ParseAddr(strings.Trim(item, "[]"))
			if err != nil {
				return nil, false
			}
			out = append(out, address.Unmap())
		}
	}
	return out, len(out) > 0
}

func (s *ServeMux) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	ctx := r.Context()
	outbound := marshaler.OutboundFromContext(ctx)

	// return Internal when Marshal failed
	const fallback = `{"code": 13, "message": "failed to marshal error message"}`

	st := status.FromError(err)
	pb := st.Status()

	w.Header().Del("Trailer")
	w.Header().Del("Transfer-Encoding")

	contentType := outbound.ContentType(pb)
	w.Header().Set("Content-Type", contentType)

	if st.IsCode(code.Code_UNAUTHENTICATED) {
		w.Header().Set("WWW-Authenticate", st.Message())
	}

	buf, mErr := outbound.Marshal(pb)
	if mErr != nil {
		slog.Error("failed to marshal error message",
			slog.String("status", fmt.Sprintf("%q", st)),
			slog.Any("error", mErr))
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := io.WriteString(w, fallback); err != nil {
			slog.Error("failed to write response", slog.Any("error", err))
		}
		return
	}

	s.writeResponse(w, r, buf, int(st.HTTPCode()))
}

func (s *ServeMux) successHandler(w http.ResponseWriter, r *http.Request, resp proto.Message) {
	ctx := r.Context()

	outbound := marshaler.OutboundFromContext(ctx)
	contentType := outbound.ContentType(resp)
	w.Header().Set("Content-Type", contentType)

	buf, err := outbound.Marshal(resp)
	if err != nil {
		slog.Info("fault to marshal resp", slog.Any("error", err))
		s.errorHandler(w, r, err)
		return
	}

	s.writeResponse(w, r, buf, 0)
}

func (s *ServeMux) writeResponse(
	w http.ResponseWriter,
	r *http.Request,
	body []byte,
	statusCode int,
) {
	ctx := r.Context()
	header, _ := metadata.FromHeaderCtx(ctx)
	trailerHeader, _ := metadata.FromTrailerCtx(ctx)
	transportmeta.WriteMetadata(w, header, s.outgoingHeaderMatcher)

	doForwardTrailers := transportmeta.AcceptsTrailers(r)
	forwardTrailers := doForwardTrailers && trailerHeader.Len() > 0
	if forwardTrailers {
		transportmeta.DeclareTrailers(w, trailerHeader, s.outgoingTrailerMatcher)
		w.Header().Set("Transfer-Encoding", "chunked")
	}

	if statusCode > 0 {
		w.WriteHeader(statusCode)
	}
	if _, err := w.Write(body); err != nil {
		slog.Error("failed to write response", slog.Any("error", err))
	}
	if forwardTrailers {
		transportmeta.WriteTrailers(w, trailerHeader, s.outgoingTrailerMatcher)
	}
}

func (s *ServeMux) outgoingHeaderMatcher(key string) string {
	if slices.Contains(s.outHeaders, key) {
		return key
	}
	return MetadataHeaderPrefix + key
}

func (s *ServeMux) outgoingTrailerMatcher(key string) string {
	if slices.Contains(s.outTrailers, key) {
		return key
	}
	return MetadataTrailerPrefix + key
}
