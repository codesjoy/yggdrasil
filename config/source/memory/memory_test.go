package memory

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMemorySourceReadAndMetadata(t *testing.T) {
	src := NewSource("mem", map[string]any{
		"app": map[string]any{
			"name": "demo",
			"list": []any{map[string]any{"k": "v"}},
		},
	})

	require.Equal(t, "memory", src.Kind())
	require.Equal(t, "mem", src.Name())

	data, err := src.Read()
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, data.Unmarshal(&out))
	require.Equal(t, "demo", out["app"].(map[string]any)["name"])

	// bytes/unmarshal output should be independent from source internal data.
	out["app"].(map[string]any)["name"] = "changed"
	data2, err := src.Read()
	require.NoError(t, err)
	var out2 map[string]any
	require.NoError(t, data2.Unmarshal(&out2))
	require.Equal(t, "demo", out2["app"].(map[string]any)["name"])

	require.NoError(t, src.Close())
}
