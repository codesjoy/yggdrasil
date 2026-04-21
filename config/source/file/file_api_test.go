package file

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileRead(t *testing.T) {
	path := t.TempDir() + "/config.yaml"
	require.NoError(t, os.WriteFile(path, []byte("app:\n  name: demo\n"), 0o600))

	src := NewSource(path, false)
	data, err := src.Read()
	require.NoError(t, err)

	var out struct {
		App struct {
			Name string `mapstructure:"name"`
		} `mapstructure:"app"`
	}
	require.NoError(t, data.Unmarshal(&out))
	require.Equal(t, "demo", out.App.Name)
}
