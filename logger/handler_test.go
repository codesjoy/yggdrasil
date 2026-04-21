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
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/v2/config"
)

type emptyHandler struct{}

func (emptyHandler) Enabled(context.Context, slog.Level) bool { return true }

func (emptyHandler) Handle(context.Context, slog.Record) error { return nil }

func (h emptyHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h emptyHandler) WithGroup(string) slog.Handler { return h }

func resetHandlerBuilders() {
	handlerBuilderMu.Lock()
	defer handlerBuilderMu.Unlock()
	handlerBuilder = make(map[string]HandlerBuilder)
	handlerBuilder["json"] = newJSONHandler
	handlerBuilder["text"] = newConsoleHandler
}

func TestDefaultHandlerBuilderAliases(t *testing.T) {
	resetHandlerBuilders()

	if _, err := GetHandlerBuilder("console"); err != nil {
		t.Fatalf("GetHandlerBuilder(console) error = %v", err)
	}
	if _, err := GetHandlerBuilder("text"); err != nil {
		t.Fatalf("GetHandlerBuilder(text) error = %v", err)
	}
}

func TestHandlerBuilderBaseRegistryOnlyContainsJSONAndText(t *testing.T) {
	resetHandlerBuilders()

	handlerBuilderMu.RLock()
	defer handlerBuilderMu.RUnlock()
	if len(handlerBuilder) != 2 {
		t.Fatalf("base handler registry size = %d, want 2", len(handlerBuilder))
	}
	if _, ok := handlerBuilder["json"]; !ok {
		t.Fatalf("base handler registry missing json")
	}
	if _, ok := handlerBuilder["text"]; !ok {
		t.Fatalf("base handler registry missing text")
	}
	if _, ok := handlerBuilder["console"]; ok {
		t.Fatalf("base handler registry should not contain console key")
	}
}

func TestHandlerBuilderConcurrentRegisterAndGet(t *testing.T) {
	resetHandlerBuilders()

	const total = 64
	var wg sync.WaitGroup
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("handler_%d", i)
			RegisterHandlerBuilder(name, func(string, config.Value) (slog.Handler, error) {
				return emptyHandler{}, nil
			})
			_, _ = GetHandlerBuilder(name)
			_, _ = GetHandlerBuilder("json")
		}(i)
	}
	wg.Wait()

	for i := 0; i < total; i++ {
		name := fmt.Sprintf("handler_%d", i)
		if _, err := GetHandlerBuilder(name); err != nil {
			t.Fatalf("GetHandlerBuilder(%q) error = %v", name, err)
		}
	}
}

type stubConfigValue struct {
	scanFn func(any) error
}

func (v stubConfigValue) Bool(def ...bool) bool {
	if len(def) > 0 {
		return def[0]
	}
	return false
}

func (v stubConfigValue) Int(def ...int) int {
	if len(def) > 0 {
		return def[0]
	}
	return 0
}

func (v stubConfigValue) Int64(def ...int64) int64 {
	if len(def) > 0 {
		return def[0]
	}
	return 0
}

func (v stubConfigValue) String(def ...string) string {
	if len(def) > 0 {
		return def[0]
	}
	return ""
}

func (v stubConfigValue) Float64(def ...float64) float64 {
	if len(def) > 0 {
		return def[0]
	}
	return 0
}

func (v stubConfigValue) Duration(def ...time.Duration) time.Duration {
	if len(def) > 0 {
		return def[0]
	}
	return 0
}

func (v stubConfigValue) StringSlice(def ...[]string) []string {
	if len(def) > 0 {
		return def[0]
	}
	return nil
}

func (v stubConfigValue) StringMap(def ...map[string]string) map[string]string {
	if len(def) > 0 {
		return def[0]
	}
	return map[string]string{}
}

func (v stubConfigValue) Map(def ...map[string]any) map[string]any {
	if len(def) > 0 {
		return def[0]
	}
	return map[string]any{}
}

func (v stubConfigValue) Scan(val any) error {
	if v.scanFn != nil {
		return v.scanFn(val)
	}
	return nil
}

func (v stubConfigValue) Bytes(def ...[]byte) []byte {
	if len(def) > 0 {
		return def[0]
	}
	return nil
}

