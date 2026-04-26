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
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internalbootstrap "github.com/codesjoy/yggdrasil/v3/app/internal/bootstrap"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source/memory"
	configtest "github.com/codesjoy/yggdrasil/v3/config/testutil"
)

func TestResolveConfigPath(t *testing.T) {
	withTestFlagSet(t)

	path, explicit := internalbootstrap.ResolveConfigPath("")
	assert.Equal(t, defaultConfigPath, path)
	assert.False(t, explicit)

	configured := filepath.Join(t.TempDir(), "custom.yaml")
	path, explicit = internalbootstrap.ResolveConfigPath(configured)
	assert.Equal(t, configured, path)
	assert.True(t, explicit)

	explicitPath := filepath.Join(t.TempDir(), "explicit.yaml")
	os.Args = []string{"yggdrasil-test", "--yggdrasil-config=" + explicitPath}
	path, explicit = internalbootstrap.ResolveConfigPath(configured)
	assert.Equal(t, explicitPath, path)
	assert.True(t, explicit)
}

func TestInitLoadsConfigAndOptionSources(t *testing.T) {
	withTestFlagSet(t)
	prev := config.Default()
	t.Cleanup(func() { config.SetDefault(prev) })

	manager := config.NewManager()
	config.SetDefault(manager)

	dir := t.TempDir()
	chdir(t, dir)
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "config.yaml"),
		[]byte(
			"app:\n  startup_order: bootstrap\nyggdrasil:\n  config:\n    sources:\n      - kind: file\n        config:\n          path: ./config.override.yaml\n",
		),
		0o600,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "config.override.yaml"),
		[]byte("app:\n  startup_order: declared\n"),
		0o600,
	))

	optionSource := &mockConfigSource{
		name: "option",
		data: map[string]any{"app": map[string]any{"startup_order": "option"}},
	}

	app := newTestApp(
		t,
		"config-order",
		WithConfigManager(manager),
		WithConfigSource("option", config.PriorityOverride, optionSource),
	)
	require.NoError(t, app.initializeLocked(context.Background()))

	var out struct {
		App struct {
			StartupOrder string `mapstructure:"startup_order"`
		} `mapstructure:"app"`
	}
	require.NoError(t, manager.Section("app").Decode(&out.App))
	assert.Equal(t, "option", out.App.StartupOrder)
}

func TestInitializeLocked_WithConfigManagerDoesNotChangeDefaultManager(t *testing.T) {
	withTestFlagSet(t)
	chdir(t, t.TempDir())

	prev := config.Default()
	defaultManager := config.NewManager()
	config.SetDefault(defaultManager)
	t.Cleanup(func() { config.SetDefault(prev) })

	app, customManager := newTestAppWithConfig(
		t,
		"config-manager-isolated",
		minimalV3Config("grpc"),
	)
	require.NoError(t, app.initializeLocked(context.Background()))

	assert.Same(t, customManager, app.opts.configManager)
	assert.Same(t, defaultManager, config.Default())
	assert.NotSame(t, customManager, config.Default())
}

func TestStopClosesManagedConfigSourcesOnce(t *testing.T) {
	withTestFlagSet(t)
	chdir(t, t.TempDir())

	var closeCount int32
	programmatic := &mockConfigSource{
		name:       "to-close",
		data:       map[string]any{},
		closeCount: &closeCount,
	}

	app := newTestApp(
		t,
		"close-sources",
		WithConfigSource("programmatic", config.PriorityOverride, programmatic),
	)
	require.NoError(t, app.initializeLocked(context.Background()))
	require.NoError(t, app.Stop(context.Background()))
	assert.Equal(t, int32(1), atomic.LoadInt32(&closeCount))
}

func TestValidateStartup_DoesNotFailForRuntimeResolvedBindings(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*configtest.T)
	}{
		{
			name: "strict missing tracer builder",
			configure: func(ct *configtest.T) {
				ct.Set("yggdrasil.admin.validation.strict", true)
				ct.Set("yggdrasil.observability.telemetry.tracer", "missing-tracer")
			},
		},
		{
			name: "non-strict missing tracer builder",
			configure: func(ct *configtest.T) {
				ct.Set("yggdrasil.admin.validation.enable", true)
				ct.Set("yggdrasil.admin.validation.strict", false)
				ct.Set("yggdrasil.observability.telemetry.tracer", "missing-tracer")
			},
		},
		{
			name: "strict missing stats handler builder",
			configure: func(ct *configtest.T) {
				ct.Set("yggdrasil.admin.validation.strict", true)
				ct.Set("yggdrasil.observability.stats.server", "missing-stats-handler")
			},
		},
		{
			name: "non-strict missing stats handler builder",
			configure: func(ct *configtest.T) {
				ct.Set("yggdrasil.admin.validation.enable", true)
				ct.Set("yggdrasil.admin.validation.strict", false)
				ct.Set("yggdrasil.observability.stats.client", "missing-stats-handler")
			},
		},
		{
			name: "strict missing rest marshaler builder",
			configure: func(ct *configtest.T) {
				ct.Set("yggdrasil.admin.validation.strict", true)
				ct.Set("yggdrasil.transports.http.rest.port", 0)
				ct.Set(
					"yggdrasil.transports.http.rest.marshaler.support",
					[]string{"nope"},
				)
			},
		},
		{
			name: "strict missing client interceptor global",
			configure: func(ct *configtest.T) {
				ct.Set("yggdrasil.admin.validation.strict", true)
				ct.Set("yggdrasil.clients.defaults.interceptors.unary", []string{"nope"})
			},
		},
		{
			name: "strict missing client interceptor by service",
			configure: func(ct *configtest.T) {
				ct.Set("yggdrasil.admin.validation.strict", true)
				ct.Set(
					"yggdrasil.clients.services.user.interceptors.unary",
					[]string{"nope"},
				)
			},
		},
		{
			name: "strict missing transport security provider",
			configure: func(ct *configtest.T) {
				ct.Set("yggdrasil.admin.validation.strict", true)
				ct.Set("yggdrasil.transports.grpc.server.security_profile", "custom")
				ct.Set("yggdrasil.transports.security.profiles.custom.type", "missing")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct := configtest.New(t)
			tt.configure(ct)
			opts := &options{configManager: ct.Manager()}
			require.NoError(t, validateStartup(opts))
		})
	}
}

