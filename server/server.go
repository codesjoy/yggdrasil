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

// Package server provides the  server implementation for the framework.
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/governor"
	"github.com/codesjoy/yggdrasil/v2/interceptor"
	"github.com/codesjoy/yggdrasil/v2/internal/constant"
	"github.com/codesjoy/yggdrasil/v2/internal/instance"
	"github.com/codesjoy/yggdrasil/v2/metadata"
	"github.com/codesjoy/yggdrasil/v2/remote"
	"github.com/codesjoy/yggdrasil/v2/rest"
	"github.com/codesjoy/yggdrasil/v2/stats"
	"github.com/codesjoy/yggdrasil/v2/status"
	"github.com/codesjoy/yggdrasil/v2/stream"
	"github.com/codesjoy/yggdrasil/v2/utils/xarray"
	"google.golang.org/genproto/googleapis/rpc/code"
)

const (
	serverStateInit = iota
	serverStateRunning
	serverStateClosing
)

var svr *server

type serverInfo struct {
	scheme   string
	address  string
	svrKind  constant.ServerKind
	metadata map[string]string
}

func (si *serverInfo) Address() string {
	return si.address
}

func (si *serverInfo) Metadata() map[string]string {
	return si.metadata
}

func (si *serverInfo) Kind() constant.ServerKind {
	return si.svrKind
}

func (si *serverInfo) Scheme() string {
	return si.scheme
}

type server struct {
	mu                sync.RWMutex
	services          map[string]*ServiceInfo // service name -> service serverInfo
	servicesDesc      map[string][]methodInfo
	restRouterDesc    []restRouterInfo
	unaryInterceptor  interceptor.UnaryServerInterceptor
	streamInterceptor interceptor.StreamServerInterceptor
	servers           []remote.Server
	state             int
	serverWG          sync.WaitGroup
	stats             stats.Handler

	restSvr    rest.Server
	restEnable bool
}

// NewServer creates a new server.
func NewServer() (Server, error) {
	svr = &server{
		services:       map[string]*ServiceInfo{},
		servicesDesc:   map[string][]methodInfo{},
		restRouterDesc: []restRouterInfo{},
		stats:          stats.GetServerHandler(),
	}
	if config.GetBool(config.Join(config.KeyBase, "rest", "enable"), false) {
		svr.restEnable = true
		var err error
		svr.restSvr, err = rest.NewServer()
		if err != nil {
			return nil, err
		}
	}
	svr.initInterceptor()
	if err := svr.initRemoteServer(); err != nil {
		return nil, err
	}
	governor.HandleFunc("/services", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		encoder := json.NewEncoder(w)
		if r.URL.Query().Get("pretty") == "true" {
			encoder.SetIndent("", "    ")
		}
		result := map[string]interface{}{
			"appName":  instance.Name(),
			"services": svr.servicesDesc,
		}
		_ = encoder.Encode(result)
	})
	governor.HandleFunc("/rest", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		encoder := json.NewEncoder(w)
		if r.URL.Query().Get("pretty") == "true" {
			encoder.SetIndent("", "    ")
		}
		result := map[string]interface{}{
			"appName": instance.Name(),
			"routers": svr.restRouterDesc,
		}
		_ = encoder.Encode(result)
	})

	return svr, nil
}

// RegisterService registers a service and its implementation to the gRPC
// server. It is called from the IDL generated code. This must be called before
// invoking Serve. If ss is non-nil (for legacy code), its type is checked to
// ensure it implements sd.HandlerType.
func (s *server) RegisterService(sd *ServiceDesc, ss interface{}) {
	if ss == nil {
		slog.Error("fault to register service: Server.RegisterService handler is nil")
		os.Exit(1)
	}
	ht := reflect.TypeOf(sd.HandlerType).Elem()
	st := reflect.TypeOf(ss)
	if !st.Implements(ht) {
		slog.Error(
			"fault to register service: Server.RegisterService found the handler does not satisfy the interface",
			slog.Any("handlerType", st),
			slog.Any("interfaceType", ht),
		)
		os.Exit(1)
	}
	s.register(sd, ss)
}

func (s *server) RegisterRestService(sd *RestServiceDesc, ss interface{}, prefix ...string) {
	if !s.restEnable {
		return
	}
	if ss == nil {
		slog.Error("fault to register rest service: Server.RegisterService handler is nil")
		os.Exit(1)
	}
	ht := reflect.TypeOf(sd.HandlerType).Elem()
	st := reflect.TypeOf(ss)
	if !st.Implements(ht) {
		slog.Error(
			"fault to register rest service: Server.RegisterService found the handler does not satisfy the interface",
			slog.Any("handlerType", st),
			slog.Any("interfaceType", ht),
		)
		os.Exit(1)
	}
	s.registerRest(sd, ss, prefix...)
}

