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

	"github.com/codesjoy/yggdrasil/v2/config"
)

// ResolveType resolves the balancer type.
func ResolveType(balancerName string) (string, error) {
	typeName := config.Get(config.Join(config.KeyBase, "balancer", balancerName, "type")).String("")

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
func LoadConfig(serviceName, balancerName string) config.Value {
	serviceKey := fmt.Sprintf("{%s}", serviceName)
	return config.GetMulti(
		config.Join(config.KeyBase, "balancer", "config"),
		config.Join(config.KeyBase, "balancer", balancerName, "config"),
		config.Join(config.KeyBase, "balancer", serviceKey, balancerName, "config"),
	)
}

// New creates a new balancer.
func New(serviceName, balancerName string, cli Client) (Balancer, error) {
	typeName, err := ResolveType(balancerName)
	if err != nil {
		return nil, err
	}
	builder, err := GetBuilder(typeName)
	if err != nil {
		return nil, err
	}
	return builder(serviceName, balancerName, cli)
}
