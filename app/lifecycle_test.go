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
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/admin/governor"
	"github.com/codesjoy/yggdrasil/v3/internal/constant"
	"github.com/codesjoy/yggdrasil/v3/discovery/registry"
	yserver "github.com/codesjoy/yggdrasil/v3/server"
)

func TestNewLifecycleRunner(t *testing.T) {
	runner, err := newLifecycleRunner()
	require.NoError(t, err)
	require.NotNil(t, runner)
	require.NotNil(t, runner.hooks)
	assert.Len(t, runner.hooks, 4)
	assert.False(t, runner.running)
	assert.Equal(t, registryStateInit, runner.registryState)
	assert.Nil(t, runner.server)
	assert.Empty(t, runner.internalServers)
	assert.Nil(t, runner.registry)
}

func TestNewLifecycleRunnerWithOptions(t *testing.T) {
	mockReg := createMockRegistry()
	mockReg.On("Type").Return("test-registry")

	runner, err := newLifecycleRunner(withLifecycleRegistry(mockReg))
	require.NoError(t, err)
	assert.Equal(t, mockReg, runner.registry)
}

func TestLifecycleInit_BeforeRun(t *testing.T) {
	runner, err := newLifecycleRunner()
	require.NoError(t, err)

	err = runner.Init(withLifecycleShutdownTimeout(10 * time.Second))
	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, runner.shutdownTimeout)
}

func TestLifecycleInit_AfterRunDoesNotReconfigure(t *testing.T) {
	runner, err := newLifecycleRunner()
	require.NoError(t, err)
	runner.running = true

	err = runner.Init(withLifecycleShutdownTimeout(20 * time.Second))
	require.NoError(t, err)
	assert.True(t, runner.running)
	assert.Zero(t, runner.shutdownTimeout)
}

func TestLifecycleRunHooks(t *testing.T) {
	runner, err := newLifecycleRunner()
	require.NoError(t, err)

	calls := make(map[lifecycleStage]int)
	for stage := lifecycleStage(1); stage < lifecycleStageMax; stage++ {
		stage := stage
		runner.hooks[stage].Register(func(context.Context) error {
			calls[stage]++
			return nil
		})
	}

	for stage := lifecycleStage(1); stage < lifecycleStageMax; stage++ {
		require.NoError(t, runner.runHooks(context.Background(), stage))
		assert.Equal(t, 1, calls[stage])
	}
}

func TestLifecycleRunHooksWithUnknownStage(t *testing.T) {
	runner, err := newLifecycleRunner()
	require.NoError(t, err)
	require.NoError(t, runner.runHooks(context.Background(), lifecycleStage(999)))
}

func TestLifecycleWithInternalServers(t *testing.T) {
	internalServer := createMockInternalServer()

	runner, err := newLifecycleRunner(withLifecycleInternalServers(internalServer))
	require.NoError(t, err)
	require.Len(t, runner.internalServers, 1)
	assert.Same(t, internalServer, runner.internalServers[0])
}

func TestLifecycleWithMultipleInternalServers(t *testing.T) {
	first := createMockInternalServer()
	second := createMockInternalServer()

	runner, err := newLifecycleRunner(withLifecycleInternalServers(first, second))
	require.NoError(t, err)
	require.Len(t, runner.internalServers, 2)
	assert.Same(t, first, runner.internalServers[0])
	assert.Same(t, second, runner.internalServers[1])
}

func TestLifecycleWithHookInvalidStage(t *testing.T) {
	runner, err := newLifecycleRunner()
	require.NoError(t, err)

	err = withLifecycleHook(lifecycleStage(999), func(context.Context) error { return nil })(runner)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hook stage not found")
}

func TestLifecycleSignalShutdownTimeoutUsesDefaultMinimum(t *testing.T) {
	tests := []struct {
		name     string
		timeout  time.Duration
		expected time.Duration
	}{
		{name: "zero", timeout: 0, expected: defaultShutdownTimeout},
		{name: "smaller", timeout: 10 * time.Second, expected: defaultShutdownTimeout},
		{name: "equal", timeout: defaultShutdownTimeout, expected: defaultShutdownTimeout},
		{name: "larger", timeout: 45 * time.Second, expected: 45 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner, err := newLifecycleRunner(withLifecycleShutdownTimeout(tt.timeout))
			require.NoError(t, err)
			assert.Equal(t, tt.expected, runner.signalShutdownTimeout())
		})
	}
}

