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

package runtime

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
)

// --- CapabilityBindingsEqual ---

func TestCapabilityBindingsEqual(t *testing.T) {
	t.Run("equal maps", func(t *testing.T) {
		left := map[string][]string{"a": {"x", "y"}, "b": {"z"}}
		right := map[string][]string{"a": {"x", "y"}, "b": {"z"}}
		assert.True(t, CapabilityBindingsEqual(left, right))
	})

	t.Run("different lengths", func(t *testing.T) {
		left := map[string][]string{"a": {"x"}}
		right := map[string][]string{"a": {"x"}, "b": {"y"}}
		assert.False(t, CapabilityBindingsEqual(left, right))
	})

	t.Run("missing key", func(t *testing.T) {
		left := map[string][]string{"a": {"x"}}
		right := map[string][]string{"b": {"x"}}
		assert.False(t, CapabilityBindingsEqual(left, right))
	})

	t.Run("different values", func(t *testing.T) {
		left := map[string][]string{"a": {"x"}}
		right := map[string][]string{"a": {"y"}}
		assert.False(t, CapabilityBindingsEqual(left, right))
	})

	t.Run("both nil maps", func(t *testing.T) {
		assert.True(t, CapabilityBindingsEqual(nil, nil))
	})

	t.Run("empty maps equal", func(t *testing.T) {
		assert.True(t, CapabilityBindingsEqual(map[string][]string{}, map[string][]string{}))
	})
}

// --- IntersectsPaths ---

func TestIntersectsPaths(t *testing.T) {
	t.Run("exact match", func(t *testing.T) {
		assert.True(t, IntersectsPaths([]string{"a.b"}, []string{"a.b"}))
	})

	t.Run("prefix match", func(t *testing.T) {
		assert.True(t, IntersectsPaths([]string{"a.b.c"}, []string{"a.b"}))
	})

	t.Run("no intersection", func(t *testing.T) {
		assert.False(t, IntersectsPaths([]string{"a.b"}, []string{"c.d"}))
	})

	t.Run("empty left", func(t *testing.T) {
		assert.False(t, IntersectsPaths(nil, []string{"a.b"}))
	})

	t.Run("empty right", func(t *testing.T) {
		assert.False(t, IntersectsPaths([]string{"a.b"}, nil))
	})

	t.Run("both empty", func(t *testing.T) {
		assert.False(t, IntersectsPaths(nil, nil))
	})
}

// --- pathPrefixMatch ---

func TestPathPrefixMatch(t *testing.T) {
	t.Run("exact equal", func(t *testing.T) {
		assert.True(t, pathPrefixMatch("a.b.c", "a.b.c"))
	})

	t.Run("left prefix of right", func(t *testing.T) {
		assert.True(t, pathPrefixMatch("a.b", "a.b.c"))
	})

	t.Run("right prefix of left", func(t *testing.T) {
		assert.True(t, pathPrefixMatch("a.b.c", "a.b"))
	})

	t.Run("no match", func(t *testing.T) {
		assert.False(t, pathPrefixMatch("a.b", "c.d"))
	})
}

// --- dedupStrings ---

func TestDedupStrings(t *testing.T) {
	t.Run("empty returns nil", func(t *testing.T) {
		assert.Nil(t, dedupStrings(nil))
		assert.Nil(t, dedupStrings([]string{}))
	})

	t.Run("removes duplicates", func(t *testing.T) {
		result := dedupStrings([]string{"a", "b", "a", "c", "b"})
		assert.Equal(t, []string{"a", "b", "c"}, result)
	})

	t.Run("preserves order", func(t *testing.T) {
		result := dedupStrings([]string{"c", "a", "b"})
		assert.Equal(t, []string{"c", "a", "b"}, result)
	})

	t.Run("removes empty strings", func(t *testing.T) {
		result := dedupStrings([]string{"a", "", "b", ""})
		assert.Equal(t, []string{"a", "b"}, result)
	})
}

// --- sortedKeys ---

func TestSortedKeys(t *testing.T) {
	t.Run("empty map", func(t *testing.T) {
		assert.Equal(t, []string{}, sortedKeys(map[string]struct{}{}))
	})

	t.Run("single key", func(t *testing.T) {
		assert.Equal(t, []string{"only"}, sortedKeys(map[string]struct{}{"only": {}}))
	})

	t.Run("multiple keys sorted", func(t *testing.T) {
		result := sortedKeys(map[string]struct{}{"c": {}, "a": {}, "b": {}})
		assert.Equal(t, []string{"a", "b", "c"}, result)
	})
}

// --- valuesEqual ---

func TestValuesEqual(t *testing.T) {
	t.Run("equal values", func(t *testing.T) {
		assert.True(t, valuesEqual("hello", "hello"))
	})

	t.Run("different values", func(t *testing.T) {
		assert.False(t, valuesEqual("hello", "world"))
	})

	t.Run("nil vs nil", func(t *testing.T) {
		assert.True(t, valuesEqual(nil, nil))
	})

	t.Run("nil vs value", func(t *testing.T) {
		assert.False(t, valuesEqual(nil, "value"))
	})

	t.Run("equal maps", func(t *testing.T) {
		assert.True(t, valuesEqual(map[string]any{"k": "v"}, map[string]any{"k": "v"}))
	})
}

// --- rootAsAny ---

