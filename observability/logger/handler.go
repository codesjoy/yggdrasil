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

// Package logger provides a slog.Handler for logging.
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/codesjoy/yggdrasil/v3/config"
)

// RegisterBuiltinHandlers registers built-in handler builders.
func RegisterBuiltinHandlers() {
	for name, builder := range BuiltinHandlerBuilders() {
		RegisterHandlerBuilder(name, builder)
	}
}

// BuiltinHandlerBuilders returns framework built-in handler providers.
func BuiltinHandlerBuilders() map[string]HandlerBuilder {
	return map[string]HandlerBuilder{
		"json": newJSONHandler,
		"text": newConsoleHandler,
	}
}

// HandlerBuilder is the interface for building a slog.Handler.
type HandlerBuilder func(writer string, cfg map[string]any) (slog.Handler, error)

var (
	handlerBuilder   = make(map[string]HandlerBuilder)
	handlerBuilderMu sync.RWMutex
)

// RegisterHandlerBuilder registers a handler builder for the given type.
func RegisterHandlerBuilder(typeName string, f HandlerBuilder) {
	handlerBuilderMu.Lock()
	defer handlerBuilderMu.Unlock()
	handlerBuilder[typeName] = f
}

// ConfigureHandlerBuilders replaces all handler builders in one shot.
func ConfigureHandlerBuilders(builders map[string]HandlerBuilder) {
	handlerBuilderMu.Lock()
	defer handlerBuilderMu.Unlock()
	next := make(map[string]HandlerBuilder, len(builders))
	for name, builder := range builders {
		next[name] = builder
	}
	handlerBuilder = next
}

// GetHandlerBuilder returns the handler builder for the given type.
func GetHandlerBuilder(typeName string) (HandlerBuilder, error) {
	if typeName == "console" {
		typeName = "text"
	}
	handlerBuilderMu.RLock()
	defer handlerBuilderMu.RUnlock()
	f, ok := handlerBuilder[typeName]
	if !ok {
		return nil, fmt.Errorf("handler builder for type %s not found", typeName)
	}
	return f, nil
}

func newJSONHandler(writer string, cfgMap map[string]any) (slog.Handler, error) {
	cfg := &JSONHandlerConfig{}
	if err := config.NewSnapshot(cfgMap).Decode(cfg); err != nil {
		return nil, err
	}
	w, err := GetWriter(writer)
	if err != nil {
		return nil, err
	}
	cfg.Writer = w
	return NewJSONHandler(cfg)
}

func newConsoleHandler(writer string, cfgMap map[string]any) (slog.Handler, error) {
	cfg := &ConsoleHandlerConfig{}
	if err := config.NewSnapshot(cfgMap).Decode(cfg); err != nil {
		return nil, err
	}
	w, err := GetWriter(writer)
	if err != nil {
		return nil, err
	}
	cfg.Writer = w
	return NewConsoleHandler(cfg)
}

// ConsoleHandlerConfig is the configuration for ConsoleHandler.
type ConsoleHandlerConfig struct {
	Level     slog.Level `mapstructure:"level"      yaml:"level"      json:"level"`
	AddTrace  bool       `mapstructure:"add_trace"  yaml:"add_trace"  json:"add_trace"`
	AddSource bool       `mapstructure:"add_source" yaml:"add_source" json:"add_source"`

	Writer io.Writer
}

// NewConsoleHandler creates a new ConsoleHandler.
func NewConsoleHandler(cfg *ConsoleHandlerConfig) (slog.Handler, error) {
	if cfg == nil {
		return nil, fmt.Errorf("nil ConsoleHandlerConfig")
	}
	w := cfg.Writer
	if w == nil {
		w = emptyWriter{}
	}
	opts := &slog.HandlerOptions{
		AddSource: cfg.AddSource,
		Level:     cfg.Level,
	}
	h := slog.NewTextHandler(w, opts)
	return wrapTraceHandler(h, cfg.AddTrace), nil
}

// JSONHandlerConfig is the configuration for JSON handler.
type JSONHandlerConfig struct {
	Level     slog.Level `mapstructure:"level"      yaml:"level"      json:"level"`
	AddTrace  bool       `mapstructure:"add_trace"  yaml:"add_trace"  json:"add_trace"`
	AddSource bool       `mapstructure:"add_source" yaml:"add_source" json:"add_source"`

	Writer io.Writer
}

// NewJSONHandler creates a new JSON handler.
func NewJSONHandler(cfg *JSONHandlerConfig) (slog.Handler, error) {
	if cfg == nil {
		return nil, fmt.Errorf("nil JSONHandlerConfig")
	}
	w := cfg.Writer
	if w == nil {
		w = emptyWriter{}
	}
	opts := &slog.HandlerOptions{
		AddSource: cfg.AddSource,
		Level:     cfg.Level,
	}
	h := slog.NewJSONHandler(w, opts)
	return wrapTraceHandler(h, cfg.AddTrace), nil
}
