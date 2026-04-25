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

func TestHashAndDiffAreCanonical(t *testing.T) {
	left := &Spec{
		Identity: IdentitySpec{AppName: "hash-test"},
		Mode:     Mode{Name: "dev", Profile: "dev", Bundle: "server-basic"},
		Modules: []ModuleRef{
			{Name: "b", Kind: "module", Source: "user"},
			{Name: "a", Kind: "builtin", Source: "framework"},
		},
		Defaults: map[string]string{
			capLoggerWriter:  "console",
			capLoggerHandler: "text",
		},
		Chains: map[string]Chain{
			chainUnaryServer: {Items: []string{"logging"}},
		},
		Decisions: []Decision{
			{
				Kind:   "override.force_default",
				Target: capLoggerHandler,
				Value:  "text",
				Source: "code_override",
			},
			{Kind: "mode", Target: "yggdrasil.mode", Value: "dev", Source: "config"},
		},
	}
	right := &Spec{
		Identity: IdentitySpec{AppName: "hash-test"},
		Mode:     Mode{Name: "dev", Profile: "dev", Bundle: "server-basic"},
		Modules: []ModuleRef{
			{Name: "a", Kind: "builtin", Source: "framework"},
			{Name: "b", Kind: "module", Source: "user"},
		},
		Defaults: map[string]string{
			capLoggerHandler: "text",
			capLoggerWriter:  "console",
		},
		Chains: map[string]Chain{
			chainUnaryServer: {Items: []string{"logging"}},
		},
		Decisions: []Decision{
			{Kind: "mode", Target: "yggdrasil.mode", Value: "dev", Source: "config"},
			{
				Kind:   "override.force_default",
				Target: capLoggerHandler,
				Value:  "text",
				Source: "code_override",
			},
		},
	}

	leftHash, err := Hash(left)
	require.NoError(t, err)
	rightHash, err := Hash(right)
	require.NoError(t, err)
	require.Equal(t, leftHash, rightHash)

	explained, err := Explain(left)
	require.NoError(t, err)
	require.Contains(t, string(explained), `"identity"`)

	diff, err := Diff(left, &Spec{
		Identity: left.Identity,
		Mode:     left.Mode,
		Modules:  left.Modules,
		Defaults: map[string]string{
			capLoggerHandler: "json",
		},
		Chains: map[string]Chain{
			chainUnaryServer: {
				Template: "default-observable",
				Version:  "v1",
				Items:    []string{"logging"},
			},
		},
		Decisions: []Decision{
			{
				Kind:   "override.force_default",
				Target: capLoggerHandler,
				Value:  "json",
				Source: "code_override",
			},
		},
	})
	require.NoError(t, err)
	require.True(t, diff.HasChanges)
	require.NotEmpty(t, diff.Defaults)
	require.NotEmpty(t, diff.Chains)
	require.NotEmpty(t, diff.Overrides)
	require.Contains(t, diff.AffectedDomains, "defaults")
	require.Contains(t, diff.AffectedDomains, "chains")
	require.Contains(t, diff.AffectedDomains, "overrides")
}
