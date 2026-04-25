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

package config

import "github.com/codesjoy/yggdrasil/v3/config/internal/tree"

// Lookup returns a deep-cloned value from the given path.
func Lookup(value any, path ...string) any {
	if len(path) == 0 {
		return tree.NormalizeValue(value)
	}
	current := value
	for i, segment := range path {
		nextMap, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		next, ok := nextMap[segment]
		if !ok {
			return nil
		}
		if i == len(path)-1 {
			return tree.NormalizeValue(next)
		}
		current = next
	}
	return nil
}

// SetPath writes a normalized value into the provided map using path segments.
func SetPath(dst map[string]any, value any, path ...string) {
	if len(path) == 0 {
		return
	}
	current := dst
	for _, segment := range path[:len(path)-1] {
		next, ok := current[segment].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[segment] = next
		}
		current = next
	}
	current[path[len(path)-1]] = tree.NormalizeValue(value)
}
