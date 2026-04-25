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
