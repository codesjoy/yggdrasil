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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source"
	"github.com/codesjoy/yggdrasil/v3/config/source/memory"
	"github.com/codesjoy/yggdrasil/v3/module"
)

// --- WithInternalServer ---

func TestWithInternalServer(t *testing.T) {
	t.Run("adds servers to opts", func(t *testing.T) {
		opts := &options{}
		s1 := &mockInternalServer{}
		s2 := &mockInternalServer{}
		err := WithInternalServer(s1, s2)(opts)
		require.NoError(t, err)
		assert.Len(t, opts.internalServers, 2)
	})
}

// --- WithBeforeStartHook ---

func TestWithBeforeStartHook(t *testing.T) {
	t.Run("adds hook functions", func(t *testing.T) {
		opts := &options{}
		fn := func(context.Context) error { return nil }
		err := WithBeforeStartHook(fn)(opts)
		require.NoError(t, err)
		assert.Len(t, opts.beforeStartHooks, 1)
	})
}

// --- WithBeforeStopHook ---

func TestWithBeforeStopHook(t *testing.T) {
	t.Run("adds hook functions", func(t *testing.T) {
		opts := &options{}
		fn := func(context.Context) error { return nil }
		err := WithBeforeStopHook(fn)(opts)
		require.NoError(t, err)
		assert.Len(t, opts.beforeStopHooks, 1)
	})
}

// --- WithAfterStopHook ---

func TestWithAfterStopHook(t *testing.T) {
	t.Run("adds hook functions", func(t *testing.T) {
		opts := &options{}
		fn := func(context.Context) error { return nil }
		err := WithAfterStopHook(fn)(opts)
		require.NoError(t, err)
		assert.Len(t, opts.afterStopHooks, 1)
	})
}

// --- WithCleanup ---

func TestWithCleanup(t *testing.T) {
	t.Run("adds cleanup function", func(t *testing.T) {
		opts := &options{}
		fn := func(context.Context) error { return nil }
		err := WithCleanup("test", fn)(opts)
		require.NoError(t, err)
		assert.Len(t, opts.lifecycleOptions, 1)
	})
}

// --- WithConfigManager ---

func TestWithConfigManager(t *testing.T) {
	t.Run("sets config manager", func(t *testing.T) {
		opts := &options{}
		mgr := config.NewManager()
		err := WithConfigManager(mgr)(opts)
		require.NoError(t, err)
		assert.Equal(t, mgr, opts.configManager)
	})

	t.Run("nil manager sets nil", func(t *testing.T) {
		opts := &options{}
		err := WithConfigManager(nil)(opts)
		require.NoError(t, err)
		assert.Nil(t, opts.configManager)
	})
}

// --- WithMode ---

func TestWithMode(t *testing.T) {
	t.Run("sets mode", func(t *testing.T) {
		opts := &options{}
		err := WithMode("production")(opts)
		require.NoError(t, err)
		assert.Equal(t, "production", opts.mode)
	})
}

// --- WithPlanOverrides ---

func TestWithPlanOverrides(t *testing.T) {
	t.Run("nil overrides are skipped", func(t *testing.T) {
		opts := &options{}
		err := WithPlanOverrides(nil, nil)(opts)
		require.NoError(t, err)
		assert.Empty(t, opts.planOverrides)
	})

	t.Run("mixed nil and valid overrides", func(t *testing.T) {
		opts := &options{}
		o1 := yassembly.EnableModule("test-module")
		err := WithPlanOverrides(nil, o1)(opts)
		require.NoError(t, err)
		assert.Len(t, opts.planOverrides, 1)
	})
}

// --- WithConfigPath ---

func TestWithConfigPath(t *testing.T) {
	t.Run("sets config path", func(t *testing.T) {
		opts := &options{}
		err := WithConfigPath("/etc/app/config.yaml")(opts)
		require.NoError(t, err)
		assert.Equal(t, "/etc/app/config.yaml", opts.configPath)
	})
}

// --- WithConfigSource ---

func TestWithConfigSource(t *testing.T) {
	t.Run("adds source", func(t *testing.T) {
		opts := &options{}
		src := memory.NewSource("test", map[string]any{"key": "value"})
		err := WithConfigSource("test", config.PriorityOverride, src)(opts)
		require.NoError(t, err)
		assert.Len(t, opts.configSources, 1)
		assert.Equal(t, "test", opts.configSources[0].Name)
	})

	t.Run("nil source skip", func(t *testing.T) {
		opts := &options{}
		err := WithConfigSource("test", config.PriorityOverride, nil)(opts)
		require.NoError(t, err)
		assert.Empty(t, opts.configSources)
	})

	t.Run("duplicate source skip", func(t *testing.T) {
		opts := &options{}
		src := memory.NewSource("test", map[string]any{"key": "value"})
		err := WithConfigSource("test", config.PriorityOverride, src)(opts)
		require.NoError(t, err)
		err = WithConfigSource("test2", config.PriorityOverride, src)(opts)
		require.NoError(t, err)
		assert.Len(t, opts.configSources, 1)
	})
}

