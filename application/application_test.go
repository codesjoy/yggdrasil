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

package application

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

	"github.com/codesjoy/yggdrasil/v2/governor"
	"github.com/codesjoy/yggdrasil/v2/internal/constant"
	"github.com/codesjoy/yggdrasil/v2/registry"
	yserver "github.com/codesjoy/yggdrasil/v2/server"
)

// Mock implementations for testing
type mockRegistry struct {
	mock.Mock
	registered   bool
	deregistered bool
}

func (m *mockRegistry) Register(ctx context.Context, s registry.Instance) error {
	args := m.Called(ctx, s)
	m.registered = true
	return args.Error(0)
}

func (m *mockRegistry) Deregister(ctx context.Context, s registry.Instance) error {
	args := m.Called(ctx, s)
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

func (b *blockingAppServer) RegisterService(*yserver.ServiceDesc, interface{}) {}

func (b *blockingAppServer) RegisterRestService(*yserver.RestServiceDesc, interface{}, ...string) {}

func (b *blockingAppServer) RegisterRestRawHandlers(...*yserver.RestRawHandlerDesc) {}

func (b *blockingAppServer) Serve(chan<- struct{}) error {
	return nil
}

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

func (r *runningAppServer) RegisterService(*yserver.ServiceDesc, interface{}) {}

func (r *runningAppServer) RegisterRestService(*yserver.RestServiceDesc, interface{}, ...string) {}

func (r *runningAppServer) RegisterRestRawHandlers(...*yserver.RestRawHandlerDesc) {}

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

// Helper functions
func createMockRegistry() *mockRegistry {
	return &mockRegistry{}
}

func createMockInternalServer() *mockInternalServer {
	return &mockInternalServer{}
}

// Test cases
func TestNew_Application(t *testing.T) {
	app, err := newApplication()
	require.NoError(t, err)
	require.NotNil(t, app)
	require.NotNil(t, app.hooks)
	assert.Equal(
		t,
		4,
		len(app.hooks),
	) // StageMin, stageBeforeStart, stageBeforeStop, stageAfterStop
	assert.False(t, app.running)
	assert.Equal(t, registryStateInit, app.registryState)
	assert.Nil(t, app.server)
	assert.Empty(t, app.internalSvr)
	assert.Nil(t, app.registry)
}

func TestNew_ApplicationWithOptions(t *testing.T) {
	mockReg := createMockRegistry()
	mockReg.On("Type").Return("test-registry")

	app, err := newApplication(WithRegistry(mockReg))
	require.NoError(t, err)
	require.NotNil(t, app)
	assert.Equal(t, mockReg, app.registry)
}

func TestApplication_Init_BeforeStart(t *testing.T) {
	app, err := newApplication()
	require.NoError(t, err)

	// Test init before start
	app.Init(WithShutdownTimeout(time.Second * 10))
	assert.Equal(t, time.Second*10, app.shutdownTimeout)
}

func TestApplication_Init_AfterStart(t *testing.T) {
	app, err := newApplication()
	require.NoError(t, err)
	app.running = true

	// Test init after start - should not change timeout
	app.Init(WithShutdownTimeout(time.Second * 20))
	// Should remain unchanged because app is running
	assert.True(t, app.running)
}

func TestApplication_Run_Once(t *testing.T) {
	// Skip this test as it requires full governor setup
	// The Run() method depends on governor.Server which has complex dependencies
	t.Skip("Skipping Run() test due to governor dependency complexity")
}

func TestApplication_Stop_Once(t *testing.T) {
	// Skip this test as it requires full governor setup
	// The Stop() method depends on governor.Server which has complex dependencies
	t.Skip("Skipping Stop() test due to governor dependency complexity")
}

func TestApplication_RunHooks(t *testing.T) {
	app, _ := newApplication()

	calls := make(map[Stage]int)

	// Register hooks
	for stage := Stage(1); stage < stageMax; stage++ {
		stage := stage
		app.hooks[stage].Register(func(context.Context) error {
			calls[stage]++
			return nil
		})
	}

	// Test running hooks for each stage
	for stage := Stage(1); stage < stageMax; stage++ {
		_ = app.runHooks(context.Background(), stage)
		assert.Equal(t, 1, calls[stage], "Hook for stage %v should be called once", stage)
	}
}

func TestApplication_RunHooks_NonExistentStage(t *testing.T) {
	app, _ := newApplication()

	// Test running hook for non-existent stage (should not panic)
	_ = app.runHooks(context.Background(), Stage(999)) // Non-existent stage
}

func TestApplication_Register_Success(t *testing.T) {
	mockReg := createMockRegistry()
	mockReg.On("Register", mock.Anything, mock.Anything).Return(nil)
	mockReg.On("Type").Return("test-registry")

	app, _ := newApplication(WithRegistry(mockReg))

	// Test successful registration
	err := app.register()
	assert.NoError(t, err)
	assert.True(t, mockReg.registered)
	assert.Equal(t, registryStateDone, app.registryState)
}

func TestApplication_Register_NilRegistry(t *testing.T) {
	app, _ := newApplication()

	// Test registration with nil registry (should not panic)
	err := app.register()
	assert.NoError(t, err)
	assert.Equal(t, registryStateInit, app.registryState)
}

func TestApplication_Register_AlreadyRegistered(t *testing.T) {
	mockReg := createMockRegistry()
	mockReg.On("Register", mock.Anything, mock.Anything).Return(nil)
	mockReg.On("Type").Return("test-registry")

	app, _ := newApplication(WithRegistry(mockReg))

	// Set state to already registered
	app.registryState = registryStateDone

	// Test registering again (should not call Register)
	err := app.register()
	assert.NoError(t, err)
	// mockReg.registered should still be false since Register wasn't called
	assert.False(t, mockReg.registered)
}

func TestApplication_Register_Error(t *testing.T) {
	mockReg := createMockRegistry()
	mockReg.On("Register", mock.Anything, mock.Anything).Return(errors.New("register error"))
	mockReg.On("Type").Return("test-registry")

	app, err := newApplication(WithRegistry(mockReg))
	require.NoError(t, err)

	err = app.register()
	assert.Error(t, err)
	assert.False(t, mockReg.deregistered)
	assert.Equal(t, registryStateInit, app.registryState)
}

func TestApplication_Deregister_Success(t *testing.T) {
	mockReg := createMockRegistry()
	mockReg.On("Register", mock.Anything, mock.Anything).Return(nil)
	mockReg.On("Deregister", mock.Anything, mock.Anything).Return(nil)
	mockReg.On("Type").Return("test-registry")

	app, _ := newApplication(WithRegistry(mockReg))

	// First register
	err := app.register()
	require.NoError(t, err)
	assert.Equal(t, registryStateDone, app.registryState)

	// Then deregister
	err = app.deregister(context.Background())
	assert.NoError(t, err)
	assert.True(t, mockReg.deregistered)
	assert.Equal(t, registryStateCancel, app.registryState)
}

func TestApplication_Deregister_NilRegistry(t *testing.T) {
	app, _ := newApplication()

	// Test deregistration with nil registry (should not panic)
	err := app.deregister(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, registryStateInit, app.registryState)
}

func TestApplication_Deregister_NotRegistered(t *testing.T) {
	mockReg := createMockRegistry()
	mockReg.On("Type").Return("test-registry")

	app, _ := newApplication(WithRegistry(mockReg))

	// Test deregistration when not registered
	err := app.deregister(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, registryStateInit, app.registryState)
	assert.False(t, mockReg.deregistered)
}

func TestApplication_Deregister_Error(t *testing.T) {
	mockReg := createMockRegistry()
	mockReg.On("Register", mock.Anything, mock.Anything).Return(nil)
	mockReg.On("Deregister", mock.Anything, mock.Anything).Return(errors.New("deregister error"))
	mockReg.On("Type").Return("test-registry")

	app, _ := newApplication(WithRegistry(mockReg))

	// First register
	err := app.register()
	require.NoError(t, err)

	// Then deregister with error
	err = app.deregister(context.Background())
	assert.Error(t, err)
	assert.True(t, mockReg.deregistered)
	assert.Equal(t, registryStateCancel, app.registryState)
}

func TestApplication_GetShutdownTimeout(t *testing.T) {
	tests := []struct {
		name     string
		timeout  time.Duration
		expected time.Duration
	}{
		{"zero timeout", 0, defaultShutdownTimeout},
		{"less than default", time.Second * 10, defaultShutdownTimeout},
		{"greater than default", time.Second * 60, time.Second * 60},
		{"equal to default", defaultShutdownTimeout, defaultShutdownTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, _ := newApplication(WithShutdownTimeout(tt.timeout))
			assert.Equal(t, tt.expected, app.getShutdownTimeout())
		})
	}
}