func (s *server) RegisterRestRawHandlers(sd ...*RestRawHandlerDesc) {
	if !s.restEnable {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range sd {
		s.restRouterDesc = append(s.restRouterDesc, restRouterInfo{
			Method: item.Method,
			Path:   item.Path,
		})
		s.restSvr.RawHandle(item.Method, item.Path, item.Handler)
	}
}

func (s *server) Stop() error {
	s.mu.Lock()
	if s.state == serverStateInit {
		s.state = serverStateClosing
		s.mu.Unlock()
		return nil
	}
	if s.state == serverStateClosing {
		s.mu.Unlock()
		return nil
	}
	s.state = serverStateClosing
	s.mu.Unlock()

	var (
		errs error
		mu   sync.Mutex
		wg   sync.WaitGroup
	)

	for _, item := range s.servers {
		wg.Add(1)
		go func(srv remote.Server) {
			defer wg.Done()
			if err := srv.Stop(); err != nil {
				mu.Lock()
				errs = errors.Join(errs, err)
				mu.Unlock()
				slog.Error("fault to stop server",
					slog.String("protocol", srv.Info().Protocol),
					slog.Any("error", err))
			}
		}(item)
	}

	if s.restEnable {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.restSvr.Stop(); err != nil {
				mu.Lock()
				errs = errors.Join(errs, err)
				mu.Unlock()
				slog.Error("fault to stop rest server",
					slog.Any("error", err))
			}
		}()
	}

	wg.Wait()
	return errs
}

func (s *server) Serve(startFlag chan<- struct{}) error {
	s.mu.Lock()
	if s.state == serverStateClosing {
		s.mu.Unlock()
		return errors.New("server stopped")
	}
	if s.state == serverStateRunning {
		s.mu.Unlock()
		return errors.New("server already serve")
	}
	s.state = serverStateRunning
	s.mu.Unlock()

	var err error
	defer func() {
		if err != nil {
			_ = s.Stop()
			close(startFlag)
		}
	}()

	for _, svr := range s.servers {
		if err = s.serve(svr); err != nil {
			return err
		}
	}

	if err = s.restServe(); err != nil {
		return err
	}

	startFlag <- struct{}{}
	s.serverWG.Wait()
	return nil
}

func (s *server) Endpoints() []Endpoint {
	endpoints := make([]Endpoint, len(s.servers))
	for i, item := range s.servers {
		e := item.Info()
		endpoints[i] = &serverInfo{
			scheme:   e.Protocol,
			address:  e.Address,
			metadata: e.Attributes,
			svrKind:  constant.ServerKindRPC,
		}
	}
	if s.restEnable {
		endpoints = append(endpoints, &serverInfo{
			scheme:   "http",
			address:  s.restSvr.Info().GetAddress(),
			metadata: s.restSvr.Info().GetAttributes(),
			svrKind:  constant.ServerKindRest,
		})
	}
	return endpoints
}

func (s *server) initInterceptor() {
	unaryNames := config.Get(config.Join(config.KeyBase, "interceptor", "unary_server")).
		StringSlice()
	if len(unaryNames) != 0 {
		s.unaryInterceptor = interceptor.ChainUnaryServerInterceptors(
			xarray.DelDupStable(unaryNames),
		)
	}
	streamNames := config.Get(config.Join(config.KeyBase, "interceptor", "stream_server")).
		StringSlice()
	if len(streamNames) != 0 {
		s.streamInterceptor = interceptor.ChainStreamServerInterceptors(
			xarray.DelDupStable(streamNames),
		)
	}
}

func (s *server) initRemoteServer() error {
	protocols := config.Get(config.Join(config.KeyBase, "server", "protocol")).StringSlice()
	if len(protocols) == 0 {
		return nil
	}
	for _, protocol := range protocols {
		builder := remote.GetServerBuilder(protocol)
		if builder == nil {
			return fmt.Errorf("server builder for protocol %s not found", protocol)
		}
		svr, err := builder(s.handleStream)
		if err != nil {
			return fmt.Errorf("fault to new %s remote server: %v", protocol, err)
		}
		s.servers = append(s.servers, svr)
	}
	return nil
}

func (s *server) register(sd *ServiceDesc, ss interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.services[sd.ServiceName]; ok {
		slog.Error(
			fmt.Sprintf(
				"fault to register service: Server.RegisterService found duplicate service registration for %q",
				sd.ServiceName,
			),
		)
		os.Exit(1)
	}
	s.registerServiceDesc(sd)
	s.registerServiceInfo(sd, ss)
}

func (s *server) registerServiceInfo(sd *ServiceDesc, ss interface{}) {
	info := &ServiceInfo{
		ServiceImpl: ss,
		Methods:     make(map[string]*MethodDesc),
		Streams:     make(map[string]*stream.Desc),
		Metadata:    sd.Metadata,
	}
	for i := range sd.Methods {
		d := &sd.Methods[i]
		info.Methods[d.MethodName] = d
	}
	for i := range sd.Streams {
		d := &sd.Streams[i]
		info.Streams[d.StreamName] = d
	}
	s.services[sd.ServiceName] = info
}

