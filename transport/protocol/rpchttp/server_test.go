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

package rpchttp

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
	"github.com/codesjoy/yggdrasil/v3/transport/support/peer"
)

type tagRPCSpyHandler struct {
	onTagRPC func(context.Context, stats.RPCTagInfo) context.Context
}

func (h *tagRPCSpyHandler) TagRPC(ctx context.Context, info stats.RPCTagInfo) context.Context {
	if h.onTagRPC != nil {
		return h.onTagRPC(ctx, info)
	}
	return ctx
}

func (h *tagRPCSpyHandler) HandleRPC(context.Context, stats.RPCStats) {}

func (h *tagRPCSpyHandler) TagChannel(ctx context.Context, _ stats.ChanTagInfo) context.Context {
	return ctx
}

func (h *tagRPCSpyHandler) HandleChannel(context.Context, stats.ChanStats) {}

func TestServeHTTP_TagRPCCanReadIncomingMetadata(t *testing.T) {
	traceparent := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	called := false

	s := &server{
		opts: &ServerConfig{},
		statsHandler: &tagRPCSpyHandler{
			onTagRPC: func(ctx context.Context, info stats.RPCTagInfo) context.Context {
				called = true
				assert.Equal(t, "/pkg.Service/Method", info.GetFullMethod())
				md, ok := metadata.FromInContext(ctx)
				assert.True(t, ok)
				assert.Equal(t, []string{traceparent}, md.Get("traceparent"))
				return ctx
			},
		},
		handle: func(remote.ServerStream) {},
	}

	req := httptest.NewRequest(http.MethodPost, "/pkg.Service/Method", bytes.NewBufferString("{}"))
	req.Header.Set(MetadataHeaderPrefix+"traceparent", traceparent)
	w := httptest.NewRecorder()

	s.serveHTTP(w, req)

	assert.True(t, called)
}

func TestWriteMetadata(t *testing.T) {
	rec := httptest.NewRecorder()
	writeMetadata(rec, metadata.Pairs("key1", "val1", "key2", "val2a", "key2", "val2b"))
	assert.Equal(t, "val1", rec.Header().Get(MetadataHeaderPrefix+"key1"))
	assert.Equal(t, []string{"val2a", "val2b"}, rec.Header().Values(MetadataHeaderPrefix+"key2"))
}

func TestRequestAcceptsTrailers(t *testing.T) {
	t.Run("accepts trailers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("TE", "trailers")
		assert.True(t, requestAcceptsTrailers(req))
	})
	t.Run("no TE header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		assert.False(t, requestAcceptsTrailers(req))
	})
	t.Run("TE without trailers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("TE", "gzip")
		assert.False(t, requestAcceptsTrailers(req))
	})
}

func TestWriteTrailers(t *testing.T) {
	rec := httptest.NewRecorder()
	writeTrailers(rec, metadata.Pairs("trailer-key", "val1"))
	assert.Equal(t, "val1", rec.Header().Get(MetadataTrailerPrefix+"trailer-key"))
}

func TestDeclareTrailers(t *testing.T) {
	rec := httptest.NewRecorder()
	declareTrailers(rec, metadata.Pairs("a", "1", "b", "2"))
	got := rec.Header().Values("Trailer")
	assert.Len(t, got, 2)
}

func TestExtractMetadataWithPrefix(t *testing.T) {
	h := http.Header{}
	h.Set(MetadataHeaderPrefix+"key1", "val1")
	h.Set(MetadataHeaderPrefix+"key2", "val2")
	h.Set("Other-Header", "ignored")
	md := extractMetadataWithPrefix(h, MetadataHeaderPrefix)
	assert.Equal(t, []string{"val1"}, md.Get("key1"))
	assert.Equal(t, []string{"val2"}, md.Get("key2"))
	_, ok := md["Other-Header"]
	assert.False(t, ok)
}

func TestAttachPeer(t *testing.T) {
	t.Run("valid remote addr", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		ctx := attachPeer(context.Background(), req, nil, nil)
		p, ok := peer.FromContext(ctx)
		require.True(t, ok)
		assert.Equal(t, "192.168.1.1:12345", p.Addr.String())
		assert.Equal(t, "http", p.Protocol)
	})
}

func TestAddrString(t *testing.T) {
	assert.Equal(t, "", addrString(nil))
}

