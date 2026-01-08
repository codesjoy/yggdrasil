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
