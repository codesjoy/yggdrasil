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

package balancer

import (
	"fmt"
	"sync"
)

// Spec describes a named balancer configuration envelope.
type Spec struct {
	Type   string         `mapstructure:"type"`
	Config map[string]any `mapstructure:"config"`
}

type configStore struct {
	defaults map[string]Spec
	services map[string]map[string]Spec
}

var (
	configMu sync.RWMutex
	configV  = configStore{
		defaults: map[string]Spec{},
		services: map[string]map[string]Spec{},
	}
)

// Configure replaces the configured balancer specs.
func Configure(defaults map[string]Spec, services map[string]map[string]Spec) {
	configMu.Lock()
	defer configMu.Unlock()
	if defaults == nil {
		defaults = map[string]Spec{}
	}
	if services == nil {
		services = map[string]map[string]Spec{}
	}
	configV = configStore{defaults: defaults, services: services}
}

// ResolveType resolves the balancer type.
func ResolveType(balancerName string) (string, error) {
	configMu.RLock()
	typeName := configV.defaults[balancerName].Type
	configMu.RUnlock()

	// Fallback to default if not configured
	if typeName == "" {
		if balancerName == DefaultBalancerName {
			// Use built-in default for "default" balancer
			return DefaultBalancerType, nil
		}
		return "", fmt.Errorf("not found balancer type, name: %s", balancerName)
	}
	return typeName, nil
}

// LoadConfig loads the balancer config.
func LoadConfig(serviceName, balancerName string) map[string]any {
	configMu.RLock()
	merged := map[string]any{}
	for key, value := range configV.defaults[balancerName].Config {
		merged[key] = value
	}
	override := map[string]any{}
	if svc, ok := configV.services[serviceName]; ok {
		override = svc[balancerName].Config
	}
	configMu.RUnlock()
	for key, value := range override {
		merged[key] = value
	}
	return merged
}

// New creates a new balancer.
func New(serviceName, balancerName string, cli Client) (Balancer, error) {
	typeName, err := ResolveType(balancerName)
	if err != nil {
		return nil, err
	}
	provider, ok := GetProvider(typeName)
	if !ok {
		return nil, fmt.Errorf("not found balancer provider, type: %s", typeName)
	}
	return provider.New(serviceName, balancerName, cli)
}
