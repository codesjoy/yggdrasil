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

package module

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/config"
)

// ---------------------------------------------------------------------------
// MarkCapabilityBindingsChanged
// ---------------------------------------------------------------------------

func TestMarkCapabilityBindingsChanged_ForceReloadAll(t *testing.T) {
	m := &testModule{name: "a", path: "mod.a"}
	h := NewHub()
	require.NoError(t, h.Use(m))
	require.NoError(t, h.Seal())
	require.NoError(t, h.Init(context.Background(), config.NewSnapshot(map[string]any{
		"mod": map[string]any{"a": map[string]any{"v": 1}},
	})))

	// Mark capability bindings changed -> reloadAll = true
	h.MarkCapabilityBindingsChanged()

	// Reload with same config should still trigger reload because reloadAll is set
	require.NoError(t, h.Reload(context.Background(), config.NewSnapshot(map[string]any{
		"mod": map[string]any{"a": map[string]any{"v": 1}},
	})))

	state := h.ReloadState()
	require.Equal(t, ReloadPhaseIdle, state.Phase)
}

// ---------------------------------------------------------------------------
// MarkRestartRequired
// ---------------------------------------------------------------------------

func TestMarkRestartRequired_WithModuleName(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(&testModule{name: "a"}))
	require.NoError(t, h.Seal())

	h.MarkRestartRequired("a")
	state := h.ReloadState()
	require.True(t, state.RestartRequired)

	diag := h.Modules()
	require.True(t, diag[0].RestartRequired)
}

func TestMarkRestartRequired_EmptyModuleName(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(&testModule{name: "a"}))
	require.NoError(t, h.Seal())

	h.MarkRestartRequired("")
	state := h.ReloadState()
	require.True(t, state.RestartRequired)

	// Empty name should not write to restartFlag for any specific module
	diag := h.Modules()
	require.False(t, diag[0].RestartRequired)
}

// ---------------------------------------------------------------------------
// Init / Start / Stop / Reload not sealed
// ---------------------------------------------------------------------------

func TestInit_NotSealed(t *testing.T) {
	h := NewHub()
	err := h.Init(context.Background(), config.NewSnapshot(map[string]any{}))
	require.Error(t, err)
	require.Equal(t, errHubNotSealed, err)
}

func TestStart_NotSealed(t *testing.T) {
	h := NewHub()
	err := h.Start(context.Background())
	require.Error(t, err)
	require.Equal(t, errHubNotSealed, err)
}

func TestStop_NotSealed(t *testing.T) {
	h := NewHub()
	err := h.Stop(context.Background())
	require.Error(t, err)
	require.Equal(t, errHubNotSealed, err)
}

func TestReload_NotSealed(t *testing.T) {
	h := NewHub()
	err := h.Reload(context.Background(), config.NewSnapshot(map[string]any{}))
	require.Error(t, err)
	require.Equal(t, errHubNotSealed, err)
}

// ---------------------------------------------------------------------------
// Init error path
// ---------------------------------------------------------------------------

type initFailModule struct {
	name string
	path string
}

func (m *initFailModule) Name() string       { return m.name }
func (m *initFailModule) ConfigPath() string { return m.path }
func (m *initFailModule) Init(context.Context, config.View) error {
	return errors.New("init explosion")
}

