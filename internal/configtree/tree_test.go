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

package configtree

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

var benchmarkNormalizeSink any

type benchmarkConfigNode struct {
	Values []string
	Labels map[string]string
	Child  *benchmarkConfigNode
}

func TestNormalizeValueDeepCloneAndMapAnyAny(t *testing.T) {
	src := map[any]any{
		"nested": map[string]any{
			"list": []any{map[any]any{1: "one"}},
		},
	}

	out, ok := NormalizeValue(src).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "one", out["nested"].(map[string]any)["list"].([]any)[0].(map[string]any)["1"])

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

func BenchmarkNormalizeValue(b *testing.B) {
	nestedMap := make(map[string]any, 100)
	for index := 0; index < 100; index++ {
		nestedMap[strconv.Itoa(index)] = []any{index, map[string]any{"enabled": true}}
	}
	typed := benchmarkConfigNode{
		Values: []string{"one", "two", "three"},
		Labels: map[string]string{"env": "benchmark", "region": "local"},
		Child: &benchmarkConfigNode{
			Values: []string{"child"},
			Labels: map[string]string{"kind": "nested"},
		},
	}
	tests := []struct {
		name  string
		value any
	}{
		{name: "scalar", value: 42},
		{name: "nested-map", value: nestedMap},
		{name: "typed-struct", value: typed},
	}

	for _, test := range tests {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				benchmarkNormalizeSink = NormalizeValue(test.value)
			}
		})
	}
}
