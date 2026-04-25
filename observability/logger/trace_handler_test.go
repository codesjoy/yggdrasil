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

package logger

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

type captureHandler struct {
	enabledResult bool
	enabledCalls  int
	enabledLevel  slog.Level

	handleCalls int
	lastMessage string
	lastAttrs   map[string]any

	withAttrsCalls int
	withAttrsInput []slog.Attr
	nextWithAttrs  slog.Handler

	withGroupCalls int
	withGroupInput string
	nextWithGroup  slog.Handler
}

func (h *captureHandler) Enabled(_ context.Context, level slog.Level) bool {
	h.enabledCalls++
	h.enabledLevel = level
	return h.enabledResult
}

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.handleCalls++
	h.lastMessage = r.Message
	h.lastAttrs = map[string]any{}
	r.Attrs(func(attr slog.Attr) bool {
		h.lastAttrs[attr.Key] = attr.Value.Any()
		return true
	})
	return nil
}

func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.withAttrsCalls++
	h.withAttrsInput = append([]slog.Attr(nil), attrs...)
	if h.nextWithAttrs != nil {
		return h.nextWithAttrs
	}
	return h
}

func (h *captureHandler) WithGroup(name string) slog.Handler {
	h.withGroupCalls++
	h.withGroupInput = name
	if h.nextWithGroup != nil {
		return h.nextWithGroup
	}
	return h
}

func TestTraceHandlerEnabledDelegates(t *testing.T) {
	base := &captureHandler{enabledResult: true}
	h := &traceHandler{base: base}

	ok := h.Enabled(context.Background(), slog.LevelWarn)
	if !ok {
		t.Fatal("Enabled() = false, want true")
	}
	if base.enabledCalls != 1 {
		t.Fatalf("base.Enabled call count = %d, want 1", base.enabledCalls)
	}
	if base.enabledLevel != slog.LevelWarn {
		t.Fatalf("base.Enabled level = %v, want %v", base.enabledLevel, slog.LevelWarn)
	}
}

func TestTraceHandlerHandleWithoutValidSpanPassThrough(t *testing.T) {
	base := &captureHandler{}
	h := &traceHandler{base: base}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "no trace", 0)
	record.AddAttrs(slog.String("k", "v"))
	if err := h.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if base.handleCalls != 1 {
		t.Fatalf("base.Handle call count = %d, want 1", base.handleCalls)
	}
	if base.lastMessage != "no trace" {
		t.Fatalf("message = %q, want %q", base.lastMessage, "no trace")
	}
	if base.lastAttrs["k"] != "v" {
		t.Fatalf("attr k = %v, want v", base.lastAttrs["k"])
	}
	if _, ok := base.lastAttrs["trace_id"]; ok {
		t.Fatalf("unexpected trace_id attr: %v", base.lastAttrs["trace_id"])
	}
	if _, ok := base.lastAttrs["span_id"]; ok {
		t.Fatalf("unexpected span_id attr: %v", base.lastAttrs["span_id"])
	}
}

func TestTraceHandlerHandleWithValidSpanAddsTraceFields(t *testing.T) {
	base := &captureHandler{}
	h := &traceHandler{base: base}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "with trace", 0)
	record.AddAttrs(slog.String("user", "alice"))
	if err := h.Handle(mustSpanContext(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if base.handleCalls != 1 {
		t.Fatalf("base.Handle call count = %d, want 1", base.handleCalls)
	}
	if base.lastAttrs["user"] != "alice" {
		t.Fatalf("attr user = %v, want alice", base.lastAttrs["user"])
	}
	traceID, ok := base.lastAttrs["trace_id"].(string)
	if !ok || traceID == "" {
		t.Fatalf(
			"trace_id = %T(%v), want non-empty string",
			base.lastAttrs["trace_id"],
			base.lastAttrs["trace_id"],
		)
	}
	spanID, ok := base.lastAttrs["span_id"].(string)
	if !ok || spanID == "" {
		t.Fatalf(
			"span_id = %T(%v), want non-empty string",
			base.lastAttrs["span_id"],
			base.lastAttrs["span_id"],
		)
	}
}

func TestTraceHandlerWithAttrsAndWithGroupDelegate(t *testing.T) {
	base := &captureHandler{}
	attrsBase := &captureHandler{}
	groupBase := &captureHandler{}
	base.nextWithAttrs = attrsBase
	base.nextWithGroup = groupBase

	h := &traceHandler{base: base}
	withAttrs := h.WithAttrs([]slog.Attr{slog.String("k", "v")})
	withGroup := h.WithGroup("req")

	attrsWrapped, ok := withAttrs.(*traceHandler)
	if !ok {
		t.Fatalf("WithAttrs() type = %T, want *traceHandler", withAttrs)
	}
	if attrsWrapped.base != attrsBase {
		t.Fatalf("WithAttrs() base = %v, want %v", attrsWrapped.base, attrsBase)
	}
	if base.withAttrsCalls != 1 {
		t.Fatalf("base.WithAttrs call count = %d, want 1", base.withAttrsCalls)
	}
	if len(base.withAttrsInput) != 1 || base.withAttrsInput[0].Key != "k" {
		t.Fatalf("base.WithAttrs input = %v, want one attr key=k", base.withAttrsInput)
	}

	groupWrapped, ok := withGroup.(*traceHandler)
	if !ok {
		t.Fatalf("WithGroup() type = %T, want *traceHandler", withGroup)
	}
	if groupWrapped.base != groupBase {
		t.Fatalf("WithGroup() base = %v, want %v", groupWrapped.base, groupBase)
	}
	if base.withGroupCalls != 1 {
		t.Fatalf("base.WithGroup call count = %d, want 1", base.withGroupCalls)
	}
	if base.withGroupInput != "req" {
		t.Fatalf("base.WithGroup input = %q, want req", base.withGroupInput)
	}
}
