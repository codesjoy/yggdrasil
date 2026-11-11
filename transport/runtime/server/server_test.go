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
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/admin/governor"
	"github.com/codesjoy/yggdrasil/v3/config"
	internalidentity "github.com/codesjoy/yggdrasil/v3/internal/identity"
	"github.com/codesjoy/yggdrasil/v3/rpc/interceptor"
	"github.com/codesjoy/yggdrasil/v3/rpc/stream"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
	restserver "github.com/codesjoy/yggdrasil/v3/transport/gateway/rest"
)

func TestServerInfoAndEndpoints(t *testing.T) {
	info := &serverInfo{
		protocol: "grpc",
		address:  "localhost:8080",
		svrKind:  EndpointKindRPC,
		metadata: map[string]string{"version": "1.0"},
	}
	require.Equal(t, "grpc", info.Protocol())
	require.Equal(t, "localhost:8080", info.Address())
	require.Equal(t, EndpointKindRPC, info.Kind())
	require.Equal(t, "1.0", info.Metadata()["version"])

	s := newTestServer()
	s.servers = []remote.Server{
		&testRemoteServer{
			info: remote.ServerInfo{
				Protocol:         "grpc",
				Address:          "0.0.0.0:9000",
				AdvertiseAddress: "10.0.0.10:9000",
				Attributes:       map[string]string{"k": "v"},
			},
		},
	}
	s.restEnable = true
	s.restSvr = &mockRestServer{
		address:          "0.0.0.0:8080",
		advertiseAddress: "10.0.0.10:8080",
		attr:             map[string]string{"kind": "rest"},
	}

	endpoints := s.Endpoints()
	require.Len(t, endpoints, 2)
	require.Equal(t, "grpc", endpoints[0].Protocol())
	require.Equal(t, "10.0.0.10:9000", endpoints[0].Address())
	require.Equal(t, "v", endpoints[0].Metadata()["k"])
	require.Equal(t, EndpointKindRPC, endpoints[0].Kind())
	require.Equal(t, "http", endpoints[1].Protocol())
	require.Equal(t, "10.0.0.10:8080", endpoints[1].Address())
	require.Equal(t, "rest", endpoints[1].Metadata()["kind"])
	require.Equal(t, EndpointKindRest, endpoints[1].Kind())
}

func TestStateNameAndRuntimeErrorReporting(t *testing.T) {
	s := newTestServer()

	states := map[int]string{
		serverStateInit:    "init",
		serverStateRunning: "running",
		serverStateClosing: "closing",
		42:                 "unknown",
	}
	for state, want := range states {
		s.state = state
		require.Equal(t, want, s.stateNameLocked())
	}

	errCh := make(chan error, 1)
	s.reportServeRuntimeError(errCh, nil)
	select {
	case err := <-errCh:
		t.Fatalf("unexpected error written: %v", err)
	default:
	}

	s.reportServeRuntimeError(errCh, errors.New("first"))
	s.reportServeRuntimeError(errCh, errors.New("second"))

	err := <-errCh
	assert.EqualError(t, err, "first")
}

func TestNewAndInitRemoteServer(t *testing.T) {
	t.Run("new server without transports", func(t *testing.T) {
		srv, err := New(newTestRuntime())
		require.NoError(t, err)
		require.NotNil(t, srv)
	})

	t.Run("new server with rest enabled", func(t *testing.T) {
		runtime := newTestRuntime()
		runtime.settings.RestEnabled = true
		runtime.restConfig = &restserver.Config{}

		srv, err := New(runtime)
		require.NoError(t, err)
		require.NotNil(t, srv)

		inner := srv.(*server)
		require.True(t, inner.restEnable)
		require.NotNil(t, inner.restSvr)
	})

	t.Run("init remote server unknown protocol", func(t *testing.T) {
		runtime := newTestRuntime()
		runtime.settings.Transports = []string{"missing-protocol"}

		s := newTestServer()
		s.runtime = runtime
		err := s.initRemoteServer()
		require.ErrorContains(
			t,
			err,
			"server transport provider for protocol missing-protocol not found",
		)
	})

	t.Run("init remote server builder error", func(t *testing.T) {
		runtime := newTestRuntime()
		runtime.serverProviders["test-builder-error"] = remote.NewTransportServerProvider(
			"test-builder-error",
			func(remote.MethodHandle) (remote.Server, error) {
				return nil, errors.New("build failed")
			},
		)
		runtime.settings.Transports = []string{"test-builder-error"}

		s := newTestServer()
		s.runtime = runtime
		err := s.initRemoteServer()
		require.ErrorContains(t, err, "fault to new test-builder-error remote server")
	})

	t.Run("init remote server builder success", func(t *testing.T) {
		runtime := newTestRuntime()
		runtime.serverProviders["test-builder-success"] = remote.NewTransportServerProvider(
			"test-builder-success",
			func(remote.MethodHandle) (remote.Server, error) {
				return &testRemoteServer{
					info: remote.ServerInfo{
						Protocol: "test-builder-success",
						Address:  "127.0.0.1:9000",
					},
				}, nil
			},
		)
		runtime.settings.Transports = []string{"test-builder-success"}

		s := newTestServer()
		s.runtime = runtime
		require.NoError(t, s.initRemoteServer())
		require.Len(t, s.servers, 1)
		require.Equal(t, "test-builder-success", s.servers[0].Info().Protocol)
	})
}

