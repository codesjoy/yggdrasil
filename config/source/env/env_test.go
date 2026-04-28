// Copyright 2022 The codesjoy Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

func TestParseValue(t *testing.T) {
	src := &env{}
	require.Equal(t, 10, src.parseValue("10"))
	require.Equal(t, true, src.parseValue("true"))
	require.Equal(t, 3.14, src.parseValue("3.14"))
	require.Equal(t, "value", src.parseValue("value"))
}

func TestEnvReadWithOptions(t *testing.T) {
	t.Setenv("APP__SERVER__PORT", "9090")
	t.Setenv("APP__FEATURES", "a,b,3,true")
	t.Setenv("RAW__PORT", "8081")
	t.Setenv("APP__IGNORED", "nope")

	src := NewSource(
		[]string{"APP"},
		[]string{"RAW"},
		SetKeyDelimiter("__"),
		WithParseArray(","),
		WithName("custom-env"),
		WithIgnoredKeys("APP__IGNORED"),
	)
	data, err := src.Read()
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(data.Bytes(), &out))
	require.Equal(t, float64(9090), out["app"].(map[string]any)["server"].(map[string]any)["port"])
	require.Equal(t, []any{"a", "b", float64(3), true}, out["app"].(map[string]any)["features"])
	require.NotContains(t, out["app"].(map[string]any), "ignored")
	require.Equal(t, float64(8081), out["port"])

	require.Equal(t, "custom-env", src.Name())
	require.Equal(t, "env", src.Kind())
	require.NoError(t, src.Close())
}

func TestEnvReadIgnoresConfigSourcesControlVariableByDefault(t *testing.T) {
	t.Setenv("YGGDRASIL_CONFIG_SOURCES", "env:APP:env")
	src := NewSource([]string{"YGGDRASIL"}, nil)
	data, err := src.Read()
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(data.Bytes(), &out))
	require.Empty(t, out)
}

func TestOptionFallbacks(t *testing.T) {
	e := &env{}
	WithParseArray("")(e)
	SetKeyDelimiter("")(e)
	require.True(t, e.parseArray)
	require.Equal(t, ";", e.arraySep)
	require.Equal(t, "_", e.delimiter)
}

func TestHasPrefix(t *testing.T) {
	require.True(t, hasPrefix("app_server_port", "app", "_"))
	require.True(t, hasPrefix("app", "app", "_"))
	require.False(t, hasPrefix("apple", "app", "_"))
	require.False(t, hasPrefix("service_app_port", "app", "_"))
}
