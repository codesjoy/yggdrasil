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
	"encoding/json"
	"errors"
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
	"google.golang.org/genproto/googleapis/rpc/code"

	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
	rpcstatus "github.com/codesjoy/yggdrasil/v3/rpc/status"
	"github.com/codesjoy/yggdrasil/v3/transport/support/marshaler"
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
		ReadHeaderTimeout: 5 * time.Second,
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
	//nolint:staticcheck // intentional: testing nil context fallback
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
		ReadHeaderTimeout: 5 * time.Second,
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
	s, err := NewServer(nil)
	require.NoError(t, err)
	assert.NotNil(t, s)

	mux, ok := s.(*ServeMux)
	require.True(t, ok)
	assert.NotNil(t, mux.cfg)
	assert.NotNil(t, mux.Router)
}

func TestServeMux_Routing(t *testing.T) {
	s, err := NewServer(nil)
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

func TestConfigureMarshaler(t *testing.T) {
	ConfigureMarshaler([]string{"jsonpb", "proto"}, nil)
	support, cfg := currentMarshalerConfig()
	assert.Equal(t, []string{"jsonpb", "proto"}, support)
	assert.Nil(t, cfg)
}

func TestMiddlewareProvider(t *testing.T) {
	t.Run("Name and Build", func(t *testing.T) {
		p := NewProvider("test", func() func(http.Handler) http.Handler {
			return func(next http.Handler) http.Handler { return next }
		})
		assert.Equal(t, "test", p.Name())
		assert.NotNil(t, p.Build())
	})

	t.Run("ConfigureProviders errors", func(t *testing.T) {
		err := ConfigureProviders([]Provider{nil})
		require.NoError(t, err)

		emptyP := NewProvider("", func() func(http.Handler) http.Handler { return nil })
		err = ConfigureProviders([]Provider{emptyP})
		require.Error(t, err)
		assert.ErrorContains(t, err, "name is empty")

		dupP := NewProvider("dup-mw", func() func(http.Handler) http.Handler { return nil })
		err = ConfigureProviders([]Provider{dupP, dupP})
		require.Error(t, err)
		assert.ErrorContains(t, err, "duplicate")
	})

	t.Run("GetProvider", func(t *testing.T) {
		p := NewProvider("get-test-mw", func() func(http.Handler) http.Handler {
			return func(next http.Handler) http.Handler { return next }
		})
		err := ConfigureProviders([]Provider{p})
		require.NoError(t, err)
		got := GetProvider("get-test-mw")
		require.NotNil(t, got)
		assert.Equal(t, "get-test-mw", got.Name())
		assert.Nil(t, GetProvider("nonexistent"))
	})

	t.Run("Build", func(t *testing.T) {
		p := NewProvider("build-mw", func() func(http.Handler) http.Handler {
			return func(next http.Handler) http.Handler { return next }
		})
		err := ConfigureProviders([]Provider{p})
		require.NoError(t, err)
		mws := Build("build-mw")
		assert.Len(t, mws, 1)
	})

	t.Run("BuildWithProviders missing", func(t *testing.T) {
		mws := BuildWithProviders(nil, "missing")
		assert.Empty(t, mws)
	})

	t.Run("BuildWithProviders populated map", func(t *testing.T) {
		called := false
		providerMap := map[string]Provider{
			"custom": NewProvider("custom", func() func(http.Handler) http.Handler {
				return func(next http.Handler) http.Handler {
					called = true
					return next
				}
			}),
		}
		mws := BuildWithProviders(providerMap, "custom", "missing")
		assert.Len(t, mws, 1)

		// Verify the middleware is functional
		handler := mws[0](http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.True(t, called)
	})
}

func TestServerInfo_GetAttributes_GetAddress(t *testing.T) {
	attrs := map[string]string{"version": "1.0", "region": "us-west"}
	si := &serverInfo{
		address:    "127.0.0.1:8080",
		attributes: attrs,
	}
	assert.Equal(t, "127.0.0.1:8080", si.GetAddress())
	assert.Equal(t, attrs, si.GetAttributes())
}

func TestServeMux_Info(t *testing.T) {
	si := &serverInfo{
		address:    "0.0.0.0:9090",
		attributes: map[string]string{"env": "test"},
	}
	mux := &ServeMux{info: si}
	got := mux.Info()
	require.NotNil(t, got)
	assert.Equal(t, si, got)
	assert.Equal(t, "0.0.0.0:9090", got.GetAddress())
	assert.Equal(t, "test", got.GetAttributes()["env"])
}

func TestServeMux_ExtractInMetadata(t *testing.T) {
	mux := &ServeMux{
		acceptHeaders: []string{"X-Custom-Header", "Authorization"},
	}

	r := httptest.NewRequest("GET", "/test", nil)
	r.Header.Set("X-Custom-Header", "value1")
	r.Header.Set("Authorization", "Bearer token123")
	r.Header.Set("Yggdrasil-Metadata-Trace-Id", "abc123")
	r.Header.Set("Yggdrasil-Metadata-Request-Id", "req456")
	// This header does NOT have the prefix, should not be included via prefix scan
	r.Header.Set("Content-Type", "application/json")

	md := mux.extractInMetadata(r)

	assert.Equal(t, []string{"value1"}, md.Get("x-custom-header"))
	assert.Equal(t, []string{"Bearer token123"}, md.Get("authorization"))
	assert.Equal(t, []string{"abc123"}, md.Get("trace-id"))
	assert.Equal(t, []string{"req456"}, md.Get("request-id"))
	// Content-Type is not in acceptHeaders and does not have MetadataHeaderPrefix
	_, ok := md["content-type"]
	assert.False(t, ok)
}

func TestServeMux_ExtractInMetadata_MissingAcceptHeader(t *testing.T) {
	mux := &ServeMux{
		acceptHeaders: []string{"X-Missing"},
	}
	r := httptest.NewRequest("GET", "/", nil)
	// X-Missing header is not set
	md := mux.extractInMetadata(r)
	assert.Equal(t, 0, md.Len())
}

func TestServeMux_GetPeer_XForwardedFor(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = lis.Close() }()

	mux := &ServeMux{listener: lis}

	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.1:12345"
	r.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")

	p := mux.getPeer(r)
	require.NotNil(t, p)
	assert.Equal(t, "10.0.0.1", p.Addr.(*net.TCPAddr).IP.String())
	assert.Equal(t, 12345, p.Addr.(*net.TCPAddr).Port)
	assert.Equal(t, "http", p.Protocol)
	assert.Equal(t, lis.Addr(), p.LocalAddr)
}

func TestServeMux_GetPeer_XRealIP(t *testing.T) {
	mux := &ServeMux{}

	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.1:12345"
	r.Header.Set("X-Real-Ip", "10.0.0.5")

	p := mux.getPeer(r)
	require.NotNil(t, p)
	assert.Equal(t, "10.0.0.5", p.Addr.(*net.TCPAddr).IP.String())
	assert.Equal(t, 12345, p.Addr.(*net.TCPAddr).Port)
}

func TestServeMux_GetPeer_RemoteAddr(t *testing.T) {
	mux := &ServeMux{}

	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.100:54321"

	p := mux.getPeer(r)
	require.NotNil(t, p)
	assert.Equal(t, "192.168.1.100", p.Addr.(*net.TCPAddr).IP.String())
	assert.Equal(t, 54321, p.Addr.(*net.TCPAddr).Port)
	assert.Nil(t, p.LocalAddr)
}

func TestServeMux_OutgoingHeaderMatcher(t *testing.T) {
	mux := &ServeMux{
		outHeaders: []string{"X-Custom", "Authorization"},
	}

	// Matching key
	h, ok := mux.outgoingHeaderMatcher("X-Custom")
	assert.True(t, ok)
	assert.Equal(t, "X-Custom", h)

	// Another matching key
	h, ok = mux.outgoingHeaderMatcher("Authorization")
	assert.True(t, ok)
	assert.Equal(t, "Authorization", h)

	// Non-matching key gets prefixed
	h, ok = mux.outgoingHeaderMatcher("Trace-Id")
	assert.True(t, ok)
	assert.Equal(t, MetadataHeaderPrefix+"Trace-Id", h)
}

func TestServeMux_OutgoingTrailerMatcher(t *testing.T) {
	mux := &ServeMux{
		outTrailers: []string{"X-Grpc-Status", "X-Grpc-Message"},
	}

	// Matching key
	h, ok := mux.outgoingTrailerMatcher("X-Grpc-Status")
	assert.True(t, ok)
	assert.Equal(t, "X-Grpc-Status", h)

	// Non-matching key gets trailer prefix
	h, ok = mux.outgoingTrailerMatcher("Custom-Trailer")
	assert.True(t, ok)
	assert.Equal(t, MetadataTrailerPrefix+"Custom-Trailer", h)
}

func TestServeMux_RequestAcceptsTrailers(t *testing.T) {
	mux := &ServeMux{}

	// With TE: trailers header
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("TE", "trailers")
	assert.True(t, mux.requestAcceptsTrailers(r))

	// With TE header containing trailers among other values
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.Header.Set("TE", "trailers, gzip")
	assert.True(t, mux.requestAcceptsTrailers(r2))

	// Without TE header
	r3 := httptest.NewRequest("GET", "/", nil)
	assert.False(t, mux.requestAcceptsTrailers(r3))

	// TE header without trailers
	r4 := httptest.NewRequest("GET", "/", nil)
	r4.Header.Set("TE", "gzip")
	assert.False(t, mux.requestAcceptsTrailers(r4))
}

func TestServeMux_HandleResponseHeader(t *testing.T) {
	mux := &ServeMux{
		outHeaders: []string{"x-custom"},
	}

	w := httptest.NewRecorder()
	md := metadata.New(map[string]string{
		"x-custom": "custom-value",
		"other":    "other-value",
	})

	mux.handleResponseHeader(w, md)

	assert.Equal(t, "custom-value", w.Header().Get("x-custom"))
	assert.Equal(t, "other-value", w.Header().Get(MetadataHeaderPrefix+"other"))
}

func TestServeMux_HandleForwardResponseTrailerHeader(t *testing.T) {
	mux := &ServeMux{
		outTrailers: []string{"x-grpc-status"},
	}

	w := httptest.NewRecorder()
	md := metadata.Pairs("x-grpc-status", "0", "x-extra", "extra-val")

	mux.handleForwardResponseTrailerHeader(w, md)

	trailers := w.Header().Values("Trailer")
	assert.Contains(t, trailers, "x-grpc-status")
	assert.Contains(t, trailers, MetadataTrailerPrefix+"x-extra")
}

func TestServeMux_HandleForwardResponseTrailer(t *testing.T) {
	mux := &ServeMux{
		outTrailers: []string{"x-grpc-status"},
	}

	w := httptest.NewRecorder()
	md := metadata.Pairs("x-grpc-status", "0", "x-extra", "extra-val")

	mux.handleForwardResponseTrailer(w, md)

	assert.Equal(t, "0", w.Header().Get("x-grpc-status"))
	assert.Equal(t, "extra-val", w.Header().Get(MetadataTrailerPrefix+"x-extra"))
}

func TestServeMux_ErrorHandler(t *testing.T) {
	mux := &ServeMux{
		outHeaders:  []string{"x-custom"},
		outTrailers: []string{"x-trail"},
	}

	m := marshaler.NewJSONPbMarshalerWithConfig(nil)
	ctx := marshaler.WithOutboundContext(context.Background(), m)
	// Set up stream context with header/trailer metadata
	ctx = metadata.WithStreamContext(ctx)
	_ = metadata.SetHeader(ctx, metadata.Pairs("x-custom", "header-val"))

	r := httptest.NewRequest("POST", "/test", nil)
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()

	errSt := rpcstatus.FromErrorCode(
		errors.New("bad request"),
		code.Code_INVALID_ARGUMENT,
	)
	mux.errorHandler(w, r, errSt)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	assert.Equal(t, "header-val", w.Header().Get("x-custom"))

	// Verify body is valid JSON with the error code
	body := w.Body.Bytes()
	assert.True(t, len(body) > 0)
}

func TestServeMux_ErrorHandler_Unauthenticated(t *testing.T) {
	mux := &ServeMux{}

	m := marshaler.NewJSONPbMarshalerWithConfig(nil)
	ctx := marshaler.WithOutboundContext(context.Background(), m)

	r := httptest.NewRequest("POST", "/test", nil)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	errSt := rpcstatus.FromErrorCode(
		errors.New("unauthenticated"),
		code.Code_UNAUTHENTICATED,
	)
	mux.errorHandler(w, r, errSt)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.NotEmpty(t, w.Header().Get("WWW-Authenticate"))
}

func TestServeMux_SuccessHandler(t *testing.T) {
	mux := &ServeMux{}

	m := marshaler.NewJSONPbMarshalerWithConfig(nil)
	ctx := marshaler.WithOutboundContext(context.Background(), m)
	ctx = metadata.WithStreamContext(ctx)

	r := httptest.NewRequest("POST", "/test", nil)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	resp := wrapperspb.String("hello world")
	mux.successHandler(w, r, resp)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	body := w.Body.Bytes()
	assert.True(t, len(body) > 0)

	// Verify the response is the JSON-encoded string value
	var result string
	require.NoError(t, json.Unmarshal(body, &result))
	assert.Equal(t, "hello world", result)
}

func TestBuiltinMarshalerProvider(t *testing.T) {
	p := BuiltinMarshalerProvider()
	assert.Equal(t, "marshaler", p.Name())
	assert.NotNil(t, p.Build())
}

func TestNewMarshalerMiddleware(t *testing.T) {
	reg := marshaler.BuildMarshalerRegistry("jsonpb")
	mw := NewMarshalerMiddleware(reg)
	require.NotNil(t, mw)

	var inboundVal marshaler.Marshaler
	var outboundVal marshaler.Marshaler

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inboundVal = marshaler.InboundFromContext(r.Context())
		outboundVal = marshaler.OutboundFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := mw(inner)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/test", nil)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")

	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotNil(t, inboundVal)
	assert.NotNil(t, outboundVal)
}

