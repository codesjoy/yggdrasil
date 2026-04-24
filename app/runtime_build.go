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
	"fmt"

	"github.com/codesjoy/yggdrasil/v3/balancer"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/interceptor"
	intlogging "github.com/codesjoy/yggdrasil/v3/interceptor/logging"
	"github.com/codesjoy/yggdrasil/v3/logger"
	"github.com/codesjoy/yggdrasil/v3/module"
	xotel "github.com/codesjoy/yggdrasil/v3/otel"
	"github.com/codesjoy/yggdrasil/v3/registry"
	"github.com/codesjoy/yggdrasil/v3/remote"
	"github.com/codesjoy/yggdrasil/v3/remote/credentials"
	"github.com/codesjoy/yggdrasil/v3/remote/marshaler"
	grpcprotocol "github.com/codesjoy/yggdrasil/v3/remote/transport/grpc"
	rpchttp "github.com/codesjoy/yggdrasil/v3/remote/transport/rpchttp"
	"github.com/codesjoy/yggdrasil/v3/resolver"
	restmiddleware "github.com/codesjoy/yggdrasil/v3/server/rest/middleware"
	"github.com/codesjoy/yggdrasil/v3/stats"
)

func (a *App) buildFoundationRuntimeSnapshot() (*Snapshot, error) {
	if a == nil || a.hub == nil || a.opts == nil {
		return nil, fmt.Errorf("runtime app is not initialized")
	}
	resolved := effectiveResolved(a.lastPlanResult, a.opts.resolvedSettings)
	bindings := selectedCapabilityBindings(a.lastPlanResult, a.opts.resolvedSettings)

	writerBuilders, err := resolveNamedRuntimeCapabilities[logger.WriterBuilder](
		a.hub,
		bindings,
		"logger.writer",
		loggerWriterCapabilitySpec,
	)
	if err != nil {
		return nil, err
	}
	writerBuilders = bindLoggerWriterBuilders(resolved, writerBuilders)

	handlerBuilders, err := resolveNamedRuntimeCapabilities[logger.HandlerBuilder](
		a.hub,
		bindings,
		"logger.handler",
		loggerHandlerCapabilitySpec,
	)
	if err != nil {
		return nil, err
	}
	handlerBuilders = bindLoggerHandlerBuilders(resolved, handlerBuilders, writerBuilders)

	tracerBuilders, err := resolveNamedRuntimeCapabilities[xotel.TracerProviderBuilder](
		a.hub,
		bindings,
		"otel.tracer_provider",
		tracerProviderCapabilitySpec,
	)
	if err != nil {
		return nil, err
	}
	meterBuilders, err := resolveNamedRuntimeCapabilities[xotel.MeterProviderBuilder](
		a.hub,
		bindings,
		"otel.meter_provider",
		meterProviderCapabilitySpec,
	)
	if err != nil {
		return nil, err
	}
	statsBuilders, err := resolveNamedRuntimeCapabilities[stats.HandlerBuilder](
		a.hub,
		bindings,
		"stats.handler",
		statsHandlerCapabilitySpec,
	)
	if err != nil {
		return nil, err
	}
	statsBuilders = bindStatsHandlerBuilders(resolved, statsBuilders)

	credentialsBuilders, err := resolveNamedRuntimeCapabilities[credentials.Builder](
		a.hub,
		bindings,
		"credentials.transport",
		credentialsCapabilitySpec,
	)
	if err != nil {
		return nil, err
	}
	credentialsBuilders = bindCredentialsBuilders(resolved, credentialsBuilders)

	marshalerBuilders, err := resolveNamedRuntimeCapabilities[marshaler.MarshallerBuilder](
		a.hub,
		bindings,
		"marshaler.scheme",
		marshalerCapabilitySpec,
	)
	if err != nil {
		return nil, err
	}

	return &Snapshot{
		Resolved:                         resolved,
		LoggerHandlerBuilders:            handlerBuilders,
		LoggerWriterBuilders:             writerBuilders,
		TracerProviderBuilders:           tracerBuilders,
		MeterProviderBuilders:            meterBuilders,
		StatsHandlerBuilders:             statsBuilders,
		ServerStats:                      stats.BuildHandlerChainWithBuilders(resolved.Telemetry.Stats, statsBuilders, true),
		ClientStats:                      stats.BuildHandlerChainWithBuilders(resolved.Telemetry.Stats, statsBuilders, false),
		CredentialsBuilders:              credentialsBuilders,
		MarshalerBuilderMap:              marshalerBuilders,
		TransportServerProviders:         map[string]remote.TransportServerProvider{},
		TransportClientProviders:         map[string]remote.TransportClientProvider{},
		UnaryServerInterceptorProviders:  map[string]interceptor.UnaryServerInterceptorProvider{},
		StreamServerInterceptorProviders: map[string]interceptor.StreamServerInterceptorProvider{},
		UnaryClientInterceptorProviders:  map[string]interceptor.UnaryClientInterceptorProvider{},
		StreamClientInterceptorProviders: map[string]interceptor.StreamClientInterceptorProvider{},
		RESTMiddlewareProviderMap:        map[string]restmiddleware.Provider{},
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

	serverProviders, err := resolveNamedRuntimeCapabilities[remote.TransportServerProvider](
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
				next.CredentialsBuilders,
			)
		case rpchttp.Protocol:
			next.TransportServerProviders[name] = rpchttp.ServerProviderWithSettings(
				resolved.Transports.HTTP,
				next.ServerStats,
				next.MarshalerBuilderMap,
			)
		default:
			next.TransportServerProviders[name] = provider
		}
	}

	clientProviders, err := resolveNamedRuntimeCapabilities[remote.TransportClientProvider](
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
				next.CredentialsBuilders,
			)
		case rpchttp.Protocol:
			next.TransportClientProviders[name] = rpchttp.ClientProviderWithSettings(
				resolved.Transports.HTTP,
				next.MarshalerBuilderMap,
			)
		default:
			next.TransportClientProviders[name] = provider
		}
	}

	loggingCfg := loggingInterceptorSource(resolved)
	unaryServerBuiltins := mapUnaryServerProviders(intlogging.BuiltinUnaryServerProvidersWithConfig(loggingCfg))
	streamServerBuiltins := mapStreamServerProviders(intlogging.BuiltinStreamServerProvidersWithConfig(loggingCfg))
	unaryClientBuiltins := mapUnaryClientProviders(intlogging.BuiltinUnaryClientProvidersWithConfig(loggingCfg))
	streamClientBuiltins := mapStreamClientProviders(intlogging.BuiltinStreamClientProvidersWithConfig(loggingCfg))

	unaryServerProviders, err := resolveOrderedRuntimeCapabilities[interceptor.UnaryServerInterceptorProvider](
		a.hub,
		bindings,
		"interceptor.unary_server",
		unaryServerInterceptorCapabilitySpec,
	)
	if err != nil {
		return nil, false, err
	}
	copyPreferredIntoMap(next.UnaryServerInterceptorProviders, unaryServerProviders, unaryServerBuiltins)

	streamServerProviders, err := resolveOrderedRuntimeCapabilities[interceptor.StreamServerInterceptorProvider](
		a.hub,
		bindings,
		"interceptor.stream_server",
		streamServerInterceptorCapabilitySpec,
	)
	if err != nil {
		return nil, false, err
	}
	copyPreferredIntoMap(next.StreamServerInterceptorProviders, streamServerProviders, streamServerBuiltins)

	unaryClientProviders, err := resolveOrderedRuntimeCapabilities[interceptor.UnaryClientInterceptorProvider](
		a.hub,
		bindings,
		"interceptor.unary_client",
		unaryClientInterceptorCapabilitySpec,
	)
	if err != nil {
		return nil, false, err
	}
	copyPreferredIntoMap(next.UnaryClientInterceptorProviders, unaryClientProviders, unaryClientBuiltins)

	streamClientProviders, err := resolveOrderedRuntimeCapabilities[interceptor.StreamClientInterceptorProvider](
		a.hub,
		bindings,
		"interceptor.stream_client",
		streamClientInterceptorCapabilitySpec,
	)
	if err != nil {
		return nil, false, err
	}
	copyPreferredIntoMap(next.StreamClientInterceptorProviders, streamClientProviders, streamClientBuiltins)

	restProviders, err := resolveOrderedRuntimeCapabilities[restmiddleware.Provider](
		a.hub,
		bindings,
		"rest.middleware",
		restMiddlewareCapabilitySpec,
	)
	if err != nil {
		return nil, false, err
	}
	for name, provider := range restProviders {
		switch name {
		case "logger":
			next.RESTMiddlewareProviderMap[name] = restmiddleware.BuiltinLoggingProvider()
		case "marshaler":
			next.RESTMiddlewareProviderMap[name] = newRuntimeMarshalerProvider(next)
		default:
			next.RESTMiddlewareProviderMap[name] = provider
		}
	}

	registryProviders, err := resolveNamedRuntimeCapabilities[registry.Provider](
		a.hub,
		bindings,
		"registry.provider",
		registryProviderCapabilitySpec,
	)
	if err != nil {
		return nil, false, err
	}
	copyIntoMap(next.RegistryProviders, registryProviders)
	if _, ok := next.RegistryProviders["multi_registry"]; ok {
		next.RegistryProviders["multi_registry"] = registry.BuiltinProviderWithFactory(
			func(typeName string, cfg map[string]any) (registry.Registry, error) {
				return next.NewRegistryByType(typeName, cfg)
			},
		)
	}

	resolverProviders, err := resolveNamedRuntimeCapabilities[resolver.Provider](
		a.hub,
		bindings,
		"resolver.provider",
		resolverProviderCapabilitySpec,
	)
	if err != nil {
		return nil, false, err
	}
	copyIntoMap(next.ResolverProviders, resolverProviders)

	balancerProviders, err := resolveNamedRuntimeCapabilities[balancer.Provider](
		a.hub,
		bindings,
		"balancer.provider",
		balancerProviderCapabilitySpec,
	)
	if err != nil {
		return nil, false, err
	}
	copyPreferredIntoMap(next.BalancerProviders, balancerProviders, map[string]balancer.Provider{
		"round_robin": balancer.BuiltinProvider(),
	})

	current := a.currentRuntimeSnapshot()
	return next, runtimeRequiresRestart(current, next), nil
}

