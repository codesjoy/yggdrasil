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

func TestEnableModule_AppliesCorrectly(t *testing.T) {
	target := newOverrideSet()
	target.DisabledModules["mymod"] = struct{}{}

	EnableModule("mymod").apply(&target)

	_, inEnabled := target.EnabledModules["mymod"]
	_, inDisabled := target.DisabledModules["mymod"]
	require.True(t, inEnabled, "should be in EnabledModules")
	require.False(t, inDisabled, "should be removed from DisabledModules")
}

func TestEnableModule_EmptyName(t *testing.T) {
	target := newOverrideSet()
	EnableModule("").apply(&target)
	require.Empty(t, target.EnabledModules)
}

func TestEnableModule_NilTarget(t *testing.T) {
	// Should not panic
	EnableModule("mymod").apply(nil)
}

func TestDisableModule_AppliesCorrectly(t *testing.T) {
	target := newOverrideSet()
	target.EnabledModules["mymod"] = struct{}{}

	DisableModule("mymod").apply(&target)

	_, inDisabled := target.DisabledModules["mymod"]
	_, inEnabled := target.EnabledModules["mymod"]
	require.True(t, inDisabled, "should be in DisabledModules")
	require.False(t, inEnabled, "should be removed from EnabledModules")
}

func TestDisableModule_EmptyName(t *testing.T) {
	target := newOverrideSet()
	DisableModule("").apply(&target)
	require.Empty(t, target.DisabledModules)
}

func TestDisableModule_NilTarget(t *testing.T) {
	// Should not panic
	DisableModule("mymod").apply(nil)
}

func TestDisableAuto_AppliesCorrectly(t *testing.T) {
	target := newOverrideSet()
	DisableAuto("rpc.interceptor.unary_server").apply(&target)
	_, ok := target.DisabledAuto["rpc.interceptor.unary_server"]
	require.True(t, ok, "should be in DisabledAuto")
}

func TestDisableAuto_EmptyPath(t *testing.T) {
	target := newOverrideSet()
	DisableAuto("").apply(&target)
	require.Empty(t, target.DisabledAuto)
}

func TestForceDefault_EmptyArgs(t *testing.T) {
	target := newOverrideSet()
	ForceDefault("", "module").apply(&target)
	ForceDefault("path", "").apply(&target)
	require.Empty(t, target.ForcedDefaults)
}

func TestForceTemplate_EmptyArgs(t *testing.T) {
	target := newOverrideSet()
	ForceTemplate("", "template", "v1").apply(&target)
	ForceTemplate("path", "", "v1").apply(&target)
	ForceTemplate("path", "template", "").apply(&target)
	require.Empty(t, target.ForcedTemplates)
}
