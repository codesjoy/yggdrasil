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

// Package source provides interfaces for loading configuration
package source

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
)

var envPlaceholderRegexp = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// ExpandEnvPlaceholders replaces ${VAR} placeholders with environment values.
func ExpandEnvPlaceholders(scope string, data []byte) ([]byte, error) {
	if len(data) == 0 || !bytes.Contains(data, []byte("${")) {
		return data, nil
	}
	if scope == "" {
		scope = "config source"
	}

	var missing string
	result := envPlaceholderRegexp.ReplaceAllStringFunc(string(data), func(match string) string {
		if missing != "" {
			return match
		}

		name := envPlaceholderRegexp.FindStringSubmatch(match)[1]
		value, ok := os.LookupEnv(name)
		if !ok {
			missing = name
			return match
		}
		return value
	})
	if missing != "" {
		return nil, fmt.Errorf(
			"expand env placeholders for %s: missing environment variable %q",
			scope,
			missing,
		)
	}

	return []byte(result), nil
}

// ExpandEnvPlaceholdersInValue replaces ${VAR} placeholders inside string values.
func ExpandEnvPlaceholdersInValue(scope string, value any) (any, error) {
	switch item := value.(type) {
	case string:
		b, err := ExpandEnvPlaceholders(scope, []byte(item))
		if err != nil {
			return nil, err
		}
		return string(b), nil
	case []any:
		result := make([]any, len(item))
		for i := range item {
			next, err := ExpandEnvPlaceholdersInValue(scope, item[i])
			if err != nil {
				return nil, err
			}
			result[i] = next
		}
		return result, nil
	case map[string]any:
		result := make(map[string]any, len(item))
		for k, v := range item {
			next, err := ExpandEnvPlaceholdersInValue(scope, v)
			if err != nil {
				return nil, err
			}
			result[k] = next
		}
		return result, nil
	case map[any]any:
		result := make(map[any]any, len(item))
		for k, v := range item {
			next, err := ExpandEnvPlaceholdersInValue(scope, v)
			if err != nil {
				return nil, err
			}
			result[k] = next
		}
		return result, nil
	default:
		return value, nil
	}
}
