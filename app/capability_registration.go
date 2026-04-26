package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/module"
)

// CapabilityRegistration registers provider-only capabilities without requiring
// callers to implement a full module type.
//
// The Capabilities callback may run during planning and Hub.Seal, before Init.
// It must therefore be deterministic, side-effect free, and must not assume
// initialization has already completed. When capability values depend on
// configuration, return lazy providers/factories that read shared state later.
type CapabilityRegistration struct {
	Name         string
	ConfigPath   string
	Init         func(context.Context, config.View) error
	Capabilities func() []module.Capability
}

// WithCapabilityRegistrations registers provider-only capability extensions.
func WithCapabilityRegistrations(regs ...CapabilityRegistration) Option {
	return func(opts *options) error {
		for i, reg := range regs {
			reg.Name = strings.TrimSpace(reg.Name)
			reg.ConfigPath = strings.TrimSpace(reg.ConfigPath)
			if reg.Name == "" {
				return fmt.Errorf("capability registration[%d] name is empty", i)
			}
			if reg.Capabilities == nil {
				return fmt.Errorf(
					"capability registration %q capabilities callback is nil",
					reg.Name,
				)
			}
			if reg.Init == nil && reg.ConfigPath != "" {
				return fmt.Errorf(
					"capability registration %q config_path requires init callback",
					reg.Name,
				)
			}
			opts.capabilityRegistrations = append(opts.capabilityRegistrations, reg)
		}
		return nil
	}
}

type capabilityRegistrationModule struct {
	reg CapabilityRegistration
}

func (m capabilityRegistrationModule) Name() string { return m.reg.Name }

func (m capabilityRegistrationModule) Scope() module.Scope { return module.ScopeProvider }

func (m capabilityRegistrationModule) ConfigPath() string { return m.reg.ConfigPath }

func (m capabilityRegistrationModule) Init(ctx context.Context, view config.View) error {
	if m.reg.Init == nil {
		return nil
	}
	return m.reg.Init(ctx, view)
}

func (m capabilityRegistrationModule) Capabilities() []module.Capability {
	if m.reg.Capabilities == nil {
		return nil
	}
	return m.reg.Capabilities()
}
