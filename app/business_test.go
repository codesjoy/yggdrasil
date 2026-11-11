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
	"log/slog"
	"net/http"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
)

// --- mock bundleInstaller ---

type mockBundleInstaller struct {
	rpcBindings  []RPCBinding
	restBindings []RESTBinding
	rawHTTP      []RawHTTPBinding
	governorHTTP []GovernorHTTPBinding
	tasks        []BackgroundTask
	hooks        []BusinessHook
}

func (m *mockBundleInstaller) installRPCBinding(b RPCBinding) error {
	m.rpcBindings = append(m.rpcBindings, b)
	return nil
}

func (m *mockBundleInstaller) installRESTBinding(b RESTBinding) error {
	m.restBindings = append(m.restBindings, b)
	return nil
}

func (m *mockBundleInstaller) installRawHTTPBinding(b RawHTTPBinding) error {
	m.rawHTTP = append(m.rawHTTP, b)
	return nil
}

func (m *mockBundleInstaller) installGovernorHTTPBinding(b GovernorHTTPBinding) error {
	m.governorHTTP = append(m.governorHTTP, b)
	return nil
}

func (m *mockBundleInstaller) addBackgroundTask(task BackgroundTask) error {
	m.tasks = append(m.tasks, task)
	return nil
}

func (m *mockBundleInstaller) addBusinessHook(hook BusinessHook) error {
	m.hooks = append(m.hooks, hook)
	return nil
}

// --- InstallContext ---