type foundationRuntimeModule struct {
	app *App
}

func (m foundationRuntimeModule) Name() string { return "foundation.runtime" }

func (m foundationRuntimeModule) ConfigPath() string { return "yggdrasil" }

func (m foundationRuntimeModule) Init(context.Context, config.View) error {
	next, err := m.app.buildFoundationRuntimeSnapshot()
	if err != nil {
		return err
	}
	m.app.commitFoundationSnapshot(next)
	return nil
}

func (m foundationRuntimeModule) PrepareReload(context.Context, config.View) (module.ReloadCommitter, error) {
	next, err := m.app.buildFoundationRuntimeSnapshot()
	if err != nil {
		return nil, err
	}
	m.app.stageFoundationSnapshot(next)
	return foundationRuntimeCommitter{app: m.app, next: next}, nil
}

type foundationRuntimeCommitter struct {
	app  *App
	next *Snapshot
}

func (c foundationRuntimeCommitter) Commit(context.Context) error {
	if c.app != nil {
		c.app.commitFoundationSnapshot(c.next)
	}
	return nil
}

func (c foundationRuntimeCommitter) Rollback(context.Context) error {
	if c.app != nil {
		c.app.rollbackFoundationSnapshot(c.next)
	}
	return nil
}

type connectivityRuntimeModule struct {
	app *App
}