func TestNewJSONHandlerFromBuilderSuccess(t *testing.T) {
	resetWriterBuilders()

	var out strings.Builder
	writerType := "handler_json_success_writer_type"
	writerName := "handler_json_success_writer_name"
	handlerName := "handler_json_success_handler_name"

	RegisterWriterBuilder(writerType, func(string) (io.Writer, error) {
		return &out, nil
	})
	if err := config.Set(config.Join(config.KeyBase, "logger", "writer", writerName, "type"), writerType); err != nil {
		t.Fatalf("config.Set(writer type) error = %v", err)
	}
	if err := config.Set(config.Join(config.KeyBase, "logger", "handler", handlerName, "config"), map[string]any{
		"level": "info",
	}); err != nil {
		t.Fatalf("config.Set(handler config) error = %v", err)
	}

	h, err := newJSONHandler(writerName, config.Get(config.Join(config.KeyBase, "logger", "handler", handlerName, "config")))
	if err != nil {
		t.Fatalf("newJSONHandler() error = %v", err)
	}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "json builder success", 0)
	record.AddAttrs(slog.String("k", "v"))
	if err := h.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if !strings.Contains(out.String(), "json builder success") {
		t.Fatalf("output missing message, got %q", out.String())
	}
}

func TestNewConsoleHandlerFromBuilderSuccess(t *testing.T) {
	resetWriterBuilders()

	var out strings.Builder
	writerType := "handler_console_success_writer_type"
	writerName := "handler_console_success_writer_name"
	handlerName := "handler_console_success_handler_name"

	RegisterWriterBuilder(writerType, func(string) (io.Writer, error) {
		return &out, nil
	})
	if err := config.Set(config.Join(config.KeyBase, "logger", "writer", writerName, "type"), writerType); err != nil {
		t.Fatalf("config.Set(writer type) error = %v", err)
	}
	if err := config.Set(config.Join(config.KeyBase, "logger", "handler", handlerName, "config"), map[string]any{
		"level": "info",
	}); err != nil {
		t.Fatalf("config.Set(handler config) error = %v", err)
	}

	h, err := newConsoleHandler(writerName, config.Get(config.Join(config.KeyBase, "logger", "handler", handlerName, "config")))
	if err != nil {
		t.Fatalf("newConsoleHandler() error = %v", err)
	}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "console builder success", 0)
	record.AddAttrs(slog.String("k", "v"))
	if err := h.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	line := out.String()
	if !strings.Contains(line, "console builder success") || !strings.Contains(line, "k=v") {
		t.Fatalf("unexpected text output: %q", line)
	}
}

func TestNewHandlerFromBuilderScanError(t *testing.T) {
	wantErr := errors.New("scan boom")
	val := stubConfigValue{
		scanFn: func(any) error {
			return wantErr
		},
	}

	if _, err := newJSONHandler("any", val); !errors.Is(err, wantErr) {
		t.Fatalf("newJSONHandler() err = %v, want %v", err, wantErr)
	}
	if _, err := newConsoleHandler("any", val); !errors.Is(err, wantErr) {
		t.Fatalf("newConsoleHandler() err = %v, want %v", err, wantErr)
	}
}

func TestNewHandlerFromBuilderGetWriterErrors(t *testing.T) {
	resetWriterBuilders()

	successScanVal := stubConfigValue{
		scanFn: func(v any) error {
			switch cfg := v.(type) {
			case *JSONHandlerConfig:
				cfg.Level = slog.LevelInfo
			case *ConsoleHandlerConfig:
				cfg.Level = slog.LevelInfo
			}
			return nil
		},
	}

	missingWriter := "handler_missing_writer_name"
	if err := config.Set(config.Join(config.KeyBase, "logger", "writer", missingWriter, "type"), "writer_builder_not_exist"); err != nil {
		t.Fatalf("config.Set(missing writer) error = %v", err)
	}
	if _, err := newJSONHandler(missingWriter, successScanVal); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("newJSONHandler() missing builder err = %v, want not found", err)
	}

	builderErr := errors.New("builder boom")
	writerType := "handler_error_writer_type"
	writerName := "handler_error_writer_name"
	RegisterWriterBuilder(writerType, func(string) (io.Writer, error) {
		return nil, builderErr
	})
	if err := config.Set(config.Join(config.KeyBase, "logger", "writer", writerName, "type"), writerType); err != nil {
		t.Fatalf("config.Set(error writer type) error = %v", err)
	}
	if _, err := newConsoleHandler(writerName, successScanVal); !errors.Is(err, builderErr) {
		t.Fatalf("newConsoleHandler() err = %v, want %v", err, builderErr)
	}
}
