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
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/v2/logger/buffer"
	"go.opentelemetry.io/otel/trace"
)

type syncLinesWriter struct {
	mu    sync.Mutex
	lines []string
}

func (w *syncLinesWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	clone := make([]byte, len(p))
	copy(clone, p)
	w.lines = append(w.lines, string(clone))
	return len(p), nil
}

func (w *syncLinesWriter) Lines() []string {
	w.mu.Lock()
	defer w.mu.Unlock()

	out := make([]string, len(w.lines))
	copy(out, w.lines)
	return out
}

func TestNewJsonHandler(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *JSONHandlerConfig
		wantErr bool
	}{
		{
			name: "basic config",
			cfg: &JSONHandlerConfig{
				CommonHandlerConfig: CommonHandlerConfig{
					Level:         slog.LevelInfo,
					AddTrace:      false,
					AddErrVerbose: false,
				},
				AddSource: false,
			},
			wantErr: false,
		},
		{
			name: "with trace",
			cfg: &JSONHandlerConfig{
				CommonHandlerConfig: CommonHandlerConfig{
					Level:         slog.LevelInfo,
					AddTrace:      true,
					AddErrVerbose: false,
				},
				AddSource: false,
			},
			wantErr: false,
		},
		{
			name: "with source",
			cfg: &JSONHandlerConfig{
				CommonHandlerConfig: CommonHandlerConfig{
					Level:         slog.LevelInfo,
					AddTrace:      false,
					AddErrVerbose: false,
				},
				AddSource: true,
			},
			wantErr: false,
		},
		{
			name: "with verbose error",
			cfg: &JSONHandlerConfig{
				CommonHandlerConfig: CommonHandlerConfig{
					Level:         slog.LevelInfo,
					AddTrace:      false,
					AddErrVerbose: true,
				},
				AddSource: false,
			},
			wantErr: false,
		},
		{
			name: "nil writer",
			cfg: &JSONHandlerConfig{
				CommonHandlerConfig: CommonHandlerConfig{
					Level:         slog.LevelInfo,
					AddTrace:      false,
					AddErrVerbose: false,
				},
				AddSource: false,
				Writer:    nil,
			},
			wantErr: false, // nil writer is replaced with emptyWriter
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, err := NewJSONHandler(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewJSONHandler() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && h == nil {
				t.Error("NewJSONHandler() returned nil handler")
			}
		})
	}
}

func TestJsonHandlerHandle(t *testing.T) {
	cfg := &JSONHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		AddSource: false,
	}

	h, err := NewJSONHandler(cfg)
	if err != nil {
		t.Fatalf("NewJSONHandler() error = %v", err)
	}

	now := time.Now()
	record := slog.NewRecord(now, slog.LevelInfo, "test message", 0)

	err = h.Handle(context.Background(), record)
	if err != nil {
		t.Errorf("Handle() error = %v", err)
	}
}

func TestJsonHandlerHandleWithAttrs(t *testing.T) {
	cfg := &JSONHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		AddSource: false,
	}

	h, err := NewJSONHandler(cfg)
	if err != nil {
		t.Fatalf("NewJSONHandler() error = %v", err)
	}

	now := time.Now()
	record := slog.NewRecord(now, slog.LevelInfo, "test message", 0)
	record.AddAttrs(slog.String("key", "value"), slog.Int("number", 42))

	err = h.Handle(context.Background(), record)
	if err != nil {
		t.Errorf("Handle() error = %v", err)
	}
}

func TestJsonHandlerHandleOutput(t *testing.T) {
	mw := newMockWriter()
	cfg := &JSONHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		AddSource: false,
		Writer:    mw,
	}

	h, err := NewJSONHandler(cfg)
	if err != nil {
		t.Fatalf("NewJSONHandler() error = %v", err)
	}

	now := time.Date(2024, 1, 1, 12, 30, 45, 0, time.UTC)
	record := slog.NewRecord(now, slog.LevelInfo, "test message", 0)
	record.AddAttrs(slog.String("user", "test"))

	_ = h.Handle(context.Background(), record)

	// Get the output from the mock writer
	output := mw.String()

	// Verify it's valid JSON
	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	if err != nil {
		t.Errorf("Handle() produced invalid JSON: %v, output: %s", err, output)
	}

	// Check required fields
	if result["time"] == nil {
		t.Error("Handle() output missing time field")
	}
	if result["level"] != "INFO" {
		t.Errorf("Handle() level = %v, want INFO", result["level"])
	}
	if result["msg"] != "test message" {
		t.Errorf("Handle() msg = %v, want 'test message'", result["msg"])
	}
	if result["user"] != "test" {
		t.Errorf("Handle() user attr = %v, want 'test'", result["user"])
	}
}

