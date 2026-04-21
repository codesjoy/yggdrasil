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

	"github.com/codesjoy/yggdrasil/v2/metadata"
	"github.com/codesjoy/yggdrasil/v2/remote"
	restserver "github.com/codesjoy/yggdrasil/v2/remote/rest"
)

type TestService interface {
	TestMethod(ctx context.Context, req interface{}) (interface{}, error)
}

type TestServiceImpl struct{}

func (t *TestServiceImpl) TestMethod(context.Context, interface{}) (interface{}, error) {
	return "test response", nil
}

type testServerStream struct {
	method            string
	ctx               context.Context
	startErr          error
	startClientStream bool
	startServerStream bool
	finishReply       any
	finishErr         error
	header            metadata.MD
	trailer           metadata.MD
}

func (s *testServerStream) Method() string { return s.method }

func (s *testServerStream) Start(isClientStream, isServerStream bool) error {
	s.startClientStream = isClientStream
	s.startServerStream = isServerStream
	return s.startErr
}

func (s *testServerStream) Finish(reply any, err error) {
	s.finishReply = reply
	s.finishErr = err
}

func (s *testServerStream) SetHeader(md metadata.MD) error {
	s.header = md
	return nil
}

func (s *testServerStream) SendHeader(metadata.MD) error { return nil }

func (s *testServerStream) SetTrailer(md metadata.MD) {
	s.trailer = md
}

func (s *testServerStream) Context() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}

func (s *testServerStream) SendMsg(any) error { return nil }
func (s *testServerStream) RecvMsg(any) error { return nil }

type testRemoteServer struct {
	info      remote.ServerInfo
	startErr  error
	handleErr error
	stopErr   error
}

func (s *testRemoteServer) Start() error               { return s.startErr }
func (s *testRemoteServer) Handle() error              { return s.handleErr }
func (s *testRemoteServer) Stop(context.Context) error { return s.stopErr }
func (s *testRemoteServer) Info() remote.ServerInfo    { return s.info }

type testRestCollector struct {
	methods []string
	paths   []string
}

func (s *testRestCollector) Info() restserver.ServerInfo { return s }
func (s *testRestCollector) GetAddress() string          { return "127.0.0.1:8080" }
func (s *testRestCollector) GetAttributes() map[string]string {
	return map[string]string{"kind": "rest"}
}
func (s *testRestCollector) Start() error                               { return nil }
func (s *testRestCollector) Serve() error                               { return nil }
func (s *testRestCollector) Stop(context.Context) error                 { return nil }
func (s *testRestCollector) RawHandle(string, string, http.HandlerFunc) {}

func (s *testRestCollector) RPCHandle(method, path string, _ restserver.HandlerFunc) {
	s.methods = append(s.methods, method)
	s.paths = append(s.paths, path)
}

type mockRestServer struct {
	address string
	attr    map[string]string
}

func (m *mockRestServer) Info() restserver.ServerInfo {
	return m
}

func (m *mockRestServer) GetAddress() string {
	return m.address
}

func (m *mockRestServer) GetAttributes() map[string]string {
	return m.attr
}

func (m *mockRestServer) RPCHandle(string, string, restserver.HandlerFunc) {}

func (m *mockRestServer) RawHandle(string, string, http.HandlerFunc) {}

func (m *mockRestServer) Start() error {
	return nil
}

func (m *mockRestServer) Serve() error {
	return nil
}

func (m *mockRestServer) Stop(context.Context) error {
	return nil
}

func preserveServerSettings(t *testing.T) {
	t.Helper()
	settingsMu.RLock()
	prev := settingsV
	settingsMu.RUnlock()
	t.Cleanup(func() { Configure(prev) })
}

func preserveRestConfig(t *testing.T) {
	t.Helper()
	prev := restserver.CurrentConfig()
	t.Cleanup(func() { restserver.Configure(prev) })
}
