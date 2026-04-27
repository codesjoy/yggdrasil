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
	"errors"
	"fmt"
	"io"
	"log/slog"

	internalruntime "github.com/codesjoy/yggdrasil/v3/app/internal/runtime"
	"github.com/codesjoy/yggdrasil/v3/discovery/registry"
	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	"github.com/codesjoy/yggdrasil/v3/internal/remotelog"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/observability/logger"
	xotel "github.com/codesjoy/yggdrasil/v3/observability/otel"
	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	"github.com/codesjoy/yggdrasil/v3/rpc/interceptor"
	intlogging "github.com/codesjoy/yggdrasil/v3/rpc/interceptor/logging"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
	"github.com/codesjoy/yggdrasil/v3/transport/gateway/rest"
	grpcprotocol "github.com/codesjoy/yggdrasil/v3/transport/protocol/grpc"
	rpchttp "github.com/codesjoy/yggdrasil/v3/transport/protocol/rpchttp"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/client/balancer"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/server"
	"github.com/codesjoy/yggdrasil/v3/transport/support/marshaler"
	"github.com/codesjoy/yggdrasil/v3/transport/support/security"

	metricnoop "go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// --- snapshot state management ---

func (a *App) currentRuntimeSnapshot() *Snapshot {
	a.runtimeMu.RLock()
	defer a.runtimeMu.RUnlock()
	return a.runtimeSnapshot
}

func (a *App) setRuntimeSnapshot(snapshot *Snapshot) {
	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	a.runtimeSnapshot = snapshot
}

func (a *App) stageFoundationSnapshot(snapshot *Snapshot) {
	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	a.preparedFoundationSnapshot = snapshot
}

func (a *App) commitFoundationSnapshot(snapshot *Snapshot) {
	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	a.foundationSnapshot = snapshot
	if a.preparedFoundationSnapshot == snapshot {
		a.preparedFoundationSnapshot = nil
	}
}

func (a *App) rollbackFoundationSnapshot(snapshot *Snapshot) {
	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	if a.preparedFoundationSnapshot == snapshot {
		a.preparedFoundationSnapshot = nil
	}
}

func (a *App) foundationSnapshotForRuntime() *Snapshot {
	a.runtimeMu.RLock()
	defer a.runtimeMu.RUnlock()
	if a.preparedFoundationSnapshot != nil {
		return a.preparedFoundationSnapshot
	}
	if a.foundationSnapshot != nil {
		return a.foundationSnapshot
	}
	return a.runtimeSnapshot
}

// --- adapter application ---

func runtimeShutdown(v any) func(context.Context) error {
	switch item := v.(type) {
	case interface{ Shutdown(context.Context) error }:
		return item.Shutdown
	case io.Closer:
		return func(context.Context) error { return item.Close() }
	default:
		return nil
	}
}

