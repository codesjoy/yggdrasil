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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
)

// --- effectiveResolved ---

func TestEffectiveResolved(t *testing.T) {
	t.Run("nil result uses fallback", func(t *testing.T) {
		fallback := settings.Resolved{App: settings.Application{Name: "fallback"}}
		result := effectiveResolved(nil, fallback)
		assert.Equal(t, "fallback", result.App.Name)
	})

	t.Run("non-nil result uses resolved", func(t *testing.T) {
		fallback := settings.Resolved{App: settings.Application{Name: "fallback"}}
		planResult := &yassembly.Result{
			EffectiveResolved: settings.Resolved{App: settings.Application{Name: "planned"}},
		}
		result := effectiveResolved(planResult, fallback)
		assert.Equal(t, "planned", result.App.Name)
	})
}

// --- selectedCapabilityBindings ---

func TestSelectedCapabilityBindings(t *testing.T) {
	t.Run("nil result uses fallback", func(t *testing.T) {
		fallback := settings.Resolved{
			CapabilityBindings: map[string][]string{"k": {"v"}},
		}
		result := selectedCapabilityBindings(nil, fallback)
		assert.Equal(t, map[string][]string{"k": {"v"}}, result)
	})

	t.Run("non-nil uses result bindings", func(t *testing.T) {
		planResult := &yassembly.Result{
			CapabilityBindings: map[string][]string{"a": {"b"}},
		}
		result := selectedCapabilityBindings(planResult, settings.Resolved{})
		assert.Equal(t, map[string][]string{"a": {"b"}}, result)
	})
}

// --- cloneCapabilityBindings ---

func TestCloneCapabilityBindings(t *testing.T) {
	t.Run("nil map returns empty", func(t *testing.T) {
		result := cloneCapabilityBindings(nil)
		assert.Equal(t, map[string][]string{}, result)
	})

	t.Run("normal map cloned", func(t *testing.T) {
		original := map[string][]string{"k": {"a", "b"}}
		cloned := cloneCapabilityBindings(original)
		assert.Equal(t, original, cloned)
	})

	t.Run("mutation safety", func(t *testing.T) {
		original := map[string][]string{"k": {"a", "b"}}
		cloned := cloneCapabilityBindings(original)
		cloned["k"][0] = "modified"
		assert.Equal(t, "a", original["k"][0])
	})
}

// --- normalizeAssemblyError ---

func TestNormalizeAssemblyError(t *testing.T) {
	t.Run("nil error returns nil", func(t *testing.T) {
		assert.Nil(t, normalizeAssemblyError(assemblyStagePrepare, nil))
	})

	t.Run("assembly.Error passthrough with stage override", func(t *testing.T) {
		original := yassembly.NewError(yassembly.ErrComposeFailed, "", "msg", nil, nil)
		result := normalizeAssemblyError(assemblyStageCompose, original)
		require.NotNil(t, result)
		assert.Equal(t, yassembly.ErrComposeFailed, result.Code)
		assert.Equal(t, "compose", result.Stage)
	})

	t.Run("plain error wrap", func(t *testing.T) {
		result := normalizeAssemblyError(assemblyStagePrepare, errors.New("plain"))
		require.NotNil(t, result)
		assert.Equal(t, yassembly.ErrRuntimeSurfaceUnavailable, result.Code)
		assert.Equal(t, "prepare", result.Stage)
	})
}

// --- defaultAssemblyErrorCode ---

func TestDefaultAssemblyErrorCode(t *testing.T) {
	tests := []struct {
		stage assemblyStage
		code  yassembly.ErrorCode
	}{
		{assemblyStagePrepare, yassembly.ErrRuntimeSurfaceUnavailable},
		{assemblyStageCompose, yassembly.ErrComposeFailed},
		{assemblyStageInstall, yassembly.ErrInstallValidationFailed},
		{assemblyStageReload, yassembly.ErrRuntimeReconcileFailed},
	}
	for _, tt := range tests {
		t.Run(string(tt.stage), func(t *testing.T) {
			assert.Equal(t, tt.code, defaultAssemblyErrorCode(tt.stage))
		})
	}
}

// --- cloneAssemblyError ---

func TestCloneAssemblyError(t *testing.T) {
	t.Run("nil error returns nil", func(t *testing.T) {
		assert.Nil(t, cloneAssemblyError(nil))
	})

	t.Run("clones with independent copy", func(t *testing.T) {
		original := yassembly.NewError(
			yassembly.ErrComposeFailed,
			"compose",
			"msg",
			nil,
			map[string]string{"key": "val"},
		)
		cloned := cloneAssemblyError(original)
		require.NotNil(t, cloned)
		assert.Equal(t, original.Code, cloned.Code)
		assert.Equal(t, original.Stage, cloned.Stage)
		assert.Equal(t, original.Message, cloned.Message)
		cloned.Context["key"] = "modified"
		assert.Equal(t, "val", original.Context["key"])
	})
}

// --- wrapAssemblyStageError ---

