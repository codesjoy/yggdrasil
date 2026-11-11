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
	"encoding/json"
	"errors"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace"
)

type jsonTestWriter struct {
	mu    sync.Mutex
	lines []string
}

func (w *jsonTestWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lines = append(w.lines, string(append([]byte(nil), p...)))
	return len(p), nil
}

func (w *jsonTestWriter) Lines() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]string, len(w.lines))
	copy(out, w.lines)
	return out
}

func decodeJSONLine(t *testing.T, line string) map[string]any {
	t.Helper()
	raw := strings.TrimSpace(line)
	got := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("invalid JSON output: %v, raw=%q", err, raw)
	}
	return got
}

func mustSpanContext() context.Context {
	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		SpanID:     trace.SpanID{2, 2, 2, 2, 2, 2, 2, 2},
		TraceFlags: trace.FlagsSampled,
	})
	return trace.ContextWithSpanContext(context.Background(), spanCtx)
}

func TestNewJSONHandlerNilConfig(t *testing.T) {
	if _, err := NewJSONHandler(nil); err == nil {
		t.Fatal("NewJSONHandler(nil) should return error")
	}
}

func TestJSONHandlerOutputOfficialJSON(t *testing.T) {
	w := &jsonTestWriter{}
	h, err := NewJSONHandler(&JSONHandlerConfig{
		Level:  slog.LevelInfo,
		Writer: w,
	})
	if err != nil {
		t.Fatalf("NewJSONHandler() error = %v", err)
	}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "hello", 0)
	record.AddAttrs(slog.String("user", "alice"), slog.Int("id", 7))
	if err := h.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	lines := w.Lines()
	if len(lines) != 1 {
		t.Fatalf("expected 1 output line, got %d", len(lines))
	}
	got := decodeJSONLine(t, lines[0])
	if got["level"] != "INFO" || got["msg"] != "hello" {
		t.Fatalf("unexpected builtin fields: %v", got)
	}
	if got["user"] != "alice" || got["id"] != float64(7) {
		t.Fatalf("unexpected attrs: %v", got)
	}
}

func TestJSONHandlerAddSource(t *testing.T) {
	w := &jsonTestWriter{}
	h, err := NewJSONHandler(&JSONHandlerConfig{
		Level:     slog.LevelInfo,
		AddSource: true,
		Writer:    w,
	})
	if err != nil {
		t.Fatalf("NewJSONHandler() error = %v", err)
	}

	pc, _, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "with source", pc)
	if err := h.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	got := decodeJSONLine(t, w.Lines()[0])
	src, ok := got["source"].(map[string]any)
	if !ok {
		t.Fatalf("source should be object, got %T(%v)", got["source"], got["source"])
	}
	if src["file"] == nil || src["line"] == nil {
		t.Fatalf("source missing file/line: %v", src)
	}
}

