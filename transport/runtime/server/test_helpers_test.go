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

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	"github.com/codesjoy/yggdrasil/v3/rpc/interceptor"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
	"github.com/codesjoy/yggdrasil/v3/transport/gateway/rest"
	"github.com/codesjoy/yggdrasil/v3/transport/support/marshaler"
)

type testRuntime struct {
	settings          Settings
	statsHandler      stats.Handler
	restConfig        *rest.Config
	restProviders     map[string]rest.Provider
	marshalerBuilders map[string]marshaler.MarshalerBuilder
	serverProviders   map[string]remote.TransportServerProvider
	unaryProviders    map[string]interceptor.UnaryServerInterceptorProvider
	streamProviders   map[string]interceptor.StreamServerInterceptorProvider
}

func newTestRuntime() *testRuntime {
	return &testRuntime{
		statsHandler:      stats.NoOpHandler,
		restProviders:     map[string]rest.Provider{},
		marshalerBuilders: map[string]marshaler.MarshalerBuilder{},
		serverProviders:   map[string]remote.TransportServerProvider{},
		unaryProviders:    map[string]interceptor.UnaryServerInterceptorProvider{},
		streamProviders:   map[string]interceptor.StreamServerInterceptorProvider{},
	}
}

func newTestServer() *server {
	return &server{
		services:       map[string]*ServiceInfo{},
		servicesDesc:   map[string][]methodInfo{},
		restRouterDesc: []restRouterInfo{},
		runtime:        newTestRuntime(),
	}
}

func (r *testRuntime) ServerSettings() Settings {
	if r == nil {
		return Settings{}
	}
	return r.settings
}

func (r *testRuntime) ServerStatsHandler() stats.Handler {
	if r == nil || r.statsHandler == nil {
		return stats.NoOpHandler
	}
	return r.statsHandler
}

func (r *testRuntime) RESTConfig() *rest.Config {
	if r == nil {
		return nil
	}
	return r.restConfig
}

func (r *testRuntime) RESTMiddlewareProviders() map[string]rest.Provider {
	if r == nil {
		return map[string]rest.Provider{}
	}
	return r.restProviders
}

func (r *testRuntime) MarshalerBuilders() map[string]marshaler.MarshalerBuilder {
	if r == nil {
		return map[string]marshaler.MarshalerBuilder{}
	}
	return r.marshalerBuilders
}

func (r *testRuntime) BuildUnaryServerInterceptor(
	names []string,
) interceptor.UnaryServerInterceptor {
	return interceptor.ChainUnaryServerInterceptorsWithProviders(names, r.unaryProviders)
}

func (r *testRuntime) BuildStreamServerInterceptor(
	names []string,
) interceptor.StreamServerInterceptor {
	return interceptor.ChainStreamServerInterceptorsWithProviders(names, r.streamProviders)
}

func (r *testRuntime) TransportServerProvider(protocol string) remote.TransportServerProvider {
	if r == nil {
		return nil
	}
	return r.serverProviders[protocol]
}

type testRemoteServer struct {
	info remote.ServerInfo
}

func (s *testRemoteServer) Start() error               { return nil }
func (s *testRemoteServer) Handle() error              { return nil }
func (s *testRemoteServer) Stop(context.Context) error { return nil }
func (s *testRemoteServer) Info() remote.ServerInfo    { return s.info }

type mockRestServer struct {
	address string
	attr    map[string]string
}

func (m *mockRestServer) RPCHandle(string, string, rest.HandlerFunc) {}
func (m *mockRestServer) RawHandle(string, string, http.HandlerFunc) {}
func (m *mockRestServer) Start() error                               { return nil }
func (m *mockRestServer) Serve() error                               { return nil }
func (m *mockRestServer) Stop(context.Context) error                 { return nil }
func (m *mockRestServer) Info() rest.ServerInfo                      { return m }
func (m *mockRestServer) GetAddress() string                         { return m.address }
func (m *mockRestServer) GetAttributes() map[string]string           { return m.attr }

type rpcHandleCall struct {
	method string
	path   string
}

type rawHandleCall struct {
	method string
	path   string
}

type testRestCollector struct {
	mockRestServer
	rpcHandles []rpcHandleCall
	rawHandles []rawHandleCall
}

func (c *testRestCollector) RPCHandle(method, path string, _ rest.HandlerFunc) {
	c.rpcHandles = append(c.rpcHandles, rpcHandleCall{method: method, path: path})
}

func (c *testRestCollector) RawHandle(method, path string, _ http.HandlerFunc) {
	c.rawHandles = append(c.rawHandles, rawHandleCall{method: method, path: path})
}

func requireStartFlagClosed(t *testing.T, startFlag <-chan struct{}) {
	t.Helper()
	select {
	case _, ok := <-startFlag:
		require.False(t, ok, "startFlag should be closed")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for startFlag to close")
	}
}

func requireStartFlagSignaledAndClosed(t *testing.T, startFlag <-chan struct{}) {
	t.Helper()
	select {
	case _, ok := <-startFlag:
		require.True(t, ok, "startFlag should be signaled before close")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for startFlag signal")
	}
	requireStartFlagClosed(t, startFlag)
}
