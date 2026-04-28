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

package chain

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source"
	"github.com/codesjoy/yggdrasil/v3/config/source/memory"
)

type failingSource struct {
	err error
}

func (s *failingSource) Kind() string { return "failing" }
func (s *failingSource) Name() string { return "failing" }
func (s *failingSource) Read() (source.Data, error) {
	return nil, s.err
}
func (s *failingSource) Close() error { return nil }

func TestLoaderLoadFileValidationAndMissingFile(t *testing.T) {
	loader := NewLoader(nil)
	manager := config.NewManager()

	_, _, err := loader.LoadFile(manager, "", true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "path is empty")

	missing := filepath.Join(t.TempDir(), "missing.yaml")
	loaded, ok, err := loader.LoadFile(manager, missing, false)
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, loaded)

	_, _, err = loader.LoadFile(manager, missing, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestLoaderLoadFileSuccessAndDisabledSourceSkipped(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	overridePath := filepath.Join(dir, "override.yaml")

	require.NoError(t, os.WriteFile(overridePath, []byte("app:\n  name: override\n"), 0o600))
	require.NoError(t, os.WriteFile(configPath, []byte(
		"app:\n  name: base\n"+
			"yggdrasil:\n"+
			"  config:\n"+
			"    sources:\n"+
			"      - kind: file\n"+
			"        config:\n"+
			"          path: "+overridePath+"\n"+
			"      - kind: env\n"+
			"        enabled: false\n"+
			"        config:\n"+
			"          prefixes: [APP]\n",
	), 0o600))

	loader := NewLoader(nil)
	manager := config.NewManager()
	loaded, ok, err := loader.LoadFile(manager, configPath, true)
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, loaded, 2) // config file + one enabled source

	var out struct {
		Name string `mapstructure:"name"`
	}
	require.NoError(t, manager.Section("app").Decode(&out))
	require.Equal(t, "override", out.Name)
}

func TestLoaderContextBuilderReadsBaseSnapshot(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	require.NoError(t, os.WriteFile(configPath, []byte(
		"bootstrap:\n  source_name: dynamic\n"+
			"app:\n  name: base\n"+
			"yggdrasil:\n"+
			"  config:\n"+
			"    sources:\n"+
			"      - kind: custom\n"+
			"        config:\n"+
			"          value: remote\n",
	), 0o600))

	registry := NewRegistry()
	registry.RegisterContext(
		"custom",
		func(ctx BuildContext, spec SourceSpec) (source.Source, config.Priority, error) {
			var bootstrap struct {
				SourceName string `mapstructure:"source_name"`
			}
			if err := ctx.Snapshot.Section("bootstrap").Decode(&bootstrap); err != nil {
				return nil, 0, err
			}
			return memory.NewSource(bootstrap.SourceName, map[string]any{
				"app": map[string]any{"name": spec.Config["value"]},
			}), config.PriorityRemote, nil
		},
	)

	manager := config.NewManager()
	loaded, ok, err := NewLoader(registry).LoadFile(manager, configPath, true)
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, loaded, 2)

	var out struct {
		Name string `mapstructure:"name"`
	}
	require.NoError(t, manager.Section("app").Decode(&out))
	require.Equal(t, "remote", out.Name)
}

func TestLoaderLoadFileBuildAndLoadErrors(t *testing.T) {
	buildErrRegistry := NewRegistry()
	buildErrRegistry.Register(
		"custom",
		func(spec SourceSpec) (source.Source, config.Priority, error) {
			return nil, 0, errors.New("build failed")
		},
	)
	loader := NewLoader(buildErrRegistry)
	manager := config.NewManager()

	dir := t.TempDir()
	buildErrPath := filepath.Join(dir, "build.yaml")
	require.NoError(t, os.WriteFile(buildErrPath, []byte(
		"yggdrasil:\n"+
			"  config:\n"+
			"    sources:\n"+
			"      - kind: custom\n",
	), 0o600))

	_, _, err := loader.LoadFile(manager, buildErrPath, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "config source[0]")
	require.Contains(t, err.Error(), "build failed")

	loadErrRegistry := NewRegistry()
	loadErrRegistry.Register(
		"custom",
		func(spec SourceSpec) (source.Source, config.Priority, error) {
			return &failingSource{err: errors.New("read failed")}, config.PriorityFile, nil
		},
	)
	loader = NewLoader(loadErrRegistry)

	loadErrPath := filepath.Join(dir, "load.yaml")
	require.NoError(t, os.WriteFile(loadErrPath, []byte(
		"yggdrasil:\n"+
			"  config:\n"+
			"    sources:\n"+
			"      - kind: custom\n",
	), 0o600))

	_, _, err = loader.LoadFile(config.NewManager(), loadErrPath, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "config source[0]")
	require.Contains(t, err.Error(), "read failed")
}
