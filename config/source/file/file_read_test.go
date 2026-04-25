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

package file

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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

func TestIsStructuredTextParser(t *testing.T) {
	require.False(t, isStructuredTextParser(nil))
	require.True(t, isStructuredTextParser(json.Unmarshal))
	require.True(t, isStructuredTextParser(yaml.Unmarshal))
	//nolint:gocritic // intentional: wrapper must differ from direct reference
	require.False(t, isStructuredTextParser(func(b []byte, v any) error {
		return json.Unmarshal(b, v)
	}))
}

func TestExpandStringValues(t *testing.T) {
	t.Setenv("APP_NAME", "demo")
	t.Setenv("APP_ENV", "prod")

	out, err := expandStringValues("scope", map[string]any{
		"name": "${APP_NAME}",
		"nested": map[string]any{
			"env": "${APP_ENV}",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "demo", out["name"])
	require.Equal(t, "prod", out["nested"].(map[string]any)["env"])

	_, err = expandStringValues("scope", map[string]any{"name": "${NOT_FOUND}"})
	require.Error(t, err)
}

func TestFileReadStructuredAndRawPlaceholders(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APP_NAME", "demo")

	yamlPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(yamlPath, []byte("app:\n  name: ${APP_NAME}\n"), 0o600))

	yamlSrc := NewSource(yamlPath, false)
	data, err := yamlSrc.Read()
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, data.Unmarshal(&out))
	require.Equal(t, "demo", out["app"].(map[string]any)["name"])
	require.Equal(t, yamlPath, yamlSrc.Name())
	require.Equal(t, "file", yamlSrc.Kind())

	rawPath := filepath.Join(dir, "config.txt")
	require.NoError(t, os.WriteFile(rawPath, []byte(`{"app":{"name":"${APP_NAME}"}}`), 0o600))
	rawParser := json.Unmarshal
	rawSrc := NewSource(rawPath, false, rawParser)
	rawData, err := rawSrc.Read()
	require.NoError(t, err)
	var rawOut map[string]any
	require.NoError(t, rawData.Unmarshal(&rawOut))
	require.Equal(t, "demo", rawOut["app"].(map[string]any)["name"])

	require.NoError(t, os.WriteFile(rawPath, []byte(`{"app":{"name":"${MISSING}"}}`), 0o600))
	_, err = rawSrc.Read()
	require.Error(t, err)

	require.NoError(t, yamlSrc.Close())
}

func TestNewSourceParserSelection(t *testing.T) {
	yamlSrc := NewSource("/tmp/a.yaml", false).(*file)
	require.NotNil(t, yamlSrc.parser)

	custom := NewSource("/tmp/a.unknown", false, json.Unmarshal).(*file)
	require.NotNil(t, custom.parser)
}
