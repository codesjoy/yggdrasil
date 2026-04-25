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

	"github.com/codesjoy/yggdrasil/v3/config/source/memory"
)

func TestDefaultSetDefaultBindCurrent(t *testing.T) {
	prev := Default()
	t.Cleanup(func() { SetDefault(prev) })

	manager := NewManager()
	old := SetDefault(manager)
	require.Equal(t, prev, old)
	require.Equal(t, manager, Default())

	require.NoError(t, manager.LoadLayer("defaults", PriorityDefaults, memory.NewSource("defaults", map[string]any{
		"app": map[string]any{"port": 8088},
	})))
	section := Bind[struct {
		Port int `mapstructure:"port"`
	}](nil, "app")
	current, err := section.Current()
	require.NoError(t, err)
	require.Equal(t, 8088, current.Port)

	returned := SetDefault(nil)
	require.Equal(t, manager, returned)
	require.NotNil(t, Default())
	require.NotEqual(t, manager, Default())
}
