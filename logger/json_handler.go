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
	"io"
	"log/slog"
	"runtime"

	"github.com/codesjoy/yggdrasil/v2/logger/buffer"
)

// JSONHandlerConfig is the configuration for JSON handler.
type JSONHandlerConfig struct {
	CommonHandlerConfig
	Encoder   JSONEncoderConfig `yaml:"encoder"   json:"encoder"`
	AddSource bool              `yaml:"addSource" json:"addSource"`

	Writer io.Writer
}

type jsonHandler struct {
	*commonHandler
	sourceHandle func(*slog.Record, ObjectEncoder)
}

// NewJSONHandler creates a new JSON handler.
func NewJSONHandler(cfg *JSONHandlerConfig) (slog.Handler, error) {
	enc, err := NewJSONEncoder(&cfg.Encoder)
	if err != nil {
		return nil, err
	}
	cfg.objEnc = enc
	cfg.writer = cfg.Writer

	commHandler, err := newCommonHandler(&cfg.CommonHandlerConfig)
	if err != nil {
		return nil, err
	}

	h := &jsonHandler{
		commonHandler: commHandler,
		sourceHandle:  func(*slog.Record, ObjectEncoder) {},
	}

	if cfg.AddSource {
		h.sourceHandle = h.addSourceHandle
	}

	return h, nil
}

func (h *jsonHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.commonHandler = h.commonHandler.WithAttrs(attrs)
	return &clone
}

func (h *jsonHandler) WithGroup(group string) slog.Handler {
	clone := *h
	clone.commonHandler = h.commonHandler.WithGroup(group)
	return &clone
}

func (h *jsonHandler) Handle(ctx context.Context, r slog.Record) error {
	objEnc := h.objEnc.Get()
	buf := buffer.Get()
	buf.AppendByte('{')
	objEnc.SetBuffer(buf)
	objEnc.AddTime(slog.TimeKey, r.Time)
	objEnc.AddString(slog.LevelKey, r.Level.String())
	h.sourceHandle(&r, objEnc)
	objEnc.AddString(slog.MessageKey, r.Message)
	h.traceHandle(ctx, objEnc)
	h.openGroups(objEnc)
	r.Attrs(func(attr slog.Attr) bool {
		h.encodeSlogAttr(attr, objEnc)
		return true
	})
	objEnc.CloseNamespace(h.nOpenGroups)
	buf.AppendByte('}')
	buf.AppendByte('\n')
	_, err := h.writer.Write(buf.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (h *jsonHandler) addSourceHandle(r *slog.Record, obj ObjectEncoder) {
	fs := runtime.CallersFrames([]uintptr{r.PC})
	f, _ := fs.Next()
	if f.Function != "" {
		obj.AddString("function", f.Function)
	}
	if f.File != "" {
		obj.AddString("file", f.File)
	}
	if f.Line != 0 {
		obj.AddInt64("line", int64(f.Line))
	}
}
