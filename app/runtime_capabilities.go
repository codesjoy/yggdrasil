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
	"reflect"
	"sort"

	"github.com/codesjoy/yggdrasil/v3/balancer"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/interceptor"
	intlogging "github.com/codesjoy/yggdrasil/v3/interceptor/logging"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/logger"
	"github.com/codesjoy/yggdrasil/v3/module"
	xotel "github.com/codesjoy/yggdrasil/v3/otel"
	"github.com/codesjoy/yggdrasil/v3/registry"
	"github.com/codesjoy/yggdrasil/v3/remote"
	"github.com/codesjoy/yggdrasil/v3/remote/credentials"
	"github.com/codesjoy/yggdrasil/v3/remote/credentials/insecure"
	"github.com/codesjoy/yggdrasil/v3/remote/credentials/local"
	ytls "github.com/codesjoy/yggdrasil/v3/remote/credentials/tls"
	"github.com/codesjoy/yggdrasil/v3/remote/marshaler"
	grpcprotocol "github.com/codesjoy/yggdrasil/v3/remote/transport/grpc"
	rpchttp "github.com/codesjoy/yggdrasil/v3/remote/transport/rpchttp"
	"github.com/codesjoy/yggdrasil/v3/resolver"
	restmiddleware "github.com/codesjoy/yggdrasil/v3/server/rest/middleware"
	"github.com/codesjoy/yggdrasil/v3/stats"
	statsotel "github.com/codesjoy/yggdrasil/v3/stats/otel"
)

var (
	loggerHandlerCapabilitySpec = module.CapabilitySpec{
		Name:        "logger.handler",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((logger.HandlerBuilder)(nil)),
	}
	loggerWriterCapabilitySpec = module.CapabilitySpec{
		Name:        "logger.writer",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((logger.WriterBuilder)(nil)),
	}
	tracerProviderCapabilitySpec = module.CapabilitySpec{
		Name:        "otel.tracer_provider",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((xotel.TracerProviderBuilder)(nil)),
	}
	meterProviderCapabilitySpec = module.CapabilitySpec{
		Name:        "otel.meter_provider",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((xotel.MeterProviderBuilder)(nil)),
	}
	statsHandlerCapabilitySpec = module.CapabilitySpec{
		Name:        "stats.handler",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((stats.HandlerBuilder)(nil)),
	}
	credentialsCapabilitySpec = module.CapabilitySpec{
		Name:        "credentials.transport",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((credentials.Builder)(nil)),
	}
	marshalerCapabilitySpec = module.CapabilitySpec{
		Name:        "marshaler.scheme",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((marshaler.MarshallerBuilder)(nil)),
	}
	transportServerProviderCapabilitySpec = module.CapabilitySpec{
		Name:        "transport.server.provider",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((*remote.TransportServerProvider)(nil)).Elem(),
	}
	transportClientProviderCapabilitySpec = module.CapabilitySpec{
		Name:        "transport.client.provider",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((*remote.TransportClientProvider)(nil)).Elem(),
	}
	unaryServerInterceptorCapabilitySpec = module.CapabilitySpec{
		Name:        "interceptor.unary_server",
		Cardinality: module.OrderedMany,
		Type:        reflect.TypeOf((*interceptor.UnaryServerInterceptorProvider)(nil)).Elem(),
	}
	streamServerInterceptorCapabilitySpec = module.CapabilitySpec{
		Name:        "interceptor.stream_server",
		Cardinality: module.OrderedMany,
		Type:        reflect.TypeOf((*interceptor.StreamServerInterceptorProvider)(nil)).Elem(),
	}
	unaryClientInterceptorCapabilitySpec = module.CapabilitySpec{
		Name:        "interceptor.unary_client",
		Cardinality: module.OrderedMany,
		Type:        reflect.TypeOf((*interceptor.UnaryClientInterceptorProvider)(nil)).Elem(),
	}
	streamClientInterceptorCapabilitySpec = module.CapabilitySpec{
		Name:        "interceptor.stream_client",
		Cardinality: module.OrderedMany,
		Type:        reflect.TypeOf((*interceptor.StreamClientInterceptorProvider)(nil)).Elem(),
	}
	restMiddlewareCapabilitySpec = module.CapabilitySpec{
		Name:        "rest.middleware",
		Cardinality: module.OrderedMany,
		Type:        reflect.TypeOf((*restmiddleware.Provider)(nil)).Elem(),
	}
	registryProviderCapabilitySpec = module.CapabilitySpec{
		Name:        "registry.provider",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((*registry.Provider)(nil)).Elem(),
	}
	resolverProviderCapabilitySpec = module.CapabilitySpec{
		Name:        "resolver.provider",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((*resolver.Provider)(nil)).Elem(),
	}
	balancerProviderCapabilitySpec = module.CapabilitySpec{
		Name:        "balancer.provider",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((*balancer.Provider)(nil)).Elem(),
	}
)

