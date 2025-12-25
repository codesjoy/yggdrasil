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

// Package config provides a set of constants for configuration keys.
package config

import (
	"regexp"
	"strings"
)

// KeyBase is the base key for framework configuration.
var KeyBase = "yggdrasil"

// Join joins the given strings with the key delimiter.
func Join(s ...string) string {
	return strings.Join(s, keyDelimiter)
}

var regx, _ = regexp.Compile(`{([\w.-]+)}`)

func genPath(key, delimiter string) []string {
	matches := make([]string, 0)
	key = regx.ReplaceAllStringFunc(key, func(s string) string {
		matches = append(matches, s[1:len(s)-1])
		return "{}"
	})
	paths := strings.Split(key, delimiter)
	j := 0
	for i, item := range paths {
		if item == "{}" {
			paths[i] = matches[j]
			j++
		}
	}
	return paths
}
