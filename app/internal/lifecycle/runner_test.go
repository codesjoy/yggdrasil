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

package lifecycle

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/admin/governor"
	"github.com/codesjoy/yggdrasil/v3/discovery/registry"
	internalidentity "github.com/codesjoy/yggdrasil/v3/internal/identity"
	yserver "github.com/codesjoy/yggdrasil/v3/transport/runtime/server"
)

func TestNewRunner(t *testing.T) {
	runner, err := New()
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

func TestNewRunnerWithOptions(t *testing.T) {
	mockReg := createMockRegistry()
	mockReg.On("Type").Return("test-registry")

	runner, err := New(WithRegistry(mockReg))
	require.NoError(t, err)
	assert.Equal(t, mockReg, runner.registry)
}

func TestLifecycleInitBeforeRun(t *testing.T) {
	runner, err := New()
	require.NoError(t, err)

	err = runner.Init(WithShutdownTimeout(10 * time.Second))
	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, runner.shutdownTimeout)
}

func TestLifecycleInitAfterRunDoesNotReconfigure(t *testing.T) {
	runner, err := New()
	require.NoError(t, err)
	runner.running = true

	err = runner.Init(WithShutdownTimeout(20 * time.Second))
	require.NoError(t, err)
	assert.True(t, runner.running)
	assert.Zero(t, runner.shutdownTimeout)
}

func TestLifecycleRunHooks(t *testing.T) {
	runner, err := New()
	require.NoError(t, err)

	calls := make(map[Stage]int)
	for stage := Stage(1); stage < StageMax; stage++ {
		stage := stage
		runner.hooks[stage].Register(func(context.Context) error {
			calls[stage]++
			return nil
		})
	}

	for stage := Stage(1); stage < StageMax; stage++ {
		require.NoError(t, runner.runHooks(context.Background(), stage))
		assert.Equal(t, 1, calls[stage])
	}
}

func TestLifecycleRunHooksWithUnknownStage(t *testing.T) {
	runner, err := New()
	require.NoError(t, err)
	require.NoError(t, runner.runHooks(context.Background(), Stage(999)))
}

func TestLifecycleWithInternalServers(t *testing.T) {
	internalServer := createMockInternalServer()

	runner, err := New(WithInternalServers(internalServer))
	require.NoError(t, err)
	require.Len(t, runner.internalServers, 1)
	assert.Same(t, internalServer, runner.internalServers[0])
}

func TestLifecycleWithMultipleInternalServers(t *testing.T) {
	first := createMockInternalServer()
	second := createMockInternalServer()

	runner, err := New(WithInternalServers(first, second))
	require.NoError(t, err)
	require.Len(t, runner.internalServers, 2)
	assert.Same(t, first, runner.internalServers[0])
	assert.Same(t, second, runner.internalServers[1])
}

func TestLifecycleWithHookInvalidStage(t *testing.T) {
	runner, err := New()
	require.NoError(t, err)

	err = WithHook(Stage(999), func(context.Context) error { return nil })(runner)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hook stage not found")
}

func TestLifecycleStopTimeout(t *testing.T) {
	tests := []struct {
		name     string
		timeout  time.Duration
		expected time.Duration
	}{
		{name: "zero", timeout: 0, expected: defaultShutdownTimeout},
		{name: "smaller", timeout: 10 * time.Second, expected: 10 * time.Second},
		{name: "equal", timeout: defaultShutdownTimeout, expected: defaultShutdownTimeout},
		{name: "larger", timeout: 45 * time.Second, expected: 45 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner, err := New(WithShutdownTimeout(tt.timeout))
			require.NoError(t, err)
			assert.Equal(t, tt.expected, runner.stopTimeout())
		})
	}
}

func TestLifecycleConcurrentInit(t *testing.T) {
	runner, err := New()
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		timeout := time.Duration(i) * time.Second
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = runner.Init(WithShutdownTimeout(timeout))
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
	runner, err := New()
	require.NoError(t, err)

	hookErr := errors.New("hook error")
	runner.hooks[StageBeforeStart].Register(func(context.Context) error {
		return hookErr
	})

	err = runner.runHooks(context.Background(), StageBeforeStart)
	require.ErrorIs(t, err, hookErr)
}

