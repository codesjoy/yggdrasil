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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- setStoppedLocked ---

func TestApp_SetStoppedLocked(t *testing.T) {
	t.Run("transitions from running to stopped", func(t *testing.T) {
		app := newTestApp(t, "test")
		app.state = lifecycleStateRunning
		app.setStoppedLocked()
		assert.Equal(t, lifecycleStateStopped, app.state)
	})

	t.Run("new state stays new", func(t *testing.T) {
		app := newTestApp(t, "test")
		app.state = lifecycleStateNew
		app.setStoppedLocked()
		assert.Equal(t, lifecycleStateNew, app.state)
	})
}

// --- finishRun ---

func TestApp_FinishRun(t *testing.T) {
	t.Run("nil waitDone", func(t *testing.T) {
		app := newTestApp(t, "test")
		app.state = lifecycleStateRunning
		app.finishRun(nil)
		assert.Equal(t, lifecycleStateStopped, app.state)
		assert.Nil(t, app.waitDone)
	})

	t.Run("non-nil waitDone closes channel", func(t *testing.T) {
		app := newTestApp(t, "test")
		app.waitDone = make(chan struct{})
		done := app.waitDone

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-done
		}()

		app.finishRun(nil)
		wg.Wait()
		assert.Nil(t, app.waitDone)
		assert.Nil(t, app.waitErr)
	})

	t.Run("records error", func(t *testing.T) {
		app := newTestApp(t, "test")
		app.waitDone = make(chan struct{})
		expectedErr := errApplicationAlreadyRunning
		app.finishRun(expectedErr)
		assert.Equal(t, expectedErr, app.waitErr)
	})
}

// --- prepareStartLocked ---

func TestApp_PrepareStartLocked_AlreadyRunning(t *testing.T) {
	t.Run("already running returns error", func(t *testing.T) {
		app := newTestApp(t, "test")
		app.state = lifecycleStateRunning
		err := app.prepareStartLocked(context.Background())
		require.Error(t, err)
		assert.ErrorIs(t, err, errApplicationAlreadyRunning)
	})

	t.Run("serving returns error", func(t *testing.T) {
		app := newTestApp(t, "test")
		app.state = lifecycleStateServing
		err := app.prepareStartLocked(context.Background())
		require.Error(t, err)
		assert.ErrorIs(t, err, errApplicationAlreadyRunning)
	})
}

func TestApp_PrepareStartLocked_AlreadyStopped(t *testing.T) {
	t.Run("stopped returns error", func(t *testing.T) {
		app := newTestApp(t, "test")
		app.state = lifecycleStateStopped
		err := app.prepareStartLocked(context.Background())
		require.Error(t, err)
		assert.ErrorIs(t, err, errRestartUnsupported)
	})
}

// --- ensureClientReadyLocked ---

func TestApp_EnsureClientReadyLocked_Stopped(t *testing.T) {
	t.Run("stopped returns error", func(t *testing.T) {
		app := newTestApp(t, "test")
		app.state = lifecycleStateStopped
		err := app.ensureClientReadyLocked(context.Background())
		require.Error(t, err)
		assert.ErrorIs(t, err, errRestartUnsupported)
	})
}

// --- stopResources ---

func TestApp_StopResources(t *testing.T) {
	t.Run("initialized app returns nil", func(t *testing.T) {
		data := minimalV3Config("grpc")
		app, _ := newInitializedAppWithConfig(t, "test-app", data)
		err := app.stopResources()
		require.NoError(t, err)
	})
}

// --- stopConfigWatchLocked ---

func TestApp_StopConfigWatchLocked(t *testing.T) {
	t.Run("nil stopWatch is no-op", func(t *testing.T) {
		app := newTestApp(t, "test")
		app.stopConfigWatchLocked()
		assert.Nil(t, app.stopWatch)
	})

	t.Run("calls and clears stopWatch", func(t *testing.T) {
		app := newTestApp(t, "test")
		called := false
		app.stopWatch = func() { called = true }
		app.stopConfigWatchLocked()
		assert.True(t, called)
		assert.Nil(t, app.stopWatch)
	})
}

// --- reloadAsync ---

func TestApp_ReloadAsync(t *testing.T) {
	t.Run("does not panic on stopped app", func(t *testing.T) {
		app := newTestApp(t, "test")
		// reloadAsync launches a goroutine that calls Reload
		// Just ensure it doesn't panic
		app.reloadAsync()
	})
}
