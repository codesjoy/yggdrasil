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
	"strings"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/v2/logger/buffer"
)

func TestNewConsoleHandler(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *ConsoleHandlerConfig
		wantErr bool
	}{
		{
			name: "basic config",
			cfg: &ConsoleHandlerConfig{
				CommonHandlerConfig: CommonHandlerConfig{
					Level:         slog.LevelInfo,
					AddTrace:      false,
					AddErrVerbose: false,
				},
				TimeHandler: "RFC3339",
				AddSource:   false,
			},
			wantErr: false,
		},
		{
			name: "with trace",
			cfg: &ConsoleHandlerConfig{
				CommonHandlerConfig: CommonHandlerConfig{
					Level:         slog.LevelInfo,
					AddTrace:      true,
					AddErrVerbose: false,
				},
				TimeHandler: "RFC3339",
				AddSource:   false,
			},
			wantErr: false,
		},
		{
			name: "with source",
			cfg: &ConsoleHandlerConfig{
				CommonHandlerConfig: CommonHandlerConfig{
					Level:         slog.LevelInfo,
					AddTrace:      false,
					AddErrVerbose: false,
				},
				TimeHandler: "RFC3339",
				AddSource:   true,
			},
			wantErr: false,
		},
		{
			name: "with nil writer",
			cfg: &ConsoleHandlerConfig{
				CommonHandlerConfig: CommonHandlerConfig{
					Level:         slog.LevelInfo,
					AddTrace:      false,
					AddErrVerbose: false,
				},
				TimeHandler: "RFC3339",
				AddSource:   false,
				Writer:      nil,
			},
			wantErr: false, // nil writer is replaced with emptyWriter
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, err := NewConsoleHandler(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConsoleHandler() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && h == nil {
				t.Error("NewConsoleHandler() returned nil handler")
			}
		})
	}
}

func TestConsoleHandlerHandle(t *testing.T) {
	cfg := &ConsoleHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		TimeHandler: "RFC3339",
		AddSource:   false,
	}

	h, err := NewConsoleHandler(cfg)
	if err != nil {
		t.Fatalf("NewConsoleHandler() error = %v", err)
	}

	now := time.Now()
	record := slog.NewRecord(now, slog.LevelInfo, "test message", 0)

	err = h.Handle(context.Background(), record)
	if err != nil {
		t.Errorf("Handle() error = %v", err)
	}
}

func TestConsoleHandlerHandleWithAttrs(t *testing.T) {
	cfg := &ConsoleHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		TimeHandler: "RFC3339",
		AddSource:   false,
	}

	h, err := NewConsoleHandler(cfg)
	if err != nil {
		t.Fatalf("NewConsoleHandler() error = %v", err)
	}

	now := time.Now()
	record := slog.NewRecord(now, slog.LevelInfo, "test message", 0)
	record.AddAttrs(slog.String("key", "value"), slog.Int("number", 42))

	err = h.Handle(context.Background(), record)
	if err != nil {
		t.Errorf("Handle() error = %v", err)
	}
}

func TestConsoleHandlerWithAttrs(t *testing.T) {
	cfg := &ConsoleHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		TimeHandler: "RFC3339",
		AddSource:   false,
	}

	h, err := NewConsoleHandler(cfg)
	if err != nil {
		t.Fatalf("NewConsoleHandler() error = %v", err)
	}

	attrs := []slog.Attr{
		slog.String("key1", "value1"),
		slog.Int("key2", 42),
	}

	newHandler := h.WithAttrs(attrs)
	if newHandler == nil {
		t.Error("WithAttrs() returned nil")
	}

	consoleHandler, ok := newHandler.(*consoleHandler)
	if !ok {
		t.Error("WithAttrs() did not return consoleHandler")
	}

	if consoleHandler == h {
		t.Error("WithAttrs() should return a new handler")
	}
}

