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
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/v2/logger/buffer"
	"go.opentelemetry.io/otel/trace"
)

// mockWriter is a mock io.Writer for testing
type mockWriter struct {
	buf *strings.Builder
}

func newMockWriter() *mockWriter {
	return &mockWriter{buf: &strings.Builder{}}
}

func (m *mockWriter) Write(p []byte) (n int, err error) {
	return m.buf.Write(p)
}

func (m *mockWriter) String() string {
	return m.buf.String()
}

func (m *mockWriter) Reset() {
	m.buf.Reset()
}

// newTestCommonHandler creates a commonHandler with a mock writer for testing
func newTestCommonHandler(level slog.Level, addTrace, addErrVerbose bool) (*commonHandler, error) {
	h := &commonHandler{
		lv:          level,
		writer:      newMockWriter(),
		traceHandle: func(context.Context, ObjectEncoder) {},
	}

	cfg := &JSONEncoderConfig{}
	objEnc, _ := NewJSONEncoder(cfg)
	h.objEnc = objEnc

	if addTrace {
		h.traceHandle = h.addTrace
	}

	if addErrVerbose {
		h.errorHandle = h.handleErrorWithVerbose
	} else {
		h.errorHandle = h.handleErrorOnlyError
	}

	return h, nil
}

func TestNewCommonHandler(t *testing.T) {
	h, err := newTestCommonHandler(slog.LevelInfo, false, false)
	if err != nil {
		t.Fatalf("newTestCommonHandler() error = %v", err)
	}

	if h == nil {
		t.Error("newTestCommonHandler() returned nil")
		return
	}

	if h.lv != slog.LevelInfo {
		t.Errorf("Expected level Info, got %v", h.lv)
	}
}

func TestCommonHandlerEnabled(t *testing.T) {
	h, _ := newTestCommonHandler(slog.LevelInfo, false, false)

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

func TestCommonHandlerWithAttrs(t *testing.T) {
	cfg := &CommonHandlerConfig{
		Level:         slog.LevelInfo,
		AddTrace:      false,
		AddErrVerbose: false,
		objEnc:        &jsonEncoder{},
	}

	h, _ := newCommonHandler(cfg)

	attrs := []slog.Attr{
		slog.String("key1", "value1"),
		slog.Int("key2", 42),
	}

	newHandler := h.WithAttrs(attrs)
	if newHandler == nil {
		t.Error("WithAttrs() returned nil")
		return
	}

	if newHandler == h {
		t.Error("WithAttrs() should return a new handler")
		return
	}
}

func TestCommonHandlerWithGroup(t *testing.T) {
	cfg := &CommonHandlerConfig{
		Level:         slog.LevelInfo,
		AddTrace:      false,
		AddErrVerbose: false,
		objEnc:        &jsonEncoder{},
	}

	h, _ := newCommonHandler(cfg)

	newHandler := h.WithGroup("test_group")
	if newHandler == nil {
		t.Error("WithGroup() returned nil")
		return
	}

	if newHandler == h {
		t.Error("WithGroup() should return a new handler")
		return
	}

	if len(newHandler.groups) != 1 || newHandler.groups[0] != "test_group" {
		t.Errorf("WithGroup() groups = %v, want [test_group]", newHandler.groups)
		return
	}
}

func TestCommonHandlerClone(t *testing.T) {
	cfg := &CommonHandlerConfig{
		Level:         slog.LevelInfo,
		AddTrace:      true,
		AddErrVerbose: true,
		objEnc:        &jsonEncoder{},
	}

	h, _ := newCommonHandler(cfg)
	h.groups = []string{"group1", "group2"}

	cloned := h.clone()
	if cloned == h {
		t.Error("clone() should return a different instance")
	}

	if cloned.lv != h.lv {
		t.Error("clone() level mismatch")
	}

	// Verify groups are cloned
	if len(cloned.groups) != len(h.groups) {
		t.Error("clone() groups length mismatch")
	}

	// Modify original groups
	h.groups[0] = "modified"
	if cloned.groups[0] == "modified" {
		t.Error("clone() did not create independent copy of groups")
	}
}

func TestCommonHandlerAddTrace(t *testing.T) {
	cfg := &CommonHandlerConfig{
		Level:         slog.LevelInfo,
		AddTrace:      true,
		AddErrVerbose: false,
		objEnc:        &jsonEncoder{},
	}

	h, _ := newCommonHandler(cfg)

	// Test with valid span context
	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
		TraceFlags: trace.FlagsSampled,
	})

	ctx := trace.ContextWithSpanContext(context.Background(), spanCtx)

	buf := buffer.Get()
	enc := &jsonEncoder{}
	enc.SetBuffer(buf)

	h.traceHandle(ctx, enc)

	result := buf.String()
	if !strings.Contains(result, "trace_id") {
		t.Error("addTrace() did not add trace_id")
	}
	if !strings.Contains(result, "span_id") {
		t.Error("addTrace() did not add span_id")
	}
}

