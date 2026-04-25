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

	"github.com/codesjoy/pkg/basic/xerror"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"google.golang.org/genproto/googleapis/rpc/code"

	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	"github.com/codesjoy/yggdrasil/v3/rpc/status"
)

type recordedInt64Histogram struct {
	noop.Int64Histogram
	values []int64
}

func (h *recordedInt64Histogram) Record(context.Context, int64, ...metric.RecordOption) {}

func (h *recordedInt64Histogram) record(value int64) {
	h.values = append(h.values, value)
}

type spyInt64Histogram struct {
	recordedInt64Histogram
}

func (h *spyInt64Histogram) Record(_ context.Context, value int64, _ ...metric.RecordOption) {
	h.record(value)
}

type spyFloat64Histogram struct {
	noop.Float64Histogram
	values []float64
}

func (h *spyFloat64Histogram) Record(_ context.Context, value float64, _ ...metric.RecordOption) {
	h.values = append(h.values, value)
}

// TestNewHandler tests newHandler function
func TestNewHandler(t *testing.T) {
	t.Run("create server handler", func(t *testing.T) {
		h := newHandler(true)
		assert.NotNil(t, h)
		assert.NotNil(t, h.tracer)
	})

	t.Run("create client handler", func(t *testing.T) {
		h := newHandler(false)
		assert.NotNil(t, h)
		assert.NotNil(t, h.tracer)
	})
}

// TestHandleWithOutMetrics tests handleWithOutMetrics function
func TestHandleWithOutMetrics(t *testing.T) {
	t.Run("handle RPCBegin", func(*testing.T) {
		h := newHandler(false)
		ctx := context.Background()
		rs := &stats.RPCBeginBase{
			Client:    true,
			BeginTime: time.Now(),
			Protocol:  "grpc",
		}

		// Should not panic
		h.handleWithOutMetrics(ctx, rs, false)
	})

	t.Run("handle RPCEnd", func(*testing.T) {
		h := newHandler(true)
		ctx := context.Background()

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
		h.handleWithOutMetrics(ctx, rs, true)
	})

	t.Run("handle RPCEnd with error", func(*testing.T) {
		h := newHandler(false)
		ctx := context.Background()

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
		h.handleWithOutMetrics(ctx, rs, false)
	})
}

// TestHandleWithMetrics tests handleWithMetrics function
func TestHandleWithMetrics(t *testing.T) {
	t.Run("handle RPCInPayload", func(*testing.T) {
		cfg := &Config{EnableMetrics: true}
		h := newHandler(true)
		h.cfg = cfg

		ctx := context.Background()

		// Create a span for the test
		ctx, span := h.tracer.Start(ctx, "test")
		defer span.End()

		rctx := &rpcContext{}
		ctx = context.WithValue(ctx, rpcContextKey{}, rctx)

		rs := &stats.RPCInPayloadBase{
			Client:        true,
			Data:          []byte("test data"),
			TransportSize: 50,
			RecvTime:      time.Now(),
			Protocol:      "grpc",
		}

		// Should not panic
		h.handleWithMetrics(ctx, rs, true)
	})

	t.Run("handle RPCOutPayload", func(*testing.T) {
		cfg := &Config{EnableMetrics: true}
		h := newHandler(false)
		h.cfg = cfg

		ctx := context.Background()

		// Create a span for the test
		ctx, span := h.tracer.Start(ctx, "test")
		defer span.End()

		rctx := &rpcContext{}
		ctx = context.WithValue(ctx, rpcContextKey{}, rctx)

		rs := &stats.RPCOutPayloadBase{
			Client:        false,
			Data:          []byte("response"),
			TransportSize: 100,
			SendTime:      time.Now(),
			Protocol:      "grpc",
		}

		// Should not panic
		h.handleWithMetrics(ctx, rs, false)
	})
}

