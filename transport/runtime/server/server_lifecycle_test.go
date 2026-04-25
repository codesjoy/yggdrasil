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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	remote "github.com/codesjoy/yggdrasil/v3/transport"
	restserver "github.com/codesjoy/yggdrasil/v3/transport/gateway/rest"
)

func TestStopReturnsImmediatelyForInitAndClosingState(t *testing.T) {
	tests := []struct {
		name  string
		state int
	}{
		{name: "init", state: serverStateInit},
		{name: "closing", state: serverStateClosing},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer()
			s.state = tt.state

			require.NoError(t, s.Stop(context.Background()))
			require.Equal(t, serverStateClosing, s.state)
		})
	}
}

func TestServeRejectsInvalidStatesAndRegistrationErrors(t *testing.T) {
	t.Run("closing state", func(t *testing.T) {
		s := newTestServer()
		s.state = serverStateClosing

		startFlag := make(chan struct{}, 1)
		err := s.Serve(startFlag)
		require.ErrorContains(t, err, "server stopped")
		requireStartFlagClosed(t, startFlag)
	})

	t.Run("running state", func(t *testing.T) {
		s := newTestServer()
		s.state = serverStateRunning

		startFlag := make(chan struct{}, 1)
		err := s.Serve(startFlag)
		require.ErrorContains(t, err, "server already serve")
		requireStartFlagClosed(t, startFlag)
	})

	t.Run("registration failed", func(t *testing.T) {
		s := newTestServer()
		s.RegisterService(&ServiceDesc{
			ServiceName: "test.service",
			HandlerType: (*TestService)(nil),
		}, nil)

		startFlag := make(chan struct{}, 1)
		err := s.Serve(startFlag)
		require.ErrorContains(t, err, "registration failed")
		requireStartFlagClosed(t, startFlag)
	})
}

func TestStopParallel(t *testing.T) {
	s := newTestServer()
	s.servers = []remote.Server{
		&mockSlowServer{delay: 100 * time.Millisecond},
		&mockSlowServer{delay: 100 * time.Millisecond},
		&mockSlowServer{delay: 100 * time.Millisecond},
	}
	s.state = serverStateRunning

	start := time.Now()
	err := s.Stop(context.Background())
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.Equal(t, serverStateClosing, s.state)
	assert.Less(
		t,
		elapsed,
		200*time.Millisecond,
		"Stop() took too long, expected parallel execution",
	)
}

func TestStopTimeout(t *testing.T) {
	blockingRPC := &mockBlockingServer{}
	blockingREST := &mockBlockingRestServer{}

	s := newTestServer()
	s.servers = []remote.Server{blockingRPC}
	s.restEnable = true
	s.restSvr = blockingREST
	s.state = serverStateRunning

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

func TestServeStartFailureStopsServer(t *testing.T) {
	s := newTestServer()
	s.servers = []remote.Server{
		&mockSlowServer{},
		&mockFailingServer{},
	}

	startFlag := make(chan struct{}, 1)
	err := s.Serve(startFlag)

	assert.EqualError(t, err, "start failed")
	s.mu.RLock()
	assert.Equal(t, serverStateClosing, s.state)
	s.mu.RUnlock()
	requireStartFlagClosed(t, startFlag)
}

func TestServeReturnsRuntimeServerHandleErrorAndStops(t *testing.T) {
	runtimeServer := &mockRuntimeErrorServer{handleErr: errors.New("handle failed")}

	s := newTestServer()
	s.servers = []remote.Server{runtimeServer}

	startFlag := make(chan struct{}, 1)
	err := s.Serve(startFlag)

	assert.ErrorContains(t, err, "handle failed")
	assert.True(t, runtimeServer.stopCalled)
	requireStartFlagSignaledAndClosed(t, startFlag)
}

func TestServeReturnsRuntimeRestServeErrorAndStops(t *testing.T) {
	restServer := &mockRuntimeErrorRestServer{serveErr: errors.New("rest failed")}

	s := newTestServer()
	s.restEnable = true
	s.restSvr = restServer

	startFlag := make(chan struct{}, 1)
	err := s.Serve(startFlag)

	assert.ErrorContains(t, err, "rest failed")
	assert.True(t, restServer.stopCalled)
	requireStartFlagSignaledAndClosed(t, startFlag)
}

func TestServeStartFlagSignalsThenCloses(t *testing.T) {
	s := newTestServer()
	startFlag := make(chan struct{}, 1)

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- s.Serve(startFlag)
	}()

	select {
	case <-startFlag:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for start signal")
	}

	for i := 0; i < 3; i++ {
		select {
		case _, ok := <-startFlag:
			if ok {
				t.Fatalf("expected closed startFlag on read %d", i)
			}
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for closed startFlag on read %d", i)
		}
	}

	require.NoError(t, <-serveErr)
}

func TestServeFailedStartupClosesStartFlag(t *testing.T) {
	s := newTestServer()
	s.servers = []remote.Server{&mockFailingServer{}}

	startFlag := make(chan struct{}, 1)
	err := s.Serve(startFlag)

	assert.EqualError(t, err, "start failed")
	requireStartFlagClosed(t, startFlag)
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