func TestJSONHandlerAddTrace(t *testing.T) {
	w := &jsonTestWriter{}
	h, err := NewJSONHandler(&JSONHandlerConfig{
		Level:    slog.LevelInfo,
		AddTrace: true,
		Writer:   w,
	})
	if err != nil {
		t.Fatalf("NewJSONHandler() error = %v", err)
	}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "trace", 0)
	if err := h.Handle(mustSpanContext(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	got := decodeJSONLine(t, w.Lines()[0])
	if got["trace_id"] == nil || got["span_id"] == nil {
		t.Fatalf("trace fields missing: %v", got)
	}
}

type jsonTestLogValuer struct{}

func (jsonTestLogValuer) LogValue() slog.Value {
	return slog.StringValue("resolved")
}

func TestJSONHandlerAnyTypeMatrix(t *testing.T) {
	type sampleStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	var typedNil *int
	tests := []struct {
		name string
		attr slog.Attr
		want func(t *testing.T, v any)
	}{
		{
			name: "nil",
			attr: slog.Any("value", nil),
			want: func(t *testing.T, v any) {
				if v != nil {
					t.Fatalf("value = %v, want nil", v)
				}
			},
		},
		{
			name: "typed nil pointer",
			attr: slog.Any("value", typedNil),
			want: func(t *testing.T, v any) {
				if v != nil {
					t.Fatalf("value = %v, want nil", v)
				}
			},
		},
		{
			name: "bool",
			attr: slog.Any("value", true),
			want: func(t *testing.T, v any) {
				if got, ok := v.(bool); !ok || !got {
					t.Fatalf("value = %T(%v), want bool(true)", v, v)
				}
			},
		},
		{
			name: "int",
			attr: slog.Any("value", 42),
			want: func(t *testing.T, v any) {
				if got, ok := v.(float64); !ok || got != 42 {
					t.Fatalf("value = %T(%v), want float64(42)", v, v)
				}
			},
		},
		{
			name: "map",
			attr: slog.Any("value", map[string]any{"k": "v", "n": 1}),
			want: func(t *testing.T, v any) {
				got, ok := v.(map[string]any)
				if !ok || got["k"] != "v" || got["n"] != float64(1) {
					t.Fatalf("value = %v, want map[k:v n:1]", v)
				}
			},
		},
		{
			name: "struct",
			attr: slog.Any("value", sampleStruct{Name: "alice", Age: 18}),
			want: func(t *testing.T, v any) {
				got, ok := v.(map[string]any)
				if !ok || got["name"] != "alice" || got["age"] != float64(18) {
					t.Fatalf("value = %v, want struct object", v)
				}
			},
		},
		{
			name: "error",
			attr: slog.Any("value", errors.New("boom")),
			want: func(t *testing.T, v any) {
				if got, ok := v.(string); !ok || got != "boom" {
					t.Fatalf("value = %T(%v), want string boom", v, v)
				}
			},
		},
		{
			name: "logvaluer",
			attr: slog.Any("value", jsonTestLogValuer{}),
			want: func(t *testing.T, v any) {
				if v != "resolved" {
					t.Fatalf("value = %v, want resolved", v)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &jsonTestWriter{}
			h, err := NewJSONHandler(&JSONHandlerConfig{
				Level:  slog.LevelInfo,
				Writer: w,
			})
			if err != nil {
				t.Fatalf("NewJSONHandler() error = %v", err)
			}

			record := slog.NewRecord(time.Now(), slog.LevelInfo, "any", 0)
			record.AddAttrs(tt.attr)
			if err := h.Handle(context.Background(), record); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}

			got := decodeJSONLine(t, w.Lines()[0])
			tt.want(t, got["value"])
		})
	}
}

func TestJSONHandlerGroupSemantics(t *testing.T) {
	w := &jsonTestWriter{}
	h, err := NewJSONHandler(&JSONHandlerConfig{
		Level:  slog.LevelInfo,
		Writer: w,
	})
	if err != nil {
		t.Fatalf("NewJSONHandler() error = %v", err)
	}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "group", 0)
	record.AddAttrs(
		slog.Group("req", slog.String("path", "/v1/health")),
		slog.Group("", slog.Int("flat", 7)),
	)
	if err := h.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	got := decodeJSONLine(t, w.Lines()[0])
	req, ok := got["req"].(map[string]any)
	if !ok || req["path"] != "/v1/health" {
		t.Fatalf("req = %v, want nested req.path", got["req"])
	}
	if got["flat"] != float64(7) {
		t.Fatalf("flat = %v, want 7", got["flat"])
	}
}

func TestJSONHandlerConcurrentHandleNoContamination(t *testing.T) {
	w := &jsonTestWriter{}
	h, err := NewJSONHandler(&JSONHandlerConfig{
		Level:  slog.LevelInfo,
		Writer: w,
	})
	if err != nil {
		t.Fatalf("NewJSONHandler() error = %v", err)
	}

	const total = 128
	var wg sync.WaitGroup
	errCh := make(chan error, total)
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			record := slog.NewRecord(time.Now(), slog.LevelInfo, "concurrent", 0)
			record.AddAttrs(slog.Int("id", id))
			if err := h.Handle(context.Background(), record); err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("Handle() error: %v", err)
	}

	lines := w.Lines()
	if len(lines) != total {
		t.Fatalf("expected %d lines, got %d", total, len(lines))
	}
}

func TestNewConsoleHandlerNilConfig(t *testing.T) {
	if _, err := NewConsoleHandler(nil); err == nil {
		t.Fatal("NewConsoleHandler(nil) should return error")
	}
}

