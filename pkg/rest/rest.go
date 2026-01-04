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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/codesjoy/yggdrasil/pkg/config"
	"github.com/codesjoy/yggdrasil/pkg/metadata"
	"github.com/codesjoy/yggdrasil/pkg/remote/peer"
	"github.com/codesjoy/yggdrasil/pkg/rest/marshaler"
	"github.com/codesjoy/yggdrasil/pkg/rest/middleware"
	"github.com/codesjoy/yggdrasil/pkg/status"
	"github.com/codesjoy/yggdrasil/pkg/utils/xarray"
	"github.com/codesjoy/yggdrasil/pkg/utils/xnet"
	"github.com/go-chi/chi/v5"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/protobuf/proto"
)

// Config is the configuration for the server.
type Config struct {
	Host              string        `mapstruct:"host"`
	Port              int           `mapstruct:"port"`
	ReadHeaderTimeout time.Duration `mapstruct:"read_header_timeout" default:"5s"`
	ReadTimeout       time.Duration `mapstruct:"read_timeout"        default:"15s"`
	WriteTimeout      time.Duration `mapstruct:"write_timeout"       default:"30s"`
	IdleTimeout       time.Duration `mapstruct:"idle_timeout"        default:"1m"`
	ShutdownTimeout   time.Duration `mapstruct:"shutdown_timeout"    default:"5s"`
	AcceptHeader      []string      `mapstruct:"accept_header"`
	OutHeader         []string      `mapstruct:"out_header"`
	OutTrailer        []string      `mapstruct:"out_trailer"`
	Middleware        struct {
		RPC []string `mapstruct:"rpc"`
		Web []string `mapstruct:"web"`
		All []string `mapstruct:"all"`
	} `mapstruct:"middleware"`
}

type serverInfo struct {
	address    string
	attributes map[string]string
}

func (s *serverInfo) GetAttributes() map[string]string {
	return s.attributes
}

func (s *serverInfo) GetAddress() string {
	return s.address
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

	acceptHeaders []string
	outHeaders    []string
	outTrailers   []string
}

// NewServer creates a new ServeMux.
func NewServer() (Server, error) {
	cfg := &Config{}
	if err := config.Get(config.Join(config.KeyBase, "rest")).Scan(cfg); err != nil {
		return nil, err
	}

	ip, _ := xnet.Extract(cfg.Host)
	address := fmt.Sprintf("%s:%d", ip, cfg.Port)

	r := chi.NewMux()
	r.Use(middleware.GetMiddlewares(xarray.DelDupStable(cfg.Middleware.All)...)...)
	rpcMiddlewares := xarray.DelDupStable(append([]string{"marshaler"}, cfg.Middleware.RPC...))
	webMiddlewares := xarray.DelDupStable(cfg.Middleware.Web)
	rpcRouter := r.Group(func(r chi.Router) {
		r.Use(middleware.GetMiddlewares(rpcMiddlewares...)...)
	})
	webRouter := r.Group(func(r chi.Router) {
		r.Use(middleware.GetMiddlewares(webMiddlewares...)...)
	})
	s := &ServeMux{
		cfg:       cfg,
		Router:    r,
		rpcRouter: rpcRouter,
		webRouter: webRouter,
		info: &serverInfo{
			address:    address,
			attributes: map[string]string{},
		},
		acceptHeaders: cfg.AcceptHeader,
		outHeaders:    cfg.OutHeader,
		outTrailers:   cfg.OutTrailer,
	}
	return s, nil
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
		s.successHandler(w, r, res.(proto.Message))
	})
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
	s.started = true
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	lis, err := (&net.ListenConfig{}).Listen(ctx, "tcp", s.info.address)
	if err != nil {
		return err
	}
	s.info.address = lis.Addr().String()
	s.listener = lis
	s.svr = &http.Server{
		Handler:           s,
		ReadHeaderTimeout: s.cfg.ReadHeaderTimeout,
		ReadTimeout:       s.cfg.ReadTimeout,
		WriteTimeout:      s.cfg.WriteTimeout,
		IdleTimeout:       s.cfg.IdleTimeout,
	}
	return nil
}

// Serve serves the server.
func (s *ServeMux) Serve() error {
	if !s.started || s.svr == nil {
		return errors.New("server is not initialized")
	}
	return s.svr.Serve(s.listener)
}

// Stop stops the server.
func (s *ServeMux) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
	defer cancel()
	return s.svr.Shutdown(ctx)
}