func TestCommonHandlerAddTraceInvalidContext(t *testing.T) {
	cfg := &CommonHandlerConfig{
		Level:         slog.LevelInfo,
		AddTrace:      true,
		AddErrVerbose: false,
		objEnc:        &jsonEncoder{},
	}

	h, _ := newCommonHandler(cfg)

	buf := buffer.Get()
	enc := &jsonEncoder{}
	enc.SetBuffer(buf)

	h.traceHandle(context.Background(), enc)

	result := buf.String()
	if result != "" {
		t.Errorf("addTrace() with invalid context should not add anything, got: %s", result)
	}
}

func TestCommonHandlerHandleErrorWithVerbose(t *testing.T) {
	cfg := &CommonHandlerConfig{
		Level:         slog.LevelInfo,
		AddTrace:      false,
		AddErrVerbose: true,
		objEnc:        &jsonEncoder{},
	}

	h, _ := newCommonHandler(cfg)

	// Test with a verbose error
	verboseErr := errors.New("basic error")

	buf := buffer.Get()
	enc := &jsonEncoder{}
	enc.SetBuffer(buf)

	h.errorHandle("error_key", verboseErr, enc)

	result := buf.String()
	if !strings.Contains(result, "error_key") {
		t.Error("handleErrorWithVerbose() did not add error key")
	}
	if !strings.Contains(result, "basic error") {
		t.Error("handleErrorWithVerbose() did not add error message")
	}
}

func TestCommonHandlerHandleErrorOnlyError(t *testing.T) {
	cfg := &CommonHandlerConfig{
		Level:         slog.LevelInfo,
		AddTrace:      false,
		AddErrVerbose: false,
		objEnc:        &jsonEncoder{},
	}

	h, _ := newCommonHandler(cfg)

	err := errors.New("simple error")

	buf := buffer.Get()
	enc := &jsonEncoder{}
	enc.SetBuffer(buf)

	h.errorHandle("error_key", err, enc)

	result := buf.String()
	if !strings.Contains(result, "error_key") {
		t.Error("handleErrorOnlyError() did not add error key")
	}
	if !strings.Contains(result, "simple error") {
		t.Error("handleErrorOnlyError() did not add error message")
	}

	if strings.Contains(result, "Verbose") {
		t.Error("handleErrorOnlyError() should not add Verbose key")
	}
}