func TestInit_ModuleInitError(t *testing.T) {
	m := &initFailModule{name: "a", path: "mod.a"}
	h := NewHub()
	require.NoError(t, h.Use(m))
	require.NoError(t, h.Seal())
	err := h.Init(context.Background(), config.NewSnapshot(map[string]any{}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "init failed")
}

// ---------------------------------------------------------------------------
// Stop multiple errors
// ---------------------------------------------------------------------------

type stopErrModule struct {
	name string
	err  error
}

func (m *stopErrModule) Name() string                            { return m.name }
func (m *stopErrModule) Stop(context.Context) error              { return m.err }
func (m *stopErrModule) Init(context.Context, config.View) error { return nil }

func TestStop_MultipleStopErrors(t *testing.T) {
	a := &stopErrModule{name: "a", err: errors.New("stop-a")}
	b := &stopErrModule{name: "b", err: errors.New("stop-b")}
	h := NewHub()
	require.NoError(t, h.Use(a, b))
	require.NoError(t, h.Seal())
	require.NoError(t, h.Init(context.Background(), config.NewSnapshot(map[string]any{})))

	err := h.Stop(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "stop-a")
	require.Contains(t, err.Error(), "stop-b")
}

// ---------------------------------------------------------------------------
// Reload no-op (no affected modules)
// ---------------------------------------------------------------------------

func TestReload_NoAffectedModules(t *testing.T) {
	m := &testModule{name: "a", path: "mod.a"}
	h := NewHub()
	require.NoError(t, h.Use(m))
	require.NoError(t, h.Seal())

	snap := config.NewSnapshot(map[string]any{
		"mod": map[string]any{"a": map[string]any{"v": 1}},
	})
	require.NoError(t, h.Init(context.Background(), snap))

	// Reload with same snapshot -> no affected modules
	err := h.Reload(context.Background(), snap)
	require.NoError(t, err)
	state := h.ReloadState()
	require.Equal(t, ReloadPhaseIdle, state.Phase)
}

// ---------------------------------------------------------------------------
// Reload: prepare failure with rollback failure -> Degraded
// ---------------------------------------------------------------------------

func TestReload_PrepareFailureWithRollbackFailure(t *testing.T) {
	// a prepares ok but rollback fails; b fails to prepare
	a := &testModule{name: "a", path: "mod.a", rollbackErr: errors.New("rollback bad")}
	b := &testModule{name: "b", path: "mod.b", prepareErr: errors.New("prepare bad")}

	h := NewHub()
	require.NoError(t, h.Use(a, b))
	require.NoError(t, h.Seal())

	oldSnap := config.NewSnapshot(map[string]any{
		"mod": map[string]any{
			"a": map[string]any{"v": 1},
			"b": map[string]any{"v": 1},
		},
	})
	require.NoError(t, h.Init(context.Background(), oldSnap))

	newSnap := config.NewSnapshot(map[string]any{
		"mod": map[string]any{
			"a": map[string]any{"v": 2},
			"b": map[string]any{"v": 2},
		},
	})
	err := h.Reload(context.Background(), newSnap)
	require.Error(t, err)

	state := h.ReloadState()
	// When prepare fails AND rollback also fails, we enter Degraded
	require.Equal(t, ReloadPhaseDegraded, state.Phase)
	require.Equal(t, ReloadFailedStageRollback, state.FailedStage)
}

// ---------------------------------------------------------------------------
// Reload: commit failure with rollback success
// ---------------------------------------------------------------------------

func TestReload_CommitFailureWithRollbackSuccess(t *testing.T) {
	a := &testModule{name: "a", path: "mod.a", commitErr: errors.New("commit bad")}
	b := &testModule{name: "b", path: "mod.b"}

	h := NewHub()
	require.NoError(t, h.Use(a, b))
	require.NoError(t, h.Seal())

	oldSnap := config.NewSnapshot(map[string]any{
		"mod": map[string]any{
			"a": map[string]any{"v": 1},
			"b": map[string]any{"v": 1},
		},
	})
	require.NoError(t, h.Init(context.Background(), oldSnap))

	newSnap := config.NewSnapshot(map[string]any{
		"mod": map[string]any{
			"a": map[string]any{"v": 2},
			"b": map[string]any{"v": 2},
		},
	})
	err := h.Reload(context.Background(), newSnap)
	require.Error(t, err)
	require.Contains(t, err.Error(), "commit reload failed")

	state := h.ReloadState()
	require.Equal(t, ReloadPhaseRollback, state.Phase)
	require.Equal(t, ReloadFailedStageCommit, state.FailedStage)
}

// ---------------------------------------------------------------------------
// configChanged / splitDotPath edge cases
// ---------------------------------------------------------------------------

func TestConfigChanged_NoConfigPath(t *testing.T) {
	m := &testModule{name: "a"} // no path
	oldSnap := config.NewSnapshot(map[string]any{"a": 1})
	newSnap := config.NewSnapshot(map[string]any{"a": 2})
	require.False(t, configChanged(m, oldSnap, newSnap))
}

func TestSplitDotPath_EmptySegments(t *testing.T) {
	result := splitDotPath("a..b...c")
	require.Equal(t, []string{"a", "b", "c"}, result)
}

// ---------------------------------------------------------------------------
// ReloadReporter in Diagnostics
// ---------------------------------------------------------------------------

type reloadReporterModule struct {
	name  string
	state ReloadState
}

func (m *reloadReporterModule) Name() string             { return m.name }
func (m *reloadReporterModule) ReloadState() ReloadState { return m.state }

func TestModules_WithReloadReporter(t *testing.T) {
	m := &reloadReporterModule{
		name:  "a",
		state: ReloadState{Phase: ReloadPhaseCommitting},
	}
	h := NewHub()
	require.NoError(t, h.Use(m))
	require.NoError(t, h.Seal())

	diag := h.Modules()
	require.Len(t, diag, 1)
	require.Equal(t, "committing", diag[0].ReloadPhase)
}

// ---------------------------------------------------------------------------
// Reload successful commit path
// ---------------------------------------------------------------------------

func TestReload_SuccessPath(t *testing.T) {
	m := &testModule{name: "a", path: "mod.a"}
	h := NewHub()
	require.NoError(t, h.Use(m))
	require.NoError(t, h.Seal())

	oldSnap := config.NewSnapshot(map[string]any{
		"mod": map[string]any{"a": map[string]any{"v": 1}},
	})
	require.NoError(t, h.Init(context.Background(), oldSnap))

	newSnap := config.NewSnapshot(map[string]any{
		"mod": map[string]any{"a": map[string]any{"v": 2}},
	})
	require.NoError(t, h.Reload(context.Background(), newSnap))
	require.Equal(t, int32(1), m.commits.Load())

	state := h.ReloadState()
	require.Equal(t, ReloadPhaseIdle, state.Phase)
}

// ---------------------------------------------------------------------------
// SetCapabilityBindings deep copy
// ---------------------------------------------------------------------------

func TestSetCapabilityBindings_DeepCopy(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(&testModule{name: "a"}))
	require.NoError(t, h.Seal())

	bindings := map[string][]string{
		"test.cap": {"p1", "p2"},
	}
	h.SetCapabilityBindings(bindings)

	// Mutate input after SetCapabilityBindings
	bindings["test.cap"][0] = "mutated"
	bindings["test.cap"] = append(bindings["test.cap"], "p3")

	// Hub's internal copy should be unaffected
	diag := h.Diagnostics()
	require.Len(t, diag.Bindings, 1)
	// The requested bindings should still be ["p1", "p2"]
	require.Equal(t, []string{"p1", "p2"}, diag.Bindings[0].Requested)
}