func TestConsoleHandlerWithGroup(t *testing.T) {
	cfg := &ConsoleHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		TimeHandler: "RFC3339",
		AddSource:   false,
	}

	h, err := NewConsoleHandler(cfg)
	if err != nil {
		t.Fatalf("NewConsoleHandler() error = %v", err)
	}

	newHandler := h.WithGroup("test_group")
	if newHandler == nil {
		t.Error("WithGroup() returned nil")
	}

	consoleHandler, ok := newHandler.(*consoleHandler)
	if !ok {
		t.Error("WithGroup() did not return consoleHandler")
	}

	if len(consoleHandler.groups) != 1 || consoleHandler.groups[0] != "test_group" {
		t.Errorf("WithGroup() groups = %v, want [test_group]", consoleHandler.groups)
	}
}

func TestConsoleHandlerTimeHandlers(t *testing.T) {
	timeHandlers := []string{
		"second",
		"millis",
		"nanos",
		"RFC3339",
		"2006-01-02 15:04:05",
	}

	for _, th := range timeHandlers {
		t.Run(th, func(t *testing.T) {
			cfg := &ConsoleHandlerConfig{
				CommonHandlerConfig: CommonHandlerConfig{
					Level:         slog.LevelInfo,
					AddTrace:      false,
					AddErrVerbose: false,
				},
				TimeHandler: th,
				AddSource:   false,
			}

			h, err := NewConsoleHandler(cfg)
			if err != nil {
				t.Fatalf("NewConsoleHandler() error = %v", err)
			}

			now := time.Now()
			record := slog.NewRecord(now, slog.LevelInfo, "test message", 0)

			err = h.Handle(context.Background(), record)
			if err != nil {
				t.Errorf("Handle() error = %v", err)
			}
		})
	}
}

func TestConsoleHandlerAddSource(t *testing.T) {
	cfg := &ConsoleHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		TimeHandler: "RFC3339",
		AddSource:   true,
	}

	h, err := NewConsoleHandler(cfg)
	if err != nil {
		t.Fatalf("NewConsoleHandler() error = %v", err)
	}

	// Create a record with PC set
	now := time.Now()
	record := slog.NewRecord(now, slog.LevelInfo, "test message", 0)
	// The PC should be set automatically when creating a record from log call

	err = h.Handle(context.Background(), record)
	if err != nil {
		t.Errorf("Handle() error = %v", err)
	}
}

func TestConsoleHandlerAddTrace(t *testing.T) {
	cfg := &ConsoleHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      true,
			AddErrVerbose: false,
		},
		TimeHandler: "RFC3339",
		AddSource:   false,
	}

	h, err := NewConsoleHandler(cfg)
	if err != nil {
		t.Fatalf("NewConsoleHandler() error = %v", err)
	}

	now := time.Now()
	record := slog.NewRecord(now, slog.LevelInfo, "test message", 0)

	err = h.Handle(context.Background(), record)
	if err != nil {
		t.Errorf("Handle() error = %v", err)
	}
}

func TestConsoleHandlerLevels(t *testing.T) {
	levels := []slog.Level{
		slog.LevelDebug,
		slog.LevelInfo,
		slog.LevelWarn,
		slog.LevelError,
	}

	for _, level := range levels {
		t.Run(level.String(), func(t *testing.T) {
			cfg := &ConsoleHandlerConfig{
				CommonHandlerConfig: CommonHandlerConfig{
					Level:         level,
					AddTrace:      false,
					AddErrVerbose: false,
				},
				TimeHandler: "RFC3339",
				AddSource:   false,
			}

			h, err := NewConsoleHandler(cfg)
			if err != nil {
				t.Fatalf("NewConsoleHandler() error = %v", err)
			}

			if h.(*consoleHandler).lv != level {
				t.Errorf("Expected level %v, got %v", level, h.(*consoleHandler).lv)
			}
		})
	}
}

func TestConsoleHandlerLevelMessages(t *testing.T) {
	// Test that consoleLevelMsg has all required levels
	requiredLevels := []slog.Level{
		slog.LevelDebug,
		slog.LevelInfo,
		slog.LevelWarn,
		slog.LevelError,
	}

	for _, level := range requiredLevels {
		if _, ok := consoleLevelMsg[level]; !ok {
			t.Errorf("consoleLevelMsg missing level %v", level)
		}
	}
}

