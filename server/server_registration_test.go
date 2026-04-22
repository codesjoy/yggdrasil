package server

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/interceptor"
	"github.com/codesjoy/yggdrasil/v3/stream"
)

func TestRegisterServiceInfo(t *testing.T) {
	s := newTestServer()
	desc := &ServiceDesc{
		ServiceName: "test.service",
		Methods: []MethodDesc{
			{MethodName: "TestMethod"},
		},
		Streams: []stream.Desc{
			{
				StreamName:    "TestStream",
				ServerStreams: true,
			},
		},
		Metadata: "test metadata",
	}

	serviceImpl := &TestServiceImpl{}
	s.registerServiceInfo(desc, serviceImpl)

	info := s.services["test.service"]
	require.NotNil(t, info)
	require.Equal(t, serviceImpl, info.ServiceImpl)
	require.Equal(t, "test metadata", info.Metadata)
	require.Contains(t, info.Methods, "TestMethod")
	require.Contains(t, info.Streams, "TestStream")
}

func TestRegisterServiceDesc(t *testing.T) {
	s := newTestServer()
	desc := &ServiceDesc{
		ServiceName: "test.service",
		Methods: []MethodDesc{
			{MethodName: "TestMethod"},
		},
		Streams: []stream.Desc{
			{
				StreamName:    "TestStream",
				ServerStreams: true,
			},
		},
	}

	s.registerServiceDesc(desc)

	methods := s.servicesDesc["test.service"]
	require.Len(t, methods, 2)
	require.Equal(t, "TestMethod", methods[0].MethodName)
	require.False(t, methods[0].ServerStreams)
	require.False(t, methods[0].ClientStreams)
	require.Equal(t, "TestStream", methods[1].MethodName)
	require.True(t, methods[1].ServerStreams)
	require.False(t, methods[1].ClientStreams)
}

func TestRegisterAddsServiceAndRejectsDuplicates(t *testing.T) {
	s := newTestServer()
	desc := &ServiceDesc{
		ServiceName: "test.service",
		HandlerType: (*TestService)(nil),
		Methods: []MethodDesc{
			{
				MethodName: "TestMethod",
				Handler: func(srv interface{}, ctx context.Context, dec func(interface{}) error, unary interceptor.UnaryServerInterceptor) (interface{}, error) {
					return "test response", nil
				},
			},
		},
	}

	impl := &TestServiceImpl{}
	s.register(desc, impl)

	info, exists := s.services["test.service"]
	require.True(t, exists)
	require.Equal(t, impl, info.ServiceImpl)
	require.Len(t, info.Methods, 1)
	require.Len(t, s.servicesDesc["test.service"], 1)

	s.register(desc, impl)
	require.Error(t, s.registerErr)
	require.Contains(t, s.registerErr.Error(), "duplicate service registration")
}

func TestRegisterRestAndRawHandlers(t *testing.T) {
	collector := &testRestCollector{}
	s := newTestServer()
	s.restEnable = true
	s.restSvr = collector

	restDesc := &RestServiceDesc{
		HandlerType: (*TestService)(nil),
		Methods: []RestMethodDesc{
			{
				Method: http.MethodGet,
				Path:   "/items",
				Handler: func(w http.ResponseWriter, r *http.Request, srv interface{}, unary interceptor.UnaryServerInterceptor) (interface{}, error) {
					return "ok", nil
				},
			},
		},
	}

	s.registerRest(restDesc, &TestServiceImpl{}, "/api")
	require.Len(t, s.restRouterDesc, 1)
	require.Equal(t, restRouterInfo{Method: http.MethodGet, Path: "/api/items"}, s.restRouterDesc[0])
	require.Len(t, collector.rpcHandles, 1)
	require.Equal(t, rpcHandleCall{method: http.MethodGet, path: "/api/items"}, collector.rpcHandles[0])

	s.RegisterRestRawHandlers(
		&RestRawHandlerDesc{
			Method: http.MethodPost,
			Path:   "/raw",
			Handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusCreated)
			},
		},
		&RestRawHandlerDesc{
			Method: http.MethodGet,
			Path:   "/health",
			Handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
		},
	)

	require.Len(t, s.restRouterDesc, 3)
	require.Equal(t, restRouterInfo{Method: http.MethodPost, Path: "/raw"}, s.restRouterDesc[1])
	require.Equal(t, restRouterInfo{Method: http.MethodGet, Path: "/health"}, s.restRouterDesc[2])
	require.Len(t, collector.rawHandles, 2)
	require.Equal(t, rawHandleCall{method: http.MethodPost, path: "/raw"}, collector.rawHandles[0])
	require.Equal(t, rawHandleCall{method: http.MethodGet, path: "/health"}, collector.rawHandles[1])
}

