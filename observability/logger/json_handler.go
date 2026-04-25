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
	"fmt"
	"io"
	"log/slog"
)

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