func TestConsoleHandlerEnabled(t *testing.T) {
	cfg := &ConsoleHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		TimeHandler: "RFC3339",
		AddSource:   false,
	}

	h, err := NewConsoleHandler(cfg)
	if err != nil {
		t.Fatalf("NewConsoleHandler() error = %v", err)
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

func TestConsoleHandlerAddSourceHandle(t *testing.T) {
	cfg := &ConsoleHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		TimeHandler: "RFC3339",
		AddSource:   true,
	}

	h, err := NewConsoleHandler(cfg)
	if err != nil {
		t.Fatalf("NewConsoleHandler() error = %v", err)
	}

	ch := h.(*consoleHandler)

	// Create a minimal record for testing
	now := time.Now()
	record := slog.NewRecord(now, slog.LevelInfo, "test", 0)

	buf := buffer.Get()
	ch.addSourceHandle(&record, buf)

	// The source handle should add file and line info if available
	result := buf.String()
	// Just verify it doesn't panic - the actual content depends on runtime
	_ = result
}

func TestConsoleHandlerMultipleWithAttrs(t *testing.T) {
	cfg := &ConsoleHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		TimeHandler: "RFC3339",
		AddSource:   false,
	}

	h, err := NewConsoleHandler(cfg)
	if err != nil {
		t.Fatalf("NewConsoleHandler() error = %v", err)
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

func TestConsoleHandlerMultipleWithGroup(t *testing.T) {
	cfg := &ConsoleHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		TimeHandler: "RFC3339",
		AddSource:   false,
	}

	h, err := NewConsoleHandler(cfg)
	if err != nil {
		t.Fatalf("NewConsoleHandler() error = %v", err)
	}

	// Chain multiple WithGroup calls
	h1 := h.WithGroup("group1")
	h2 := h1.WithGroup("group2")

	consoleHandler, ok := h2.(*consoleHandler)
	if !ok {
		t.Fatal("WithGroup() did not return consoleHandler")
	}

	if len(consoleHandler.groups) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(consoleHandler.groups))
	}

	if consoleHandler.groups[0] != "group1" || consoleHandler.groups[1] != "group2" {
		t.Errorf("Groups = %v, want [group1, group2]", consoleHandler.groups)
	}
}

func TestConsoleHandlerWithEmptyAttrs(t *testing.T) {
	cfg := &ConsoleHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		TimeHandler: "RFC3339",
		AddSource:   false,
	}

	h, err := NewConsoleHandler(cfg)
	if err != nil {
		t.Fatalf("NewConsoleHandler() error = %v", err)
	}

	emptyAttrs := []slog.Attr{}
	newHandler := h.WithAttrs(emptyAttrs)

	if newHandler == nil {
		t.Error("WithAttrs(empty) returned nil")
	}
}

func TestConsoleHandlerWithAttrsAffectsOutput(t *testing.T) {
	mw := newMockWriter()
	cfg := &ConsoleHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelInfo,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		TimeHandler: "RFC3339",
		AddSource:   false,
		Writer:      mw,
	}

	h, err := NewConsoleHandler(cfg)
	if err != nil {
		t.Fatalf("NewConsoleHandler() error = %v", err)
	}
	h = h.WithAttrs([]slog.Attr{slog.String("service", "yggdrasil")})

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)
	if err := h.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	attrs := parseConsoleAttrsBlock(t, mw.String())
	if attrs["service"] != "yggdrasil" {
		t.Fatalf("WithAttrs field missing from console output, got service=%v", attrs["service"])
	}
}

