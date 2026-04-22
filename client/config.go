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

package client

import (
	"sync"

	"github.com/codesjoy/yggdrasil/v3/internal/backoff"
	"github.com/codesjoy/yggdrasil/v3/resolver"
)

// RemoteConfig contains static endpoints and attributes for a client service.
type RemoteConfig struct {
	Endpoints  []resolver.BaseEndpoint `mapstructure:"endpoints"`
	Attributes map[string]any          `mapstructure:"attributes"`
}

// InterceptorConfig contains interceptor names for a client service.
type InterceptorConfig struct {
	Unary  []string `mapstructure:"unary"`
	Stream []string `mapstructure:"stream"`
}

// ServiceConfig contains the resolved client config for one service.
type ServiceConfig struct {
	FastFail     bool              `mapstructure:"fast_fail"`
	Resolver     string            `mapstructure:"resolver"`
	Balancer     string            `mapstructure:"balancer"`
	Backoff      backoff.Config    `mapstructure:"backoff"`
	Remote       RemoteConfig      `mapstructure:"remote"`
	Interceptors InterceptorConfig `mapstructure:"interceptors"`
}

// Settings contains resolved client settings for all services.
type Settings struct {
	Services map[string]ServiceConfig `mapstructure:"services"`
}

var (
	settingsMu sync.RWMutex
	settingsV  = Settings{Services: map[string]ServiceConfig{}}
)

// Configure replaces the resolved client settings.
func Configure(next Settings) {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	if next.Services == nil {
		next.Services = map[string]ServiceConfig{}
	}
	settingsV = next
}

// CurrentConfig returns the resolved config for the given service.
func CurrentConfig(name string) ServiceConfig {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	return settingsV.Services[name]
}
