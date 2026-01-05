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

	"github.com/codesjoy/yggdrasil/v2/stats"
	"github.com/codesjoy/yggdrasil/v2/status"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/codes"
	"google.golang.org/genproto/googleapis/rpc/code"
)

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
			// Create a real status.Status with the given code using status.New
			st := status.New(tt.code, "test message")

			statusCode, _ := serverStatus(st)
			assert.Equal(t, tt.expectStatus, statusCode)
		})
	}
}
