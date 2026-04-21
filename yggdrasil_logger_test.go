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

package yggdrasil

import (
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/logger"
)

type testLogWriter struct {
	mu    sync.Mutex
	lines []string
}

func (w *testLogWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lines = append(w.lines, string(append([]byte(nil), p...)))
	return len(p), nil
}

func (w *testLogWriter) LastLine() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.lines) == 0 {
		return ""
	}
	return w.lines[len(w.lines)-1]
}

func TestInitLoggerTextTypeUsesOfficialTextHandler(t *testing.T) {
	w := &testLogWriter{}
	writerType := "test_logger_default_text"
	logger.RegisterWriterBuilder(writerType, func(string) (io.Writer, error) {
		return w, nil
	})

	require.NoError(
		t,
		config.Set(config.Join(config.KeyBase, "logger", "writer", "default", "type"), writerType),
	)
	require.NoError(
		t,
		config.Set(config.Join(config.KeyBase, "logger", "handler", "default", "type"), "text"),
	)
	require.NoError(t, config.Set(config.Join(config.KeyBase, "logger", "handler", "default", "writer"), "default"))
	require.NoError(
		t,
		config.Set(config.Join(config.KeyBase, "logger", "handler", "default", "config", "level"), "info"),
	)
	require.NoError(t, config.Set(config.Join(config.KeyBase, "remote", "logger_level"), "error"))

	require.NoError(t, initLogger())
	slog.Info("default text", slog.String("k", "v"))

	line := strings.TrimSpace(w.LastLine())
	require.Contains(t, line, "level=INFO")
	require.Contains(t, line, `msg="default text"`)
	require.Contains(t, line, "k=v")
}

func TestInitLoggerConsoleAliasStillWorks(t *testing.T) {
	w := &testLogWriter{}
	writerType := "test_logger_console_alias"
	logger.RegisterWriterBuilder(writerType, func(string) (io.Writer, error) {
		return w, nil
	})

	require.NoError(
		t,
		config.Set(config.Join(config.KeyBase, "logger", "writer", "default", "type"), writerType),
	)
	require.NoError(
		t,
		config.Set(config.Join(config.KeyBase, "logger", "handler", "default", "type"), "console"),
	)
	require.NoError(t, config.Set(config.Join(config.KeyBase, "logger", "handler", "default", "writer"), "default"))
	require.NoError(
		t,
		config.Set(config.Join(config.KeyBase, "logger", "handler", "default", "config", "level"), "info"),
	)
	require.NoError(t, config.Set(config.Join(config.KeyBase, "remote", "logger_level"), "error"))

	require.NoError(t, initLogger())
	slog.Info("console alias", slog.String("k", "v"))

	line := strings.TrimSpace(w.LastLine())
	require.Contains(t, line, "level=INFO")
	require.Contains(t, line, `msg="console alias"`)
	require.Contains(t, line, "k=v")
}

func TestInitLoggerJSONTypeUsesOfficialJSONHandler(t *testing.T) {
	w := &testLogWriter{}
	writerType := "test_logger_default_json"
	logger.RegisterWriterBuilder(writerType, func(string) (io.Writer, error) {
		return w, nil
	})

	require.NoError(
		t,
		config.Set(config.Join(config.KeyBase, "logger", "writer", "default", "type"), writerType),
	)
	require.NoError(
		t,
		config.Set(config.Join(config.KeyBase, "logger", "handler", "default", "type"), "json"),
	)
	require.NoError(t, config.Set(config.Join(config.KeyBase, "logger", "handler", "default", "writer"), "default"))
	require.NoError(
		t,
		config.Set(config.Join(config.KeyBase, "logger", "handler", "default", "config", "level"), "info"),
	)
	require.NoError(t, config.Set(config.Join(config.KeyBase, "remote", "logger_level"), "error"))

	require.NoError(t, initLogger())
	slog.Info("default json", slog.Int("id", 11))

	line := strings.TrimSpace(w.LastLine())
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(line), &got))
	require.Equal(t, "INFO", got["level"])
	require.Equal(t, "default json", got["msg"])
	require.Equal(t, float64(11), got["id"])
}

func TestInitLoggerIgnoresLegacyDefaultLevel(t *testing.T) {
	w := &testLogWriter{}
	writerType := "test_logger_legacy_level_ignored"
	logger.RegisterWriterBuilder(writerType, func(string) (io.Writer, error) {
		return w, nil
	})

	require.NoError(
		t,
		config.Set(config.Join(config.KeyBase, "logger", "writer", "default", "type"), writerType),
	)
	require.NoError(
		t,
		config.Set(config.Join(config.KeyBase, "logger", "handler", "default", "type"), "json"),
	)
	require.NoError(t, config.Set(config.Join(config.KeyBase, "logger", "handler", "default", "writer"), "default"))
	require.NoError(
		t,
		config.Set(config.Join(config.KeyBase, "logger", "handler", "default", "config", "level"), "error"),
	)
	require.NoError(
		t,
		config.Set(config.Join(config.KeyBase, "logger", "handler", "default", "level"), "debug"),
	)
	require.NoError(t, config.Set(config.Join(config.KeyBase, "remote", "logger_level"), "error"))

	require.NoError(t, initLogger())
	slog.Info("should be filtered by config.level")
	require.Len(t, w.lines, 0)

	slog.Error("should pass")
	require.Len(t, w.lines, 1)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(w.LastLine())), &got))
	require.Equal(t, "should pass", got["msg"])
}

func TestInitLoggerIgnoresDeprecatedHandlerKeys(t *testing.T) {
	w := &testLogWriter{}
	writerType := "test_logger_deprecated_keys_ignored"
	logger.RegisterWriterBuilder(writerType, func(string) (io.Writer, error) {
		return w, nil
	})

	require.NoError(
		t,
		config.Set(config.Join(config.KeyBase, "logger", "writer", "default", "type"), writerType),
	)
	require.NoError(
		t,
		config.Set(config.Join(config.KeyBase, "logger", "handler", "default", "type"), "json"),
	)
	require.NoError(t, config.Set(config.Join(config.KeyBase, "logger", "handler", "default", "writer"), "default"))
	require.NoError(
		t,
		config.Set(config.Join(config.KeyBase, "logger", "handler", "default", "config", "level"), "info"),
	)
	require.NoError(
		t,
		config.Set(
			config.Join(config.KeyBase, "logger", "handler", "default", "config", "add_err_verbose"),
			true,
		),
	)
	require.NoError(
		t,
		config.Set(
			config.Join(config.KeyBase, "logger", "handler", "default", "config", "time_handler"),
			"millis",
		),
	)
	require.NoError(
		t,
		config.Set(
			config.Join(config.KeyBase, "logger", "handler", "default", "config", "encoder", "spaced"),
			true,
		),
	)
	require.NoError(t, config.Set(config.Join(config.KeyBase, "remote", "logger_level"), "error"))

	require.NoError(t, initLogger())
	slog.Error("deprecated keys", slog.Any("error", io.EOF))

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(w.LastLine())), &got))
	require.Equal(t, "deprecated keys", got["msg"])
	require.Equal(t, "EOF", got["error"])
	_, ok := got["errorVerbose"]
	require.False(t, ok)
}
