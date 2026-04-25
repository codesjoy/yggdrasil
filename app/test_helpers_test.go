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

package app

import (
	"context"
	"flag"
	"net/http"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/admin/governor"
	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source"
	"github.com/codesjoy/yggdrasil/v3/config/source/memory"
	"github.com/codesjoy/yggdrasil/v3/module"
	"github.com/codesjoy/yggdrasil/v3/rpc/interceptor"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
	yserver "github.com/codesjoy/yggdrasil/v3/transport/runtime/server"
)

type mockConfigSource struct {
	name       string
	data       map[string]any
	closeCount *int32
}

func (m *mockConfigSource) Kind() string { return "mock" }
func (m *mockConfigSource) Name() string { return m.name }
func (m *mockConfigSource) Read() (source.Data, error) {
	return source.NewMapData(m.data), nil
}

func (m *mockConfigSource) Close() error {
	if m.closeCount != nil {
		atomic.AddInt32(m.closeCount, 1)
	}
	return nil
}

func withTestFlagSet(t *testing.T) {
	t.Helper()
	oldCommandLine := flag.CommandLine
	oldArgs := os.Args
	flag.CommandLine = flag.NewFlagSet("yggdrasil-test", flag.ContinueOnError)
	os.Args = []string{"yggdrasil-test"}
	t.Cleanup(func() {
		flag.CommandLine = oldCommandLine
		os.Args = oldArgs
	})
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	oldDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		_ = os.Chdir(oldDir)
	})
}

func minimalV3Config(serverTransport string) map[string]any {
	return map[string]any{
		"yggdrasil": map[string]any{
			"admin": map[string]any{
				"governor": map[string]any{
					"port": 0,
				},
			},
			"server": map[string]any{
				"transports": []any{serverTransport},
			},
			"transports": map[string]any{
				"grpc": map[string]any{
					"server": map[string]any{},
					"client": map[string]any{},
				},
				"http": map[string]any{
					"server": map[string]any{},
					"client": map[string]any{},
				},
			},
		},
	}
}

func newTestManager(t *testing.T, data map[string]any) *config.Manager {
	t.Helper()

	manager := config.NewManager()
	if data != nil {
		require.NoError(
			t,
			manager.LoadLayer("test", config.PriorityOverride, memory.NewSource("test", data)),
		)
	}
	return manager
}

func newTestApp(t *testing.T, name string, ops ...Option) *App {
	t.Helper()

	app, err := New(name, ops...)
	require.NoError(t, err)
	return app
}

func newInitializedApp(t *testing.T, name string, ops ...Option) *App {
	t.Helper()

	app := newTestApp(t, name, ops...)
	require.NoError(t, app.initializeLocked(context.Background()))
	return app
}

func newTestAppWithConfig(
	t *testing.T,
	name string,
	data map[string]any,
	ops ...Option,
) (*App, *config.Manager) {
	t.Helper()

	manager := newTestManager(t, data)
	ops = append([]Option{WithConfigManager(manager)}, ops...)
	return newTestApp(t, name, ops...), manager
}

func newInitializedAppWithConfig(
	t *testing.T,
	name string,
	data map[string]any,
	ops ...Option,
) (*App, *config.Manager) {
	t.Helper()

	manager := newTestManager(t, data)
	ops = append([]Option{WithConfigManager(manager)}, ops...)
	return newInitializedApp(t, name, ops...), manager
}

func serveGovernorAsync(_ *testing.T, gov *governor.Server) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- gov.Serve()
	}()
	return errCh
}

func waitGovernorStarted(t *testing.T, gov *governor.Server) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, gov.WaitStarted(ctx))
}

func requireAsyncNoError(t *testing.T, errCh <-chan error, timeoutMsg string) {
	t.Helper()

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal(timeoutMsg)
	}
}

func findBindingDiag(
	t *testing.T,
	diag module.Diagnostics,
	spec string,
) module.CapabilityBindingDiag {
	t.Helper()
	for _, item := range diag.Bindings {
		if item.Spec == spec {
			return item
		}
	}
	t.Fatalf("binding %q not found", spec)
	return module.CapabilityBindingDiag{}
}

func requireAssemblyErrorCode(t *testing.T, err error, code yassembly.ErrorCode) *yassembly.Error {
	t.Helper()
	var assemblyErr *yassembly.Error
	require.ErrorAs(t, err, &assemblyErr)
	require.Equal(t, code, assemblyErr.Code)
	return assemblyErr
}

const testAssemblyServiceName = "test.assembly.service"