// --- WithModules ---

func TestWithModules(t *testing.T) {
	t.Run("adds modules", func(t *testing.T) {
		opts := &options{}
		m := &stubModule{name: "test-mod"}
		err := WithModules(m)(opts)
		require.NoError(t, err)
		assert.Len(t, opts.modules, 1)
	})

	t.Run("nil modules skipped", func(t *testing.T) {
		opts := &options{}
		err := WithModules(nil, nil)(opts)
		require.NoError(t, err)
		assert.Empty(t, opts.modules)
	})
}

func TestWithCapabilityRegistrations(t *testing.T) {
	t.Run("adds registrations", func(t *testing.T) {
		opts := &options{}
		err := WithCapabilityRegistrations(CapabilityRegistration{
			Name:         "capability.test.provider",
			Capabilities: func() []module.Capability { return nil },
		})(opts)
		require.NoError(t, err)
		require.Len(t, opts.capabilityRegistrations, 1)
		assert.Equal(t, "capability.test.provider", opts.capabilityRegistrations[0].Name)
	})

	t.Run("empty name is rejected", func(t *testing.T) {
		opts := &options{}
		err := WithCapabilityRegistrations(CapabilityRegistration{
			Capabilities: func() []module.Capability { return nil },
		})(opts)
		require.ErrorContains(t, err, "name is empty")
	})

	t.Run("nil capabilities callback is rejected", func(t *testing.T) {
		opts := &options{}
		err := WithCapabilityRegistrations(CapabilityRegistration{
			Name: "capability.test.nil",
		})(opts)
		require.ErrorContains(t, err, "capabilities callback is nil")
	})

	t.Run("config path without init is rejected", func(t *testing.T) {
		opts := &options{}
		err := WithCapabilityRegistrations(CapabilityRegistration{
			Name:         "capability.test.path",
			ConfigPath:   "yggdrasil.test",
			Capabilities: func() []module.Capability { return nil },
		})(opts)
		require.ErrorContains(t, err, "config_path requires init callback")
	})
}

// --- applyOptions ---

func TestApplyOptions(t *testing.T) {
	t.Run("applies all options", func(t *testing.T) {
		opts := &options{}
		err := applyOptions(opts,
			WithMode("dev"),
		)
		require.NoError(t, err)
		assert.Equal(t, "dev", opts.mode)
	})

	t.Run("error stops early", func(t *testing.T) {
		opts := &options{}
		expected := errors.New("boom")
		err := applyOptions(opts,
			func(o *options) error { return expected },
			WithMode("should-not-apply"),
		)
		require.ErrorIs(t, err, expected)
		assert.Equal(t, "", opts.mode)
	})
}

// --- New ---

func TestNew(t *testing.T) {
	t.Run("creates app with options", func(t *testing.T) {
		app, err := New("open-test", WithMode("dev"))
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.Equal(t, "open-test", app.name)
		assert.Equal(t, "dev", app.opts.mode)
	})

	t.Run("empty app name fails", func(t *testing.T) {
		app, err := New("")
		require.Error(t, err)
		assert.Nil(t, app)
		assert.Contains(t, err.Error(), "app name is required")
	})
}

// --- options.buildLifecycleOptions ---

func TestBuildLifecycleOptions(t *testing.T) {
	t.Run("builds all lifecycle options", func(t *testing.T) {
		opts := &options{
			beforeStartHooks: []func(context.Context) error{
				func(context.Context) error { return nil },
			},
			beforeStopHooks: []func(context.Context) error{
				func(context.Context) error { return nil },
			},
			afterStopHooks: []func(context.Context) error{
				func(context.Context) error { return nil },
			},
		}
		result := opts.buildLifecycleOptions()
		assert.NotEmpty(t, result)
		assert.Len(t, result, 8)
	})

	t.Run("includes extra lifecycle options", func(t *testing.T) {
		opts := &options{
			lifecycleOptions: []lifecycleOption{func(r *lifecycleRunner) error { return nil }},
		}
		result := opts.buildLifecycleOptions()
		assert.Len(t, result, 9)
	})
}

// --- configLayerSource ---

func TestConfigLayerSource(t *testing.T) {
	t.Run("fields are set correctly", func(t *testing.T) {
		src := memory.NewSource("test", map[string]any{"k": "v"})
		cls := configLayerSource{
			Name:     "layer",
			Priority: config.PriorityOverride,
			Source:   src,
		}
		assert.Equal(t, "layer", cls.Name)
		assert.Equal(t, config.PriorityOverride, cls.Priority)
		assert.Equal(t, src, cls.Source)
	})
}

// --- helpers for options tests ---

type stubModule struct {
	name string
}

func (s *stubModule) Name() string                            { return s.name }
func (s *stubModule) ConfigPath() string                      { return "" }
func (s *stubModule) Init(context.Context, config.View) error { return nil }

// Ensure mockConfigSource still satisfies source.Source
var _ source.Source = (*mockConfigSource)(nil)
