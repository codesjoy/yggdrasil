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

	"github.com/codesjoy/yggdrasil/v3/internal/settings"
)

type stubNamedProvider struct {
	name string
}

func (s *stubNamedProvider) Name() string { return s.name }

func TestMapNamedProviders(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		result := MapNamedProviders([]*stubNamedProvider{})
		assert.Empty(t, result)
	})

	t.Run("single item", func(t *testing.T) {
		result := MapNamedProviders([]*stubNamedProvider{{name: "a"}})
		assert.Len(t, result, 1)
		assert.Contains(t, result, "a")
	})

	t.Run("multiple items", func(t *testing.T) {
		result := MapNamedProviders([]*stubNamedProvider{
			{name: "a"}, {name: "b"}, {name: "c"},
		})
		assert.Len(t, result, 3)
	})

	t.Run("dedup by name", func(t *testing.T) {
		result := MapNamedProviders([]*stubNamedProvider{
			{name: "a"}, {name: "a"},
		})
		assert.Len(t, result, 1)
	})
}

func TestResolvedRequiresRestart(t *testing.T) {
	t.Run("identical settings return false", func(t *testing.T) {
		assert.False(t, ResolvedRequiresRestart(settings.Resolved{}, settings.Resolved{}))
	})

	t.Run("different server settings return true", func(t *testing.T) {
		current := settings.Resolved{}
		next := settings.Resolved{}
		next.Server.Transports = []string{"grpc"}
		assert.True(t, ResolvedRequiresRestart(current, next))
	})

	t.Run("different discovery return true", func(t *testing.T) {
		current := settings.Resolved{}
		next := settings.Resolved{}
		next.Discovery.Registry.Type = "multi_registry"
		assert.True(t, ResolvedRequiresRestart(current, next))
	})

	t.Run("different grpc standard service settings return true", func(t *testing.T) {
		current := settings.Resolved{}
		next := settings.Resolved{}
		next.Transports.GRPC.Server.ReflectionEnabled = true
		assert.True(t, ResolvedRequiresRestart(current, next))
	})
}

func TestLoggingInterceptorSource(t *testing.T) {
	t.Run("logging key", func(t *testing.T) {
		resolved := settings.Resolved{}
		resolved.Logging.Interceptors = map[string]map[string]any{
			"logging": {"level": "info"},
		}
		result := LoggingInterceptorSource(resolved)
		require.NotNil(t, result)
	})

	t.Run("logger key", func(t *testing.T) {
		resolved := settings.Resolved{}
		resolved.Logging.Interceptors = map[string]map[string]any{
			"logger": {"level": "debug"},
		}
		result := LoggingInterceptorSource(resolved)
		require.NotNil(t, result)
	})

	t.Run("missing key returns nil", func(t *testing.T) {
		resolved := settings.Resolved{}
		resolved.Logging.Interceptors = map[string]map[string]any{
			"other": {"level": "info"},
		}
		result := LoggingInterceptorSource(resolved)
		assert.Nil(t, result)
	})

	t.Run("empty interceptors returns nil", func(t *testing.T) {
		resolved := settings.Resolved{}
		result := LoggingInterceptorSource(resolved)
		assert.Nil(t, result)
	})
}
