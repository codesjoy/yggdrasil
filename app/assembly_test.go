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
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internalinstall "github.com/codesjoy/yggdrasil/v3/app/internal/install"
	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source/memory"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/module"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/server"
)

func TestOpenPrepareExposesRuntimeWithoutServing(t *testing.T) {
	recorder := newTransportRecorder()
	manager := newTestManager(t, assemblyTestConfig(false))

	app, err := New("prepare-runtime",
		WithConfigManager(manager), WithModules(testTransportModule{recorder: recorder}),
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

	app, err := New(
		"compose-install",
		WithConfigManager(
			manager,
		),
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

	conflictApp, err := New(
		"compose-conflict",
		WithConfigManager(
			newTestManager(t, assemblyTestConfig(true)),
		),
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

	badApp, err := New(
		"compose-bad-handler",
		WithConfigManager(
			newTestManager(t, assemblyTestConfig(false)),
		),
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

	app, err := New("start-wait-stop",
		WithConfigManager(manager), WithModules(testTransportModule{recorder: recorder}),
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

func TestStartBlocksUntilModulesAreRunning(t *testing.T) {
	recorder := newTransportRecorder()
	manager := newTestManager(t, assemblyTestConfig(false))

	app, err := New(
		"start-synchronous",
		WithConfigManager(manager),
		WithModules(testTransportModule{recorder: recorder}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	require.NoError(t, app.Start(context.Background()))
	require.Equal(t, int32(1), atomic.LoadInt32(&recorder.startCalls))
	require.Equal(t, int32(1), atomic.LoadInt32(&recorder.handleCalls))
}

func TestRuntimeLookupSupportsTypedTargets(t *testing.T) {
	manager := newTestManager(t, assemblyTestConfig(false))
	app, err := New(
		"runtime-lookup",
		WithConfigManager(
			manager,
		),
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
	app, err := New(
		"runtime-state",
		WithConfigManager(
			newTestManager(t, assemblyTestConfig(false)),
		),
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
	app, err := New(
		"prepare-cleanup",
		WithConfigManager(
			config.NewManager(),
		),
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
	app, err := New(
		"compose-cleanup",
		WithConfigManager(
			config.NewManager(),
		),
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
	app, err := New(
		"install-cleanup",
		WithConfigManager(
			config.NewManager(),
		),
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

type stagedInstallExtension struct {
	kind      string
	install   func(*InstallContext) error
	rollback  func(*InstallContext) error
	rollbacks *[]string
}

type noOpBackgroundTask struct{}

func (*noOpBackgroundTask) Serve() error               { return nil }
func (*noOpBackgroundTask) Stop(context.Context) error { return nil }

func (e stagedInstallExtension) Kind() string {
	return e.kind
}

func (e stagedInstallExtension) Install(ctx *InstallContext) error {
	if e.install == nil {
		return nil
	}
	return e.install(ctx)
}

func (e stagedInstallExtension) RollbackInstall(ctx *InstallContext) error {
	if e.rollbacks != nil {
		*e.rollbacks = append(*e.rollbacks, e.kind)
	}
	if e.rollback == nil {
		return nil
	}
	return e.rollback(ctx)
}

func TestInstallBusinessStagesUntilExtensionsAndRollsBack(t *testing.T) {
	recorder := newTransportRecorder()
	app, err := New(
		"staged-install-rollback",
		WithConfigManager(newTestManager(t, assemblyTestConfig(false))),
		WithModules(testTransportModule{recorder: recorder}),
	)
	require.NoError(t, err)
	require.NoError(t, app.Prepare(context.Background()))

	rollbacks := []string{}
	installErr := errors.New("extension install failed")
	err = app.InstallBusiness(&BusinessBundle{
		RPCBindings: []RPCBinding{{
			ServiceName: testAssemblyServiceName,
			Desc:        &testAssemblyRPCServiceDesc,
			Impl:        &testAssemblyServiceImpl{},
		}},
		Tasks: []BackgroundTask{&mockInternalServer{}},
		Hooks: []BusinessHook{{
			Stage: BusinessHookBeforeStart,
			Func:  func(context.Context) error { return nil },
		}},
		Extensions: []BusinessInstallable{
			stagedInstallExtension{
				kind: "first",
				install: func(ctx *InstallContext) error {
					return ctx.AddTask(&mockInternalServer{})
				},
				rollbacks: &rollbacks,
			},
			stagedInstallExtension{
				kind: "second",
				install: func(ctx *InstallContext) error {
					if err := ctx.AddHook(BusinessHook{
						Stage: BusinessHookAfterStop,
						Func:  func(context.Context) error { return nil },
					}); err != nil {
						return err
					}
					return installErr
				},
				rollbacks: &rollbacks,
			},
		},
	})
	require.ErrorIs(t, err, installErr)
	require.Equal(t, []string{"second", "first"}, rollbacks)
	require.Empty(t, app.installedRPCServices)
	require.Zero(t, len(app.opts.internalServers))
	require.Zero(t, len(app.opts.beforeStartHooks))
	require.Zero(t, len(app.opts.afterStopHooks))
}

func TestInstallBusinessCommitsStagedExtensionsAtomically(t *testing.T) {
	app, err := New(
		"staged-install-commit",
		WithConfigManager(newTestManager(t, assemblyTestConfig(false))),
		WithModules(testTransportModule{recorder: newTransportRecorder()}),
	)
	require.NoError(t, err)
	require.NoError(t, app.Prepare(context.Background()))
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	err = app.InstallBusiness(&BusinessBundle{
		Extensions: []BusinessInstallable{
			stagedInstallExtension{
				kind: "task",
				install: func(ctx *InstallContext) error {
					return ctx.AddTask(&noOpBackgroundTask{})
				},
			},
			stagedInstallExtension{
				kind: "rpc",
				install: func(ctx *InstallContext) error {
					return ctx.RegisterRPC(RPCBinding{
						ServiceName: testAssemblyServiceName,
						Desc:        &testAssemblyRPCServiceDesc,
						Impl:        &testAssemblyServiceImpl{},
					})
				},
			},
		},
	})
	require.NoError(t, err)
	require.Contains(t, app.installedRPCServices, testAssemblyServiceName)
	require.Len(t, app.opts.internalServers, 1)
	require.True(t, app.explicitBundleInstalled)
}

func TestInstallBusinessExtensionCanUseRuntimeWithoutAppLock(t *testing.T) {
	app, err := New(
		"staged-install-runtime",
		WithConfigManager(newTestManager(t, assemblyTestConfig(false))),
		WithModules(testTransportModule{recorder: newTransportRecorder()}),
	)
	require.NoError(t, err)
	require.NoError(t, app.Prepare(context.Background()))
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	done := make(chan error, 1)
	go func() {
		done <- app.InstallBusiness(&BusinessBundle{
			Extensions: []BusinessInstallable{
				stagedInstallExtension{
					kind: "runtime-client",
					install: func(ctx *InstallContext) error {
						_, _ = ctx.Runtime.NewClient(context.Background(), "missing-service")
						return nil
					},
				},
			},
		})
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("InstallBusiness deadlocked while an extension used Runtime")
	}
}

func TestInstallBusinessRejectsConcurrentInstallation(t *testing.T) {
	app, err := New(
		"staged-install-concurrent",
		WithConfigManager(newTestManager(t, assemblyTestConfig(false))),
		WithModules(testTransportModule{recorder: newTransportRecorder()}),
	)
	require.NoError(t, err)
	require.NoError(t, app.Prepare(context.Background()))
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	entered := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- app.InstallBusiness(&BusinessBundle{
			Extensions: []BusinessInstallable{
				stagedInstallExtension{
					kind: "blocking",
					install: func(*InstallContext) error {
						close(entered)
						<-release
						return nil
					},
				},
			},
		})
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("first installation did not enter the extension callback")
	}

	err = app.InstallBusiness(&BusinessBundle{})
	requireAssemblyErrorCode(t, err, yassembly.ErrInstallRegistrationConflict)
	close(release)
	require.NoError(t, <-firstDone)
}

func TestStopWaitsForInFlightBusinessInstallation(t *testing.T) {
	app, err := New(
		"staged-install-stop",
		WithConfigManager(newTestManager(t, assemblyTestConfig(false))),
		WithModules(testTransportModule{recorder: newTransportRecorder()}),
	)
	require.NoError(t, err)
	require.NoError(t, app.Prepare(context.Background()))

	entered := make(chan struct{})
	release := make(chan struct{})
	installDone := make(chan error, 1)
	go func() {
		installDone <- app.InstallBusiness(&BusinessBundle{
			Extensions: []BusinessInstallable{
				stagedInstallExtension{
					kind: "blocking-stop",
					install: func(*InstallContext) error {
						close(entered)
						<-release
						return nil
					},
				},
			},
		})
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("installation did not enter the extension callback")
	}

	stopDone := make(chan error, 1)
	go func() {
		stopDone <- app.Stop(context.Background())
	}()
	select {
	case err := <-stopDone:
		t.Fatalf("Stop returned before installation completed: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	require.NoError(t, <-installDone)
	require.NoError(t, <-stopDone)
}

// --- effectiveResolved ---

func TestEffectiveResolved(t *testing.T) {
	t.Run("nil result uses fallback", func(t *testing.T) {
		fallback := settings.Resolved{Mode: "fallback"}
		result := effectiveResolved(nil, fallback)
		assert.Equal(t, "fallback", result.Mode)
	})

	t.Run("non-nil result uses resolved", func(t *testing.T) {
		fallback := settings.Resolved{Mode: "fallback"}
		planResult := &yassembly.Result{
			EffectiveResolved: settings.Resolved{Mode: "planned"},
		}
		result := effectiveResolved(planResult, fallback)
		assert.Equal(t, "planned", result.Mode)
	})
}

// --- selectedCapabilityBindings ---

func TestSelectedCapabilityBindings(t *testing.T) {
	t.Run("nil result uses fallback", func(t *testing.T) {
		fallback := settings.Resolved{
			CapabilityBindings: map[string][]string{"k": {"v"}},
		}
		result := selectedCapabilityBindings(nil, fallback)
		assert.Equal(t, map[string][]string{"k": {"v"}}, result)
	})

	t.Run("non-nil uses result bindings", func(t *testing.T) {
		planResult := &yassembly.Result{
			CapabilityBindings: map[string][]string{"a": {"b"}},
		}
		result := selectedCapabilityBindings(planResult, settings.Resolved{})
		assert.Equal(t, map[string][]string{"a": {"b"}}, result)
	})
}

// --- cloneCapabilityBindings ---

func TestCloneCapabilityBindings(t *testing.T) {
	t.Run("nil map returns empty", func(t *testing.T) {
		result := cloneCapabilityBindings(nil)
		assert.Equal(t, map[string][]string{}, result)
	})

	t.Run("normal map cloned", func(t *testing.T) {
		original := map[string][]string{"k": {"a", "b"}}
		cloned := cloneCapabilityBindings(original)
		assert.Equal(t, original, cloned)
	})

	t.Run("mutation safety", func(t *testing.T) {
		original := map[string][]string{"k": {"a", "b"}}
		cloned := cloneCapabilityBindings(original)
		cloned["k"][0] = "modified"
		assert.Equal(t, "a", original["k"][0])
	})
}

// --- preparedAssembly.Close ---

func TestPreparedAssembly_Close(t *testing.T) {
	t.Run("nil receiver returns nil", func(t *testing.T) {
		var pa *preparedAssembly
		assert.Nil(
			t,
			pa.Close(nil), //nolint:staticcheck // intentional: testing nil context handling
		)
	})

	t.Run("nil CloseFunc returns nil", func(t *testing.T) {
		pa := &preparedAssembly{}
		assert.Nil(
			t,
			pa.Close(nil), //nolint:staticcheck // intentional: testing nil context handling
		)
	})

	t.Run("calls CloseFunc", func(t *testing.T) {
		called := false
		pa := &preparedAssembly{
			CloseFunc: func(_ context.Context) error {
				called = true
				return nil
			},
		}
		assert.Nil(
			t,
			pa.Close(nil), //nolint:staticcheck // intentional: testing nil context handling
		)
		assert.True(t, called)
	})
}

// --- diagnostics ---

func TestStatsOtelAutoSpecEnablesRealModule(t *testing.T) {
	manager := newTestManager(t, map[string]any{
		"yggdrasil": map[string]any{
			"admin": map[string]any{
				"governor": map[string]any{"port": 0},
			},
			"observability": map[string]any{
				"telemetry": map[string]any{
					"stats": map[string]any{
						"server": "otel",
					},
				},
			},
		},
	})
	app, err := New("stats-auto",
		WithConfigManager(manager))
	require.NoError(t, err)
	require.NoError(t, app.Prepare(context.Background()))
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	require.Contains(
		t,
		app.lastPlanResult.CapabilityBindings["observability.stats.handler"],
		"otel",
	)
	require.Contains(
		t,
		app.lastPlanResult.AffectedPathsByDomain["modules"],
		"yggdrasil.observability.telemetry.stats.server",
	)
	require.True(t, containsPlannedModule(app.assemblySpec, "observability.stats.otel"))
}

func TestDiagnosticsEndpointIncludesAssemblyState(t *testing.T) {
	recorder := newTransportRecorder()
	manager := newTestManager(t, map[string]any{
		"yggdrasil": map[string]any{
			"mode": "prod-http-gateway",
			"admin": map[string]any{
				"governor": map[string]any{"port": 0},
			},
			"server": map[string]any{
				"transports": []any{"test"},
			},
			"transports": map[string]any{
				"http": map[string]any{
					"rest": map[string]any{
						"host": "127.0.0.1",
						"port": 0,
					},
				},
			},
		},
	})
	app, err := New("diagnostics-assembly",
		WithConfigManager(manager), WithModules(testTransportModule{recorder: recorder}),
		WithPlanOverrides(yassembly.ForceDefault("observability.logger.handler", "text")),
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
	t.Cleanup(func() {
		_ = app.opts.governor.Stop()
		_ = app.Stop(context.Background())
	})

	errCh := serveGovernorAsync(t, app.opts.governor)
	waitGovernorStarted(t, app.opts.governor)

	resp, err := http.Get("http://" + app.opts.governor.Info().Address + "/diagnostics")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var doc struct {
		Assembly struct {
			Spec struct {
				Mode struct {
					Name string `json:"name"`
				} `json:"mode"`
			} `json:"spec"`
			LastStableSpec struct {
				Mode struct {
					Name string `json:"name"`
				} `json:"mode"`
			} `json:"last_stable_spec"`
			CurrentSpecHash    string `json:"current_spec_hash"`
			LastStableSpecHash string `json:"last_stable_spec_hash"`
			Mode               struct {
				Name string `json:"name"`
			} `json:"mode"`
			SelectedDefaults map[string]struct {
				Value  string `json:"value"`
				Source string `json:"source"`
			} `json:"selected_defaults"`
			DefaultCandidates map[string][]struct {
				Provider string `json:"provider"`
			} `json:"default_candidates"`
			BusinessInputPaths []string `json:"business_input_paths"`
			BundleDiagnostics  []struct {
				Code string `json:"code"`
			} `json:"bundle_diagnostics"`
		} `json:"assembly"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&doc))
	require.Equal(t, "prod-http-gateway", doc.Assembly.Mode.Name)
	require.NotEmpty(t, doc.Assembly.CurrentSpecHash)
	require.Equal(t, doc.Assembly.CurrentSpecHash, doc.Assembly.LastStableSpecHash)
	require.Equal(t, doc.Assembly.Spec.Mode.Name, doc.Assembly.LastStableSpec.Mode.Name)
	require.Equal(t, "text", doc.Assembly.SelectedDefaults["observability.logger.handler"].Value)
	require.Equal(
		t,
		"code_override",
		doc.Assembly.SelectedDefaults["observability.logger.handler"].Source,
	)
	require.NotEmpty(t, doc.Assembly.BusinessInputPaths)
	require.Equal(
		t,
		string(yassembly.ErrComposeLocalResourceLeaked),
		doc.Assembly.BundleDiagnostics[0].Code,
	)

	require.NoError(t, app.opts.governor.Stop())
	requireAsyncNoError(t, errCh, "governor serve goroutine did not exit")
}

// --- reload ---

func TestReloadWithInstalledBusinessMarksRestartRequired(t *testing.T) {
	recorder := newTransportRecorder()
	manager := newTestManager(t, assemblyTestConfig(false))

	app, err := New("reload-installed-business",
		WithConfigManager(manager), WithModules(testTransportModule{recorder: recorder}),
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
	require.NoError(t, app.Start(context.Background()))
	waitForChannel(t, recorder.started, 2*time.Second, "reload server did not start")
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	require.NoError(
		t,
		manager.LoadLayer(
			"override",
			config.PriorityOverride,
			memory.NewSource("override", map[string]any{
				"yggdrasil": map[string]any{
					"observability": map[string]any{
						"logging": map[string]any{
							"remote_level": "warn",
						},
					},
				},
			}),
		),
	)
	err = app.Reload(context.Background())
	requireAssemblyErrorCode(t, err, yassembly.ErrReloadRequiresRestart)
	require.True(t, app.hub.ReloadState().RestartRequired)

	app.mu.Lock()
	manager = app.opts.configManager
	app.opts.configManager = nil
	app.mu.Unlock()
	err = app.Reload(context.Background())
	requireAssemblyErrorCode(t, err, yassembly.ErrReloadRequiresRestart)
	app.mu.Lock()
	app.opts.configManager = manager
	app.mu.Unlock()

	require.NoError(
		t,
		manager.LoadLayer(
			"override",
			config.PriorityOverride,
			memory.NewSource("override", map[string]any{
				"yggdrasil": map[string]any{
					"observability": map[string]any{
						"logging": map[string]any{
							"remote_level": "error",
						},
					},
				},
			}),
		),
	)
	err = app.Reload(context.Background())
	requireAssemblyErrorCode(t, err, yassembly.ErrReloadRequiresRestart)
}

func TestReloadRuntimeOnlyChangeHotReloadsWithoutBusinessBundle(t *testing.T) {
	recorder := newTransportRecorder()
	manager := newTestManager(t, assemblyTestConfig(false))

	app, err := New("reload-runtime-only",
		WithConfigManager(manager), WithModules(testTransportModule{recorder: recorder}),
	)
	require.NoError(t, err)
	require.NoError(t, app.Start(context.Background()))
	waitForChannel(t, recorder.started, 2*time.Second, "runtime-only reload server did not start")
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	require.NoError(
		t,
		manager.LoadLayer(
			"override",
			config.PriorityOverride,
			memory.NewSource("override", map[string]any{
				"yggdrasil": map[string]any{
					"observability": map[string]any{
						"logging": map[string]any{
							"remote_level": "warn",
						},
					},
				},
			}),
		),
	)
	require.NoError(t, app.Reload(context.Background()))
	require.Nil(t, app.assemblyErrors.Reload.Err)
	require.Equal(t, module.ReloadPhaseIdle, app.hub.ReloadState().Phase)
	require.Equal(
		t,
		"warn",
		app.currentRuntimeSnapshot().Resolved.Logging.RemoteLevel,
	)
	diag := app.assemblyDiagnostics()
	require.Same(t, app.lastPlanResult.Spec, diag.Spec)
	require.Equal(t, app.lastPlanResult.Hash, diag.CurrentSpecHash)
	require.Same(t, app.assemblySpec, diag.LastStableSpec)
	require.Equal(t, app.lastStablePlanHash, diag.LastStableSpecHash)
}

type failingAfterFoundationModule struct{}

func (failingAfterFoundationModule) Name() string {
	return "test.fail.after-foundation"
}

func (failingAfterFoundationModule) DependsOn() []string {
	return []string{"connectivity.runtime"}
}

func (failingAfterFoundationModule) ConfigPath() string {
	return "test.failure"
}

func (failingAfterFoundationModule) PrepareReload(
	context.Context,
	config.View,
) (module.ReloadCommitter, error) {
	return failingAfterFoundationCommitter{failingAfterFoundationModule{}}, nil
}

type failingAfterFoundationCommitter struct {
	failingAfterFoundationModule
}

func (failingAfterFoundationCommitter) Commit(context.Context) error {
	return errors.New("later module commit failed")
}

func (failingAfterFoundationCommitter) Rollback(context.Context) error {
	return nil
}

func TestReloadRestoresFoundationSnapshotWhenLaterModuleCommitFails(t *testing.T) {
	configData := assemblyTestConfig(false)
	configData["test"] = map[string]any{
		"failure": map[string]any{"value": "initial"},
	}
	mod := failingAfterFoundationModule{}
	manager := newTestManager(t, configData)
	app, err := New(
		"foundation-rollback",
		WithConfigManager(manager),
		WithModules(testTransportModule{recorder: newTransportRecorder()}, mod),
		WithPlanOverrides(yassembly.EnableModule(mod.Name())),
	)
	require.NoError(t, err)
	require.NoError(t, app.Prepare(context.Background()))
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	app.mu.Lock()
	app.state = lifecycleStateRunning
	previousFoundation := app.foundationSnapshot
	app.mu.Unlock()
	require.NotNil(t, previousFoundation)

	require.NoError(t, manager.LoadLayer(
		"rollback",
		config.PriorityOverride,
		memory.NewSource("rollback", map[string]any{
			"yggdrasil": map[string]any{
				"observability": map[string]any{
					"logging": map[string]any{"remote_level": "warn"},
				},
			},
			"test": map[string]any{
				"failure": map[string]any{"value": "next"},
			},
		}),
	))
	err = app.Reload(context.Background())
	var assemblyErr *yassembly.Error
	require.ErrorAs(t, err, &assemblyErr)
	require.Equal(t, yassembly.ErrRuntimeReconcileFailed, assemblyErr.Code)
	require.ErrorContains(t, assemblyErr.Cause, "later module commit failed")
	require.Same(t, previousFoundation, app.foundationSnapshot)
	require.True(t, app.hub.ReloadState().RestartRequired)
	diag := app.assemblyDiagnostics()
	require.Same(t, app.lastPlanResult.Spec, diag.Spec)
	require.Equal(t, app.lastPlanResult.Hash, diag.CurrentSpecHash)
	require.Same(t, app.assemblySpec, diag.LastStableSpec)
	require.Equal(t, app.lastStablePlanHash, diag.LastStableSpecHash)
}

type nonReloadableAppTestModule struct{}

func (nonReloadableAppTestModule) Name() string { return "test.nonreloadable" }

func (nonReloadableAppTestModule) ConfigPath() string { return "test.nonreloadable" }

func TestReloadWithNonReloadableModuleDoesNotApplyRuntimeConfig(t *testing.T) {
	configData := minimalV3Config("grpc")
	configData["yggdrasil"].(map[string]any)["observability"] = map[string]any{
		"logging": map[string]any{"remote_level": "info"},
	}
	configData["test"] = map[string]any{
		"nonreloadable": map[string]any{"value": "initial"},
	}
	mod := nonReloadableAppTestModule{}
	manager := newTestManager(t, configData)
	app, err := New(
		"nonreloadable-atomic-reload",
		WithConfigManager(manager),
		WithModules(mod),
		WithPlanOverrides(yassembly.EnableModule(mod.Name())),
	)
	require.NoError(t, err)
	require.NoError(t, app.Prepare(context.Background()))
	t.Cleanup(func() { _ = app.Stop(context.Background()) })
	app.mu.Lock()
	app.state = lifecycleStateRunning
	app.mu.Unlock()

	previousSnapshot := app.currentRuntimeSnapshot()
	previousPlan := app.runtimePlan()
	require.Equal(t, "info", previousSnapshot.Resolved.Logging.RemoteLevel)

	override := map[string]any{
		"yggdrasil": map[string]any{
			"observability": map[string]any{
				"logging": map[string]any{"remote_level": "debug"},
			},
		},
		"test": map[string]any{
			"nonreloadable": map[string]any{"value": "changed"},
		},
	}
	require.NoError(t, manager.LoadLayer(
		"nonreloadable",
		config.PriorityOverride,
		memory.NewSource("nonreloadable", override),
	))
	requireAssemblyErrorCode(
		t,
		app.Reload(context.Background()),
		yassembly.ErrReloadRequiresRestart,
	)

	require.Same(t, previousSnapshot, app.currentRuntimeSnapshot())
	require.Equal(t, "info", app.currentRuntimeSnapshot().Resolved.Logging.RemoteLevel)
	require.Same(t, previousPlan, app.runtimePlan())
	require.True(t, app.hub.ReloadState().RestartRequired)
}

func TestReloadRestoresCapabilityBindingsWhenHubCommitFails(t *testing.T) {
	configData := minimalV3Config("grpc")
	configData["test"] = map[string]any{
		"failure": map[string]any{"value": "initial"},
	}
	mod := failingAfterFoundationModule{}
	manager := newTestManager(t, configData)
	app, err := New(
		"binding-rollback",
		WithConfigManager(manager),
		WithModules(mod),
		WithPlanOverrides(yassembly.EnableModule(mod.Name())),
	)
	require.NoError(t, err)
	require.NoError(t, app.Prepare(context.Background()))
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	app.mu.Lock()
	app.state = lifecycleStateRunning
	app.mu.Unlock()

	override := minimalV3Config("http")
	override["test"] = map[string]any{
		"failure": map[string]any{"value": "next"},
	}
	require.NoError(t, manager.LoadLayer(
		"binding-rollback",
		config.PriorityOverride,
		memory.NewSource("binding-rollback", override),
	))
	require.Error(t, app.Reload(context.Background()))

	binding := findBindingDiag(t, app.hub.Diagnostics(), "transport.server.provider")
	require.Equal(t, []string{"grpc"}, binding.Requested)
	require.Equal(t, []string{"grpc"}, binding.Resolved)
}

func TestReloadUpdatesPlanHashesAndDiffDiagnostics(t *testing.T) {
	recorder := newTransportRecorder()
	manager := newTestManager(t, map[string]any{
		"yggdrasil": map[string]any{
			"mode": "dev",
			"admin": map[string]any{
				"governor": map[string]any{"port": 0},
			},
			"server": map[string]any{
				"transports": []any{"test"},
			},
		},
	})
	app, err := New("reload-plan-diff",
		WithConfigManager(manager), WithModules(testTransportModule{recorder: recorder}),
	)
	require.NoError(t, err)
	require.NoError(t, app.Start(context.Background()))
	waitForChannel(t, recorder.started, 2*time.Second, "reload plan server did not start")
	initialHash := app.lastPlanHash
	require.NotEmpty(t, initialHash)
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	require.NoError(
		t,
		manager.LoadLayer("mode", config.PriorityOverride, memory.NewSource("mode", map[string]any{
			"yggdrasil": map[string]any{
				"mode": "prod-grpc",
			},
		})),
	)
	err = app.Reload(context.Background())
	requireAssemblyErrorCode(t, err, yassembly.ErrReloadRequiresRestart)
	require.True(t, app.hub.ReloadState().RestartRequired)
	require.NotNil(t, app.lastSpecDiff)
	require.True(t, app.lastSpecDiff.HasChanges)
	require.Contains(t, app.lastSpecDiff.AffectedDomains, "mode")
	require.NotEqual(t, initialHash, app.lastPlanHash)
	require.Equal(t, initialHash, app.lastStablePlanHash)
	diag := app.assemblyDiagnostics()
	require.Same(t, app.lastPlanResult.Spec, diag.Spec)
	require.Equal(t, app.lastPlanResult.Hash, diag.CurrentSpecHash)
	require.Same(t, app.assemblySpec, diag.LastStableSpec)
	require.Equal(t, initialHash, diag.LastStableSpecHash)
}