func TestHandleWithMetrics_RequestResponseSemantics(t *testing.T) {
	t.Run("server side payload direction", func(t *testing.T) {
		h := newHandler(true)
		h.cfg = &Config{EnableMetrics: true}
		reqSize := &spyInt64Histogram{}
		respSize := &spyInt64Histogram{}
		reqPerRPC := &spyInt64Histogram{}
		respPerRPC := &spyInt64Histogram{}
		dur := &spyFloat64Histogram{}
		h.rpcRequestSize = reqSize
		h.rpcResponseSize = respSize
		h.rpcRequestsPerRPC = reqPerRPC
		h.rpcResponsesPerRPC = respPerRPC
		h.rpcDuration = dur

		ctx, span := h.tracer.Start(context.Background(), "server")
		defer span.End()
		ctx = context.WithValue(ctx, rpcContextKey{}, &rpcContext{})

		h.handleWithMetrics(ctx, &stats.RPCInPayloadBase{
			Client:        false,
			TransportSize: 12,
			Protocol:      "grpc",
		}, true)
		h.handleWithMetrics(ctx, &stats.RPCOutPayloadBase{
			Client:        false,
			TransportSize: 34,
			Protocol:      "grpc",
		}, true)
		begin := time.Now()
		h.handleWithMetrics(ctx, &stats.RPCEndBase{
			Client:    false,
			BeginTime: begin,
			EndTime:   begin.Add(25 * time.Millisecond),
			Protocol:  "grpc",
		}, true)

		assert.Equal(t, []int64{12}, reqSize.values)
		assert.Equal(t, []int64{34}, respSize.values)
		assert.Equal(t, []int64{1}, reqPerRPC.values)
		assert.Equal(t, []int64{1}, respPerRPC.values)
		assert.Len(t, dur.values, 1)
	})

	t.Run("client side payload direction", func(t *testing.T) {
		h := newHandler(false)
		h.cfg = &Config{EnableMetrics: true}
		reqSize := &spyInt64Histogram{}
		respSize := &spyInt64Histogram{}
		reqPerRPC := &spyInt64Histogram{}
		respPerRPC := &spyInt64Histogram{}
		dur := &spyFloat64Histogram{}
		h.rpcRequestSize = reqSize
		h.rpcResponseSize = respSize
		h.rpcRequestsPerRPC = reqPerRPC
		h.rpcResponsesPerRPC = respPerRPC
		h.rpcDuration = dur

		ctx, span := h.tracer.Start(context.Background(), "client")
		defer span.End()
		ctx = context.WithValue(ctx, rpcContextKey{}, &rpcContext{})

		h.handleWithMetrics(ctx, &stats.RPCOutPayloadBase{
			Client:        true,
			TransportSize: 56,
			Protocol:      "grpc",
		}, false)
		h.handleWithMetrics(ctx, &stats.RPCInPayloadBase{
			Client:        true,
			TransportSize: 78,
			Protocol:      "grpc",
		}, false)
		begin := time.Now()
		h.handleWithMetrics(ctx, &stats.RPCEndBase{
			Client:    true,
			BeginTime: begin,
			EndTime:   begin.Add(25 * time.Millisecond),
			Protocol:  "grpc",
		}, false)

		assert.Equal(t, []int64{56}, reqSize.values)
		assert.Equal(t, []int64{78}, respSize.values)
		assert.Equal(t, []int64{1}, reqPerRPC.values)
		assert.Equal(t, []int64{1}, respPerRPC.values)
		assert.Len(t, dur.values, 1)
	})
}