func TestValidateStartup_Strict_FailsOnInvalidTLSSecurityProfileConfig(t *testing.T) {
	ct := configtest.New(t)
	ct.Set("yggdrasil.admin.validation.strict", true)
	ct.Set("yggdrasil.transports.grpc.server.security_profile", "tls-server")
	ct.Set("yggdrasil.transports.security.profiles.tls-server.type", "tls")
	ct.Set(
		"yggdrasil.transports.security.profiles.tls-server.config.server.cert_file",
		"/tmp/missing-cert.pem",
	)
	ct.Set(
		"yggdrasil.transports.security.profiles.tls-server.config.server.key_file",
		"/tmp/missing-key.pem",
	)

	opts := &options{configManager: ct.Manager()}
	require.Error(t, validateStartup(opts))
}

func TestInitializeLocked_FailsWhenDefaultLoggerHandlerBuilderIsMissingAtRuntime(t *testing.T) {
	manager := newTestManager(t, map[string]any{
		"yggdrasil": map[string]any{
			"observability": map[string]any{
				"logging": map[string]any{
					"handlers": map[string]any{
						"default": map[string]any{
							"type":   "missing",
							"writer": "default",
						},
					},
					"writers": map[string]any{
						"default": map[string]any{"type": "console"},
					},
				},
			},
		},
	})
	app := newTestApp(t, "missing-logger-handler", WithConfigManager(manager))
	require.Error(t, app.initializeLocked(context.Background()))
}

func TestInitializeLocked_FailsWhenServerTransportProviderIsMissingAtRuntime(t *testing.T) {
	manager := newTestManager(t, map[string]any{
		"yggdrasil": map[string]any{
			"admin": map[string]any{
				"governor": map[string]any{"port": 0},
			},
			"server": map[string]any{
				"transports": []any{"missing-server"},
			},
		},
	})
	app := newTestApp(t, "missing-server-provider", WithConfigManager(manager))
	require.Error(t, app.initializeLocked(context.Background()))
}

// --- addManagedConfigSource ---

func TestAddManagedConfigSource(t *testing.T) {
	t.Run("adds source", func(t *testing.T) {
		opts := &options{}
		src := memory.NewSource("test", map[string]any{"k": "v"})
		addManagedConfigSource(opts, src)
		assert.Len(t, opts.managedConfigSources, 1)
	})

	t.Run("nil item skip", func(t *testing.T) {
		opts := &options{}
		addManagedConfigSource(opts, nil)
		assert.Empty(t, opts.managedConfigSources)
	})

	t.Run("duplicate skip", func(t *testing.T) {
		opts := &options{}
		src := memory.NewSource("test", map[string]any{"k": "v"})
		addManagedConfigSource(opts, src)
		addManagedConfigSource(opts, src)
		assert.Len(t, opts.managedConfigSources, 1)
	})
}

// --- resolveIdentityLocked ---

func TestResolveIdentityLocked(t *testing.T) {
	t.Run("name from opts", func(t *testing.T) {
		app := &App{opts: &options{appName: "from-opts"}}
		err := app.resolveIdentityLocked()
		require.NoError(t, err)
		assert.Equal(t, "from-opts", app.name)
	})

	t.Run("name already set", func(t *testing.T) {
		app := &App{
			name: "already-set",
			opts: &options{},
		}
		err := app.resolveIdentityLocked()
		require.NoError(t, err)
		assert.Equal(t, "already-set", app.name)
	})

	t.Run("nil app returns error", func(t *testing.T) {
		var app *App
		err := app.resolveIdentityLocked()
		require.Error(t, err)
	})

	t.Run("nil opts returns error", func(t *testing.T) {
		app := &App{}
		err := app.resolveIdentityLocked()
		require.Error(t, err)
	})

	t.Run("missing name returns error", func(t *testing.T) {
		app := &App{opts: &options{}}
		err := app.resolveIdentityLocked()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "app name is required")
	})
}
