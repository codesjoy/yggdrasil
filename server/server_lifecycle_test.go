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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v2/interceptor"
	"github.com/codesjoy/yggdrasil/v2/internal/constant"
	"github.com/codesjoy/yggdrasil/v2/remote"
	restserver "github.com/codesjoy/yggdrasil/v2/remote/rest"
	"github.com/codesjoy/yggdrasil/v2/stream"
)

func TestServerConfigureAndCurrentSettings(t *testing.T) {
	preserveServerSettings(t)

	Configure(Settings{
		Transports: []string{"grpc"},
		Interceptors: InterceptorConfig{
			Unary:  []string{"u1"},
			Stream: []string{"s1"},
		},
		RestEnabled: true,
	})

	got := CurrentSettings()
	require.Equal(t, []string{"grpc"}, got.Transports)
	require.Equal(t, []string{"u1"}, got.Interceptors.Unary)
	require.Equal(t, []string{"s1"}, got.Interceptors.Stream)
	require.True(t, got.RestEnabled)
}

func TestNewServerAndInitRemoteServer(t *testing.T) {
	t.Run("new server without transports", func(t *testing.T) {
		preserveServerSettings(t)
		preserveRestConfig(t)
		Configure(Settings{})
		restserver.Configure(nil)

		s, err := NewServer()
		require.NoError(t, err)
		require.NotNil(t, s)
	})

	t.Run("new server with rest enabled", func(t *testing.T) {
		preserveServerSettings(t)
		preserveRestConfig(t)
		Configure(Settings{RestEnabled: true})
		restserver.Configure(&restserver.Config{})

		srv, err := NewServer()
		require.NoError(t, err)
		require.NotNil(t, srv)
		inner := srv.(*server)
		require.True(t, inner.restEnable)
		require.NotNil(t, inner.restSvr)
	})

	t.Run("init remote server unknown protocol", func(t *testing.T) {
		preserveServerSettings(t)
		Configure(Settings{Transports: []string{"missing-protocol"}})
		s := &server{}
		err := s.initRemoteServer()
		require.ErrorContains(t, err, "builder for protocol missing-protocol not found")
	})

	t.Run("init remote server builder error", func(t *testing.T) {
		preserveServerSettings(t)
		remote.RegisterServerBuilder("test-builder-error", func(remote.MethodHandle) (remote.Server, error) {
			return nil, errors.New("build failed")
		})
		Configure(Settings{Transports: []string{"test-builder-error"}})

		s := &server{}
		err := s.initRemoteServer()
		require.ErrorContains(t, err, "fault to new test-builder-error remote server")
	})

	t.Run("init remote server builder success", func(t *testing.T) {
		preserveServerSettings(t)
		remote.RegisterServerBuilder("test-builder-success", func(remote.MethodHandle) (remote.Server, error) {
			return &testRemoteServer{info: remote.ServerInfo{Protocol: "test-builder-success", Address: "127.0.0.1:9000"}}, nil
		})
		Configure(Settings{Transports: []string{"test-builder-success"}})

		s := &server{}
		require.NoError(t, s.initRemoteServer())
		require.Len(t, s.servers, 1)
		require.Equal(t, "test-builder-success", s.servers[0].Info().Protocol)
	})
}

func TestServerInfo(t *testing.T) {
	si := &serverInfo{
		scheme:   "grpc",
		address:  "localhost:8080",
		svrKind:  constant.ServerKindRPC,
		metadata: map[string]string{"version": "1.0"},
	}

	assert.Equal(t, "grpc", si.Scheme())
	assert.Equal(t, "localhost:8080", si.Address())
	assert.Equal(t, constant.ServerKindRPC, si.Kind())
	assert.Equal(t, "1.0", si.Metadata()["version"])
}

func TestEndpoints(t *testing.T) {
	s := &server{
		servers: []remote.Server{},
	}

	endpoints := s.Endpoints()
	assert.Equal(t, 0, len(endpoints))

	s.restEnable = true
	s.restSvr = &mockRestServer{
		address: "localhost:9000",
		attr:    map[string]string{"type": "rest"},
	}

	endpoints = s.Endpoints()
	assert.Equal(t, 1, len(endpoints))
	assert.Equal(t, "http", endpoints[0].Scheme())
	assert.Equal(t, "localhost:9000", endpoints[0].Address())
	assert.Equal(t, constant.ServerKindRest, endpoints[0].Kind())
}

