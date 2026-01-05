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

// Package xstring provides string utilities for yggdrasil.
package xstring

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// ToLowerFirstCamelCase returns the given string in camelcase formatted string
// but with the first letter being lowercase.
func ToLowerFirstCamelCase(s string) string {
	if s == "" {
		return s
	}
	if len(s) == 1 {
		return strings.ToLower(string(s[0]))
	}
	s = StrToCamelCase(s)
	return strings.ToLower(string(s[0])) + s[1:]
}

// StrToCamelCase converts from underscore separated form to camel case form.
func StrToCamelCase(s string) string {
	if s == "" {
		return ""
	}
	words := strings.ReplaceAll(s, "_", " ")
	caser := cases.Title(language.Und)
	titleStr := caser.String(words)
	return strings.ReplaceAll(titleStr, " ", "")
}
