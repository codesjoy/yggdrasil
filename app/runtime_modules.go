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

	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/discovery/registry"
	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	"github.com/codesjoy/yggdrasil/v3/module"
	"github.com/codesjoy/yggdrasil/v3/observability/logger"
	xotel "github.com/codesjoy/yggdrasil/v3/observability/otel"
	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	statsotel "github.com/codesjoy/yggdrasil/v3/observability/stats/otel"
	"github.com/codesjoy/yggdrasil/v3/rpc/interceptor"
	intlogging "github.com/codesjoy/yggdrasil/v3/rpc/interceptor/logging"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
	"github.com/codesjoy/yggdrasil/v3/transport/gateway/rest"
	grpcprotocol "github.com/codesjoy/yggdrasil/v3/transport/protocol/grpc"
	rpchttp "github.com/codesjoy/yggdrasil/v3/transport/protocol/rpchttp"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/client/balancer"
	"github.com/codesjoy/yggdrasil/v3/transport/support/marshaler"
	"github.com/codesjoy/yggdrasil/v3/transport/support/security"
	"github.com/codesjoy/yggdrasil/v3/transport/support/security/insecure"
	"github.com/codesjoy/yggdrasil/v3/transport/support/security/local"
	ytls "github.com/codesjoy/yggdrasil/v3/transport/support/security/tls"
)

// --- CapabilitySpec declarations ---

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
	securityProfileCapabilitySpec = module.CapabilitySpec{
		Name:        "security.profile.provider",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((*security.Provider)(nil)).Elem(),
	}
	marshalerCapabilitySpec = module.CapabilitySpec{
		Name:        "marshaler.scheme",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((marshaler.MarshalerBuilder)(nil)),
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
		Name:        "transport.rest.middleware",
		Cardinality: module.OrderedMany,
		Type:        reflect.TypeOf((*rest.Provider)(nil)).Elem(),
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
		Name:        "transport.balancer.provider",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((*balancer.Provider)(nil)).Elem(),
	}
)

// --- Builtin capability modules ---

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

	out = appendSortedCapabilities(out, securityProfileCapabilitySpec, map[string]any{
		"insecure": insecure.BuiltinProvider(),
		"local":    local.BuiltinProvider(),
		"tls":      ytls.BuiltinProvider(),
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
		"logger":    rest.BuiltinLoggingProvider(),
		"marshaler": rest.BuiltinMarshalerProvider(),
	})
	out = appendSortedCapabilities(out, registryProviderCapabilitySpec, map[string]any{
		"multi_registry": registry.BuiltinProvider(),
	})
	out = appendSortedCapabilities(out, balancerProviderCapabilitySpec, map[string]any{
		"round_robin": balancer.BuiltinProvider(),
	})

	return out
}

// --- Module structs ---

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

func (m foundationRuntimeModule) PrepareReload(
	context.Context,
	config.View,
) (module.ReloadCommitter, error) {
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

func (m connectivityRuntimeModule) PrepareReload(
	context.Context,
	config.View,
) (module.ReloadCommitter, error) {
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
