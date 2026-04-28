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

package yggdrasil

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/module"

	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source"
	"github.com/codesjoy/yggdrasil/v3/config/source/memory"
)

type blockingTask struct {
	started chan struct{}
	stopCh  chan struct{}
	once    sync.Once
	err     error
}

func newBlockingTask(err error) *blockingTask {
	return &blockingTask{
		started: make(chan struct{}),
		stopCh:  make(chan struct{}),
		err:     err,
	}
}

func (t *blockingTask) Serve() error {
	close(t.started)
	if t.err != nil {
		return t.err
	}
	<-t.stopCh
	return nil
}

func (t *blockingTask) Stop(context.Context) error {
	t.once.Do(func() {
		close(t.stopCh)
	})
	return nil
}

func rootConfigSource() source.Source {
	return memory.NewSource("root", map[string]any{
		"yggdrasil": map[string]any{
			"admin": map[string]any{
				"governor": map[string]any{
					"host": "127.0.0.1",
					"port": 0,
				},
			},
		},
	})
}

func useTestManager() {
	config.SetDefault(config.NewManager())
}

func TestOpenComposeAndInstallStartStop(t *testing.T) {
	useTestManager()

	app, err := New(
		"root-open",
		WithConfigSource("root", config.PriorityOverride, rootConfigSource()),
	)
	require.NoError(t, err)

	task := newBlockingTask(nil)
	require.NoError(
		t,
		app.ComposeAndInstall(context.Background(), func(Runtime) (*BusinessBundle, error) {
			return &BusinessBundle{
				Tasks: []BackgroundTask{task},
			}, nil
		}),
	)

	require.NoError(t, app.Start(context.Background()))
	select {
	case <-task.started:
	case <-time.After(2 * time.Second):
		t.Fatal("task did not start")
	}

	require.NoError(t, app.Stop(context.Background()))
	require.NoError(t, app.Wait())
	require.NoError(t, app.Stop(context.Background()))
}

func TestComposeAndInstallFailureStopsFacade(t *testing.T) {
	useTestManager()

	app, err := New(
		"root-compose-failure",
		WithConfigSource("root", config.PriorityOverride, rootConfigSource()),
	)
	require.NoError(t, err)

	composeErr := errors.New("compose failed")
	err = app.ComposeAndInstall(context.Background(), func(Runtime) (*BusinessBundle, error) {
		return nil, composeErr
	})
	require.ErrorIs(t, err, composeErr)
	require.Error(t, app.Start(context.Background()))
}

func TestRunHappyPathStopsOnContextCancel(t *testing.T) {
	useTestManager()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	task := newBlockingTask(nil)
	go func() {
		<-task.started
		cancel()
	}()

	err := Run(
		ctx,
		"root-run",
		func(Runtime) (*BusinessBundle, error) {
			return &BusinessBundle{
				Tasks: []BackgroundTask{task},
			}, nil
		},
		WithConfigSource("root", config.PriorityOverride, rootConfigSource()),
	)
	require.NoError(t, err)
}

func TestWaitPropagatesServeFailure(t *testing.T) {
	useTestManager()

	app, err := New(
		"root-wait-error",
		WithConfigSource("root", config.PriorityOverride, rootConfigSource()),
	)
	require.NoError(t, err)

	taskErr := errors.New("serve failed")
	require.NoError(
		t,
		app.ComposeAndInstall(context.Background(), func(Runtime) (*BusinessBundle, error) {
			return &BusinessBundle{
				Tasks: []BackgroundTask{newBlockingTask(taskErr)},
			}, nil
		}),
	)
	require.NoError(t, app.Start(context.Background()))
	require.ErrorIs(t, app.Wait(), taskErr)
}

func TestAppNilReceiver(t *testing.T) {
	var a *App
	ctx := context.Background()

	err := a.ComposeAndInstall(ctx, nil)
	assert.Equal(t, "app is not initialized", err.Error())

	err = a.Start(ctx)
	assert.Equal(t, "app is not initialized", err.Error())

	err = a.Wait()
	assert.Equal(t, "app is not initialized", err.Error())

	err = a.Stop(ctx)
	assert.Equal(t, "app is not initialized", err.Error())
}

func TestRunWithNilContext(t *testing.T) {
	//nolint:staticcheck // intentional: testing nil context validation
	err := Run(nil, "test-app", nil)
	require.Equal(t, "run context is nil", err.Error())
}

func TestRunWithEmptyAppName(t *testing.T) {
	err := Run(context.Background(), "", nil)
	require.Equal(t, "app name is required", err.Error())
}

func TestNewWithEmptyAppName(t *testing.T) {
	app, err := New("")
	require.Error(t, err)
	assert.Nil(t, app)
	assert.Contains(t, err.Error(), "app name is required")
}