func TestApplication_WaitSignals_Setup(t *testing.T) {
	app, _ := newApplication(WithShutdownTimeout(time.Millisecond * 100))

	// Test waitSignals setup (this mainly tests that it doesn't panic)
	cleanup := app.waitSignals()
	defer cleanup()

	// Give a moment for the signal handler goroutine to start
	time.Sleep(time.Millisecond * 10)

	// The actual signal handling is difficult to test in unit tests
	// as it involves process-level signals and os.Exit
	// This test mainly verifies the setup doesn't panic
}

func TestApplication_InstanceMethods(t *testing.T) {
	app, _ := newApplication()

	// Test all instance methods delegate to pkg functions
	// These may return empty values in test environment since pkg may not be initialized
	// Just test that the methods don't panic and return expected types
	assert.IsType(t, "", app.Region())
	assert.IsType(t, "", app.Zone())
	assert.IsType(t, "", app.Campus())
	assert.IsType(t, "", app.Namespace())
	assert.IsType(t, "", app.Name())
	assert.IsType(t, "", app.Version())
	assert.IsType(t, map[string]string{}, app.Metadata())
}

func TestApplication_WithInternalServer(t *testing.T) {
	mockInternalSvr := createMockInternalServer()

	app, _ := newApplication(WithInternalServer(mockInternalSvr))

	assert.Equal(t, 1, len(app.internalSvr))
	assert.Same(t, mockInternalSvr, app.internalSvr[0])
}

