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
	"runtime"
	"strings"
	"testing"
	"time"
)

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