func TestServerEndpointsReportAndStateName(t *testing.T) {
	epServer := &testRemoteServer{
		info: remote.ServerInfo{
			Protocol:   "grpc",
			Address:    "127.0.0.1:9000",
			Attributes: map[string]string{"k": "v"},
		},
	}
	s := &server{
		servers:    []remote.Server{epServer},
		restEnable: true,
		restSvr:    &testRestCollector{},
	}
	endpoints := s.Endpoints()
	require.Len(t, endpoints, 2)
	require.Equal(t, "grpc", endpoints[0].Scheme())
	require.Equal(t, "127.0.0.1:9000", endpoints[0].Address())
	require.Equal(t, "v", endpoints[0].Metadata()["k"])
	require.Equal(t, "http", endpoints[1].Scheme())

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

	ch := make(chan error, 1)
	s.reportServeRuntimeError(ch, nil)
	select {
	case err := <-ch:
		t.Fatalf("unexpected error written: %v", err)
	default:
	}

	s.reportServeRuntimeError(ch, errors.New("first"))
	s.reportServeRuntimeError(ch, errors.New("second"))
	err := <-ch
	require.EqualError(t, err, "first")
}

func TestStop(t *testing.T) {
	s := &server{
		servers: []remote.Server{},
		state:   serverStateRunning,
	}

	err := s.Stop(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, serverStateClosing, s.state)
}

func TestStopFromInitState(t *testing.T) {
	s := &server{
		servers: []remote.Server{},
		state:   serverStateInit,
	}

	err := s.Stop(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, serverStateClosing, s.state)
}

func TestStopFromClosingState(t *testing.T) {
	s := &server{
		servers: []remote.Server{},
		state:   serverStateClosing,
	}

	err := s.Stop(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, serverStateClosing, s.state)
}

func TestServeEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		setupState  int
		expectError bool
		errorMsg    string
	}{
		{
			name:        "serve from closing state",
			setupState:  serverStateClosing,
			expectError: true,
			errorMsg:    "server stopped",
		},
		{
			name:        "serve from running state",
			setupState:  serverStateRunning,
			expectError: true,
			errorMsg:    "server already serve",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &server{
				servers: []remote.Server{},
				state:   tt.setupState,
			}

			startFlag := make(chan struct{}, 1)
			err := s.Serve(startFlag)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			}
		})
	}
}

func TestInitInterceptor(t *testing.T) {
	s := &server{}
	s.initInterceptor()
}

func TestServerInitInterceptorDedup(t *testing.T) {
	var unaryBuildCalls int32
	var streamBuildCalls int32
	interceptor.RegisterUnaryServerIntBuilder("server-dedup-unary", func() interceptor.UnaryServerInterceptor {
		atomic.AddInt32(&unaryBuildCalls, 1)
		return func(ctx context.Context, req interface{}, info *interceptor.UnaryServerInfo, handler interceptor.UnaryHandler) (interface{}, error) {
			return handler(ctx, req)
		}
	})
	interceptor.RegisterStreamServerIntBuilder("server-dedup-stream", func() interceptor.StreamServerInterceptor {
		atomic.AddInt32(&streamBuildCalls, 1)
		return func(srv interface{}, ss stream.ServerStream, info *interceptor.StreamServerInfo, handler stream.Handler) error {
			return handler(srv, ss)
		}
	})

	preserveServerSettings(t)
	Configure(Settings{
		Interceptors: InterceptorConfig{
			Unary:  []string{"server-dedup-unary", "server-dedup-unary"},
			Stream: []string{"server-dedup-stream", "server-dedup-stream"},
		},
	})

	s := &server{}
	s.initInterceptor()
	require.NotNil(t, s.unaryInterceptor)
	require.NotNil(t, s.streamInterceptor)
	require.Equal(t, int32(1), atomic.LoadInt32(&unaryBuildCalls))
	require.Equal(t, int32(1), atomic.LoadInt32(&streamBuildCalls))
}

func TestServerInfoInterfaceCompliance(t *testing.T) {
	si := &serverInfo{
		scheme:   "grpc",
		address:  "localhost:8080",
		svrKind:  constant.ServerKindRPC,
		metadata: map[string]string{"version": "1.0"},
	}
	var _ Endpoint = si

	assert.Equal(t, "grpc", si.Scheme())
	assert.Equal(t, "localhost:8080", si.Address())
	assert.Equal(t, constant.ServerKindRPC, si.Kind())
	assert.Equal(t, "1.0", si.Metadata()["version"])
}

type mockSlowServer struct {
	delay time.Duration
}

func (m *mockSlowServer) Info() remote.ServerInfo {
	return remote.ServerInfo{Protocol: "mock"}
}