func (a *App) applyRuntimeAdapters(snapshot *Snapshot) error {
	if snapshot == nil {
		return errors.New("runtime snapshot is nil")
	}

	snapshot.Identity = publicIdentity(a.identity)
	handler, err := snapshot.BuildDefaultLoggerHandler()
	if err != nil {
		return err
	}
	snapshot.Logger = slog.New(handler)
	remoteLoggerLvStr := snapshot.Resolved.Logging.RemoteLevel
	if remoteLoggerLvStr == "" {
		remoteLoggerLvStr = "error"
	}
	var remoteLoggerLv slog.Level
	if err = remoteLoggerLv.UnmarshalText([]byte(remoteLoggerLvStr)); err != nil {
		return err
	}
	snapshot.RemoteLogger = remotelog.New(remoteLoggerLv, handler)
	snapshot.TextMapPropagator = xotel.DefaultPropagator()

	oldTracer := a.swapTracerShutdown(nil)
	if tp, ok := snapshot.BuildTracerProvider(a.identity.AppName); ok {
		snapshot.TracerProvider = tp
		a.swapTracerShutdown(runtimeShutdown(tp))
	} else {
		snapshot.TracerProvider = tracenoop.NewTracerProvider()
		a.swapTracerShutdown(nil)
	}

	oldMeter := a.swapMeterShutdown(nil)
	if mp, ok := snapshot.BuildMeterProvider(a.identity.AppName); ok {
		snapshot.MeterProvider = mp
		a.swapMeterShutdown(runtimeShutdown(mp))
	} else {
		snapshot.MeterProvider = metricnoop.NewMeterProvider()
		a.swapMeterShutdown(nil)
	}

	statsBuilders := internalruntime.BindStatsHandlerBuildersWithRuntime(
		snapshot.Resolved,
		snapshot.StatsHandlerBuilders,
		snapshot.TracerProvider,
		snapshot.MeterProvider,
		snapshot.TextMapPropagator,
	)
	snapshot.StatsHandlerBuilders = statsBuilders
	snapshot.ServerStats = stats.BuildHandlerChainWithBuilders(
		snapshot.Resolved.Telemetry.Stats,
		statsBuilders,
		true,
	)
	snapshot.ClientStats = stats.BuildHandlerChainWithBuilders(
		snapshot.Resolved.Telemetry.Stats,
		statsBuilders,
		false,
	)
	if a.opts != nil && a.opts.processDefaults && a.processDefaultsLease != nil {
		a.processDefaultsLease.install(snapshot)
	}
	if oldTracer != nil {
		if shutdownErr := oldTracer(context.Background()); shutdownErr != nil {
			slog.Warn("shutdown previous tracer provider failed", slog.Any("error", shutdownErr))
		}
	}
	if oldMeter != nil {
		if shutdownErr := oldMeter(context.Background()); shutdownErr != nil {
			slog.Warn("shutdown previous meter provider failed", slog.Any("error", shutdownErr))
		}
	}

	return nil
}

func (a *App) swapTracerShutdown(next func(context.Context) error) func(context.Context) error {
	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	prev := a.tracerShutdown
	a.tracerShutdown = next
	return prev
}

func (a *App) swapMeterShutdown(next func(context.Context) error) func(context.Context) error {
	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	prev := a.meterShutdown
	a.meterShutdown = next
	return prev
}

func (a *App) shutdownRuntimeAdapters(ctx context.Context) error {
	a.runtimeMu.Lock()
	tracerShutdown := a.tracerShutdown
	meterShutdown := a.meterShutdown
	lease := a.processDefaultsLease
	a.tracerShutdown = nil
	a.meterShutdown = nil
	a.processDefaultsLease = nil
	a.runtimeMu.Unlock()

	var err error
	if lease != nil {
		err = errors.Join(err, lease.release(ctx))
	}
	if tracerShutdown != nil {
		err = errors.Join(err, tracerShutdown(ctx))
	}
	if meterShutdown != nil {
		err = errors.Join(err, meterShutdown(ctx))
	}
	return err
}

// --- initRegistry / initServer ---

func (a *App) initRegistry() {
	if a == nil || a.opts == nil {
		return
	}
	snapshot := a.currentRuntimeSnapshot()
	if snapshot == nil {
		return
	}
	typeName := snapshot.Resolved.Discovery.Registry.Type
	if typeName == "" {
		return
	}
	r, err := snapshot.NewRegistry()
	if err != nil {
		slog.Warn(
			"fault to initialize registry",
			slog.String("type", typeName),
			slog.Any("error", err),
		)
		return
	}
	a.opts.registry = r
	if c, ok := r.(io.Closer); ok {
		a.opts.lifecycleOptions = append(
			a.opts.lifecycleOptions,
			withLifecycleCleanup("registry", func(context.Context) error {
				return c.Close()
			}),
		)
	}
}

func (a *App) initServer() error {
	if a == nil || a.opts == nil {
		return nil
	}
	if a.opts.server != nil {
		return nil
	}
	resolved := a.opts.resolvedSettings
	if len(resolved.Server.Transports) == 0 && !resolved.Server.RestEnabled {
		return nil
	}
	svr, err := server.New(a.currentRuntimeSnapshot())
	if err != nil {
		return err
	}
	server.RegisterGovernorRoutes(a.opts.governor, svr, a.identity)
	a.opts.server = svr
	return nil
}

// --- snapshot building ---

