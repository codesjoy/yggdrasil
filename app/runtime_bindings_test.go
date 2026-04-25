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

	"github.com/codesjoy/yggdrasil/v3/internal/settings"
)

// --- copyIntoMap ---

func TestCopyIntoMap(t *testing.T) {
	t.Run("copies all entries", func(t *testing.T) {
		dst := map[string]int{}
		src := map[string]int{"a": 1, "b": 2}
		copyIntoMap(dst, src)
		assert.Equal(t, 1, dst["a"])
		assert.Equal(t, 2, dst["b"])
	})

	t.Run("overwrites existing", func(t *testing.T) {
		dst := map[string]int{"a": 10}
		src := map[string]int{"a": 1}
		copyIntoMap(dst, src)
		assert.Equal(t, 1, dst["a"])
	})
}

// --- copyPreferredIntoMap ---

func TestCopyPreferredIntoMap(t *testing.T) {
	t.Run("preferred overrides source", func(t *testing.T) {
		dst := map[string]int{}
		src := map[string]int{"a": 1, "b": 2}
		preferred := map[string]int{"a": 10}
		copyPreferredIntoMap(dst, src, preferred)
		assert.Equal(t, 10, dst["a"])
		assert.Equal(t, 2, dst["b"])
	})

	t.Run("source fills missing", func(t *testing.T) {
		dst := map[string]int{}
		src := map[string]int{"a": 1, "b": 2}
		preferred := map[string]int{}
		copyPreferredIntoMap(dst, src, preferred)
		assert.Equal(t, 1, dst["a"])
		assert.Equal(t, 2, dst["b"])
	})
}

// --- mapNamedProviders ---

type stubNamedProvider struct {
	name string
}

func (s *stubNamedProvider) Name() string { return s.name }

func TestMapNamedProviders(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		result := mapNamedProviders([]*stubNamedProvider{})
		assert.Empty(t, result)
	})

	t.Run("single item", func(t *testing.T) {
		result := mapNamedProviders([]*stubNamedProvider{{name: "a"}})
		assert.Len(t, result, 1)
		assert.Contains(t, result, "a")
	})

	t.Run("multiple items", func(t *testing.T) {
		result := mapNamedProviders([]*stubNamedProvider{
			{name: "a"}, {name: "b"}, {name: "c"},
		})
		assert.Len(t, result, 3)
	})

	t.Run("dedup by name", func(t *testing.T) {
		result := mapNamedProviders([]*stubNamedProvider{
			{name: "a"}, {name: "a"},
		})
		assert.Len(t, result, 1)
	})
}

// --- runtimeRequiresRestart ---

func TestRuntimeRequiresRestart(t *testing.T) {
	t.Run("nil snapshots return false", func(t *testing.T) {
		assert.False(t, runtimeRequiresRestart(nil, nil))
		assert.False(t, runtimeRequiresRestart(nil, &Snapshot{}))
		assert.False(t, runtimeRequiresRestart(&Snapshot{}, nil))
	})

	t.Run("identical snapshots return false", func(t *testing.T) {
		current := &Snapshot{Resolved: settings.Resolved{}}
		next := &Snapshot{Resolved: settings.Resolved{}}
		assert.False(t, runtimeRequiresRestart(current, next))
	})

	t.Run("different server settings return true", func(t *testing.T) {
		current := &Snapshot{Resolved: settings.Resolved{}}
		next := &Snapshot{Resolved: settings.Resolved{}}
		next.Resolved.Server.Transports = []string{"grpc"}
		assert.True(t, runtimeRequiresRestart(current, next))
	})

	t.Run("different discovery return true", func(t *testing.T) {
		current := &Snapshot{Resolved: settings.Resolved{}}
		next := &Snapshot{Resolved: settings.Resolved{}}
		next.Resolved.Discovery.Registry.Type = "multi_registry"
		assert.True(t, runtimeRequiresRestart(current, next))
	})
}

// --- splitConfigPath ---

func TestSplitConfigPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"empty", "", []string{}},
		{"single segment", "yggdrasil", []string{"yggdrasil"}},
		{"multiple segments", "a.b.c", []string{"a", "b", "c"}},
		{"leading dot", ".a.b", []string{"a", "b"}},
		{"trailing dot", "a.b.", []string{"a", "b"}},
		{"double dot", "a..b", []string{"a", "b"}},
		{"whitespace", " a . b ", []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitConfigPath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- loggingInterceptorSource ---

func TestLoggingInterceptorSource(t *testing.T) {
	t.Run("logging key", func(t *testing.T) {
		resolved := settings.Resolved{}
		resolved.Logging.Interceptors = map[string]map[string]any{
			"logging": {"level": "info"},
		}
		result := loggingInterceptorSource(resolved)
		require.NotNil(t, result)
	})

	t.Run("logger key", func(t *testing.T) {
		resolved := settings.Resolved{}
		resolved.Logging.Interceptors = map[string]map[string]any{
			"logger": {"level": "debug"},
		}
		result := loggingInterceptorSource(resolved)
		require.NotNil(t, result)
	})

	t.Run("missing key returns nil", func(t *testing.T) {
		resolved := settings.Resolved{}
		resolved.Logging.Interceptors = map[string]map[string]any{
			"other": {"level": "info"},
		}
		result := loggingInterceptorSource(resolved)
		assert.Nil(t, result)
	})

	t.Run("empty interceptors returns nil", func(t *testing.T) {
		resolved := settings.Resolved{}
		result := loggingInterceptorSource(resolved)
		assert.Nil(t, result)
	})
}
