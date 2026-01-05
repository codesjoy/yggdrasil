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
	"log/slog"
	"slices"

	"go.opentelemetry.io/otel/trace"
)

// ContextHandle is the function that handles context.
type ContextHandle func(ctx context.Context, enc ObjectEncoder)

// DefaultContextHandle is the default ContextHandle.
func DefaultContextHandle(ctx context.Context, enc ObjectEncoder) error {
	spanCtx := trace.SpanFromContext(ctx).SpanContext()
	if spanCtx.IsValid() {
		enc.AddString("trace_id", spanCtx.TraceID().String())
		enc.AddString("span_id", spanCtx.SpanID().String())
	}
	return nil
}

// ErrorHandle is the function that handles errors.
type ErrorHandle func(key string, err error, enc ObjectEncoder)

// DefaultErrorHandle is the default ErrorHandle.
func DefaultErrorHandle(key string, err error, enc ObjectEncoder) {
	basic := err.Error()
	enc.AddString(key, basic)
	switch e := err.(type) {
	case fmt.Formatter:
		verbose := fmt.Sprintf("%+v", e)
		if verbose != basic {
			enc.AddString(key+"Verbose", verbose)
		}
	}
}

type multiHandler struct {
	handlers []slog.Handler
}

// Enabled reports whether the multiHandler handles records at the given level.
func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range h.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) Handle(ctx context.Context, record slog.Record) error {
	var errs []error
	for _, item := range h.handlers {
		if item.Enabled(ctx, record.Level) {
			if err := item.Handle(ctx, record); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

func (h *multiHandler) WithGroup(group string) slog.Handler {
	clone := *h
	clone.handlers = slices.Clone(h.handlers)
	for i, item := range h.handlers {
		clone.handlers[i] = item.WithGroup(group)
	}
	return &clone
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if countEmptyGroups(attrs) == len(attrs) {
		return h
	}
	clone := *h
	clone.handlers = slices.Clone(h.handlers)
	for i, item := range h.handlers {
		clone.handlers[i] = item.WithAttrs(attrs)
	}
	return &clone
}

// countEmptyGroups returns the number of empty group values in its argument.
func countEmptyGroups(as []slog.Attr) int {
	n := 0
	for _, a := range as {
		if a.Value.Kind() == slog.KindGroup && len(a.Value.Group()) == 0 {
			n++
		}
	}
	return n
}
