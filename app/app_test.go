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

func TestNewClientTriggersInitialization(t *testing.T) {
	app := newTestApp(t, "client-init")

	_, err := app.NewClient("missing-service")
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

	_, err := app.NewClient("svc")
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
	app := newInitializedApp(t, "with-modules", WithModules(testExtraModule{name: "test.extra.module"}))
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

	require.NoError(t, manager.LoadLayer("test", config.PriorityOverride, memory.NewSource("test", minimalV3Config("http"))))
	require.NoError(t, app.Reload(context.Background()))

	state := app.hub.ReloadState()
	require.True(t, state.RestartRequired)

	diag := app.hub.Diagnostics()
	binding := findBindingDiag(t, diag, "transport.server.provider")
	require.Equal(t, []string{"http"}, binding.Requested)
	require.Equal(t, []string{"http"}, binding.Resolved)
}

func TestModuleHubDiagnosticsSchemaFileIsValid(t *testing.T) {
	data, err := os.ReadFile("../docs/module-hub-diagnostics.schema.json")
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(data, &doc))
	properties, ok := doc["properties"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, properties, "bindings")
	require.Contains(t, properties, "reload_state")
}
