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
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v2/interceptor"
	"github.com/codesjoy/yggdrasil/v2/stream"
)

func TestRegisterServiceInfo(t *testing.T) {
	s := &server{
		services:     make(map[string]*ServiceInfo),
		servicesDesc: make(map[string][]methodInfo),
	}

	serviceDesc := &ServiceDesc{
		ServiceName: "test.service",
		Methods: []MethodDesc{
			{
				MethodName: "TestMethod",
				Handler:    nil,
			},
		},
		Streams: []stream.Desc{
			{
				StreamName:    "TestStream",
				ServerStreams: true,
				ClientStreams: false,
			},
		},
		Metadata: "test metadata",
	}

	serviceImpl := &TestServiceImpl{}
	s.registerServiceInfo(serviceDesc, serviceImpl)

	info := s.services["test.service"]
	assert.NotNil(t, info)
	assert.Equal(t, serviceImpl, info.ServiceImpl)
	assert.Equal(t, "test metadata", info.Metadata)
	assert.Equal(t, 1, len(info.Methods))
	assert.Equal(t, "TestMethod", info.Methods["TestMethod"].MethodName)
	assert.Equal(t, 1, len(info.Streams))
	assert.Equal(t, "TestStream", info.Streams["TestStream"].StreamName)
}

func TestRegisterServiceDesc(t *testing.T) {
	s := &server{
		services:     make(map[string]*ServiceInfo),
		servicesDesc: make(map[string][]methodInfo),
	}

	serviceDesc := &ServiceDesc{
		ServiceName: "test.service",
		Methods: []MethodDesc{
			{
				MethodName: "TestMethod",
			},
		},
		Streams: []stream.Desc{
			{
				StreamName:    "TestStream",
				ServerStreams: true,
				ClientStreams: false,
			},
		},
	}

	s.registerServiceDesc(serviceDesc)

	methods := s.servicesDesc["test.service"]
	assert.Equal(t, 2, len(methods))
	assert.Equal(t, "TestMethod", methods[0].MethodName)
	assert.False(t, methods[0].ServerStreams)
	assert.False(t, methods[0].ClientStreams)
	assert.Equal(t, "TestStream", methods[1].MethodName)
	assert.True(t, methods[1].ServerStreams)
	assert.False(t, methods[1].ClientStreams)
}

func TestRegister(t *testing.T) {
	s := &server{
		services:     make(map[string]*ServiceInfo),
		servicesDesc: make(map[string][]methodInfo),
	}

	serviceDesc := &ServiceDesc{
		ServiceName: "test.service",
		HandlerType: (*TestService)(nil),
		Methods: []MethodDesc{
			{
				MethodName: "TestMethod",
				Handler: func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor interceptor.UnaryServerInterceptor) (interface{}, error) {
					return "test response", nil
				},
			},
		},
		Streams: []stream.Desc{},
	}

	serviceImpl := &TestServiceImpl{}
	s.register(serviceDesc, serviceImpl)

	serviceInfo, exists := s.services["test.service"]
	assert.True(t, exists)
	assert.Equal(t, serviceImpl, serviceInfo.ServiceImpl)
	assert.Equal(t, 1, len(serviceInfo.Methods))
	assert.Equal(t, 1, len(s.servicesDesc["test.service"]))
}

func TestRegisterDuplicate(t *testing.T) {
	s := &server{
		services:     make(map[string]*ServiceInfo),
		servicesDesc: make(map[string][]methodInfo),
	}

	serviceDesc := &ServiceDesc{
		ServiceName: "duplicate.service",
		HandlerType: (*TestService)(nil),
		Methods:     []MethodDesc{},
	}

	serviceImpl := &TestServiceImpl{}
	s.register(serviceDesc, serviceImpl)

	_, exists := s.services["duplicate.service"]
	assert.True(t, exists)
}

