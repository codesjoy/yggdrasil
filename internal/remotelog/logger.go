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
	if handler == nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	logger = slog.New(&levelFilterHandler{level: level, base: handler})
}

// Logger returns the internal remote logger.
func Logger() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return logger
}
