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

package yggdrasil

import (
	"flag"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/config/source"
)

type mockConfigSource struct {
	name       string
	data       map[string]any
	closeCount *int32
}

func (m *mockConfigSource) Kind() string { return "mock" }
func (m *mockConfigSource) Name() string { return m.name }
func (m *mockConfigSource) Read() (source.Data, error) {
	return source.NewMapData(m.data), nil
}
func (m *mockConfigSource) Close() error {
	if m.closeCount != nil {
		atomic.AddInt32(m.closeCount, 1)
	}
	return nil
}

func withTestFlagSet(t *testing.T) {
	t.Helper()
	oldCommandLine := flag.CommandLine
	oldArgs := os.Args
	flag.CommandLine = flag.NewFlagSet("yggdrasil-test", flag.ContinueOnError)
	os.Args = []string{"yggdrasil-test"}
	t.Cleanup(func() {
		flag.CommandLine = oldCommandLine
		os.Args = oldArgs
	})
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	oldDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		_ = os.Chdir(oldDir)
	})
}

func TestResolveBootstrapConfigPath(t *testing.T) {
	withTestFlagSet(t)

	path, explicit := resolveBootstrapConfigPath("")
	assert.Equal(t, defaultBootstrapConfigPath, path)
	assert.False(t, explicit)

	configured := filepath.Join(t.TempDir(), "custom.yaml")
	path, explicit = resolveBootstrapConfigPath(configured)
	assert.Equal(t, configured, path)
	assert.True(t, explicit)

	explicitPath := filepath.Join(t.TempDir(), "explicit.yaml")
	os.Args = []string{"yggdrasil-test", "--yggdrasil-config=" + explicitPath}
	path, explicit = resolveBootstrapConfigPath(configured)
	assert.Equal(t, explicitPath, path)
	assert.True(t, explicit)
}

func TestInitLoadsBootstrapAndOptionSources(t *testing.T) {
	resetLifecycleStateForTest(t)
	withTestFlagSet(t)
	manager := config.NewManager()
	config.SetDefault(manager)

	dir := t.TempDir()
	chdir(t, dir)
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "config.yaml"),
		[]byte("app:\n  startup_order: bootstrap\nyggdrasil:\n  bootstrap:\n    sources:\n      - kind: file\n        config:\n          path: ./config.override.yaml\n"),
		0o600,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "config.override.yaml"),
		[]byte("app:\n  startup_order: declared\n"),
		0o600,
	))

	optionSource := &mockConfigSource{
		name: "option",
		data: map[string]any{"app": map[string]any{"startup_order": "option"}},
	}

	require.NoError(t, Init("bootstrap-order", WithConfigManager(manager), WithBootstrapSource("option", config.PriorityOverride, optionSource)))

	var out struct {
		App struct {
			StartupOrder string `mapstructure:"startup_order"`
		} `mapstructure:"app"`
	}
	require.NoError(t, manager.Section("app").Decode(&out.App))
	assert.Equal(t, "option", out.App.StartupOrder)
}

func TestStopClosesManagedConfigSourcesOnce(t *testing.T) {
	resetLifecycleStateForTest(t)
	withTestFlagSet(t)
	chdir(t, t.TempDir())

	var closeCount int32
	programmatic := &mockConfigSource{
		name:       "to-close",
		data:       map[string]any{},
		closeCount: &closeCount,
	}

	require.NoError(t, Init("close-sources", WithBootstrapSource("programmatic", config.PriorityOverride, programmatic)))
	require.NoError(t, Stop())
	assert.Equal(t, int32(1), atomic.LoadInt32(&closeCount))
}