func TestInstallContext_RegisterRPC(t *testing.T) {
	t.Run("valid binding", func(t *testing.T) {
		installer := &mockBundleInstaller{}
		ctx := &InstallContext{installer: installer}
		binding := RPCBinding{
			ServiceName: "test",
			Desc:        &testAssemblyRPCServiceDesc,
			Impl:        &testAssemblyServiceImpl{},
		}
		err := ctx.RegisterRPC(binding)
		require.NoError(t, err)
		assert.Len(t, installer.rpcBindings, 1)
	})

	t.Run("nil installer returns error", func(t *testing.T) {
		ctx := &InstallContext{}
		err := ctx.RegisterRPC(RPCBinding{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not ready")
	})

	t.Run("nil context returns error", func(t *testing.T) {
		var ctx *InstallContext
		err := ctx.RegisterRPC(RPCBinding{})
		require.Error(t, err)
	})
}

func TestInstallContext_RegisterREST(t *testing.T) {
	t.Run("valid binding", func(t *testing.T) {
		installer := &mockBundleInstaller{}
		ctx := &InstallContext{installer: installer}
		binding := RESTBinding{
			Name: "test",
			Desc: &testAssemblyRESTServiceDesc,
			Impl: &testAssemblyServiceImpl{},
		}
		err := ctx.RegisterREST(binding)
		require.NoError(t, err)
		assert.Len(t, installer.restBindings, 1)
	})

	t.Run("nil installer returns error", func(t *testing.T) {
		ctx := &InstallContext{}
		err := ctx.RegisterREST(RESTBinding{})
		require.Error(t, err)
	})
}

func TestInstallContext_RegisterRawHTTP(t *testing.T) {
	t.Run("valid binding", func(t *testing.T) {
		installer := &mockBundleInstaller{}
		ctx := &InstallContext{installer: installer}
		binding := RawHTTPBinding{
			Method:  "GET",
			Path:    "/test",
			Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		}
		err := ctx.RegisterRawHTTP(binding)
		require.NoError(t, err)
		assert.Len(t, installer.rawHTTP, 1)
	})

	t.Run("nil installer returns error", func(t *testing.T) {
		ctx := &InstallContext{}
		err := ctx.RegisterRawHTTP(RawHTTPBinding{})
		require.Error(t, err)
	})
}

func TestInstallContext_RegisterGovernorHTTP(t *testing.T) {
	t.Run("valid binding", func(t *testing.T) {
		installer := &mockBundleInstaller{}
		ctx := &InstallContext{installer: installer}
		binding := GovernorHTTPBinding{
			Method:  "GET",
			Path:    "/test",
			Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		}
		err := ctx.RegisterGovernorHTTP(binding)
		require.NoError(t, err)
		assert.Len(t, installer.governorHTTP, 1)
	})

	t.Run("nil installer returns error", func(t *testing.T) {
		ctx := &InstallContext{}
		err := ctx.RegisterGovernorHTTP(GovernorHTTPBinding{})
		require.Error(t, err)
	})
}

func TestInstallContext_AddTask(t *testing.T) {
	t.Run("valid task", func(t *testing.T) {
		installer := &mockBundleInstaller{}
		ctx := &InstallContext{installer: installer}
		task := &mockInternalServer{}
		err := ctx.AddTask(task)
		require.NoError(t, err)
		assert.Len(t, installer.tasks, 1)
	})

	t.Run("nil installer returns error", func(t *testing.T) {
		ctx := &InstallContext{}
		err := ctx.AddTask(&mockInternalServer{})
		require.Error(t, err)
	})
}

func TestInstallContext_AddHook(t *testing.T) {
	t.Run("valid hook", func(t *testing.T) {
		installer := &mockBundleInstaller{}
		ctx := &InstallContext{installer: installer}
		hook := BusinessHook{
			Name:  "test",
			Stage: BusinessHookBeforeStart,
			Func:  func(context.Context) error { return nil },
		}
		err := ctx.AddHook(hook)
		require.NoError(t, err)
		assert.Len(t, installer.hooks, 1)
	})

	t.Run("nil installer returns error", func(t *testing.T) {
		ctx := &InstallContext{}
		err := ctx.AddHook(BusinessHook{
			Stage: BusinessHookBeforeStart,
			Func:  func(context.Context) error { return nil },
		})
		require.Error(t, err)
	})
}

func TestInstallContext_InstallerOrError(t *testing.T) {
	t.Run("with installer", func(t *testing.T) {
		installer := &mockBundleInstaller{}
		ctx := &InstallContext{installer: installer}
		got, err := ctx.installerOrError()
		require.NoError(t, err)
		assert.Equal(t, installer, got)
	})

	t.Run("without installer returns error", func(t *testing.T) {
		ctx := &InstallContext{}
		_, err := ctx.installerOrError()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not ready")
	})

	t.Run("nil context returns error", func(t *testing.T) {
		var ctx *InstallContext
		_, err := ctx.installerOrError()
		require.Error(t, err)
	})
}

// --- runtimeSurface ---

func TestRuntimeSurface_NilReceiver(t *testing.T) {
	t.Run("NewClient returns error", func(t *testing.T) {
		var r *runtimeSurface
		_, err := r.NewClient(context.Background(), "svc")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not ready")
	})

	t.Run("Config returns nil", func(t *testing.T) {
		var r *runtimeSurface
		assert.Nil(t, r.Config())
	})

	t.Run("Lookup nil target error", func(t *testing.T) {
		r := &runtimeSurface{lookups: map[reflect.Type]any{}}
		err := r.Lookup(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil")
	})

	t.Run("Lookup non-pointer error", func(t *testing.T) {
		r := &runtimeSurface{lookups: map[reflect.Type]any{}}
		err := r.Lookup("string")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "non-nil pointer")
	})

	t.Run("Lookup valid type found", func(t *testing.T) {
		r := &runtimeSurface{lookups: map[reflect.Type]any{}}
		var logger *slog.Logger
		r.lookups[reflect.TypeOf(logger)] = slog.Default()
		var target *slog.Logger
		err := r.Lookup(&target)
		require.NoError(t, err)
		assert.NotNil(t, target)
	})

	t.Run("Lookup type not found", func(t *testing.T) {
		r := &runtimeSurface{lookups: map[reflect.Type]any{}}
		var target string
		err := r.Lookup(&target)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// --- App.installOptions ---

func TestApp_InstallOptions(t *testing.T) {
	t.Run("builds options from initialized app", func(t *testing.T) {
		data := minimalV3Config("grpc")
		app, _ := newInitializedAppWithConfig(t, "test-app", data)
		opts, err := app.installOptions()
		require.NoError(t, err)
		assert.NotNil(t, opts)
	})

	t.Run("nil app returns error", func(t *testing.T) {
		var a *App
		_, err := a.installOptions()
		require.Error(t, err)
	})
}

// --- App.installBundle ---

func TestApp_InstallBundleLocked(t *testing.T) {
	t.Run("nil bundle returns nil", func(t *testing.T) {
		data := minimalV3Config("grpc")
		app, _ := newInitializedAppWithConfig(t, "test-app", data)
		err := app.installBundle(nil)
		require.NoError(t, err)
	})

	t.Run("install bundle with RPC binding", func(t *testing.T) {
		data := minimalV3Config("grpc")
		data["yggdrasil"].(map[string]any)["transports"].(map[string]any)["http"].(map[string]any)["rest"] = map[string]any{
			"enabled": true,
		}
		app, _ := newInitializedAppWithConfig(t, "test-app", data)
		bundle := &BusinessBundle{
			RPCBindings: []RPCBinding{
				{
					ServiceName: testAssemblyServiceName,
					Desc:        &testAssemblyRPCServiceDesc,
					Impl:        &testAssemblyServiceImpl{},
				},
			},
		}
		err := app.installBundle(bundle)
		require.NoError(t, err)
	})
}

// --- App.addBusinessHook ---

func TestApp_AddBusinessHook(t *testing.T) {
	t.Run("valid hook before start", func(t *testing.T) {
		data := minimalV3Config("grpc")
		app, _ := newInitializedAppWithConfig(t, "test-app", data)
		hook := BusinessHook{
			Name:  "test-hook",
			Stage: BusinessHookBeforeStart,
			Func:  func(context.Context) error { return nil },
		}
		err := installBusinessHook(app, hook)
		require.NoError(t, err)
		assert.Len(t, app.opts.beforeStartHooks, 1)
	})

	t.Run("valid hook before stop", func(t *testing.T) {
		data := minimalV3Config("grpc")
		app, _ := newInitializedAppWithConfig(t, "test-app", data)
		hook := BusinessHook{
			Name:  "test-hook",
			Stage: BusinessHookBeforeStop,
			Func:  func(context.Context) error { return nil },
		}
		err := installBusinessHook(app, hook)
		require.NoError(t, err)
		assert.Len(t, app.opts.beforeStopHooks, 1)
	})

	t.Run("valid hook after stop", func(t *testing.T) {
		data := minimalV3Config("grpc")
		app, _ := newInitializedAppWithConfig(t, "test-app", data)
		hook := BusinessHook{
			Name:  "test-hook",
			Stage: BusinessHookAfterStop,
			Func:  func(context.Context) error { return nil },
		}
		err := installBusinessHook(app, hook)
		require.NoError(t, err)
		assert.Len(t, app.opts.afterStopHooks, 1)
	})

	t.Run("nil func error", func(t *testing.T) {
		data := minimalV3Config("grpc")
		app, _ := newInitializedAppWithConfig(t, "test-app", data)
		hook := BusinessHook{
			Name:  "test-hook",
			Stage: BusinessHookBeforeStart,
			Func:  nil,
		}
		err := installBusinessHook(app, hook)
		require.Error(t, err)
	})

	t.Run("unsupported stage error", func(t *testing.T) {
		data := minimalV3Config("grpc")
		app, _ := newInitializedAppWithConfig(t, "test-app", data)
		hook := BusinessHook{
			Name:  "test-hook",
			Stage: BusinessHookStage("unknown"),
			Func:  func(context.Context) error { return nil },
		}
		err := installBusinessHook(app, hook)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported")
	})
}

// --- App.addBackgroundTask ---

func TestApp_AddBackgroundTask(t *testing.T) {
	t.Run("valid task", func(t *testing.T) {
		data := minimalV3Config("grpc")
		app, _ := newInitializedAppWithConfig(t, "test-app", data)
		task := &mockInternalServer{}
		err := installBackgroundTask(app, task)
		require.NoError(t, err)
		assert.Len(t, app.opts.internalServers, 1)
	})

	t.Run("nil task error", func(t *testing.T) {
		data := minimalV3Config("grpc")
		app, _ := newInitializedAppWithConfig(t, "test-app", data)
		err := installBackgroundTask(app, nil)
		require.Error(t, err)
	})
}

func installBusinessHook(app *App, hook BusinessHook) error {
	staged := newStagedInstall(app)
	if err := staged.addBusinessHook(hook); err != nil {
		return err
	}
	return staged.commit()
}

func installBackgroundTask(app *App, task BackgroundTask) error {
	staged := newStagedInstall(app)
	if err := staged.addBackgroundTask(task); err != nil {
		return err
	}
	return staged.commit()
}

// --- App.InstallBusiness edge cases ---

func TestApp_InstallBusiness_Errors(t *testing.T) {
	t.Run("already installed returns error", func(t *testing.T) {
		data := minimalV3Config("grpc")
		app, _ := newInitializedAppWithConfig(t, "test-app", data)
		app.explicitBundleInstalled = true
		err := app.InstallBusiness(&BusinessBundle{})
		require.Error(t, err)
		var assemblyErr *yassembly.Error
		assert.ErrorAs(t, err, &assemblyErr)
		assert.Equal(t, yassembly.ErrInstallRegistrationConflict, assemblyErr.Code)
	})

	t.Run("stopped app returns error", func(t *testing.T) {
		data := minimalV3Config("grpc")
		app, _ := newInitializedAppWithConfig(t, "test-app", data)
		app.state = lifecycleStateStopped
		err := app.InstallBusiness(&BusinessBundle{})
		require.Error(t, err)
		var assemblyErr *yassembly.Error
		assert.ErrorAs(t, err, &assemblyErr)
		assert.Equal(t, yassembly.ErrRuntimeNotReady, assemblyErr.Code)
	})
}

func TestInstallBusiness_GovernorHTTP(t *testing.T) {
	app := newGovernorBindingTestApp(t, map[string]any{"bind": "127.0.0.1", "port": 0})

	require.NoError(t, app.InstallBusiness(&BusinessBundle{
		GovernorHTTP: []GovernorHTTPBinding{{
			Method: http.MethodGet,
			Path:   "/hello",
			Handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("hi"))
			},
		}},
	}))

	serveGovernorAsync(t, app.opts.governor)
	waitGovernorStarted(t, app.opts.governor)

	base := "http://" + app.opts.governor.Info().Address

	resp, err := http.Get(base + "/hello")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	resp, err = http.Get(base + "/routes")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var routes []string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&routes))
	assert.Contains(t, routes, "/hello")
}

func TestInstallBusiness_GovernorHTTP_DisabledGovernor(t *testing.T) {
	app := newGovernorBindingTestApp(t, map[string]any{"enabled": false, "bind": "127.0.0.1"})

	require.NoError(t, app.InstallBusiness(&BusinessBundle{
		GovernorHTTP: []GovernorHTTPBinding{{
			Method:  http.MethodGet,
			Path:    "/hello",
			Handler: func(http.ResponseWriter, *http.Request) {},
		}},
	}))
}

func TestInstallGovernorHTTPBinding_Rejections(t *testing.T) {
	app := newGovernorBindingTestApp(t, map[string]any{"bind": "127.0.0.1", "port": 0})
	handler := func(http.ResponseWriter, *http.Request) {}

	t.Run("builtin path rejected", func(t *testing.T) {
		err := installGovernorBinding(app, GovernorHTTPBinding{
			Method:  http.MethodGet,
			Path:    "/configs",
			Handler: handler,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already installed")
	})

	t.Run("duplicate path rejected", func(t *testing.T) {
		require.NoError(t, installGovernorBinding(app, GovernorHTTPBinding{
			Method:  http.MethodGet,
			Path:    "/dup",
			Handler: handler,
		}))
		err := installGovernorBinding(app, GovernorHTTPBinding{
			Method:  http.MethodPost,
			Path:    "/dup",
			Handler: handler,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already installed")
	})

	t.Run("invalid binding rejected", func(t *testing.T) {
		err := installGovernorBinding(app, GovernorHTTPBinding{
			Method:  http.MethodGet,
			Path:    "no-slash",
			Handler: handler,
		})
		require.Error(t, err)
	})
}

func installGovernorBinding(app *App, binding GovernorHTTPBinding) error {
	staged := newStagedInstall(app)
	if err := staged.installGovernorHTTPBinding(binding); err != nil {
		return err
	}
	return staged.commit()
}

func newGovernorBindingTestApp(t *testing.T, governorCfg map[string]any) *App {
	t.Helper()

	manager := newTestManager(t, map[string]any{
		"yggdrasil": map[string]any{
			"mode":   "prod-http-gateway",
			"admin":  map[string]any{"governor": governorCfg},
			"server": map[string]any{"transports": []any{"test"}},
		},
	})
	app, err := New(
		"governor-binding",
		WithConfigManager(manager),
		WithModules(testTransportModule{recorder: newTransportRecorder()}),
		WithPlanOverrides(yassembly.ForceDefault("observability.logger.handler", "text")),
	)
	require.NoError(t, err)
	require.NoError(t, app.Prepare(t.Context()))

	t.Cleanup(func() {
		_ = app.opts.governor.Stop()
		_ = app.Stop(context.Background())
	})
	return app
}