func (a *App) buildFoundationRuntimeSnapshot() (*Snapshot, error) {
	if a == nil || a.hub == nil || a.opts == nil {
		return nil, fmt.Errorf("runtime app is not initialized")
	}
	resolved := effectiveResolved(a.lastPlanResult, a.opts.resolvedSettings)
	bindings := selectedCapabilityBindings(a.lastPlanResult, a.opts.resolvedSettings)

	writerBuilders, err := internalruntime.ResolveNamedRuntimeCapabilities[logger.WriterBuilder](
		a.hub,
		bindings,
		"observability.logger.writer",
		loggerWriterCapabilitySpec,
	)
	if err != nil {
		return nil, err
	}
	writerBuilders = internalruntime.BindLoggerWriterBuilders(resolved, writerBuilders)

	handlerBuilders, err := internalruntime.ResolveNamedRuntimeCapabilities[logger.HandlerBuilder](
		a.hub,
		bindings,
		"observability.logger.handler",
		loggerHandlerCapabilitySpec,
	)
	if err != nil {
		return nil, err
	}
	handlerBuilders = internalruntime.BindLoggerHandlerBuilders(
		resolved,
		handlerBuilders,
		writerBuilders,
	)

	tracerBuilders, err := internalruntime.ResolveNamedRuntimeCapabilities[xotel.TracerProviderBuilder](
		a.hub,
		bindings,
		"observability.otel.tracer_provider",
		tracerProviderCapabilitySpec,
	)
	if err != nil {
		return nil, err
	}
	meterBuilders, err := internalruntime.ResolveNamedRuntimeCapabilities[xotel.MeterProviderBuilder](
		a.hub,
		bindings,
		"observability.otel.meter_provider",
		meterProviderCapabilitySpec,
	)
	if err != nil {
		return nil, err
	}
	statsBuilders, err := internalruntime.ResolveNamedRuntimeCapabilities[stats.HandlerBuilder](
		a.hub,
		bindings,
		"observability.stats.handler",
		statsHandlerCapabilitySpec,
	)
	if err != nil {
		return nil, err
	}
	statsBuilders = internalruntime.BindStatsHandlerBuilders(resolved, statsBuilders)

	securityProviders, err := internalruntime.ResolveNamedRuntimeCapabilities[security.Provider](
		a.hub,
		bindings,
		"security.profile.provider",
		securityProfileCapabilitySpec,
	)
	if err != nil {
		return nil, err
	}

	marshalerBuilders, err := internalruntime.ResolveNamedRuntimeCapabilities[marshaler.MarshalerBuilder](
		a.hub,
		bindings,
		"marshaler.scheme",
		marshalerCapabilitySpec,
	)
	if err != nil {
		return nil, err
	}

	return &Snapshot{
		Resolved:               resolved,
		LoggerHandlerBuilders:  handlerBuilders,
		LoggerWriterBuilders:   writerBuilders,
		TracerProviderBuilders: tracerBuilders,
		MeterProviderBuilders:  meterBuilders,
		StatsHandlerBuilders:   statsBuilders,
		ServerStats: stats.BuildHandlerChainWithBuilders(
			resolved.Telemetry.Stats,
			statsBuilders,
			true,
		),
		ClientStats: stats.BuildHandlerChainWithBuilders(
			resolved.Telemetry.Stats,
			statsBuilders,
			false,
		),
		SecurityProviders:                securityProviders,
		SecurityProfiles:                 map[string]security.Profile{},
		MarshalerBuilderMap:              marshalerBuilders,
		TransportServerProviders:         map[string]remote.TransportServerProvider{},
		TransportClientProviders:         map[string]remote.TransportClientProvider{},
		UnaryServerInterceptorProviders:  map[string]interceptor.UnaryServerInterceptorProvider{},
		StreamServerInterceptorProviders: map[string]interceptor.StreamServerInterceptorProvider{},
		UnaryClientInterceptorProviders:  map[string]interceptor.UnaryClientInterceptorProvider{},
		StreamClientInterceptorProviders: map[string]interceptor.StreamClientInterceptorProvider{},
		RESTMiddlewareProviderMap:        map[string]rest.Provider{},
		RegistryProviders:                map[string]registry.Provider{},
		ResolverProviders:                map[string]resolver.Provider{},
		BalancerProviders:                map[string]balancer.Provider{},
	}, nil
}