func (m connectivityRuntimeModule) Name() string { return "connectivity.runtime" }

func (m connectivityRuntimeModule) DependsOn() []string { return []string{"foundation.runtime"} }

func (m connectivityRuntimeModule) ConfigPath() string { return "yggdrasil" }

func (m connectivityRuntimeModule) Init(context.Context, config.View) error {
	next, _, err := m.app.buildRuntimeSnapshot()
	if err != nil {
		return err
	}
	m.app.setRuntimeSnapshot(next)
	return nil
}

func (m connectivityRuntimeModule) PrepareReload(context.Context, config.View) (module.ReloadCommitter, error) {
	next, restartRequired, err := m.app.buildRuntimeSnapshot()
	if err != nil {
		return nil, err
	}
	return connectivityRuntimeCommitter{
		app:             m.app,
		next:            next,
		restartRequired: restartRequired,
	}, nil
}

type connectivityRuntimeCommitter struct {
	app             *App
	next            *Snapshot
	restartRequired bool
}

func (c connectivityRuntimeCommitter) Commit(context.Context) error {
	if c.app == nil {
		return nil
	}
	c.app.setRuntimeSnapshot(c.next)
	if c.restartRequired && c.app.hub != nil {
		c.app.hub.MarkRestartRequired("connectivity.runtime")
	}
	return c.app.applyRuntimeAdapters(c.next)
}

func (connectivityRuntimeCommitter) Rollback(context.Context) error { return nil }
