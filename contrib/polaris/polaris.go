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

package polaris

import (
	"github.com/codesjoy/yggdrasil/v2/config"
	yregistry "github.com/codesjoy/yggdrasil/v2/registry"
	yresolver "github.com/codesjoy/yggdrasil/v2/resolver"
)

func init() {
	yregistry.RegisterBuilder("polaris", func(cfgVal config.Value) (yregistry.Registry, error) {
		var cfg RegistryConfig
		_ = cfgVal.Scan(&cfg)
		cfg.Addresses = resolveSDKAddresses("default", cfg.SDK, cfg.Addresses)
		r, err := NewRegistry("default", cfg)
		if err != nil {
			reg := NewRegistryWithError(cfg, err)
			if cfg.SDK != "" {
				reg.instanceName = cfg.SDK
			} else {
				reg.instanceName = "default"
			}
			return reg, nil
		}
		if cfg.SDK != "" {
			r.instanceName = cfg.SDK
		} else {
			r.instanceName = "default"
		}
		return r, nil
	})

	yresolver.RegisterBuilder("polaris", func(name string) (yresolver.Resolver, error) {
		cfg := LoadResolverConfig(name)
		r, err := NewResolver(name, cfg)
		if err != nil {
			return nil, err
		}
		return r, nil
	})
}