func TestConsoleHandlerWithAttrsAndWithGroupOrder(t *testing.T) {
	newRecord := func() slog.Record {
		record := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)
		record.AddAttrs(slog.String("dynamic", "d"))
		return record
	}

	buildHandler := func(writer *mockWriter) slog.Handler {
		cfg := &ConsoleHandlerConfig{
			CommonHandlerConfig: CommonHandlerConfig{
				Level:         slog.LevelInfo,
				AddTrace:      false,
				AddErrVerbose: false,
			},
			TimeHandler: "RFC3339",
			AddSource:   false,
			Writer:      writer,
		}
		h, err := NewConsoleHandler(cfg)
		if err != nil {
			t.Fatalf("NewConsoleHandler() error = %v", err)
		}
		return h
	}

	mw1 := newMockWriter()
	h1 := buildHandler(mw1).WithAttrs([]slog.Attr{slog.String("static", "s")}).WithGroup("scope")
	record1 := newRecord()
	if err := h1.Handle(context.Background(), record1); err != nil {
		t.Fatalf("Handle() error for attrs->group order = %v", err)
	}
	attrs1 := parseConsoleAttrsBlock(t, mw1.String())
	if attrs1["static"] != "s" {
		t.Fatalf("attrs->group should keep static attr at root, got static=%v", attrs1["static"])
	}
	scope1, ok := attrs1["scope"].(map[string]interface{})
	if !ok {
		t.Fatalf("attrs->group should contain scope object, got %T", attrs1["scope"])
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
	attrs2 := parseConsoleAttrsBlock(t, mw2.String())
	if _, ok := attrs2["static"]; ok {
		t.Fatalf(
			"group->attrs should nest static attr in scope, got root static=%v",
			attrs2["static"],
		)
	}
	scope2, ok := attrs2["scope"].(map[string]interface{})
	if !ok {
		t.Fatalf("group->attrs should contain scope object, got %T", attrs2["scope"])
	}
	if scope2["static"] != "s" || scope2["dynamic"] != "d" {
		t.Fatalf("group->attrs should keep both attrs in scope, got scope=%v", scope2)
	}
}

func TestConsoleHandlerUnknownLevelFallback(t *testing.T) {
	mw := newMockWriter()
	cfg := &ConsoleHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level:         slog.LevelDebug,
			AddTrace:      false,
			AddErrVerbose: false,
		},
		TimeHandler: "RFC3339",
		AddSource:   false,
		Writer:      mw,
	}

	h, err := NewConsoleHandler(cfg)
	if err != nil {
		t.Fatalf("NewConsoleHandler() error = %v", err)
	}

	level := slog.Level(10)
	record := slog.NewRecord(time.Now(), level, "test message", 0)
	if err := h.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	output := mw.String()
	if !strings.Contains(output, level.String()) {
		t.Fatalf("output should contain fallback level string %q, got %q", level.String(), output)
	}
}

type consoleHandlerLogValuer struct{}

func (consoleHandlerLogValuer) LogValue() slog.Value {
	return slog.StringValue("resolved_by_logvaluer")
}

