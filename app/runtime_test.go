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

package app

import (
	"context"
	"io"
	"log/slog"
	"reflect"
	"sync/atomic"
	"testing"

	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	"github.com/codesjoy/yggdrasil/v3/internal/remotelog"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/module"
	"github.com/codesjoy/yggdrasil/v3/observability/logger"
	xotel "github.com/codesjoy/yggdrasil/v3/observability/otel"
	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
)

func TestAppNewClientUsesAppScopedRuntimeInsteadOfGlobalStores(t *testing.T) {
	app, _ := newTestAppWithConfig(t, "app-scoped-client", map[string]any{
		"yggdrasil": map[string]any{
			"clients": map[string]any{
				"services": map[string]any{
					"svc": map[string]any{
						"remote": map[string]any{
							"endpoints": []any{
								map[string]any{
									"address":  "127.0.0.1:18080",
									"protocol": "http",
								},
							},
						},
					},
				},
			},
			"transports": map[string]any{
				"grpc": map[string]any{
					"client": map[string]any{},
					"server": map[string]any{},
				},
				"http": map[string]any{
					"client": map[string]any{},
					"server": map[string]any{},
				},
			},
		},
	})
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	cli, err := app.NewClient(context.Background(), "svc")
	require.NoError(t, err)
	require.NotNil(t, cli)

	snapshot := app.currentRuntimeSnapshot()
	require.NotNil(t, snapshot)
	require.Len(t, snapshot.ClientSettings("svc").Remote.Endpoints, 1)
	require.NotNil(t, snapshot.TransportClientProvider("http"))
}

type moduleResolver struct{ resolverName string }

func (r *moduleResolver) AddWatch(string, resolver.Client) error { return nil }
func (r *moduleResolver) DelWatch(string, resolver.Client) error { return nil }
func (r *moduleResolver) Type() string                           { return r.resolverName }

type resolverCapabilityModule struct{}

func (resolverCapabilityModule) Name() string { return "test.resolver.capability" }

func (resolverCapabilityModule) Capabilities() []module.Capability {
	return []module.Capability{
		{
			Spec: module.CapabilitySpec{
				Name:        "discovery.resolver.provider",
				Cardinality: module.NamedOne,
				Type:        reflect.TypeOf((*resolver.Provider)(nil)).Elem(),
			},
			Name: "module-resolver",
			Value: resolver.NewProvider(
				"module-resolver",
				func(name string) (resolver.Resolver, error) {
					return &moduleResolver{resolverName: name}, nil
				},
			),
		},
	}
}

func TestModuleSuppliedResolverProviderIsUsedByRuntimeSnapshotAndClient(t *testing.T) {
	app, _ := newInitializedAppWithConfig(t, "module-resolver-client", map[string]any{
		"yggdrasil": map[string]any{
			"clients": map[string]any{
				"services": map[string]any{
					"svc": map[string]any{
						"resolver": "svc",
					},
				},
			},
			"discovery": map[string]any{
				"resolvers": map[string]any{
					"svc": map[string]any{
						"type": "module-resolver",
					},
				},
			},
			"transports": map[string]any{
				"grpc": map[string]any{
					"client": map[string]any{},
					"server": map[string]any{},
				},
				"http": map[string]any{
					"client": map[string]any{},
					"server": map[string]any{},
				},
			},
		},
	},
		WithModules(resolverCapabilityModule{}),
	)
	t.Cleanup(func() { _ = app.Stop(context.Background()) })
	snapshot := app.currentRuntimeSnapshot()
	require.NotNil(t, snapshot)

	resolvedResolver, err := snapshot.NewResolver("svc")
	require.NoError(t, err)
	require.NotNil(t, resolvedResolver)
	require.Equal(t, "svc", resolvedResolver.Type())

	cli, err := app.NewClient(context.Background(), "svc")
	require.NoError(t, err)
	require.NotNil(t, cli)

	diag := app.hub.Diagnostics()
	binding := findBindingDiag(t, diag, "discovery.resolver.provider")
	require.Equal(t, []string{"module-resolver"}, binding.Requested)
	require.Equal(t, []string{"module-resolver"}, binding.Resolved)
}

