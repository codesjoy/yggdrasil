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
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source/memory"
	"github.com/codesjoy/yggdrasil/v3/module"
)

func TestNewOptionError(t *testing.T) {
	_, err := New("retry-app", func(*options) error {
		return errors.New("inject init error")
	})
	require.Error(t, err)
}

func TestBuildLifecycleOptionsIncludesAppCleanup(t *testing.T) {
	app := newTestApp(t, "runtime-cleanup")
	tracerShutdowns := 0
	meterShutdowns := 0
	watchStops := 0
	app.swapTracerShutdown(func(context.Context) error {
		tracerShutdowns++
		return nil
	})
	app.swapMeterShutdown(func(context.Context) error {
		meterShutdowns++
		return nil
	})
	app.stopWatch = func() { watchStops++ }

	require.NoError(t, app.lifecycle.Init(app.buildLifecycleOptions()...))
	require.NoError(t, app.lifecycle.Stop())
	require.NoError(t, app.shutdownRuntimeAdapters(context.Background()))
	assert.Equal(t, 1, tracerShutdowns)
	assert.Equal(t, 1, meterShutdowns)
	assert.Equal(t, 1, watchStops)
	assert.Nil(t, app.stopWatch)
}

func TestNewClientTriggersInitialization(t *testing.T) {
	app := newTestApp(t, "client-init")

	_, err := app.NewClient(context.Background(), "missing-service")
	require.Error(t, err)

	app.mu.Lock()
	state := app.state
	app.mu.Unlock()
	assert.Equal(t, lifecycleStateInitialized, state)
}

func TestRestartUnsupportedAfterStop(t *testing.T) {
	app := newTestApp(t, "stop-app")
	require.NoError(t, app.initializeLocked(context.Background()))
	require.NoError(t, app.Stop(context.Background()))

	app.mu.Lock()
	state := app.state
	app.mu.Unlock()
	assert.Equal(t, lifecycleStateStopped, state)

	_, err := app.NewClient(context.Background(), "svc")
	require.ErrorIs(t, err, errRestartUnsupported)
}

type testExtraModule struct {
	name string
}

func (m testExtraModule) Name() string { return m.name }

func (m testExtraModule) Capabilities() []module.Capability {
	return []module.Capability{
		{
			Spec: module.CapabilitySpec{
				Name:        "test.extra",
				Cardinality: module.NamedOne,
				Type:        reflect.TypeOf((func() string)(nil)),
			},
			Name:  "default",
			Value: func() string { return "ok" },
		},
	}
}

func TestWithModulesRegistersBusinessModules(t *testing.T) {
	app := newInitializedApp(
		t,
		"with-modules",
		WithModules(testExtraModule{name: "test.extra.module"}),
	)
	diag := app.hub.Diagnostics()
	names := make([]string, 0, len(diag.Topology))
	names = append(names, diag.Topology...)
	require.Contains(t, names, "test.extra.module")
}

func TestModuleHubEndpointIncludesBindings(t *testing.T) {
	app, _ := newInitializedAppWithConfig(t, "module-hub-endpoint", minimalV3Config("grpc"))
	t.Cleanup(func() {
		_ = app.opts.governor.Stop()
		_ = app.Stop(context.Background())
	})

	errCh := serveGovernorAsync(t, app.opts.governor)
	waitGovernorStarted(t, app.opts.governor)

	resp, err := http.Get("http://" + app.opts.governor.Info().Address + "/module-hub")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var diag module.Diagnostics
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&diag))
	binding := findBindingDiag(t, diag, "transport.server.provider")
	require.Equal(t, []string{"grpc"}, binding.Requested)
	require.Equal(t, []string{"grpc"}, binding.Resolved)

	require.NoError(t, app.opts.governor.Stop())
	requireAsyncNoError(t, errCh, "governor serve goroutine did not exit")
}

func TestReloadUpdatesCapabilityBindingsAndMarksRestartRequired(t *testing.T) {
	app, manager := newInitializedAppWithConfig(t, "binding-reload", minimalV3Config("grpc"))
	t.Cleanup(func() {
		_ = app.Stop(context.Background())
	})

	app.mu.Lock()
	app.state = lifecycleStateRunning
	app.mu.Unlock()

	require.NoError(
		t,
		manager.LoadLayer(
			"test",
			config.PriorityOverride,
			memory.NewSource("test", minimalV3Config("http")),
		),
	)
	require.NoError(t, app.Reload(context.Background()))

	state := app.hub.ReloadState()
	require.True(t, state.RestartRequired)

	diag := app.hub.Diagnostics()
	binding := findBindingDiag(t, diag, "transport.server.provider")
	require.Equal(t, []string{"http"}, binding.Requested)
	require.Equal(t, []string{"http"}, binding.Resolved)
}

func TestDiagnosticsPayloadIncludesModuleHubState(t *testing.T) {
	app, _ := newInitializedAppWithConfig(t, "diagnostics-payload", minimalV3Config("grpc"))
	t.Cleanup(func() {
		_ = app.Stop(context.Background())
	})

	data, err := json.Marshal(map[string]any{
		"module_hub": app.hub.Diagnostics(),
		"assembly":   app.assemblyDiagnostics(),
	})
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(data, &doc))
	moduleHub, ok := doc["module_hub"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, moduleHub, "bindings")
	require.Contains(t, moduleHub, "reload_state")
	require.Contains(t, doc, "assembly")
}

