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

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v2/config/source/env"
	"github.com/codesjoy/yggdrasil/v2/config/source/file"
)

func TestConfig_EnvSourceOverridesInterpolatedFileValue(t *testing.T) {
	t.Setenv("TESTCFG_FILE_DB_PASSWORD", "from-file")
	t.Setenv("TESTCFG_DATABASE_PASSWORD", "from-env-source")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(
		t,
		os.WriteFile(
			configPath,
			[]byte("database:\n  password: ${TESTCFG_FILE_DB_PASSWORD}\n"),
			0o600,
		),
	)

	cfg := NewConfig(".")
	err := cfg.LoadSource(
		file.NewSource(configPath, false),
		env.NewSource(nil, []string{"TESTCFG"}),
	)
	require.NoError(t, err)

	assert.Equal(t, "from-env-source", cfg.Get("database.password").String())
}

func TestConfig_TextPlaceholderDoesNotProvideTypedInjection(t *testing.T) {
	t.Setenv("TESTCFG_SERVER_PORT", "6379")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(
		t,
		os.WriteFile(configPath, []byte("server:\n  port: ${TESTCFG_SERVER_PORT}\n"), 0o600),
	)

	cfg := NewConfig(".")
	err := cfg.LoadSource(file.NewSource(configPath, false))
	require.NoError(t, err)

	assert.Equal(t, "6379", cfg.Get("server.port").String())

	type serverConfig struct {
		Port int `mapstructure:"port"`
	}

	var server serverConfig
	err = cfg.Get("server").Scan(&server)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected type 'int'")
}