func TestRegisterValidation(t *testing.T) {
	t.Run("service invalid handler type", func(t *testing.T) {
		s := newTestServer()
		s.RegisterService(&ServiceDesc{
			ServiceName: "svc",
			HandlerType: (*TestService)(nil),
		}, struct{}{})
		require.Error(t, s.registerErr)
		require.Contains(t, s.registerErr.Error(), "does not satisfy interface")
	})

	t.Run("rest service success", func(t *testing.T) {
		s, collector := newRestRegistrationServer()
		s.RegisterRestService(&RestServiceDesc{
			HandlerType: (*TestService)(nil),
			Methods: []RestMethodDesc{
				{
					Method: http.MethodGet,
					Path:   "/items",
					Handler: func(w http.ResponseWriter, r *http.Request, srv interface{}, unary interceptor.UnaryServerInterceptor) (interface{}, error) {
						return "ok", nil
					},
				},
			},
		}, &TestServiceImpl{}, "/api")

		require.Len(t, s.restRouterDesc, 1)
		require.Equal(t, "/api/items", s.restRouterDesc[0].Path)
		require.Len(t, collector.rpcHandles, 1)
		require.Equal(t, "/api/items", collector.rpcHandles[0].path)
	})

	t.Run("rest service nil handler", func(t *testing.T) {
		s, _ := newRestRegistrationServer()
		s.RegisterRestService(&RestServiceDesc{HandlerType: (*TestService)(nil)}, nil)
		require.Error(t, s.registerErr)
		require.Contains(t, s.registerErr.Error(), "handler is nil")
	})

	t.Run("rest service invalid handler type", func(t *testing.T) {
		s, _ := newRestRegistrationServer()
		s.RegisterRestService(&RestServiceDesc{HandlerType: (*TestService)(nil)}, struct{}{})
		require.Error(t, s.registerErr)
		require.Contains(t, s.registerErr.Error(), "does not satisfy interface")
	})
}

func TestServeReturnsRegistrationErrors(t *testing.T) {
	t.Run("invalid service registration", func(t *testing.T) {
		s := newTestServer()
		s.RegisterService(&ServiceDesc{
			ServiceName: "test.service",
			HandlerType: (*TestService)(nil),
		}, nil)

		startFlag := make(chan struct{}, 1)
		err := s.Serve(startFlag)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "registration failed")
		requireStartFlagClosed(t, startFlag)
	})

	t.Run("duplicate service registration", func(t *testing.T) {
		s := newTestServer()
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
		requireStartFlagClosed(t, startFlag)
	})

	t.Run("invalid rest registration", func(t *testing.T) {
		s, _ := newRestRegistrationServer()
		s.RegisterRestService(&RestServiceDesc{HandlerType: (*TestService)(nil)}, struct{}{})

		startFlag := make(chan struct{}, 1)
		err := s.Serve(startFlag)
		require.Error(t, err)
		require.Contains(t, err.Error(), "registration failed")
		requireStartFlagClosed(t, startFlag)
	})
}

func TestRestRegistrationNoopsWhenRESTDisabled(t *testing.T) {
	s := newTestServer()

	s.RegisterRestService(&RestServiceDesc{
		HandlerType: (*TestService)(nil),
		Methods:     []RestMethodDesc{},
	}, &TestServiceImpl{})
	s.RegisterRestRawHandlers(&RestRawHandlerDesc{
		Method: http.MethodGet,
		Path:   "/health",
		Handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	})

	assert.Empty(t, s.restRouterDesc)
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
			s, _ := newRestRegistrationServer()
			s.state = tt.state

			s.RegisterService(&ServiceDesc{
				ServiceName: "late.service",
				HandlerType: (*TestService)(nil),
			}, &TestServiceImpl{})

			s.RegisterRestService(&RestServiceDesc{
				HandlerType: (*TestService)(nil),
				Methods: []RestMethodDesc{
					{
						Method: http.MethodGet,
						Path:   "/late",
						Handler: func(w http.ResponseWriter, r *http.Request, srv interface{}, unary interceptor.UnaryServerInterceptor) (interface{}, error) {
							return nil, nil
						},
					},
				},
			}, &TestServiceImpl{})

			s.RegisterRestRawHandlers(&RestRawHandlerDesc{
				Method: http.MethodGet,
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

func newRestRegistrationServer() (*server, *testRestCollector) {
	collector := &testRestCollector{}
	s := newTestServer()
	s.restEnable = true
	s.restSvr = collector
	return s, collector
}
