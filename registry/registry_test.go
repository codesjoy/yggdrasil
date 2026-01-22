package registry

import (
	"context"
	"testing"

	"github.com/codesjoy/yggdrasil/v2/config"
)

type testRegistry struct {
	instanceName string
}

func (r *testRegistry) Register(context.Context, Instance) error   { return nil }
func (r *testRegistry) Deregister(context.Context, Instance) error { return nil }
func (r *testRegistry) Type() string                               { return r.instanceName }

func resetRegistryState() {
	mu.Lock()
	defer mu.Unlock()
	builders = make(map[string]Builder)
	defaultReg = nil
}

func TestGet_SingleConfig_CachesDefaultInstance(t *testing.T) {
	resetRegistryState()
	origKeyBase := config.KeyBase
	config.KeyBase = "yggdrasil_test_registry_single"
	t.Cleanup(func() { config.KeyBase = origKeyBase })

	if err := config.Set(config.Join(config.KeyBase, "registry", "type"), "mock"); err != nil {
		t.Fatalf("Set(registry.type) error = %v", err)
	}

	RegisterBuilder("mock", func(cfg config.Value) (Registry, error) {
		name := "default"
		if m := cfg.Map(nil); m != nil {
			if v, ok := m["name"].(string); ok && v != "" {
				name = v
			}
		}
		return &testRegistry{instanceName: name}, nil
	})

	if err := config.Set(config.Join(config.KeyBase, "registry", "config"), map[string]any{"name": "r"}); err != nil {
		t.Fatalf("Set(registry.config) error = %v", err)
	}

	r1, err := Get()
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	r2, err := Get()
	if err != nil {
		t.Fatalf("Get() second call error = %v", err)
	}
	if r1 != r2 {
		t.Fatalf("expected Get() to return cached instance")
	}
	if r1.Type() != "r" {
		t.Fatalf("unexpected registry type: %q", r1.Type())
	}
}

func TestGet_MissingTypeReturnsError(t *testing.T) {
	resetRegistryState()
	origKeyBase := config.KeyBase
	config.KeyBase = "yggdrasil_test_registry_missing_type"
	t.Cleanup(func() { config.KeyBase = origKeyBase })

	if _, err := Get(); err == nil {
		t.Fatalf("Get() expected error, got nil")
	}
}
