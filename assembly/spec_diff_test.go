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

func TestDiffCoversCanonicalHashFields(t *testing.T) {
	old := &Spec{
		Identity: IdentitySpec{
			AppName:  "old",
			Metadata: map[string]string{"team": "a"},
		},
		Decisions: []Decision{{
			Kind:   "default",
			Target: "logger",
			Value:  "text",
			Source: "config",
			Reason: "old",
			Stage:  "plan",
		}},
		Warnings:  []Warning{{Code: "old", Message: "old warning"}},
		Conflicts: []Conflict{{Code: "old", Message: "old conflict"}},
	}
	newSpec := &Spec{
		Identity: IdentitySpec{
			AppName:  "new",
			Metadata: map[string]string{"team": "b"},
		},
		Decisions: []Decision{{
			Kind:   "default",
			Target: "logger",
			Value:  "json",
			Source: "config",
			Reason: "new",
			Stage:  "plan",
		}},
		Warnings:  []Warning{{Code: "new", Message: "new warning"}},
		Conflicts: []Conflict{{Code: "new", Message: "new conflict"}},
	}

	diff, err := Diff(old, newSpec)
	require.NoError(t, err)
	require.True(t, diff.HasChanges)
	require.NotEmpty(t, diff.Identity)
	require.NotEmpty(t, diff.Decisions)
	require.NotEmpty(t, diff.Warnings)
	require.NotEmpty(t, diff.Conflicts)
	require.Contains(t, diff.AffectedDomains, "identity")
	require.Contains(t, diff.AffectedDomains, "decisions")
	require.Contains(t, diff.AffectedDomains, "warnings")
	require.Contains(t, diff.AffectedDomains, "conflicts")
}

func TestDiffDetectsDuplicateCanonicalEntries(t *testing.T) {
	old := &Spec{
		Decisions: []Decision{{
			Kind:   "default",
			Target: "logger",
			Value:  "text",
			Source: "config",
			Reason: "first",
			Stage:  "plan",
		}},
		Warnings: []Warning{{Code: "same", Message: "warning"}},
	}
	withDuplicates := &Spec{
		Decisions: append(
			append([]Decision(nil), old.Decisions...),
			old.Decisions[0],
		),
		Warnings: append(
			append([]Warning(nil), old.Warnings...),
			old.Warnings[0],
		),
	}

	diff, err := Diff(old, withDuplicates)
	require.NoError(t, err)
	require.True(t, diff.HasChanges)
	require.NotEmpty(t, diff.Decisions)
	require.NotEmpty(t, diff.Warnings)

	reverse, err := Diff(withDuplicates, old)
	require.NoError(t, err)
	require.True(t, reverse.HasChanges)
	require.NotEmpty(t, reverse.Decisions)
	require.NotEmpty(t, reverse.Warnings)
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