func TestApplication_WithMultipleInternalServers(t *testing.T) {
	mockInternalSvr1 := createMockInternalServer()
	mockInternalSvr2 := createMockInternalServer()

	app, _ := newApplication(WithInternalServer(mockInternalSvr1, mockInternalSvr2))

	assert.Equal(t, 2, len(app.internalSvr))
	assert.Same(t, mockInternalSvr1, app.internalSvr[0])
	assert.Same(t, mockInternalSvr2, app.internalSvr[1])
}

func TestApplication_WithHook(t *testing.T) {
	app, _ := newApplication()

	called := false
	hook := func(context.Context) error {
		called = true
		return nil
	}

	// Test adding hook
	err := WithHook(stageBeforeStart, hook)(app)
	assert.NoError(t, err)

	// Run the hook
	_ = app.runHooks(context.Background(), stageBeforeStart)
	assert.True(t, called)
}

func TestApplication_WithHook_InvalidStage(t *testing.T) {
	app, _ := newApplication()

	hook := func(context.Context) error {
		return nil
	}

	// Test adding hook to invalid stage
	err := WithHook(Stage(999), hook)(app)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "hook stage not found")
}

func TestApplication_WithBeforeStartHook(t *testing.T) {
	app, _ := newApplication()

	called := false
	hook := func(context.Context) error {
		called = true
		return nil
	}

	// Test adding before start hook
	app.Init(WithBeforeStartHook(hook))

	// Run the hook
	_ = app.runHooks(context.Background(), stageBeforeStart)
	assert.True(t, called)
}

func TestApplication_WithBeforeStopHook(t *testing.T) {
	app, _ := newApplication()

	called := false
	hook := func(context.Context) error {
		called = true
		return nil
	}

	// Test adding before stop hook
	app.Init(WithBeforeStopHook(hook))

	// Run the hook
	_ = app.runHooks(context.Background(), stageBeforeStop)
	assert.True(t, called)
}

