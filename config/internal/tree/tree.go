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

// Package tree provides internal config tree normalization helpers.
package tree

import "fmt"

// NormalizeValue recursively normalizes nested map types and deep-clones structured values.
func NormalizeValue(v any) any {
	switch item := v.(type) {
	case map[string]any:
		return NormalizeMap(item)
	case map[any]any:
		normalized := make(map[string]any, len(item))
		for k, val := range item {
			normalized[fmt.Sprintf("%v", k)] = NormalizeValue(val)
		}
		return normalized
	case []any:
		out := make([]any, len(item))
		for i := range item {
			out[i] = NormalizeValue(item[i])
		}
		return out
	default:
		return v
	}
}

// NormalizeMap recursively normalizes nested values and returns a deep clone.
func NormalizeMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = NormalizeValue(v)
	}
	return out
}

// MergeMaps recursively overlays src onto dst. Source values replace destination
// values when the types differ.
func MergeMaps(dst, src map[string]any) map[string]any {
	if dst == nil {
		dst = map[string]any{}
	}
	for k, srcVal := range src {
		srcMap, srcIsMap := srcVal.(map[string]any)
		dstMap, dstIsMap := dst[k].(map[string]any)
		if srcIsMap && dstIsMap {
			dst[k] = MergeMaps(dstMap, srcMap)
			continue
		}
		dst[k] = NormalizeValue(srcVal)
	}
	return dst
}
