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

package source

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpandEnvPlaceholders(t *testing.T) {
	t.Setenv("APP_NAME", "demo")
	t.Setenv("PORT", "8080")

	plain, err := ExpandEnvPlaceholders("scope", []byte("no placeholders"))
	require.NoError(t, err)
	require.Equal(t, "no placeholders", string(plain))

	out, err := ExpandEnvPlaceholders("cfg", []byte(`name=${APP_NAME},port=${PORT}`))
	require.NoError(t, err)
	require.Equal(t, "name=demo,port=8080", string(out))

	_, err = ExpandEnvPlaceholders("cfg", []byte(`name=${MISSING}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), `missing environment variable "MISSING"`)
}

func TestExpandEnvPlaceholdersInValue(t *testing.T) {
	t.Setenv("APP_NAME", "demo")
	t.Setenv("APP_ENV", "prod")

	src := map[string]any{
		"name": "${APP_NAME}",
		"arr":  []any{"${APP_ENV}", map[string]any{"k": "${APP_NAME}"}},
		"any":  map[any]any{1: "${APP_ENV}"},
	}

	out, err := ExpandEnvPlaceholdersInValue("scope", src)
	require.NoError(t, err)
	result := out.(map[string]any)
	require.Equal(t, "demo", result["name"])
	require.Equal(t, "prod", result["arr"].([]any)[0])
	require.Equal(t, "demo", result["arr"].([]any)[1].(map[string]any)["k"])
	require.Equal(t, "prod", result["any"].(map[any]any)[1])

	value, err := ExpandEnvPlaceholdersInValue("scope", 123)
	require.NoError(t, err)
	require.Equal(t, 123, value)

	_, err = ExpandEnvPlaceholdersInValue("scope", map[string]any{"x": "${NOT_FOUND}"})
	require.Error(t, err)
}
