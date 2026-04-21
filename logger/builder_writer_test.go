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
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/natefinch/lumberjack.v2"
)

func TestInterceptorConfigReturnsCopyAndNilForMissing(t *testing.T) {
	Configure(Settings{
		Interceptors: map[string]map[string]any{
			"metrics": {
				"enabled": true,
				"rate":    0.5,
			},
		},
	})

	got := InterceptorConfig("metrics")
	require.NotNil(t, got)
	require.Equal(t, true, got["enabled"])
	require.Equal(t, 0.5, got["rate"])

	// Returned config should be a copy, not a shared map.
	got["enabled"] = false
	got["new_key"] = "new_value"

	again := InterceptorConfig("metrics")
	require.Equal(t, true, again["enabled"])
	require.Equal(t, 0.5, again["rate"])
	_, exists := again["new_key"]
	require.False(t, exists)

	require.Nil(t, InterceptorConfig("missing"))
}

func TestGetHandlerBuilderConsoleAlias(t *testing.T) {
	resetHandlerBuilders()

	sentinelErr := errors.New("sentinel text builder error")
	RegisterHandlerBuilder("text", func(string, map[string]any) (slog.Handler, error) {
		return nil, sentinelErr
	})

	builder, err := GetHandlerBuilder("console")
	require.NoError(t, err)

	_, err = builder("default", nil)
	require.ErrorIs(t, err, sentinelErr)
}

func TestGetHandlerBuilderNotFound(t *testing.T) {
	resetHandlerBuilders()

	_, err := GetHandlerBuilder("missing")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestNewConsoleHandlerBuildsFromConfigMap(t *testing.T) {
	resetWriterBuilders()
	resetHandlerBuilders()

	var out strings.Builder
	RegisterWriterBuilder("memory", func(string) (io.Writer, error) {
		return &out, nil
	})
	Configure(Settings{
		Writers: map[string]WriterSpec{
			"default": {Type: "memory"},
		},
	})

	handler, err := newConsoleHandler("default", map[string]any{"level": "info"})
	require.NoError(t, err)

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "hello", 0)
	record.AddAttrs(slog.String("k", "v"))
	require.NoError(t, handler.Handle(context.Background(), record))

	require.Contains(t, out.String(), "hello")
	require.Contains(t, out.String(), "k=v")
}

func TestNewConsoleHandlerDecodeError(t *testing.T) {
	resetWriterBuilders()
	resetHandlerBuilders()
	Configure(Settings{
		Writers: map[string]WriterSpec{
			"default": {Type: "console"},
		},
	})

	_, err := newConsoleHandler("default", map[string]any{
		"level": map[string]any{"bad": "value"},
	})
	require.Error(t, err)
}

func TestNewConsoleHandlerWriterError(t *testing.T) {
	resetWriterBuilders()
	resetHandlerBuilders()
	Configure(Settings{Writers: map[string]WriterSpec{}})

	_, err := newConsoleHandler("missing", map[string]any{"level": "info"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "writer builder")
}

func TestGetWriterDefaultFallbackToConsole(t *testing.T) {
	resetWriterBuilders()

	want := &strings.Builder{}
	RegisterWriterBuilder("console", func(string) (io.Writer, error) {
		return want, nil
	})
	Configure(Settings{
		Writers: map[string]WriterSpec{
			"default": {},
		},
	})

	got, err := GetWriter("default")
	require.NoError(t, err)
	require.Same(t, want, got)
}

func TestGetWriterBuilderNotFound(t *testing.T) {
	resetWriterBuilders()

	_, err := GetWriterBuilder("missing")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestGetWriterBuilderErrorPath(t *testing.T) {
	resetWriterBuilders()

	sentinelErr := errors.New("writer build failed")
	RegisterWriterBuilder("failing", func(string) (io.Writer, error) {
		return nil, sentinelErr
	})
	Configure(Settings{
		Writers: map[string]WriterSpec{
			"bad": {Type: "failing"},
		},
	})

	_, err := GetWriter("bad")
	require.ErrorIs(t, err, sentinelErr)
}

func TestNewFileWriterBuildsLumberjackFromConfig(t *testing.T) {
	Configure(Settings{
		Writers: map[string]WriterSpec{
			"file": {
				Type: "file",
				Config: map[string]any{
					"Filename":   "/tmp/yggdrasil.log",
					"MaxSize":    128,
					"MaxBackups": 5,
					"MaxAge":     7,
					"Compress":   true,
				},
			},
		},
	})

	w, err := newFileWriter("file")
	require.NoError(t, err)

	lj, ok := w.(*lumberjack.Logger)
	require.True(t, ok)
	require.Equal(t, "/tmp/yggdrasil.log", lj.Filename)
	require.Equal(t, 128, lj.MaxSize)
	require.Equal(t, 5, lj.MaxBackups)
	require.Equal(t, 7, lj.MaxAge)
	require.True(t, lj.Compress)
}

func TestNewFileWriterDecodeError(t *testing.T) {
	Configure(Settings{
		Writers: map[string]WriterSpec{
			"file": {
				Type: "file",
				Config: map[string]any{
					"MaxSize": map[string]any{"bad": "value"},
				},
			},
		},
	})

	_, err := newFileWriter("file")
	require.Error(t, err)
}

func TestEmptyWriterWriteReturnsZeroAndNil(t *testing.T) {
	n, err := (emptyWriter{}).Write([]byte("hello"))
	require.NoError(t, err)
	require.Zero(t, n)
}