func TestCommonHandlerEncodeSlogAttr(t *testing.T) {
	cfg := &CommonHandlerConfig{
		Level:         slog.LevelInfo,
		AddTrace:      false,
		AddErrVerbose: false,
		objEnc:        &jsonEncoder{},
	}

	h, _ := newCommonHandler(cfg)

	tests := []struct {
		name  string
		attr  slog.Attr
		check func(string) bool
	}{
		{
			"bool",
			slog.Bool("key", true),
			func(s string) bool { return strings.Contains(s, "true") },
		},
		{
			"duration",
			slog.Duration("key", time.Second),
			func(s string) bool { return s != "" },
		},
		{
			"float64",
			slog.Float64("key", 3.14),
			func(s string) bool { return strings.Contains(s, "3.14") },
		},
		{
			"int64",
			slog.Int64("key", 42),
			func(s string) bool { return strings.Contains(s, "42") },
		},
		{
			"string",
			slog.String("key", "value"),
			func(s string) bool { return strings.Contains(s, "value") },
		},
		{
			"time",
			slog.Time("key", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
			func(s string) bool { return s != "" },
		},
		{
			"uint64",
			slog.Uint64("key", 100),
			func(s string) bool { return strings.Contains(s, "100") },
		},
		{
			"group",
			slog.Group("key", slog.String("inner", "value")),
			func(s string) bool { return strings.Contains(s, "value") },
		},
		{
			"error",
			slog.Any("error", errors.New("test error")),
			func(s string) bool { return strings.Contains(s, "test error") },
		},
		{
			"any",
			slog.Any("key", "any value"),
			func(s string) bool { return strings.Contains(s, "any value") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := buffer.Get()
			objEnc := &jsonEncoder{}
			objEnc.SetBuffer(buf)

			h.encodeSlogAttr(tt.attr, objEnc)

			result := buf.String()
			if !tt.check(result) {
				t.Errorf("encodeSlogAttr(%s) result = %s", tt.name, result)
			}
		})
	}
}

// verboseError is an error that implements fmt.Formatter
type verboseError struct {
	basic   string
	verbose string
}

func (e verboseError) Error() string {
	return e.basic
}

func (e verboseError) Format(state fmt.State, verb rune) {
	if verb == 'v' && state.Flag('+') {
		_, _ = fmt.Fprintf(state, "%s", e.verbose)
	} else {
		_, _ = fmt.Fprintf(state, "%s", e.basic)
	}
}

func newVerboseError(basic, verbose string) verboseError {
	return verboseError{basic: basic, verbose: verbose}
}

func TestCommonHandlerHandleVerboseError(t *testing.T) {
	cfg := &CommonHandlerConfig{
		Level:         slog.LevelInfo,
		AddTrace:      false,
		AddErrVerbose: true,
		objEnc:        &jsonEncoder{},
	}

	h, _ := newCommonHandler(cfg)

	// Create a verbose error with different messages
	err := newVerboseError("basic error", "verbose error with stack trace")

	buf := buffer.Get()
	enc := &jsonEncoder{}
	enc.SetBuffer(buf)

	h.errorHandle("error", err, enc)

	result := buf.String()
	if !strings.Contains(result, "basic error") {
		t.Error("handleErrorWithVerbose() did not add basic error message")
	}
	if !strings.Contains(result, "errorVerbose") {
		t.Error("handleErrorWithVerbose() did not add Verbose key for verbose error")
	}
}

func TestCommonHandlerAddTraceNotEnabled(t *testing.T) {
	cfg := &CommonHandlerConfig{
		Level:         slog.LevelInfo,
		AddTrace:      false,
		AddErrVerbose: false,
		objEnc:        &jsonEncoder{},
	}

	h, _ := newCommonHandler(cfg)

	buf := buffer.Get()
	enc := &jsonEncoder{}
	enc.SetBuffer(buf)

	// When AddTrace is false, traceHandle should be a no-op
	h.traceHandle(context.Background(), enc)

	result := buf.String()
	if result != "" {
		t.Errorf("traceHandle when AddTrace is false should be no-op, got: %s", result)
	}
}

func TestCommonHandlerOpenGroups(t *testing.T) {
	cfg := &CommonHandlerConfig{
		Level:         slog.LevelInfo,
		AddTrace:      false,
		AddErrVerbose: false,
		objEnc:        &jsonEncoder{},
	}

	h, _ := newCommonHandler(cfg)
	h.groups = []string{"group1", "group2"}

	buf := buffer.Get()
	objEnc := &jsonEncoder{}
	objEnc.SetBuffer(buf)

	h.openGroups(objEnc)

	if h.nOpenGroups != 2 {
		t.Errorf("openGroups() nOpenGroups = %d, want 2", h.nOpenGroups)
	}
}
