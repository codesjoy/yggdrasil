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

package module

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// moduleDependencyErrors
// ---------------------------------------------------------------------------

func TestModuleDependencyErrors_Matching(t *testing.T) {
	all := []string{
		`module "a" depends on missing module "x"`,
		`module "b" depends on missing module "y"`,
		`module "a" depends on missing module "z"`,
	}
	result := moduleDependencyErrors("a", all)
	require.Len(t, result, 2)
	require.Contains(t, result[0], `module "a"`)
	require.Contains(t, result[1], `module "a"`)
}

func TestModuleDependencyErrors_NoMatch(t *testing.T) {
	all := []string{
		`module "b" depends on missing module "y"`,
	}
	result := moduleDependencyErrors("a", all)
	require.Empty(t, result)
}

func TestModuleDependencyErrors_EmptyInput(t *testing.T) {
	result := moduleDependencyErrors("a", nil)
	require.Empty(t, result)
}

// ---------------------------------------------------------------------------
// capabilityConflictsForSpec
// ---------------------------------------------------------------------------

func TestCapabilityConflictsForSpec_Matching(t *testing.T) {
	all := []string{
		`capability "foo" has duplicate provider name "bar"`,
		`capability "baz" cardinality mismatch`,
		`capability "foo" allows at most one provider`,
	}
	result := capabilityConflictsForSpec("foo", all)
	require.Len(t, result, 2)
}

func TestCapabilityConflictsForSpec_NoMatch(t *testing.T) {
	all := []string{
		`capability "baz" cardinality mismatch`,
	}
	result := capabilityConflictsForSpec("foo", all)
	require.Empty(t, result)
}

func TestCapabilityConflictsForSpec_EmptyInput(t *testing.T) {
	result := capabilityConflictsForSpec("foo", nil)
	require.Empty(t, result)
}

// ---------------------------------------------------------------------------
// Diagnostics with ReloadReporter module
// ---------------------------------------------------------------------------

func TestDiagnostics_ModulesIncludesReloadReporter(t *testing.T) {
	m := &reloadReporterModule{
		name:  "reporter-mod",
		state: ReloadState{Phase: ReloadPhaseRollback},
	}
	h := NewHub()
	require.NoError(t, h.Use(m))
	require.NoError(t, h.Seal())

	diag := h.Diagnostics()
	require.Len(t, diag.Modules, 1)
	require.Equal(t, "rollback", diag.Modules[0].ReloadPhase)
	require.Equal(t, "reporter-mod", diag.Modules[0].Name)
}

// ---------------------------------------------------------------------------
// Diagnostics: bindings with requested but no capability provider
// ---------------------------------------------------------------------------

func TestDiagnostics_BindingsWithRequestedButNoCapabilityProviders(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(&testModule{name: "a"}))
	require.NoError(t, h.Seal())

	h.SetCapabilityBindings(map[string][]string{
		"missing.spec": {"provider-x"},
	})

	diag := h.Diagnostics()
	require.Len(t, diag.Bindings, 1)
	require.Equal(t, "missing.spec", diag.Bindings[0].Spec)
	require.Equal(t, []string{"provider-x"}, diag.Bindings[0].Requested)
	require.Empty(t, diag.Bindings[0].Resolved)
	require.Equal(t, []string{"provider-x"}, diag.Bindings[0].Missing)
}
