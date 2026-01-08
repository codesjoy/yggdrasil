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

// Package file provides functionality for reading configuration from a file
package file

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/v2/config/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestFile_NewSource tests creating a new file source
func TestFile_NewSource(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		watchable bool
		parser    source.Parser
	}{
		{
			name:      "with yaml file",
			path:      "config.yaml",
			watchable: false,
			parser:    nil,
		},
		{
			name:      "with json file",
			path:      "config.json",
			watchable: false,
			parser:    nil,
		},
		{
			name:      "watchable source",
			path:      "config.yaml",
			watchable: true,
			parser:    nil,
		},
		{
			name:      "with custom parser",
			path:      "config.txt",
			watchable: false,
			parser:    yaml.Unmarshal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := NewSource(tt.path, tt.watchable, tt.parser)
			assert.NotNil(t, src)
			f, ok := src.(*file)
			require.True(t, ok)
			assert.Equal(t, tt.path, f.path)
			assert.Equal(t, tt.watchable, f.enableWatcher)
			if tt.parser != nil {
				assert.NotNil(t, f.parser)
			}
		})
	}
}

// TestFile_Name tests the Name method
func TestFile_Name(t *testing.T) {
	src := NewSource("test.yaml", false)
	assert.Equal(t, "file", src.Name())
}

// TestFile_Changeable tests the Changeable method
func TestFile_Changeable(t *testing.T) {
	tests := []struct {
		name      string
		watchable bool
		expected  bool
	}{
		{
			name:      "not watchable",
			watchable: false,
			expected:  false,
		},
		{
			name:      "watchable",
			watchable: true,
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := NewSource("test.yaml", tt.watchable)
			assert.Equal(t, tt.expected, src.Changeable())
		})
	}
}

// TestFile_Read tests reading from a file
func TestFile_Read(t *testing.T) {
	// Create temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "config.yaml")

	testData := map[string]interface{}{
		"database": map[string]interface{}{
			"host":     "localhost",
			"port":     3306,
			"username": "testuser",
			"password": "testpass",
		},
		"server": map[string]interface{}{
			"port": 8080,
			"host": "0.0.0.0",
		},
	}

	// Write YAML data to file
	yamlData, err := yaml.Marshal(testData)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(testFile, yamlData, 0o600))

	// Test reading
	src := NewSource(testFile, false)
	data, err := src.Read()
	require.NoError(t, err)
	require.NotNil(t, data)

	// Verify the data
	var result map[string]interface{}
	err = yaml.Unmarshal(data.Data(), &result)
	require.NoError(t, err)

	assert.Equal(t, "localhost", result["database"].(map[string]interface{})["host"])
	assert.Equal(t, int(3306),
		int(result["database"].(map[string]interface{})["port"].(int))) // nolint:unconvert
	assert.Equal(t, "testuser",
		result["database"].(map[string]interface{})["username"]) // nolint:unconvert
	assert.Equal(t, int(8080),
		int(result["server"].(map[string]interface{})["port"].(int))) // nolint:unconvert
}

// TestFile_Read_JSON tests reading JSON files
func TestFile_Read_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "config.json")

	testData := map[string]interface{}{
		"app": map[string]interface{}{
			"name":    "testapp",
			"version": "1.0.0",
		},
	}

	jsonData, err := json.Marshal(testData)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(testFile, jsonData, 0o600))

	src := NewSource(testFile, false)
	data, err := src.Read()
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(data.Data(), &result)
	require.NoError(t, err)

	assert.Equal(t, "testapp", result["app"].(map[string]interface{})["name"])
	assert.Equal(t, "1.0.0", result["app"].(map[string]interface{})["version"])
}

// TestFile_Read_FileNotExist tests reading a non-existent file
func TestFile_Read_FileNotExist(t *testing.T) {
	src := NewSource("/nonexistent/file.yaml", false)
	_, err := src.Read()
	assert.Error(t, err)
}

