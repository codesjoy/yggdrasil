package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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

func TestSnapshotHelpersAndDecodeEdgeCases(t *testing.T) {
	snapshot := NewSnapshot(map[string]any{
		"app": map[string]any{
			"name": "demo",
		},
		"value": 12,
	})

	m := snapshot.Map()
	m["app"].(map[string]any)["name"] = "changed"
	require.Equal(t, "demo", snapshot.Map()["app"].(map[string]any)["name"])
	require.Contains(t, string(snapshot.Bytes()), `"value":12`)
	require.False(t, snapshot.Empty())

	value := snapshot.Value().(map[string]any)
	value["value"] = 100
	require.Equal(t, 12, snapshot.Value().(map[string]any)["value"])

	empty := NewSnapshot(nil)
	require.True(t, empty.Empty())
	require.Equal(t, map[string]any{}, empty.Map())

	var nonPtr int
	require.Error(t, snapshot.Decode(nonPtr))
	var nilPtr *int
	require.Error(t, snapshot.Decode(nilPtr))

	type withDefault struct {
		Port int `default:"8080"`
	}
	var cfg withDefault
	require.NoError(t, empty.Decode(&cfg))
	require.Equal(t, 8080, cfg.Port)

	var structTarget struct {
		Value int `mapstructure:"value"`
	}
	require.Error(t, NewSnapshot("bad").Decode(&structTarget))

	var intTarget int
	require.Error(t, NewSnapshot("bad").Decode(&intTarget))
}
