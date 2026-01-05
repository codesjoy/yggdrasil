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

package env

// Option is the option for env
type Option func(*env)

// WithParseArray set parse array
func WithParseArray(seg string) Option {
	if seg == "" {
		seg = ";"
	}
	return func(e *env) {
		e.parseArray = true
		e.arraySep = seg
	}
}

// SetKeyDelimiter set key delimiter
func SetKeyDelimiter(delimiter string) Option {
	if delimiter == "" {
		delimiter = "_"
	}
	return func(e *env) {
		e.delimiter = delimiter
	}
}
