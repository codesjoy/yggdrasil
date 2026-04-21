package env

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvRead(t *testing.T) {
	t.Setenv("APP_SERVER_PORT", "8081")
	src := NewSource([]string{"APP"}, nil)
	data, err := src.Read()
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(data.Bytes(), &out))
	require.Equal(t, float64(8081), out["app"].(map[string]any)["server"].(map[string]any)["port"])
}