func TestLifecycleHookExecutionOrder(t *testing.T) {
	runner, err := New()
	require.NoError(t, err)

	executed := []int{}
	runner.hooks[StageBeforeStart].Register(func(context.Context) error {
		executed = append(executed, 1)
		return nil
	})
	runner.hooks[StageBeforeStart].Register(func(context.Context) error {
		executed = append(executed, 2)
		return nil
	})

	require.NoError(t, runner.runHooks(context.Background(), StageBeforeStart))
	assert.Len(t, executed, 2)
	assert.Contains(t, executed, 1)
	assert.Contains(t, executed, 2)
}

func TestLifecycleRegisterSuccess(t *testing.T) {
	mockReg := createMockRegistry()
	mockReg.On("Register", mock.Anything, mock.Anything).Return(nil)
	mockReg.On("Type").Return("test-registry")

	runner, err := New(WithRegistry(mockReg))
	require.NoError(t, err)

	require.NoError(t, runner.register())
	assert.True(t, mockReg.registered)
	assert.Equal(t, registryStateDone, runner.registryState)
}

func TestLifecycleRegisterNilRegistry(t *testing.T) {
	runner, err := New()
	require.NoError(t, err)

	require.NoError(t, runner.register())
	assert.Equal(t, registryStateInit, runner.registryState)
}

func TestLifecycleRegisterAlreadyRegistered(t *testing.T) {
	mockReg := createMockRegistry()
	mockReg.On("Type").Return("test-registry")

	runner, err := New(WithRegistry(mockReg))
	require.NoError(t, err)
	runner.registryState = registryStateDone

	require.NoError(t, runner.register())
	assert.False(t, mockReg.registered)
}

func TestLifecycleRegisterError(t *testing.T) {
	mockReg := createMockRegistry()
	mockReg.On("Register", mock.Anything, mock.Anything).Return(errors.New("register error"))
	mockReg.On("Type").Return("test-registry")

	runner, err := New(WithRegistry(mockReg))
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

	runner, err := New(WithRegistry(mockReg))
	require.NoError(t, err)
	require.NoError(t, runner.register())

	require.NoError(t, runner.deregister(context.Background()))
	assert.True(t, mockReg.deregistered)
	assert.Equal(t, registryStateCancel, runner.registryState)
}

func TestLifecycleDeregisterNilRegistry(t *testing.T) {
	runner, err := New()
	require.NoError(t, err)

	require.NoError(t, runner.deregister(context.Background()))
	assert.Equal(t, registryStateInit, runner.registryState)
}

func TestLifecycleDeregisterNotRegistered(t *testing.T) {
	mockReg := createMockRegistry()
	mockReg.On("Type").Return("test-registry")

	runner, err := New(WithRegistry(mockReg))
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

	runner, err := New(WithRegistry(mockReg))
	require.NoError(t, err)
	require.NoError(t, runner.register())

	err = runner.deregister(context.Background())
	require.Error(t, err)
	assert.True(t, mockReg.deregistered)
	assert.Equal(t, registryStateCancel, runner.registryState)
}

func TestLifecycleInstanceMethods(t *testing.T) {
	identity := internalidentity.Identity{
		AppName:   "app-a",
		Namespace: "ns-a",
		Version:   "1.2.3",
		Region:    "r1",
		Zone:      "z1",
		Campus:    "c1",
		Metadata:  map[string]string{"k": "v"},
	}
	runner, err := New(WithIdentity(identity))
	require.NoError(t, err)

	assert.Equal(t, "r1", runner.Region())
	assert.Equal(t, "z1", runner.Zone())
	assert.Equal(t, "c1", runner.Campus())
	assert.Equal(t, "ns-a", runner.Namespace())
	assert.Equal(t, "app-a", runner.Name())
	assert.Equal(t, "1.2.3", runner.Version())
	assert.Equal(t, map[string]string{"k": "v"}, runner.Metadata())
	identity.Metadata["k"] = "changed"
	assert.Equal(t, map[string]string{"k": "v"}, runner.Metadata())
}

func TestLifecycleEndpointsBasic(t *testing.T) {
	runner, err := New()
	require.NoError(t, err)

	endpoints := runner.Endpoints()
	assert.NotNil(t, endpoints)
	assert.Empty(t, endpoints)
}

