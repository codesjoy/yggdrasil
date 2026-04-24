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
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
)

func TestStatsOtelAutoSpecEnablesRealModule(t *testing.T) {
	manager := newTestManager(t, map[string]any{
		"yggdrasil": map[string]any{
			"admin": map[string]any{
				"governor": map[string]any{"port": 0},
			},
			"telemetry": map[string]any{
				"stats": map[string]any{
					"server": "otel",
				},
			},
		},
	})
	app, err := Open(
		WithConfigManager(manager),
		WithAppName("stats-auto"),
	)
	require.NoError(t, err)
	require.NoError(t, app.Prepare(context.Background()))
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	require.Contains(t, app.lastPlanResult.CapabilityBindings["stats.handler"], "otel")
	require.Contains(t, app.lastPlanResult.AffectedPathsByDomain["modules"], "yggdrasil.telemetry.stats.server")
	require.True(t, containsPlannedModule(app.assemblySpec, "telemetry.stats.otel"))
}

func TestDiagnosticsEndpointIncludesAssemblyState(t *testing.T) {
	recorder := newTransportRecorder()
	manager := newTestManager(t, map[string]any{
		"yggdrasil": map[string]any{
			"mode": "prod-http-gateway",
			"admin": map[string]any{
				"governor": map[string]any{"port": 0},
			},
			"server": map[string]any{
				"transports": []any{"test"},
			},
			"transports": map[string]any{
				"http": map[string]any{
					"rest": map[string]any{
						"host": "127.0.0.1",
						"port": 0,
					},
				},
			},
		},
	})
	app, err := Open(
		WithConfigManager(manager),
		WithAppName("diagnostics-assembly"),
		WithModules(testTransportModule{recorder: recorder}),
		WithPlanOverrides(yassembly.ForceDefault("logger.handler", "text")),
	)
	require.NoError(t, err)
	require.NoError(t, app.ComposeAndInstall(context.Background(), func(Runtime) (*BusinessBundle, error) {
		return &BusinessBundle{
			Diagnostics: []BundleDiag{
				{
					Code:    string(yassembly.ErrComposeLocalResourceLeaked),
					Message: "local resource left outside managed bundle scope",
				},
			},
		}, nil
	}))
	t.Cleanup(func() {
		_ = app.opts.governor.Stop()
		_ = app.Stop(context.Background())
	})

	errCh := serveGovernorAsync(t, app.opts.governor)
	waitGovernorStarted(t, app.opts.governor)

	resp, err := http.Get("http://" + app.opts.governor.Info().Address + "/diagnostics")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var doc struct {
		Assembly struct {
			CurrentSpecHash string `json:"current_spec_hash"`
			Mode            struct {
				Name string `json:"name"`
			} `json:"mode"`
			SelectedDefaults map[string]struct {
				Value  string `json:"value"`
				Source string `json:"source"`
			} `json:"selected_defaults"`
			DefaultCandidates map[string][]struct {
				Provider string `json:"provider"`
			} `json:"default_candidates"`
			BusinessInputPaths []string `json:"business_input_paths"`
			BundleDiagnostics  []struct {
				Code string `json:"code"`
			} `json:"bundle_diagnostics"`
		} `json:"assembly"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&doc))
	require.Equal(t, "prod-http-gateway", doc.Assembly.Mode.Name)
	require.NotEmpty(t, doc.Assembly.CurrentSpecHash)
	require.Equal(t, "text", doc.Assembly.SelectedDefaults["logger.handler"].Value)
	require.Equal(t, "code_override", doc.Assembly.SelectedDefaults["logger.handler"].Source)
	require.NotEmpty(t, doc.Assembly.BusinessInputPaths)
	require.Equal(t, string(yassembly.ErrComposeLocalResourceLeaked), doc.Assembly.BundleDiagnostics[0].Code)

	require.NoError(t, app.opts.governor.Stop())
	requireAsyncNoError(t, errCh, "governor serve goroutine did not exit")
}
