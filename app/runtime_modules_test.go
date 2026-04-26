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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source/memory"
	"github.com/codesjoy/yggdrasil/v3/module"
)

// --- appendSortedCapabilities ---

func TestAppendSortedCapabilities(t *testing.T) {
	t.Run("empty providers", func(t *testing.T) {
		result := appendSortedCapabilities(
			nil,
			module.CapabilitySpec{Name: "test"},
			map[string]any{},
		)
		assert.Empty(t, result)
	})

	t.Run("single provider", func(t *testing.T) {
		providers := map[string]any{"a": "value-a"}
		result := appendSortedCapabilities(nil, module.CapabilitySpec{Name: "test"}, providers)
		assert.Len(t, result, 1)
		assert.Equal(t, "a", result[0].Name)
		assert.Equal(t, "value-a", result[0].Value)
	})

	t.Run("multiple providers sorted by name", func(t *testing.T) {
		providers := map[string]any{"c": "val-c", "a": "val-a", "b": "val-b"}
		result := appendSortedCapabilities(nil, module.CapabilitySpec{Name: "test"}, providers)
		assert.Len(t, result, 3)
		assert.Equal(t, "a", result[0].Name)
		assert.Equal(t, "b", result[1].Name)
		assert.Equal(t, "c", result[2].Name)
	})

	t.Run("appends to existing slice", func(t *testing.T) {
		existing := []module.Capability{{Name: "existing"}}
		providers := map[string]any{"new": "val"}
		result := appendSortedCapabilities(existing, module.CapabilitySpec{Name: "test"}, providers)
		assert.Len(t, result, 2)
		assert.Equal(t, "existing", result[0].Name)
		assert.Equal(t, "new", result[1].Name)
	})
}

// --- configPathAutoRule.Match ---

func TestConfigPathAutoRule_Match(t *testing.T) {
	t.Run("matching path", func(t *testing.T) {
		rule := configPathAutoRule{path: "yggdrasil.observability.telemetry.stats.server"}
		mgr := config.NewManager()
		require.NoError(t, mgr.LoadLayer("test", config.PriorityOverride,
			memory.NewSource("test", map[string]any{
				"yggdrasil": map[string]any{
					"observability": map[string]any{
						"telemetry": map[string]any{
							"stats": map[string]any{
								"server": map[string]any{"enabled": true},
							},
						},
					},
				},
			}),
		))
		ctx := module.AutoRuleContext{Snapshot: mgr.Snapshot()}
		assert.True(t, rule.Match(ctx))
	})

	t.Run("non-matching path", func(t *testing.T) {
		rule := configPathAutoRule{path: "yggdrasil.observability.telemetry.stats.server"}
		mgr := config.NewManager()
		ctx := module.AutoRuleContext{Snapshot: mgr.Snapshot()}
		assert.False(t, rule.Match(ctx))
	})
}

// --- configPathAutoRule.Describe ---

func TestConfigPathAutoRule_Describe(t *testing.T) {
	t.Run("returns description", func(t *testing.T) {
		rule := configPathAutoRule{description: "server stats handler configured"}
		assert.Equal(t, "server stats handler configured", rule.Describe())
	})

	t.Run("empty description fallback", func(t *testing.T) {
		rule := configPathAutoRule{description: ""}
		assert.Equal(t, "configured path matched", rule.Describe())
	})

	t.Run("whitespace description fallback", func(t *testing.T) {
		rule := configPathAutoRule{description: "  "}
		assert.Equal(t, "configured path matched", rule.Describe())
	})
}

// --- configPathAutoRule.AffectedPaths ---

func TestConfigPathAutoRule_AffectedPaths(t *testing.T) {
	t.Run("returns paths", func(t *testing.T) {
		rule := configPathAutoRule{path: "yggdrasil.observability.telemetry.stats.server"}
		assert.Equal(
			t,
			[]string{"yggdrasil.observability.telemetry.stats.server"},
			rule.AffectedPaths(),
		)
	})

	t.Run("empty path returns nil", func(t *testing.T) {
		rule := configPathAutoRule{path: ""}
		assert.Nil(t, rule.AffectedPaths())
	})

	t.Run("whitespace path returns nil", func(t *testing.T) {
		rule := configPathAutoRule{path: "  "}
		assert.Nil(t, rule.AffectedPaths())
	})
}

// --- splitConfigPath (in app package) ---

func TestSplitConfigPathCapabilities(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"single segment", "yggdrasil", []string{"yggdrasil"}},
		{
			"multiple segments",
			"yggdrasil.observability.stats",
			[]string{"yggdrasil", "observability", "stats"},
		},
		{"leading dot", ".yggdrasil", []string{"yggdrasil"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitConfigPath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