func (m *mockSlowServer) Start() error {
	return nil
}

func (m *mockSlowServer) Handle() error {
	return nil
}

func (m *mockSlowServer) Stop(context.Context) error {
	time.Sleep(m.delay)
	return nil
}

type mockBlockingServer struct {
	stopCtx context.Context
}

func (m *mockBlockingServer) Info() remote.ServerInfo {
	return remote.ServerInfo{Protocol: "mock"}
}

func (m *mockBlockingServer) Start() error {
	return nil
}

func (m *mockBlockingServer) Handle() error {
	return nil
}

func (m *mockBlockingServer) Stop(ctx context.Context) error {
	m.stopCtx = ctx
	<-ctx.Done()
	return ctx.Err()
}

type mockBlockingRestServer struct {
	stopCtx context.Context
}

func (m *mockBlockingRestServer) GetAddress() string {
	return ""
}

func (m *mockBlockingRestServer) GetAttributes() map[string]string {
	return nil
}

func (m *mockBlockingRestServer) Info() restserver.ServerInfo {
	return m
}

func (m *mockBlockingRestServer) RPCHandle(string, string, restserver.HandlerFunc) {}

func (m *mockBlockingRestServer) RawHandle(string, string, http.HandlerFunc) {}

func (m *mockBlockingRestServer) Start() error {
	return nil
}

func (m *mockBlockingRestServer) Serve() error {
	return nil
}

func (m *mockBlockingRestServer) Stop(ctx context.Context) error {
	m.stopCtx = ctx
	<-ctx.Done()
	return ctx.Err()
}

type mockRuntimeErrorServer struct {
	stopCalled bool
	handleErr  error
}

func (m *mockRuntimeErrorServer) Info() remote.ServerInfo {
	return remote.ServerInfo{Protocol: "mock"}
}

func (m *mockRuntimeErrorServer) Start() error {
	return nil
}

func (m *mockRuntimeErrorServer) Handle() error {
	return m.handleErr
}

func (m *mockRuntimeErrorServer) Stop(context.Context) error {
	m.stopCalled = true
	return nil
}

type mockRuntimeErrorRestServer struct {
	serveErr   error
	stopCalled bool
}

func (m *mockRuntimeErrorRestServer) GetAddress() string {
	return "127.0.0.1:8080"
}

func (m *mockRuntimeErrorRestServer) GetAttributes() map[string]string {
	return nil
}

func (m *mockRuntimeErrorRestServer) Info() restserver.ServerInfo {
	return m
}

func (m *mockRuntimeErrorRestServer) RPCHandle(string, string, restserver.HandlerFunc) {}

func (m *mockRuntimeErrorRestServer) RawHandle(string, string, http.HandlerFunc) {}

func (m *mockRuntimeErrorRestServer) Start() error {
	return nil
}

func (m *mockRuntimeErrorRestServer) Serve() error {
	return m.serveErr
}

func (m *mockRuntimeErrorRestServer) Stop(context.Context) error {
	m.stopCalled = true
	return nil
}

type mockFailingServer struct {
	mockSlowServer
}

func (m *mockFailingServer) Start() error {
	return errors.New("start failed")
}

func TestStopParallel(t *testing.T) {
	s := &server{
		servers: []remote.Server{
			&mockSlowServer{delay: 100 * time.Millisecond},
			&mockSlowServer{delay: 100 * time.Millisecond},
			&mockSlowServer{delay: 100 * time.Millisecond},
		},
		state: serverStateRunning,
	}

	start := time.Now()
	err := s.Stop(context.Background())
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.Equal(t, serverStateClosing, s.state)
	assert.Less(t, elapsed, 200*time.Millisecond, "Stop() took too long, expected parallel execution")
}

func TestStopTimeout(t *testing.T) {
	blockingRPC := &mockBlockingServer{}
	blockingREST := &mockBlockingRestServer{}
	s := &server{
		servers:    []remote.Server{blockingRPC},
		restEnable: true,
		restSvr:    blockingREST,
		state:      serverStateRunning,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := s.Stop(ctx)
	elapsed := time.Since(start)

	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, elapsed, 250*time.Millisecond)
	if assert.NotNil(t, blockingRPC.stopCtx) {
		assert.ErrorIs(t, blockingRPC.stopCtx.Err(), context.DeadlineExceeded)
	}
	if assert.NotNil(t, blockingREST.stopCtx) {
		assert.ErrorIs(t, blockingREST.stopCtx.Err(), context.DeadlineExceeded)
	}
}