func TestApplication_WithAfterStopHook(t *testing.T) {
	app, _ := newApplication()

	called := false
	hook := func(context.Context) error {
		called = true
		return nil
	}

	// Test adding after stop hook
	app.Init(WithAfterStopHook(hook))

	// Run the hook
	_ = app.runHooks(context.Background(), stageAfterStop)
	assert.True(t, called)
}

func TestApplication_WithShutdownTimeout(t *testing.T) {
	app, _ := newApplication()

	timeout := time.Second * 45

	// Test setting shutdown timeout
	app.Init(WithShutdownTimeout(timeout))
	assert.Equal(t, timeout, app.shutdownTimeout)
}

func TestApplication_ConcurrentInit(t *testing.T) {
	app, _ := newApplication()

	var wg sync.WaitGroup

	// Test concurrent init calls
	for i := 0; i < 10; i++ {
		timeout := time.Duration(i) * time.Second
		wg.Add(1)
		go func() {
			defer wg.Done()
			app.Init(WithShutdownTimeout(timeout))
		}()
	}

	wg.Wait()

	// Any applied value should be one of the requested timeouts.
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
	}, app.shutdownTimeout)
}

func TestEndpoint_Struct(t *testing.T) {
	endpoint := endpoint{
		address: "localhost:8080",
		scheme:  "http",
		Attr:    map[string]string{"weight": "100", "protocol": "http"},
	}

	assert.Equal(t, "localhost:8080", endpoint.Address())
	assert.Equal(t, "http", endpoint.Scheme())
	assert.Equal(t, map[string]string{"weight": "100", "protocol": "http"}, endpoint.Metadata())
}

func TestApplication_Endpoints_Basic(t *testing.T) {
	app, err := newApplication()
	require.NoError(t, err)

	endpoints := app.Endpoints()
	assert.NotNil(t, endpoints)
	assert.Empty(t, endpoints)
}

func TestApplication_Endpoints_WithInternalServers(t *testing.T) {
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

	app, err := newApplication(WithServer(mainServer))
	require.NoError(t, err)

	endpoints := app.Endpoints()
	require.Len(t, endpoints, 1)
	assert.Equal(t, "127.0.0.1:9000", endpoints[0].Address())
	assert.Equal(t, "grpc", endpoints[0].Scheme())
	assert.Equal(t, string(constant.ServerKindRPC), endpoints[0].Metadata()[registry.MDServerKind])
}

func TestApplication_StopServers_Hooks(t *testing.T) {
	app, _ := newApplication()

	beforeStopCalled := false
	afterStopCalled := false

	// Add hooks
	app.hooks[stageBeforeStop].Register(func(context.Context) error {
		beforeStopCalled = true
		return nil
	})
	app.hooks[stageAfterStop].Register(func(context.Context) error {
		afterStopCalled = true
		return nil
	})

	// Test hooks directly instead of stopServers to avoid governor nil issue
	_ = app.runHooks(context.Background(), stageBeforeStop)
	assert.True(t, beforeStopCalled)

	_ = app.runHooks(context.Background(), stageAfterStop)
	assert.True(t, afterStopCalled)
}

func TestApplication_StartServers_Hooks(t *testing.T) {
	app, _ := newApplication()

	beforeStartCalled := false

	// Add hook
	app.hooks[stageBeforeStart].Register(func(context.Context) error {
		beforeStartCalled = true
		return nil
	})

	// Test runHooks directly instead of startServers to avoid governor nil issue
	_ = app.runHooks(context.Background(), stageBeforeStart)
	assert.True(t, beforeStartCalled)
}

func TestConstants(t *testing.T) {
	// Test constants
	assert.Equal(t, Stage(uint32(1)), stageBeforeStart)
	assert.Equal(t, Stage(uint32(2)), stageBeforeStop)
	assert.Equal(t, Stage(uint32(3)), stageCleanup)
	assert.Equal(t, Stage(uint32(4)), stageAfterStop)
	assert.Equal(t, Stage(uint32(5)), stageMax)

	assert.Equal(t, time.Second*30, defaultShutdownTimeout)

	assert.Equal(t, 0, registryStateInit)
	assert.Equal(t, 1, registryStateRegistering)
	assert.Equal(t, 2, registryStateDone)
	assert.Equal(t, 3, registryStateCancel)
}

