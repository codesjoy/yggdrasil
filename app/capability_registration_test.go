package app

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/capabilities"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/module"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
)

func TestCapabilityRegistrationWrapperModuleShape(t *testing.T) {
	reg := CapabilityRegistration{
		Name:         "capability.test.wrapper",
		Capabilities: func() []module.Capability { return nil },
	}
	app := newTestApp(t, "registration-wrapper", WithCapabilityRegistrations(reg))

	var wrapped module.Module
	for _, item := range app.plannedModules() {
		if item.Name() == reg.Name {
			wrapped = item
			break
		}
	}
	require.NotNil(t, wrapped)

	scoped, ok := wrapped.(module.Scoped)
	require.True(t, ok)
	assert.Equal(t, module.ScopeProvider, scoped.Scope())

	configurable, ok := wrapped.(module.Configurable)
	require.True(t, ok)
	assert.Equal(t, "", configurable.ConfigPath())

	_, ok = wrapped.(module.CapabilityProvider)
	assert.True(t, ok)
	_, ok = wrapped.(module.Startable)
	assert.False(t, ok)
	_, ok = wrapped.(module.Stoppable)
	assert.False(t, ok)
	_, ok = wrapped.(module.Reloadable)
	assert.False(t, ok)
	_, ok = wrapped.(module.Dependent)
	assert.False(t, ok)
	_, ok = wrapped.(module.AutoDescribed)
	assert.False(t, ok)
	_, ok = wrapped.(module.Ordered)
	assert.False(t, ok)
}

