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

// Package otel provides OpenTelemetry stats handler
package otel

import (
	"context"

	"github.com/codesjoy/yggdrasil/pkg/stats"
	"go.opentelemetry.io/otel"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

type clientHandler struct {
	handler
}

func newCliHandler() stats.Handler {
	h := &clientHandler{
		handler: newHandler(false),
	}
	return h
}

// TagRPC can attach some information to the given context.
func (h *clientHandler) TagRPC(ctx context.Context, info stats.RPCTagInfo) context.Context {
	spanName, attrs := parseFullMethod(info.GetFullMethod())
	attrs = append(attrs, semconv.RPCSystemKey.String("yggdrasil"))
	ctx, _ = h.tracer.Start(
		ctx,
		spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...),
	)

	gctx := rpcContext{
		metricAttrs: attrs,
	}

	return inject(context.WithValue(ctx, rpcContextKey{}, &gctx), otel.GetTextMapPropagator())
}

// HandleRPC handles the given RPC stats.
func (h *clientHandler) HandleRPC(ctx context.Context, rs stats.RPCStats) {
	h.handleRPC(ctx, rs, false)
}

// TagChannel can attach some information to the given context.
func (h *clientHandler) TagChannel(ctx context.Context, _ stats.ChanTagInfo) context.Context {
	return ctx
}

// HandleChannel handles the given channel stats.
func (h *clientHandler) HandleChannel(context.Context, stats.ChanStats) {
	// no-op
}
