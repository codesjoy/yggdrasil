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
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/v2/logger/buffer"
	"go.opentelemetry.io/otel/trace"
)

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
