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
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/codesjoy/yggdrasil/v3/capabilities"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/discovery/registry"
	"github.com/codesjoy/yggdrasil/v3/module"
	"github.com/codesjoy/yggdrasil/v3/observability/logger"
	statsotel "github.com/codesjoy/yggdrasil/v3/observability/stats/otel"
	intlogging "github.com/codesjoy/yggdrasil/v3/rpc/interceptor/logging"
	"github.com/codesjoy/yggdrasil/v3/transport/gateway/rest"
	grpcprotocol "github.com/codesjoy/yggdrasil/v3/transport/protocol/grpc"
	rpchttp "github.com/codesjoy/yggdrasil/v3/transport/protocol/rpchttp"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/client/balancer"
	"github.com/codesjoy/yggdrasil/v3/transport/support/marshaler"
	"github.com/codesjoy/yggdrasil/v3/transport/support/security/insecure"
	"github.com/codesjoy/yggdrasil/v3/transport/support/security/local"
	ytls "github.com/codesjoy/yggdrasil/v3/transport/support/security/tls"
)

// --- CapabilitySpec declarations ---

var (
	loggerHandlerCapabilitySpec           = capabilities.LoggerHandlerSpec
	loggerWriterCapabilitySpec            = capabilities.LoggerWriterSpec
	tracerProviderCapabilitySpec          = capabilities.TracerProviderSpec
	meterProviderCapabilitySpec           = capabilities.MeterProviderSpec
	statsHandlerCapabilitySpec            = capabilities.StatsHandlerSpec
	securityProfileCapabilitySpec         = capabilities.SecurityProfileProviderSpec
	marshalerCapabilitySpec               = capabilities.MarshalerSchemeSpec
	transportServerProviderCapabilitySpec = capabilities.TransportServerProviderSpec
	transportClientProviderCapabilitySpec = capabilities.TransportClientProviderSpec
	unaryServerInterceptorCapabilitySpec  = capabilities.UnaryServerInterceptorSpec
	streamServerInterceptorCapabilitySpec = capabilities.StreamServerInterceptorSpec
	unaryClientInterceptorCapabilitySpec  = capabilities.UnaryClientInterceptorSpec
	streamClientInterceptorCapabilitySpec = capabilities.StreamClientInterceptorSpec
	restMiddlewareCapabilitySpec          = capabilities.RESTMiddlewareSpec
	registryProviderCapabilitySpec        = capabilities.RegistryProviderSpec
	resolverProviderCapabilitySpec        = capabilities.ResolverProviderSpec
	balancerProviderCapabilitySpec        = capabilities.BalancerProviderSpec
)

// --- Builtin capability modules ---

type appLocalStaticModule struct{}

func (appLocalStaticModule) IsolationMode() module.IsolationMode {
	return module.IsolationModeAppLocal
}

func (appLocalStaticModule) Init(context.Context, config.View) error { return nil }

func (appLocalStaticModule) PrepareReload(
	context.Context,
	config.View,
) (module.ReloadCommitter, error) {
	return staticReloadCommitter{}, nil
}

type foundationBuiltinCapabilityModule struct {
	appLocalStaticModule
}

type staticReloadCommitter struct{}

func (staticReloadCommitter) Commit(context.Context) error   { return nil }
func (staticReloadCommitter) Rollback(context.Context) error { return nil }

func (foundationBuiltinCapabilityModule) Name() string { return "foundation.capabilities" }

func (foundationBuiltinCapabilityModule) ConfigPath() string { return "yggdrasil" }

func appendSortedCapabilities(
	out []module.Capability,
	spec module.CapabilitySpec,
	providers map[string]any,
) []module.Capability {
	for _, name := range slices.Sorted(maps.Keys(providers)) {
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

type statsOtelCapabilityModule struct {
	appLocalStaticModule
}

func (statsOtelCapabilityModule) Name() string { return "observability.stats.otel" }

func (statsOtelCapabilityModule) ConfigPath() string {
	return "yggdrasil.observability.telemetry.stats"
}

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
				path:        "yggdrasil.observability.telemetry.stats.server",
				description: "server stats handler configured",
			},
			configPathAutoRule{
				path:        "yggdrasil.observability.telemetry.stats.client",
				description: "client stats handler configured",
			},
			configPathAutoRule{
				path:        "yggdrasil.observability.telemetry.stats.providers.otel",
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

type connectivityBuiltinCapabilityModule struct {
	appLocalStaticModule
}

func (connectivityBuiltinCapabilityModule) Name() string { return "connectivity.capabilities" }

func (connectivityBuiltinCapabilityModule) ConfigPath() string { return "yggdrasil" }

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
	appLocalStaticModule
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
	return &foundationRuntimeCommitter{app: m.app, next: next}, nil
}

type foundationRuntimeCommitter struct {
	mu sync.Mutex

	app      *App
	next     *Snapshot
	previous *Snapshot

	committed bool
	closed    bool
}

func (c *foundationRuntimeCommitter) Commit(context.Context) error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.committed {
		return nil
	}
	if c.app != nil {
		c.previous = c.app.commitFoundationSnapshotTransaction(c.next)
	}
	c.committed = true
	return nil
}

func (c *foundationRuntimeCommitter) Rollback(context.Context) error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	if c.app != nil {
		c.app.rollbackFoundationSnapshotTransaction(c.next, c.previous, c.committed)
	}
	c.closed = true
	return nil
}

type connectivityRuntimeModule struct {
	appLocalStaticModule
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
	preparedAdapters, err := m.app.prepareRuntimeAdapters(next)
	if err != nil {
		return nil, err
	}
	m.app.stageRuntimeAdapters(preparedAdapters)
	return connectivityRuntimeCommitter{
		app:              m.app,
		next:             next,
		preparedAdapters: preparedAdapters,
		restartRequired:  restartRequired,
	}, nil
}

func (m connectivityRuntimeModule) FinalizeReload(context.Context) error {
	return m.app.finalizePendingRuntimeAdapters()
}

type connectivityRuntimeCommitter struct {
	app              *App
	next             *Snapshot
	preparedAdapters *preparedRuntimeAdapters
	restartRequired  bool
}

func (c connectivityRuntimeCommitter) Commit(context.Context) error {
	if c.app == nil {
		return nil
	}
	if err := c.app.activateRuntimeAdapters(c.preparedAdapters); err != nil {
		return err
	}
	if c.restartRequired && c.app.hub != nil {
		c.app.hub.MarkRestartRequired("connectivity.runtime")
	}
	return nil
}

func (c connectivityRuntimeCommitter) Rollback(context.Context) error {
	if c.app == nil {
		return nil
	}
	return c.app.rollbackPendingRuntimeAdapters(c.preparedAdapters)
}
