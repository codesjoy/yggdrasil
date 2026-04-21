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
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

func TestServeMux_StartFailureThenStopDoesNotPanic(t *testing.T) {
	s := &ServeMux{
		cfg: &Config{
			ShutdownTimeout: 5 * time.Millisecond,
		},
		info: &serverInfo{
			address: "bad address",
		},
	}

	err := s.Start()
	require.Error(t, err)
	assert.False(t, s.started)
	assert.Nil(t, s.svr)
	assert.Nil(t, s.listener)
	assert.NotPanics(t, func() {
		require.NoError(t, s.Stop(context.Background()))
	})
}

func TestServeMux_StopUsesShutdownTimeoutWhenContextNil(t *testing.T) {
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseNow := func() {
		releaseOnce.Do(func() {
			close(release)
		})
	}
	requestStarted := make(chan struct{})
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	httpServer := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			select {
			case <-requestStarted:
			default:
				close(requestStarted)
			}
			<-release
			w.WriteHeader(http.StatusOK)
		}),
	}
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- httpServer.Serve(lis)
	}()
	t.Cleanup(func() {
		releaseNow()
		_ = httpServer.Close()
		select {
		case <-serveDone:
		case <-time.After(2 * time.Second):
			t.Fatal("http server did not stop")
		}
	})

	clientErr := make(chan error, 1)
	go func() {
		// nolint:noctx
		resp, reqErr := http.Get("http://" + lis.Addr().String() + "/")
		if resp != nil {
			_ = resp.Body.Close()
		}
		clientErr <- reqErr
	}()
	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not start")
	}

	mux := &ServeMux{
		cfg: &Config{
			ShutdownTimeout: 20 * time.Millisecond,
		},
		svr:      httpServer,
		listener: lis,
		started:  true,
	}
	start := time.Now()
	stopErr := mux.Stop(nil)
	elapsed := time.Since(start)

	require.Error(t, stopErr)
	assert.ErrorIs(t, stopErr, context.DeadlineExceeded)
	assert.GreaterOrEqual(t, elapsed, 20*time.Millisecond)
	assert.Less(t, elapsed, 350*time.Millisecond)

	releaseNow()
	select {
	case err = <-clientErr:
		_ = err
	case <-time.After(2 * time.Second):
		t.Fatal("client request did not exit")
	}
}

func TestServeMux_StopRespectsProvidedContext(t *testing.T) {
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseNow := func() {
		releaseOnce.Do(func() {
			close(release)
		})
	}
	requestStarted := make(chan struct{})
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	httpServer := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			select {
			case <-requestStarted:
			default:
				close(requestStarted)
			}
			<-release
			w.WriteHeader(http.StatusOK)
		}),
	}
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- httpServer.Serve(lis)
	}()
	t.Cleanup(func() {
		releaseNow()
		_ = httpServer.Close()
		select {
		case <-serveDone:
		case <-time.After(2 * time.Second):
			t.Fatal("http server did not stop")
		}
	})

	var clientWG sync.WaitGroup
	clientWG.Add(1)
	go func() {
		defer clientWG.Done()
		// nolint:noctx
		resp, reqErr := http.Get("http://" + lis.Addr().String() + "/")
		if resp != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		if reqErr != nil && !strings.Contains(reqErr.Error(), "Server closed") {
			t.Errorf("unexpected request error: %v", reqErr)
		}
	}()
	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not start")
	}

	mux := &ServeMux{
		cfg: &Config{
			ShutdownTimeout: 2 * time.Second,
		},
		svr:      httpServer,
		listener: lis,
		started:  true,
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stopErr := mux.Stop(ctx)

	require.Error(t, stopErr)
	assert.ErrorIs(t, stopErr, context.Canceled)

	releaseNow()
	clientWG.Wait()
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