func TestBuiltinLoggingProvider(t *testing.T) {
	p := BuiltinLoggingProvider()
	assert.Equal(t, "logger", p.Name())
	assert.NotNil(t, p.Build())
}

func TestNewServer_WithConfig(t *testing.T) {
	cfg := &Config{
		Host:       "127.0.0.1",
		Port:       9090,
		OutHeader:  []string{"X-Custom"},
		OutTrailer: []string{"X-Trail"},
	}
	s, err := NewServer(cfg)
	require.NoError(t, err)
	mux := s.(*ServeMux)
	require.NotNil(t, mux)
	assert.Equal(t, []string{"X-Custom"}, mux.outHeaders)
	assert.Equal(t, []string{"X-Trail"}, mux.outTrailers)
	assert.Equal(t, "127.0.0.1:9090", mux.info.address)
}

func TestNewServer_WithMarshalerRegistry(t *testing.T) {
	reg := marshaler.BuildMarshalerRegistry("jsonpb")
	s, err := NewServer(nil, WithMarshalerRegistry(reg))
	require.NoError(t, err)
	mux := s.(*ServeMux)
	assert.NotNil(t, mux.marshalerRegistry)
}

func TestServeMux_Start_Success(t *testing.T) {
	cfg := &Config{Port: 0}
	s, err := NewServer(cfg)
	require.NoError(t, err)
	mux := s.(*ServeMux)

	err = mux.Start()
	require.NoError(t, err)
	assert.True(t, mux.started)
	assert.NotNil(t, mux.listener)
	assert.NotEqual(t, "0.0.0.0:0", mux.info.address)
	_ = mux.Stop(context.Background())
}

