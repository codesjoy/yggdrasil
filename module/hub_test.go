package module

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/config"
)

type testModule struct {
	name        string
	deps        []string
	order       int
	path        string
	startErr    error
	prepareErr  error
	commitErr   error
	rollbackErr error

	started   atomic.Bool
	stopped   atomic.Int32
	commits   atomic.Int32
	rollbacks atomic.Int32
}

func (m *testModule) Name() string                            { return m.name }
func (m *testModule) DependsOn() []string                     { return m.deps }
func (m *testModule) InitOrder() int                          { return m.order }
func (m *testModule) ConfigPath() string                      { return m.path }
func (m *testModule) Init(context.Context, config.View) error { return nil }
func (m *testModule) Start(context.Context) error {
	if m.startErr != nil {
		return m.startErr
	}
	m.started.Store(true)
	return nil
}
func (m *testModule) Stop(context.Context) error {
	m.stopped.Add(1)
	return nil
}
func (m *testModule) PrepareReload(context.Context, config.View) (ReloadCommitter, error) {
	if m.prepareErr != nil {
		return nil, m.prepareErr
	}
	return reloadCommitter{m: m}, nil
}

type reloadCommitter struct {
	m *testModule
}

func (c reloadCommitter) Commit(context.Context) error {
	if c.m != nil {
		c.m.commits.Add(1)
		return c.m.commitErr
	}
	return nil
}
func (c reloadCommitter) Rollback(context.Context) error {
	if c.m != nil {
		c.m.rollbacks.Add(1)
		return c.m.rollbackErr
	}
	return nil
}

type capProvider struct {
	name string
}

func (m capProvider) Name() string { return m.name }
func (m capProvider) Capabilities() []Capability {
	return []Capability{{
		Spec: CapabilitySpec{
			Name:        "test.cap",
			Cardinality: ExactlyOne,
			Type:        reflect.TypeOf((*fmt.Stringer)(nil)).Elem(),
		},
		Value: namedStringer(m.name),
	}}
}

type namedStringer string

func (n namedStringer) String() string { return string(n) }

func TestHubSealAndStartCompensate(t *testing.T) {
	a := &testModule{name: "a"}
	b := &testModule{name: "b", deps: []string{"a"}}
	c := &testModule{name: "c", deps: []string{"b"}, startErr: errors.New("boom")}

	h := NewHub()
	require.NoError(t, h.Use(a, b, c))
	require.NoError(t, h.Seal())
	require.NoError(t, h.Init(context.Background(), config.NewSnapshot(map[string]any{})))
	err := h.Start(context.Background())
	require.Error(t, err)
	require.Equal(t, int32(1), b.stopped.Load())
	require.Equal(t, int32(1), a.stopped.Load())
}

func TestHubDependencyValidation(t *testing.T) {
	a := &testModule{name: "a", deps: []string{"missing"}}
	h := NewHub()
	require.NoError(t, h.Use(a))
	require.Error(t, h.Seal())
}

func TestCapabilityResolve(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(capProvider{name: "p1"}))
	require.NoError(t, h.Seal())
	spec := CapabilitySpec{
		Name:        "test.cap",
		Cardinality: ExactlyOne,
		Type:        reflect.TypeOf((*fmt.Stringer)(nil)).Elem(),
	}
	got, err := ResolveExactlyOne[fmt.Stringer](h, spec)
	require.NoError(t, err)
	require.Equal(t, "p1", got.String())
}

func TestReloadPrepareFailureSetsState(t *testing.T) {
	m := &testModule{name: "a", path: "mod.a", prepareErr: errors.New("bad config")}
	h := NewHub()
	require.NoError(t, h.Use(m))
	require.NoError(t, h.Seal())
	require.NoError(t, h.Init(context.Background(), config.NewSnapshot(map[string]any{
		"mod": map[string]any{"a": map[string]any{"v": 1}},
	})))
	err := h.Reload(context.Background(), config.NewSnapshot(map[string]any{
		"mod": map[string]any{"a": map[string]any{"v": 2}},
	}))
	require.Error(t, err)
	state := h.ReloadState()
	require.Equal(t, ReloadPhaseRollback, state.Phase)
	require.Equal(t, "a", state.FailedModule)
	require.Equal(t, ReloadFailedStagePrepare, state.FailedStage)
}

func TestReloadCommitFailureSetsFailedStageCommit(t *testing.T) {
	a := &testModule{name: "a", path: "mod.a", commitErr: errors.New("commit failed")}
	h := NewHub()
	require.NoError(t, h.Use(a))
	require.NoError(t, h.Seal())
	require.NoError(t, h.Init(context.Background(), config.NewSnapshot(map[string]any{
		"mod": map[string]any{"a": map[string]any{"v": 1}},
	})))
	err := h.Reload(context.Background(), config.NewSnapshot(map[string]any{
		"mod": map[string]any{"a": map[string]any{"v": 2}},
	}))
	require.Error(t, err)
	state := h.ReloadState()
	require.Equal(t, ReloadPhaseRollback, state.Phase)
	require.Equal(t, "a", state.FailedModule)
	require.Equal(t, ReloadFailedStageCommit, state.FailedStage)
	require.True(t, state.Diverged)
	require.True(t, state.RestartRequired)
}