func TestRegisterRest(t *testing.T) {
	s := &server{
		restRouterDesc: []restRouterInfo{},
	}
	s.restSvr = &mockRestServer{}

	restServiceDesc := &RestServiceDesc{
		HandlerType: (*TestService)(nil),
		Methods: []RestMethodDesc{
			{
				Method: "GET",
				Path:   "/test",
				Handler: func(w http.ResponseWriter, r *http.Request, srv interface{}, interceptor interceptor.UnaryServerInterceptor) (interface{}, error) {
					return "rest response", nil
				},
			},
		},
	}

	serviceImpl := &TestServiceImpl{}
	s.registerRest(restServiceDesc, serviceImpl, "/api")

	assert.Equal(t, 1, len(s.restRouterDesc))
	assert.Equal(t, "GET", s.restRouterDesc[0].Method)
	assert.Equal(t, "/api/test", s.restRouterDesc[0].Path)
}

func TestRegisterRestRawHandlers(t *testing.T) {
	s := &server{
		restRouterDesc: []restRouterInfo{},
		restEnable:     true,
	}
	s.restSvr = &mockRestServer{}

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	rawHandlers := []*RestRawHandlerDesc{
		{
			Method:  "POST",
			Path:    "/raw",
			Handler: handler,
		},
		{
			Method:  "GET",
			Path:    "/health",
			Handler: handler,
		},
	}

	s.RegisterRestRawHandlers(rawHandlers...)

	assert.Equal(t, 2, len(s.restRouterDesc))
	assert.Equal(t, "POST", s.restRouterDesc[0].Method)
	assert.Equal(t, "/raw", s.restRouterDesc[0].Path)
	assert.Equal(t, "GET", s.restRouterDesc[1].Method)
	assert.Equal(t, "/health", s.restRouterDesc[1].Path)
}

func TestRegisterServiceAndRestServiceValidation(t *testing.T) {
	t.Run("register service invalid handler type", func(t *testing.T) {
		s := &server{
			services:     map[string]*ServiceInfo{},
			servicesDesc: map[string][]methodInfo{},
		}
		s.RegisterService(&ServiceDesc{
			ServiceName: "svc",
			HandlerType: (*TestService)(nil),
		}, struct{}{})
		require.Error(t, s.registerErr)
		require.Contains(t, s.registerErr.Error(), "does not satisfy interface")
	})

	t.Run("register rest service success", func(t *testing.T) {
		s := &server{
			restEnable:     true,
			restRouterDesc: []restRouterInfo{},
			restSvr:        &testRestCollector{},
		}
		s.RegisterRestService(&RestServiceDesc{
			HandlerType: (*TestService)(nil),
			Methods: []RestMethodDesc{
				{
					Method: "GET",
					Path:   "/items",
					Handler: func(w http.ResponseWriter, r *http.Request, srv interface{}, interceptor interceptor.UnaryServerInterceptor) (interface{}, error) {
						return "ok", nil
					},
				},
			},
		}, &TestServiceImpl{}, "/api")

		require.Len(t, s.restRouterDesc, 1)
		require.Equal(t, "/api/items", s.restRouterDesc[0].Path)
	})

	t.Run("register rest service nil handler", func(t *testing.T) {
		s := &server{
			restEnable:     true,
			restRouterDesc: []restRouterInfo{},
			restSvr:        &testRestCollector{},
		}
		s.RegisterRestService(&RestServiceDesc{HandlerType: (*TestService)(nil)}, nil)
		require.Error(t, s.registerErr)
		require.Contains(t, s.registerErr.Error(), "handler is nil")
	})

	t.Run("register rest service invalid handler type", func(t *testing.T) {
		s := &server{
			restEnable:     true,
			restRouterDesc: []restRouterInfo{},
			restSvr:        &testRestCollector{},
		}
		s.RegisterRestService(&RestServiceDesc{
			HandlerType: (*TestService)(nil),
		}, struct{}{})
		require.Error(t, s.registerErr)
		require.Contains(t, s.registerErr.Error(), "does not satisfy interface")
	})
}

func TestRegisterServiceErrorIsReturnedByServe(t *testing.T) {
	s := &server{
		services:     map[string]*ServiceInfo{},
		servicesDesc: map[string][]methodInfo{},
	}
	desc := &ServiceDesc{
		ServiceName: "test.service",
		HandlerType: (*TestService)(nil),
	}
	s.RegisterService(desc, nil)

	startFlag := make(chan struct{}, 1)
	err := s.Serve(startFlag)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "registration failed")
	select {
	case _, ok := <-startFlag:
		assert.False(t, ok)
	case <-time.After(time.Second):
		t.Fatal("startFlag should be closed when registration failed")
	}
}

