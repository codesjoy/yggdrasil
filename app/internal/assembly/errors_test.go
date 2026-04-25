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

package assembly

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
)

func TestNormalizeError(t *testing.T) {
	t.Run("nil error returns nil", func(t *testing.T) {
		assert.Nil(t, NormalizeError(StagePrepare, nil))
	})

	t.Run("assembly.Error passthrough with stage override", func(t *testing.T) {
		original := yassembly.NewError(yassembly.ErrComposeFailed, "", "msg", nil, nil)
		result := NormalizeError(StageCompose, original)
		require.NotNil(t, result)
		assert.Equal(t, yassembly.ErrComposeFailed, result.Code)
		assert.Equal(t, "compose", result.Stage)
	})

	t.Run("plain error wrap", func(t *testing.T) {
		result := NormalizeError(StagePrepare, errors.New("plain"))
		require.NotNil(t, result)
		assert.Equal(t, yassembly.ErrRuntimeSurfaceUnavailable, result.Code)
		assert.Equal(t, "prepare", result.Stage)
	})
}

func TestDefaultErrorCode(t *testing.T) {
	tests := []struct {
		stage Stage
		code  yassembly.ErrorCode
	}{
		{StagePrepare, yassembly.ErrRuntimeSurfaceUnavailable},
		{StageCompose, yassembly.ErrComposeFailed},
		{StageInstall, yassembly.ErrInstallValidationFailed},
		{StageReload, yassembly.ErrRuntimeReconcileFailed},
	}
	for _, tt := range tests {
		t.Run(string(tt.stage), func(t *testing.T) {
			assert.Equal(t, tt.code, DefaultErrorCode(tt.stage))
		})
	}
}

func TestCloneError(t *testing.T) {
	t.Run("nil error returns nil", func(t *testing.T) {
		assert.Nil(t, CloneError(nil))
	})

	t.Run("clones with independent copy", func(t *testing.T) {
		original := yassembly.NewError(
			yassembly.ErrComposeFailed,
			"compose",
			"msg",
			nil,
			map[string]string{"key": "val"},
		)
		cloned := CloneError(original)
		require.NotNil(t, cloned)
		assert.Equal(t, original.Code, cloned.Code)
		assert.Equal(t, original.Stage, cloned.Stage)
		assert.Equal(t, original.Message, cloned.Message)
		cloned.Context["key"] = "modified"
		assert.Equal(t, "val", original.Context["key"])
	})
}

func TestWrapStageError(t *testing.T) {
	t.Run("nil error returns nil", func(t *testing.T) {
		assert.Nil(t, WrapStageError("prepare", nil))
	})

	t.Run("assembly.Error passthrough with empty stage filled", func(t *testing.T) {
		original := yassembly.NewError(yassembly.ErrComposeFailed, "", "msg", nil, nil)
		wrapped := WrapStageError("compose", original)
		require.NotNil(t, wrapped)
		var assemblyErr *yassembly.Error
		require.ErrorAs(t, wrapped, &assemblyErr)
		assert.Equal(t, "compose", assemblyErr.Stage)
	})

	t.Run("assembly.Error with existing stage kept", func(t *testing.T) {
		original := yassembly.NewError(yassembly.ErrComposeFailed, "existing", "msg", nil, nil)
		wrapped := WrapStageError("compose", original)
		var assemblyErr *yassembly.Error
		require.ErrorAs(t, wrapped, &assemblyErr)
		assert.Equal(t, "existing", assemblyErr.Stage)
	})

	t.Run("plain error wrap", func(t *testing.T) {
		wrapped := WrapStageError("install", errors.New("plain"))
		require.NotNil(t, wrapped)
		var assemblyErr *yassembly.Error
		require.ErrorAs(t, wrapped, &assemblyErr)
		assert.Equal(t, yassembly.ErrInstallValidationFailed, assemblyErr.Code)
		assert.Equal(t, "install", assemblyErr.Stage)
	})
}

func TestErrorStateRecord(t *testing.T) {
	t.Run("records error at stage", func(t *testing.T) {
		state := &ErrorState{}
		err := yassembly.NewError(yassembly.ErrComposeFailed, "compose", "test", nil, nil)
		state.Record(StageCompose, err)
		assert.Equal(t, err, state.Compose.Err)
	})

	t.Run("nil error clears stage", func(t *testing.T) {
		state := &ErrorState{}
		err := yassembly.NewError(yassembly.ErrComposeFailed, "compose", "test", nil, nil)
		state.Record(StageCompose, err)
		state.Record(StageCompose, nil)
		assert.Nil(t, state.Compose.Err)
	})

	t.Run("updates latest", func(t *testing.T) {
		state := &ErrorState{}
		err := yassembly.NewError(yassembly.ErrComposeFailed, "compose", "test", nil, nil)
		state.Record(StageCompose, err)
		assert.Equal(t, err, state.Latest)
		assert.Equal(t, StageCompose, state.LatestStage)
	})
}

func TestErrorStateClear(t *testing.T) {
	t.Run("clears error at stage", func(t *testing.T) {
		state := &ErrorState{}
		err := yassembly.NewError(yassembly.ErrComposeFailed, "compose", "test", nil, nil)
		state.Record(StageCompose, err)
		state.Clear(StageCompose)
		assert.Nil(t, state.Compose.Err)
		assert.Nil(t, state.Latest)
	})
}

func TestErrorStateSnapshot(t *testing.T) {
	t.Run("empty snapshot", func(t *testing.T) {
		state := &ErrorState{}
		snap := state.Snapshot()
		assert.Nil(t, snap.LastError)
		assert.Nil(t, snap.Errors.Prepare)
	})

	t.Run("after record", func(t *testing.T) {
		state := &ErrorState{}
		err := yassembly.NewError(yassembly.ErrComposeFailed, "compose", "test", nil, nil)
		state.Record(StageCompose, err)
		snap := state.Snapshot()
		require.NotNil(t, snap.Errors.Compose)
		assert.Equal(t, yassembly.ErrComposeFailed, snap.Errors.Compose.Code)
	})

	t.Run("after clear", func(t *testing.T) {
		state := &ErrorState{}
		err := yassembly.NewError(yassembly.ErrComposeFailed, "compose", "test", nil, nil)
		state.Record(StageCompose, err)
		state.Clear(StageCompose)
		snap := state.Snapshot()
		assert.Nil(t, snap.Errors.Compose)
		assert.Nil(t, snap.LastError)
	})
}

func TestErrorStateRecalculateLatest(t *testing.T) {
	t.Run("multiple stages latest reflects most recent", func(t *testing.T) {
		state := &ErrorState{}
		err1 := yassembly.NewError(
			yassembly.ErrRuntimeSurfaceUnavailable,
			"prepare",
			"err1",
			nil,
			nil,
		)
		err2 := yassembly.NewError(yassembly.ErrComposeFailed, "compose", "err2", nil, nil)
		state.Record(StagePrepare, err1)
		state.Record(StageCompose, err2)
		assert.Equal(t, err2, state.Latest)
		assert.Equal(t, StageCompose, state.LatestStage)
	})
}
