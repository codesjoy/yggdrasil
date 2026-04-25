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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source/memory"
	"github.com/codesjoy/yggdrasil/v3/internal/instance"
)

func TestInstanceFunctions(t *testing.T) {
	instance.InitInstanceInfo("test-name", instance.Config{
		Namespace: "test-ns",
		Version:   "1.0.0",
		Region:    "test-region",
		Zone:      "test-zone",
		Campus:    "test-campus",
		Metadata:  map[string]string{"key": "value"},
	})

	assert.Equal(t, "test-ns", InstanceNamespace())
	assert.Equal(t, "test-name", InstanceName())
	assert.Equal(t, "1.0.0", InstanceVersion())
	assert.Equal(t, "test-region", InstanceRegion())
	assert.Equal(t, "test-zone", InstanceZone())
	assert.Equal(t, "test-campus", InstanceCampus())
	assert.Equal(t, map[string]string{"key": "value"}, InstanceMetadata())
}

func TestVersionConstants(t *testing.T) {
	assert.NotEmpty(t, Version)
	assert.Equal(t, "yggdrasil", Name)
}

func TestAppNilReceiver(t *testing.T) {
	var a *App
	ctx := context.Background()

	err := a.ComposeAndInstall(ctx, nil)
	assert.Equal(t, "app is not initialized", err.Error())

	err = a.Start(ctx)
	assert.Equal(t, "app is not initialized", err.Error())

	err = a.Wait()
	assert.Equal(t, "app is not initialized", err.Error())

	err = a.Stop(ctx)
	assert.Equal(t, "app is not initialized", err.Error())
}

func TestRunWithNilContext(t *testing.T) {
	//nolint:staticcheck // intentional: testing nil context validation
	err := Run(nil, nil)
	require.Equal(t, "run context is nil", err.Error())
}

func TestWithOptions(t *testing.T) {
	t.Run("WithAppName", func(t *testing.T) {
		opts := options{}
		err := WithAppName("test-app")(&opts)
		require.NoError(t, err)
		assert.Equal(t, "test-app", opts.appName)
	})

	t.Run("WithConfigPath", func(t *testing.T) {
		opts := options{}
		err := WithConfigPath("/etc/config.yaml")(&opts)
		require.NoError(t, err)
		assert.Equal(t, "/etc/config.yaml", opts.configPath)
	})

	t.Run("WithMode", func(t *testing.T) {
		opts := options{}
		err := WithMode("debug")(&opts)
		require.NoError(t, err)
		assert.Equal(t, "debug", opts.mode)
	})

	t.Run("WithConfigSource nil source is skipped", func(t *testing.T) {
		opts := options{}
		err := WithConfigSource("test", config.PriorityOverride, nil)(&opts)
		require.NoError(t, err)
		assert.Empty(t, opts.configSources)
	})

	t.Run("WithConfigSource non-nil source is appended", func(t *testing.T) {
		opts := options{}
		src := memory.NewSource("test", map[string]any{"k": "v"})
		err := WithConfigSource("test", config.PriorityOverride, src)(&opts)
		require.NoError(t, err)
		require.Len(t, opts.configSources, 1)
		assert.Equal(t, "test", opts.configSources[0].name)
	})

	t.Run("nil Option is skipped in convertOptions", func(t *testing.T) {
		appOpts, err := convertOptions(nil, WithAppName("test"))
		require.NoError(t, err)
		assert.NotEmpty(t, appOpts)
	})

	t.Run("option returning error propagates", func(t *testing.T) {
		badOpt := func(opts *options) error {
			return assert.AnError
		}
		_, err := convertOptions(badOpt)
		require.ErrorIs(t, err, assert.AnError)
	})
}