func TestRootAsAny(t *testing.T) {
	t.Run("valid root", func(t *testing.T) {
		root := settings.Root{
			Yggdrasil: settings.Framework{
				Mode: "test",
			},
		}
		result, err := rootAsAny(root)
		require.NoError(t, err)
		topMap, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Contains(t, topMap, "Yggdrasil")
	})

	t.Run("empty root", func(t *testing.T) {
		result, err := rootAsAny(settings.Root{})
		require.NoError(t, err)
		_, ok := result.(map[string]any)
		require.True(t, ok)
	})
}

// --- lookupRootSection ---

func TestLookupRootSection(t *testing.T) {
	t.Run("has Yggdrasil key", func(t *testing.T) {
		value := map[string]any{"Yggdrasil": map[string]any{"app": map[string]any{"name": "test"}}}
		result := lookupRootSection(value)
		assert.NotNil(t, result)
	})

	t.Run("missing key returns nil", func(t *testing.T) {
		value := map[string]any{"other": "value"}
		result := lookupRootSection(value)
		assert.Nil(t, result)
	})

	t.Run("non-map value returns nil", func(t *testing.T) {
		result := lookupRootSection("string")
		assert.Nil(t, result)
	})
}

// --- collectChangedPaths ---

func TestCollectChangedPaths(t *testing.T) {
	t.Run("flat diff", func(t *testing.T) {
		changes := map[string]struct{}{}
		collectChangedPaths("root.key", "old", "new", changes)
		assert.Contains(t, changes, "root.key")
	})

	t.Run("no changes for equal values", func(t *testing.T) {
		changes := map[string]struct{}{}
		collectChangedPaths("root.key", "same", "same", changes)
		_, exists := changes["root.key"]
		assert.False(t, exists)
	})

	t.Run("nested map diff", func(t *testing.T) {
		old := map[string]any{"a": "1", "b": "2"}
		newVal := map[string]any{"a": "1", "b": "3"}
		changes := map[string]struct{}{}
		collectChangedPaths("root", old, newVal, changes)
		assert.Contains(t, changes, "root.b")
		assert.NotContains(t, changes, "root.a")
	})

	t.Run("empty maps no changes", func(t *testing.T) {
		changes := map[string]struct{}{}
		collectChangedPaths("root", map[string]any{}, map[string]any{}, changes)
		assert.Empty(t, changes)
	})
}

// --- ChangedConfigPaths ---

func TestChangedConfigPaths(t *testing.T) {
	t.Run("nil plans return nil", func(t *testing.T) {
		assert.Nil(t, ChangedConfigPaths(nil, nil))
		assert.Nil(t, ChangedConfigPaths(&yassembly.Result{}, nil))
		assert.Nil(t, ChangedConfigPaths(nil, &yassembly.Result{}))
	})

	t.Run("identical plans return empty", func(t *testing.T) {
		resolved := settings.Resolved{}
		plan := &yassembly.Result{
			EffectiveResolved: resolved,
		}
		// Both have empty root so no changes
		paths := ChangedConfigPaths(plan, plan)
		assert.Empty(t, paths)
	})
}

// --- ReloadRequiresRestart ---

func TestReloadRequiresRestart(t *testing.T) {
	t.Run("nil diff no changes returns false", func(t *testing.T) {
		assert.False(t, ReloadRequiresRestart(nil, nil, nil, false))
	})

	t.Run("no changes returns false", func(t *testing.T) {
		assert.False(t, ReloadRequiresRestart(&yassembly.SpecDiff{}, nil, nil, false))
	})

	t.Run("mode domain changed returns true", func(t *testing.T) {
		diff := &yassembly.SpecDiff{
			HasChanges:      true,
			AffectedDomains: []string{"mode"},
		}
		assert.True(t, ReloadRequiresRestart(diff, nil, nil, false))
	})

	t.Run("modules domain changed returns true", func(t *testing.T) {
		diff := &yassembly.SpecDiff{
			HasChanges:      true,
			AffectedDomains: []string{"modules"},
		}
		assert.True(t, ReloadRequiresRestart(diff, nil, nil, false))
	})

	t.Run("business installed with intersecting paths returns true", func(t *testing.T) {
		assert.True(t, ReloadRequiresRestart(nil, []string{"a.b"}, []string{"a.b"}, true))
	})

	t.Run("business installed no intersection returns false", func(t *testing.T) {
		assert.False(t, ReloadRequiresRestart(nil, []string{"a.b"}, []string{"c.d"}, true))
	})

	t.Run("business not installed returns false for paths", func(t *testing.T) {
		assert.False(t, ReloadRequiresRestart(nil, []string{"a.b"}, []string{"a.b"}, false))
	})

	t.Run("unrelated domain returns false", func(t *testing.T) {
		diff := &yassembly.SpecDiff{
			HasChanges:      true,
			AffectedDomains: []string{"unknown"},
		}
		assert.False(t, ReloadRequiresRestart(diff, nil, nil, false))
	})

	t.Run("defaults domain changed returns true", func(t *testing.T) {
		diff := &yassembly.SpecDiff{
			HasChanges:      true,
			AffectedDomains: []string{"defaults"},
		}
		assert.True(t, ReloadRequiresRestart(diff, nil, nil, false))
	})

	t.Run("chains domain changed returns true", func(t *testing.T) {
		diff := &yassembly.SpecDiff{
			HasChanges:      true,
			AffectedDomains: []string{"chains"},
		}
		assert.True(t, ReloadRequiresRestart(diff, nil, nil, false))
	})

	t.Run("overrides domain changed returns true", func(t *testing.T) {
		diff := &yassembly.SpecDiff{
			HasChanges:      true,
			AffectedDomains: []string{"overrides"},
		}
		assert.True(t, ReloadRequiresRestart(diff, nil, nil, false))
	})
}