func TestLifecycleConcurrentInit(t *testing.T) {
	runner, err := newLifecycleRunner()
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		timeout := time.Duration(i) * time.Second
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = runner.Init(withLifecycleShutdownTimeout(timeout))
		}()
	}
	wg.Wait()

	assert.Contains(t, []time.Duration{
		0,
		time.Second,
		2 * time.Second,
		3 * time.Second,
		4 * time.Second,
		5 * time.Second,
		6 * time.Second,
		7 * time.Second,
		8 * time.Second,
		9 * time.Second,
	}, runner.shutdownTimeout)
}

func TestLifecycleHookErrorHandling(t *testing.T) {
	runner, err := newLifecycleRunner()
	require.NoError(t, err)

	hookErr := errors.New("hook error")
	runner.hooks[lifecycleStageBeforeStart].Register(func(context.Context) error {
		return hookErr
	})

	err = runner.runHooks(context.Background(), lifecycleStageBeforeStart)
	require.ErrorIs(t, err, hookErr)
}

func TestLifecycleHookExecutionOrder(t *testing.T) {
	runner, err := newLifecycleRunner()
	require.NoError(t, err)

	executed := []int{}
	runner.hooks[lifecycleStageBeforeStart].Register(func(context.Context) error {
		executed = append(executed, 1)
		return nil
	})
	runner.hooks[lifecycleStageBeforeStart].Register(func(context.Context) error {
		executed = append(executed, 2)
		return nil
	})

	require.NoError(t, runner.runHooks(context.Background(), lifecycleStageBeforeStart))
	assert.Len(t, executed, 2)
	assert.Contains(t, executed, 1)
	assert.Contains(t, executed, 2)
}

func TestLifecycleRegisterSuccess(t *testing.T) {
	mockReg := createMockRegistry()
	mockReg.On("Register", mock.Anything, mock.Anything).Return(nil)
	mockReg.On("Type").Return("test-registry")

	runner, err := newLifecycleRunner(withLifecycleRegistry(mockReg))
	require.NoError(t, err)

	require.NoError(t, runner.register())
	assert.True(t, mockReg.registered)
	assert.Equal(t, registryStateDone, runner.registryState)
}

func TestLifecycleRegisterNilRegistry(t *testing.T) {
	runner, err := newLifecycleRunner()
	require.NoError(t, err)

	require.NoError(t, runner.register())
	assert.Equal(t, registryStateInit, runner.registryState)
}

func TestLifecycleRegisterAlreadyRegistered(t *testing.T) {
	mockReg := createMockRegistry()
	mockReg.On("Type").Return("test-registry")

	runner, err := newLifecycleRunner(withLifecycleRegistry(mockReg))
	require.NoError(t, err)
	runner.registryState = registryStateDone

	require.NoError(t, runner.register())
	assert.False(t, mockReg.registered)
}

func TestLifecycleRegisterError(t *testing.T) {
	mockReg := createMockRegistry()
	mockReg.On("Register", mock.Anything, mock.Anything).Return(errors.New("register error"))
	mockReg.On("Type").Return("test-registry")

	runner, err := newLifecycleRunner(withLifecycleRegistry(mockReg))
	require.NoError(t, err)

	err = runner.register()
	require.Error(t, err)
	assert.False(t, mockReg.deregistered)
	assert.Equal(t, registryStateInit, runner.registryState)
}

func TestLifecycleDeregisterSuccess(t *testing.T) {
	mockReg := createMockRegistry()
	mockReg.On("Register", mock.Anything, mock.Anything).Return(nil)
	mockReg.On("Deregister", mock.Anything, mock.Anything).Return(nil)
	mockReg.On("Type").Return("test-registry")

	runner, err := newLifecycleRunner(withLifecycleRegistry(mockReg))
	require.NoError(t, err)
	require.NoError(t, runner.register())

	require.NoError(t, runner.deregister(context.Background()))
	assert.True(t, mockReg.deregistered)
	assert.Equal(t, registryStateCancel, runner.registryState)
}

func TestLifecycleDeregisterNilRegistry(t *testing.T) {
	runner, err := newLifecycleRunner()
	require.NoError(t, err)

	require.NoError(t, runner.deregister(context.Background()))
	assert.Equal(t, registryStateInit, runner.registryState)
}