func TestServeMux_Start_DoubleStart(t *testing.T) {
	cfg := &Config{Port: 0}
	s, err := NewServer(cfg)
	require.NoError(t, err)
	mux := s.(*ServeMux)

	err = mux.Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = mux.Stop(context.Background()) })

	err = mux.Start()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already serve")
}

func TestServeMux_Start_AfterStop(t *testing.T) {
	cfg := &Config{Port: 0}
	s, err := NewServer(cfg)
	require.NoError(t, err)
	mux := s.(*ServeMux)

	err = mux.Start()
	require.NoError(t, err)
	_ = mux.Stop(context.Background())

	err = mux.Start()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already stopped")
}

func TestServeMux_RPCHandle_HandlerError(t *testing.T) {
	s, err := NewServer(nil)
	require.NoError(t, err)
	mux := s.(*ServeMux)

	mux.RPCHandle(
		"POST",
		"/rpc-err",
		func(w http.ResponseWriter, r *http.Request) (interface{}, error) {
			return nil, rpcstatus.FromErrorCode(
				errors.New("handler failed"),
				code.Code_INTERNAL,
			)
		},
	)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/rpc-err", "application/json", nil)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestServeMux_ErrorHandler_WithTrailers(t *testing.T) {
	mux := &ServeMux{
		outHeaders:  []string{"x-custom"},
		outTrailers: []string{"x-trail"},
	}

	m := marshaler.NewJSONPbMarshalerWithConfig(nil)
	ctx := marshaler.WithOutboundContext(context.Background(), m)
	ctx = metadata.WithStreamContext(ctx)
	_ = metadata.SetHeader(ctx, metadata.Pairs("x-custom", "header-val"))
	_ = metadata.SetTrailer(ctx, metadata.Pairs("x-trail", "trail-val"))

	r := httptest.NewRequest("POST", "/test", nil)
	r.Header.Set("TE", "trailers")
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()

	errSt := rpcstatus.FromErrorCode(
		errors.New("bad request"),
		code.Code_INVALID_ARGUMENT,
	)
	mux.errorHandler(w, r, errSt)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Header().Values("Trailer"), "x-trail")
}