type foundationBuiltinCapabilityModule struct{}

func (foundationBuiltinCapabilityModule) Name() string { return "foundation.capabilities" }

func (foundationBuiltinCapabilityModule) ConfigPath() string { return "yggdrasil" }

func (foundationBuiltinCapabilityModule) Init(context.Context, config.View) error { return nil }

func appendSortedCapabilities(
	out []module.Capability,
	spec module.CapabilitySpec,
	providers map[string]any,
) []module.Capability {
	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		out = append(out, module.Capability{
			Spec:  spec,
			Name:  name,
			Value: providers[name],
		})
	}
	return out
}

func (foundationBuiltinCapabilityModule) Capabilities() []module.Capability {
	out := make([]module.Capability, 0)

	loggerHandlers := map[string]any{}
	for name, builder := range logger.BuiltinHandlerBuilders() {
		loggerHandlers[name] = builder
	}
	out = appendSortedCapabilities(out, loggerHandlerCapabilitySpec, loggerHandlers)

	loggerWriters := map[string]any{}
	for name, builder := range logger.BuiltinWriterBuilders() {
		loggerWriters[name] = builder
	}
	out = appendSortedCapabilities(out, loggerWriterCapabilitySpec, loggerWriters)

	out = appendSortedCapabilities(out, statsHandlerCapabilitySpec, map[string]any{
		"otel": statsotel.BuiltinHandlerBuilder(),
	})
	out = appendSortedCapabilities(out, credentialsCapabilitySpec, map[string]any{
		"insecure": insecure.BuiltinBuilder(),
		"local":    local.BuiltinBuilder(),
		"tls":      ytls.BuiltinBuilder(),
	})
	out = appendSortedCapabilities(out, marshalerCapabilitySpec, map[string]any{
		"jsonpb": marshaler.JSONPbBuilder(),
		"proto":  marshaler.ProtoBuilder(),
	})

	return out
}

type connectivityBuiltinCapabilityModule struct{}

func (connectivityBuiltinCapabilityModule) Name() string { return "connectivity.capabilities" }

func (connectivityBuiltinCapabilityModule) ConfigPath() string { return "yggdrasil" }

func (connectivityBuiltinCapabilityModule) Init(context.Context, config.View) error { return nil }

func (connectivityBuiltinCapabilityModule) Capabilities() []module.Capability {
	out := make([]module.Capability, 0)

	out = appendSortedCapabilities(out, transportServerProviderCapabilitySpec, map[string]any{
		grpcprotocol.Protocol: grpcprotocol.ServerProvider(),
		rpchttp.Protocol:      rpchttp.ServerProvider(),
	})
	out = appendSortedCapabilities(out, transportClientProviderCapabilitySpec, map[string]any{
		grpcprotocol.Protocol: grpcprotocol.ClientProvider(),
		rpchttp.Protocol:      rpchttp.ClientProvider(),
	})

	unaryServer := map[string]any{}
	for _, item := range intlogging.BuiltinUnaryServerProviders() {
		unaryServer[item.Name()] = item
	}
	out = appendSortedCapabilities(out, unaryServerInterceptorCapabilitySpec, unaryServer)

	streamServer := map[string]any{}
	for _, item := range intlogging.BuiltinStreamServerProviders() {
		streamServer[item.Name()] = item
	}
	out = appendSortedCapabilities(out, streamServerInterceptorCapabilitySpec, streamServer)

	unaryClient := map[string]any{}
	for _, item := range intlogging.BuiltinUnaryClientProviders() {
		unaryClient[item.Name()] = item
	}
	out = appendSortedCapabilities(out, unaryClientInterceptorCapabilitySpec, unaryClient)

	streamClient := map[string]any{}
	for _, item := range intlogging.BuiltinStreamClientProviders() {
		streamClient[item.Name()] = item
	}
	out = appendSortedCapabilities(out, streamClientInterceptorCapabilitySpec, streamClient)

	out = appendSortedCapabilities(out, restMiddlewareCapabilitySpec, map[string]any{
		"logger":    restmiddleware.BuiltinLoggingProvider(),
		"marshaler": restmiddleware.BuiltinMarshalerProvider(),
	})
	out = appendSortedCapabilities(out, registryProviderCapabilitySpec, map[string]any{
		"multi_registry": registry.BuiltinProvider(),
	})
	out = appendSortedCapabilities(out, balancerProviderCapabilitySpec, map[string]any{
		"round_robin": balancer.BuiltinProvider(),
	})

	return out
}

