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

package assembly

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiff_NoChanges(t *testing.T) {
	spec := &Spec{
		Identity: IdentitySpec{AppName: "test"},
		Mode:     Mode{Name: "dev", Profile: "dev", Bundle: "server-basic"},
		Defaults: map[string]string{
			capLoggerHandler: "text",
		},
	}
	diff, err := Diff(spec, spec)
	require.NoError(t, err)
	require.False(t, diff.HasChanges)
}

func TestDiff_ModuleChanges(t *testing.T) {
	old := &Spec{
		Identity: IdentitySpec{AppName: "test"},
		Modules: []ModuleRef{
			{Name: "mod.a", Kind: "module", Source: "user"},
		},
	}
	newSpec := &Spec{
		Identity: IdentitySpec{AppName: "test"},
		Modules: []ModuleRef{
			{Name: "mod.b", Kind: "module", Source: "user"},
		},
	}
	diff, err := Diff(old, newSpec)
	require.NoError(t, err)
	require.True(t, diff.HasChanges)
	require.Len(t, diff.Modules, 2) // one removed, one added

	removed := false
	added := false
	for _, m := range diff.Modules {
		if m.Name == "mod.a" && m.Action == "removed" {
			removed = true
		}
		if m.Name == "mod.b" && m.Action == "added" {
			added = true
		}
	}
	require.True(t, removed)
	require.True(t, added)
	require.Contains(t, diff.AffectedDomains, "modules")
}

func TestDiff_ModeChange(t *testing.T) {
	old := &Spec{
		Identity: IdentitySpec{AppName: "test"},
		Mode:     Mode{Name: "dev", Profile: "dev"},
	}
	newSpec := &Spec{
		Identity: IdentitySpec{AppName: "test"},
		Mode:     Mode{Name: "prod-grpc", Profile: "prod"},
	}
	diff, err := Diff(old, newSpec)
	require.NoError(t, err)
	require.True(t, diff.HasChanges)
	require.NotNil(t, diff.Mode)
	require.Equal(t, "dev", diff.Mode.Old)
	require.Equal(t, "prod-grpc", diff.Mode.New)
	require.Contains(t, diff.AffectedDomains, "mode")
}

func TestHash_NilSpec(t *testing.T) {
	hash, err := Hash(nil)
	require.NoError(t, err)
	require.NotEmpty(t, hash)
}

func TestExplain_NilSpec(t *testing.T) {
	data, err := Explain(nil)
	require.NoError(t, err)
	require.Contains(t, string(data), `"identity"`)
}