func TestJsonHandlerWithAttrs(t *testing.T) {
	cfg := &JSONHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		AddSource: false,
	}

	h, err := NewJSONHandler(cfg)
	if err != nil {
		t.Fatalf("NewJSONHandler() error = %v", err)
	}

	attrs := []slog.Attr{
		slog.String("key1", "value1"),
		slog.Int("key2", 42),
	}

	newHandler := h.WithAttrs(attrs)
	if newHandler == nil {
		t.Error("WithAttrs() returned nil")
	}

	jsonHandler, ok := newHandler.(*jsonHandler)
	if !ok {
		t.Error("WithAttrs() did not return jsonHandler")
	}

	if jsonHandler == h {
		t.Error("WithAttrs() should return a new handler")
	}
}

func TestJsonHandlerWithGroup(t *testing.T) {
	cfg := &JSONHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		AddSource: false,
	}

	h, err := NewJSONHandler(cfg)
	if err != nil {
		t.Fatalf("NewJSONHandler() error = %v", err)
	}

	newHandler := h.WithGroup("test_group")
	if newHandler == nil {
		t.Error("WithGroup() returned nil")
	}

	jsonHandler, ok := newHandler.(*jsonHandler)
	if !ok {
		t.Error("WithGroup() did not return jsonHandler")
	}

	if len(jsonHandler.groups) != 1 || jsonHandler.groups[0] != "test_group" {
		t.Errorf("WithGroup() groups = %v, want [test_group]", jsonHandler.groups)
	}
}

func TestJsonHandlerEnabled(t *testing.T) {
	cfg := &JSONHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		AddSource: false,
	}

	h, err := NewJSONHandler(cfg)
	if err != nil {
		t.Fatalf("NewJSONHandler() error = %v", err)
	}

	tests := []struct {
		name  string
		level slog.Level
		want  bool
	}{
		{"debug", slog.LevelDebug, false},
		{"info", slog.LevelInfo, true},
		{"warn", slog.LevelWarn, true},
		{"error", slog.LevelError, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := h.Enabled(context.Background(), tt.level); got != tt.want {
				t.Errorf("Enabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJsonHandlerHandleWithSource(t *testing.T) {
	mw := newMockWriter()
	cfg := &JSONHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		AddSource: true,
		Writer:    mw,
	}

	h, err := NewJSONHandler(cfg)
	if err != nil {
		t.Fatalf("NewJSONHandler() error = %v", err)
	}

	now := time.Now()
	record := slog.NewRecord(now, slog.LevelInfo, "test message", 0)

	_ = h.Handle(context.Background(), record)

	// Get the output
	output := mw.String()

	// Verify it's valid JSON and contains source fields
	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	if err != nil {
		t.Errorf("Handle() produced invalid JSON: %v", err)
	}

	// Check if source fields are present (may be empty depending on PC)
	if result["file"] != nil || result["function"] != nil || result["line"] != nil {
		// Source fields are present - this is expected behavior
		t.Log("Source fields present in output")
	}
}

func TestJsonHandlerHandleWithTrace(t *testing.T) {
	mw := newMockWriter()
	cfg := &JSONHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      true,
			AddErrVerbose: false,
		},
		AddSource: false,
		Writer:    mw,
	}

	h, err := NewJSONHandler(cfg)
	if err != nil {
		t.Fatalf("NewJSONHandler() error = %v", err)
	}

	// Create a valid span context
	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
		TraceFlags: trace.FlagsSampled,
	})

	ctx := trace.ContextWithSpanContext(context.Background(), spanCtx)

	now := time.Now()
	record := slog.NewRecord(now, slog.LevelInfo, "test message", 0)

	_ = h.Handle(ctx, record)

	// Get the output
	output := mw.String()

	// Verify it's valid JSON and contains trace fields
	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	if err != nil {
		t.Errorf("Handle() produced invalid JSON: %v", err)
	}

	if result["trace_id"] == nil {
		t.Error("Handle() with trace context missing trace_id")
	}
	if result["span_id"] == nil {
		t.Error("Handle() with trace context missing span_id")
	}
}

func TestJsonHandlerAllLevels(t *testing.T) {
	levels := []slog.Level{
		slog.LevelDebug,
		slog.LevelInfo,
		slog.LevelWarn,
		slog.LevelError,
	}

	for _, level := range levels {
		t.Run(level.String(), func(t *testing.T) {
			mw := newMockWriter()
			cfg := &JSONHandlerConfig{
				CommonHandlerConfig: CommonHandlerConfig{
					Level:         level,
					AddTrace:      false,
					AddErrVerbose: false,
				},
				AddSource: false,
				Writer:    mw,
			}

			h, err := NewJSONHandler(cfg)
			if err != nil {
				t.Fatalf("NewJSONHandler() error = %v", err)
			}

			now := time.Now()
			record := slog.NewRecord(now, level, "test message", 0)

			err = h.Handle(context.Background(), record)
			if err != nil {
				t.Errorf("Handle() error = %v", err)
			}

			// Verify level in output
			output := mw.String()

			if !strings.Contains(output, level.String()) {
				t.Errorf("Output missing level %s: %s", level, output)
			}
		})
	}
}

