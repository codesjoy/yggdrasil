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
	"context"
	"io"
	"log/slog"
	"runtime"
	"time"

	"github.com/codesjoy/yggdrasil/v2/logger/buffer"
	"github.com/codesjoy/yggdrasil/v2/logger/internal/xcolor"
)

// ConsoleHandlerConfig is the configuration for ConsoleHandler.
type ConsoleHandlerConfig struct {
	CommonHandlerConfig
	TimeHandler string            `mapstructure:"time_handler"`
	Encoder     JSONEncoderConfig `mapstructure:"encoder"`
	AddSource   bool              `mapstructure:"add_source"`

	Writer io.Writer
}

var consoleLevelMsg = map[slog.Level]string{
	slog.LevelDebug: xcolor.Blue("DEBUG"),
	slog.LevelInfo:  xcolor.Green("INFO "),
	slog.LevelWarn:  xcolor.Yellow("WARN "),
	slog.LevelError: xcolor.Red("ERROR"),
}

type consoleHandler struct {
	*commonHandler
	timeHandle   func(time.Time, *buffer.Buffer)
	sourceHandle func(r *slog.Record, buf *buffer.Buffer)
}

// NewConsoleHandler creates a new ConsoleHandler.
func NewConsoleHandler(cfg *ConsoleHandlerConfig) (slog.Handler, error) {
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
	h := &consoleHandler{
		commonHandler: commHandler,
		sourceHandle:  func(*slog.Record, *buffer.Buffer) {},
	}

	if cfg.AddSource {
		h.sourceHandle = h.addSourceHandle
	}

	// TimeEncoder serializes a time.Time to a primitive type.
	switch cfg.TimeHandler {
	case "second":
		h.timeHandle = func(t time.Time, b *buffer.Buffer) {
			nanos := t.UnixNano()
			sec := float64(nanos) / float64(time.Second)
			b.AppendFloat(sec, 64)
		}
	case "millis":
		h.timeHandle = func(t time.Time, b *buffer.Buffer) {
			nanos := t.UnixNano()
			millis := float64(nanos) / float64(time.Millisecond)
			b.AppendFloat(millis, 64)
		}
	case "nanos":
		h.timeHandle = func(t time.Time, b *buffer.Buffer) {
			b.AppendInt(t.UnixNano())
		}
	case "RFC3339", "":
		h.timeHandle = func(t time.Time, b *buffer.Buffer) {
			b.AppendString(t.Format(time.RFC3339))
		}
	default:
		h.timeHandle = func(t time.Time, b *buffer.Buffer) {
			b.AppendString(t.Format(cfg.TimeHandler))
		}
	}

	return h, nil
}

func (h *consoleHandler) Handle(ctx context.Context, r slog.Record) error {
	buf := buffer.Get()
	h.timeHandle(r.Time, buf)
	buf.AppendString("  ")
	buf.AppendString(consoleLevelMsg[r.Level])
	h.sourceHandle(&r, buf)
	buf.AppendString("  ")
	buf.AppendString(r.Message)

	if r.NumAttrs() > 0 {
		buf.AppendString("  ")
		buf.AppendByte('{')
		objEnc := h.objEnc.Get()
		objEnc.SetBuffer(buf)

		h.traceHandle(ctx, objEnc)
		h.openGroups(objEnc)
		r.Attrs(func(attr slog.Attr) bool {
			h.encodeSlogAttr(attr, objEnc)
			return true
		})
		objEnc.CloseNamespace(h.nOpenGroups)
		buf.AppendByte('}')
	}

	buf.AppendByte('\n')
	_, err := h.writer.Write(buf.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (h *consoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.commonHandler = h.commonHandler.WithAttrs(attrs)
	return &clone
}

func (h *consoleHandler) WithGroup(group string) slog.Handler {
	clone := *h
	clone.commonHandler = h.commonHandler.WithGroup(group)
	return &clone
}

func (h *consoleHandler) addSourceHandle(r *slog.Record, buf *buffer.Buffer) {
	fs := runtime.CallersFrames([]uintptr{r.PC})
	f, _ := fs.Next()
	if f.File == "" {
		return
	}
	buf.AppendString("  ")
	buf.AppendString(f.File)
	if f.Line != 0 {
		buf.AppendString(":")
		buf.AppendInt(int64(f.Line))
	}
}
