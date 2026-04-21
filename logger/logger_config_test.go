package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func resetWriterBuilders() {
	writerBuilderMu.Lock()
	defer writerBuilderMu.Unlock()
	writerBuilder = map[string]WriterBuilder{
		"file":    newFileWriter,
		"console": newConsoleWriter,
	}
}

func resetHandlerBuilders() {
	handlerBuilderMu.Lock()
	defer handlerBuilderMu.Unlock()
	handlerBuilder = map[string]HandlerBuilder{
		"json": newJSONHandler,
		"text": newConsoleHandler,
	}
}

func TestGetWriterUsesSettingsTree(t *testing.T) {
	resetWriterBuilders()
	Configure(Settings{Writers: map[string]WriterSpec{"default": {Type: "console"}}})

	writer, err := GetWriter("default")
	require.NoError(t, err)
	require.Equal(t, io.Writer(os.Stdout), writer)
}

func TestNewJSONHandlerBuildsFromConfigMap(t *testing.T) {
	resetWriterBuilders()
	resetHandlerBuilders()

	var out strings.Builder
	RegisterWriterBuilder("memory", func(string) (io.Writer, error) {
		return &out, nil
	})
	Configure(Settings{Writers: map[string]WriterSpec{"default": {Type: "memory"}}})

	handler, err := newJSONHandler("default", map[string]any{"level": "info"})
	require.NoError(t, err)

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "hello", 0)
	record.AddAttrs(slog.String("k", "v"))
	require.NoError(t, handler.Handle(context.Background(), record))
	require.Contains(t, out.String(), "hello")
}
