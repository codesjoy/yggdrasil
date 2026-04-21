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

package memory

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMemorySourceReadAndMetadata(t *testing.T) {
	src := NewSource("mem", map[string]any{
		"app": map[string]any{
			"name": "demo",
			"list": []any{map[string]any{"k": "v"}},
		},
	})

	require.Equal(t, "memory", src.Kind())
	require.Equal(t, "mem", src.Name())

	data, err := src.Read()
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, data.Unmarshal(&out))
	require.Equal(t, "demo", out["app"].(map[string]any)["name"])

	// bytes/unmarshal output should be independent from source internal data.
	out["app"].(map[string]any)["name"] = "changed"
	data2, err := src.Read()
	require.NoError(t, err)
	var out2 map[string]any
	require.NoError(t, data2.Unmarshal(&out2))
	require.Equal(t, "demo", out2["app"].(map[string]any)["name"])

	require.NoError(t, src.Close())
}
