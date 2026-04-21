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

package config

// Section is a typed view over a manager subsection.
type Section[T any] struct {
	manager *Manager
	path    []string
}

// Bind creates a typed section bound to the given manager path.
func Bind[T any](manager *Manager, path ...string) Section[T] {
	if manager == nil {
		manager = Default()
	}
	return Section[T]{
		manager: manager,
		path:    append([]string(nil), path...),
	}
}

// Current returns the current typed value for the section.
func (s Section[T]) Current() (T, error) {
	var out T
	err := s.manager.Section(s.path...).Decode(&out)
	return out, err
}

// Watch subscribes to typed section updates. The callback is invoked immediately.
func (s Section[T]) Watch(fn func(T, error)) func() {
	return s.manager.watch(s.path, func(snapshot Snapshot) {
		var out T
		err := snapshot.Decode(&out)
		if fn != nil {
			fn(out, err)
		}
	})
}
