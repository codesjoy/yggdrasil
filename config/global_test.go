package config

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v2/config/source/memory"
)

func TestDefaultSetDefaultBindCurrent(t *testing.T) {
	prev := Default()
	t.Cleanup(func() { SetDefault(prev) })

	manager := NewManager()
	old := SetDefault(manager)
	require.Equal(t, prev, old)
	require.Equal(t, manager, Default())

	require.NoError(t, manager.LoadLayer("defaults", PriorityDefaults, memory.NewSource("defaults", map[string]any{
		"app": map[string]any{"port": 8088},
	})))
	section := Bind[struct {
		Port int `mapstructure:"port"`
	}](nil, "app")
	current, err := section.Current()
	require.NoError(t, err)
	require.Equal(t, 8088, current.Port)

	returned := SetDefault(nil)
	require.Equal(t, manager, returned)
	require.NotNil(t, Default())
	require.NotEqual(t, manager, Default())
}
