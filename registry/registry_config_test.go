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

package registry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type testRegistry struct{ kind string }

func (t *testRegistry) Register(context.Context, Instance) error   { return nil }
func (t *testRegistry) Deregister(context.Context, Instance) error { return nil }
func (t *testRegistry) Type() string                               { return t.kind }

func TestGetLoadsRegistryFromDiscoveryTree(t *testing.T) {
	RegisterBuilder("mock", func(cfg map[string]any) (Registry, error) {
		return &testRegistry{kind: cfg["name"].(string)}, nil
	})
	Configure(Spec{Type: "mock", Config: map[string]any{"name": "demo"}})

	reg, err := Get()
	require.NoError(t, err)
	require.Equal(t, "demo", reg.Type())
}

func TestNewMultiRegistry(t *testing.T) {
	RegisterBuilder("child", func(cfg map[string]any) (Registry, error) {
		return &testRegistry{kind: cfg["name"].(string)}, nil
	})

	reg, err := newMultiRegistry(map[string]any{
		"registries": []any{
			map[string]any{"type": "child", "config": map[string]any{"name": "a"}},
			map[string]any{"type": "child", "config": map[string]any{"name": "b"}},
		},
	})
	require.NoError(t, err)
	require.Equal(t, multiRegistryType, reg.Type())
}