func TestInitInterceptorDedupsConfiguredNames(t *testing.T) {
	var unaryBuildCalls int32
	var streamBuildCalls int32

	runtime := newTestRuntime()
	runtime.settings.Interceptors = InterceptorSettings{
		Unary:  []string{"server-dedup-unary", "server-dedup-unary"},
		Stream: []string{"server-dedup-stream", "server-dedup-stream"},
	}
	runtime.unaryProviders["server-dedup-unary"] = interceptor.NewUnaryServerInterceptorProvider(
		"server-dedup-unary",
		func() interceptor.UnaryServerInterceptor {
			atomic.AddInt32(&unaryBuildCalls, 1)
			return func(
				ctx context.Context,
				req interface{},
				info *interceptor.UnaryServerInfo,
				handler interceptor.UnaryHandler,
			) (interface{}, error) {
				return handler(ctx, req)
			}
		},
	)
	runtime.streamProviders["server-dedup-stream"] = interceptor.NewStreamServerInterceptorProvider(
		"server-dedup-stream",
		func() interceptor.StreamServerInterceptor {
			atomic.AddInt32(&streamBuildCalls, 1)
			return func(
				srv interface{},
				ss stream.ServerStream,
				info *interceptor.StreamServerInfo,
				handler stream.Handler,
			) error {
				return handler(srv, ss)
			}
		},
	)

	s := newTestServer()
	s.runtime = runtime
	s.initInterceptor()

	require.NotNil(t, s.unaryInterceptor)
	require.NotNil(t, s.streamInterceptor)
	require.Equal(t, int32(1), atomic.LoadInt32(&unaryBuildCalls))
	require.Equal(t, int32(1), atomic.LoadInt32(&streamBuildCalls))
}

func TestNewTwiceDoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		first, err := New(newTestRuntime())
		assert.NoError(t, err)
		assert.NotNil(t, first)

		second, err := New(newTestRuntime())
		assert.NoError(t, err)
		assert.NotNil(t, second)
	})
}

func TestRegisterGovernorRoutesInstanceIsolation(t *testing.T) {
	govA, err := governor.NewServerWithConfig(governor.Config{}, config.NewManager())
	require.NoError(t, err)
	govB, err := governor.NewServerWithConfig(governor.Config{}, config.NewManager())
	require.NoError(t, err)

	srvA := &server{
		servicesDesc: map[string][]methodInfo{
			"svc.alpha": {
				{MethodName: "Alpha"},
			},
		},
		restRouterDesc: []restRouterInfo{
			{Method: "GET", Path: "/alpha"},
		},
	}
	srvB := &server{
		servicesDesc: map[string][]methodInfo{
			"svc.beta": {
				{MethodName: "Beta"},
			},
		},
		restRouterDesc: []restRouterInfo{
			{Method: "POST", Path: "/beta"},
		},
	}

	RegisterGovernorRoutes(govA, srvA, internalidentity.Identity{AppName: "app-a"})
	RegisterGovernorRoutes(govB, srvB, internalidentity.Identity{AppName: "app-b"})

	servicesA := governorRouteBody(t, govA, "/services")
	servicesB := governorRouteBody(t, govB, "/services")
	assert.Contains(t, servicesA, "svc.alpha")
	assert.NotContains(t, servicesA, "svc.beta")
	assert.Contains(t, servicesA, `"appName":"app-a"`)
	assert.Contains(t, servicesB, "svc.beta")
	assert.NotContains(t, servicesB, "svc.alpha")
	assert.Contains(t, servicesB, `"appName":"app-b"`)

	restA := governorRouteBody(t, govA, "/rest")
	restB := governorRouteBody(t, govB, "/rest")
	assert.Contains(t, restA, "/alpha")
	assert.NotContains(t, restA, "/beta")
	assert.Contains(t, restB, "/beta")
	assert.NotContains(t, restB, "/alpha")
}

func TestRegisterGovernorRoutesIgnoresNil(t *testing.T) {
	assert.NotPanics(t, func() {
		RegisterGovernorRoutes(nil, nil, internalidentity.Identity{})
	})
}

func governorRouteBody(t *testing.T, gov *governor.Server, path string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	gov.Handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}
