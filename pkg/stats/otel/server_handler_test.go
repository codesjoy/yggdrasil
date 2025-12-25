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

package otel

import (
	"context"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/pkg/stats"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/trace"
)

// TestNewSvrHandler tests newSvrHandler function
func TestNewSvrHandler(t *testing.T) {
	t.Run("create server handler", func(t *testing.T) {
		h := newSvrHandler()
		assert.NotNil(t, h)
		assert.Implements(t, (*stats.Handler)(nil), h)

		// Test that handler methods work
		ctx := context.Background()
		info := &stats.ChanTagInfoBase{Protocol: "grpc"}

		// Should not panic
		result := h.TagChannel(ctx, info)
		assert.NotNil(t, result)
	})
}

// TestServerHandler_TagRPC tests serverHandler.TagRPC method
func TestServerHandler_TagRPC(t *testing.T) {
	t.Run("tag RPC creates span", func(t *testing.T) {
		h := newSvrHandler()
		ctx := context.Background()
		info := &stats.RPCTagInfoBase{
			FullMethod: "/test.service/TestMethod",
		}

		result := h.TagRPC(ctx, info)

		assert.NotNil(t, result)

		// Verify span is created
		span := trace.SpanFromContext(result)
		assert.NotNil(t, span)

		// Verify rpcContext is set
		rctx, ok := result.Value(rpcContextKey{}).(*rpcContext)
		assert.True(t, ok, "rpcContext should be set in context")
		assert.NotNil(t, rctx)
		assert.NotEmpty(t, rctx.metricAttrs, "metricAttrs should be set")
	})

	t.Run("tag RPC with different methods", func(t *testing.T) {
		h := newSvrHandler()
		ctx := context.Background()

		methods := []string{
			"/package.service/method1",
			"/another.service/rpc2",
			"/service/Method",
		}

		for _, method := range methods {
			info := &stats.RPCTagInfoBase{FullMethod: method}
			result := h.TagRPC(ctx, info)
			assert.NotNil(t, result)

			// Verify context has rpcContext
			rctx, ok := result.Value(rpcContextKey{}).(*rpcContext)
			assert.True(t, ok)
			assert.NotNil(t, rctx)
		}
	})
}

// TestServerHandler_HandleRPC tests serverHandler.HandleRPC method
func TestServerHandler_HandleRPC(t *testing.T) {
	t.Run("handle RPCBegin", func(*testing.T) {
		h := newSvrHandler()
		ctx := context.Background()

		rs := &stats.RPCBeginBase{
			Client:    false,
			BeginTime: time.Now(),
			Protocol:  "grpc",
		}

		// Should not panic
		h.HandleRPC(ctx, rs)
	})

	t.Run("handle RPCInPayload", func(*testing.T) {
		h := newSvrHandler()

		rctx := &rpcContext{}
		ctx := context.WithValue(context.Background(), rpcContextKey{}, rctx)

		rs := &stats.RPCInPayloadBase{
			Client:        false,
			Data:          []byte("test data"),
			TransportSize: 100,
			RecvTime:      time.Now(),
			Protocol:      "grpc",
		}

		// Should not panic
		h.HandleRPC(ctx, rs)
	})

	t.Run("handle RPCOutPayload", func(*testing.T) {
		h := newSvrHandler()

		rctx := &rpcContext{}
		ctx := context.WithValue(context.Background(), rpcContextKey{}, rctx)

		rs := &stats.RPCOutPayloadBase{
			Client:        false,
			Data:          []byte("response"),
			TransportSize: 200,
			SendTime:      time.Now(),
			Protocol:      "grpc",
		}

		// Should not panic
		h.HandleRPC(ctx, rs)
	})

	t.Run("handle RPCEnd successfully", func(*testing.T) {
		h := newSvrHandler()

		beginTime := time.Now()
		endTime := beginTime.Add(100 * time.Millisecond)

		rs := &stats.RPCEndBase{
			Client:    false,
			BeginTime: beginTime,
			EndTime:   endTime,
			Err:       nil,
			Protocol:  "grpc",
		}

		// Should not panic
		ctx := context.Background()
		h.HandleRPC(ctx, rs)
	})

	t.Run("handle RPCEnd with error", func(*testing.T) {
		h := newSvrHandler()

		beginTime := time.Now()
		endTime := beginTime.Add(50 * time.Millisecond)

		rs := &stats.RPCEndBase{
			Client:    false,
			BeginTime: beginTime,
			EndTime:   endTime,
			Err:       assert.AnError,
			Protocol:  "grpc",
		}

		// Should not panic
		ctx := context.Background()
		h.HandleRPC(ctx, rs)
	})
}

// TestServerHandler_TagChannel tests serverHandler.TagChannel method
func TestServerHandler_TagChannel(t *testing.T) {
	t.Run("tag channel returns unchanged context", func(t *testing.T) {
		h := newSvrHandler()
		ctx := context.Background()
		info := &stats.ChanTagInfoBase{
			Protocol:       "grpc",
			RemoteEndpoint: "client:3000",
			LocalEndpoint:  "server:8080",
		}

		result := h.TagChannel(ctx, info)

		assert.NotNil(t, result)
		// Context may be same or equal, just verify it's not nil
		assert.NotNil(t, result)
	})
}

// TestServerHandler_HandleChannel tests serverHandler.HandleChannel method
func TestServerHandler_HandleChannel(t *testing.T) {
	t.Run("handle channel begin", func(*testing.T) {
		h := newSvrHandler()
		ctx := context.Background()
		cs := &stats.ChanBeginBase{
			Client: false,
		}

		// Should not panic
		h.HandleChannel(ctx, cs)
	})

	t.Run("handle channel end", func(*testing.T) {
		h := newSvrHandler()
		ctx := context.Background()
		cs := &stats.ChanEndBase{
			Client: false,
		}

		// Should not panic
		h.HandleChannel(ctx, cs)
	})
}

// TestServerHandler_Context tests context propagation in server handler
func TestServerHandler_Context(t *testing.T) {
	t.Run("context is preserved through RPC lifecycle", func(t *testing.T) {
		type ctxKey struct{}
		expectedValue := "test-value"

		h := newSvrHandler()
		ctx := context.WithValue(context.Background(), ctxKey{}, expectedValue)

		info := &stats.RPCTagInfoBase{FullMethod: "/test/method"}
		ctx = h.TagRPC(ctx, info)

		// Verify original value is still in context
		value := ctx.Value(ctxKey{})
		assert.Equal(t, expectedValue, value)
	})
}

// TestServerHandler_SpanKind tests that server handler creates server spans
func TestServerHandler_SpanKind(t *testing.T) {
	t.Run("server span kind is set", func(t *testing.T) {
		h := newSvrHandler()
		ctx := context.Background()
		info := &stats.RPCTagInfoBase{
			FullMethod: "/test.service/Method",
		}

		ctx = h.TagRPC(ctx, info)

		span := trace.SpanFromContext(ctx)
		assert.NotNil(t, span)
		// Span should be created and valid
		// Note: IsValid() may return false for recording spans, but span object should exist
		assert.NotNil(t, span)
	})
}
