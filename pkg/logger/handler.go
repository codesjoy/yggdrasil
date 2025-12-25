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
	"log/slog"

	"github.com/codesjoy/yggdrasil/pkg/config"
)

func init() {
	RegisterHandlerBuilder("json", newJSONHandler)
	RegisterHandlerBuilder("console", newConsoleHandler)
}

// HandlerBuilder is the interface for building a slog.Handler.
type HandlerBuilder func(writer string, val config.Value) (slog.Handler, error)

var handlerBuilder = make(map[string]HandlerBuilder)

// RegisterHandlerBuilder registers a handler builder for the given type.
func RegisterHandlerBuilder(typeName string, f HandlerBuilder) {
	handlerBuilder[typeName] = f
}

// GetHandlerBuilder returns the handler builder for the given type.
func GetHandlerBuilder(typeName string) (HandlerBuilder, error) {
	f, ok := handlerBuilder[typeName]
	if !ok {
		return nil, fmt.Errorf("handler builder for type %s not found", typeName)
	}
	return f, nil
}

func newJSONHandler(writer string, val config.Value) (slog.Handler, error) {
	cfg := &JSONHandlerConfig{}
	if err := val.Scan(cfg); err != nil {
		return nil, err
	}
	w, err := GetWriter(writer)
	if err != nil {
		return nil, err
	}
	cfg.Writer = w
	return NewJSONHandler(cfg)
}

func newConsoleHandler(writer string, val config.Value) (slog.Handler, error) {
	cfg := &ConsoleHandlerConfig{}
	if err := val.Scan(cfg); err != nil {
		return nil, err
	}
	w, err := GetWriter(writer)
	if err != nil {
		return nil, err
	}
	cfg.Writer = w
	return NewConsoleHandler(cfg)
}