func TestReloadRollbackFailureEntersDegradedAndBlocksNextReload(t *testing.T) {
	a := &testModule{name: "a", path: "mod.a", commitErr: errors.New("commit failed")}
	b := &testModule{name: "b", path: "mod.b", rollbackErr: errors.New("rollback failed")}
	h := NewHub()
	require.NoError(t, h.Use(a, b))
	require.NoError(t, h.Seal())
	require.NoError(t, h.Init(context.Background(), config.NewSnapshot(map[string]any{
		"mod": map[string]any{
			"a": map[string]any{"v": 1},
			"b": map[string]any{"v": 1},
		},
	})))

	err := h.Reload(context.Background(), config.NewSnapshot(map[string]any{
		"mod": map[string]any{
			"a": map[string]any{"v": 2},
			"b": map[string]any{"v": 2},
		},
	}))
	require.Error(t, err)
	state := h.ReloadState()
	require.Equal(t, ReloadPhaseDegraded, state.Phase)
	require.Equal(t, ReloadFailedStageRollback, state.FailedStage)
	require.True(t, state.Diverged)
	require.True(t, state.RestartRequired)

	err = h.Reload(context.Background(), config.NewSnapshot(map[string]any{
		"mod": map[string]any{"a": map[string]any{"v": 3}},
	}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "restart required")
}

type nonReloadableModule struct {
	name string
	path string
}

func (m nonReloadableModule) Name() string       { return m.name }
func (m nonReloadableModule) ConfigPath() string { return m.path }

func TestRestartRequiredStickyAcrossSuccessfulReload(t *testing.T) {
	a := nonReloadableModule{name: "a", path: "mod.a"}
	b := &testModule{name: "b", path: "mod.b"}
	h := NewHub()
	require.NoError(t, h.Use(a, b))
	require.NoError(t, h.Seal())
	require.NoError(t, h.Init(context.Background(), config.NewSnapshot(map[string]any{
		"mod": map[string]any{
			"a": map[string]any{"v": 1},
			"b": map[string]any{"v": 1},
		},
	})))
	require.NoError(t, h.Reload(context.Background(), config.NewSnapshot(map[string]any{
		"mod": map[string]any{
			"a": map[string]any{"v": 2},
			"b": map[string]any{"v": 1},
		},
	})))
	state := h.ReloadState()
	require.Equal(t, ReloadPhaseIdle, state.Phase)
	require.True(t, state.RestartRequired)
	require.NoError(t, h.Reload(context.Background(), config.NewSnapshot(map[string]any{
		"mod": map[string]any{
			"a": map[string]any{"v": 2},
			"b": map[string]any{"v": 2},
		},
	})))
	state = h.ReloadState()
	require.Equal(t, ReloadPhaseIdle, state.Phase)
	require.True(t, state.RestartRequired)
}

func TestDiagnosticsIncludesTopologyAndCapabilityData(t *testing.T) {
	a := capProvider{name: "cap-a"}
	h := NewHub()
	require.NoError(t, h.Use(a))
	require.NoError(t, h.Seal())
	diag := h.Diagnostics()
	require.NotEmpty(t, diag.Topology)
	require.NotEmpty(t, diag.LastStableTopology)
	require.NotEmpty(t, diag.Capabilities)
	require.Equal(t, "test.cap", diag.Capabilities[0].Spec)
	require.Equal(t, "exactly_one", diag.Capabilities[0].Cardinality)
}

func TestDiagnosticsIncludesRequestedCapabilityBindings(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(capProvider{name: "cap-a"}))
	require.NoError(t, h.Seal())
	h.SetCapabilityBindings(map[string][]string{
		"test.cap": []string{"cap-a", "missing"},
	})

	diag := h.Diagnostics()
	require.Len(t, diag.Bindings, 1)
	require.Equal(t, "test.cap", diag.Bindings[0].Spec)
	require.Equal(t, []string{"cap-a", "missing"}, diag.Bindings[0].Requested)
	require.Equal(t, []string{"cap-a"}, diag.Bindings[0].Resolved)
	require.Equal(t, []string{"missing"}, diag.Bindings[0].Missing)
}

type runtimeFactoryModule struct{ name string }

func (m runtimeFactoryModule) Name() string { return m.name }
func (runtimeFactoryModule) Scope() Scope   { return ScopeRuntimeFactory }

func TestHubRejectsRuntimeFactoryScope(t *testing.T) {
	h := NewHub()
	err := h.Use(runtimeFactoryModule{name: "runtime.factory"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "runtime_factory")
}

func TestResolveHelperRejectsWrongCardinality(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(capProvider{name: "cap-a"}))
	require.NoError(t, h.Seal())

	_, err := ResolveNamed[fmt.Stringer](h, CapabilitySpec{
		Name:        "test.cap",
		Cardinality: ExactlyOne,
		Type:        reflect.TypeOf((*fmt.Stringer)(nil)).Elem(),
	}, "cap-a")
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires named_one cardinality")
}