func (a *App) buildRuntimeSnapshot() (*Snapshot, bool, error) {
	if a == nil || a.opts == nil {
		return nil, false, fmt.Errorf("runtime app is not initialized")
	}
	base := a.foundationSnapshotForRuntime()
	if base == nil {
		return nil, false, fmt.Errorf("foundation runtime snapshot is not available")
	}
	resolved := effectiveResolved(a.lastPlanResult, a.opts.resolvedSettings)
	bindings := selectedCapabilityBindings(a.lastPlanResult, a.opts.resolvedSettings)
	next := base.Copy()
	next.Resolved = resolved
	profiles, err := internalruntime.CompileSecurityProfiles(resolved, next.SecurityProviders)
	if err != nil {
		return nil, false, err
	}
	next.SecurityProfiles = profiles

	serverProviders, err := internalruntime.ResolveNamedRuntimeCapabilities[remote.TransportServerProvider](
		a.hub,
		bindings,
		"transport.server.provider",
		transportServerProviderCapabilitySpec,
	)
	if err != nil {
		return nil, false, err
	}
	for name, provider := range serverProviders {
		switch name {
		case grpcprotocol.Protocol:
			next.TransportServerProviders[name] = grpcprotocol.ServerProviderWithSettings(
				resolved.Transports.GRPC,
				next.ServerStats,
				next.SecurityProfiles,
			)
		case rpchttp.Protocol:
			next.TransportServerProviders[name] = rpchttp.ServerProviderWithSettings(
				resolved.Transports.HTTP,
				next.ServerStats,
				next.MarshalerBuilderMap,
				next.SecurityProfiles,
			)
		default:
			next.TransportServerProviders[name] = provider
		}
	}

	clientProviders, err := internalruntime.ResolveNamedRuntimeCapabilities[remote.TransportClientProvider](
		a.hub,
		bindings,
		"transport.client.provider",
		transportClientProviderCapabilitySpec,
	)
	if err != nil {
		return nil, false, err
	}
	for name, provider := range clientProviders {
		switch name {
		case grpcprotocol.Protocol:
			next.TransportClientProviders[name] = grpcprotocol.ClientProviderWithSettings(
				resolved.Transports.GRPC,
				next.SecurityProfiles,
			)
		case rpchttp.Protocol:
			next.TransportClientProviders[name] = rpchttp.ClientProviderWithSettings(
				resolved.Transports.HTTP,
				next.MarshalerBuilderMap,
				next.SecurityProfiles,
			)
		default:
			next.TransportClientProviders[name] = provider
		}
	}

	loggingCfg := internalruntime.LoggingInterceptorSource(resolved)
	unaryServerBuiltins := internalruntime.MapUnaryServerProviders(
		intlogging.BuiltinUnaryServerProvidersWithConfig(loggingCfg),
	)
	streamServerBuiltins := internalruntime.MapStreamServerProviders(
		intlogging.BuiltinStreamServerProvidersWithConfig(loggingCfg),
	)
	unaryClientBuiltins := internalruntime.MapUnaryClientProviders(
		intlogging.BuiltinUnaryClientProvidersWithConfig(loggingCfg),
	)
	streamClientBuiltins := internalruntime.MapStreamClientProviders(
		intlogging.BuiltinStreamClientProvidersWithConfig(loggingCfg),
	)

	unaryServerProviders, err := internalruntime.ResolveOrderedRuntimeCapabilities[interceptor.UnaryServerInterceptorProvider](
		a.hub,
		bindings,
		"rpc.interceptor.unary_server",
		unaryServerInterceptorCapabilitySpec,
	)
	if err != nil {
		return nil, false, err
	}
	internalruntime.CopyPreferredIntoMap(
		next.UnaryServerInterceptorProviders,
		unaryServerProviders,
		unaryServerBuiltins,
	)

	streamServerProviders, err := internalruntime.ResolveOrderedRuntimeCapabilities[interceptor.StreamServerInterceptorProvider](
		a.hub,
		bindings,
		"rpc.interceptor.stream_server",
		streamServerInterceptorCapabilitySpec,
	)
	if err != nil {
		return nil, false, err
	}
	internalruntime.CopyPreferredIntoMap(
		next.StreamServerInterceptorProviders,
		streamServerProviders,
		streamServerBuiltins,
	)

	unaryClientProviders, err := internalruntime.ResolveOrderedRuntimeCapabilities[interceptor.UnaryClientInterceptorProvider](
		a.hub,
		bindings,
		"rpc.interceptor.unary_client",
		unaryClientInterceptorCapabilitySpec,
	)
	if err != nil {
		return nil, false, err
	}
	internalruntime.CopyPreferredIntoMap(
		next.UnaryClientInterceptorProviders,
		unaryClientProviders,
		unaryClientBuiltins,
	)

	streamClientProviders, err := internalruntime.ResolveOrderedRuntimeCapabilities[interceptor.StreamClientInterceptorProvider](
		a.hub,
		bindings,
		"rpc.interceptor.stream_client",
		streamClientInterceptorCapabilitySpec,
	)
	if err != nil {
		return nil, false, err
	}
	internalruntime.CopyPreferredIntoMap(
		next.StreamClientInterceptorProviders,
		streamClientProviders,
		streamClientBuiltins,
	)

	restProviders, err := internalruntime.ResolveOrderedRuntimeCapabilities[rest.Provider](
		a.hub,
		bindings,
		"transport.rest.middleware",
		restMiddlewareCapabilitySpec,
	)
	if err != nil {
		return nil, false, err
	}
	for name, provider := range restProviders {
		switch name {
		case "logger":
			next.RESTMiddlewareProviderMap[name] = rest.BuiltinLoggingProvider()
		case "marshaler":
			next.RESTMiddlewareProviderMap[name] = newRuntimeMarshalerProvider(next)
		default:
			next.RESTMiddlewareProviderMap[name] = provider
		}
	}

	registryProviders, err := internalruntime.ResolveNamedRuntimeCapabilities[registry.Provider](
		a.hub,
		bindings,
		"discovery.registry.provider",
		registryProviderCapabilitySpec,
	)
	if err != nil {
		return nil, false, err
	}
	internalruntime.CopyIntoMap(next.RegistryProviders, registryProviders)
	if _, ok := next.RegistryProviders["multi_registry"]; ok {
		next.RegistryProviders["multi_registry"] = registry.BuiltinProviderWithFactory(
			func(typeName string, cfg map[string]any) (registry.Registry, error) {
				return next.NewRegistryByType(typeName, cfg)
			},
		)
	}

	resolverProviders, err := internalruntime.ResolveNamedRuntimeCapabilities[resolver.Provider](
		a.hub,
		bindings,
		"discovery.resolver.provider",
		resolverProviderCapabilitySpec,
	)
	if err != nil {
		return nil, false, err
	}
	internalruntime.CopyIntoMap(next.ResolverProviders, resolverProviders)

	balancerProviders, err := internalruntime.ResolveNamedRuntimeCapabilities[balancer.Provider](
		a.hub,
		bindings,
		"transport.balancer.provider",
		balancerProviderCapabilitySpec,
	)
	if err != nil {
		return nil, false, err
	}
	internalruntime.CopyPreferredIntoMap(
		next.BalancerProviders,
		balancerProviders,
		map[string]balancer.Provider{
			"round_robin": balancer.BuiltinProvider(),
		},
	)

	current := a.currentRuntimeSnapshot()
	return next, runtimeRequiresRestart(current, next), nil
}

func newRuntimeMarshalerProvider(snapshot *Snapshot) rest.Provider {
	if snapshot == nil {
		return internalruntime.NewMarshalerProvider(settings.Resolved{}, nil)
	}
	return internalruntime.NewMarshalerProvider(snapshot.Resolved, snapshot.MarshalerBuilderMap)
}

func runtimeRequiresRestart(current, next *Snapshot) bool {
	if current == nil || next == nil {
		return false
	}
	return internalruntime.ResolvedRequiresRestart(current.Resolved, next.Resolved)
}