func (s *server) registerServiceDesc(desc *ServiceDesc) {
	methods := make([]methodInfo, 0, len(desc.Methods)+len(desc.Streams))
	for _, item := range desc.Methods {
		methods = append(methods, methodInfo{
			MethodName: item.MethodName,
		})
	}
	for _, item := range desc.Streams {
		methods = append(methods, methodInfo{
			MethodName:    item.StreamName,
			ServerStreams: item.ServerStreams,
			ClientStreams: item.ClientStreams,
		})
	}
	s.servicesDesc[desc.ServiceName] = methods
}

func (s *server) registerRest(sd *RestServiceDesc, ss interface{}, prefix ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var pathPrefix string
	if len(prefix) != 0 {
		pathPrefix = "/" + strings.TrimPrefix(prefix[0], "/")
	}

	for _, item := range sd.Methods {
		method := item.Method
		path := pathPrefix + item.Path
		handler := item.Handler
		s.restRouterDesc = append(s.restRouterDesc, restRouterInfo{
			Method: method,
			Path:   path,
		})
		s.restSvr.RPCHandle(
			method,
			path,
			func(w http.ResponseWriter, r *http.Request) (interface{}, error) {
				return handler(w, r, ss, s.unaryInterceptor)
			},
		)
	}
}

func (s *server) serve(svr remote.Server) error {
	err := svr.Start()
	if err != nil {
		slog.Error(
			"fault to start server",
			slog.String("protocol", svr.Info().Protocol),
			slog.Any("error", err),
		)
		return err
	}
	slog.Info(
		"server started",
		slog.String("protocol", svr.Info().Protocol),
		slog.String("endpoint", svr.Info().Address),
	)
	s.serverWG.Add(1)
	go func() {
		defer s.serverWG.Done()
		if err = svr.Handle(); err != nil {
			slog.Error(
				"the server exits abnormally",
				slog.String("protocol", svr.Info().Protocol),
				slog.Any("error", err),
			)
		}
	}()
	return nil
}

func (s *server) restServe() error {
	if !s.restEnable {
		return nil
	}
	err := s.restSvr.Start()
	if err != nil {
		slog.Error("fault to start rest server", slog.Any("error", err))
		return err
	}
	slog.Info("rest server started", slog.String("endpoint", s.restSvr.Info().GetAddress()))
	s.serverWG.Add(1)
	go func() {
		defer s.serverWG.Done()
		if err = s.restSvr.Serve(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("fault to serve rest server", slog.Any("error", err))
		}
	}()
	return nil
}

func (s *server) handleStream(ss remote.ServerStream) {
	sm := ss.Method()
	if sm != "" && sm[0] == '/' {
		sm = sm[1:]
	}
	pos := strings.LastIndex(sm, "/")
	if pos == -1 {
		ss.Finish(
			nil,
			status.New(code.Code_UNIMPLEMENTED, fmt.Sprintf("malformed method name: %q", sm)),
		)
		return
	}
	service := sm[:pos]
	method := sm[pos+1:]

	srv, knownService := s.services[service]
	if knownService {
		if md, ok := srv.Methods[method]; ok {
			s.processUnaryRPC(md, srv, ss)
			return
		}
		if sd, ok := srv.Streams[method]; ok {
			s.processStreamRPC(sd, srv, ss)
			return
		}
	}
	var errDesc string
	if !knownService {
		errDesc = fmt.Sprintf("unknown service %v", service)
	} else {
		errDesc = fmt.Sprintf("unknown method %v for service %v", method, service)
	}
	ss.Finish(nil, status.New(code.Code_UNIMPLEMENTED, errDesc))
}

func (s *server) processUnaryRPC(desc *MethodDesc, srv *ServiceInfo, ss remote.ServerStream) {
	var (
		reply any
		err   error
	)
	defer func() {
		ss.Finish(reply, err)
	}()
	if err = ss.Start(false, false); err != nil {
		return
	}

	ctx := metadata.WithStreamContext(ss.Context())
	reply, err = desc.Handler(srv.ServiceImpl, ctx, ss.RecvMsg, s.unaryInterceptor)
	if header, ok := metadata.FromHeaderCtx(ctx); ok {
		_ = ss.SetHeader(header)
	}
	if trailer, ok := metadata.FromTrailerCtx(ctx); ok {
		ss.SetTrailer(trailer)
	}
}

func (s *server) processStreamRPC(desc *stream.Desc, srv *ServiceInfo, ss remote.ServerStream) {
	var err error
	defer func() {
		ss.Finish(nil, err)
	}()
	if err = ss.Start(desc.ClientStreams, desc.ServerStreams); err != nil {
		return
	}
	si := &interceptor.StreamServerInfo{
		FullMethod:     ss.Method(),
		IsClientStream: desc.ClientStreams,
		IsServerStream: desc.ServerStreams,
	}
	err = s.streamInterceptor(srv.ServiceImpl, ss, si, desc.Handler)
}
