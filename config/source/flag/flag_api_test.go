package flag

import (
	flag2 "flag"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFlagRead(t *testing.T) {
	fs := flag2.NewFlagSet("test", flag2.ContinueOnError)
	value := fs.String("app-server-port", "8080", "")
	require.NoError(t, fs.Parse([]string{"--app-server-port=8088"}))
	require.Equal(t, "8088", *value)

	src := NewSource(fs)
	data, err := src.Read()
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(data.Bytes(), &out))
	require.Equal(t, "8088", out["app"].(map[string]any)["server"].(map[string]any)["port"])
}
