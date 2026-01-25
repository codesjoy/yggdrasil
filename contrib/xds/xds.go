package xds

import (
	"github.com/codesjoy/yggdrasil/v2/resolver"
)

func init() {
	resolver.RegisterBuilder("xds", func(name string) (resolver.Resolver, error) {
		cfg := LoadResolverConfig(name)
		return NewResolver(name, cfg)
	})
}
