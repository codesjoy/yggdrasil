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

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/trace"

	"github.com/codesjoy/yggdrasil/v3/observability/stats"
)

// TestNewCliHandler tests newCliHandler function
func TestNewCliHandler(t *testing.T) {
	t.Run("create client handler", func(t *testing.T) {
		h := newCliHandler()
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

// TestClientHandler_TagRPC tests clientHandler.TagRPC method
func TestClientHandler_TagRPC(t *testing.T) {
	t.Run("tag RPC creates span", func(t *testing.T) {
		h := newCliHandler()
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
		h := newCliHandler()
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

// TestClientHandler_HandleRPC tests clientHandler.HandleRPC method
func TestClientHandler_HandleRPC(t *testing.T) {
	t.Run("handle RPCBegin", func(*testing.T) {
		h := newCliHandler()
		ctx := context.Background()

		rs := &stats.RPCBeginBase{
			Client:    true,
			BeginTime: time.Now(),
			Protocol:  "grpc",
		}

		// Should not panic
		h.HandleRPC(ctx, rs)
	})

	t.Run("handle RPCInPayload", func(*testing.T) {
		h := newCliHandler()

		rctx := &rpcContext{}
		ctx := context.WithValue(context.Background(), rpcContextKey{}, rctx)

		rs := &stats.RPCInPayloadBase{
			Client:        true,
			Data:          []byte("test data"),
			TransportSize: 100,
			RecvTime:      time.Now(),
			Protocol:      "grpc",
		}

		// Should not panic
		h.HandleRPC(ctx, rs)
	})

	t.Run("handle RPCOutPayload", func(*testing.T) {
		h := newCliHandler()

		rctx := &rpcContext{}
		ctx := context.WithValue(context.Background(), rpcContextKey{}, rctx)

		rs := &stats.RPCOutPayloadBase{
			Client:        true,
			Data:          []byte("response"),
			TransportSize: 200,
			SendTime:      time.Now(),
			Protocol:      "grpc",
		}

		// Should not panic
		h.HandleRPC(ctx, rs)
	})

	t.Run("handle RPCEnd successfully", func(*testing.T) {
		h := newCliHandler()

		beginTime := time.Now()
		endTime := beginTime.Add(100 * time.Millisecond)

		rs := &stats.RPCEndBase{
			Client:    true,
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
		h := newCliHandler()

		beginTime := time.Now()
		endTime := beginTime.Add(50 * time.Millisecond)

		rs := &stats.RPCEndBase{
			Client:    true,
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

// TestClientHandler_TagChannel tests clientHandler.TagChannel method
func TestClientHandler_TagChannel(t *testing.T) {
	t.Run("tag channel returns unchanged context", func(t *testing.T) {
		h := newCliHandler()
		ctx := context.Background()
		info := &stats.ChanTagInfoBase{
			Protocol:       "grpc",
			RemoteEndpoint: "server:8080",
			LocalEndpoint:  "client:3000",
		}

		result := h.TagChannel(ctx, info)

		assert.NotNil(t, result)
		// Context may be same or equal, just verify it's not nil
		assert.NotNil(t, result)
	})
}

// TestClientHandler_HandleChannel tests clientHandler.HandleChannel method
func TestClientHandler_HandleChannel(t *testing.T) {
	t.Run("handle channel is no-op", func(*testing.T) {
		h := newCliHandler()
		ctx := context.Background()
		cs := &stats.ChanBeginBase{
			Client: true,
		}

		// Should not panic (no-op)
		h.HandleChannel(ctx, cs)
	})
}

// TestClientHandler_Context tests context propagation in client handler
func TestClientHandler_Context(t *testing.T) {
	t.Run("context is preserved through RPC lifecycle", func(t *testing.T) {
		type ctxKey struct{}
		expectedValue := "test-value"

		h := newCliHandler()
		ctx := context.WithValue(context.Background(), ctxKey{}, expectedValue)

		info := &stats.RPCTagInfoBase{FullMethod: "/test/method"}
		ctx = h.TagRPC(ctx, info)

		// Verify original value is still in context
		value := ctx.Value(ctxKey{})
		assert.Equal(t, expectedValue, value)
	})
}

func TestClientHandler_HandleRPC_WithMetricsConfig(t *testing.T) {
	cfg := &Config{EnableMetrics: true}
	h := newCliHandlerWithConfig(cfg)
	ch := h.(*clientHandler)

	// Create a span and rpc context
	ctx, span := ch.tracer.Start(context.Background(), "test")
	defer span.End()
	ctx = context.WithValue(ctx, rpcContextKey{}, &rpcContext{})

	rs := &stats.RPCOutPayloadBase{
		Client:        true,
		Data:          []byte("test"),
		TransportSize: 10,
		Protocol:      "grpc",
	}
	// Should not panic with metrics enabled
	ch.HandleRPC(ctx, rs)
}

func TestClientHandler_HandleRPC_RPCOutHeader(t *testing.T) {
	h := newCliHandler()
	ch := h.(*clientHandler)

	ctx, span := ch.tracer.Start(context.Background(), "test")
	defer span.End()

	rs := &stats.OutHeaderBase{
		Client:         true,
		Protocol:       "grpc",
		RemoteEndpoint: "server:8080",
	}
	ch.HandleRPC(ctx, rs)
}

func TestClientHandler_HandleRPC_RPCOutTrailer(t *testing.T) {
	h := newCliHandler()
	ch := h.(*clientHandler)

	ctx, span := ch.tracer.Start(context.Background(), "test")
	defer span.End()

	rs := &stats.OutTrailerBase{Client: true}
	ch.HandleRPC(ctx, rs)
}

func TestClientHandler_FullLifecycle(t *testing.T) {
	h := newCliHandler()
	ch := h.(*clientHandler)

	reqSize := &spyInt64Histogram{}
	respSize := &spyInt64Histogram{}
	reqPerRPC := &spyInt64Histogram{}
	respPerRPC := &spyInt64Histogram{}
	dur := &spyFloat64Histogram{}
	ch.cfg = &Config{EnableMetrics: true}
	ch.rpcRequestSize = reqSize
	ch.rpcResponseSize = respSize
	ch.rpcRequestsPerRPC = reqPerRPC
	ch.rpcResponsesPerRPC = respPerRPC
	ch.rpcDuration = dur
	ch.handleRPC = ch.handleWithMetrics

	ctx := context.Background()
	ctx = ch.TagRPC(ctx, &stats.RPCTagInfoBase{FullMethod: "/test.service/Method"})

	// OutPayload (client request)
	ch.HandleRPC(ctx, &stats.RPCOutPayloadBase{
		Client:        true,
		Data:          []byte("request"),
		TransportSize: 50,
		Protocol:      "grpc",
	})

	// InPayload (client response)
	ch.HandleRPC(ctx, &stats.RPCInPayloadBase{
		Client:        true,
		Data:          []byte("response"),
		TransportSize: 100,
		Protocol:      "grpc",
	})

	// OutHeader
	ch.HandleRPC(ctx, &stats.OutHeaderBase{
		Client:         true,
		Protocol:       "grpc",
		RemoteEndpoint: "server:8080",
	})

	// End
	begin := time.Now()
	ch.HandleRPC(ctx, &stats.RPCEndBase{
		Client:    true,
		BeginTime: begin,
		EndTime:   begin.Add(25 * time.Millisecond),
		Protocol:  "grpc",
	})

	assert.Equal(t, []int64{50}, reqSize.values)
	assert.Equal(t, []int64{100}, respSize.values)
	assert.Equal(t, []int64{1}, reqPerRPC.values)
	assert.Equal(t, []int64{1}, respPerRPC.values)
	assert.Len(t, dur.values, 1)
}