func TestServePartialFailure(t *testing.T) {
	successServer := &mockSlowServer{delay: 0}
	failServer := &mockFailingServer{}

	s := &server{
		servers: []remote.Server{successServer, failServer},
		state:   serverStateInit,
	}

	startFlag := make(chan struct{}, 1)
	err := s.Serve(startFlag)

	assert.Error(t, err)
	assert.Equal(t, "start failed", err.Error())

	s.mu.RLock()
	assert.Equal(t, serverStateClosing, s.state)
	s.mu.RUnlock()
}

func TestNewServerTwiceDoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		first, err := NewServer()
		assert.NoError(t, err)
		assert.NotNil(t, first)

		second, err := NewServer()
		assert.NoError(t, err)
		assert.NotNil(t, second)
	})
}

func TestServeReturnsRuntimeServerHandleErrorAndStops(t *testing.T) {
	runtimeServer := &mockRuntimeErrorServer{handleErr: errors.New("handle failed")}
	s := &server{
		servers:        []remote.Server{runtimeServer},
		state:          serverStateInit,
		services:       make(map[string]*ServiceInfo),
		servicesDesc:   make(map[string][]methodInfo),
		restRouterDesc: []restRouterInfo{},
	}

	startFlag := make(chan struct{}, 1)
	err := s.Serve(startFlag)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "handle failed")
	assert.True(t, runtimeServer.stopCalled)
}

func TestServeReturnsRuntimeRestServeErrorAndStops(t *testing.T) {
	restServer := &mockRuntimeErrorRestServer{serveErr: errors.New("rest failed")}
	s := &server{
		state:          serverStateInit,
		restEnable:     true,
		restSvr:        restServer,
		services:       make(map[string]*ServiceInfo),
		servicesDesc:   make(map[string][]methodInfo),
		restRouterDesc: []restRouterInfo{},
	}

	startFlag := make(chan struct{}, 1)
	err := s.Serve(startFlag)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rest failed")
	assert.True(t, restServer.stopCalled)
}

func TestServe_SignalHandling(t *testing.T) {
	t.Run("successful startup closes channel", func(t *testing.T) {
		s := &server{
			servers:        []remote.Server{},
			state:          serverStateInit,
			restEnable:     false,
			serverWG:       sync.WaitGroup{},
			services:       make(map[string]*ServiceInfo),
			servicesDesc:   make(map[string][]methodInfo),
			restRouterDesc: []restRouterInfo{},
			stats:          nil,
		}
		s.initInterceptor()
		startFlag := make(chan struct{}, 1)

		var serveErr error
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			serveErr = s.Serve(startFlag)
		}()

		select {
		case <-startFlag:
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for server to start")
		}

		select {
		case _, ok := <-startFlag:
			if ok {
				t.Error("startFlag should be closed after Serve() sends start signal")
			}
		case <-time.After(1 * time.Second):
			t.Error("Timeout verifying channel closure")
		}

		if err := s.Stop(context.Background()); err != nil {
			t.Errorf("Stop() error = %v", err)
		}

		wg.Wait()
		assert.NoError(t, serveErr)
	})

	t.Run("failed startup closes channel", func(t *testing.T) {
		failServer := &mockFailingServer{}

		s := &server{
			servers: []remote.Server{failServer},
			state:   serverStateInit,
		}

		startFlag := make(chan struct{}, 1)
		err := s.Serve(startFlag)

		assert.Error(t, err)

		select {
		case _, ok := <-startFlag:
			if ok {
				t.Error("startFlag should be closed after failed Serve()")
			}
		case <-time.After(1 * time.Second):
			t.Error("Timeout verifying channel closure after failure")
		}
	})

	t.Run("channel closure synchronization", func(t *testing.T) {
		s := &server{
			servers:        []remote.Server{},
			state:          serverStateInit,
			restEnable:     false,
			serverWG:       sync.WaitGroup{},
			services:       make(map[string]*ServiceInfo),
			servicesDesc:   make(map[string][]methodInfo),
			restRouterDesc: []restRouterInfo{},
		}
		s.initInterceptor()
		startFlag := make(chan struct{}, 1)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.Serve(startFlag)
		}()

		<-startFlag

		for i := 0; i < 5; i++ {
			select {
			case _, ok := <-startFlag:
				if ok {
					t.Errorf("Iteration %d: startFlag should be closed", i)
				}
			case <-time.After(100 * time.Millisecond):
				t.Error("Timeout reading from closed channel")
			}
		}

		if err := s.Stop(context.Background()); err != nil {
			t.Errorf("Stop() error = %v", err)
		}

		wg.Wait()
	})
}
