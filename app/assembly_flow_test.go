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
	"errors"
	"net/http"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	internalinstall "github.com/codesjoy/yggdrasil/v3/app/internal/install"
	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/module"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/server"
)

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

func TestOpenPrepareExposesRuntimeWithoutServing(t *testing.T) {
	recorder := newTransportRecorder()
	manager := newTestManager(t, assemblyTestConfig(false))

	app, err := Open(
		WithConfigManager(manager),
		WithAppName("prepare-runtime"),
		WithModules(testTransportModule{recorder: recorder}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	require.NoError(t, app.Prepare(context.Background()))
	require.NotNil(t, app.Runtime())
	require.NotNil(t, app.assemblySpec)
	require.NotNil(t, app.runtimeAssembly)
	require.Equal(t, int32(0), atomic.LoadInt32(&recorder.startCalls))
	require.Equal(t, int32(0), atomic.LoadInt32(&recorder.handleCalls))

	var mgr *config.Manager
	require.NoError(t, app.Runtime().Lookup(&mgr))
	require.Same(t, manager, mgr)
}

func TestComposeAndInstallRegistersBindingsAndRejectsConflicts(t *testing.T) {
	manager := newTestManager(t, assemblyTestConfig(true))

	app, err := Open(
		WithConfigManager(manager),
		WithAppName("compose-install"),
		WithModules(testTransportModule{recorder: newTransportRecorder()}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	require.NoError(t, app.Prepare(context.Background()))
	require.NoError(
		t,
		app.ComposeAndInstall(context.Background(), func(Runtime) (*BusinessBundle, error) {
			return &BusinessBundle{
				RPCBindings: []RPCBinding{
					{
						ServiceName: testAssemblyServiceName,
						Desc:        &testAssemblyRPCServiceDesc,
						Impl:        &testAssemblyServiceImpl{},
					},
				},
				RESTBindings: []RESTBinding{
					{
						Name: "library-rest",
						Desc: &testAssemblyRESTServiceDesc,
						Impl: &testAssemblyServiceImpl{},
					},
				},
				RawHTTP: []RawHTTPBinding{
					{
						Method:  http.MethodGet,
						Path:    "/healthz",
						Handler: func(http.ResponseWriter, *http.Request) {},
					},
				},
			}, nil
		}),
	)
	require.Contains(t, app.installedRPCServices, testAssemblyServiceName)
	require.Contains(
		t,
		app.installedHTTPRoutes,
		internalinstall.RouteKey(http.MethodGet, "/healthz"),
	)

	conflictApp, err := Open(
		WithConfigManager(newTestManager(t, assemblyTestConfig(true))),
		WithAppName("compose-conflict"),
		WithModules(testTransportModule{recorder: newTransportRecorder()}),
	)
	require.NoError(t, err)
	err = conflictApp.ComposeAndInstall(
		context.Background(),
		func(Runtime) (*BusinessBundle, error) {
			return &BusinessBundle{
				RawHTTP: []RawHTTPBinding{
					{
						Method:  http.MethodGet,
						Path:    "/duplicate",
						Handler: func(http.ResponseWriter, *http.Request) {},
					},
					{
						Desc: &server.RestRawHandlerDesc{
							Method:  http.MethodGet,
							Path:    "/duplicate",
							Handler: func(http.ResponseWriter, *http.Request) {},
						},
					},
				},
			}, nil
		},
	)
	require.ErrorContains(t, err, "already installed")
	require.ErrorIs(t, conflictApp.Start(context.Background()), errRestartUnsupported)

	badApp, err := Open(
		WithConfigManager(newTestManager(t, assemblyTestConfig(false))),
		WithAppName("compose-bad-handler"),
		WithModules(testTransportModule{recorder: newTransportRecorder()}),
	)
	require.NoError(t, err)
	err = badApp.ComposeAndInstall(context.Background(), func(Runtime) (*BusinessBundle, error) {
		return &BusinessBundle{
			RPCBindings: []RPCBinding{
				{
					ServiceName: testAssemblyServiceName,
					Desc:        &testAssemblyRPCServiceDesc,
					Impl:        struct{}{},
				},
			},
		}, nil
	})
	require.ErrorContains(t, err, "handler does not satisfy interface")
	require.ErrorIs(t, badApp.Start(context.Background()), errRestartUnsupported)
}

func TestStartWaitStopWithPreparedBusinessBundle(t *testing.T) {
	recorder := newTransportRecorder()
	manager := newTestManager(t, assemblyTestConfig(false))

	app, err := Open(
		WithConfigManager(manager),
		WithAppName("start-wait-stop"),
		WithModules(testTransportModule{recorder: recorder}),
	)
	require.NoError(t, err)
	require.NoError(
		t,
		app.ComposeAndInstall(context.Background(), func(Runtime) (*BusinessBundle, error) {
			return &BusinessBundle{
				RPCBindings: []RPCBinding{
					{
						ServiceName: testAssemblyServiceName,
						Desc:        &testAssemblyRPCServiceDesc,
						Impl:        &testAssemblyServiceImpl{},
					},
				},
			}, nil
		}),
	)

	startDone := make(chan error, 1)
	go func() {
		startDone <- app.Start(context.Background())
	}()
	select {
	case err := <-startDone:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("start did not return quickly")
	}
	waitForChannel(t, recorder.started, 2*time.Second, "server did not start")

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- app.Wait()
	}()

	select {
	case err := <-waitDone:
		t.Fatalf("wait returned too early: %v", err)
	case <-time.After(200 * time.Millisecond):
	}

	require.NoError(t, app.Stop(context.Background()))
	select {
	case err := <-waitDone:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("wait did not return after stop")
	}
	require.Equal(t, int32(1), atomic.LoadInt32(&recorder.startCalls))
	require.Equal(t, int32(1), atomic.LoadInt32(&recorder.handleCalls))
	require.Equal(t, int32(1), atomic.LoadInt32(&recorder.stopCalls))
}

func TestRuntimeLookupSupportsTypedTargets(t *testing.T) {
	manager := newTestManager(t, assemblyTestConfig(false))
	app, err := Open(
		WithConfigManager(manager),
		WithAppName("runtime-lookup"),
		WithModules(testTransportModule{recorder: newTransportRecorder()}),
	)
	require.NoError(t, err)
	require.NoError(
		t,
		app.ComposeAndInstall(context.Background(), func(Runtime) (*BusinessBundle, error) {
			return &BusinessBundle{
				Diagnostics: []BundleDiag{
					{
						Code:    string(yassembly.ErrComposeLocalResourceLeaked),
						Message: "local resource left outside managed bundle scope",
					},
				},
			}, nil
		}),
	)

	var catalog settings.Catalog
	require.NoError(t, app.Runtime().Lookup(&catalog))
	gotRoot, err := catalog.Root().Current()
	require.NoError(t, err)
	require.True(t, reflect.DeepEqual(gotRoot.Yggdrasil.Server.Transports, []string{"test"}))
}

func TestRuntimeUnavailableBeforePrepareAndAfterStop(t *testing.T) {
	app, err := Open(
		WithConfigManager(newTestManager(t, assemblyTestConfig(false))),
		WithAppName("runtime-state"),
		WithModules(testTransportModule{recorder: newTransportRecorder()}),
	)
	require.NoError(t, err)

	require.Nil(t, app.Runtime())
	require.NoError(t, app.Prepare(context.Background()))
	require.NotNil(t, app.Runtime())
	require.NoError(t, app.Stop(context.Background()))
	require.Nil(t, app.Runtime())
}

func TestPrepareFailureCleansUpSynchronously(t *testing.T) {
	var closeCount int32
	source := &mockConfigSource{
		name: "invalid-mode",
		data: map[string]any{
			"yggdrasil": map[string]any{
				"mode": "unknown-mode",
			},
		},
		closeCount: &closeCount,
	}
	app, err := Open(
		WithConfigManager(config.NewManager()),
		WithAppName("prepare-cleanup"),
		WithConfigSource("invalid-mode", config.PriorityOverride, source),
	)
	require.NoError(t, err)

	err = app.Prepare(context.Background())
	requireAssemblyErrorCode(t, err, yassembly.ErrInvalidMode)
	require.Equal(t, int32(1), atomic.LoadInt32(&closeCount))
	require.Equal(t, lifecycleStateStopped, app.state)
}

func TestComposeFailureCleansUpBeforeReturn(t *testing.T) {
	var closeCount int32
	source := &mockConfigSource{
		name:       "compose-source",
		data:       map[string]any{"yggdrasil": map[string]any{}},
		closeCount: &closeCount,
	}
	app, err := Open(
		WithConfigManager(config.NewManager()),
		WithAppName("compose-cleanup"),
		WithConfigSource("compose-source", config.PriorityOverride, source),
	)
	require.NoError(t, err)

	composeErr := errors.New("compose failed")
	_, err = app.Compose(context.Background(), func(Runtime) (*BusinessBundle, error) {
		return nil, composeErr
	})
	require.ErrorIs(t, err, composeErr)
	require.Equal(t, int32(1), atomic.LoadInt32(&closeCount))
	require.ErrorIs(t, app.Start(context.Background()), errRestartUnsupported)
}

func TestInstallBusinessPartialFailureCleansUpBeforeReturn(t *testing.T) {
	var closeCount int32
	source := &mockConfigSource{
		name:       "install-source",
		data:       map[string]any{"yggdrasil": map[string]any{}},
		closeCount: &closeCount,
	}
	app, err := Open(
		WithConfigManager(config.NewManager()),
		WithAppName("install-cleanup"),
		WithConfigSource("install-source", config.PriorityOverride, source),
	)
	require.NoError(t, err)
	require.NoError(t, app.Prepare(context.Background()))

	err = app.InstallBusiness(&BusinessBundle{
		Hooks: []BusinessHook{
			{Stage: BusinessHookBeforeStart, Func: func(context.Context) error { return nil }},
		},
		Tasks: []BackgroundTask{nil},
	})
	requireAssemblyErrorCode(t, err, yassembly.ErrInstallValidationFailed)
	require.Equal(t, int32(1), atomic.LoadInt32(&closeCount))
	require.ErrorIs(t, app.Start(context.Background()), errRestartUnsupported)
}
