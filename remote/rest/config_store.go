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

package rest

import "sync"

var (
	configMu sync.RWMutex
	configV  *Config
)

// Configure replaces the current rest server config.
func Configure(cfg *Config) {
	configMu.Lock()
	defer configMu.Unlock()
	configV = cfg
}

// CurrentConfig returns the current rest server config.
func CurrentConfig() *Config {
	configMu.RLock()
	defer configMu.RUnlock()
	return configV
}
