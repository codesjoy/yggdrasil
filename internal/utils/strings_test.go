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

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDedupStableStrings(t *testing.T) {
	tests := []struct {
		name   string
		in     []string
		out    []string
		nilOut bool
	}{
		{
			name:   "nil",
			in:     nil,
			nilOut: true,
		},
		{
			name: "empty",
			in:   []string{},
			out:  []string{},
		},
		{
			name: "single",
			in:   []string{"a"},
			out:  []string{"a"},
		},
		{
			name: "dedup keeps order",
			in:   []string{"a", "b", "a", "c", "b", "d"},
			out:  []string{"a", "b", "c", "d"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var values []string
			if tt.in != nil {
				values = make([]string, len(tt.in))
				copy(values, tt.in)
			}
			got := DedupStableStrings(values)
			if tt.nilOut {
				assert.Nil(t, got)
				return
			}
			assert.Equal(t, tt.out, got)
		})
	}
}
