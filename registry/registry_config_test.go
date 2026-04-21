package registry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type testRegistry struct{ kind string }

func (t *testRegistry) Register(context.Context, Instance) error   { return nil }
func (t *testRegistry) Deregister(context.Context, Instance) error { return nil }
func (t *testRegistry) Type() string                               { return t.kind }

func TestGetLoadsRegistryFromDiscoveryTree(t *testing.T) {
	RegisterBuilder("mock", func(cfg map[string]any) (Registry, error) {
		return &testRegistry{kind: cfg["name"].(string)}, nil
	})
	Configure(Spec{Type: "mock", Config: map[string]any{"name": "demo"}})

	reg, err := Get()
	require.NoError(t, err)
	require.Equal(t, "demo", reg.Type())
}

func TestNewMultiRegistry(t *testing.T) {
	RegisterBuilder("child", func(cfg map[string]any) (Registry, error) {
		return &testRegistry{kind: cfg["name"].(string)}, nil
	})

	reg, err := newMultiRegistry(map[string]any{
		"registries": []any{
			map[string]any{"type": "child", "config": map[string]any{"name": "a"}},
			map[string]any{"type": "child", "config": map[string]any{"name": "b"}},
		},
	})
	require.NoError(t, err)
	require.Equal(t, multiRegistryType, reg.Type())
}