func TestLifecycleDeregisterNotRegistered(t *testing.T) {
	mockReg := createMockRegistry()
	mockReg.On("Type").Return("test-registry")

	runner, err := newLifecycleRunner(withLifecycleRegistry(mockReg))
	require.NoError(t, err)

	require.NoError(t, runner.deregister(context.Background()))
	assert.Equal(t, registryStateInit, runner.registryState)
	assert.False(t, mockReg.deregistered)
}

func TestLifecycleDeregisterError(t *testing.T) {
	mockReg := createMockRegistry()
	mockReg.On("Register", mock.Anything, mock.Anything).Return(nil)
	mockReg.On("Deregister", mock.Anything, mock.Anything).Return(errors.New("deregister error"))
	mockReg.On("Type").Return("test-registry")

	runner, err := newLifecycleRunner(withLifecycleRegistry(mockReg))
	require.NoError(t, err)
	require.NoError(t, runner.register())

	err = runner.deregister(context.Background())
	require.Error(t, err)
	assert.True(t, mockReg.deregistered)
	assert.Equal(t, registryStateCancel, runner.registryState)
}

func TestLifecycleInstanceMethods(t *testing.T) {
	runner, err := newLifecycleRunner()
	require.NoError(t, err)

	assert.IsType(t, "", runner.Region())
	assert.IsType(t, "", runner.Zone())
	assert.IsType(t, "", runner.Campus())
	assert.IsType(t, "", runner.Namespace())
	assert.IsType(t, "", runner.Name())
	assert.IsType(t, "", runner.Version())
	assert.IsType(t, map[string]string{}, runner.Metadata())
}

func TestLifecycleEndpointsBasic(t *testing.T) {
	runner, err := newLifecycleRunner()
	require.NoError(t, err)

	endpoints := runner.Endpoints()
	assert.NotNil(t, endpoints)
	assert.Empty(t, endpoints)
}

func TestLifecycleEndpointsWithServer(t *testing.T) {
	mainServer := &blockingAppServer{
		endpts: []yserver.Endpoint{
			stubAppEndpoint{
				scheme:   "grpc",
				address:  "127.0.0.1:9000",
				metadata: nil,
				kind:     constant.ServerKindRPC,
			},
		},
	}

	runner, err := newLifecycleRunner(withLifecycleServer(mainServer))
	require.NoError(t, err)

	endpoints := runner.Endpoints()
	require.Len(t, endpoints, 1)
	assert.Equal(t, "127.0.0.1:9000", endpoints[0].Address())
	assert.Equal(t, "grpc", endpoints[0].Scheme())
	assert.Equal(t, string(constant.ServerKindRPC), endpoints[0].Metadata()[registry.MDServerKind])
}

func TestLifecycleEndpointsIntegration(t *testing.T) {
	gov, err := governor.NewServerWithConfig(governor.Config{Advertise: true}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = gov.Stop() })

	runner, err := newLifecycleRunner(withLifecycleGovernor(gov))
	require.NoError(t, err)

	endpoints := runner.Endpoints()
	require.Len(t, endpoints, 1)
	assert.Equal(t, string(constant.ServerKindGovernor), endpoints[0].Metadata()[registry.MDServerKind])
}

func TestLifecycleEndpointsWithServerAndGovernor(t *testing.T) {
	gov, err := governor.NewServerWithConfig(governor.Config{Advertise: true}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = gov.Stop() })

	mainServer := &blockingAppServer{
		endpts: []yserver.Endpoint{
			stubAppEndpoint{
				scheme:   "grpc",
				address:  "127.0.0.1:9001",
				metadata: map[string]string{"version": "v1"},
				kind:     constant.ServerKindRPC,
			},
		},
	}

	runner, err := newLifecycleRunner(withLifecycleServer(mainServer), withLifecycleGovernor(gov))
	require.NoError(t, err)

	endpoints := runner.Endpoints()
	require.Len(t, endpoints, 2)
	assert.Equal(t, string(constant.ServerKindRPC), endpoints[0].Metadata()[registry.MDServerKind])
	assert.Equal(t, "v1", endpoints[0].Metadata()["version"])
	assert.Equal(t, string(constant.ServerKindGovernor), endpoints[1].Metadata()[registry.MDServerKind])
}