func TestConsoleHandlerHandleAnyTypeMatrix(t *testing.T) {
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
				got, ok := v.(bool)
				if !ok || !got {
					t.Fatalf("value = %T(%v), want bool(true)", v, v)
				}
			},
		},
		{
			name: "int",
			attr: slog.Any("value", 42),
			want: func(t *testing.T, v any) {
				got, ok := v.(float64)
				if !ok || got != 42 {
					t.Fatalf("value = %T(%v), want float64(42)", v, v)
				}
			},
		},
		{
			name: "float64",
			attr: slog.Any("value", 3.14),
			want: func(t *testing.T, v any) {
				got, ok := v.(float64)
				if !ok || got != 3.14 {
					t.Fatalf("value = %T(%v), want float64(3.14)", v, v)
				}
			},
		},
		{
			name: "string",
			attr: slog.Any("value", "hello"),
			want: func(t *testing.T, v any) {
				got, ok := v.(string)
				if !ok || got != "hello" {
					t.Fatalf("value = %T(%v), want string(\"hello\")", v, v)
				}
			},
		},
		{
			name: "map",
			attr: slog.Any("value", map[string]any{"k": "v", "n": 1}),
			want: func(t *testing.T, v any) {
				got, ok := v.(map[string]any)
				if !ok {
					t.Fatalf("value = %T(%v), want map[string]any", v, v)
				}
				if got["k"] != "v" || got["n"] != float64(1) {
					t.Fatalf("map value = %v, want map[k:v n:1]", got)
				}
			},
		},
		{
			name: "struct",
			attr: slog.Any("value", sampleStruct{Name: "alice", Age: 18}),
			want: func(t *testing.T, v any) {
				got, ok := v.(map[string]any)
				if !ok {
					t.Fatalf("value = %T(%v), want map[string]any", v, v)
				}
				if got["name"] != "alice" || got["age"] != float64(18) {
					t.Fatalf("struct value = %v, want name=alice age=18", got)
				}
			},
		},
		{
			name: "error",
			attr: slog.Any("value", errors.New("boom")),
			want: func(t *testing.T, v any) {
				got, ok := v.(string)
				if !ok || got != "boom" {
					t.Fatalf("value = %T(%v), want string(\"boom\")", v, v)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := newMockWriter()
			h, err := NewConsoleHandler(&ConsoleHandlerConfig{
				CommonHandlerConfig: CommonHandlerConfig{
					Level: slog.LevelInfo,
				},
				TimeHandler: "RFC3339",
				Writer:      mw,
			})
			if err != nil {
				t.Fatalf("NewConsoleHandler() error = %v", err)
			}

			record := slog.NewRecord(time.Now(), slog.LevelInfo, "any type", 0)
			record.AddAttrs(tt.attr)
			if err := h.Handle(context.Background(), record); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}

			attrs := parseConsoleAttrsBlock(t, mw.String())
			tt.want(t, attrs["value"])
		})
	}
}

func TestConsoleHandlerHandleGroupSemantics(t *testing.T) {
	mw := newMockWriter()
	h, err := NewConsoleHandler(&ConsoleHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level: slog.LevelInfo,
		},
		TimeHandler: "RFC3339",
		Writer:      mw,
	})
	if err != nil {
		t.Fatalf("NewConsoleHandler() error = %v", err)
	}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "group test", 0)
	record.AddAttrs(
		slog.Group("req", slog.String("path", "/v1/health")),
		slog.Group("", slog.Int("flat", 7)),
	)
	if err := h.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	attrs := parseConsoleAttrsBlock(t, mw.String())
	req, ok := attrs["req"].(map[string]any)
	if !ok {
		t.Fatalf("req should be nested object, got %T(%v)", attrs["req"], attrs["req"])
	}
	if req["path"] != "/v1/health" {
		t.Fatalf("req.path = %v, want /v1/health", req["path"])
	}
	if attrs["flat"] != float64(7) {
		t.Fatalf("flat = %v, want 7", attrs["flat"])
	}
}

func TestConsoleHandlerHandleLogValuer(t *testing.T) {
	mw := newMockWriter()
	h, err := NewConsoleHandler(&ConsoleHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level: slog.LevelInfo,
		},
		TimeHandler: "RFC3339",
		Writer:      mw,
	})
	if err != nil {
		t.Fatalf("NewConsoleHandler() error = %v", err)
	}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "logvaluer test", 0)
	record.AddAttrs(slog.Any("value", consoleHandlerLogValuer{}))
	if err := h.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	attrs := parseConsoleAttrsBlock(t, mw.String())
	if attrs["value"] != "resolved_by_logvaluer" {
		t.Fatalf("value = %v, want resolved_by_logvaluer", attrs["value"])
	}
}

func parseConsoleAttrsBlock(t *testing.T, output string) map[string]interface{} {
	t.Helper()

	start := strings.IndexByte(output, '{')
	end := strings.LastIndexByte(output, '}')
	if start < 0 || end < start {
		t.Fatalf("console output missing attrs block: %q", output)
	}

	raw := output[start : end+1]
	var attrs map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &attrs); err != nil {
		t.Fatalf("console attrs block is not valid JSON: %v, raw=%q", err, raw)
	}
	return attrs
}