func TestLifecycleEndpointsWithServer(t *testing.T) {
	mainServer := &blockingAppServer{
		endpts: []yserver.Endpoint{
			stubEndpoint{
				protocol: "grpc",
				address:  "127.0.0.1:9000",
				metadata: nil,
				kind:     yserver.EndpointKindRPC,
			},
		},
	}

	runner, err := New(WithServer(mainServer))
	require.NoError(t, err)

	endpoints := runner.Endpoints()
	require.Len(t, endpoints, 1)
	assert.Equal(t, "127.0.0.1:9000", endpoints[0].Address())
	assert.Equal(t, "grpc", endpoints[0].Scheme())
	assert.Equal(t, string(yserver.EndpointKindRPC), endpoints[0].Metadata()[registry.MDServerKind])
}

func TestLifecycleEndpointsIntegration(t *testing.T) {
	gov, err := governor.NewServerWithConfig(governor.Config{Advertise: true}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = gov.Stop() })

	runner, err := New(WithGovernor(gov))
	require.NoError(t, err)

	endpoints := runner.Endpoints()
	require.Len(t, endpoints, 1)
	assert.Equal(
		t,
		string(yserver.EndpointKindGovernor),
		endpoints[0].Metadata()[registry.MDServerKind],
	)
}

func TestLifecycleEndpointsWithServerAndGovernor(t *testing.T) {
	gov, err := governor.NewServerWithConfig(governor.Config{Advertise: true}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = gov.Stop() })

	mainServer := &blockingAppServer{
		endpts: []yserver.Endpoint{
			stubEndpoint{
				protocol: "grpc",
				address:  "127.0.0.1:9001",
				metadata: map[string]string{"version": "v1"},
				kind:     yserver.EndpointKindRPC,
			},
		},
	}

	runner, err := New(WithServer(mainServer), WithGovernor(gov))
	require.NoError(t, err)

	endpoints := runner.Endpoints()
	require.Len(t, endpoints, 2)
	assert.Equal(t, string(yserver.EndpointKindRPC), endpoints[0].Metadata()[registry.MDServerKind])
	assert.Equal(t, "v1", endpoints[0].Metadata()["version"])
	assert.Equal(
		t,
		string(yserver.EndpointKindGovernor),
		endpoints[1].Metadata()[registry.MDServerKind],
	)
}

func TestLifecycleRegistryStateTransitionsAreSafe(t *testing.T) {
	runner, err := New()
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
	runner, err := New()
	require.NoError(t, err)

	err = runner.Run(context.Background())
	require.ErrorIs(t, err, errGovernorRequired)
}

func TestLifecycleRunStopIntegration(t *testing.T) {
	gov, err := governor.NewServerWithConfig(governor.Config{Advertise: true}, nil)
	require.NoError(t, err)

	runner, err := New(WithGovernor(gov))
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		done <- runner.Run(context.Background())
	}()

	time.Sleep(10 * time.Millisecond)
	require.NoError(t, runner.Stop(context.Background()))

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

	runner, err := New(
		WithGovernor(gov),
		WithServer(mainServer),
		WithInternalServers(internalServer),
	)
	require.NoError(t, err)

	err = runner.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal server")
	if assert.NotNil(t, mainServer.stopCtx) {
		assert.NotNil(t, mainServer.stopCtx.Done())
	}
}

func TestLifecycleStopUsesShutdownTimeout(t *testing.T) {
	mainServer := &blockingAppServer{}
	internalServer := &blockingInternalServer{}

	runner, err := New(
		WithServer(mainServer),
		WithInternalServers(internalServer),
		WithShutdownTimeout(30*time.Millisecond),
	)
	require.NoError(t, err)

	start := time.Now()
	err = runner.Stop(context.Background())
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

	runner, err := New(
		WithBeforeStopHooks(func(context.Context) error {
			beforeStopCalled = true
			return nil
		}),
		WithCleanup("cleanup", func(context.Context) error {
			cleanupCalled = true
			return nil
		}),
		WithAfterStopHooks(func(context.Context) error {
			afterStopCalled = true
			return nil
		}),
	)
	require.NoError(t, err)

	require.NoError(t, runner.Stop(context.Background()))
	assert.True(t, beforeStopCalled)
	assert.True(t, cleanupCalled)
	assert.True(t, afterStopCalled)
}