func TestWrapAssemblyStageError(t *testing.T) {
	t.Run("nil error returns nil", func(t *testing.T) {
		assert.Nil(t, wrapAssemblyStageError("prepare", nil))
	})

	t.Run("assembly.Error passthrough with empty stage filled", func(t *testing.T) {
		original := yassembly.NewError(yassembly.ErrComposeFailed, "", "msg", nil, nil)
		wrapped := wrapAssemblyStageError("compose", original)
		require.NotNil(t, wrapped)
		var assemblyErr *yassembly.Error
		require.ErrorAs(t, wrapped, &assemblyErr)
		assert.Equal(t, "compose", assemblyErr.Stage)
	})

	t.Run("assembly.Error with existing stage kept", func(t *testing.T) {
		original := yassembly.NewError(yassembly.ErrComposeFailed, "existing", "msg", nil, nil)
		wrapped := wrapAssemblyStageError("compose", original)
		var assemblyErr *yassembly.Error
		require.ErrorAs(t, wrapped, &assemblyErr)
		assert.Equal(t, "existing", assemblyErr.Stage)
	})

	t.Run("plain error wrap", func(t *testing.T) {
		wrapped := wrapAssemblyStageError("install", errors.New("plain"))
		require.NotNil(t, wrapped)
		var assemblyErr *yassembly.Error
		require.ErrorAs(t, wrapped, &assemblyErr)
		assert.Equal(t, yassembly.ErrInstallValidationFailed, assemblyErr.Code)
		assert.Equal(t, "install", assemblyErr.Stage)
	})
}

// --- assemblyErrorState.Record ---

func TestAssemblyErrorState_Record(t *testing.T) {
	t.Run("records error at stage", func(t *testing.T) {
		state := &assemblyErrorState{}
		err := yassembly.NewError(yassembly.ErrComposeFailed, "compose", "test", nil, nil)
		state.record(assemblyStageCompose, err)
		assert.Equal(t, err, state.compose.err)
	})

	t.Run("nil error clears stage", func(t *testing.T) {
		state := &assemblyErrorState{}
		err := yassembly.NewError(yassembly.ErrComposeFailed, "compose", "test", nil, nil)
		state.record(assemblyStageCompose, err)
		state.record(assemblyStageCompose, nil)
		assert.Nil(t, state.compose.err)
	})

	t.Run("updates latest", func(t *testing.T) {
		state := &assemblyErrorState{}
		err := yassembly.NewError(yassembly.ErrComposeFailed, "compose", "test", nil, nil)
		state.record(assemblyStageCompose, err)
		assert.Equal(t, err, state.latest)
		assert.Equal(t, assemblyStageCompose, state.latestStage)
	})
}

// --- assemblyErrorState.Clear ---

func TestAssemblyErrorState_Clear(t *testing.T) {
	t.Run("clears error at stage", func(t *testing.T) {
		state := &assemblyErrorState{}
		err := yassembly.NewError(yassembly.ErrComposeFailed, "compose", "test", nil, nil)
		state.record(assemblyStageCompose, err)
		state.clear(assemblyStageCompose)
		assert.Nil(t, state.compose.err)
		assert.Nil(t, state.latest)
	})
}

// --- assemblyErrorState.Snapshot ---

func TestAssemblyErrorState_Snapshot(t *testing.T) {
	t.Run("empty snapshot", func(t *testing.T) {
		state := &assemblyErrorState{}
		snap := state.snapshot()
		assert.Nil(t, snap.lastError)
		assert.Nil(t, snap.errors.Prepare)
	})

	t.Run("after record", func(t *testing.T) {
		state := &assemblyErrorState{}
		err := yassembly.NewError(yassembly.ErrComposeFailed, "compose", "test", nil, nil)
		state.record(assemblyStageCompose, err)
		snap := state.snapshot()
		require.NotNil(t, snap.errors.Compose)
		assert.Equal(t, yassembly.ErrComposeFailed, snap.errors.Compose.Code)
	})

	t.Run("after clear", func(t *testing.T) {
		state := &assemblyErrorState{}
		err := yassembly.NewError(yassembly.ErrComposeFailed, "compose", "test", nil, nil)
		state.record(assemblyStageCompose, err)
		state.clear(assemblyStageCompose)
		snap := state.snapshot()
		assert.Nil(t, snap.errors.Compose)
		assert.Nil(t, snap.lastError)
	})
}

// --- assemblyErrorState.RecalculateLatest ---

func TestAssemblyErrorState_RecalculateLatest(t *testing.T) {
	t.Run("multiple stages latest reflects most recent", func(t *testing.T) {
		state := &assemblyErrorState{}
		err1 := yassembly.NewError(
			yassembly.ErrRuntimeSurfaceUnavailable,
			"prepare",
			"err1",
			nil,
			nil,
		)
		err2 := yassembly.NewError(yassembly.ErrComposeFailed, "compose", "err2", nil, nil)
		state.record(assemblyStagePrepare, err1)
		state.record(assemblyStageCompose, err2)
		assert.Equal(t, err2, state.latest)
		assert.Equal(t, assemblyStageCompose, state.latestStage)
	})
}

// --- preparedAssembly.Close ---

func TestPreparedAssembly_Close(t *testing.T) {
	t.Run("nil receiver returns nil", func(t *testing.T) {
		var pa *preparedAssembly
		assert.Nil(
			t,
			pa.Close(nil), //nolint:staticcheck // intentional: testing nil context handling
		)
	})

	t.Run("nil CloseFunc returns nil", func(t *testing.T) {
		pa := &preparedAssembly{}
		assert.Nil(
			t,
			pa.Close(nil), //nolint:staticcheck // intentional: testing nil context handling
		)
	})

	t.Run("calls CloseFunc", func(t *testing.T) {
		called := false
		pa := &preparedAssembly{
			CloseFunc: func(_ context.Context) error {
				called = true
				return nil
			},
		}
		assert.Nil(
			t,
			pa.Close(nil), //nolint:staticcheck // intentional: testing nil context handling
		)
		assert.True(t, called)
	})
}