type testAssemblyService interface {
	Ping(context.Context, any) (any, error)
}

type testAssemblyServiceImpl struct{}

func (*testAssemblyServiceImpl) Ping(context.Context, any) (any, error) {
	return nil, nil
}

var testAssemblyRPCServiceDesc = yserver.ServiceDesc{
	ServiceName: testAssemblyServiceName,
	HandlerType: (*testAssemblyService)(nil),
	Methods: []yserver.MethodDesc{
		{
			MethodName: "Ping",
			Handler: func(
				srv interface{},
				ctx context.Context,
				_ func(interface{}) error,
				_ interceptor.UnaryServerInterceptor,
			) (interface{}, error) {
				return srv.(testAssemblyService).Ping(ctx, nil)
			},
		},
	},
}

var testAssemblyRESTServiceDesc = yserver.RestServiceDesc{
	HandlerType: (*testAssemblyService)(nil),
	Methods: []yserver.RestMethodDesc{
		{
			Method: http.MethodGet,
			Path:   "/test/ping",
			Handler: func(
				_ http.ResponseWriter,
				r *http.Request,
				srv interface{},
				_ interceptor.UnaryServerInterceptor,
			) (interface{}, error) {
				return srv.(testAssemblyService).Ping(r.Context(), nil)
			},
		},
	},
}

type mockInternalServer struct {
	mock.Mock
	started bool
	stopped bool
}

func (m *mockInternalServer) Serve() error {
	args := m.Called()
	m.started = true
	return args.Error(0)
}

func (m *mockInternalServer) Stop(ctx context.Context) error {
	args := m.Called(ctx)
	m.stopped = true
	return args.Error(0)
}

// --- Assembly shared helpers ---

type transportRecorder struct {
	startCalls  int32
	handleCalls int32
	stopCalls   int32
	started     chan struct{}
	stopCh      chan struct{}
}

func newTransportRecorder() *transportRecorder {
	return &transportRecorder{
		started: make(chan struct{}),
		stopCh:  make(chan struct{}),
	}
}

func (r *transportRecorder) buildServer() remote.Server {
	return &recordedRemoteServer{recorder: r}
}

type recordedRemoteServer struct {
	recorder *transportRecorder
}

func (s *recordedRemoteServer) Start() error {
	atomic.AddInt32(&s.recorder.startCalls, 1)
	select {
	case <-s.recorder.started:
	default:
		close(s.recorder.started)
	}
	return nil
}

func (s *recordedRemoteServer) Handle() error {
	atomic.AddInt32(&s.recorder.handleCalls, 1)
	<-s.recorder.stopCh
	return nil
}

func (s *recordedRemoteServer) Stop(context.Context) error {
	atomic.AddInt32(&s.recorder.stopCalls, 1)
	select {
	case <-s.recorder.stopCh:
	default:
		close(s.recorder.stopCh)
	}
	return nil
}

func (s *recordedRemoteServer) Info() remote.ServerInfo {
	return remote.ServerInfo{
		Protocol: "test",
		Address:  "127.0.0.1:0",
	}
}

type testTransportModule struct {
	recorder *transportRecorder
}

func (m testTransportModule) Name() string { return "test.transport.server" }

func (m testTransportModule) Capabilities() []module.Capability {
	return []module.Capability{
		{
			Spec: transportServerProviderCapabilitySpec,
			Name: "test",
			Value: remote.NewTransportServerProvider(
				"test",
				func(remote.MethodHandle) (remote.Server, error) {
					return m.recorder.buildServer(), nil
				},
			),
		},
	}
}

func assemblyTestConfig(enableREST bool) map[string]any {
	root := map[string]any{
		"yggdrasil": map[string]any{
			"admin": map[string]any{
				"governor": map[string]any{
					"port": 0,
				},
			},
			"server": map[string]any{
				"transports": []any{"test"},
			},
		},
	}
	if enableREST {
		root["yggdrasil"].(map[string]any)["transports"] = map[string]any{
			"http": map[string]any{
				"rest": map[string]any{
					"host": "127.0.0.1",
					"port": 0,
				},
			},
		}
	}
	return root
}

func waitForChannel(t *testing.T, ch <-chan struct{}, timeout time.Duration, msg string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(timeout):
		t.Fatal(msg)
	}
}

func containsPlannedModule(spec *yassembly.Spec, name string) bool {
	if spec == nil {
		return false
	}
	for _, item := range spec.Modules {
		if item.Name == name {
			return true
		}
	}
	return false
}
