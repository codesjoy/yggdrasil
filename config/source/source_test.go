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

package source

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestMapDataUnmarshal(t *testing.T) {
	data := NewMapData(map[string]any{
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
	data := NewBytesData([]byte(`{"app":{"name":"demo"}}`), json.Unmarshal)
	var out map[string]any
	require.NoError(t, data.Unmarshal(&out))
	require.Equal(t, "demo", out["app"].(map[string]any)["name"])
}

func TestParserUnmarshalText(t *testing.T) {
	var p Parser
	require.NoError(t, p.UnmarshalText([]byte("json")))
	require.NoError(t, p.UnmarshalText([]byte("yaml")))
	require.NoError(t, p.UnmarshalText([]byte("yml")))
	require.NoError(t, p.UnmarshalText([]byte("toml")))

	err := p.UnmarshalText([]byte("unknown"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown parser format")

	var nilParser *Parser
	err = nilParser.UnmarshalText([]byte("json"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "nil *Parser")
}

func TestParseParser(t *testing.T) {
	p, err := ParseParser("json")
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, p([]byte(`{"a":1}`), &out))
	require.Equal(t, float64(1), out["a"])

	_, err = ParseParser("unknown")
	require.Error(t, err)
}

func TestBytesDataBytesIsClone(t *testing.T) {
	data := NewBytesData([]byte(`{"app":{"name":"demo"}}`), json.Unmarshal)
	raw := data.Bytes()
	raw[0] = '['

	var out map[string]any
	require.NoError(t, data.Unmarshal(&out))
	require.Equal(t, "demo", out["app"].(map[string]any)["name"])
}

func TestMapDataBytesAndUnmarshal(t *testing.T) {
	data := NewMapData(map[string]any{
		"app": map[string]any{
			"name": "demo",
		},
	})

	raw := data.Bytes()
	var out map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &out))
	require.Equal(t, "demo", out["app"].(map[string]any)["name"])
}
