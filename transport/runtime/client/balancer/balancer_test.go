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

// Package balancer implements load balancing algorithms for client requests.
package balancer

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegisterBuilder(t *testing.T) {
	testBuilder := func(serviceName, balancerName string, cli Client) (Balancer, error) {
		return nil, nil
	}

	// Register a new builder
	RegisterBuilder("test_balancer", testBuilder)

	// Verify it can be retrieved
	builder, err := GetBuilder("test_balancer")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if builder == nil {
		t.Fatal("expected builder to be non-nil")
	}
}

func TestGetBuilder_NotFound(t *testing.T) {
	_, err := GetBuilder("non_existent_balancer")
	if err == nil {
		t.Fatal("expected error for non-existent balancer")
	}
	expectedErr := "not found balancer builder, type: non_existent_balancer"
	if err.Error() != expectedErr {
		t.Fatalf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestGetBuilder_RoundRobin(t *testing.T) {
	// round_robin is registered in init()
	builder, err := GetBuilder("round_robin")
	if err != nil {
		t.Fatalf("expected no error for round_robin, got %v", err)
	}
	if builder == nil {
		t.Fatal("expected round_robin builder to be non-nil")
	}
}

func TestRegisterBuilder_Override(t *testing.T) {
	called := false
	testBuilder1 := func(serviceName, balancerName string, cli Client) (Balancer, error) {
		return nil, errors.New("builder1")
	}
	testBuilder2 := func(serviceName, balancerName string, cli Client) (Balancer, error) {
		called = true
		return nil, errors.New("builder2")
	}

	RegisterBuilder("override_test", testBuilder1)
	RegisterBuilder("override_test", testBuilder2)

	builder, err := GetBuilder("override_test")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, err = builder("test_service", "test_balancer", nil)
	if !called {
		t.Fatal("expected second builder to be called")
	}
	if err == nil || err.Error() != "builder2" {
		t.Fatalf("expected error 'builder2', got %v", err)
	}
}

func RegisterBuilder(typeName string, f Builder) {
	mu.Lock()
	defer mu.Unlock()
	providers[typeName] = NewProvider(typeName, f)
}

func GetBuilder(typeName string) (Builder, error) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := providers[typeName]
	if !ok {
		return nil, fmt.Errorf("not found balancer builder, type: %s", typeName)
	}
	return p.New, nil
}

func TestResolveType_WithDefault(t *testing.T) {
	tests := []struct {
		name         string
		balancerName string
		setupConfig  func()
		want         string
		wantErr      bool
	}{
		{
			name:         "default balancer with no config uses round_robin",
			balancerName: "default",
			setupConfig:  func() {},
			want:         "round_robin",
		},
		{
			name:         "custom balancer without config returns error",
			balancerName: "custom",
			setupConfig:  func() {},
			wantErr:      true,
		},
		{
			name:         "configured balancer returns configured type",
			balancerName: "my-balancer",
			setupConfig: func() {
				Configure(map[string]Spec{"my-balancer": {Type: "weighted"}}, nil)
			},
			want: "weighted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupConfig()
			got, err := ResolveType(tt.balancerName)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestLoadConfig(t *testing.T) {
	t.Run("defaults only", func(t *testing.T) {
		Configure(map[string]Spec{
			"mybal": {Type: "round_robin", Config: map[string]any{"key1": "val1"}},
		}, nil)
		t.Cleanup(func() { Configure(nil, nil) })

		cfg := LoadConfig("svc", "mybal")
		require.Equal(t, "val1", cfg["key1"])
	})

	t.Run("service override merges", func(t *testing.T) {
		Configure(
			map[string]Spec{
				"mybal": {Type: "round_robin", Config: map[string]any{"key1": "val1"}},
			},
			map[string]map[string]Spec{
				"svc": {
					"mybal": {Config: map[string]any{"key2": "val2"}},
				},
			},
		)
		t.Cleanup(func() { Configure(nil, nil) })

		cfg := LoadConfig("svc", "mybal")
		require.Equal(t, "val1", cfg["key1"])
		require.Equal(t, "val2", cfg["key2"])
	})

	t.Run("no config", func(t *testing.T) {
		Configure(nil, nil)
		require.Empty(t, LoadConfig("svc", "mybal"))
	})
}

func TestNew(t *testing.T) {
	t.Run("default balancer", func(t *testing.T) {
		b, err := New("svc", DefaultBalancerName, newMockBalancerClient())
		require.NoError(t, err)
		require.NotNil(t, b)
		require.Equal(t, "round_robin", b.Type())
	})

	t.Run("unknown type", func(t *testing.T) {
		Configure(nil, nil)
		_, err := New("svc", "unknown_balancer", newMockBalancerClient())
		require.Error(t, err)
	})

	t.Run("no provider for type", func(t *testing.T) {
		Configure(map[string]Spec{
			"mybal": {Type: "nonexistent_type"},
		}, nil)
		t.Cleanup(func() { Configure(nil, nil) })

		_, err := New("svc", "mybal", newMockBalancerClient())
		require.Error(t, err)
	})
}

func TestConfigureProviders(t *testing.T) {
	t.Cleanup(func() {
		require.NoError(t, ConfigureProviders([]Provider{BuiltinProvider()}))
	})

	t.Run("valid", func(t *testing.T) {
		p := NewProvider(
			"custom",
			func(string, string, Client) (Balancer, error) { return nil, nil },
		)
		require.NoError(t, ConfigureProviders([]Provider{p}))

		got, ok := GetProvider("custom")
		require.True(t, ok)
		require.Equal(t, "custom", got.Type())
	})

	t.Run("duplicate type", func(t *testing.T) {
		provider := NewProvider(
			"dup",
			func(string, string, Client) (Balancer, error) { return nil, nil },
		)
		require.Error(t, ConfigureProviders([]Provider{provider, provider}))
	})

	t.Run("empty type", func(t *testing.T) {
		provider := NewProvider(
			"",
			func(string, string, Client) (Balancer, error) { return nil, nil },
		)
		require.Error(t, ConfigureProviders([]Provider{provider}))
	})

	t.Run("nil skipped", func(t *testing.T) {
		require.NoError(t, ConfigureProviders([]Provider{nil, BuiltinProvider()}))
		got, ok := GetProvider("round_robin")
		require.True(t, ok)
		require.NotNil(t, got)
	})
}

func TestGetProvider(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		got, ok := GetProvider("round_robin")
		require.True(t, ok)
		require.Equal(t, "round_robin", got.Type())
	})

	t.Run("not found", func(t *testing.T) {
		_, ok := GetProvider("nonexistent_type")
		require.False(t, ok)
	})
}

func TestProvider(t *testing.T) {
	t.Run("type", func(t *testing.T) {
		provider := NewProvider("mytype", nil)
		require.Equal(t, "mytype", provider.Type())
	})

	t.Run("new", func(t *testing.T) {
		called := false
		provider := NewProvider(
			"test_new",
			func(string, string, Client) (Balancer, error) {
				called = true
				return nil, errors.New("test error")
			},
		)
		_, err := provider.New("svc", "bal", nil)
		require.True(t, called)
		require.EqualError(t, err, "test error")
	})
}
