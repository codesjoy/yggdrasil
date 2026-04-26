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

// --- effectiveResolved ---

func TestEffectiveResolved(t *testing.T) {
	t.Run("nil result uses fallback", func(t *testing.T) {
		fallback := settings.Resolved{App: settings.Application{Name: "fallback"}}
		result := effectiveResolved(nil, fallback)
		assert.Equal(t, "fallback", result.App.Name)
	})

	t.Run("non-nil result uses resolved", func(t *testing.T) {
		fallback := settings.Resolved{App: settings.Application{Name: "fallback"}}
		planResult := &yassembly.Result{
			EffectiveResolved: settings.Resolved{App: settings.Application{Name: "planned"}},
		}
		result := effectiveResolved(planResult, fallback)
		assert.Equal(t, "planned", result.App.Name)
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
	app, err := Open(
		WithConfigManager(manager),
		WithAppName("stats-auto"),
	)
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
	app, err := Open(
		WithConfigManager(manager),
		WithAppName("diagnostics-assembly"),
		WithModules(testTransportModule{recorder: recorder}),
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
			CurrentSpecHash string `json:"current_spec_hash"`
			Mode            struct {
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

	app, err := Open(
		WithConfigManager(manager),
		WithAppName("reload-installed-business"),
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
}

func TestReloadRuntimeOnlyChangeHotReloadsWithoutBusinessBundle(t *testing.T) {
	recorder := newTransportRecorder()
	manager := newTestManager(t, assemblyTestConfig(false))

	app, err := Open(
		WithConfigManager(manager),
		WithAppName("reload-runtime-only"),
		WithModules(testTransportModule{recorder: recorder}),
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
	app, err := Open(
		WithConfigManager(manager),
		WithAppName("reload-plan-diff"),
		WithModules(testTransportModule{recorder: recorder}),
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
}
