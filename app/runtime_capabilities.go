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
	"strings"

	"github.com/codesjoy/yggdrasil/v3/balancer"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/rpc/interceptor"
	intlogging "github.com/codesjoy/yggdrasil/v3/rpc/interceptor/logging"
	"github.com/codesjoy/yggdrasil/v3/observability/logger"
	"github.com/codesjoy/yggdrasil/v3/module"
	xotel "github.com/codesjoy/yggdrasil/v3/observability/otel"
	"github.com/codesjoy/yggdrasil/v3/discovery/registry"
	"github.com/codesjoy/yggdrasil/v3/remote"
	"github.com/codesjoy/yggdrasil/v3/remote/credentials"
	"github.com/codesjoy/yggdrasil/v3/remote/credentials/insecure"
	"github.com/codesjoy/yggdrasil/v3/remote/credentials/local"
	ytls "github.com/codesjoy/yggdrasil/v3/remote/credentials/tls"
	"github.com/codesjoy/yggdrasil/v3/remote/marshaler"
	grpcprotocol "github.com/codesjoy/yggdrasil/v3/remote/transport/grpc"
	rpchttp "github.com/codesjoy/yggdrasil/v3/remote/transport/rpchttp"
	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	restmiddleware "github.com/codesjoy/yggdrasil/v3/server/rest/middleware"
	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	statsotel "github.com/codesjoy/yggdrasil/v3/observability/stats/otel"
)

var (
	loggerHandlerCapabilitySpec = module.CapabilitySpec{
		Name:        "observability.logger.handler",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((logger.HandlerBuilder)(nil)),
	}
	loggerWriterCapabilitySpec = module.CapabilitySpec{
		Name:        "observability.logger.writer",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((logger.WriterBuilder)(nil)),
	}
	tracerProviderCapabilitySpec = module.CapabilitySpec{
		Name:        "observability.otel.tracer_provider",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((xotel.TracerProviderBuilder)(nil)),
	}
	meterProviderCapabilitySpec = module.CapabilitySpec{
		Name:        "observability.otel.meter_provider",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((xotel.MeterProviderBuilder)(nil)),
	}
	statsHandlerCapabilitySpec = module.CapabilitySpec{
		Name:        "observability.stats.handler",
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
		Name:        "rpc.interceptor.unary_server",
		Cardinality: module.OrderedMany,
		Type:        reflect.TypeOf((*interceptor.UnaryServerInterceptorProvider)(nil)).Elem(),
	}
	streamServerInterceptorCapabilitySpec = module.CapabilitySpec{
		Name:        "rpc.interceptor.stream_server",
		Cardinality: module.OrderedMany,
		Type:        reflect.TypeOf((*interceptor.StreamServerInterceptorProvider)(nil)).Elem(),
	}
	unaryClientInterceptorCapabilitySpec = module.CapabilitySpec{
		Name:        "rpc.interceptor.unary_client",
		Cardinality: module.OrderedMany,
		Type:        reflect.TypeOf((*interceptor.UnaryClientInterceptorProvider)(nil)).Elem(),
	}
	streamClientInterceptorCapabilitySpec = module.CapabilitySpec{
		Name:        "rpc.interceptor.stream_client",
		Cardinality: module.OrderedMany,
		Type:        reflect.TypeOf((*interceptor.StreamClientInterceptorProvider)(nil)).Elem(),
	}
	restMiddlewareCapabilitySpec = module.CapabilitySpec{
		Name:        "rest.middleware",
		Cardinality: module.OrderedMany,
		Type:        reflect.TypeOf((*restmiddleware.Provider)(nil)).Elem(),
	}
	registryProviderCapabilitySpec = module.CapabilitySpec{
		Name:        "discovery.registry.provider",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((*registry.Provider)(nil)).Elem(),
	}
	resolverProviderCapabilitySpec = module.CapabilitySpec{
		Name:        "discovery.resolver.provider",
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

type statsOtelCapabilityModule struct{}

func (statsOtelCapabilityModule) Name() string { return "telemetry.stats.otel" }

func (statsOtelCapabilityModule) ConfigPath() string { return "yggdrasil.telemetry.stats" }

func (statsOtelCapabilityModule) Init(context.Context, config.View) error { return nil }

func (statsOtelCapabilityModule) Capabilities() []module.Capability {
	return appendSortedCapabilities(nil, statsHandlerCapabilitySpec, map[string]any{
		"otel": statsotel.BuiltinHandlerBuilder(),
	})
}

func (statsOtelCapabilityModule) AutoSpec() module.AutoSpec {
	return module.AutoSpec{
		Provides: []module.CapabilitySpec{statsHandlerCapabilitySpec},
		AutoRules: []module.AutoRule{
			configPathAutoRule{
				path:        "yggdrasil.telemetry.stats.server",
				description: "server stats handler configured",
			},
			configPathAutoRule{
				path:        "yggdrasil.telemetry.stats.client",
				description: "client stats handler configured",
			},
			configPathAutoRule{
				path:        "yggdrasil.telemetry.stats.providers.otel",
				description: "otel stats provider configured",
			},
		},
	}
}

type configPathAutoRule struct {
	path        string
	description string
}

func (r configPathAutoRule) Match(ctx module.AutoRuleContext) bool {
	return !ctx.Snapshot.Section(splitConfigPath(r.path)...).Empty()
}

func (r configPathAutoRule) Describe() string {
	if strings.TrimSpace(r.description) == "" {
		return "configured path matched"
	}
	return r.description
}

func (r configPathAutoRule) AffectedPaths() []string {
	if strings.TrimSpace(r.path) == "" {
		return nil
	}
	return []string{r.path}
}

func splitConfigPath(path string) []string {
	parts := strings.Split(path, ".")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
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