func (a *App) applyConnectivityCapabilities() error {
	return nil
}

func (a *App) applyFoundationCapabilities() error {
	if a == nil || a.hub == nil || a.opts == nil {
		return nil
	}
	resolved := a.opts.resolvedSettings
	if err := configureNamedCapabilityMap(
		resolved,
		"logger.handler",
		loggerHandlerCapabilitySpec,
		func(name string) (logger.HandlerBuilder, error) {
			return module.ResolveNamed[logger.HandlerBuilder](a.hub, loggerHandlerCapabilitySpec, name)
		},
		func(in map[string]logger.HandlerBuilder) { logger.ConfigureHandlerBuilders(in) },
	); err != nil {
		return err
	}
	if err := configureNamedCapabilityMap(
		resolved,
		"logger.writer",
		loggerWriterCapabilitySpec,
		func(name string) (logger.WriterBuilder, error) {
			return module.ResolveNamed[logger.WriterBuilder](a.hub, loggerWriterCapabilitySpec, name)
		},
		func(in map[string]logger.WriterBuilder) { logger.ConfigureWriterBuilders(in) },
	); err != nil {
		return err
	}
	if err := configureNamedCapabilityMap(
		resolved,
		"otel.tracer_provider",
		tracerProviderCapabilitySpec,
		func(name string) (xotel.TracerProviderBuilder, error) {
			return module.ResolveNamed[xotel.TracerProviderBuilder](a.hub, tracerProviderCapabilitySpec, name)
		},
		func(in map[string]xotel.TracerProviderBuilder) { xotel.ConfigureTracerProviderBuilders(in) },
	); err != nil {
		return err
	}
	if err := configureNamedCapabilityMap(
		resolved,
		"otel.meter_provider",
		meterProviderCapabilitySpec,
		func(name string) (xotel.MeterProviderBuilder, error) {
			return module.ResolveNamed[xotel.MeterProviderBuilder](a.hub, meterProviderCapabilitySpec, name)
		},
		func(in map[string]xotel.MeterProviderBuilder) { xotel.ConfigureMeterProviderBuilders(in) },
	); err != nil {
		return err
	}
	if err := configureNamedCapabilityMap(
		resolved,
		"stats.handler",
		statsHandlerCapabilitySpec,
		func(name string) (stats.HandlerBuilder, error) {
			return module.ResolveNamed[stats.HandlerBuilder](a.hub, statsHandlerCapabilitySpec, name)
		},
		func(in map[string]stats.HandlerBuilder) { stats.ConfigureHandlerBuilders(in) },
	); err != nil {
		return err
	}
	if err := configureNamedCapabilityMap(
		resolved,
		"credentials.transport",
		credentialsCapabilitySpec,
		func(name string) (credentials.Builder, error) {
			return module.ResolveNamed[credentials.Builder](a.hub, credentialsCapabilitySpec, name)
		},
		func(in map[string]credentials.Builder) { credentials.ConfigureBuilders(in) },
	); err != nil {
		return err
	}
	if err := configureNamedCapabilityMap(
		resolved,
		"marshaler.scheme",
		marshalerCapabilitySpec,
		func(name string) (marshaler.MarshallerBuilder, error) {
			return module.ResolveNamed[marshaler.MarshallerBuilder](a.hub, marshalerCapabilitySpec, name)
		},
		func(in map[string]marshaler.MarshallerBuilder) { marshaler.ConfigureBuilders(in) },
	); err != nil {
		return err
	}
	xotel.ConfigureDefaultPropagator()
	return nil
}

func configureNamedCapabilityMap[T any](
	resolved settings.Resolved,
	bindingKey string,
	spec module.CapabilitySpec,
	resolve func(name string) (T, error),
	apply func(map[string]T),
) error {
	next, err := resolveNamedCapabilityMap(resolved.CapabilityBindings[bindingKey], spec, resolve)
	if err != nil {
		return err
	}
	apply(next)
	return nil
}
