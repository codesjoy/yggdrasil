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

// Package memory provides an in-memory configuration source useful for tests.
package memory

import (
	configinternal "github.com/codesjoy/yggdrasil/v3/config/internal"
	"github.com/codesjoy/yggdrasil/v3/config/source"
)

type memory struct {
	name string
	data map[string]any
}

func (m *memory) Kind() string {
	return "memory"
}

func (m *memory) Name() string {
	return m.name
}

func (m *memory) Read() (source.Data, error) {
	return source.NewMapData(configinternal.NormalizeMap(m.data)), nil
}

func (m *memory) Close() error {
	return nil
}

// NewSource returns a new in-memory source with the provided name.
func NewSource(name string, data map[string]any) source.Source {
	return &memory{
		name: name,
		data: configinternal.NormalizeMap(data),
	}
}