func TestCapabilityRegistrationInitReceivesRootViewWhenConfigPathEmpty(t *testing.T) {
	manager := newTestManager(t, map[string]any{
		"yggdrasil": map[string]any{
			"admin": map[string]any{
				"governor": map[string]any{
					"port": 0,
				},
			},
		},
	})
	var gotPath string
	var hasRoot bool
	app, err := New("registration-root-view",
		WithConfigManager(manager), WithCapabilityRegistrations(CapabilityRegistration{
			Name: "capability.test.root-view",
			Init: func(_ context.Context, view config.View) error {
				gotPath = view.Path()
				var raw struct {
					Yggdrasil map[string]any `mapstructure:"yggdrasil"`
				}
				if err := view.Decode(&raw); err != nil {
					return err
				}
				hasRoot = raw.Yggdrasil != nil
				return nil
			},
			Capabilities: func() []module.Capability { return nil },
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	require.NoError(t, app.Prepare(context.Background()))
	assert.Equal(t, "", gotPath)
	assert.True(t, hasRoot)
}

func TestCapabilityRegistrationPrepareExposesTransportProvider(t *testing.T) {
	recorder := newTransportRecorder()
	manager := newTestManager(t, assemblyTestConfig(false))
	app, err := New(
		"registration-provider",
		WithConfigManager(
			manager,
		),
		WithCapabilityRegistrations(testServerTransportRegistration("test", recorder)),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	require.NoError(t, app.Prepare(context.Background()))
	snapshot := app.Snapshot()
	require.NotNil(t, snapshot)
	require.NotNil(t, snapshot.TransportServerProvider("test"))
	assert.Equal(t, int32(0), atomic.LoadInt32(&recorder.startCalls))
}

func TestCapabilityRegistrationDuplicateNameRejected(t *testing.T) {
	manager := newTestManager(t, nil)
	app, err := New("registration-duplicate",
		WithConfigManager(manager), WithCapabilityRegistrations(
			CapabilityRegistration{
				Name:         "capability.test.duplicate",
				Capabilities: func() []module.Capability { return nil },
			},
			CapabilityRegistration{
				Name:         "capability.test.duplicate",
				Capabilities: func() []module.Capability { return nil },
			},
		),
	)
	require.NoError(t, err)
	err = app.Prepare(context.Background())
	require.ErrorContains(t, err, `module "capability.test.duplicate" already exists`)
}

func TestCapabilityRegistrationConflictsWithModuleName(t *testing.T) {
	manager := newTestManager(t, nil)
	app, err := New("registration-module-conflict",
		WithConfigManager(manager), WithModules(&stubModule{name: "capability.test.conflict"}),
		WithCapabilityRegistrations(CapabilityRegistration{
			Name:         "capability.test.conflict",
			Capabilities: func() []module.Capability { return nil },
		}),
	)
	require.NoError(t, err)
	err = app.Prepare(context.Background())
	require.ErrorContains(t, err, `module "capability.test.conflict" already exists`)
}

func TestCapabilityRegistrationCapabilitiesRunBeforeInit(t *testing.T) {
	manager := newTestManager(t, nil)
	var capCalls atomic.Int32
	var sawCapabilitiesBeforeInit atomic.Bool
	app, err := New("registration-order",
		WithConfigManager(manager), WithCapabilityRegistrations(CapabilityRegistration{
			Name: "capability.test.order",
			Init: func(context.Context, config.View) error {
				if capCalls.Load() > 0 {
					sawCapabilitiesBeforeInit.Store(true)
				}
				return nil
			},
			Capabilities: func() []module.Capability {
				capCalls.Add(1)
				return nil
			},
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	require.NoError(t, app.Prepare(context.Background()))
	assert.True(t, sawCapabilitiesBeforeInit.Load())
	assert.Greater(t, capCalls.Load(), int32(0))
}

func TestCapabilityRegistrationLazyProviderCanUseInitializedState(t *testing.T) {
	type lazyConfig struct {
		Address string `mapstructure:"address"`
	}
	type lazyState struct {
		mu      sync.RWMutex
		address string
	}

	state := &lazyState{}
	manager := newTestManager(t, map[string]any{
		"yggdrasil": map[string]any{
			"admin": map[string]any{
				"governor": map[string]any{
					"port": 0,
				},
			},
			"server": map[string]any{
				"transports": []any{"lazy"},
			},
			"transports": map[string]any{
				"lazy": map[string]any{
					"address": "127.0.0.1:56101",
				},
			},
		},
	})

	reg := CapabilityRegistration{
		Name:       "capability.test.lazy",
		ConfigPath: "yggdrasil.transports.lazy",
		Init: func(_ context.Context, view config.View) error {
			var cfg lazyConfig
			if err := view.Decode(&cfg); err != nil {
				return err
			}
			state.mu.Lock()
			state.address = cfg.Address
			state.mu.Unlock()
			return nil
		},
		Capabilities: func() []module.Capability {
			return []module.Capability{
				capabilities.ProvideNamed(
					capabilities.TransportServerProviderSpec,
					"lazy",
					remote.NewTransportServerProvider(
						"lazy",
						func(remote.MethodHandle) (remote.Server, error) {
							state.mu.RLock()
							defer state.mu.RUnlock()
							if state.address == "" {
								return nil, errors.New("lazy state address is empty")
							}
							return &lazyServer{address: state.address}, nil
						},
					),
				),
			}
		},
	}

	app, err := New("registration-lazy",
		WithConfigManager(manager), WithCapabilityRegistrations(reg),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	require.NoError(t, app.Prepare(context.Background()))
	require.NotNil(t, app.opts.server)
}

func testServerTransportRegistration(
	protocol string,
	recorder *transportRecorder,
) CapabilityRegistration {
	return CapabilityRegistration{
		Name: "capability.transport." + protocol,
		Capabilities: func() []module.Capability {
			return []module.Capability{
				capabilities.ProvideNamed(
					capabilities.TransportServerProviderSpec,
					protocol,
					remote.NewTransportServerProvider(
						protocol,
						func(remote.MethodHandle) (remote.Server, error) {
							return recorder.buildServer(), nil
						},
					),
				),
			}
		},
	}
}

type lazyServer struct {
	address string
}

func (s *lazyServer) Start() error               { return nil }
func (s *lazyServer) Handle() error              { return nil }
func (s *lazyServer) Stop(context.Context) error { return nil }
func (s *lazyServer) Info() remote.ServerInfo {
	return remote.ServerInfo{
		Protocol: "lazy",
		Address:  s.address,
	}
}