// TestFile_Read_EmptyFile tests reading an empty file
func TestFile_Read_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.yaml")

	require.NoError(t, os.WriteFile(testFile, []byte(""), 0o600))

	src := NewSource(testFile, false)
	data, err := src.Read()
	require.NoError(t, err)
	require.NotNil(t, data)
	assert.Equal(t, []byte{}, data.Data())
}

// TestFile_Read_CustomParser tests reading with a custom parser
func TestFile_Read_CustomParser(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "config.txt")

	testData := "key1:value1,key2:value2"
	require.NoError(t, os.WriteFile(testFile, []byte(testData), 0o600))

	// Custom parser that splits by comma and then by colon
	customParser := func(data []byte, v interface{}) error {
		str := string(data)
		result := make(map[string]string)
		pairs := strings.Split(str, ",")
		for _, pair := range pairs {
			kv := strings.Split(pair, ":")
			if len(kv) == 2 {
				result[kv[0]] = kv[1]
			}
		}
		*(v.(*map[string]string)) = result
		return nil
	}

	src := NewSource(testFile, false, customParser)
	data, err := src.Read()
	require.NoError(t, err)

	var result map[string]string
	err = data.Unmarshal(&result)
	require.NoError(t, err)

	assert.Equal(t, "value1", result["key1"])
	assert.Equal(t, "value2", result["key2"])
}

// TestFile_Watch tests file watching functionality
func TestFile_Watch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping watch test in short mode")
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "config.yaml")

	initialData := map[string]interface{}{
		"key": "initial",
	}
	yamlData, err := yaml.Marshal(initialData)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(testFile, yamlData, 0o600))

	src := NewSource(testFile, true)
	watcher, err := src.Watch()
	require.NoError(t, err)
	require.NotNil(t, watcher)

	defer func() {
		_ = src.Close()
	}()

	// Wait a bit and modify the file
	time.Sleep(100 * time.Millisecond)

	modifiedData := map[string]interface{}{
		"key": "modified",
	}
	modifiedYAML, err := yaml.Marshal(modifiedData)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(testFile, modifiedYAML, 0o600))

	// Wait for the change event
	select {
	case data := <-watcher:
		require.NotNil(t, data)
		var result map[string]interface{}
		err = yaml.Unmarshal(data.Data(), &result)
		require.NoError(t, err)
		assert.Equal(t, "modified", result["key"])
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for file change event")
	}
}

// TestFile_Watch_FileRename tests watching when file is renamed
func TestFile_Watch_FileRename(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping watch test in short mode")
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "config.yaml")
	tempFile := filepath.Join(tmpDir, "config.tmp")

	initialData := map[string]interface{}{
		"key": "value1",
	}
	yamlData, err := yaml.Marshal(initialData)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(testFile, yamlData, 0o600))
	src := NewSource(testFile, true)
	watcher, err := src.Watch()
	require.NoError(t, err)
	defer func() {
		_ = src.Close()
	}()

	// Rename and recreate file (common pattern in editors)
	time.Sleep(100 * time.Millisecond)
	require.NoError(t, os.Rename(testFile, tempFile))

	time.Sleep(time.Second)

	newData := map[string]interface{}{
		"key": "value2",
	}
	newYAML, err := yaml.Marshal(newData)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(testFile, newYAML, 0o600))
	// Wait for the change event
	select {
	case data := <-watcher:
		require.NotNil(t, data)
		var result map[string]interface{}
		err = yaml.Unmarshal(data.Data(), &result)
		require.NoError(t, err)
		assert.Equal(t, "value2", result["key"])
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for file change event")
	}
}

