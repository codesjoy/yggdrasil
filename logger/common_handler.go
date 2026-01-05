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

// Package logger provides a slog.Handler implementation for logging.
package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"slices"

	"github.com/codesjoy/yggdrasil/v2/logger/buffer"
	"go.opentelemetry.io/otel/trace"
)

// CommonHandlerConfig defines the configuration for the commonHandler.
type CommonHandlerConfig struct {
	Level         slog.Level `yaml:"level"         json:"level"`
	AddTrace      bool       `yaml:"addTrace"      json:"addTrace"`
	AddErrVerbose bool       `yaml:"addErrVerbose" json:"addErrVerbose"`

	writer io.Writer
	objEnc ObjectEncoder
}

type commonHandler struct {
	lv                slog.Level
	preformattedAttrs []byte
	groups            []string
	nOpenGroups       int

	objEnc ObjectEncoder
	writer io.Writer

	errorHandle func(key string, err error, enc ObjectEncoder)
	traceHandle func(ctx context.Context, enc ObjectEncoder)
}

func newCommonHandler(cfg *CommonHandlerConfig) (*commonHandler, error) {
	h := &commonHandler{
		lv:          cfg.Level,
		traceHandle: func(context.Context, ObjectEncoder) {},
		writer:      cfg.writer,
		objEnc:      cfg.objEnc,
	}

	if h.writer == nil {
		h.writer = emptyWriter{}
	}

	if h.objEnc == nil {
		return nil, fmt.Errorf("no object encoder")
	}

	if cfg.AddTrace {
		h.traceHandle = h.addTrace
	}

	if cfg.AddErrVerbose {
		h.errorHandle = h.handleErrorWithVerbose
	} else {
		h.errorHandle = h.handleErrorOnlyError
	}

	return h, nil
}

// Enabled reports whether the multiHandler handles records at the given level.
func (h *commonHandler) Enabled(_ context.Context, level slog.Level) bool {
	return h.lv <= level
}

func (h *commonHandler) WithAttrs(attrs []slog.Attr) *commonHandler {
	newNec := h.clone()

	// Pre-format the attributes
	buf := (*buffer.Buffer)(&newNec.preformattedAttrs)
	objEnc := h.objEnc.Get()
	objEnc.SetBuffer(buf)
	h.openGroups(objEnc)
	for _, attr := range attrs {
		h.encodeSlogAttr(attr, objEnc)
	}
	return newNec
}

func (h *commonHandler) WithGroup(group string) *commonHandler {
	newNec := h.clone()
	newNec.groups = append(newNec.groups, group)
	return newNec
}

func (h *commonHandler) clone() *commonHandler {
	newEnc := *h
	newEnc.preformattedAttrs = slices.Clone(h.preformattedAttrs)
	newEnc.groups = slices.Clone(h.groups)
	return &newEnc
}

func (h *commonHandler) openGroups(objEnc ObjectEncoder) {
	for _, n := range h.groups[h.nOpenGroups:] {
		objEnc.OpenNamespace(n)
	}
	h.nOpenGroups = len(h.groups)
}

func (h *commonHandler) addTrace(ctx context.Context, enc ObjectEncoder) {
	spanCtx := trace.SpanFromContext(ctx).SpanContext()
	if spanCtx.IsValid() {
		enc.AddString("trace_id", spanCtx.TraceID().String())
		enc.AddString("span_id", spanCtx.SpanID().String())
	}
}

func (h *commonHandler) handleErrorWithVerbose(key string, err error, enc ObjectEncoder) {
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

func (h *commonHandler) handleErrorOnlyError(key string, err error, enc ObjectEncoder) {
	basic := err.Error()
	enc.AddString(key, basic)
}

func (h *commonHandler) encodeSlogAttr(attr slog.Attr, objEnc ObjectEncoder) {
	switch attr.Value.Kind() {
	case slog.KindBool:
		objEnc.AddBool(attr.Key, attr.Value.Bool())
	case slog.KindDuration:
		objEnc.AddDuration(attr.Key, attr.Value.Duration())
	case slog.KindFloat64:
		objEnc.AddFloat64(attr.Key, attr.Value.Float64())
	case slog.KindInt64:
		objEnc.AddInt64(attr.Key, attr.Value.Int64())
	case slog.KindString:
		objEnc.AddString(attr.Key, attr.Value.String())
	case slog.KindTime:
		objEnc.AddTime(attr.Key, attr.Value.Time())
	case slog.KindUint64:
		objEnc.AddUint64(attr.Key, attr.Value.Uint64())
	case slog.KindGroup:
		for _, groupAttr := range attr.Value.Group() {
			h.encodeSlogAttr(groupAttr, objEnc)
		}
	case slog.KindAny:
		a := attr.Value.Any()
		switch v := a.(type) {
		case error:
			h.errorHandle(attr.Key, v, objEnc)
		default:
			objEnc.AddAny(attr.Key, v)
		}
	default:
	}
}
