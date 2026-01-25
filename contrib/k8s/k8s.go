package k8s

import (
	"github.com/codesjoy/yggdrasil/v2/resolver"
)

func init() {
	resolver.RegisterBuilder("kubernetes", func(name string) (resolver.Resolver, error) {
		cfg := LoadResolverConfig(name)
		r, err := NewResolver(name, cfg)
		if err != nil {
			return nil, err
		}
		return r, nil
	})
}