func TestRuntimeAppliesLoggingFromResolvedSettings(t *testing.T) {
	app, _ := newInitializedAppWithConfig(t, "logger-runtime", map[string]any{
		"yggdrasil": map[string]any{
			"logging": map[string]any{
				"writers": map[string]any{
					"default": map[string]any{"type": "console"},
				},
				"handlers": map[string]any{
					"default": map[string]any{
						"type":   "text",
						"writer": "default",
						"config": map[string]any{"level": "info"},
					},
				},
				"remote_level": "error",
			},
		},
	})
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	snapshot := app.currentRuntimeSnapshot()
	require.NotNil(t, snapshot)
	_, err := snapshot.BuildDefaultLoggerHandler()
	require.NoError(t, err)

	require.True(t, slog.Default().Handler().Enabled(context.Background(), slog.LevelInfo))
	require.False(t, remotelog.Logger().Handler().Enabled(context.Background(), slog.LevelInfo))
	require.True(t, remotelog.Logger().Handler().Enabled(context.Background(), slog.LevelError))
}

type shutdownTracerProvider struct {
	trace.TracerProvider
	shutdowns *int32
}

func (p *shutdownTracerProvider) Shutdown(context.Context) error {
	atomic.AddInt32(p.shutdowns, 1)
	return nil
}

type shutdownMeterProvider struct {
	metric.MeterProvider
	shutdowns *int32
}

func (p *shutdownMeterProvider) Shutdown(context.Context) error {
	atomic.AddInt32(p.shutdowns, 1)
	return nil
}

func TestApplyRuntimeAdapters_ShutsDownPreviousTracerAndMeterProviders(t *testing.T) {
	var tracerShutdowns int32
	var meterShutdowns int32

	buildSnapshot := func(tracerName, meterName string) *Snapshot {
		return &Snapshot{
			Resolved: settings.Resolved{
				Logging: logger.Settings{
					Handlers: map[string]logger.HandlerSpec{
						"default": {
							Type:   "text",
							Writer: "default",
						},
					},
					Writers: map[string]logger.WriterSpec{
						"default": {Type: "console"},
					},
					RemoteLevel: "error",
				},
				Telemetry: settings.Telemetry{
					Tracer: tracerName,
					Meter:  meterName,
				},
			},
			LoggerHandlerBuilders: map[string]logger.HandlerBuilder{
				"text": func(string, map[string]any) (slog.Handler, error) {
					return slog.NewTextHandler(io.Discard, nil), nil
				},
			},
			TracerProviderBuilders: map[string]xotel.TracerProviderBuilder{
				tracerName: func(string) trace.TracerProvider {
					return &shutdownTracerProvider{
						TracerProvider: tracenoop.NewTracerProvider(),
						shutdowns:      &tracerShutdowns,
					}
				},
			},
			MeterProviderBuilders: map[string]xotel.MeterProviderBuilder{
				meterName: func(string) metric.MeterProvider {
					return &shutdownMeterProvider{
						MeterProvider: metricnoop.NewMeterProvider(),
						shutdowns:     &meterShutdowns,
					}
				},
			},
		}
	}

	app := &App{}
	require.NoError(t, app.applyRuntimeAdapters(buildSnapshot("tracer-a", "meter-a")))
	require.NoError(t, app.applyRuntimeAdapters(buildSnapshot("tracer-b", "meter-b")))

	require.Equal(t, int32(1), atomic.LoadInt32(&tracerShutdowns))
	require.Equal(t, int32(1), atomic.LoadInt32(&meterShutdowns))
	require.NoError(t, app.shutdownRuntimeAdapters(context.Background()))
	require.Equal(t, int32(2), atomic.LoadInt32(&tracerShutdowns))
	require.Equal(t, int32(2), atomic.LoadInt32(&meterShutdowns))
}

func TestSnapshotReturnsDetachedCopy(t *testing.T) {
	app, _ := newInitializedAppWithConfig(t, "snapshot-copy", minimalV3Config("grpc"))
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	first := app.Snapshot()
	require.NotNil(t, first)
	first.TransportClientProviders["detached"] = nil

	second := app.Snapshot()
	require.NotNil(t, second)
	require.NotSame(t, first, second)
	require.NotContains(t, second.TransportClientProviders, "detached")
}

// --- runtimeShutdown ---

type shutdownable struct {
	called bool
}

func (s *shutdownable) Shutdown(_ context.Context) error {
	s.called = true
	return nil
}

type closable struct {
	called bool
}

func (c *closable) Close() error {
	c.called = true
	return nil
}