func TestConsoleHandlerOutputOfficialText(t *testing.T) {
	w := &jsonTestWriter{}
	h, err := NewConsoleHandler(&ConsoleHandlerConfig{
		Level:  slog.LevelInfo,
		Writer: w,
	})
	if err != nil {
		t.Fatalf("NewConsoleHandler() error = %v", err)
	}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "hello world", 0)
	record.AddAttrs(slog.String("user", "alice"), slog.Int("id", 7))
	if err := h.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	out := strings.TrimSpace(w.Lines()[0])
	for _, want := range []string{"level=INFO", `msg="hello world"`, "user=alice", "id=7"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output should contain %q, got %q", want, out)
		}
	}
}

func TestConsoleHandlerAddSource(t *testing.T) {
	w := &jsonTestWriter{}
	h, err := NewConsoleHandler(&ConsoleHandlerConfig{
		Level:     slog.LevelInfo,
		AddSource: true,
		Writer:    w,
	})
	if err != nil {
		t.Fatalf("NewConsoleHandler() error = %v", err)
	}

	pc, _, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "with source", pc)
	if err := h.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	out := strings.TrimSpace(w.Lines()[0])
	if !strings.Contains(out, "source=") {
		t.Fatalf("output should contain source=..., got %q", out)
	}
}

func TestConsoleHandlerAddTrace(t *testing.T) {
	w := &jsonTestWriter{}
	h, err := NewConsoleHandler(&ConsoleHandlerConfig{
		Level:    slog.LevelInfo,
		AddTrace: true,
		Writer:   w,
	})
	if err != nil {
		t.Fatalf("NewConsoleHandler() error = %v", err)
	}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "trace", 0)
	if err := h.Handle(mustSpanContext(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	out := strings.TrimSpace(w.Lines()[0])
	if !strings.Contains(out, "trace_id=") || !strings.Contains(out, "span_id=") {
		t.Fatalf("trace fields missing in output: %q", out)
	}
}

type consoleTestLogValuer struct{}

func (consoleTestLogValuer) LogValue() slog.Value {
	return slog.StringValue("resolved")
}

func TestConsoleHandlerAnyAndLogValuer(t *testing.T) {
	var typedNil *int
	w := &jsonTestWriter{}
	h, err := NewConsoleHandler(&ConsoleHandlerConfig{
		Level:  slog.LevelInfo,
		Writer: w,
	})
	if err != nil {
		t.Fatalf("NewConsoleHandler() error = %v", err)
	}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "any", 0)
	record.AddAttrs(
		slog.Any("nil_value", nil),
		slog.Any("typed_nil", typedNil),
		slog.Any("bool_value", true),
		slog.Any("obj", map[string]any{"k": "v"}),
		slog.Any("lv", consoleTestLogValuer{}),
	)
	if err := h.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	out := strings.TrimSpace(w.Lines()[0])
	for _, want := range []string{"nil_value=<nil>", "typed_nil=<nil>", "bool_value=true", "lv=resolved"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output should contain %q, got %q", want, out)
		}
	}
	if !strings.Contains(out, `obj="map[k:v]"`) && !strings.Contains(out, "obj=map[k:v]") {
		t.Fatalf("object output mismatch: %q", out)
	}
}

func TestConsoleHandlerGroupSemantics(t *testing.T) {
	w := &jsonTestWriter{}
	h, err := NewConsoleHandler(&ConsoleHandlerConfig{
		Level:  slog.LevelInfo,
		Writer: w,
	})
	if err != nil {
		t.Fatalf("NewConsoleHandler() error = %v", err)
	}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "group", 0)
	record.AddAttrs(
		slog.Group("req", slog.String("path", "/v1/health")),
		slog.Group("", slog.Int("flat", 7)),
	)
	if err := h.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	out := strings.TrimSpace(w.Lines()[0])
	if !strings.Contains(out, "req.path=/v1/health") {
		t.Fatalf("nested group output mismatch: %q", out)
	}
	if !strings.Contains(out, "flat=7") {
		t.Fatalf("flat group output mismatch: %q", out)
	}
}

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

func (h *captureHandler) Handle(_ context.Context, record slog.Record) error {
	h.handleCalls++
	h.lastMessage = record.Message
	h.lastAttrs = map[string]any{}
	record.Attrs(func(attr slog.Attr) bool {
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

	ok := h.Enabled(t.Context(), slog.LevelWarn)
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
	if err := h.Handle(t.Context(), record); err != nil {
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
