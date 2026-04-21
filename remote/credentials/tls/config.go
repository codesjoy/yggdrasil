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

package tls

import "sync"

// BuilderConfig is the exported TLS credential config shape.
type BuilderConfig = builderConfig

var (
	configMu sync.RWMutex
	globalV  builderConfig
	serviceV = map[string]builderConfig{}
)

// Configure installs resolved TLS credential settings.
func Configure(global builderConfig, services map[string]builderConfig) {
	configMu.Lock()
	defer configMu.Unlock()
	globalV = global
	if services == nil {
		services = map[string]builderConfig{}
	}
	serviceV = services
}

func currentBuilderConfig(serviceName string) builderConfig {
	configMu.RLock()
	defer configMu.RUnlock()
	cfg := globalV
	if serviceName == "" {
		return cfg
	}
	if svc, ok := serviceV[serviceName]; ok {
		mergeBuilderConfig(&cfg, svc)
	}
	return cfg
}
