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
	"bytes"
	"flag"
	"fmt"
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
	priority   source.Priority
	data       map[string]any
	closeCount *int32
}

func (m *mockConfigSource) Type() string { return "mock" }

func (m *mockConfigSource) Name() string {
	if m.name == "" {
		return "mock"
	}
	return m.name
}

func (m *mockConfigSource) Read() (source.Data, error) {
	return source.NewMapSourceData(m.priority, m.data), nil
}

func (m *mockConfigSource) Changeable() bool { return false }

func (m *mockConfigSource) Watch() (<-chan source.Data, error) { return nil, nil }

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

func TestResolveBootstrapConfigPath_UsesExistingFlagDefault(t *testing.T) {
	withTestFlagSet(t)
	defaultPath := filepath.Join(t.TempDir(), "custom-config.yaml")
	_ = flag.CommandLine.String("yggdrasil-config", defaultPath, "config path")

	path, explicit := resolveBootstrapConfigPath()
	assert.Equal(t, defaultPath, path)
	assert.False(t, explicit)
}

func TestResolveBootstrapConfigPath_ExplicitArgWins(t *testing.T) {
	withTestFlagSet(t)
	defaultPath := filepath.Join(t.TempDir(), "custom-config.yaml")
	_ = flag.CommandLine.String("yggdrasil-config", defaultPath, "config path")

	explicitPath := filepath.Join(t.TempDir(), "explicit.yaml")
	os.Args = []string{"yggdrasil-test", "--yggdrasil-config=" + explicitPath}
	path, explicit := resolveBootstrapConfigPath()
	assert.Equal(t, explicitPath, path)
	assert.True(t, explicit)
}

func TestResolveBootstrapConfigPath_LegacyArgIgnored(t *testing.T) {
	withTestFlagSet(t)

	os.Args = []string{"yggdrasil-test", "--config=/tmp/legacy.yaml"}
	path, explicit := resolveBootstrapConfigPath()

	assert.Equal(t, defaultBootstrapConfigPath, path)
	assert.False(t, explicit)
}

func TestInstallBootstrapUsageHint(t *testing.T) {
	withTestFlagSet(t)
	var out bytes.Buffer
	flag.CommandLine.SetOutput(&out)
	flag.CommandLine.Usage = func() {
		_, _ = fmt.Fprintln(flag.CommandLine.Output(), "Usage of yggdrasil-test:")
	}

	flag.CommandLine.Usage()
	body := out.String()
	assert.Contains(t, body, "Usage of yggdrasil-test:")
}

func TestInit_DefaultBootstrapMissingFallback(t *testing.T) {
	resetLifecycleStateForTest(t)
	withTestFlagSet(t)
	chdir(t, t.TempDir())

	err := Init("bootstrap-missing-fallback")
	require.NoError(t, err)
}

func TestInit_ExplicitBootstrapMissingFails(t *testing.T) {
	resetLifecycleStateForTest(t)
	withTestFlagSet(t)
	chdir(t, t.TempDir())

	missingPath := filepath.Join(t.TempDir(), "not-exists.yaml")
	os.Args = []string{"yggdrasil-test", "--yggdrasil-config=" + missingPath}
	err := Init("bootstrap-missing-explicit")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bootstrap config file")
}

func TestInit_LoadOrderBootstrapDeclaredAndOptionSource(t *testing.T) {
	resetLifecycleStateForTest(t)
	withTestFlagSet(t)
	dir := t.TempDir()
	chdir(t, dir)

	bootstrapPath := filepath.Join(dir, "config.yaml")
	overridePath := filepath.Join(dir, "config.override.yaml")
	require.NoError(t, os.WriteFile(
		overridePath,
		[]byte("app:\n  startup_order: yaml-source\n"),
		0o600,
	))
	require.NoError(t, os.WriteFile(
		bootstrapPath,
		[]byte(
			"app:\n  startup_order: bootstrap\n"+
				"yggdrasil:\n"+
				"  config:\n"+
				"    sources:\n"+
				"      - type: file\n"+
				"        config:\n"+
				"          path: ./config.override.yaml\n"+
				"          watch: false\n",
		),
		0o600,
	))

	programmatic := &mockConfigSource{
		name:     "programmatic",
		priority: source.PriorityFlag,
		data: map[string]any{
			"app": map[string]any{
				"startup_order": "option-source",
			},
		},
	}
	err := Init("bootstrap-order", WithConfigSource(programmatic))
	require.NoError(t, err)
	assert.Equal(t, "option-source", config.GetString("app.startup_order"))
}

