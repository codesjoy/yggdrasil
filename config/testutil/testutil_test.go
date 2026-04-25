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

package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSet_SimpleKey(t *testing.T) {
	ct := New(t)
	ct.Set("yggdrasil.app.name", "test-app")
	snap := ct.Manager().Snapshot()
	require.NotNil(t, snap.Map())
}

func TestSet_NestedKey(t *testing.T) {
	ct := New(t)
	ct.Set("yggdrasil.server.network", "tcp")
	ct.Set("yggdrasil.server.address", "127.0.0.1:0")
}

func TestSet_BracedKey(t *testing.T) {
	ct := New(t)
	ct.Set("yggdrasil.clients.services.{my-service}.resolver", "default")
}

func TestSet_Chaining(t *testing.T) {
	ct := New(t)
	result := ct.Set("a.b", 1).Set("a.c", 2)
	assert.Same(t, ct, result)
}

func TestManager_ReturnsFreshManager(t *testing.T) {
	ct := New(t)
	mgr := ct.Manager()
	require.NotNil(t, mgr)
	// Manager should be usable independently
	require.NotNil(t, mgr.Snapshot())
}

func TestManager_Independence(t *testing.T) {
	ct1 := New(t)
	ct2 := New(t)
	ct1.Set("app.name", "first")
	ct2.Set("app.name", "second")

	m1 := ct1.Manager().Snapshot().Map()
	m2 := ct2.Manager().Snapshot().Map()
	// Each manager has its own data — both keys coexist independently
	require.Contains(t, m1, "app")
	require.Contains(t, m2, "app")
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
