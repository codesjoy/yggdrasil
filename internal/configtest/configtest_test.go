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

package configtest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/config"
)

func TestSet_SimpleKey(t *testing.T) {
	config.SetDefault(config.NewManager())
	Set(t, "yggdrasil.app.name", "test-app")
	// The key should be set in the config
}

func TestSet_NestedKey(t *testing.T) {
	config.SetDefault(config.NewManager())
	Set(t, "yggdrasil.server.network", "tcp")
	Set(t, "yggdrasil.server.address", "127.0.0.1:0")
}

func TestSet_BracedKey(t *testing.T) {
	config.SetDefault(config.NewManager())
	Set(t, "yggdrasil.clients.services.{my-service}.resolver", "default")
}

func TestPathSegments(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		parts := pathSegments("a.b.c")
		assert.Equal(t, []string{"a", "b", "c"}, parts)
	})
	t.Run("with braces", func(t *testing.T) {
		parts := pathSegments("a.{b.c}.d")
		assert.Equal(t, []string{"a", "b.c", "d"}, parts)
	})
	t.Run("empty panics", func(t *testing.T) {
		assert.Panics(t, func() { pathSegments("") })
	})
}

func TestApplySet(t *testing.T) {
	t.Run("simple key", func(t *testing.T) {
		dst := map[string]any{}
		applySet(dst, "key", "val")
		require.Equal(t, "val", dst["key"])
	})
	t.Run("nested key", func(t *testing.T) {
		dst := map[string]any{}
		applySet(dst, "a.b", "val")
		require.Equal(t, "val", dst["a"].(map[string]any)["b"])
	})
	t.Run("deep nested", func(t *testing.T) {
		dst := map[string]any{}
		applySet(dst, "a.b.c", "val")
		require.Equal(t, "val", dst["a"].(map[string]any)["b"].(map[string]any)["c"])
	})
}
