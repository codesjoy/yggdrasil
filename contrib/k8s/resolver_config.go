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

package k8s

import (
	"os"
	"time"

	"github.com/codesjoy/yggdrasil/v2/config"
)

type resolverMode string

const (
	modeEndpointSlice resolverMode = "endpointslice"
)

// ResolverConfig resolver config
type ResolverConfig struct {
	Namespace    string        `mapstructure:"namespace"`
	Mode         string        `mapstructure:"mode"`
	PortName     string        `mapstructure:"portName"`
	Port         int32         `mapstructure:"port"`
	Protocol     string        `mapstructure:"protocol"`
	Kubeconfig   string        `mapstructure:"kubeconfig"`
	ResyncPeriod time.Duration `mapstructure:"resyncPeriod"`
	Timeout      time.Duration `mapstructure:"timeout"`
	Backoff      backoffConfig `mapstructure:"backoff"`

	EndpointAttributes map[string]string `mapstructure:"endpointAttributes"`
}

type backoffConfig struct {
	BaseDelay  time.Duration `mapstructure:"baseDelay"`
	Multiplier float64       `mapstructure:"multiplier"`
	Jitter     float64       `mapstructure:"jitter"`
	MaxDelay   time.Duration `mapstructure:"maxDelay"`
}

// LoadResolverConfig load resolver config
func LoadResolverConfig(name string) ResolverConfig {
	var cfg ResolverConfig
	_ = config.Get(config.Join(config.KeyBase, "resolver", name, "config")).Scan(&cfg)

	if cfg.Namespace == "" {
		if ns := os.Getenv("KUBERNETES_NAMESPACE"); ns != "" {
			cfg.Namespace = ns
		} else {
			cfg.Namespace = "default"
		}
	}
	if cfg.Mode == "" {
		cfg.Mode = string(modeEndpointSlice)
	}
	if cfg.Protocol == "" {
		cfg.Protocol = "grpc"
	}
	if cfg.Backoff.BaseDelay == 0 {
		cfg.Backoff.BaseDelay = time.Second
	}
	if cfg.Backoff.Multiplier == 0 {
		cfg.Backoff.Multiplier = 1.6
	}
	if cfg.Backoff.Jitter == 0 {
		cfg.Backoff.Jitter = 0.2
	}
	if cfg.Backoff.MaxDelay == 0 {
		cfg.Backoff.MaxDelay = time.Second * 30
	}
	return cfg
}