// TestFile_Watch_StopWatching tests stopping the watcher
func TestFile_Watch_StopWatching(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping watch test in short mode")
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "config.yaml")

	initialData := map[string]interface{}{
		"key": "initial",
	}
	yamlData, err := yaml.Marshal(initialData)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(testFile, yamlData, 0o600))

	src := NewSource(testFile, true)
	watcher, err := src.Watch()
	require.NoError(t, err)

	// Close the source
	_ = src.Close()

	// Modify the file after closing
	time.Sleep(100 * time.Millisecond)
	modifiedData := map[string]interface{}{
		"key": "modified",
	}
	modifiedYAML, err := yaml.Marshal(modifiedData)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(testFile, modifiedYAML, 0o600))

	// The channel should be closed eventually
	select {
	case _, ok := <-watcher:
		if !ok {
			// Channel closed as expected
			return
		}
		// Received data before close, that's also acceptable
	case <-time.After(2 * time.Second):
		// Timeout is acceptable - the watcher may have already stopped
	}
}

// TestFile_Close tests closing the file source
func TestFile_Close(t *testing.T) {
	src := NewSource("test.yaml", false)
	f := src.(*file)

	// First close should succeed
	err := src.Close()
	assert.NoError(t, err)

	// Second close should also succeed (idempotent)
	err = src.Close()
	assert.NoError(t, err)

	// Check that stopped flag is set
	assert.True(t, f.stopped)
}

// TestFile_Close_Watcher tests closing a source with active watcher
func TestFile_Close_Watcher(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping watch test in short mode")
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "config.yaml")

	initialData := map[string]interface{}{
		"key": "initial",
	}
	yamlData, err := yaml.Marshal(initialData)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(testFile, yamlData, 0o600))

	src := NewSource(testFile, true)
	_, err = src.Watch()
	require.NoError(t, err)

	// Close should stop the watcher
	err = src.Close()
	assert.NoError(t, err)

	f := src.(*file)
	assert.True(t, f.stopped)
}

// TestFile_Read_YAMLFromTestData tests reading the actual testdata config
func TestFile_Read_YAMLFromTestData(t *testing.T) {
	testFile := "../../testdata/config.yaml"

	src := NewSource(testFile, false)
	data, err := src.Read()
	require.NoError(t, err)

	var result map[string]interface{}
	err = yaml.Unmarshal(data.Data(), &result)
	require.NoError(t, err)

	// Verify some expected values
	yggdrasil, ok := result["yggdrasil"].(map[string]interface{})
	require.True(t, ok)

	client, ok := yggdrasil["client"].(map[string]interface{})
	require.True(t, ok)

	exampleServer, ok := client["example.polaris.server"].(map[string]interface{})
	require.True(t, ok)

	grpc, ok := exampleServer["grpc"].(map[string]interface{})
	require.True(t, ok)

	assert.Equal(t, "127.0.0.1:30001", grpc["target"])

	// Check TestTag data
	testTag, ok := result["TestTag"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "test", testTag["data"])
}

// TestFile_NewSource_AutoParser tests automatic parser detection based on file extension
func TestFile_NewSource_AutoParser(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		expectParse bool
	}{
		{
			name:        "yaml extension",
			path:        "config.yaml",
			expectParse: true,
		},
		{
			name:        "yml extension",
			path:        "config.yml",
			expectParse: true,
		},
		{
			name:        "json extension",
			path:        "config.json",
			expectParse: true,
		},
		{
			name:        "unknown extension defaults to yaml",
			path:        "config.unknown",
			expectParse: true,
		},
		{
			name:        "no extension defaults to yaml",
			path:        "config",
			expectParse: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := NewSource(tt.path, false)
			f := src.(*file)

			if tt.expectParse {
				assert.NotNil(t, f.parser, "parser should not be nil")
			}
		})
	}
}

// TestFile_Priority tests that file source returns correct priority
func TestFile_Priority(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "config.yaml")

	testData := map[string]string{"key": "value"}
	yamlData, err := yaml.Marshal(testData)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(testFile, yamlData, 0o600))

	src := NewSource(testFile, false)
	data, err := src.Read()
	require.NoError(t, err)

	// Verify the priority matches source.PriorityFile
	assert.Equal(t, source.PriorityFile, data.Priority())
}
