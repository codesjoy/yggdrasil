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
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source"
	"github.com/codesjoy/yggdrasil/v3/config/source/memory"
	"github.com/codesjoy/yggdrasil/v3/governor"
	"github.com/codesjoy/yggdrasil/v3/internal/constant"
	"github.com/codesjoy/yggdrasil/v3/module"
	"github.com/codesjoy/yggdrasil/v3/registry"
	yserver "github.com/codesjoy/yggdrasil/v3/server"
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
		require.NoError(t, manager.LoadLayer("test", config.PriorityOverride, memory.NewSource("test", data)))
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

func newTestAppWithConfig(t *testing.T, name string, data map[string]any, ops ...Option) (*App, *config.Manager) {
	t.Helper()

	manager := newTestManager(t, data)
	ops = append([]Option{WithConfigManager(manager)}, ops...)
	return newTestApp(t, name, ops...), manager
}

func newInitializedAppWithConfig(t *testing.T, name string, data map[string]any, ops ...Option) (*App, *config.Manager) {
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

func findBindingDiag(t *testing.T, diag module.Diagnostics, spec string) module.CapabilityBindingDiag {
	t.Helper()
	for _, item := range diag.Bindings {
		if item.Spec == spec {
			return item
		}
	}
	t.Fatalf("binding %q not found", spec)
	return module.CapabilityBindingDiag{}
}

type mockRegistry struct {
	mock.Mock
	registered   bool
	deregistered bool
}

func (m *mockRegistry) Register(ctx context.Context, instance registry.Instance) error {
	args := m.Called(ctx, instance)
	m.registered = true
	return args.Error(0)
}

func (m *mockRegistry) Deregister(ctx context.Context, instance registry.Instance) error {
	args := m.Called(ctx, instance)
	m.deregistered = true
	return args.Error(0)
}

func (m *mockRegistry) Type() string {
	args := m.Called()
	return args.String(0)
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

type blockingInternalServer struct {
	stopCtx context.Context
}

func (b *blockingInternalServer) Serve() error {
	return nil
}

func (b *blockingInternalServer) Stop(ctx context.Context) error {
	b.stopCtx = ctx
	<-ctx.Done()
	return ctx.Err()
}

type blockingAppServer struct {
	stopCtx context.Context
	endpts  []yserver.Endpoint
}

func (b *blockingAppServer) RegisterService(*yserver.ServiceDesc, interface{})                    {}
func (b *blockingAppServer) RegisterRestService(*yserver.RestServiceDesc, interface{}, ...string) {}
func (b *blockingAppServer) RegisterRestRawHandlers(...*yserver.RestRawHandlerDesc)               {}
func (b *blockingAppServer) Serve(chan<- struct{}) error                                          { return nil }

func (b *blockingAppServer) Stop(ctx context.Context) error {
	b.stopCtx = ctx
	<-ctx.Done()
	return ctx.Err()
}

func (b *blockingAppServer) Endpoints() []yserver.Endpoint {
	return b.endpts
}

type runningAppServer struct {
	stopCtx  context.Context
	stopCh   chan struct{}
	stopOnce sync.Once
}

func (r *runningAppServer) RegisterService(*yserver.ServiceDesc, interface{})                    {}
func (r *runningAppServer) RegisterRestService(*yserver.RestServiceDesc, interface{}, ...string) {}
func (r *runningAppServer) RegisterRestRawHandlers(...*yserver.RestRawHandlerDesc)               {}

func (r *runningAppServer) Serve(startFlag chan<- struct{}) error {
	if startFlag != nil {
		startFlag <- struct{}{}
	}
	if r.stopCh == nil {
		return nil
	}
	<-r.stopCh
	return nil
}

func (r *runningAppServer) Stop(ctx context.Context) error {
	r.stopCtx = ctx
	r.stopOnce.Do(func() {
		if r.stopCh == nil {
			r.stopCh = make(chan struct{})
		}
		close(r.stopCh)
	})
	return nil
}

func (r *runningAppServer) Endpoints() []yserver.Endpoint {
	return nil
}

type failingInternalServer struct {
	serveErr error
	stopCtx  context.Context
}

func (f *failingInternalServer) Serve() error {
	return f.serveErr
}

func (f *failingInternalServer) Stop(ctx context.Context) error {
	f.stopCtx = ctx
	return nil
}

type stubAppEndpoint struct {
	scheme   string
	address  string
	metadata map[string]string
	kind     constant.ServerKind
}

func (e stubAppEndpoint) Scheme() string {
	return e.scheme
}

func (e stubAppEndpoint) Address() string {
	return e.address
}

func (e stubAppEndpoint) Metadata() map[string]string {
	return e.metadata
}

func (e stubAppEndpoint) Kind() constant.ServerKind {
	return e.kind
}

func createMockRegistry() *mockRegistry {
	return &mockRegistry{}
}

func createMockInternalServer() *mockInternalServer {
	return &mockInternalServer{}
}