// ---------------------------------------------------------------------------
// Seal idempotency
// ---------------------------------------------------------------------------

func TestSeal_Idempotent(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(&testModule{name: "a"}))
	require.NoError(t, h.Seal())
	require.NoError(t, h.Seal())
}

// ---------------------------------------------------------------------------
// MarkRestartRequired seen in Diagnostics
// ---------------------------------------------------------------------------

func TestMarkRestartRequired_SeenInDiagnostics(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(
		&testModule{name: "a"},
		&testModule{name: "b"},
	))
	require.NoError(t, h.Seal())

	h.MarkRestartRequired("b")

	mods := h.Modules()
	for _, m := range mods {
		if m.Name == "b" {
			require.True(t, m.RestartRequired)
		} else {
			require.False(t, m.RestartRequired)
		}
	}
}

// ---------------------------------------------------------------------------
// Diagnostics with requested bindings that have no capability
// ---------------------------------------------------------------------------

func TestDiagnostics_BindingsWithRequestedButNoCapability(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(&testModule{name: "a"}))
	require.NoError(t, h.Seal())

	h.SetCapabilityBindings(map[string][]string{
		"nonexistent.cap": {"p1"},
	})

	diag := h.Diagnostics()
	require.Len(t, diag.Bindings, 1)
	require.Equal(t, "nonexistent.cap", diag.Bindings[0].Spec)
	require.Equal(t, []string{"p1"}, diag.Bindings[0].Requested)
	require.Empty(t, diag.Bindings[0].Resolved)
	require.Equal(t, []string{"p1"}, diag.Bindings[0].Missing)
}

// ---------------------------------------------------------------------------
// Reload with MarkCapabilityBindingsChanged forces reload of all modules
// ---------------------------------------------------------------------------

func TestReload_MarkCapabilityBindingsChanged_ForcesReload(t *testing.T) {
	m := &testModule{name: "a", path: "mod.a"}
	h := NewHub()
	require.NoError(t, h.Use(m))
	require.NoError(t, h.Seal())

	snap := config.NewSnapshot(map[string]any{
		"mod": map[string]any{"a": map[string]any{"v": 1}},
	})
	require.NoError(t, h.Init(context.Background(), snap))

	// Mark capability bindings changed before reload
	h.MarkCapabilityBindingsChanged()

	// Reload with same snapshot; it should still reload because reloadAll is set
	require.NoError(t, h.Reload(context.Background(), snap))
	require.Equal(t, int32(1), m.commits.Load())
}

// Silence unused import
var _ fmt.Stringer = namedStringer("")
