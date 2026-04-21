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

package governor

import (
	"sync"

	"github.com/codesjoy/yggdrasil/v2/config"
)

var (
	configMu      sync.RWMutex
	configV       Config
	configManager *config.Manager
)

// Configure installs the resolved governor config and backing manager.
func Configure(cfg Config, manager *config.Manager) {
	configMu.Lock()
	defer configMu.Unlock()
	configV = cfg
	configManager = manager
}

func currentConfig() Config {
	configMu.RLock()
	defer configMu.RUnlock()
	return configV
}

func currentManager() *config.Manager {
	configMu.RLock()
	defer configMu.RUnlock()
	return configManager
}