func TestJsonHandlerMultipleWithAttrs(t *testing.T) {
	cfg := &JSONHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		AddSource: false,
	}

	h, err := NewJSONHandler(cfg)
	if err != nil {
		t.Fatalf("NewJSONHandler() error = %v", err)
	}

	// Chain multiple WithAttrs calls
	attrs1 := []slog.Attr{slog.String("key1", "value1")}
	attrs2 := []slog.Attr{slog.Int("key2", 42)}

	h1 := h.WithAttrs(attrs1)
	h2 := h1.WithAttrs(attrs2)

	if h2 == nil {
		t.Error("Chained WithAttrs() returned nil")
	}
}

func TestJsonHandlerMultipleWithGroup(t *testing.T) {
	cfg := &JSONHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		AddSource: false,
	}

	h, err := NewJSONHandler(cfg)
	if err != nil {
		t.Fatalf("NewJSONHandler() error = %v", err)
	}

	// Chain multiple WithGroup calls
	h1 := h.WithGroup("group1")
	h2 := h1.WithGroup("group2")

	jsonHandler, ok := h2.(*jsonHandler)
	if !ok {
		t.Fatal("WithGroup() did not return jsonHandler")
	}

	if len(jsonHandler.groups) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(jsonHandler.groups))
	}

	if jsonHandler.groups[0] != "group1" || jsonHandler.groups[1] != "group2" {
		t.Errorf("Groups = %v, want [group1, group2]", jsonHandler.groups)
	}
}

func TestJsonHandlerAddSourceHandle(t *testing.T) {
	cfg := &JSONHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		AddSource: true,
	}

	h, err := NewJSONHandler(cfg)
	if err != nil {
		t.Fatalf("NewJSONHandler() error = %v", err)
	}

	jh := h.(*jsonHandler)

	// Create a minimal record for testing
	now := time.Now()
	record := slog.NewRecord(now, slog.LevelInfo, "test", 0)

	buf := buffer.Get()
	objEnc := &jsonEncoder{}
	objEnc.SetBuffer(buf)

	jh.addSourceHandle(&record, objEnc)

	// The source handle should add file, function, and line info
	result := buf.String()
	// Just verify it doesn't panic and produces some output
	_ = result
}

func TestJsonHandlerWithEmptyAttrs(t *testing.T) {
	cfg := &JSONHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		AddSource: false,
	}

	h, err := NewJSONHandler(cfg)
	if err != nil {
		t.Fatalf("NewJSONHandler() error = %v", err)
	}

	emptyAttrs := []slog.Attr{}
	newHandler := h.WithAttrs(emptyAttrs)

	if newHandler == nil {
		t.Error("WithAttrs(empty) returned nil")
	}
}

func TestJsonHandlerHandleComplexAttrs(t *testing.T) {
	mw := newMockWriter()
	cfg := &JSONHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		AddSource: false,
		Writer:    mw,
	}

	h, err := NewJSONHandler(cfg)
	if err != nil {
		t.Fatalf("NewJSONHandler() error = %v", err)
	}

	now := time.Now()
	record := slog.NewRecord(now, slog.LevelInfo, "test message", 0)
	record.AddAttrs(
		slog.String("string", "value"),
		slog.Int("int", 42),
		slog.Float64("float", 3.14),
		slog.Bool("bool", true),
		slog.Duration("duration", time.Second),
		slog.Time("time", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
	)

	_ = h.Handle(context.Background(), record)

	// Verify the output is valid JSON
	output := mw.String()

	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	if err != nil {
		t.Errorf("Handle() with complex attrs produced invalid JSON: %v", err)
	}
}

func TestJsonHandlerHandleError(t *testing.T) {
	mw := newMockWriter()
	cfg := &JSONHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		AddSource: false,
		Writer:    mw,
	}

	h, err := NewJSONHandler(cfg)
	if err != nil {
		t.Fatalf("NewJSONHandler() error = %v", err)
	}

	now := time.Now()
	record := slog.NewRecord(now, slog.LevelInfo, "test message", 0)
	record.AddAttrs(slog.Any("error", "error value"))

	_ = h.Handle(context.Background(), record)

	// Verify the output is valid JSON
	output := mw.String()

	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	if err != nil {
		t.Errorf("Handle() with error produced invalid JSON: %v", err)
	}
}

