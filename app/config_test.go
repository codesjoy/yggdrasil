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

	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/internal/configtest"
	_ "github.com/codesjoy/yggdrasil/v3/remote/credentials/tls"
)

func TestResolveConfigPath(t *testing.T) {
	withTestFlagSet(t)

	path, explicit := resolveConfigPath("")
	assert.Equal(t, defaultConfigPath, path)
	assert.False(t, explicit)

	configured := filepath.Join(t.TempDir(), "custom.yaml")
	path, explicit = resolveConfigPath(configured)
	assert.Equal(t, configured, path)
	assert.True(t, explicit)

	explicitPath := filepath.Join(t.TempDir(), "explicit.yaml")
	os.Args = []string{"yggdrasil-test", "--yggdrasil-config=" + explicitPath}
	path, explicit = resolveConfigPath(configured)
	assert.Equal(t, explicitPath, path)
	assert.True(t, explicit)
}

func TestInitLoadsConfigAndOptionSources(t *testing.T) {
	withTestFlagSet(t)
	manager := config.NewManager()
	config.SetDefault(manager)

	dir := t.TempDir()
	chdir(t, dir)
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "config.yaml"),
		[]byte("app:\n  startup_order: bootstrap\nyggdrasil:\n  config:\n    sources:\n      - kind: file\n        config:\n          path: ./config.override.yaml\n"),
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

func TestStopClosesManagedConfigSourcesOnce(t *testing.T) {
	withTestFlagSet(t)
	chdir(t, t.TempDir())

	var closeCount int32
	programmatic := &mockConfigSource{
		name:       "to-close",
		data:       map[string]any{},
		closeCount: &closeCount,
	}

	app := newTestApp(t, "close-sources", WithConfigSource("programmatic", config.PriorityOverride, programmatic))
	require.NoError(t, app.initializeLocked(context.Background()))
	require.NoError(t, app.Stop(context.Background()))
	assert.Equal(t, int32(1), atomic.LoadInt32(&closeCount))
}

func TestValidateStartup_DoesNotFailForRuntimeResolvedBindings(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*testing.T)
	}{
		{
			name: "strict missing tracer builder",
			configure: func(t *testing.T) {
				configtest.Set(t, "yggdrasil.admin.validation.strict", true)
				configtest.Set(t, "yggdrasil.telemetry.tracer", "missing-tracer")
			},
		},
		{
			name: "non-strict missing tracer builder",
			configure: func(t *testing.T) {
				configtest.Set(t, "yggdrasil.admin.validation.enable", true)
				configtest.Set(t, "yggdrasil.admin.validation.strict", false)
				configtest.Set(t, "yggdrasil.telemetry.tracer", "missing-tracer")
			},
		},
		{
			name: "strict missing stats handler builder",
			configure: func(t *testing.T) {
				configtest.Set(t, "yggdrasil.admin.validation.strict", true)
				configtest.Set(t, "yggdrasil.telemetry.stats.server", "missing-stats-handler")
			},
		},
		{
			name: "non-strict missing stats handler builder",
			configure: func(t *testing.T) {
				configtest.Set(t, "yggdrasil.admin.validation.enable", true)
				configtest.Set(t, "yggdrasil.admin.validation.strict", false)
				configtest.Set(t, "yggdrasil.telemetry.stats.client", "missing-stats-handler")
			},
		},
		{
			name: "strict missing rest marshaler builder",
			configure: func(t *testing.T) {
				configtest.Set(t, "yggdrasil.admin.validation.strict", true)
				configtest.Set(t, "yggdrasil.transports.http.rest.port", 0)
				configtest.Set(t, "yggdrasil.transports.http.rest.marshaler.support", []string{"nope"})
			},
		},
		{
			name: "strict missing client interceptor global",
			configure: func(t *testing.T) {
				configtest.Set(t, "yggdrasil.admin.validation.strict", true)
				configtest.Set(t, "yggdrasil.clients.defaults.interceptors.unary", []string{"nope"})
			},
		},
		{
			name: "strict missing client interceptor by service",
			configure: func(t *testing.T) {
				configtest.Set(t, "yggdrasil.admin.validation.strict", true)
				configtest.Set(t, "yggdrasil.clients.services.user.interceptors.unary", []string{"nope"})
			},
		},
		{
			name: "strict missing remote credentials builder",
			configure: func(t *testing.T) {
				configtest.Set(t, "yggdrasil.admin.validation.strict", true)
				configtest.Set(t, "yggdrasil.transports.grpc.server.creds_proto", "missing")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.configure(t)
			require.NoError(t, validateStartup(nil))
		})
	}
}

func TestValidateStartup_Strict_FailsOnInvalidTLSCredentialsConfig(t *testing.T) {
	configtest.Set(t, "yggdrasil.admin.validation.strict", true)
	configtest.Set(t, "yggdrasil.transports.grpc.server.creds_proto", "tls")
	configtest.Set(t, "yggdrasil.transports.grpc.credentials.tls.server.cert_file", "/tmp/missing-cert.pem")
	configtest.Set(t, "yggdrasil.transports.grpc.credentials.tls.server.key_file", "/tmp/missing-key.pem")

	require.Error(t, validateStartup(nil))
}

func TestInitializeLocked_FailsWhenDefaultLoggerHandlerBuilderIsMissingAtRuntime(t *testing.T) {
	manager := newTestManager(t, map[string]any{
		"yggdrasil": map[string]any{
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
