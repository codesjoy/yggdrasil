package grpcx

import (
	"context"
	"sync"

	"github.com/codesjoy/yggdrasil/v3/app"
	"github.com/codesjoy/yggdrasil/v3/capabilities"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	"github.com/codesjoy/yggdrasil/v3/module"
	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
	grpcprotocol "github.com/codesjoy/yggdrasil/v3/transport/protocol/grpc"
)

const (
	// Protocol is the custom transport protocol name used by this example.
	Protocol = "grpcx"

	// RegistrationName is the visible registration/module name used by the app.
	RegistrationName = "capability.transport.grpcx"

	configPath = "yggdrasil.transports.grpcx"
)

// Config is the example-local config schema decoded from yggdrasil.transports.grpcx.
type Config struct {
	Client grpcprotocol.ClientConfig `mapstructure:"client"`
	Server grpcprotocol.ServerConfig `mapstructure:"server"`
}

type state struct {
	mu  sync.RWMutex
	cfg Config
}

// NewRegistration creates one provider-only capability registration.
func NewRegistration() app.CapabilityRegistration {
	st := &state{}
	return app.CapabilityRegistration{
		Name:       RegistrationName,
		ConfigPath: configPath,
		Init:       st.init,
		Capabilities: func() []module.Capability {
			return st.capabilities()
		},
	}
}

func (s *state) init(_ context.Context, view config.View) error {
	var cfg Config
	if err := view.Decode(&cfg); err != nil {
		return err
	}
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	return nil
}

func (s *state) capabilities() []module.Capability {
	return []module.Capability{
		capabilities.ProvideNamed(
			capabilities.TransportServerProviderSpec,
			Protocol,
			s.newServerProvider(),
		),
		capabilities.ProvideNamed(
			capabilities.TransportClientProviderSpec,
			Protocol,
			s.newClientProvider(),
		),
	}
}

func (s *state) newServerProvider() remote.TransportServerProvider {
	return remote.NewTransportServerProvider(
		Protocol,
		func(handle remote.MethodHandle) (remote.Server, error) {
			settings := s.settingsSnapshot()
			provider := grpcprotocol.ServerProviderWithSettings(
				settings,
				stats.NoOpHandler,
				nil,
			)
			return provider.NewServer(handle)
		},
	)
}

func (s *state) newClientProvider() remote.TransportClientProvider {
	return remote.NewTransportClientProvider(
		Protocol,
		func(
			ctx context.Context,
			serviceName string,
			endpoint resolver.Endpoint,
			_ stats.Handler,
			onStateChange remote.OnStateChange,
		) (remote.Client, error) {
			settings := s.settingsSnapshot()
			provider := grpcprotocol.ClientProviderWithSettings(settings, nil)
			return provider.NewClient(
				ctx,
				serviceName,
				endpoint,
				stats.NoOpHandler,
				onStateChange,
			)
		},
	)
}

func (s *state) settingsSnapshot() grpcprotocol.Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return grpcprotocol.Settings{
		Client:         s.cfg.Client,
		ClientServices: map[string]grpcprotocol.ClientConfig{},
		Server:         s.cfg.Server,
	}
}