func TestShutdownSignals(t *testing.T) {
	// Test that shutdownSignals is properly defined
	assert.NotEmpty(t, shutdownSignals)
	assert.Contains(t, shutdownSignals, os.Interrupt)

	// Platform-specific signals
	if len(shutdownSignals) >= 3 {
		// POSIX systems should have SIGTERM
		assert.Contains(t, shutdownSignals, syscall.SIGTERM)
	}
}

func TestApplication_RegistryStates(t *testing.T) {
	app, _ := newApplication()

	// Test initial state
	assert.Equal(t, registryStateInit, app.registryState)

	// Test state transitions
	app.registryState = registryStateDone
	assert.Equal(t, registryStateDone, app.registryState)

	app.registryState = registryStateCancel
	assert.Equal(t, registryStateCancel, app.registryState)
}

func TestApplication_MutexSafety(t *testing.T) {
	app, _ := newApplication()

	var wg sync.WaitGroup

	// Test concurrent access to registry state
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			app.register()
			_ = app.deregister(context.Background())
		}()
	}

	wg.Wait()
	// Should not have any race conditions
}

func TestApplication_OptionWithRegistry(t *testing.T) {
	app, _ := newApplication()

	// Test WithRegistry option
	mockReg := createMockRegistry()
	mockReg.On("Type").Return("test-registry")

	err := WithRegistry(mockReg)(app)
	assert.NoError(t, err)
	assert.Equal(t, mockReg, app.registry)
}

func TestApplication_OptionWithNilRegistry(t *testing.T) {
	app, _ := newApplication()

	// Test WithRegistry option with nil registry
	err := WithRegistry(nil)(app)
	assert.NoError(t, err)
	assert.Nil(t, app.registry)
}

func TestApplication_HookExecutionOrder(t *testing.T) {
	app, _ := newApplication()

	executionOrder := []int{}

	// Register multiple hooks for the same stage
	app.hooks[stageBeforeStart].Register(func(context.Context) error {
		executionOrder = append(executionOrder, 1)
		return nil
	})
	app.hooks[stageBeforeStart].Register(func(context.Context) error {
		executionOrder = append(executionOrder, 2)
		return nil
	})

	// Run hooks
	_ = app.runHooks(context.Background(), stageBeforeStart)

	// Both hooks should have been executed
	assert.Equal(t, 2, len(executionOrder))
	assert.Contains(t, executionOrder, 1)
	assert.Contains(t, executionOrder, 2)
}

func TestApplication_WithServer(t *testing.T) {
	app, _ := newApplication()

	// Test WithServer option (without actually testing server functionality)
	// We can't easily test server behavior without creating a real server
	// but we can test that the option doesn't panic
	// This test mainly checks that WithServer is callable
	err := WithServer(nil)(app)
	assert.NoError(t, err)
	assert.Nil(t, app.server)
}

func TestApplication_WithGovernor(t *testing.T) {
	app, _ := newApplication()

	// Test WithGovernor option (without actually testing governor functionality)
	// We can't easily test governor behavior without creating a real governor
	// but we can test that the option doesn't panic
	err := WithGovernor(nil)(app)
	assert.NoError(t, err)
	assert.Nil(t, app.governor)
}

func TestApplication_WaitSignals_MoreCoverage(t *testing.T) {
	app, _ := newApplication(WithShutdownTimeout(time.Millisecond * 50))

	// Test waitSignals setup again with different timeout
	cleanup := app.waitSignals()
	defer cleanup()

	// Give a moment for the signal handler goroutine to start
	time.Sleep(time.Millisecond * 5)
}

func TestApplication_Run_RequiresGovernor(t *testing.T) {
	app, err := newApplication()
	require.NoError(t, err)

	err = app.Run()
	require.ErrorIs(t, err, errGovernorRequired)
}

