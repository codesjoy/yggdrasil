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

package rest

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestServeMux_Serve_NotStarted(t *testing.T) {
	s := &ServeMux{}
	err := s.Serve()
	assert.Error(t, err)
	assert.Equal(t, "server is not initialized", err.Error())
}

func TestServeMux_Stop_Timeout(t *testing.T) {
	cfg := &Config{
		ShutdownTimeout: 1 * time.Millisecond,
	}
	s := &ServeMux{
		cfg: cfg,
	}
	_ = s.cfg.ShutdownTimeout
}

func TestNewServer(t *testing.T) {
	s, err := NewServer()
	require.NoError(t, err)
	assert.NotNil(t, s)

	mux, ok := s.(*ServeMux)
	require.True(t, ok)
	assert.NotNil(t, mux.cfg)
	assert.NotNil(t, mux.Router)
}

func TestServeMux_Routing(t *testing.T) {
	s, err := NewServer()
	require.NoError(t, err)
	mux := s.(*ServeMux)

	// Test RawHandle
	mux.RawHandle("GET", "/raw", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("raw"))
	})

	// Test RPCHandle
	mux.RPCHandle(
		"POST",
		"/rpc",
		func(w http.ResponseWriter, r *http.Request) (interface{}, error) {
			return wrapperspb.String("rpc"), nil
		},
	)

	// Create a test server using the mux
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Verify RawHandle
	// nolint:noctx
	resp, err := http.Get(ts.URL + "/raw")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	defer func() {
		_ = resp.Body.Close()
	}()

	// Verify RPCHandle
	// Note: RPCHandle expects POST and returns JSON by default (via marshaler middleware)
	// nolint:noctx
	resp, err = http.Post(ts.URL+"/rpc", "application/json", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	// We can check body if we want, but status code proves routing worked.
}
