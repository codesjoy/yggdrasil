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

package internal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeValueDeepCloneAndMapAnyAny(t *testing.T) {
	src := map[any]any{
		"nested": map[string]any{
			"list": []any{map[any]any{1: "one"}},
		},
	}

	out, ok := NormalizeValue(src).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "one", out["nested"].(map[string]any)["list"].([]any)[0].(map[string]any)["1"])

	// mutate result and ensure original structure is unaffected
	out["nested"].(map[string]any)["list"].([]any)[0].(map[string]any)["1"] = "changed"
	require.Equal(t, "one", src["nested"].(map[string]any)["list"].([]any)[0].(map[any]any)[1])
}

func TestNormalizeMapNilReturnsEmpty(t *testing.T) {
	out := NormalizeMap(nil)
	require.NotNil(t, out)
	require.Len(t, out, 0)
}

func TestMergeMapsMergesAndReplacesOnTypeMismatch(t *testing.T) {
	dst := map[string]any{
		"a": map[string]any{"x": 1, "keep": true},
		"b": 1,
	}
	src := map[string]any{
		"a": map[string]any{"x": 2, "y": 3},
		"b": map[string]any{"nested": "v"},
	}

	merged := MergeMaps(dst, src)
	require.Equal(t, map[string]any{
		"a": map[string]any{"x": 2, "y": 3, "keep": true},
		"b": map[string]any{"nested": "v"},
	}, merged)
}

func TestHasPrefix(t *testing.T) {
	require.True(t, HasPrefix("app_server_port", "app", "_"))
	require.True(t, HasPrefix("app", "app", "_"))
	require.False(t, HasPrefix("apple", "app", "_"))
	require.False(t, HasPrefix("service_app_port", "app", "_"))
}
