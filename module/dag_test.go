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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// detectCycle
// ---------------------------------------------------------------------------

func TestDetectCycle_SimpleCycle(t *testing.T) {
	// A -> B -> C -> A
	modA := &testModule{name: "a", deps: []string{"c"}}
	modB := &testModule{name: "b", deps: []string{"a"}}
	modC := &testModule{name: "c", deps: []string{"b"}}
	modules := []Module{modA, modB, modC}
	index := map[string]Module{"a": modA, "b": modB, "c": modC}

	_, depErrs, err := buildDAG(modules, index)
	require.Error(t, err)
	require.True(t, len(depErrs) > 0)
	require.Contains(t, depErrs[0], "cycle detected")
}

func TestDetectCycle_NoCycle(t *testing.T) {
	modA := &testModule{name: "a"}
	modB := &testModule{name: "b", deps: []string{"a"}}
	modC := &testModule{name: "c", deps: []string{"b"}}
	modules := []Module{modA, modB, modC}
	index := map[string]Module{"a": modA, "b": modB, "c": modC}

	result, depErrs, err := buildDAG(modules, index)
	require.NoError(t, err)
	require.Empty(t, depErrs)
	require.Len(t, result.order, 3)
	require.Equal(t, "a", result.order[0].Name())
	require.Equal(t, "b", result.order[1].Name())
	require.Equal(t, "c", result.order[2].Name())
}

func TestDetectCycle_SelfLoop(t *testing.T) {
	modA := &testModule{name: "a", deps: []string{"a"}}
	modules := []Module{modA}
	index := map[string]Module{"a": modA}

	_, depErrs, err := buildDAG(modules, index)
	require.Error(t, err)
	require.True(t, len(depErrs) > 0)
	// The cycle should include a -> a
	found := false
	for _, e := range depErrs {
		if strings.Contains(e, "a -> a") {
			found = true
			break
		}
	}
	require.True(t, found, "expected cycle a -> a in dependency errors: %v", depErrs)
}

func TestDetectCycle_TwoNodes(t *testing.T) {
	// A <-> B
	modA := &testModule{name: "a", deps: []string{"b"}}
	modB := &testModule{name: "b", deps: []string{"a"}}
	modules := []Module{modA, modB}
	index := map[string]Module{"a": modA, "b": modB}

	_, depErrs, err := buildDAG(modules, index)
	require.Error(t, err)
	require.True(t, len(depErrs) > 0)
	require.Contains(t, depErrs[0], "cycle detected")
}

func TestDetectCycle_DisconnectedWithOneCycle(t *testing.T) {
	// Component 1: X -> Y (no cycle)
	// Component 2: A -> B -> A (cycle)
	modX := &testModule{name: "x"}
	modY := &testModule{name: "y", deps: []string{"x"}}
	modA := &testModule{name: "a", deps: []string{"b"}}
	modB := &testModule{name: "b", deps: []string{"a"}}
	modules := []Module{modX, modY, modA, modB}
	index := map[string]Module{"x": modX, "y": modY, "a": modA, "b": modB}

	_, depErrs, err := buildDAG(modules, index)
	require.Error(t, err)
	require.True(t, len(depErrs) > 0)
	require.Contains(t, depErrs[0], "cycle detected")
}

// ---------------------------------------------------------------------------
// compareModules
// ---------------------------------------------------------------------------

func TestCompareModules_SameOrderDifferentName(t *testing.T) {
	a := &testModule{name: "beta", order: 0}
	b := &testModule{name: "alpha", order: 0}
	require.Equal(t, 1, compareModules(a, b))  // beta > alpha
	require.Equal(t, -1, compareModules(b, a)) // alpha < beta
}

func TestCompareModules_DifferentOrder(t *testing.T) {
	a := &testModule{name: "a", order: 1}
	b := &testModule{name: "b", order: 2}
	require.Equal(t, -1, compareModules(a, b)) // order 1 < order 2
	require.Equal(t, 1, compareModules(b, a))  // order 2 > order 1
}

func TestCompareModules_SameOrderSameName(t *testing.T) {
	a := &testModule{name: "same", order: 0}
	b := &testModule{name: "same", order: 0}
	require.Equal(t, 0, compareModules(a, b))
}

// ---------------------------------------------------------------------------
// dependsOn
// ---------------------------------------------------------------------------

func TestDependsOn_DeduplicatesAndSorts(t *testing.T) {
	m := &testModule{name: "a", deps: []string{"c", "a", "b", "c", "b"}}
	result := dependsOn(m)
	require.Equal(t, []string{"a", "b", "c"}, result)
}

func TestDependsOn_FiltersEmptyStrings(t *testing.T) {
	m := &testModule{name: "a", deps: []string{"", "b", "", "a"}}
	result := dependsOn(m)
	require.Equal(t, []string{"a", "b"}, result)
}

func TestDependsOn_NonDependent(t *testing.T) {
	// testModule implements Dependent, so dependsOn returns empty slice even with no deps
	m := &testModule{name: "a"} // no deps
	result := dependsOn(m)
	require.Empty(t, result)
}

func TestDependsOn_TrueNonDependent(t *testing.T) {
	// A module that does NOT implement Dependent at all
	m := simpleNamedModule{name: "a"}
	result := dependsOn(m)
	require.Nil(t, result)
}

// simpleNamedModule only implements Module (Name), not Dependent.
type simpleNamedModule struct{ name string }

func (m simpleNamedModule) Name() string { return m.name }

// ---------------------------------------------------------------------------
// buildDAG integration
// ---------------------------------------------------------------------------

func TestBuildDAG_CycleTriggersDetectCycle(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(
		&testModule{name: "a", deps: []string{"b"}},
		&testModule{name: "b", deps: []string{"c"}},
		&testModule{name: "c", deps: []string{"a"}},
	))
	err := h.Seal()
	require.Error(t, err)
	require.Contains(t, err.Error(), "cycle detected")
}

func TestBuildDAG_MissingDependency(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(&testModule{name: "a", deps: []string{"nonexistent"}}))
	err := h.Seal()
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing module")
}

func TestBuildDAG_LayerAssignment(t *testing.T) {
	a := &testModule{name: "a"}
	b := &testModule{name: "b", deps: []string{"a"}}
	c := &testModule{name: "c", deps: []string{"b"}}
	d := &testModule{name: "d", deps: []string{"a"}}

	modules := []Module{a, b, c, d}
	index := map[string]Module{"a": a, "b": b, "c": c, "d": d}
	result, depErrs, err := buildDAG(modules, index)
	require.NoError(t, err)
	require.Empty(t, depErrs)

	require.Equal(t, 0, result.layers["a"])
	require.Equal(t, 1, result.layers["b"])
	require.Equal(t, 1, result.layers["d"])
	require.Equal(t, 2, result.layers["c"])
}

func TestBuildDAG_OrderTieBreaker(t *testing.T) {
	m1 := &testModule{name: "z", order: 1}
	m2 := &testModule{name: "a", order: 2}
	m3 := &testModule{name: "m", order: 1}

	modules := []Module{m1, m2, m3}
	index := map[string]Module{"z": m1, "a": m2, "m": m3}
	result, _, err := buildDAG(modules, index)
	require.NoError(t, err)

	// Order 1 first (m, z alphabetically), then order 2 (a)
	names := make([]string, len(result.order))
	for i, m := range result.order {
		names[i] = m.Name()
	}
	require.Equal(t, []string{"m", "z", "a"}, names)
}