func TestInit_BootstrapInterpolatesContentAndDeclaredFilePath(t *testing.T) {
	resetLifecycleStateForTest(t)
	withTestFlagSet(t)
	dir := t.TempDir()
	chdir(t, dir)

	overridePath := filepath.Join(dir, "config.override.yaml")
	require.NoError(
		t,
		os.WriteFile(overridePath, []byte("app:\n  override_value: from-override\n"), 0o600),
	)

	t.Setenv("TEST_BOOTSTRAP_VALUE", "from-bootstrap")
	t.Setenv("TEST_BOOTSTRAP_OVERRIDE_PATH", overridePath)

	bootstrapPath := filepath.Join(dir, "config.yaml")
	require.NoError(
		t,
		os.WriteFile(
			bootstrapPath,
			[]byte(
				"app:\n"+
					"  bootstrap_value: ${TEST_BOOTSTRAP_VALUE}\n"+
					"yggdrasil:\n"+
					"  config:\n"+
					"    sources:\n"+
					"      - type: file\n"+
					"        config:\n"+
					"          path: ${TEST_BOOTSTRAP_OVERRIDE_PATH}\n"+
					"          watch: false\n",
			),
			0o600,
		),
	)

	err := Init("bootstrap-interpolate")
	require.NoError(t, err)
	assert.Equal(t, "from-bootstrap", config.GetString("app.bootstrap_value"))
	assert.Equal(t, "from-override", config.GetString("app.override_value"))
}

func TestInit_BootstrapMissingEnvPlaceholderFails(t *testing.T) {
	resetLifecycleStateForTest(t)
	withTestFlagSet(t)
	dir := t.TempDir()
	chdir(t, dir)

	bootstrapPath := filepath.Join(dir, "config.yaml")
	require.NoError(
		t,
		os.WriteFile(
			bootstrapPath,
			[]byte("app:\n  bootstrap_value: ${TEST_BOOTSTRAP_MISSING}\n"),
			0o600,
		),
	)
	os.Args = []string{"yggdrasil-test", "--yggdrasil-config=" + bootstrapPath}

	err := Init("bootstrap-missing-env")
	require.Error(t, err)
	assert.Contains(t, err.Error(), bootstrapPath)
	assert.Contains(t, err.Error(), "TEST_BOOTSTRAP_MISSING")
}

func TestDropServeStageConfigSources(t *testing.T) {
	source1 := &mockConfigSource{name: "s1", priority: source.PriorityMemory}
	source2 := &mockConfigSource{name: "s2", priority: source.PriorityMemory}
	opt := &options{
		configSources:         []source.Source{source1, source2},
		initConfigSourceCount: 1,
	}

	dropServeStageConfigSources(opt)
	require.Len(t, opt.configSources, 1)
	assert.Equal(t, source1, opt.configSources[0])
}

func TestStopClosesManagedConfigSourcesOnce(t *testing.T) {
	resetLifecycleStateForTest(t)
	withTestFlagSet(t)
	chdir(t, t.TempDir())

	var closeCount int32
	programmatic := &mockConfigSource{
		name:       "to-close",
		priority:   source.PriorityFlag,
		data:       map[string]any{},
		closeCount: &closeCount,
	}

	require.NoError(t, Init("close-sources", WithConfigSource(programmatic)))
	require.NoError(t, Stop())
	assert.Equal(t, int32(1), atomic.LoadInt32(&closeCount))
}
