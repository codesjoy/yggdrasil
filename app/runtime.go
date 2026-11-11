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
	"maps"

	internallifecycle "github.com/codesjoy/yggdrasil/v3/app/internal/lifecycle"
	internalruntime "github.com/codesjoy/yggdrasil/v3/app/internal/runtime"
	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
	"github.com/codesjoy/yggdrasil/v3/discovery/registry"
	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	"github.com/codesjoy/yggdrasil/v3/internal/remotelog"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/module"
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

func (a *App) runtimePlan() *yassembly.Result {
	if a == nil {
		return nil
	}
	a.planMu.RLock()
	defer a.planMu.RUnlock()
	if a.reloadCandidate != nil {
		return a.reloadCandidate
	}
	return a.stablePlanResult
}

func (a *App) stablePlan() *yassembly.Result {
	if a == nil {
		return nil
	}
	a.planMu.RLock()
	defer a.planMu.RUnlock()
	return a.stablePlanResult
}

func (a *App) stageReloadCandidate(result *yassembly.Result) {
	a.planMu.Lock()
	a.reloadCandidate = result
	a.planMu.Unlock()
}

func (a *App) clearReloadCandidate() {
	a.planMu.Lock()
	a.reloadCandidate = nil
	a.planMu.Unlock()
}

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
	_ = a.commitFoundationSnapshotTransaction(snapshot)
}

func (a *App) commitFoundationSnapshotTransaction(snapshot *Snapshot) *Snapshot {
	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	previous := a.foundationSnapshot
	a.foundationSnapshot = snapshot
	if a.preparedFoundationSnapshot == snapshot {
		a.preparedFoundationSnapshot = nil
	}
	return previous
}

