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

// Package xarray provides array functionality.
package xarray

// DelDupStable removes duplicate elements from a slice while preserving the original order.
func DelDupStable[T comparable](slc []T) []T {
	if len(slc) < 2 {
		return slc
	}
	seen := make(map[T]bool)
	i := 0
	for _, value := range slc {
		if _, ok := seen[value]; !ok {
			seen[value] = true
			slc[i] = value
			i++
		}
	}

	return slc[:i]
}
