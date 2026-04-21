package source_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v2/config/source"
)

func TestMapDataUnmarshal(t *testing.T) {
	data := source.NewMapData(map[string]any{
		"app": map[string]any{"port": 8080},
	})

	var out struct {
		App struct {
			Port int `mapstructure:"port"`
		} `mapstructure:"app"`
	}
	require.NoError(t, data.Unmarshal(&out))
	require.Equal(t, 8080, out.App.Port)
}

func TestBytesDataBytes(t *testing.T) {
	data := source.NewBytesData([]byte(`{"app":{"name":"demo"}}`), json.Unmarshal)
	var out map[string]any
	require.NoError(t, data.Unmarshal(&out))
	require.Equal(t, "demo", out["app"].(map[string]any)["name"])
}
