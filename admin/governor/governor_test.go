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

package governor

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/config"
)

func TestConfig_SetDefault(t *testing.T) {
	cfg := Config{}
	require.NoError(t, cfg.SetDefault())
	require.NotNil(t, cfg.Enabled)
	assert.True(t, *cfg.Enabled)
	assert.Equal(t, "127.0.0.1", cfg.Bind)
	assert.False(t, cfg.ExposePprof)
	assert.False(t, cfg.ExposeEnv)
	assert.False(t, cfg.AllowConfigPatch)
	assert.False(t, cfg.Advertise)
}

func TestConfig_SetDefault_ValidateAuth(t *testing.T) {
	cfg := Config{
		Auth: AuthConfig{
			Token: "token",
			Basic: BasicAuthConfig{Username: "u", Password: "p"},
		},
	}
	err := cfg.SetDefault()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be configured together")
}

func TestNewServerDoesNotListenUntilServe(t *testing.T) {
	port := mustAllocPort(t)
	cfg := Config{Bind: "127.0.0.1", Port: uint64(port)}
	s, err := NewServerWithConfig(cfg, config.NewManager())
	require.NoError(t, err)
	require.NotNil(t, s)

	probe, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	require.NoError(t, err)
	_ = probe.Close()
}

func TestServerServeAndStop(t *testing.T) {
	s := startGovernor(t, Config{}, config.NewManager())

	resp, err := http.Get("http://" + s.Info().Address + "/routes")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	require.NoError(t, s.Stop())
	require.NoError(t, s.Stop())
}

func TestSecurityBaseline_DefaultRoutes(t *testing.T) {
	s := startGovernor(t, Config{}, config.NewManager())

	assertStatus(t, "GET", "http://"+s.Info().Address+"/env", "", nil, http.StatusNotFound)
	assertStatus(t, "GET", "http://"+s.Info().Address+"/debug/pprof/", "", nil, http.StatusNotFound)
	assertStatus(
		t,
		"POST",
		"http://"+s.Info().Address+"/configs",
		`{"paths":[["test","key"]],"data":["value"]}`,
		map[string]string{"Content-Type": "application/json"},
		http.StatusForbidden,
	)
}

func TestConfigHandle_MethodNotAllowed(t *testing.T) {
	s := startGovernor(t, Config{}, config.NewManager())
	assertStatus(t, "DELETE", "http://"+s.Info().Address+"/configs", "", nil, http.StatusMethodNotAllowed)
}

func TestConfigPatchValidation(t *testing.T) {
	s := startGovernor(t, Config{AllowConfigPatch: true}, config.NewManager())

	assertStatus(
		t,
		"POST",
		"http://"+s.Info().Address+"/configs",
		`{"paths":[["a"]],"data":[]}`,
		map[string]string{"Content-Type": "application/json"},
		http.StatusBadRequest,
	)
	assertStatus(
		t,
		"POST",
		"http://"+s.Info().Address+"/configs",
		`{"paths":[[]],"data":[1]}`,
		map[string]string{"Content-Type": "application/json"},
		http.StatusBadRequest,
	)
	assertStatus(
		t,
		"POST",
		"http://"+s.Info().Address+"/configs",
		`{"paths":[["","b"]],"data":[1]}`,
		map[string]string{"Content-Type": "application/json"},
		http.StatusBadRequest,
	)
}

func TestConfigPatchSuccess(t *testing.T) {
	manager := config.NewManager()
	s := startGovernor(t, Config{AllowConfigPatch: true}, manager)

	assertStatus(
		t,
		"POST",
		"http://"+s.Info().Address+"/configs",
		`{"paths":[["app","flag"]],"data":[true]}`,
		map[string]string{"Content-Type": "application/json"},
		http.StatusNoContent,
	)

	resp, err := http.Get("http://" + s.Info().Address + "/configs")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	assert.Equal(t, map[string]any{"flag": true}, payload["app"])
}