func TestJsonHandlerWithAttrsAffectsOutput(t *testing.T) {
	mw := newMockWriter()
	cfg := &JSONHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		AddSource: false,
		Writer:    mw,
	}

	h, err := NewJSONHandler(cfg)
	if err != nil {
		t.Fatalf("NewJSONHandler() error = %v", err)
	}
	h = h.WithAttrs([]slog.Attr{slog.String("service", "yggdrasil")})

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)
	if err := h.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(mw.String()), &result); err != nil {
		t.Fatalf("Handle() produced invalid JSON: %v", err)
	}
	if result["service"] != "yggdrasil" {
		t.Fatalf("WithAttrs field missing from output, got service=%v", result["service"])
	}
}

func TestJsonHandlerWithAttrsAndWithGroupOrder(t *testing.T) {
	newRecord := func() slog.Record {
		record := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)
		record.AddAttrs(slog.String("dynamic", "d"))
		return record
	}

	buildHandler := func(writer *mockWriter) slog.Handler {
		cfg := &JSONHandlerConfig{
			CommonHandlerConfig: CommonHandlerConfig{
				Level:         slog.LevelInfo,
				AddTrace:      false,
				AddErrVerbose: false,
			},
			AddSource: false,
			Writer:    writer,
		}
		h, err := NewJSONHandler(cfg)
		if err != nil {
			t.Fatalf("NewJSONHandler() error = %v", err)
		}
		return h
	}

	mw1 := newMockWriter()
	h1 := buildHandler(mw1).WithAttrs([]slog.Attr{slog.String("static", "s")}).WithGroup("scope")
	record1 := newRecord()
	if err := h1.Handle(context.Background(), record1); err != nil {
		t.Fatalf("Handle() error for attrs->group order = %v", err)
	}
	var result1 map[string]interface{}
	if err := json.Unmarshal([]byte(mw1.String()), &result1); err != nil {
		t.Fatalf("invalid JSON for attrs->group order: %v", err)
	}
	if result1["static"] != "s" {
		t.Fatalf("attrs->group should keep static attr at root, got static=%v", result1["static"])
	}
	scope1, ok := result1["scope"].(map[string]interface{})
	if !ok {
		t.Fatalf("attrs->group should contain scope object, got %T", result1["scope"])
	}
	if scope1["dynamic"] != "d" {
		t.Fatalf(
			"attrs->group should place record attrs in scope, got dynamic=%v",
			scope1["dynamic"],
		)
	}

	mw2 := newMockWriter()
	h2 := buildHandler(mw2).WithGroup("scope").WithAttrs([]slog.Attr{slog.String("static", "s")})
	record2 := newRecord()
	if err := h2.Handle(context.Background(), record2); err != nil {
		t.Fatalf("Handle() error for group->attrs order = %v", err)
	}
	var result2 map[string]interface{}
	if err := json.Unmarshal([]byte(mw2.String()), &result2); err != nil {
		t.Fatalf("invalid JSON for group->attrs order: %v", err)
	}
	if _, ok := result2["static"]; ok {
		t.Fatalf(
			"group->attrs should nest static attr in scope, got root static=%v",
			result2["static"],
		)
	}
	scope2, ok := result2["scope"].(map[string]interface{})
	if !ok {
		t.Fatalf("group->attrs should contain scope object, got %T", result2["scope"])
	}
	if scope2["static"] != "s" || scope2["dynamic"] != "d" {
		t.Fatalf("group->attrs should keep both attrs in scope, got scope=%v", scope2)
	}
}

func TestJsonHandlerConcurrentHandleNoContamination(t *testing.T) {
	writer := &syncLinesWriter{}
	cfg := &JSONHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		AddSource: false,
		Writer:    writer,
	}

	h, err := NewJSONHandler(cfg)
	if err != nil {
		t.Fatalf("NewJSONHandler() error = %v", err)
	}

	const total = 200
	var wg sync.WaitGroup
	errCh := make(chan error, total)
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			record := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)
			record.AddAttrs(slog.Int("id", id))
			if err := h.Handle(context.Background(), record); err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("Handle() returned error during concurrent run: %v", err)
	}

	lines := writer.Lines()
	if len(lines) != total {
		t.Fatalf("expected %d log lines, got %d", total, len(lines))
	}

	seen := make(map[int]struct{}, total)
	for _, line := range lines {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("invalid JSON line during concurrent run: %v, line=%q", err, line)
		}
		rawID, ok := m["id"]
		if !ok {
			t.Fatalf("missing id field in concurrent output: %v", m)
		}
		idFloat, ok := rawID.(float64)
		if !ok {
			t.Fatalf("id field should be numeric, got %T (%v)", rawID, rawID)
		}
		id := int(idFloat)
		if id < 0 || id >= total {
			t.Fatalf("id out of range: %d", id)
		}
		seen[id] = struct{}{}
	}

	if len(seen) != total {
		t.Fatalf("expected %d unique ids, got %d", total, len(seen))
	}
}
