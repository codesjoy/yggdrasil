package registry

import (
	"context"
	"errors"
	"testing"

	"github.com/codesjoy/yggdrasil/v2/config"
)

type countingRegistry struct {
	id              string
	registerCalls   int
	deregisterCalls int
	registerErr     error
}

func (r *countingRegistry) Type() string { return r.id }
func (r *countingRegistry) Register(context.Context, Instance) error {
	r.registerCalls++
	return r.registerErr
}

func (r *countingRegistry) Deregister(context.Context, Instance) error {
	r.deregisterCalls++
	return nil
}

func TestMultiRegistry_DispatchesRegisterToAllChildren(t *testing.T) {
	resetRegistryState()
	RegisterBuilder(multiRegistryType, newMultiRegistry)

	var children []*countingRegistry
	RegisterBuilder("mockchild", func(cfg config.Value) (Registry, error) {
		id := "x"
		if m := cfg.Map(nil); m != nil {
			if v, ok := m["id"].(string); ok && v != "" {
				id = v
			}
		}
		cr := &countingRegistry{id: id}
		children = append(children, cr)
		return cr, nil
	})

	c := config.NewConfig(".")
	_ = c.Set("cfg", map[string]any{
		"registries": []any{
			map[string]any{"type": "mockchild", "config": map[string]any{"id": "a"}},
			map[string]any{"type": "mockchild", "config": map[string]any{"id": "b"}},
		},
	})
	mr, err := New(multiRegistryType, c.Get("cfg"))
	if err != nil {
		t.Fatalf("New(multi_registry) error = %v", err)
	}

	if err := mr.Register(context.Background(), nil); err != nil {
		t.Fatalf("Register error = %v", err)
	}
	if len(children) != 2 {
		t.Fatalf("children = %d, want 2", len(children))
	}
	for _, child := range children {
		if child.registerCalls != 1 {
			t.Fatalf("child %s registerCalls = %d, want 1", child.id, child.registerCalls)
		}
	}
}

func TestMultiRegistry_FailFastStopsOnFirstError(t *testing.T) {
	resetRegistryState()
	RegisterBuilder(multiRegistryType, newMultiRegistry)

	var children []*countingRegistry
	RegisterBuilder("mockchild", func(cfg config.Value) (Registry, error) {
		id := "x"
		if m := cfg.Map(nil); m != nil {
			if v, ok := m["id"].(string); ok && v != "" {
				id = v
			}
		}
		cr := &countingRegistry{id: id}
		if id == "a" {
			cr.registerErr = errors.New("boom")
		}
		children = append(children, cr)
		return cr, nil
	})

	c := config.NewConfig(".")
	_ = c.Set("cfg", map[string]any{
		"failFast": true,
		"registries": []any{
			map[string]any{"type": "mockchild", "config": map[string]any{"id": "a"}},
			map[string]any{"type": "mockchild", "config": map[string]any{"id": "b"}},
		},
	})
	mr, err := New(multiRegistryType, c.Get("cfg"))
	if err != nil {
		t.Fatalf("New(multi_registry) error = %v", err)
	}

	if err := mr.Register(context.Background(), nil); err == nil {
		t.Fatalf("expected error, got nil")
	}
	if len(children) != 2 {
		t.Fatalf("children = %d, want 2", len(children))
	}
	if children[0].registerCalls != 1 {
		t.Fatalf("child a registerCalls = %d, want 1", children[0].registerCalls)
	}
	if children[1].registerCalls != 0 {
		t.Fatalf("child b registerCalls = %d, want 0", children[1].registerCalls)
	}
}