func TestDuplicateRegisterServiceErrorIsReturnedByServe(t *testing.T) {
	s := &server{
		services:     map[string]*ServiceInfo{},
		servicesDesc: map[string][]methodInfo{},
	}
	desc := &ServiceDesc{
		ServiceName: "test.service",
		HandlerType: (*TestService)(nil),
	}
	impl := &TestServiceImpl{}
	s.RegisterService(desc, impl)
	s.RegisterService(desc, impl)

	startFlag := make(chan struct{}, 1)
	err := s.Serve(startFlag)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate service registration")
	select {
	case _, ok := <-startFlag:
		assert.False(t, ok)
	case <-time.After(time.Second):
		t.Fatal("startFlag should be closed when registration failed")
	}
}

func TestRegisterRestServiceDisabled(t *testing.T) {
	s := &server{
		restEnable:     false,
		restRouterDesc: []restRouterInfo{},
	}

	restServiceDesc := &RestServiceDesc{
		HandlerType: (*TestService)(nil),
		Methods:     []RestMethodDesc{},
	}

	serviceImpl := &TestServiceImpl{}
	s.RegisterRestService(restServiceDesc, serviceImpl)

	assert.Equal(t, 0, len(s.restRouterDesc))
}

func TestRegisterRestRawHandlersDisabled(t *testing.T) {
	s := &server{
		restEnable:     false,
		restRouterDesc: []restRouterInfo{},
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	rawHandlers := []*RestRawHandlerDesc{
		{
			Method:  "GET",
			Path:    "/health",
			Handler: handler,
		},
	}

	s.RegisterRestRawHandlers(rawHandlers...)

	assert.Equal(t, 0, len(s.restRouterDesc))
}

func TestServerRegistrationErrorsBubbleToServe(t *testing.T) {
	s := &server{
		services:       map[string]*ServiceInfo{},
		servicesDesc:   map[string][]methodInfo{},
		restRouterDesc: []restRouterInfo{},
		restEnable:     true,
		restSvr:        &testRestCollector{},
	}
	s.RegisterRestService(&RestServiceDesc{HandlerType: (*TestService)(nil)}, struct{}{})

	err := s.Serve(make(chan struct{}, 1))
	require.Error(t, err)
	require.Contains(t, err.Error(), "registration failed")
}

func TestRegisterRejectedWhenServerNotInit(t *testing.T) {
	tests := []struct {
		name  string
		state int
	}{
		{name: "running", state: serverStateRunning},
		{name: "closing", state: serverStateClosing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &server{
				services:       map[string]*ServiceInfo{},
				servicesDesc:   map[string][]methodInfo{},
				restRouterDesc: []restRouterInfo{},
				restEnable:     true,
				restSvr:        &mockRestServer{},
				state:          tt.state,
			}

			s.RegisterService(&ServiceDesc{
				ServiceName: "late.service",
				HandlerType: (*TestService)(nil),
			}, &TestServiceImpl{})

			s.RegisterRestService(&RestServiceDesc{
				HandlerType: (*TestService)(nil),
				Methods: []RestMethodDesc{
					{
						Method: "GET",
						Path:   "/late",
						Handler: func(w http.ResponseWriter, r *http.Request, srv interface{}, interceptor interceptor.UnaryServerInterceptor) (interface{}, error) {
							return nil, nil
						},
					},
				},
			}, &TestServiceImpl{})

			s.RegisterRestRawHandlers(&RestRawHandlerDesc{
				Method: "GET",
				Path:   "/raw",
				Handler: func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				},
			})

			s.mu.RLock()
			defer s.mu.RUnlock()
			assert.Empty(t, s.services)
			assert.Empty(t, s.servicesDesc)
			assert.Empty(t, s.restRouterDesc)
			assert.Error(t, s.registerErr)
		})
	}
}