// --- setStoppedLocked ---

func TestApp_SetStoppedLocked(t *testing.T) {
	t.Run("transitions from running to stopped", func(t *testing.T) {
		app := newTestApp(t, "test")
		app.state = lifecycleStateRunning
		app.setStoppedLocked()
		assert.Equal(t, lifecycleStateStopped, app.state)
	})

	t.Run("new state stays new", func(t *testing.T) {
		app := newTestApp(t, "test")
		app.state = lifecycleStateNew
		app.setStoppedLocked()
		assert.Equal(t, lifecycleStateNew, app.state)
	})
}

// --- finishRun ---

func TestApp_FinishRun(t *testing.T) {
	t.Run("nil waitDone", func(t *testing.T) {
		app := newTestApp(t, "test")
		app.state = lifecycleStateRunning
		app.finishRun(nil)
		assert.Equal(t, lifecycleStateStopped, app.state)
		assert.Nil(t, app.waitDone)
	})

	t.Run("non-nil waitDone closes channel", func(t *testing.T) {
		app := newTestApp(t, "test")
		app.waitDone = make(chan struct{})
		done := app.waitDone

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-done
		}()

		app.finishRun(nil)
		wg.Wait()
		assert.Nil(t, app.waitDone)
		assert.Nil(t, app.waitErr)
	})

	t.Run("records error", func(t *testing.T) {
		app := newTestApp(t, "test")
		app.waitDone = make(chan struct{})
		expectedErr := errApplicationAlreadyRunning
		app.finishRun(expectedErr)
		assert.Equal(t, expectedErr, app.waitErr)
	})
}

// --- prepareStartLocked ---

func TestApp_PrepareStartLocked_AlreadyRunning(t *testing.T) {
	t.Run("already running returns error", func(t *testing.T) {
		app := newTestApp(t, "test")
		app.state = lifecycleStateRunning
		err := app.prepareStartLocked(context.Background())
		require.Error(t, err)
		assert.ErrorIs(t, err, errApplicationAlreadyRunning)
	})

	t.Run("serving returns error", func(t *testing.T) {
		app := newTestApp(t, "test")
		app.state = lifecycleStateServing
		err := app.prepareStartLocked(context.Background())
		require.Error(t, err)
		assert.ErrorIs(t, err, errApplicationAlreadyRunning)
	})
}

func TestApp_PrepareStartLocked_AlreadyStopped(t *testing.T) {
	t.Run("stopped returns error", func(t *testing.T) {
		app := newTestApp(t, "test")
		app.state = lifecycleStateStopped
		err := app.prepareStartLocked(context.Background())
		require.Error(t, err)
		assert.ErrorIs(t, err, errRestartUnsupported)
	})
}

// --- ensureClientReadyLocked ---

func TestApp_EnsureClientReadyLocked_Stopped(t *testing.T) {
	t.Run("stopped returns error", func(t *testing.T) {
		app := newTestApp(t, "test")
		app.state = lifecycleStateStopped
		err := app.ensureClientReadyLocked(context.Background())
		require.Error(t, err)
		assert.ErrorIs(t, err, errRestartUnsupported)
	})
}

// --- stopResources ---

func TestApp_StopResources(t *testing.T) {
	t.Run("initialized app returns nil", func(t *testing.T) {
		data := minimalV3Config("grpc")
		app, _ := newInitializedAppWithConfig(t, "test-app", data)
		err := app.stopResources()
		require.NoError(t, err)
	})
}

// --- stopConfigWatchLocked ---

func TestApp_StopConfigWatchLocked(t *testing.T) {
	t.Run("nil stopWatch is no-op", func(t *testing.T) {
		app := newTestApp(t, "test")
		app.stopConfigWatchLocked()
		assert.Nil(t, app.stopWatch)
	})

	t.Run("calls and clears stopWatch", func(t *testing.T) {
		app := newTestApp(t, "test")
		called := false
		app.stopWatch = func() { called = true }
		app.stopConfigWatchLocked()
		assert.True(t, called)
		assert.Nil(t, app.stopWatch)
	})
}

// --- reloadAsync ---

func TestApp_ReloadAsync(t *testing.T) {
	t.Run("does not panic on stopped app", func(t *testing.T) {
		app := newTestApp(t, "test")
		// reloadAsync launches a goroutine that calls Reload
		// Just ensure it doesn't panic
		app.reloadAsync()
	})
}

// --- Phase 6 Governor ---

func TestPhase6GovernorServeStopsCleanly(t *testing.T) {
	if os.Getenv("YGGDRASIL_PHASE6_LEAK") != "1" {
		t.Skip("set YGGDRASIL_PHASE6_LEAK=1 to run leak-oriented checks")
	}

	app, _ := newInitializedAppWithConfig(t, "phase6-leak", minimalV3Config("grpc"))
	t.Cleanup(func() {
		_ = app.opts.governor.Stop()
		_ = app.Stop(context.Background())
	})

	errCh := serveGovernorAsync(t, app.opts.governor)
	waitGovernorStarted(t, app.opts.governor)
	require.NoError(t, app.opts.governor.Stop())

	requireAsyncNoError(t, errCh, "governor serve goroutine did not exit")
}