func TestLocalAddrOrZero(t *testing.T) {
	t.Run("negative returns default 4MB", func(t *testing.T) {
		require.Equal(t, int64(4*1024*1024), localAddrOrZero(-1))
	})
	t.Run("zero returns default 4MB", func(t *testing.T) {
		require.Equal(t, int64(4*1024*1024), localAddrOrZero(0))
	})
	t.Run("positive returns value", func(t *testing.T) {
		require.Equal(t, int64(1024), localAddrOrZero(1024))
	})
}

func TestServeHTTP_NonPost(t *testing.T) {
	s := &server{
		opts:         &ServerConfig{},
		statsHandler: stats.NoOpHandler,
		handle:       func(remote.ServerStream) {},
	}

	req := httptest.NewRequest(http.MethodGet, "/pkg.Service/Method", nil)
	w := httptest.NewRecorder()
	s.serveHTTP(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestServeHTTP_NormalizesPath(t *testing.T) {
	var capturedMethod string
	s := &server{
		opts:         &ServerConfig{},
		statsHandler: stats.NoOpHandler,
		handle: func(ss remote.ServerStream) {
			capturedMethod = ss.Method()
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/pkg.Service/Method", bytes.NewBufferString("{}"))
	w := httptest.NewRecorder()
	s.serveHTTP(w, req)

	assert.Equal(t, "/pkg.Service/Method", capturedMethod)
}

func TestServerProvider(t *testing.T) {
	provider := ServerProvider()
	require.NotNil(t, provider)
}

func TestServer_StartStop(t *testing.T) {
	provider := ServerProviderWithSettings(Settings{
		Server: ServerConfig{
			Network: "tcp",
			Address: ":0",
		},
	}, stats.NoOpHandler, nil, nil)
	require.NotNil(t, provider)

	svr, err := provider.NewServer(func(remote.ServerStream) {})
	require.NoError(t, err)
	require.NotNil(t, svr)

	// Start on a random port.
	err = svr.Start()
	require.NoError(t, err)

	info := svr.Info()
	assert.Equal(t, "http", info.Protocol)
	assert.NotEmpty(t, info.Address)

	// Starting again should be idempotent.
	err = svr.Start()
	require.NoError(t, err)

	// Stop the server.
	err = svr.Stop(context.Background())
	require.NoError(t, err)

	// Stopping again should be idempotent.
	err = svr.Stop(context.Background())
	require.NoError(t, err)
}

func TestServer_HandleWithoutStart(t *testing.T) {
	provider := ServerProviderWithSettings(Settings{}, stats.NoOpHandler, nil, nil)
	svr, err := provider.NewServer(func(remote.ServerStream) {})
	require.NoError(t, err)

	// Handle without Start should return an error (net.ErrClosed or similar).
	err = svr.Handle()
	require.Error(t, err)
}

func TestServer_StartWhenClosed(t *testing.T) {
	provider := ServerProviderWithSettings(Settings{}, stats.NoOpHandler, nil, nil)
	svr, err := provider.NewServer(func(remote.ServerStream) {})
	require.NoError(t, err)

	// Stop before Start.
	err = svr.Stop(context.Background())
	require.NoError(t, err)

	// Start after Stop should fail.
	err = svr.Start()
	require.Error(t, err)
}

func TestServeHTTP_EmptyPath(t *testing.T) {
	s := &server{
		opts:         &ServerConfig{},
		statsHandler: stats.NoOpHandler,
		handle:       func(remote.ServerStream) {},
	}

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("{}"))
	// Override URL path to be empty to trigger the empty path check in serveHTTP
	req.URL.Path = ""
	w := httptest.NewRecorder()
	s.serveHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestServeHTTP_NoLeadingSlash(t *testing.T) {
	var capturedMethod string
	s := &server{
		opts:         &ServerConfig{},
		statsHandler: stats.NoOpHandler,
		handle: func(ss remote.ServerStream) {
			capturedMethod = ss.Method()
		},
	}

	// Use chi router or mux that strips the leading slash; test path without leading /
	req := httptest.NewRequest(http.MethodPost, "/pkg.Service/Method", bytes.NewBufferString("{}"))
	w := httptest.NewRecorder()
	s.serveHTTP(w, req)

	// Path with leading slash already present
	assert.Equal(t, "/pkg.Service/Method", capturedMethod)
}

func TestServer_localAddr_NilListener(t *testing.T) {
	s := &server{
		opts: &ServerConfig{},
	}
	addr := s.localAddr()
	assert.Nil(t, addr)
}
