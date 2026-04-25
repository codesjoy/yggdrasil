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

package bootstrap

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/config/source"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/observability/logger"
	"github.com/codesjoy/yggdrasil/v3/transport/gateway/rest"
)

func TestParseNamedFlagArg(t *testing.T) {
	t.Run("long form --name=value", func(t *testing.T) {
		val, ok := ParseNamedFlagArg(
			[]string{"--yggdrasil-config=/path/to/config.yaml"},
			"yggdrasil-config",
		)
		assert.True(t, ok)
		assert.Equal(t, "/path/to/config.yaml", val)
	})

	t.Run("short form -name value", func(t *testing.T) {
		val, ok := ParseNamedFlagArg(
			[]string{"-yggdrasil-config", "/path/to/config.yaml"},
			"yggdrasil-config",
		)
		assert.True(t, ok)
		assert.Equal(t, "/path/to/config.yaml", val)
	})

	t.Run("long form without value", func(t *testing.T) {
		val, ok := ParseNamedFlagArg([]string{"--yggdrasil-config"}, "yggdrasil-config")
		assert.True(t, ok)
		assert.Equal(t, "", val)
	})

	t.Run("short form without value", func(t *testing.T) {
		val, ok := ParseNamedFlagArg([]string{"-yggdrasil-config"}, "yggdrasil-config")
		assert.True(t, ok)
		assert.Equal(t, "", val)
	})

	t.Run("short form with equals", func(t *testing.T) {
		val, ok := ParseNamedFlagArg([]string{"-yggdrasil-config=/path"}, "yggdrasil-config")
		assert.True(t, ok)
		assert.Equal(t, "/path", val)
	})

	t.Run("not found returns false", func(t *testing.T) {
		val, ok := ParseNamedFlagArg([]string{"--other-flag=value"}, "yggdrasil-config")
		assert.False(t, ok)
		assert.Equal(t, "", val)
	})
}

func TestCloseConfigSourcesReverse(t *testing.T) {
	t.Run("closes in reverse order", func(t *testing.T) {
		var order []string
		s1 := &trackingConfigSource{name: "s1", onClose: func() { order = append(order, "s1") }}
		s2 := &trackingConfigSource{name: "s2", onClose: func() { order = append(order, "s2") }}
		err := CloseConfigSourcesReverse([]source.Source{s1, s2})
		require.NoError(t, err)
		assert.Equal(t, []string{"s2", "s1"}, order)
	})

	t.Run("nil items skipped", func(t *testing.T) {
		err := CloseConfigSourcesReverse([]source.Source{nil, nil})
		require.NoError(t, err)
	})

	t.Run("close error joined", func(t *testing.T) {
		s1 := &errorConfigSource{name: "s1"}
		s2 := &errorConfigSource{name: "s2"}
		err := CloseConfigSourcesReverse([]source.Source{s1, s2})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "s1")
		assert.Contains(t, err.Error(), "s2")
	})

	t.Run("empty slice returns nil", func(t *testing.T) {
		err := CloseConfigSourcesReverse(nil)
		require.NoError(t, err)
	})
}

func TestNeedsDefaultStartupSettings(t *testing.T) {
	t.Run("all empty returns true", func(t *testing.T) {
		assert.True(t, NeedsDefaultStartupSettings(settings.Resolved{}))
	})

	t.Run("non-empty logging handlers returns false", func(t *testing.T) {
		resolved := settings.Resolved{}
		resolved.Logging.Handlers = map[string]logger.HandlerSpec{"default": {}}
		assert.False(t, NeedsDefaultStartupSettings(resolved))
	})

	t.Run("non-empty registry type returns false", func(t *testing.T) {
		resolved := settings.Resolved{}
		resolved.Discovery.Registry.Type = "multi_registry"
		assert.False(t, NeedsDefaultStartupSettings(resolved))
	})

	t.Run("non-empty transports returns false", func(t *testing.T) {
		resolved := settings.Resolved{}
		resolved.Server.Transports = []string{"grpc"}
		assert.False(t, NeedsDefaultStartupSettings(resolved))
	})

	t.Run("non-nil rest returns false", func(t *testing.T) {
		resolved := settings.Resolved{}
		resolved.Transports.Rest = &rest.Config{}
		assert.False(t, NeedsDefaultStartupSettings(resolved))
	})
}

func TestStartupValidatorAdd(t *testing.T) {
	t.Run("nil err is no-op", func(t *testing.T) {
		v := &startupValidator{strict: true}
		v.add("msg", nil)
		assert.Nil(t, v.err)
	})

	t.Run("strict mode accumulates errors", func(t *testing.T) {
		v := &startupValidator{strict: true}
		v.add("msg1", errors.New("err1"))
		v.add("msg2", errors.New("err2"))
		require.Error(t, v.err)
		assert.Contains(t, v.err.Error(), "msg1")
		assert.Contains(t, v.err.Error(), "msg2")
	})
}

type trackingConfigSource struct {
	name    string
	onClose func()
}

func (t *trackingConfigSource) Kind() string               { return "tracking" }
func (t *trackingConfigSource) Name() string               { return t.name }
func (t *trackingConfigSource) Read() (source.Data, error) { return nil, nil }
func (t *trackingConfigSource) Close() error               { t.onClose(); return nil }

type errorConfigSource struct {
	name string
}

func (e *errorConfigSource) Kind() string               { return "error" }
func (e *errorConfigSource) Name() string               { return e.name }
func (e *errorConfigSource) Read() (source.Data, error) { return nil, nil }
func (e *errorConfigSource) Close() error               { return errors.New("close failed") }
