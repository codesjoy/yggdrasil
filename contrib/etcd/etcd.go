package etcd

import (
	"github.com/codesjoy/yggdrasil/v2/config"
	yregistry "github.com/codesjoy/yggdrasil/v2/registry"
	yresolver "github.com/codesjoy/yggdrasil/v2/resolver"
)

func init() {
	yregistry.RegisterBuilder("etcd", func(cfgVal config.Value) (yregistry.Registry, error) {
		var cfg RegistryConfig
		_ = cfgVal.Scan(&cfg)
		return NewRegistry(cfg)
	})

	yresolver.RegisterBuilder("etcd", func(name string) (yresolver.Resolver, error) {
		cfg := LoadResolverConfig(name)
		return NewResolver(name, cfg)
	})
}