func (a *App) rollbackFoundationSnapshotTransaction(
	snapshot *Snapshot,
	previous *Snapshot,
	committed bool,
) {
	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	if a.preparedFoundationSnapshot == snapshot {
		a.preparedFoundationSnapshot = nil
	}
	if !committed || a.foundationSnapshot != snapshot {
		return
	}
	a.foundationSnapshot = previous
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

type preparedRuntimeAdapters struct {
	snapshot *Snapshot

	previousSnapshot *Snapshot
	oldTracer        func(context.Context) error
	oldMeter         func(context.Context) error

	tracerShutdown func(context.Context) error
	meterShutdown  func(context.Context) error
	lease          *processDefaultsLease

	activated  bool
	finalized  bool
	rolledBack bool
}

func (a *App) applyRuntimeAdapters(snapshot *Snapshot) error {
	prepared, err := a.prepareRuntimeAdapters(snapshot)
	if err != nil {
		return err
	}
	if err = a.activateRuntimeAdapters(prepared); err != nil {
		return err
	}
	return a.finalizeRuntimeAdapters(prepared)
}

func (a *App) prepareRuntimeAdapters(snapshot *Snapshot) (*preparedRuntimeAdapters, error) {
	if snapshot == nil {
		return nil, errors.New("runtime snapshot is nil")
	}
	if err := a.configureSnapshotLogging(snapshot); err != nil {
		return nil, err
	}
	prepared := &preparedRuntimeAdapters{snapshot: snapshot}
	configureSnapshotProviders(snapshot, prepared, a.identity.AppName)
	bindSnapshotStats(snapshot)
	if a.opts != nil && a.opts.processDefaults {
		a.runtimeMu.RLock()
		prepared.lease = a.processDefaultsLease
		a.runtimeMu.RUnlock()
	}
	return prepared, nil
}

func (a *App) configureSnapshotLogging(snapshot *Snapshot) error {
	snapshot.Identity = publicIdentity(a.identity)
	if snapshot.Logger == nil {
		handler, err := snapshot.BuildDefaultLoggerHandler()
		if err != nil {
			return err
		}
		snapshot.Logger = slog.New(handler)
	}
	handler := snapshot.Logger.Handler()
	remoteLoggerLvStr := snapshot.Resolved.Logging.RemoteLevel
	if remoteLoggerLvStr == "" {
		remoteLoggerLvStr = "error"
	}
	var remoteLoggerLv slog.Level
	if err := remoteLoggerLv.UnmarshalText([]byte(remoteLoggerLvStr)); err != nil {
		return err
	}
	snapshot.RemoteLogger = remotelog.New(remoteLoggerLv, handler)
	snapshot.TextMapPropagator = xotel.DefaultPropagator()
	return nil
}

func configureSnapshotProviders(
	snapshot *Snapshot,
	prepared *preparedRuntimeAdapters,
	instanceName string,
) {
	if tp, ok := snapshot.BuildTracerProvider(instanceName); ok {
		snapshot.TracerProvider = tp
		prepared.tracerShutdown = runtimeShutdown(tp)
	} else {
		snapshot.TracerProvider = tracenoop.NewTracerProvider()
	}

	if mp, ok := snapshot.BuildMeterProvider(instanceName); ok {
		snapshot.MeterProvider = mp
		prepared.meterShutdown = runtimeShutdown(mp)
	} else {
		snapshot.MeterProvider = metricnoop.NewMeterProvider()
	}
}

func bindSnapshotStats(snapshot *Snapshot) {
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
}

func (a *App) activateRuntimeAdapters(prepared *preparedRuntimeAdapters) error {
	if prepared == nil || prepared.snapshot == nil {
		return errors.New("prepared runtime adapters are required")
	}
	a.runtimeMu.Lock()
	if !prepared.activated {
		prepared.previousSnapshot = a.runtimeSnapshot
		prepared.oldTracer = a.tracerShutdown
		prepared.oldMeter = a.meterShutdown
		a.runtimeSnapshot = prepared.snapshot
		a.tracerShutdown = prepared.tracerShutdown
		a.meterShutdown = prepared.meterShutdown
		prepared.activated = true
	}
	a.runtimeMu.Unlock()
	prepared.lease.install(prepared.snapshot)
	return nil
}

func (a *App) rollbackRuntimeAdapters(prepared *preparedRuntimeAdapters) error {
	if prepared == nil || prepared.rolledBack {
		return nil
	}
	prepared.rolledBack = true
	if prepared.activated {
		a.runtimeMu.Lock()
		a.runtimeSnapshot = prepared.previousSnapshot
		a.tracerShutdown = prepared.oldTracer
		a.meterShutdown = prepared.oldMeter
		a.runtimeMu.Unlock()
		prepared.lease.install(prepared.previousSnapshot)
	}
	return shutdownPreparedAdapters(
		context.Background(),
		a.runtimeLogger(),
		prepared.tracerShutdown,
		prepared.meterShutdown,
	)
}

func (a *App) finalizeRuntimeAdapters(prepared *preparedRuntimeAdapters) error {
	if prepared == nil || prepared.finalized {
		return nil
	}
	prepared.finalized = true
	return shutdownPreparedAdapters(
		context.Background(),
		a.runtimeLogger(),
		prepared.oldTracer,
		prepared.oldMeter,
	)
}

func (a *App) stageRuntimeAdapters(prepared *preparedRuntimeAdapters) {
	a.runtimeMu.Lock()
	a.pendingRuntimeAdapters = prepared
	a.runtimeMu.Unlock()
}

func (a *App) rollbackPendingRuntimeAdapters(
	prepared *preparedRuntimeAdapters,
) error {
	err := a.rollbackRuntimeAdapters(prepared)
	a.runtimeMu.Lock()
	if a.pendingRuntimeAdapters == prepared {
		a.pendingRuntimeAdapters = nil
	}
	a.runtimeMu.Unlock()
	return err
}

func (a *App) finalizePendingRuntimeAdapters() error {
	a.runtimeMu.Lock()
	prepared := a.pendingRuntimeAdapters
	a.pendingRuntimeAdapters = nil
	a.runtimeMu.Unlock()
	return a.finalizeRuntimeAdapters(prepared)
}

func shutdownPreparedAdapters(
	ctx context.Context,
	logger *slog.Logger,
	tracerShutdown func(context.Context) error,
	meterShutdown func(context.Context) error,
) error {
	var err error
	if tracerShutdown != nil {
		if shutdownErr := tracerShutdown(ctx); shutdownErr != nil {
			err = errors.Join(err, shutdownErr)
			logger.Warn(
				"shutdown previous tracer provider failed",
				slog.Any("error", shutdownErr),
			)
		}
	}
	if meterShutdown != nil {
		if shutdownErr := meterShutdown(ctx); shutdownErr != nil {
			err = errors.Join(err, shutdownErr)
			logger.Warn(
				"shutdown previous meter provider failed",
				slog.Any("error", shutdownErr),
			)
		}
	}
	return err
}

func (a *App) shutdownRuntimeAdapters(ctx context.Context) error {
	a.runtimeMu.Lock()
	pending := a.pendingRuntimeAdapters
	tracerShutdown := a.tracerShutdown
	meterShutdown := a.meterShutdown
	lease := a.processDefaultsLease
	a.pendingRuntimeAdapters = nil
	a.tracerShutdown = nil
	a.meterShutdown = nil
	a.processDefaultsLease = nil
	a.runtimeMu.Unlock()

	var err error
	if pending != nil {
		err = errors.Join(err, a.rollbackRuntimeAdapters(pending))
	}
	if lease != nil {
		lease.release()
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

func (a *App) initRegistry() error {
	if a == nil || a.opts == nil {
		return nil
	}
	snapshot := a.currentRuntimeSnapshot()
	if snapshot == nil {
		return nil
	}
	typeName := snapshot.Resolved.Discovery.Registry.Type
	if typeName == "" {
		return nil
	}
	r, err := snapshot.NewRegistry()
	if err != nil {
		return fmt.Errorf(
			"initialize registry %q: %w",
			typeName,
			err,
		)
	}
	a.opts.registry = r
	if c, ok := r.(io.Closer); ok {
		a.opts.lifecycleOptions = append(
			a.opts.lifecycleOptions,
			internallifecycle.WithCleanup("registry", func(context.Context) error {
				return c.Close()
			}),
		)
	}
	return nil
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
	planResult := a.runtimePlan()
	resolved := effectiveResolved(planResult, a.opts.resolvedSettings)
	bindings := selectedCapabilityBindings(planResult, a.opts.resolvedSettings)
	capabilities, err := a.resolveFoundationCapabilities(resolved, bindings)
	if err != nil {
		return nil, err
	}
	return newFoundationSnapshot(resolved, capabilities), nil
}

type foundationRuntimeCapabilities struct {
	writerBuilders    map[string]logger.WriterBuilder
	handlerBuilders   map[string]logger.HandlerBuilder
	tracerBuilders    map[string]xotel.TracerProviderBuilder
	meterBuilders     map[string]xotel.MeterProviderBuilder
	statsBuilders     map[string]stats.HandlerBuilder
	security          map[string]security.Provider
	marshalerBuilders map[string]marshaler.MarshalerBuilder
}

func (a *App) resolveFoundationCapabilities(
	resolved settings.Resolved,
	bindings map[string][]string,
) (foundationRuntimeCapabilities, error) {
	var out foundationRuntimeCapabilities
	writerBuilders, err := internalruntime.ResolveNamedRuntimeCapabilities[logger.WriterBuilder](
		a.hub,
		bindings,
		"observability.logger.writer",
		loggerWriterCapabilitySpec,
	)
	if err != nil {
		return foundationRuntimeCapabilities{}, err
	}
	out.writerBuilders = internalruntime.BindLoggerWriterBuilders(resolved, writerBuilders)
	handlerBuilders, err := internalruntime.ResolveNamedRuntimeCapabilities[logger.HandlerBuilder](
		a.hub,
		bindings,
		"observability.logger.handler",
		loggerHandlerCapabilitySpec,
	)
	if err != nil {
		return foundationRuntimeCapabilities{}, err
	}
	out.handlerBuilders = internalruntime.BindLoggerHandlerBuilders(
		resolved,
		handlerBuilders,
		out.writerBuilders,
	)
	out.tracerBuilders, err = internalruntime.ResolveNamedRuntimeCapabilities[xotel.TracerProviderBuilder](
		a.hub,
		bindings,
		"observability.otel.tracer_provider",
		tracerProviderCapabilitySpec,
	)
	if err != nil {
		return foundationRuntimeCapabilities{}, err
	}
	out.meterBuilders, err = internalruntime.ResolveNamedRuntimeCapabilities[xotel.MeterProviderBuilder](
		a.hub,
		bindings,
		"observability.otel.meter_provider",
		meterProviderCapabilitySpec,
	)
	if err != nil {
		return foundationRuntimeCapabilities{}, err
	}
	out.statsBuilders, err = internalruntime.ResolveNamedRuntimeCapabilities[stats.HandlerBuilder](
		a.hub,
		bindings,
		"observability.stats.handler",
		statsHandlerCapabilitySpec,
	)
	if err != nil {
		return foundationRuntimeCapabilities{}, err
	}
	out.statsBuilders = internalruntime.BindStatsHandlerBuilders(resolved, out.statsBuilders)
	out.security, err = internalruntime.ResolveNamedRuntimeCapabilities[security.Provider](
		a.hub,
		bindings,
		"security.profile.provider",
		securityProfileCapabilitySpec,
	)
	if err != nil {
		return foundationRuntimeCapabilities{}, err
	}
	out.marshalerBuilders, err = internalruntime.ResolveNamedRuntimeCapabilities[marshaler.MarshalerBuilder](
		a.hub,
		bindings,
		"marshaler.scheme",
		marshalerCapabilitySpec,
	)
	if err != nil {
		return foundationRuntimeCapabilities{}, err
	}
	return out, nil
}

func newFoundationSnapshot(
	resolved settings.Resolved,
	capabilities foundationRuntimeCapabilities,
) *Snapshot {
	return &Snapshot{
		Resolved:               resolved,
		LoggerHandlerBuilders:  capabilities.handlerBuilders,
		LoggerWriterBuilders:   capabilities.writerBuilders,
		TracerProviderBuilders: capabilities.tracerBuilders,
		MeterProviderBuilders:  capabilities.meterBuilders,
		StatsHandlerBuilders:   capabilities.statsBuilders,
		ServerStats: stats.BuildHandlerChainWithBuilders(
			resolved.Telemetry.Stats,
			capabilities.statsBuilders,
			true,
		),
		ClientStats: stats.BuildHandlerChainWithBuilders(
			resolved.Telemetry.Stats,
			capabilities.statsBuilders,
			false,
		),
		SecurityProviders:                capabilities.security,
		SecurityProfiles:                 map[string]security.Profile{},
		MarshalerBuilderMap:              capabilities.marshalerBuilders,
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
	}
}

func (a *App) buildRuntimeSnapshot() (*Snapshot, bool, error) {
	next, resolved, bindings, err := a.newRuntimeSnapshot()
	if err != nil {
		return nil, false, err
	}
	if err = a.configureRuntimeCapabilities(next, resolved, bindings); err != nil {
		return nil, false, err
	}
	current := a.currentRuntimeSnapshot()
	return next, runtimeRequiresRestart(current, next), nil
}

func (a *App) newRuntimeSnapshot() (
	*Snapshot,
	settings.Resolved,
	map[string][]string,
	error,
) {
	if a == nil || a.opts == nil {
		return nil, settings.Resolved{}, nil, fmt.Errorf("runtime app is not initialized")
	}
	base := a.foundationSnapshotForRuntime()
	if base == nil {
		return nil, settings.Resolved{}, nil, fmt.Errorf(
			"foundation runtime snapshot is not available",
		)
	}
	planResult := a.runtimePlan()
	resolved := effectiveResolved(planResult, a.opts.resolvedSettings)
	bindings := selectedCapabilityBindings(planResult, a.opts.resolvedSettings)
	next := base.Copy()
	next.Identity = publicIdentity(a.identity)
	next.Resolved = resolved
	if next.Logger == nil {
		handler, err := next.BuildDefaultLoggerHandler()
		if err != nil {
			return nil, settings.Resolved{}, nil, err
		}
		next.Logger = slog.New(handler)
	}
	profiles, err := internalruntime.CompileSecurityProfiles(resolved, next.SecurityProviders)
	if err != nil {
		return nil, settings.Resolved{}, nil, err
	}
	next.SecurityProfiles = profiles
	return next, resolved, bindings, nil
}

func (a *App) configureRuntimeCapabilities(
	next *Snapshot,
	resolved settings.Resolved,
	bindings map[string][]string,
) error {
	if err := a.configureTransportProviders(next, resolved, bindings); err != nil {
		return err
	}
	builtins, err := loadBuiltinInterceptors(next.Logger, resolved)
	if err != nil {
		return err
	}
	if err = a.configureInterceptors(next, bindings, builtins); err != nil {
		return err
	}
	if err = a.configureRESTProviders(next, bindings); err != nil {
		return err
	}
	if err = a.configureRegistryProviders(next, bindings); err != nil {
		return err
	}
	if err = a.configureResolverProviders(next, bindings); err != nil {
		return err
	}
	return a.configureBalancerProviders(next, bindings)
}

func (a *App) configureRESTProviders(next *Snapshot, bindings map[string][]string) error {
	restProviders, err := internalruntime.ResolveOrderedCapabilityMap[rest.Provider](
		a.hub,
		bindings["transport.rest.middleware"],
		restMiddlewareCapabilitySpec,
	)
	if err != nil {
		return err
	}
	for name, provider := range restProviders {
		switch name {
		case "logger":
			next.RESTMiddlewareProviderMap[name] = rest.BuiltinLoggingProviderWithLogger(
				next.Logger,
			)
		case "marshaler":
			next.RESTMiddlewareProviderMap[name] = newRuntimeMarshalerProvider(next)
		default:
			next.RESTMiddlewareProviderMap[name] = provider
		}
	}
	return nil
}

func (a *App) configureRegistryProviders(
	next *Snapshot,
	bindings map[string][]string,
) error {
	registryProviders, err := internalruntime.ResolveNamedRuntimeCapabilities[registry.Provider](
		a.hub,
		bindings,
		"discovery.registry.provider",
		registryProviderCapabilitySpec,
	)
	if err != nil {
		return err
	}
	maps.Copy(next.RegistryProviders, registryProviders)
	if _, ok := next.RegistryProviders["multi_registry"]; ok {
		next.RegistryProviders["multi_registry"] = registry.BuiltinProviderWithFactory(
			func(typeName string, cfg map[string]any) (registry.Registry, error) {
				return next.NewRegistryByType(typeName, cfg)
			},
		)
	}
	return nil
}

func (a *App) configureResolverProviders(
	next *Snapshot,
	bindings map[string][]string,
) error {
	resolverProviders, err := internalruntime.ResolveNamedRuntimeCapabilities[resolver.Provider](
		a.hub,
		bindings,
		"discovery.resolver.provider",
		resolverProviderCapabilitySpec,
	)
	if err != nil {
		return err
	}
	maps.Copy(next.ResolverProviders, resolverProviders)
	return nil
}

func (a *App) configureBalancerProviders(
	next *Snapshot,
	bindings map[string][]string,
) error {
	balancerProviders, err := internalruntime.ResolveNamedRuntimeCapabilities[balancer.Provider](
		a.hub,
		bindings,
		"transport.balancer.provider",
		balancerProviderCapabilitySpec,
	)
	if err != nil {
		return err
	}
	maps.Copy(next.BalancerProviders, balancerProviders)
	next.BalancerProviders["round_robin"] = balancer.BuiltinProvider()
	return nil
}

func (a *App) configureTransportProviders(
	next *Snapshot,
	resolved settings.Resolved,
	bindings map[string][]string,
) error {
	serverProviders, err := internalruntime.ResolveNamedRuntimeCapabilities[remote.TransportServerProvider](
		a.hub,
		bindings,
		"transport.server.provider",
		transportServerProviderCapabilitySpec,
	)
	if err != nil {
		return err
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
		return err
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
	return nil
}

type builtinInterceptorProviders struct {
	unaryServer  map[string]interceptor.UnaryServerInterceptorProvider
	streamServer map[string]interceptor.StreamServerInterceptorProvider
	unaryClient  map[string]interceptor.UnaryClientInterceptorProvider
	streamClient map[string]interceptor.StreamClientInterceptorProvider
}

func loadBuiltinInterceptors(
	logger *slog.Logger,
	resolved settings.Resolved,
) (builtinInterceptorProviders, error) {
	loggingCfg := internalruntime.LoggingInterceptorSource(resolved)
	unaryServerProvidersBuiltin, err := intlogging.BuiltinUnaryServerProvidersWithConfigE(
		loggingCfg,
		intlogging.WithProviderLogger(logger),
	)
	if err != nil {
		return builtinInterceptorProviders{}, err
	}
	streamServerProvidersBuiltin, err := intlogging.BuiltinStreamServerProvidersWithConfigE(
		loggingCfg,
		intlogging.WithProviderLogger(logger),
	)
	if err != nil {
		return builtinInterceptorProviders{}, err
	}
	unaryClientProvidersBuiltin, err := intlogging.BuiltinUnaryClientProvidersWithConfigE(
		loggingCfg,
		intlogging.WithProviderLogger(logger),
	)
	if err != nil {
		return builtinInterceptorProviders{}, err
	}
	streamClientProvidersBuiltin, err := intlogging.BuiltinStreamClientProvidersWithConfigE(
		loggingCfg,
		intlogging.WithProviderLogger(logger),
	)
	if err != nil {
		return builtinInterceptorProviders{}, err
	}
	return builtinInterceptorProviders{
		unaryServer:  internalruntime.MapNamedProviders(unaryServerProvidersBuiltin),
		streamServer: internalruntime.MapNamedProviders(streamServerProvidersBuiltin),
		unaryClient:  internalruntime.MapNamedProviders(unaryClientProvidersBuiltin),
		streamClient: internalruntime.MapNamedProviders(streamClientProvidersBuiltin),
	}, nil
}

func (a *App) configureInterceptors(
	next *Snapshot,
	bindings map[string][]string,
	builtins builtinInterceptorProviders,
) error {
	if err := copyOrderedRuntimeCapabilities(
		a.hub,
		next.UnaryServerInterceptorProviders,
		bindings,
		"rpc.interceptor.unary_server",
		unaryServerInterceptorCapabilitySpec,
		builtins.unaryServer,
	); err != nil {
		return err
	}
	if err := copyOrderedRuntimeCapabilities(
		a.hub,
		next.StreamServerInterceptorProviders,
		bindings,
		"rpc.interceptor.stream_server",
		streamServerInterceptorCapabilitySpec,
		builtins.streamServer,
	); err != nil {
		return err
	}
	if err := copyOrderedRuntimeCapabilities(
		a.hub,
		next.UnaryClientInterceptorProviders,
		bindings,
		"rpc.interceptor.unary_client",
		unaryClientInterceptorCapabilitySpec,
		builtins.unaryClient,
	); err != nil {
		return err
	}
	if err := copyOrderedRuntimeCapabilities(
		a.hub,
		next.StreamClientInterceptorProviders,
		bindings,
		"rpc.interceptor.stream_client",
		streamClientInterceptorCapabilitySpec,
		builtins.streamClient,
	); err != nil {
		return err
	}
	return nil
}

func copyOrderedRuntimeCapabilities[T any](
	hub *module.Hub,
	dst map[string]T,
	bindings map[string][]string,
	capability string,
	spec module.CapabilitySpec,
	preferred map[string]T,
) error {
	providers, err := internalruntime.ResolveOrderedCapabilityMap[T](
		hub,
		bindings[capability],
		spec,
	)
	if err != nil {
		return err
	}
	maps.Copy(dst, providers)
	maps.Copy(dst, preferred)
	return nil
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