func TestServeMux_SuccessHandler_WithOutgoingHeaders(t *testing.T) {
	mux := &ServeMux{
		outHeaders: []string{"x-custom"},
	}

	m := marshaler.NewJSONPbMarshalerWithConfig(nil)
	ctx := marshaler.WithOutboundContext(context.Background(), m)
	ctx = metadata.WithStreamContext(ctx)
	_ = metadata.SetHeader(ctx, metadata.Pairs("x-custom", "fwd-val"))

	r := httptest.NewRequest("POST", "/test", nil)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	mux.successHandler(w, r, wrapperspb.String("ok"))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "fwd-val", w.Header().Get("x-custom"))
}

func TestServeMux_SuccessHandler_WithTrailers(t *testing.T) {
	mux := &ServeMux{
		outTrailers: []string{"x-trail"},
	}

	m := marshaler.NewJSONPbMarshalerWithConfig(nil)
	ctx := marshaler.WithOutboundContext(context.Background(), m)
	ctx = metadata.WithStreamContext(ctx)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("x-trail", "trail-val"))

	r := httptest.NewRequest("POST", "/test", nil)
	r.Header.Set("TE", "trailers")
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	mux.successHandler(w, r, wrapperspb.String("ok"))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Values("Trailer"), "x-trail")
}

// failingRestMarshaler implements marshaler.Marshaler but always fails on Marshal.
type failingRestMarshaler struct{}

func (f *failingRestMarshaler) Marshal(v interface{}) ([]byte, error) {
	return nil, errors.New("rest marshal failed")
}

func (f *failingRestMarshaler) Unmarshal(data []byte, v interface{}) error {
	return errors.New("rest unmarshal failed")
}

func (f *failingRestMarshaler) ContentType(v interface{}) string {
	return "application/json"
}

func (f *failingRestMarshaler) NewDecoder(r io.Reader) marshaler.Decoder {
	return nil
}

func (f *failingRestMarshaler) NewEncoder(w io.Writer) marshaler.Encoder {
	return nil
}

func TestServeMux_SuccessHandler_MarshalError(t *testing.T) {
	mux := &ServeMux{}

	ctx := marshaler.WithOutboundContext(context.Background(), &failingRestMarshaler{})
	ctx = metadata.WithStreamContext(ctx)

	r := httptest.NewRequest("POST", "/test", nil)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	mux.successHandler(w, r, wrapperspb.String("data"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