func TestRuntimeShutdown(t *testing.T) {
	t.Run("Shutdown interface", func(t *testing.T) {
		s := &shutdownable{}
		fn := runtimeShutdown(s)
		require.NotNil(t, fn)
		require.NoError(t, fn(context.Background()))
		assert.True(t, s.called)
	})

	t.Run("Closer interface", func(t *testing.T) {
		c := &closable{}
		fn := runtimeShutdown(c)
		require.NotNil(t, fn)
		require.NoError(t, fn(context.Background()))
		assert.True(t, c.called)
	})

	t.Run("non-matching type returns nil", func(t *testing.T) {
		fn := runtimeShutdown("string")
		assert.Nil(t, fn)
	})

	t.Run("nil returns nil", func(t *testing.T) {
		fn := runtimeShutdown(nil)
		assert.Nil(t, fn)
	})
}

// --- stage/commit/rollback foundation snapshot ---

func TestApp_StageCommitRollbackFoundationSnapshot(t *testing.T) {
	t.Run("stage and commit lifecycle", func(t *testing.T) {
		app := newTestApp(t, "test")
		snap := &Snapshot{Resolved: settings.Resolved{}}

		app.stageFoundationSnapshot(snap)
		assert.Equal(t, snap, app.preparedFoundationSnapshot)

		app.commitFoundationSnapshot(snap)
		assert.Equal(t, snap, app.foundationSnapshot)
		assert.Nil(t, app.preparedFoundationSnapshot)
	})

	t.Run("stage and rollback lifecycle", func(t *testing.T) {
		app := newTestApp(t, "test")
		snap := &Snapshot{Resolved: settings.Resolved{}}

		app.stageFoundationSnapshot(snap)
		assert.Equal(t, snap, app.preparedFoundationSnapshot)

		app.rollbackFoundationSnapshot(snap)
		assert.Nil(t, app.preparedFoundationSnapshot)
	})

	t.Run("rollback different snapshot is no-op", func(t *testing.T) {
		app := newTestApp(t, "test")
		snap1 := &Snapshot{Resolved: settings.Resolved{App: settings.Application{Name: "1"}}}
		snap2 := &Snapshot{Resolved: settings.Resolved{App: settings.Application{Name: "2"}}}

		app.stageFoundationSnapshot(snap1)
		app.rollbackFoundationSnapshot(snap2)
		assert.Equal(t, snap1, app.preparedFoundationSnapshot)
	})
}

// --- swapTracerShutdown / swapMeterShutdown ---

func TestApp_SwapTracerShutdown(t *testing.T) {
	t.Run("swaps and returns previous", func(t *testing.T) {
		app := newTestApp(t, "test")
		prev := func(context.Context) error { return nil }
		next := func(context.Context) error { return nil }

		app.swapTracerShutdown(prev)
		returned := app.swapTracerShutdown(next)
		assert.NotNil(t, returned)
	})
}

func TestApp_SwapMeterShutdown(t *testing.T) {
	t.Run("swaps and returns previous", func(t *testing.T) {
		app := newTestApp(t, "test")
		prev := func(context.Context) error { return nil }
		next := func(context.Context) error { return nil }

		app.swapMeterShutdown(prev)
		returned := app.swapMeterShutdown(next)
		assert.NotNil(t, returned)
	})
}

// --- initRegistry ---

func TestApp_InitRegistry(t *testing.T) {
	t.Run("nil app returns without panic", func(t *testing.T) {
		var app *App
		app.initRegistry()
	})

	t.Run("nil opts returns without panic", func(t *testing.T) {
		app := &App{}
		app.initRegistry()
	})

	t.Run("nil snapshot returns without panic", func(t *testing.T) {
		app := newTestApp(t, "test")
		app.initRegistry()
	})
}

// --- foundationSnapshotForRuntime ---

