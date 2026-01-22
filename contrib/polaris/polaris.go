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
