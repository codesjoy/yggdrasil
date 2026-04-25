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

	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
	"github.com/codesjoy/yggdrasil/v3/remote"
	"github.com/codesjoy/yggdrasil/v3/observability/stats"
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
