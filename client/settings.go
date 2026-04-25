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
	"github.com/codesjoy/yggdrasil/v3/internal/backoff"
	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
)

// RemoteSettings contains static endpoints and attributes for a client service.
type RemoteSettings struct {
	Endpoints  []resolver.BaseEndpoint `mapstructure:"endpoints"`
	Attributes map[string]any          `mapstructure:"attributes"`
}

// InterceptorSettings contains interceptor names for a client service.
type InterceptorSettings struct {
	Unary  []string `mapstructure:"unary"`
	Stream []string `mapstructure:"stream"`
}

// ServiceSettings contains the resolved client settings for one service.
type ServiceSettings struct {
	FastFail     bool                `mapstructure:"fast_fail"`
	Resolver     string              `mapstructure:"resolver"`
	Balancer     string              `mapstructure:"balancer"`
	Backoff      backoff.Config      `mapstructure:"backoff"`
	Remote       RemoteSettings      `mapstructure:"remote"`
	Interceptors InterceptorSettings `mapstructure:"interceptors"`
}

// Settings contains resolved client settings for all services.
type Settings struct {
	Services map[string]ServiceSettings `mapstructure:"services"`
}
