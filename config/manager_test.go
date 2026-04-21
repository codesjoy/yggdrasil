package config

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v2/config/source/memory"
)

func TestManagerLoadLayerRespectsPriorityAndReplacement(t *testing.T) {
	manager := NewManager()

	require.NoError(t, manager.LoadLayer("defaults", PriorityDefaults, memory.NewSource("defaults", map[string]any{
		"app": map[string]any{
			"name": "base",
			"port": 8080,
		},
	})))
	require.NoError(t, manager.LoadLayer("env", PriorityEnv, memory.NewSource("env", map[string]any{
		"app": map[string]any{
			"port": 9090,
		},
	})))

	var cfg struct {
		Name string `mapstructure:"name"`
		Port int    `mapstructure:"port"`
	}
	require.NoError(t, manager.Section("app").Decode(&cfg))
	require.Equal(t, "base", cfg.Name)
	require.Equal(t, 9090, cfg.Port)

	require.NoError(t, manager.LoadLayer("env", PriorityEnv, memory.NewSource("env", map[string]any{
		"app": map[string]any{
			"port": 10001,
		},
	})))
	require.NoError(t, manager.Section("app").Decode(&cfg))
	require.Equal(t, 10001, cfg.Port)
}

func TestSnapshotSectionAndDecode(t *testing.T) {
	snapshot := NewSnapshot(map[string]any{
		"clients": map[string]any{
			"services": map[string]any{
				"github.com.demo.service": map[string]any{
					"resolver": "mock",
				},
			},
		},
	})

	var resolver string
	require.NoError(t, snapshot.Section("clients", "services", "github.com.demo.service", "resolver").Decode(&resolver))
	require.Equal(t, "mock", resolver)
}

func TestTypedSectionWatchIsScoped(t *testing.T) {
	manager := NewManager()
	type appCfg struct {
		Enabled bool `mapstructure:"enabled"`
	}

	section := Bind[appCfg](manager, "app")
	events := make([]bool, 0, 2)
	cancel := section.Watch(func(next appCfg, err error) {
		require.NoError(t, err)
		events = append(events, next.Enabled)
	})
	defer cancel()

	require.NoError(t, manager.LoadLayer("first", PriorityFile, memory.NewSource("first", map[string]any{
		"app": map[string]any{"enabled": true},
	})))
	require.NoError(t, manager.LoadLayer("other", PriorityOverride, memory.NewSource("other", map[string]any{
		"other": map[string]any{"value": 1},
	})))

	require.Equal(t, []bool{false, true}, events)
}
