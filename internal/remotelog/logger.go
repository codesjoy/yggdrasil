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

package remotelog

import (
	"context"
	"log/slog"
	"sync"
)

var (
	mu     sync.RWMutex
	logger = slog.Default()
)

type levelFilterHandler struct {
	level slog.Level
	base  slog.Handler
}

func (h *levelFilterHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level && h.base.Enabled(ctx, level)
}

func (h *levelFilterHandler) Handle(ctx context.Context, record slog.Record) error {
	return h.base.Handle(ctx, record)
}

func (h *levelFilterHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &levelFilterHandler{
		level: h.level,
		base:  h.base.WithAttrs(attrs),
	}
}

func (h *levelFilterHandler) WithGroup(name string) slog.Handler {
	return &levelFilterHandler{
		level: h.level,
		base:  h.base.WithGroup(name),
	}
}

// Init configures the internal remote logger.
func Init(level slog.Level, handler slog.Handler) {
	if next := New(level, handler); next != nil {
		SetLogger(next)
	}
}

// New builds a remote logger with the framework remote-level filter.
func New(level slog.Level, handler slog.Handler) *slog.Logger {
	if handler == nil {
		return nil
	}
	return slog.New(&levelFilterHandler{level: level, base: handler})
}

// SetLogger swaps the internal remote logger and returns the previous logger.
func SetLogger(next *slog.Logger) *slog.Logger {
	if next == nil {
		next = slog.Default()
	}
	mu.Lock()
	defer mu.Unlock()
	prev := logger
	logger = next
	return prev
}

// Logger returns the internal remote logger.
func Logger() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return logger
}
