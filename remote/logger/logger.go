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

// Package logger provides a logger for the remote package.
package logger

import (
	"context"
	"log/slog"
)

var logger = slog.Default()

type handler struct {
	lv slog.Level
	slog.Handler
}

func (h *handler) Enabled(_ context.Context, level slog.Level) bool {
	return h.lv <= level
}

// InitLogger initializes the logger.
func InitLogger(level slog.Level, h slog.Handler) {
	logger = slog.New(&handler{lv: level, Handler: h})
}

// GetLogger returns the logger.
func GetLogger() *slog.Logger {
	return logger
}
