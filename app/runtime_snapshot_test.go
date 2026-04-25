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

	"github.com/codesjoy/yggdrasil/v3/discovery/registry"
	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	"github.com/codesjoy/yggdrasil/v3/transport/gateway/rest"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/client"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/client/balancer"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/server"
)

// --- Snapshot nil receiver ---

func TestSnapshot_NilReceiver(t *testing.T) {
	t.Run("Copy returns nil", func(t *testing.T) {
		var s *Snapshot
		assert.Nil(t, s.Copy())
	})

	t.Run("ClientSettings returns zero", func(t *testing.T) {
		var s *Snapshot
		result := s.ClientSettings("svc")
		assert.Equal(t, client.ServiceSettings{}, result)
	})

	t.Run("ClientStatsHandler returns NoOp", func(t *testing.T) {
		var s *Snapshot
		assert.Equal(t, stats.NoOpHandler, s.ClientStatsHandler())
	})

	t.Run("ServerStatsHandler returns NoOp", func(t *testing.T) {
		var s *Snapshot
		assert.Equal(t, stats.NoOpHandler, s.ServerStatsHandler())
	})

	t.Run("ServerSettings returns zero", func(t *testing.T) {
		var s *Snapshot
		result := s.ServerSettings()
		assert.Equal(t, server.Settings{}, result)
	})

	t.Run("RESTConfig returns nil", func(t *testing.T) {
		var s *Snapshot
		assert.Nil(t, s.RESTConfig())
	})

	t.Run("RESTMiddlewareProviders returns empty", func(t *testing.T) {
		var s *Snapshot
		assert.Empty(t, s.RESTMiddlewareProviders())
	})

	t.Run("MarshalerBuilders returns empty", func(t *testing.T) {
		var s *Snapshot
		assert.Empty(t, s.MarshalerBuilders())
	})

	t.Run("TransportServerProvider returns nil", func(t *testing.T) {
		var s *Snapshot
		assert.Nil(t, s.TransportServerProvider("grpc"))
	})

	t.Run("TransportClientProvider returns nil", func(t *testing.T) {
		var s *Snapshot
		assert.Nil(t, s.TransportClientProvider("grpc"))
	})

	t.Run("NewRegistry returns error", func(t *testing.T) {
		var s *Snapshot
		_, err := s.NewRegistry()
		require.Error(t, err)
	})

	t.Run("NewResolver returns error", func(t *testing.T) {
		var s *Snapshot
		_, err := s.NewResolver("test")
		require.Error(t, err)
	})

	t.Run("BuildDefaultLoggerHandler returns error", func(t *testing.T) {
		var s *Snapshot
		_, err := s.BuildDefaultLoggerHandler()
		require.Error(t, err)
	})

	t.Run("BuildTracerProvider returns false", func(t *testing.T) {
		var s *Snapshot
		_, ok := s.BuildTracerProvider("test")
		assert.False(t, ok)
	})

	t.Run("BuildMeterProvider returns false", func(t *testing.T) {
		var s *Snapshot
		_, ok := s.BuildMeterProvider("test")
		assert.False(t, ok)
	})
}

// --- Snapshot.NewRegistry ---

func TestSnapshot_NewRegistry(t *testing.T) {
	t.Run("empty type error", func(t *testing.T) {
		s := &Snapshot{Resolved: settings.Resolved{}}
		_, err := s.NewRegistry()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found registry type")
	})
}

// --- Snapshot.NewRegistryByType ---

func TestSnapshot_NewRegistryByType(t *testing.T) {
	t.Run("missing provider returns error", func(t *testing.T) {
		s := &Snapshot{
			Resolved:          settings.Resolved{},
			RegistryProviders: map[string]registry.Provider{},
		}
		_, err := s.NewRegistryByType("missing", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found registry provider")
	})
}

// --- Snapshot.NewResolver ---

func TestSnapshot_NewResolver(t *testing.T) {
	t.Run("missing type returns error", func(t *testing.T) {
		s := &Snapshot{Resolved: settings.Resolved{}}
		s.Resolved.Discovery.Resolvers = map[string]resolver.Spec{
			"custom": {Type: "missing"},
		}
		_, err := s.NewResolver("custom")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found resolver provider")
	})

	t.Run("default resolver with empty type returns nil nil", func(t *testing.T) {
		s := &Snapshot{Resolved: settings.Resolved{}}
		r, err := s.NewResolver(resolver.DefaultResolverName)
		require.NoError(t, err)
		assert.Nil(t, r)
	})
}

// --- Snapshot.NewBalancer ---

func TestSnapshot_NewBalancer(t *testing.T) {
	t.Run("missing provider returns error", func(t *testing.T) {
		s := &Snapshot{
			Resolved:          settings.Resolved{},
			BalancerProviders: map[string]balancer.Provider{},
		}
		s.Resolved.Balancers.Defaults = map[string]balancer.Spec{
			"test": {Type: "missing"},
		}
		_, err := s.NewBalancer("svc", "test", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found balancer provider")
	})

	t.Run("missing type with non-default name returns error", func(t *testing.T) {
		s := &Snapshot{
			Resolved:          settings.Resolved{},
			BalancerProviders: map[string]balancer.Provider{},
		}
		_, err := s.NewBalancer("svc", "unknown", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found balancer type")
	})
}

// --- Snapshot.ClientSettings ---

func TestSnapshot_ClientSettings(t *testing.T) {
	t.Run("returns per-service settings", func(t *testing.T) {
		s := &Snapshot{Resolved: settings.Resolved{}}
		s.Resolved.Clients.Services = map[string]client.ServiceSettings{
			"svc": {Resolver: "test-resolver"},
		}
		result := s.ClientSettings("svc")
		assert.Equal(t, "test-resolver", result.Resolver)
	})

	t.Run("missing service returns zero", func(t *testing.T) {
		s := &Snapshot{Resolved: settings.Resolved{}}
		result := s.ClientSettings("missing")
		assert.Equal(t, client.ServiceSettings{}, result)
	})
}

// --- Snapshot.ServerSettings ---

func TestSnapshot_ServerSettings(t *testing.T) {
	t.Run("returns server settings", func(t *testing.T) {
		s := &Snapshot{Resolved: settings.Resolved{}}
		s.Resolved.Server.Transports = []string{"grpc"}
		result := s.ServerSettings()
		assert.Equal(t, []string{"grpc"}, result.Transports)
	})
}

// --- Snapshot.RESTConfig ---

func TestSnapshot_RESTConfig(t *testing.T) {
	t.Run("returns nil when no rest config", func(t *testing.T) {
		s := &Snapshot{Resolved: settings.Resolved{}}
		assert.Nil(t, s.RESTConfig())
	})

	t.Run("returns rest config", func(t *testing.T) {
		s := &Snapshot{Resolved: settings.Resolved{}}
		s.Resolved.Transports.Rest = &rest.Config{}
		assert.NotNil(t, s.RESTConfig())
	})
}

// --- cloneMap ---

func TestCloneMap(t *testing.T) {
	t.Run("nil map returns empty", func(t *testing.T) {
		result := cloneMap[string, int](nil)
		assert.Equal(t, map[string]int{}, result)
	})

	t.Run("normal map cloned", func(t *testing.T) {
		original := map[string]int{"a": 1, "b": 2}
		cloned := cloneMap(original)
		assert.Equal(t, original, cloned)
	})

	t.Run("mutation safety for keys", func(t *testing.T) {
		original := map[string]int{"a": 1}
		cloned := cloneMap(original)
		cloned["new"] = 2
		_, exists := original["new"]
		assert.False(t, exists)
	})
}