// TestServerStatus tests serverStatus function
func TestServerStatus(t *testing.T) {
	tests := []struct {
		name         string
		code         code.Code
		expectStatus codes.Code
	}{
		{
			name:         "OK",
			code:         code.Code_OK,
			expectStatus: codes.Unset,
		},
		{
			name:         "UNKNOWN",
			code:         code.Code_UNKNOWN,
			expectStatus: codes.Error,
		},
		{
			name:         "INTERNAL",
			code:         code.Code_INTERNAL,
			expectStatus: codes.Error,
		},
		{
			name:         "DEADLINE_EXCEEDED",
			code:         code.Code_DEADLINE_EXCEEDED,
			expectStatus: codes.Error,
		},
		{
			name:         "UNIMPLEMENTED",
			code:         code.Code_UNIMPLEMENTED,
			expectStatus: codes.Error,
		},
		{
			name:         "UNAVAILABLE",
			code:         code.Code_UNAVAILABLE,
			expectStatus: codes.Error,
		},
		{
			name:         "DATA_LOSS",
			code:         code.Code_DATA_LOSS,
			expectStatus: codes.Error,
		},
		{
			name:         "INVALID_ARGUMENT",
			code:         code.Code_INVALID_ARGUMENT,
			expectStatus: codes.Unset,
		},
		{
			name:         "NOT_FOUND",
			code:         code.Code_NOT_FOUND,
			expectStatus: codes.Unset,
		},
		{
			name:         "ALREADY_EXISTS",
			code:         code.Code_ALREADY_EXISTS,
			expectStatus: codes.Unset,
		},
		{
			name:         "PERMISSION_DENIED",
			code:         code.Code_PERMISSION_DENIED,
			expectStatus: codes.Unset,
		},
		{
			name:         "UNAUTHENTICATED",
			code:         code.Code_UNAUTHENTICATED,
			expectStatus: codes.Unset,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := status.FromError(xerror.New(tt.code, "test message"))

			statusCode, _ := serverStatus(st)
			assert.Equal(t, tt.expectStatus, statusCode)
		})
	}
}

func TestNewHandlerWithConfig_NilConfig(t *testing.T) {
	h := newHandlerWithConfig(nil, true)
	assert.NotNil(t, h)
	assert.NotNil(t, h.cfg)
}

func TestNewHandlerWithConfig_MetricsDisabled(t *testing.T) {
	cfg := &Config{EnableMetrics: false}
	h := newHandlerWithConfig(cfg, true)
	assert.NotNil(t, h)
	// handleRPC should be handleWithOutMetrics when metrics disabled
	assert.NotNil(t, h.handleRPC)
}

func TestHandleWithMetrics_RPCOutHeader(t *testing.T) {
	h := newHandler(false)
	h.cfg = &Config{EnableMetrics: true}

	ctx, span := h.tracer.Start(context.Background(), "test")
	defer span.End()

	rs := &stats.OutHeaderBase{
		Client:         true,
		Protocol:       "grpc",
		RemoteEndpoint: "server:8080",
	}
	// Should not panic
	h.handleWithMetrics(ctx, rs, false)
}

func TestHandleWithMetrics_RPCOutTrailer(t *testing.T) {
	h := newHandler(false)
	h.cfg = &Config{EnableMetrics: true}

	ctx, span := h.tracer.Start(context.Background(), "test")
	defer span.End()

	rs := &stats.OutTrailerBase{Client: true}
	// No-op case
	h.handleWithMetrics(ctx, rs, false)
}

func TestHandleWithMetrics_DefaultCase(t *testing.T) {
	h := newHandler(false)
	h.cfg = &Config{EnableMetrics: true}

	ctx := context.Background()
	// Unknown stats type should return early without panic
	h.handleWithMetrics(ctx, &stats.RPCBeginBase{}, false)
}

func TestHandleWithOutMetrics_RPCInPayload_NoContext(t *testing.T) {
	h := newHandler(false)
	h.cfg = &Config{ReceivedEvent: true}

	ctx, span := h.tracer.Start(context.Background(), "test")
	defer span.End()

	// No rpcContext in context - should not panic
	rs := &stats.RPCInPayloadBase{
		Client:        true,
		Data:          []byte("data"),
		TransportSize: 10,
		Protocol:      "grpc",
	}
	h.handleWithOutMetrics(ctx, rs, false)
}

func TestHandleWithOutMetrics_RPCOutPayload_NoContext(t *testing.T) {
	h := newHandler(false)
	h.cfg = &Config{SentEvent: true}

	ctx, span := h.tracer.Start(context.Background(), "test")
	defer span.End()

	// No rpcContext in context - should not panic
	rs := &stats.RPCOutPayloadBase{
		Client:        true,
		Data:          []byte("data"),
		TransportSize: 10,
		Protocol:      "grpc",
	}
	h.handleWithOutMetrics(ctx, rs, false)
}