func TestWithOptions(t *testing.T) {
	t.Run("WithConfigPath", func(t *testing.T) {
		opts := options{}
		err := WithConfigPath("/etc/config.yaml")(&opts)
		require.NoError(t, err)
		assert.Equal(t, "/etc/config.yaml", opts.configPath)
	})

	t.Run("WithMode", func(t *testing.T) {
		opts := options{}
		err := WithMode("debug")(&opts)
		require.NoError(t, err)
		assert.Equal(t, "debug", opts.mode)
	})

	t.Run("WithSignalHandling", func(t *testing.T) {
		opts := options{}
		err := WithSignalHandling(false)(&opts)
		require.NoError(t, err)
		require.NotNil(t, opts.signalHandling)
		assert.False(t, *opts.signalHandling)
	})

	t.Run("WithShutdownSignals", func(t *testing.T) {
		opts := options{}
		err := WithShutdownSignals(os.Interrupt)(&opts)
		require.NoError(t, err)
		assert.Equal(t, []os.Signal{os.Interrupt}, opts.shutdownSignals)
	})

	t.Run("WithConfigSource nil source is skipped", func(t *testing.T) {
		opts := options{}
		err := WithConfigSource("test", config.PriorityOverride, nil)(&opts)
		require.NoError(t, err)
		assert.Empty(t, opts.configSources)
	})

	t.Run("WithConfigSource non-nil source is appended", func(t *testing.T) {
		opts := options{}
		src := memory.NewSource("test", map[string]any{"k": "v"})
		err := WithConfigSource("test", config.PriorityOverride, src)(&opts)
		require.NoError(t, err)
		require.Len(t, opts.configSources, 1)
		assert.Equal(t, "test", opts.configSources[0].name)
	})

	t.Run("nil Option is skipped in convertOptions", func(t *testing.T) {
		appOpts, err := convertOptions(nil, WithMode("test"))
		require.NoError(t, err)
		assert.NotEmpty(t, appOpts)
	})

	t.Run("option returning error propagates", func(t *testing.T) {
		badOpt := func(opts *options) error {
			return assert.AnError
		}
		_, err := convertOptions(badOpt)
		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("WithModules", func(t *testing.T) {
		opts := options{}
		mod := &rootInitModule{name: "root.test.module"}
		err := WithModules(mod)(&opts)
		require.NoError(t, err)
		require.Len(t, opts.modules, 1)
		assert.Equal(t, mod, opts.modules[0])
	})

	t.Run("WithCapabilityRegistrations", func(t *testing.T) {
		opts := options{}
		reg := CapabilityRegistration{
			Name:         "capability.root.test",
			Capabilities: func() []module.Capability { return nil },
		}
		err := WithCapabilityRegistrations(reg)(&opts)
		require.NoError(t, err)
		require.Len(t, opts.capabilityRegistrations, 1)
		assert.Equal(t, reg.Name, opts.capabilityRegistrations[0].Name)
	})
}

func TestRunContextSignalHandlingDisabled(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := options{}
	require.NoError(t, WithSignalHandling(false)(&opts))
	ctx, cleanup := opts.runContext(parent)
	defer cleanup()

	assert.Same(t, parent, ctx)
}

func TestRootWithModulesPassThrough(t *testing.T) {
	useTestManager()

	var initCalled atomic.Bool
	app, err := New(
		"root-modules-pass-through",
		WithConfigSource("root", config.PriorityOverride, rootConfigSource()),
		WithModules(&rootInitModule{
			name: "root.test.module",
			init: func(context.Context, config.View) error {
				initCalled.Store(true)
				return nil
			},
		}),
	)
	require.NoError(t, err)

	require.NoError(
		t,
		app.ComposeAndInstall(context.Background(), func(Runtime) (*BusinessBundle, error) {
			return &BusinessBundle{}, nil
		}),
	)
	assert.True(t, initCalled.Load())
	require.NoError(t, app.Stop(context.Background()))
}

func TestRootWithCapabilityRegistrationsPassThrough(t *testing.T) {
	useTestManager()

	var initCalled atomic.Bool
	app, err := New(
		"root-registrations-pass-through",
		WithConfigSource("root", config.PriorityOverride, rootConfigSource()),
		WithCapabilityRegistrations(CapabilityRegistration{
			Name: "capability.root.pass-through",
			Init: func(context.Context, config.View) error {
				initCalled.Store(true)
				return nil
			},
			Capabilities: func() []module.Capability { return nil },
		}),
	)
	require.NoError(t, err)

	require.NoError(
		t,
		app.ComposeAndInstall(context.Background(), func(Runtime) (*BusinessBundle, error) {
			return &BusinessBundle{}, nil
		}),
	)
	assert.True(t, initCalled.Load())
	require.NoError(t, app.Stop(context.Background()))
}

type rootInitModule struct {
	name string
	init func(context.Context, config.View) error
}

func (m *rootInitModule) Name() string { return m.name }

func (m *rootInitModule) Init(ctx context.Context, view config.View) error {
	if m.init != nil {
		return m.init(ctx, view)
	}
	return nil
}
