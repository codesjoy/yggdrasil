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

	"github.com/codesjoy/yggdrasil/pkg/stats"
	"go.opentelemetry.io/otel"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

type serverHandler struct {
	handler
}

func newSvrHandler() *serverHandler {
	return &serverHandler{
		handler: newHandler(true),
	}
}

// TagChannel can attach some information to the given context.
func (h *serverHandler) TagChannel(ctx context.Context, _ stats.ChanTagInfo) context.Context {
	return ctx
}

// HandleChannel processes the channel stats.
func (h *serverHandler) HandleChannel(context.Context, stats.ChanStats) {
}

// TagRPC can attach some information to the given context.
func (h *serverHandler) TagRPC(ctx context.Context, info stats.RPCTagInfo) context.Context {
	ctx = extract(ctx, otel.GetTextMapPropagator())

	spanName, attrs := parseFullMethod(info.GetFullMethod())
	attrs = append(attrs, semconv.RPCSystemKey.String("yggdrasil"))
	ctx, _ = h.tracer.Start(
		trace.ContextWithRemoteSpanContext(ctx, trace.SpanContextFromContext(ctx)),
		spanName,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(attrs...),
	)

	return context.WithValue(ctx, rpcContextKey{}, &rpcContext{
		metricAttrs: attrs,
	})
}

// HandleRPC processes the RPC stats.
func (h *serverHandler) HandleRPC(ctx context.Context, rs stats.RPCStats) {
	h.handleRPC(ctx, rs, true)
}