func TestHandleWithOutMetrics_RPCOutHeader(t *testing.T) {
	h := newHandler(false)
	h.cfg = &Config{}

	ctx, span := h.tracer.Start(context.Background(), "test")
	defer span.End()

	rs := &stats.OutHeaderBase{
		Client:         true,
		Protocol:       "grpc",
		RemoteEndpoint: "server:8080",
	}
	h.handleWithOutMetrics(ctx, rs, false)
}

func TestHandleWithOutMetrics_DefaultCase(t *testing.T) {
	h := newHandler(false)
	ctx := context.Background()
	// Unknown stats type should return early
	h.handleWithOutMetrics(ctx, &stats.RPCBeginBase{}, false)
}

func TestHandleWithMetrics_RPCEnd_WithNilRpcContext(t *testing.T) {
	h := newHandler(true)
	h.cfg = &Config{EnableMetrics: true}
	dur := &spyFloat64Histogram{}
	h.rpcDuration = dur

	ctx, span := h.tracer.Start(context.Background(), "test")
	defer span.End()

	begin := time.Now()
	// No rpcContext in context, should still record duration
	h.handleWithMetrics(ctx, &stats.RPCEndBase{
		Client:    false,
		BeginTime: begin,
		EndTime:   begin.Add(10 * time.Millisecond),
		Protocol:  "grpc",
	}, true)

	assert.Len(t, dur.values, 1)
}

func TestHandleWithMetrics_ReceivedEventDisabled(t *testing.T) {
	h := newHandler(true)
	h.cfg = &Config{EnableMetrics: true, ReceivedEvent: false}

	ctx, span := h.tracer.Start(context.Background(), "test")
	defer span.End()
	ctx = context.WithValue(ctx, rpcContextKey{}, &rpcContext{})

	// Should not panic even with ReceivedEvent disabled
	rs := &stats.RPCInPayloadBase{
		Client:        false,
		Data:          []byte("data"),
		TransportSize: 10,
		Protocol:      "grpc",
	}
	h.handleWithMetrics(ctx, rs, true)
}

func TestHandleWithMetrics_SentEventDisabled(t *testing.T) {
	h := newHandler(false)
	h.cfg = &Config{EnableMetrics: true, SentEvent: false}

	ctx, span := h.tracer.Start(context.Background(), "test")
	defer span.End()
	ctx = context.WithValue(ctx, rpcContextKey{}, &rpcContext{})

	rs := &stats.RPCOutPayloadBase{
		Client:        true,
		Data:          []byte("data"),
		TransportSize: 10,
		Protocol:      "grpc",
	}
	h.handleWithMetrics(ctx, rs, false)
}

func TestHandleWithMetrics_ServerErrorStatus(t *testing.T) {
	h := newHandler(true)
	h.cfg = &Config{EnableMetrics: true}
	dur := &spyFloat64Histogram{}
	h.rpcDuration = dur

	ctx, span := h.tracer.Start(context.Background(), "test")
	defer span.End()
	ctx = context.WithValue(ctx, rpcContextKey{}, &rpcContext{})

	begin := time.Now()
	h.handleWithMetrics(ctx, &stats.RPCEndBase{
		Client:    false,
		BeginTime: begin,
		EndTime:   begin.Add(10 * time.Millisecond),
		Err:       xerror.New(code.Code_INTERNAL, "internal error"),
		Protocol:  "grpc",
	}, true)

	assert.Len(t, dur.values, 1)
}

func TestNewSvrHandlerWithConfig(t *testing.T) {
	cfg := &Config{EnableMetrics: false}
	h := newSvrHandlerWithConfig(cfg)
	assert.NotNil(t, h)
	assert.Implements(t, (*stats.Handler)(nil), h)
}

func TestNewCliHandlerWithConfig(t *testing.T) {
	cfg := &Config{EnableMetrics: false}
	h := newCliHandlerWithConfig(cfg)
	assert.NotNil(t, h)
	assert.Implements(t, (*stats.Handler)(nil), h)
}
