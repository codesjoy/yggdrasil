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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type handlerState struct {
	enabled    bool
	handled    int
	attrCalls  int
	groupCalls int
}

type fakeHandler struct {
	state *handlerState
}

func (h *fakeHandler) Enabled(context.Context, slog.Level) bool {
	return h.state.enabled
}

func (h *fakeHandler) Handle(context.Context, slog.Record) error {
	h.state.handled++
	return nil
}

func (h *fakeHandler) WithAttrs([]slog.Attr) slog.Handler {
	h.state.attrCalls++
	return &fakeHandler{state: h.state}
}

func (h *fakeHandler) WithGroup(string) slog.Handler {
	h.state.groupCalls++
	return &fakeHandler{state: h.state}
}

func TestLevelFilterHandlerDelegatesAndPreservesLevel(t *testing.T) {
	state := &handlerState{enabled: true}
	base := &fakeHandler{state: state}
	handler := &levelFilterHandler{
		level: slog.LevelWarn,
		base:  base,
	}

	require.False(t, handler.Enabled(context.Background(), slog.LevelInfo))
	require.True(t, handler.Enabled(context.Background(), slog.LevelError))
	require.NoError(
		t,
		handler.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)),
	)

	withAttrs, ok := handler.WithAttrs([]slog.Attr{slog.String("k", "v")}).(*levelFilterHandler)
	require.True(t, ok)
	require.Equal(t, slog.LevelWarn, withAttrs.level)

	withGroup, ok := handler.WithGroup("remote").(*levelFilterHandler)
	require.True(t, ok)
	require.Equal(t, slog.LevelWarn, withGroup.level)
	require.Equal(t, 1, state.handled)
	require.Equal(t, 1, state.attrCalls)
	require.Equal(t, 1, state.groupCalls)
}

func TestInitAndLogger(t *testing.T) {
	original := logger
	defer func() {
		logger = original
	}()

	before := Logger()
	Init(slog.LevelInfo, nil)
	require.Same(t, before, Logger())

	state := &handlerState{enabled: true}
	Init(slog.LevelInfo, &fakeHandler{state: state})
	require.NotSame(t, before, Logger())

	Logger().With("service", "demo").WithGroup("remote").Info("hello")
	require.Equal(t, 1, state.handled)
	require.Equal(t, 1, state.attrCalls)
	require.Equal(t, 1, state.groupCalls)
}
