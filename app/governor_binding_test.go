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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
)

func TestInstallBusiness_GovernorHTTP(t *testing.T) {
	app := newGovernorBindingTestApp(t, map[string]any{"bind": "127.0.0.1", "port": 0})

	require.NoError(t, app.InstallBusiness(&BusinessBundle{
		GovernorHTTP: []GovernorHTTPBinding{{
			Method: http.MethodGet,
			Path:   "/hello",
			Handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("hi"))
			},
		}},
	}))

	serveGovernorAsync(t, app.opts.governor)
	waitGovernorStarted(t, app.opts.governor)

	base := "http://" + app.opts.governor.Info().Address

	resp, err := http.Get(base + "/hello")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	resp, err = http.Get(base + "/routes")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var routes []string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&routes))
	assert.Contains(t, routes, "/hello")
}

func TestInstallBusiness_GovernorHTTP_DisabledGovernor(t *testing.T) {
	app := newGovernorBindingTestApp(t, map[string]any{"enabled": false, "bind": "127.0.0.1"})

	require.NoError(t, app.InstallBusiness(&BusinessBundle{
		GovernorHTTP: []GovernorHTTPBinding{{
			Method:  http.MethodGet,
			Path:    "/hello",
			Handler: func(http.ResponseWriter, *http.Request) {},
		}},
	}))
}

func TestInstallGovernorHTTPBinding_Rejections(t *testing.T) {
	app := newGovernorBindingTestApp(t, map[string]any{"bind": "127.0.0.1", "port": 0})
	handler := func(http.ResponseWriter, *http.Request) {}

	t.Run("builtin path rejected", func(t *testing.T) {
		err := app.installGovernorHTTPBinding(GovernorHTTPBinding{
			Method:  http.MethodGet,
			Path:    "/configs",
			Handler: handler,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already installed")
	})

	t.Run("duplicate path rejected", func(t *testing.T) {
		require.NoError(t, app.installGovernorHTTPBinding(GovernorHTTPBinding{
			Method:  http.MethodGet,
			Path:    "/dup",
			Handler: handler,
		}))
		err := app.installGovernorHTTPBinding(GovernorHTTPBinding{
			Method:  http.MethodPost,
			Path:    "/dup",
			Handler: handler,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already installed")
	})

	t.Run("invalid binding rejected", func(t *testing.T) {
		err := app.installGovernorHTTPBinding(GovernorHTTPBinding{
			Method:  http.MethodGet,
			Path:    "no-slash",
			Handler: handler,
		})
		require.Error(t, err)
	})
}

// newGovernorBindingTestApp prepares an app whose governor config is supplied
// inline, so governor-route installation can be exercised without starting the
// full runtime.
func newGovernorBindingTestApp(t *testing.T, governorCfg map[string]any) *App {
	t.Helper()

	manager := newTestManager(t, map[string]any{
		"yggdrasil": map[string]any{
			"mode":   "prod-http-gateway",
			"admin":  map[string]any{"governor": governorCfg},
			"server": map[string]any{"transports": []any{"test"}},
		},
	})
	app, err := New(
		"governor-binding",
		WithConfigManager(manager),
		WithModules(testTransportModule{recorder: newTransportRecorder()}),
		WithPlanOverrides(yassembly.ForceDefault("observability.logger.handler", "text")),
	)
	require.NoError(t, err)
	require.NoError(t, app.Prepare(context.Background()))

	t.Cleanup(func() {
		_ = app.opts.governor.Stop()
		_ = app.Stop(context.Background())
	})
	return app
}