func TestAuthToken(t *testing.T) {
	s := startGovernor(t, Config{Auth: AuthConfig{Token: "secret"}}, config.NewManager())

	assertStatus(t, "GET", "http://"+s.Info().Address+"/routes", "", nil, http.StatusUnauthorized)
	assertStatus(
		t,
		"GET",
		"http://"+s.Info().Address+"/routes",
		"",
		map[string]string{"Authorization": "Bearer wrong"},
		http.StatusUnauthorized,
	)
	assertStatus(
		t,
		"GET",
		"http://"+s.Info().Address+"/routes",
		"",
		map[string]string{"Authorization": "Bearer secret"},
		http.StatusOK,
	)
}

func TestAuthBasic(t *testing.T) {
	s := startGovernor(t, Config{
		Auth: AuthConfig{
			Basic: BasicAuthConfig{
				Username: "admin",
				Password: "password",
			},
		},
	}, config.NewManager())

	assertStatus(t, "GET", "http://"+s.Info().Address+"/routes", "", nil, http.StatusUnauthorized)
	assertStatus(
		t,
		"GET",
		"http://"+s.Info().Address+"/routes",
		"",
		map[string]string{"Authorization": "Basic Zm9vOmJhcg=="},
		http.StatusUnauthorized,
	)
	assertStatus(
		t,
		"GET",
		"http://"+s.Info().Address+"/routes",
		"",
		map[string]string{"Authorization": "Basic YWRtaW46cGFzc3dvcmQ="},
		http.StatusOK,
	)
}

func TestWarnOnExposedRoutesWithoutAuth(t *testing.T) {
	var logBuf bytes.Buffer
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() {
		slog.SetDefault(oldLogger)
	})

	s := startGovernor(t, Config{
		ExposePprof:      true,
		ExposeEnv:        true,
		AllowConfigPatch: true,
	}, config.NewManager())
	require.NotNil(t, s)

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "governor high-risk routes are exposed without authentication")
	assert.Contains(t, logOutput, "pprof")
	assert.Contains(t, logOutput, "env")
	assert.Contains(t, logOutput, "config_patch")
}

func TestCompatibilityHandleFunc(t *testing.T) {
	s, err := NewServerWithConfig(Config{}, config.NewManager())
	require.NoError(t, err)
	HandleFunc("/compat", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	errCh := make(chan error, 1)
	go func() { errCh <- s.Serve() }()
	waitCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	require.NoError(t, s.WaitStarted(waitCtx))
	t.Cleanup(func() {
		_ = s.Stop()
		select {
		case err := <-errCh:
			assert.NoError(t, err)
		case <-time.After(2 * time.Second):
			t.Fatal("governor serve goroutine did not exit")
		}
	})

	resp, err := http.Get("http://" + s.Info().Address + "/compat")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestWaitStartedReturnsServeError(t *testing.T) {
	s, err := NewServerWithConfig(Config{Bind: "999.999.999.999"}, config.NewManager())
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() { done <- s.Serve() }()
	waitCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	require.Error(t, s.WaitStarted(waitCtx))
	require.Error(t, <-done)
}

func startGovernor(t *testing.T, cfg Config, manager *config.Manager) *Server {
	t.Helper()
	s, err := NewServerWithConfig(cfg, manager)
	require.NoError(t, err)

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Serve()
	}()
	waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, s.WaitStarted(waitCtx))

	t.Cleanup(func() {
		_ = s.Stop()
		select {
		case err := <-errCh:
			assert.NoError(t, err)
		case <-time.After(2 * time.Second):
			t.Fatal("governor serve goroutine did not exit")
		}
	})
	return s
}

func assertStatus(
	t *testing.T,
	method string,
	url string,
	body string,
	headers map[string]string,
	expected int,
) {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	require.NoError(t, err)
	for key, val := range headers {
		req.Header.Set(key, val)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, expected, resp.StatusCode)
}

func mustAllocPort(t *testing.T) int {
	t.Helper()
	lis, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = lis.Close() }()
	_, port, err := net.SplitHostPort(lis.Addr().String())
	require.NoError(t, err)
	value, err := strconv.Atoi(port)
	require.NoError(t, err)
	return value
}