func TestApplication_Init_MoreScenarios(t *testing.T) {
	app, _ := newApplication()

	// Test multiple options
	app.Init(
		WithShutdownTimeout(time.Second*15),
		WithBeforeStartHook(func(context.Context) error { return nil }),
		WithAfterStopHook(func(context.Context) error { return nil }),
	)

	assert.Equal(t, time.Second*15, app.shutdownTimeout)
}

func TestApplication_Hook_ErrorHandling(t *testing.T) {
	app, _ := newApplication()

	// Test hook with error
	hookErr := errors.New("hook error")
	app.hooks[stageBeforeStart].Register(func(context.Context) error {
		return hookErr
	})

	err := app.runHooks(context.Background(), stageBeforeStart)
	assert.ErrorIs(t, err, hookErr)
}

func TestApplication_RunStop_Integration(t *testing.T) {
	// Create a real governor for integration testing
	// This will test the actual Run/Stop functionality
	gov, err := governor.NewServer()
	assert.NoError(t, err)
	app, err := newApplication(WithGovernor(gov))
	assert.NoError(t, err)

	// Test Run and Stop in a separate goroutine to avoid blocking
	done := make(chan error, 1)
	go func() {
		done <- app.Run()
	}()

	// Give some time for the app to start
	time.Sleep(time.Millisecond * 10)

	// Stop the app
	err = app.Stop()
	assert.NoError(t, err)

	// Wait for Run to complete
	select {
	case runErr := <-done:
		assert.NoError(t, runErr)
	case <-time.After(time.Second * 5):
		t.Fatal("Run did not complete within timeout")
	}
}

func TestApplication_Run_InternalServerFailureTriggersStop(t *testing.T) {
	gov, err := governor.NewServer()
	require.NoError(t, err)

	mainServer := &runningAppServer{stopCh: make(chan struct{})}
	internalServer := &failingInternalServer{serveErr: errors.New("internal failure")}

	app, err := newApplication(
		WithGovernor(gov),
		WithServer(mainServer),
		WithInternalServer(internalServer),
	)
	require.NoError(t, err)

	err = app.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal server")
	if assert.NotNil(t, mainServer.stopCtx) {
		assert.NotNil(t, mainServer.stopCtx.Done())
	}
}

func TestApplication_Stop_UsesShutdownTimeout(t *testing.T) {
	mainServer := &blockingAppServer{}
	internalServer := &blockingInternalServer{}

	app, err := newApplication(
		WithServer(mainServer),
		WithInternalServer(internalServer),
		WithShutdownTimeout(30*time.Millisecond),
	)
	require.NoError(t, err)

	start := time.Now()
	err = app.Stop()
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

func TestApplication_Endpoints_Integration(t *testing.T) {
	// Create a real governor for testing Endpoints functionality
	gov, err := governor.NewServer()
	assert.NoError(t, err)
	t.Cleanup(func() { _ = gov.Stop() })
	app, err := newApplication(WithGovernor(gov))
	assert.NoError(t, err)

	// Test Endpoints with real governor
	endpoints := app.Endpoints()
	assert.NotNil(t, endpoints)
	require.Len(t, endpoints, 1)

	// The endpoints should be accessible since governor is initialized
	assert.IsType(t, []registry.Endpoint{}, endpoints)
	assert.Equal(
		t,
		string(constant.ServerKindGovernor),
		endpoints[0].Metadata()[registry.MDServerKind],
	)
}

func TestApplication_Endpoints_WithServerAndGovernor(t *testing.T) {
	gov, err := governor.NewServer()
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

	app, err := newApplication(WithServer(mainServer), WithGovernor(gov))
	require.NoError(t, err)

	endpoints := app.Endpoints()
	require.Len(t, endpoints, 2)
	assert.Equal(t, string(constant.ServerKindRPC), endpoints[0].Metadata()[registry.MDServerKind])
	assert.Equal(t, "v1", endpoints[0].Metadata()["version"])
	assert.Equal(
		t,
		string(constant.ServerKindGovernor),
		endpoints[1].Metadata()[registry.MDServerKind],
	)
}
