package settings

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v2/config"
)

func TestCompile_ClientFastFailServiceOverrideFalse(t *testing.T) {
	root := decodeRoot(t, map[string]any{
		"yggdrasil": map[string]any{
			"clients": map[string]any{
				"defaults": map[string]any{
					"fast_fail": true,
				},
				"services": map[string]any{
					"svc": map[string]any{
						"fast_fail": false,
					},
				},
			},
		},
	})

	resolved, err := Compile(root)
	require.NoError(t, err)
	require.False(t, resolved.Clients.Services["svc"].FastFail)
}

func TestCompile_ClientFastFailServiceInheritsDefault(t *testing.T) {
	root := decodeRoot(t, map[string]any{
		"yggdrasil": map[string]any{
			"clients": map[string]any{
				"defaults": map[string]any{
					"fast_fail": true,
				},
				"services": map[string]any{
					"svc": map[string]any{},
				},
			},
		},
	})

	resolved, err := Compile(root)
	require.NoError(t, err)
	require.True(t, resolved.Clients.Services["svc"].FastFail)
}

func decodeRoot(t *testing.T, payload map[string]any) Root {
	t.Helper()
	var root Root
	require.NoError(t, config.NewSnapshot(payload).Decode(&root))
	return root
}