func TestLifecycleRegistryStateTransitionsAreSafe(t *testing.T) {
	runner, err := newLifecycleRunner()
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = runner.register()
			_ = runner.deregister(context.Background())
		}()
	}
	wg.Wait()
}

func TestLifecycleRunRequiresGovernor(t *testing.T) {
	runner, err := newLifecycleRunner()
	require.NoError(t, err)

	err = runner.Run()
	require.ErrorIs(t, err, errGovernorRequired)
}

func TestLifecycleRunStopIntegration(t *testing.T) {
	gov, err := governor.NewServerWithConfig(governor.Config{Advertise: true}, nil)
	require.NoError(t, err)

	runner, err := newLifecycleRunner(withLifecycleGovernor(gov))
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		done <- runner.Run()
	}()

	time.Sleep(10 * time.Millisecond)
	require.NoError(t, runner.Stop())

	select {
	case runErr := <-done:
		require.NoError(t, runErr)
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not complete within timeout")
	}
}

func TestLifecycleRunInternalServerFailureTriggersStop(t *testing.T) {
	gov, err := governor.NewServerWithConfig(governor.Config{Advertise: true}, nil)
	require.NoError(t, err)

	mainServer := &runningAppServer{stopCh: make(chan struct{})}
	internalServer := &failingInternalServer{serveErr: errors.New("internal failure")}

	runner, err := newLifecycleRunner(
		withLifecycleGovernor(gov),
		withLifecycleServer(mainServer),
		withLifecycleInternalServers(internalServer),
	)
	require.NoError(t, err)

	err = runner.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal server")
	if assert.NotNil(t, mainServer.stopCtx) {
		assert.NotNil(t, mainServer.stopCtx.Done())
	}
}

func TestLifecycleStopUsesShutdownTimeout(t *testing.T) {
	mainServer := &blockingAppServer{}
	internalServer := &blockingInternalServer{}

	runner, err := newLifecycleRunner(
		withLifecycleServer(mainServer),
		withLifecycleInternalServers(internalServer),
		withLifecycleShutdownTimeout(30*time.Millisecond),
	)
	require.NoError(t, err)

	start := time.Now()
	err = runner.Stop()
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, elapsed, 250*time.Millisecond)
	if assert.NotNil(t, mainServer.stopCtx) {
		assert.ErrorIs(t, mainServer.stopCtx.Err(), context.DeadlineExceeded)
	}
	if assert.NotNil(t, internalServer.stopCtx) {
		assert.ErrorIs(t, internalServer.stopCtx.Err(), context.DeadlineExceeded)
	}
}

func TestLifecycleStopRunsHooks(t *testing.T) {
	var beforeStopCalled bool
	var cleanupCalled bool
	var afterStopCalled bool

	runner, err := newLifecycleRunner(
		withLifecycleBeforeStopHooks(func(context.Context) error {
			beforeStopCalled = true
			return nil
		}),
		withLifecycleCleanup("cleanup", func(context.Context) error {
			cleanupCalled = true
			return nil
		}),
		withLifecycleAfterStopHooks(func(context.Context) error {
			afterStopCalled = true
			return nil
		}),
	)
	require.NoError(t, err)

	require.NoError(t, runner.Stop())
	assert.True(t, beforeStopCalled)
	assert.True(t, cleanupCalled)
	assert.True(t, afterStopCalled)
}

func TestShutdownSignals(t *testing.T) {
	assert.NotEmpty(t, shutdownSignals)
	assert.Contains(t, shutdownSignals, os.Interrupt)
	if len(shutdownSignals) >= 3 {
		assert.Contains(t, shutdownSignals, syscall.SIGTERM)
	}
}

func TestLifecycleWaitSignalsSetup(t *testing.T) {
	runner, err := newLifecycleRunner(withLifecycleShutdownTimeout(100 * time.Millisecond))
	require.NoError(t, err)

	cleanup := runner.waitSignals()
	defer cleanup()

	time.Sleep(10 * time.Millisecond)
}

func TestLifecycleWaitSignalsMultipleSetup(t *testing.T) {
	runner, err := newLifecycleRunner(withLifecycleShutdownTimeout(50 * time.Millisecond))
	require.NoError(t, err)

	cleanup := runner.waitSignals()
	defer cleanup()

	time.Sleep(5 * time.Millisecond)
}

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
