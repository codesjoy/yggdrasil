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

// Scope describes the runtime lifetime contract of one module.
type Scope int

const (
	// ScopeApp lives for the entire App lifetime.
	ScopeApp Scope = iota
	// ScopeProvider exposes factories/providers without owning hot-path instances.
	ScopeProvider
	// ScopeRuntimeFactory builds hot-path runtime objects and must not be registered in Hub.
	ScopeRuntimeFactory
)

// Scoped exposes the declared runtime scope of one module.
type Scoped interface {
	Scope() Scope
}

func moduleScope(mod Module) Scope {
	if scoped, ok := mod.(Scoped); ok {
		return scoped.Scope()
	}
	return ScopeApp
}
