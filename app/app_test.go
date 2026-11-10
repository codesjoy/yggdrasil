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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internallifecycle "github.com/codesjoy/yggdrasil/v3/app/internal/lifecycle"
	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
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

func TestRuntimeLoggerBeforeSnapshotUsesPrivateDiscardLogger(t *testing.T) {
	var global bytes.Buffer
	oldDefault := slog.Default()
	t.Cleanup(func() { slog.SetDefault(oldDefault) })
	slog.SetDefault(slog.New(slog.NewJSONHandler(&global, nil)))

	app, err := New("bootstrap-logger")
	require.NoError(t, err)
	require.NotSame(t, oldDefault, app.runtimeLogger())
	app.runtimeLogger().Error("bootstrap-only message")
	assert.Empty(t, global.String())
}

func TestBuildLifecycleOptionsIncludesAppCleanup(t *testing.T) {
	app := newTestApp(t, "runtime-cleanup")
	tracerShutdowns := 0
	meterShutdowns := 0
	watchStops := 0
	app.tracerShutdown = func(context.Context) error {
		tracerShutdowns++
		return nil
	}
	app.meterShutdown = func(context.Context) error {
		meterShutdowns++
		return nil
	}
	app.stopWatch = func() { watchStops++ }

	require.NoError(t, app.lifecycle.Init(app.buildLifecycleOptions()...))
	require.NoError(t, app.lifecycle.Stop(context.Background()))
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

func TestStopBeforeStartBlocksRestartAndClientInitialization(t *testing.T) {
	app := newTestApp(t, "stop-before-start")
	require.NoError(t, app.Stop(context.Background()))
	require.ErrorIs(t, app.Start(context.Background()), errRestartUnsupported)

	_, err := app.NewClient(context.Background(), "svc")
	require.ErrorIs(t, err, errRestartUnsupported)
}

func TestWaitWaitsForStopBeforeStart(t *testing.T) {
	app := newTestApp(t, "wait-stop-before-start")
	cleanupEntered := make(chan struct{})
	releaseCleanup := make(chan struct{})
	require.NoError(
		t,
		app.lifecycle.Init(
			internallifecycle.WithCleanup("blocking", func(ctx context.Context) error {
				close(cleanupEntered)
				select {
				case <-releaseCleanup:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}),
		),
	)

	stopDone := make(chan error, 1)
	go func() {
		stopDone <- app.Stop(context.Background())
	}()
	select {
	case <-cleanupEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not reach cleanup")
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- app.Wait()
	}()
	select {
	case err := <-waitDone:
		t.Fatalf("Wait returned before Stop completed: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(releaseCleanup)
	require.NoError(t, <-stopDone)
	select {
	case err := <-waitDone:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Wait did not return after Stop completed")
	}
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

type lifecycleTestModule struct {
	name    string
	stopped atomic.Int32
}

func (m *lifecycleTestModule) Name() string { return m.name }
func (m *lifecycleTestModule) Init(context.Context, config.View) error {
	return nil
}

func (m *lifecycleTestModule) Stop(context.Context) error {
	m.stopped.Add(1)
	return nil
}

type blockingReloadTestModule struct {
	name    string
	entered chan struct{}
	release chan struct{}
}

func (m *blockingReloadTestModule) Name() string { return m.name }
func (m *blockingReloadTestModule) ConfigPath() string {
	return "test"
}

func (m *blockingReloadTestModule) PrepareReload(
	context.Context,
	config.View,
) (module.ReloadCommitter, error) {
	select {
	case m.entered <- struct{}{}:
	default:
	}
	<-m.release
	return noOpReloadCommitter{}, nil
}

type noOpReloadCommitter struct{}

func (noOpReloadCommitter) Commit(context.Context) error   { return nil }
func (noOpReloadCommitter) Rollback(context.Context) error { return nil }

func TestWithModulesRegistersCandidatesAndRequiresExplicitEnable(t *testing.T) {
	moduleName := "test.extra.module"
	app := newInitializedApp(
		t,
		"with-modules",
		WithModules(testExtraModule{name: moduleName}),
	)
	diag := app.hub.Diagnostics()
	names := make([]string, 0, len(diag.Topology))
	names = append(names, diag.Topology...)
	require.NotContains(t, names, moduleName)

	enabled := newInitializedApp(
		t,
		"with-modules-enabled",
		WithModules(testExtraModule{name: moduleName}),
		WithPlanOverrides(yassembly.EnableModule(moduleName)),
	)
	diag = enabled.hub.Diagnostics()
	names = append(names[:0], diag.Topology...)
	require.Contains(t, names, moduleName)
}

func TestPrepareThenStopRunsModuleCleanup(t *testing.T) {
	mod := &lifecycleTestModule{name: "prepare-stop.module"}
	app := newInitializedApp(
		t,
		"prepare-stop-cleanup",
		WithModules(mod),
		WithPlanOverrides(yassembly.EnableModule(mod.Name())),
	)

	require.NoError(t, app.Stop(context.Background()))
	require.Equal(t, int32(1), mod.stopped.Load())
}

func TestStopWaitsForInFlightReload(t *testing.T) {
	mod := &blockingReloadTestModule{
		name:    "blocking.reload",
		entered: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	configData := assemblyTestConfig(false)
	configData["test"] = map[string]any{"value": "one"}
	app, manager := newInitializedAppWithConfig(
		t,
		"reload-stop-serial",
		configData,
		WithModules(mod, testTransportModule{recorder: newTransportRecorder()}),
		WithPlanOverrides(yassembly.EnableModule(mod.Name())),
	)
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	app.mu.Lock()
	app.state = lifecycleStateRunning
	app.mu.Unlock()
	require.NoError(t, manager.LoadLayer(
		"reload",
		config.PriorityOverride,
		memory.NewSource("reload", map[string]any{
			"test": map[string]any{"value": "two"},
		}),
	))

	reloadDone := make(chan error, 1)
	go func() {
		reloadDone <- app.Reload(context.Background())
	}()
	select {
	case <-mod.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("reload did not reach prepare")
	}

	stopDone := make(chan error, 1)
	go func() {
		stopDone <- app.Stop(context.Background())
	}()
	select {
	case err := <-stopDone:
		t.Fatalf("Stop returned before in-flight reload completed: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(mod.release)
	require.NoError(t, <-reloadDone)
	require.NoError(t, <-stopDone)
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
	requireAssemblyErrorCode(
		t,
		app.Reload(context.Background()),
		yassembly.ErrReloadRequiresRestart,
	)

	state := app.hub.ReloadState()
	require.True(t, state.RestartRequired)

	diag := app.hub.Diagnostics()
	binding := findBindingDiag(t, diag, "transport.server.provider")
	require.Equal(t, []string{"http"}, binding.Requested)
	require.Equal(t, []string{"http"}, binding.Resolved)
}

func TestReloadRestartRequiredRemainsStickyAndDoesNotAdvanceStablePlan(t *testing.T) {
	app, manager := newInitializedAppWithConfig(
		t,
		"restart-sticky",
		minimalV3Config("grpc"),
	)
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	app.mu.Lock()
	app.state = lifecycleStateRunning
	app.mu.Unlock()

	stablePlan := app.runtimePlan()
	stableSpec := app.assemblySpec
	stableHash := app.lastStablePlanHash
	stableResolved := app.opts.resolvedSettings
	require.NotNil(t, stablePlan)
	require.NotNil(t, app.runtimeAssembly)
	require.Same(t, stableSpec, app.runtimeAssembly.Spec)

	require.NoError(t, manager.LoadLayer(
		"test",
		config.PriorityOverride,
		memory.NewSource("test", minimalV3Config("http")),
	))
	requireAssemblyErrorCode(
		t,
		app.Reload(context.Background()),
		yassembly.ErrReloadRequiresRestart,
	)
	require.Same(t, stablePlan, app.runtimePlan())
	require.Same(t, stableSpec, app.assemblySpec)
	require.Same(t, stableSpec, app.runtimeAssembly.Spec)
	require.Equal(t, stableHash, app.lastStablePlanHash)
	require.Equal(t, stableResolved, app.opts.resolvedSettings)

	requireAssemblyErrorCode(
		t,
		app.Reload(context.Background()),
		yassembly.ErrReloadRequiresRestart,
	)

	require.NoError(t, manager.LoadLayer(
		"test",
		config.PriorityOverride,
		memory.NewSource("test", minimalV3Config("grpc")),
	))
	requireAssemblyErrorCode(
		t,
		app.Reload(context.Background()),
		yassembly.ErrReloadRequiresRestart,
	)
	require.Same(t, stablePlan, app.runtimePlan())
	require.Equal(t, stableHash, app.lastStablePlanHash)
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
		err := app.stopResources(context.Background())
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