func TestApp_FoundationSnapshotForRuntime(t *testing.T) {
	t.Run("prefers prepared snapshot", func(t *testing.T) {
		app := newTestApp(t, "test")
		prepared := &Snapshot{
			Resolved: settings.Resolved{App: settings.Application{Name: "prepared"}},
		}
		foundation := &Snapshot{
			Resolved: settings.Resolved{App: settings.Application{Name: "foundation"}},
		}

		app.stageFoundationSnapshot(prepared)
		app.commitFoundationSnapshot(foundation)

		result := app.foundationSnapshotForRuntime()
		assert.Equal(t, "prepared", result.Resolved.App.Name)
	})

	t.Run("falls back to foundation snapshot", func(t *testing.T) {
		app := newTestApp(t, "test")
		foundation := &Snapshot{
			Resolved: settings.Resolved{App: settings.Application{Name: "foundation"}},
		}

		app.commitFoundationSnapshot(foundation)

		result := app.foundationSnapshotForRuntime()
		assert.Equal(t, "foundation", result.Resolved.App.Name)
	})

	t.Run("falls back to runtime snapshot", func(t *testing.T) {
		app := newTestApp(t, "test")
		runtimeSnap := &Snapshot{
			Resolved: settings.Resolved{App: settings.Application{Name: "runtime"}},
		}

		app.setRuntimeSnapshot(runtimeSnap)

		result := app.foundationSnapshotForRuntime()
		assert.Equal(t, "runtime", result.Resolved.App.Name)
	})
}

// --- runtimeRequiresRestart ---

func TestRuntimeRequiresRestart(t *testing.T) {
	t.Run("nil snapshots return false", func(t *testing.T) {
		assert.False(t, runtimeRequiresRestart(nil, nil))
		assert.False(t, runtimeRequiresRestart(nil, &Snapshot{}))
		assert.False(t, runtimeRequiresRestart(&Snapshot{}, nil))
	})

	t.Run("identical snapshots return false", func(t *testing.T) {
		current := &Snapshot{Resolved: settings.Resolved{}}
		next := &Snapshot{Resolved: settings.Resolved{}}
		assert.False(t, runtimeRequiresRestart(current, next))
	})

	t.Run("different server settings return true", func(t *testing.T) {
		current := &Snapshot{Resolved: settings.Resolved{}}
		next := &Snapshot{Resolved: settings.Resolved{}}
		next.Resolved.Server.Transports = []string{"grpc"}
		assert.True(t, runtimeRequiresRestart(current, next))
	})

	t.Run("different discovery return true", func(t *testing.T) {
		current := &Snapshot{Resolved: settings.Resolved{}}
		next := &Snapshot{Resolved: settings.Resolved{}}
		next.Resolved.Discovery.Registry.Type = "multi_registry"
		assert.True(t, runtimeRequiresRestart(current, next))
	})
}

// --- Transport providers from snapshot ---

func TestRuntimeSnapshotTransportProvidersBuildClientsAndServersFromExplicitBuilders(t *testing.T) {
	app, _ := newInitializedAppWithConfig(t, "runtime-transport-providers", map[string]any{
		"yggdrasil": map[string]any{
			"server": map[string]any{
				"transports": []any{"grpc", "http"},
			},
			"clients": map[string]any{
				"services": map[string]any{
					"svc": map[string]any{
						"remote": map[string]any{
							"endpoints": []any{
								map[string]any{
									"address":  "127.0.0.1:18080",
									"protocol": "http",
								},
							},
						},
					},
				},
			},
			"transports": map[string]any{
				"grpc": map[string]any{
					"client": map[string]any{
						"transport": map[string]any{
							"security_profile": "insecure-default",
						},
					},
					"server": map[string]any{},
				},
				"security": map[string]any{
					"profiles": map[string]any{
						"insecure-default": map[string]any{"type": "insecure"},
					},
				},
				"http": map[string]any{
					"client": map[string]any{},
					"server": map[string]any{},
				},
			},
		},
	})
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	snapshot := app.currentRuntimeSnapshot()
	require.NotNil(t, snapshot)

	for _, protocol := range []string{"grpc", "http"} {
		serverProvider := snapshot.TransportServerProvider(protocol)
		require.NotNil(t, serverProvider)
		server, err := serverProvider.NewServer(func(remote.ServerStream) {})
		require.NoError(t, err)
		require.Equal(t, protocol, server.Info().Protocol)

		clientProvider := snapshot.TransportClientProvider(protocol)
		require.NotNil(t, clientProvider)
		client, err := clientProvider.NewClient(
			context.Background(),
			"svc",
			resolver.BaseEndpoint{Protocol: protocol, Address: "127.0.0.1:65535"},
			stats.NoOpHandler,
			nil,
		)
		require.NoError(t, err)
		require.Equal(t, protocol, client.Protocol())
		require.NoError(t, client.Close())
	}
}