// Info returns the server info.
func (s *ServeMux) Info() ServerInfo {
	return s.info
}

func (s *ServeMux) extractInMetadata(r *http.Request) metadata.MD {
	md := metadata.New(nil)
	for _, item := range s.acceptHeaders {
		vals := r.Header.Values(item)
		if vals == nil {
			continue
		}
		md.Append(item, vals...)
	}

	for key, vals := range r.Header {
		if strings.HasPrefix(key, MetadataHeaderPrefix) {
			md.Append(key[len(MetadataHeaderPrefix):], vals...)
		}
	}
	return md
}

func (s *ServeMux) getPeer(r *http.Request) *peer.Peer {
	ip, portStr, _ := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	port, _ := strconv.Atoi(portStr)
	clientIP := r.Header.Get("X-Forwarded-For")
	clientIP = strings.TrimSpace(strings.Split(clientIP, ",")[0])
	if clientIP == "" {
		clientIP = strings.TrimSpace(r.Header.Get("X-Real-Ip"))
	}
	if clientIP != "" {
		ip = clientIP
	}
	var localAddr net.Addr
	if s.listener != nil {
		localAddr = s.listener.Addr()
	}
	return &peer.Peer{
		Addr: &net.TCPAddr{
			IP:   net.ParseIP(ip),
			Port: port,
		},
		LocalAddr: localAddr,
		Protocol:  "http",
	}
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

	header, _ := metadata.FromHeaderCtx(ctx)
	trailerHeader, _ := metadata.FromTrailerCtx(ctx)

	s.handleResponseHeader(w, header)

	doForwardTrailers := s.requestAcceptsTrailers(r)

	if doForwardTrailers && trailerHeader.Len() > 0 {
		s.handleForwardResponseTrailerHeader(w, trailerHeader)
		w.Header().Set("Transfer-Encoding", "chunked")
	}

	w.WriteHeader(int(st.HTTPCode()))
	if _, err := w.Write(buf); err != nil {
		slog.Error("failed to write response", slog.Any("error", err))
	}

	if doForwardTrailers && trailerHeader.Len() > 0 {
		s.handleForwardResponseTrailer(w, trailerHeader)
	}
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

	header, _ := metadata.FromHeaderCtx(ctx)
	trailerHeader, _ := metadata.FromTrailerCtx(ctx)

	s.handleResponseHeader(w, header)

	doForwardTrailers := s.requestAcceptsTrailers(r)

	if doForwardTrailers && trailerHeader.Len() > 0 {
		s.handleForwardResponseTrailerHeader(w, trailerHeader)
		w.Header().Set("Transfer-Encoding", "chunked")
	}

	if _, err = w.Write(buf); err != nil {
		slog.Error("failed to write response", slog.Any("error", err))
	}

	if doForwardTrailers && trailerHeader.Len() > 0 {
		s.handleForwardResponseTrailer(w, trailerHeader)
	}
}

func (s *ServeMux) handleResponseHeader(w http.ResponseWriter, md metadata.MD) {
	for k, vs := range md {
		if h, ok := s.outgoingHeaderMatcher(k); ok {
			for _, v := range vs {
				w.Header().Add(h, v)
			}
		}
	}
}

func (s *ServeMux) outgoingHeaderMatcher(key string) (string, bool) {
	for _, item := range s.outHeaders {
		if item == key {
			return key, true
		}
	}
	return fmt.Sprintf("%s%s", MetadataHeaderPrefix, key), true
}

func (s *ServeMux) requestAcceptsTrailers(req *http.Request) bool {
	te := req.Header.Get("TE")
	return strings.Contains(strings.ToLower(te), "trailers")
}

func (s *ServeMux) handleForwardResponseTrailerHeader(w http.ResponseWriter, md metadata.MD) {
	for k := range md {
		if h, ok := s.outgoingTrailerMatcher(k); ok {
			w.Header().Add("Trailer", h)
		}
	}
}

func (s *ServeMux) handleForwardResponseTrailer(w http.ResponseWriter, md metadata.MD) {
	for k, vs := range md {
		if h, ok := s.outgoingTrailerMatcher(k); ok {
			for _, v := range vs {
				w.Header().Add(h, v)
			}
		}
	}
}

func (s *ServeMux) outgoingTrailerMatcher(key string) (string, bool) {
	for _, item := range s.outTrailers {
		if item == key {
			return key, true
		}
	}
	return fmt.Sprintf("%s%s", MetadataTrailerPrefix, key), true
}
