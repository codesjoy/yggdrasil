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

package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/internal/settings"
)

// --- runtimeShutdown ---

type shutdownable struct {
	called bool
}

func (s *shutdownable) Shutdown(_ context.Context) error {
	s.called = true
	return nil
}

type closable struct {
	called bool
}

func (c *closable) Close() error {
	c.called = true
	return nil
}

func TestRuntimeShutdown(t *testing.T) {
	t.Run("Shutdown interface", func(t *testing.T) {
		s := &shutdownable{}
		fn := runtimeShutdown(s)
		require.NotNil(t, fn)
		require.NoError(t, fn(context.Background()))
		assert.True(t, s.called)
	})

	t.Run("Closer interface", func(t *testing.T) {
		c := &closable{}
		fn := runtimeShutdown(c)
		require.NotNil(t, fn)
		require.NoError(t, fn(context.Background()))
		assert.True(t, c.called)
	})

	t.Run("non-matching type returns nil", func(t *testing.T) {
		fn := runtimeShutdown("string")
		assert.Nil(t, fn)
	})

	t.Run("nil returns nil", func(t *testing.T) {
		fn := runtimeShutdown(nil)
		assert.Nil(t, fn)
	})
}

// --- stage/commit/rollback foundation snapshot ---

func TestApp_StageCommitRollbackFoundationSnapshot(t *testing.T) {
	t.Run("stage and commit lifecycle", func(t *testing.T) {
		app := newTestApp(t, "test")
		snap := &Snapshot{Resolved: settings.Resolved{}}

		app.stageFoundationSnapshot(snap)
		assert.Equal(t, snap, app.preparedFoundationSnapshot)

		app.commitFoundationSnapshot(snap)
		assert.Equal(t, snap, app.foundationSnapshot)
		assert.Nil(t, app.preparedFoundationSnapshot)
	})

	t.Run("stage and rollback lifecycle", func(t *testing.T) {
		app := newTestApp(t, "test")
		snap := &Snapshot{Resolved: settings.Resolved{}}

		app.stageFoundationSnapshot(snap)
		assert.Equal(t, snap, app.preparedFoundationSnapshot)

		app.rollbackFoundationSnapshot(snap)
		assert.Nil(t, app.preparedFoundationSnapshot)
	})

	t.Run("rollback different snapshot is no-op", func(t *testing.T) {
		app := newTestApp(t, "test")
		snap1 := &Snapshot{Resolved: settings.Resolved{App: settings.Application{Name: "1"}}}
		snap2 := &Snapshot{Resolved: settings.Resolved{App: settings.Application{Name: "2"}}}

		app.stageFoundationSnapshot(snap1)
		app.rollbackFoundationSnapshot(snap2)
		assert.Equal(t, snap1, app.preparedFoundationSnapshot)
	})
}

// --- swapTracerShutdown / swapMeterShutdown ---

func TestApp_SwapTracerShutdown(t *testing.T) {
	t.Run("swaps and returns previous", func(t *testing.T) {
		app := newTestApp(t, "test")
		prev := func(context.Context) error { return nil }
		next := func(context.Context) error { return nil }

		app.swapTracerShutdown(prev)
		returned := app.swapTracerShutdown(next)
		assert.NotNil(t, returned)
	})
}

func TestApp_SwapMeterShutdown(t *testing.T) {
	t.Run("swaps and returns previous", func(t *testing.T) {
		app := newTestApp(t, "test")
		prev := func(context.Context) error { return nil }
		next := func(context.Context) error { return nil }

		app.swapMeterShutdown(prev)
		returned := app.swapMeterShutdown(next)
		assert.NotNil(t, returned)
	})
}

// --- initRegistry ---

func TestApp_InitRegistry(t *testing.T) {
	t.Run("nil app returns without panic", func(t *testing.T) {
		var app *App
		app.initRegistry()
	})

	t.Run("nil opts returns without panic", func(t *testing.T) {
		app := &App{}
		app.initRegistry()
	})

	t.Run("nil snapshot returns without panic", func(t *testing.T) {
		app := newTestApp(t, "test")
		app.initRegistry()
	})
}

// --- foundationSnapshotForRuntime ---

func TestApp_FoundationSnapshotForRuntime(t *testing.T) {
	t.Run("prefers prepared snapshot", func(t *testing.T) {
		app := newTestApp(t, "test")
		prepared := &Snapshot{
			Resolved: settings.Resolved{App: settings.Application{Name: "prepared"}},
		}
		foundation := &Snapshot{
			Resolved: settings.Resolved{App: settings.Application{Name: "foundation"}},
		}

		app.stageFoundationSnapshot(prepared)
		app.commitFoundationSnapshot(foundation)

		result := app.foundationSnapshotForRuntime()
		assert.Equal(t, "prepared", result.Resolved.App.Name)
	})

	t.Run("falls back to foundation snapshot", func(t *testing.T) {
		app := newTestApp(t, "test")
		foundation := &Snapshot{
			Resolved: settings.Resolved{App: settings.Application{Name: "foundation"}},
		}

		app.commitFoundationSnapshot(foundation)

		result := app.foundationSnapshotForRuntime()
		assert.Equal(t, "foundation", result.Resolved.App.Name)
	})

	t.Run("falls back to runtime snapshot", func(t *testing.T) {
		app := newTestApp(t, "test")
		runtimeSnap := &Snapshot{
			Resolved: settings.Resolved{App: settings.Application{Name: "runtime"}},
		}

		app.setRuntimeSnapshot(runtimeSnap)

		result := app.foundationSnapshotForRuntime()
		assert.Equal(t, "runtime", result.Resolved.App.Name)
	})
}
