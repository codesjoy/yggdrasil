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

package server

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strings"

	"github.com/codesjoy/yggdrasil/v3/rpc/stream"
)

// RegisterService registers a service and its implementation to the gRPC
// server. It is called from the IDL generated code. This must be called before
// invoking Serve. If ss is non-nil (for legacy code), its type is checked to
// ensure it implements sd.HandlerType.
func (s *server) RegisterService(sd *ServiceDesc, ss interface{}) {
	if !s.ensureRegisterable("service") {
		return
	}
	if !s.validateServiceHandler(sd, ss) {
		return
	}
	s.register(sd, ss)
}

func (s *server) RegisterRestService(sd *RestServiceDesc, ss interface{}, prefix ...string) {
	if !s.restEnable {
		return
	}
	if !s.ensureRegisterable("rest service") {
		return
	}
	if !s.validateRestServiceHandler(sd, ss) {
		return
	}
	s.registerRest(sd, ss, prefix...)
}

func (s *server) RegisterRestRawHandlers(sd ...*RestRawHandlerDesc) {
	if !s.restEnable {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ensureRegisterableLocked("rest raw handlers") {
		return
	}
	for _, item := range sd {
		s.appendRestRouteLocked(item.Method, item.Path)
		s.restSvr.RawHandle(item.Method, item.Path, item.Handler)
	}
}

func (s *server) register(sd *ServiceDesc, ss interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ensureRegisterableLocked("service") {
		return
	}
	if _, ok := s.services[sd.ServiceName]; ok {
		err := fmt.Errorf(
			"fault to register service: Server.RegisterService found duplicate service registration for %q",
			sd.ServiceName,
		)
		slog.Error(err.Error())
		s.appendRegisterErrorLocked(err)
		return
	}
	s.servicesDesc[sd.ServiceName] = buildMethodInfoSet(sd)
	s.services[sd.ServiceName] = newServiceInfo(sd, ss)
}

func (s *server) registerServiceInfo(sd *ServiceDesc, ss interface{}) {
	s.services[sd.ServiceName] = newServiceInfo(sd, ss)
}

func (s *server) registerServiceDesc(desc *ServiceDesc) {
	s.servicesDesc[desc.ServiceName] = buildMethodInfoSet(desc)
}

func (s *server) registerRest(sd *RestServiceDesc, ss interface{}, prefix ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ensureRegisterableLocked("rest service") {
		return
	}
	pathPrefix := buildRoutePrefix(prefix)
	for _, item := range sd.Methods {
		method := item.Method
		path := pathPrefix + item.Path
		handler := item.Handler
		s.appendRestRouteLocked(method, path)
		s.restSvr.RPCHandle(
			method,
			path,
			func(w http.ResponseWriter, r *http.Request) (interface{}, error) {
				return handler(w, r, ss, s.unaryInterceptor)
			},
		)
	}
}

func (s *server) ensureRegisterable(action string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ensureRegisterableLocked(action)
}

func (s *server) ensureRegisterableLocked(action string) bool {
	if s.state == serverStateInit {
		return true
	}
	err := fmt.Errorf("server register %s: server is %s", action, s.stateNameLocked())
	s.appendRegisterErrorLocked(err)
	slog.Error("fault to register after server startup",
		slog.String("action", action),
		slog.String("state", s.stateNameLocked()),
	)
	return false
}

func (s *server) stateNameLocked() string {
	switch s.state {
	case serverStateInit:
		return "init"
	case serverStateRunning:
		return "running"
	case serverStateClosing:
		return "closing"
	default:
		return "unknown"
	}
}

func (s *server) serviceDescSnapshot() map[string][]methodInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string][]methodInfo, len(s.servicesDesc))
	for serviceName, methods := range s.servicesDesc {
		result[serviceName] = append([]methodInfo(nil), methods...)
	}
	return result
}

func (s *server) restRouteSnapshot() []restRouterInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]restRouterInfo(nil), s.restRouterDesc...)
}

func (s *server) validateServiceHandler(sd *ServiceDesc, ss interface{}) bool {
	if ss == nil {
		s.recordRegisterError(
			"fault to register service: Server.RegisterService handler is nil",
			errors.New("server register service: handler is nil"),
		)
		return false
	}
	interfaceType, handlerType := registrationTypes(sd.HandlerType, ss)
	if handlerType.Implements(interfaceType) {
		return true
	}
	s.recordRegisterError(
		"fault to register service: Server.RegisterService found the handler does not satisfy the interface",
		fmt.Errorf(
			"server register service %q: handler does not satisfy interface",
			sd.ServiceName,
		),
		slog.Any("handlerType", handlerType),
		slog.Any("interfaceType", interfaceType),
	)
	return false
}

func (s *server) validateRestServiceHandler(sd *RestServiceDesc, ss interface{}) bool {
	if ss == nil {
		s.recordRegisterError(
			"fault to register rest service: Server.RegisterService handler is nil",
			errors.New("server register rest service: handler is nil"),
		)
		return false
	}
	interfaceType, handlerType := registrationTypes(sd.HandlerType, ss)
	if handlerType.Implements(interfaceType) {
		return true
	}
	s.recordRegisterError(
		"fault to register rest service: Server.RegisterService found the handler does not satisfy the interface",
		errors.New("server register rest service: handler does not satisfy interface"),
		slog.Any("handlerType", handlerType),
		slog.Any("interfaceType", interfaceType),
	)
	return false
}

func (s *server) recordRegisterError(logMsg string, err error, attrs ...any) {
	slog.Error(logMsg, attrs...)
	s.appendRegisterError(err)
}

func (s *server) appendRegisterError(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.appendRegisterErrorLocked(err)
}

func (s *server) appendRegisterErrorLocked(err error) {
	if err == nil {
		return
	}
	s.registerErr = errors.Join(s.registerErr, err)
}

func (s *server) appendRestRouteLocked(method, path string) {
	s.restRouterDesc = append(s.restRouterDesc, restRouterInfo{
		Method: method,
		Path:   path,
	})
}

func registrationTypes(handlerType, ss interface{}) (reflect.Type, reflect.Type) {
	return reflect.TypeOf(handlerType).Elem(), reflect.TypeOf(ss)
}

func buildRoutePrefix(prefix []string) string {
	if len(prefix) == 0 {
		return ""
	}
	return "/" + strings.TrimPrefix(prefix[0], "/")
}

func newServiceInfo(sd *ServiceDesc, ss interface{}) *ServiceInfo {
	info := &ServiceInfo{
		ServiceImpl: ss,
		Methods:     make(map[string]*MethodDesc, len(sd.Methods)),
		Streams:     make(map[string]*stream.Desc, len(sd.Streams)),
		Metadata:    sd.Metadata,
	}
	for i := range sd.Methods {
		item := &sd.Methods[i]
		info.Methods[item.MethodName] = item
	}
	for i := range sd.Streams {
		item := &sd.Streams[i]
		info.Streams[item.StreamName] = item
	}
	return info
}

func buildMethodInfoSet(desc *ServiceDesc) []methodInfo {
	methods := make([]methodInfo, 0, len(desc.Methods)+len(desc.Streams))
	for _, item := range desc.Methods {
		methods = append(methods, methodInfo{MethodName: item.MethodName})
	}
	for _, item := range desc.Streams {
		methods = append(methods, methodInfo{
			MethodName:    item.StreamName,
			ServerStreams: item.ServerStreams,
			ClientStreams: item.ClientStreams,
		})
	}
	return methods
}
