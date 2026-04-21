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

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLookupAndSetPath(t *testing.T) {
	root := map[string]any{
		"app": map[string]any{
			"name": "demo",
		},
	}

	all := Lookup(root).(map[string]any)
	all["app"].(map[string]any)["name"] = "changed"
	require.Equal(t, "demo", root["app"].(map[string]any)["name"])

	require.Nil(t, Lookup(root, "missing"))
	require.Nil(t, Lookup(map[string]any{"app": "value"}, "app", "name"))

	SetPath(root, 9090, "app", "server", "port")
	require.Equal(t, 9090, root["app"].(map[string]any)["server"].(map[string]any)["port"])

	root["app"] = "not-map"
	SetPath(root, "replaced", "app", "name")
	require.Equal(t, "replaced", root["app"].(map[string]any)["name"])

	before := root["app"]
	SetPath(root, "noop")
	require.Equal(t, before, root["app"])
}
