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

import "sync"

var (
	defaultMu      sync.RWMutex
	defaultManager = NewManager()
)

// Default returns the process-global configuration manager used by yggdrasil bootstrapping.
func Default() *Manager {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultManager
}

// SetDefault swaps the process-global configuration manager and returns the previous one.
func SetDefault(next *Manager) *Manager {
	if next == nil {
		next = NewManager()
	}
	defaultMu.Lock()
	prev := defaultManager
	defaultManager = next
	defaultMu.Unlock()
	return prev
}
