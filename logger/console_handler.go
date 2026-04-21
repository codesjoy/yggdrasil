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

// Package logger provides a console handler for the slog package.
package logger

import (
	"fmt"
	"io"
	"log/slog"
)

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
