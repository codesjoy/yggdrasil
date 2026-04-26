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

package settings

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	"github.com/codesjoy/yggdrasil/v3/observability/logger"
	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	grpcprotocol "github.com/codesjoy/yggdrasil/v3/transport/protocol/grpc"
	rpchttp "github.com/codesjoy/yggdrasil/v3/transport/protocol/rpchttp"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/client"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/client/balancer"
)

// Compile normalizes the raw framework root into per-module resolved settings.
func Compile(root Root) (Resolved, error) {
	fw := root.Yggdrasil
	resolved := Resolved{
		Root:      root,
		App:       fw.App,
		Mode:      fw.Mode,
		Overrides: fw.Overrides,
		Server:    fw.Server,
		Logging:   fw.Observability.Logging,
		Discovery: fw.Discovery,
		Balancers: fw.Balancers,
		Telemetry: fw.Observability.Telemetry,
		Admin:     fw.Admin,
		Extensions: Extensions{
			Interceptors: fw.Extensions.Interceptors,
			Middleware:   fw.Extensions.Middleware,
		},
		Clients: client.Settings{
			Services: map[string]client.ServiceSettings{},
		},
		Transports: ResolvedTransports{
			GRPC: grpcprotocol.Settings{
				Client:         fw.Transports.GRPC.Client,
				ClientServices: map[string]grpcprotocol.ClientConfig{},
				Server:         fw.Transports.GRPC.Server,
			},
			HTTP: rpchttp.Settings{
				Client:         fw.Transports.HTTP.Client,
				ClientServices: map[string]rpchttp.ClientConfig{},
				Server:         fw.Transports.HTTP.Server,
			},
			Rest:             fw.Transports.HTTP.Rest,
			SecurityProfiles: cloneSecurityProfiles(fw.Transports.Security.Profiles),
		},
		ModuleViews: map[string]string{
			"logging":                 "yggdrasil.observability.logging",
			"telemetry":               "yggdrasil.observability",
			"telemetry.stats":         "yggdrasil.observability.telemetry.stats",
			"transport.security":      "yggdrasil.transports.security",
			"transport.marshaler":     "yggdrasil.transports.http.rest.marshaler",
			"server.transports":       "yggdrasil.server.transports",
			"extensions.interceptors": "yggdrasil.extensions.interceptors",
			"server.rest.middleware":  "yggdrasil.extensions.middleware",
			"discovery.registry":      "yggdrasil.discovery.registry",
			"discovery.resolvers":     "yggdrasil.discovery.resolvers",
			"discovery.balancers":     "yggdrasil.balancers",
		},
		CapabilityBindings: map[string][]string{},
	}

	normalizeLogging(&resolved.Logging)
	ensureCollections(&resolved)

	if items, ok := normalizeExtensionOrderList(fw.Extensions.Interceptors.UnaryServer); ok {
		resolved.Server.Interceptors.Unary = items
		resolved.OrderedExtensions.UnaryServer = append([]string(nil), items...)
	}
	if items, ok := normalizeExtensionOrderList(fw.Extensions.Interceptors.StreamServer); ok {
		resolved.Server.Interceptors.Stream = items
		resolved.OrderedExtensions.StreamServer = append([]string(nil), items...)
	}

	defaultClientSettings := fw.Clients.Defaults.ServiceSettings
	if items, ok := normalizeExtensionOrderList(fw.Extensions.Interceptors.UnaryClient); ok {
		defaultClientSettings.Interceptors.Unary = items
		resolved.OrderedExtensions.UnaryClient = append([]string(nil), items...)
	}
	if items, ok := normalizeExtensionOrderList(fw.Extensions.Interceptors.StreamClient); ok {
		defaultClientSettings.Interceptors.Stream = items
		resolved.OrderedExtensions.StreamClient = append([]string(nil), items...)
	}

	resolved.Server.RestEnabled = resolved.Transports.Rest != nil
	for serviceName, spec := range fw.Clients.Services {
		resolved.Clients.Services[serviceName] = mergeClientServiceSettings(
			defaultClientSettings,
			spec.ServiceSettings,
		)
		resolved.Transports.GRPC.ClientServices[serviceName] = mergeGRPCClientConfig(
			fw.Transports.GRPC.Client,
			spec.Transports.GRPC.Config,
		)
		resolved.Transports.HTTP.ClientServices[serviceName] = mergeHTTPClientConfig(
			fw.Transports.HTTP.Client,
			spec.Transports.HTTP,
		)
	}
	if resolved.Transports.Rest != nil {
		if items, ok := normalizeExtensionOrderList(fw.Extensions.Middleware.RestAll); ok {
			resolved.Transports.Rest.Middleware.All = items
			resolved.OrderedExtensions.RestAll = append([]string(nil), items...)
		}
		if items, ok := normalizeExtensionOrderList(fw.Extensions.Middleware.RestRPC); ok {
			resolved.Transports.Rest.Middleware.RPC = items
			resolved.OrderedExtensions.RestRPC = append([]string(nil), items...)
		}
		if items, ok := normalizeExtensionOrderList(fw.Extensions.Middleware.RestWeb); ok {
			resolved.Transports.Rest.Middleware.Web = items
			resolved.OrderedExtensions.RestWeb = append([]string(nil), items...)
		}
	}

	if err := validateSecurityProfileReferences(resolved); err != nil {
		return Resolved{}, err
	}

	resolved.CapabilityBindings = compileCapabilityBindings(resolved)

	return resolved, nil
}

func normalizeLogging(logging *logger.Settings) {
	if logging.Handlers == nil {
		logging.Handlers = map[string]logger.HandlerSpec{}
	}
	if logging.Writers == nil {
		logging.Writers = map[string]logger.WriterSpec{}
	}
	if logging.Interceptors == nil {
		logging.Interceptors = map[string]map[string]any{}
	}
	defaultHandler := logging.Handlers["default"]
	if defaultHandler.Type == "" {
		defaultHandler.Type = "text"
	}
	if defaultHandler.Writer == "" {
		defaultHandler.Writer = "default"
	}
	logging.Handlers["default"] = defaultHandler
	if _, ok := logging.Writers["default"]; !ok {
		logging.Writers["default"] = logger.WriterSpec{Type: "console"}
	}
	if logging.RemoteLevel == "" {
		logging.RemoteLevel = "error"
	}
}

func ensureCollections(resolved *Resolved) {
	if resolved.Discovery.Resolvers == nil {
		resolved.Discovery.Resolvers = map[string]resolver.Spec{}
	}
	if resolved.Balancers.Defaults == nil {
		resolved.Balancers.Defaults = map[string]balancer.Spec{}
	}
	if resolved.Balancers.Services == nil {
		resolved.Balancers.Services = map[string]map[string]balancer.Spec{}
	}
	if resolved.Root.Yggdrasil.Clients.Services == nil {
		resolved.Root.Yggdrasil.Clients.Services = map[string]ClientServiceSpec{}
	}
	if resolved.Transports.SecurityProfiles == nil {
		resolved.Transports.SecurityProfiles = map[string]SecurityProfileSpec{}
	}
}

func cloneSecurityProfiles(src map[string]SecurityProfileSpec) map[string]SecurityProfileSpec {
	if src == nil {
		return map[string]SecurityProfileSpec{}
	}
	out := make(map[string]SecurityProfileSpec, len(src))
	for name, spec := range src {
		out[name] = SecurityProfileSpec{
			Type:   spec.Type,
			Config: cloneAnyMap(spec.Config),
		}
	}
	return out
}

func normalizeOrderList(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return slices.Clip(out)
}

func normalizeExtensionOrderList(raw any) ([]string, bool) {
	switch value := raw.(type) {
	case nil:
		return nil, false
	case []string:
		return normalizeOrderList(value), true
	case []any:
		items := make([]string, 0, len(value))
		for _, item := range value {
			str, ok := item.(string)
			if !ok {
				return nil, false
			}
			items = append(items, str)
		}
		return normalizeOrderList(items), true
	case string:
		if strings.TrimSpace(value) == "" {
			return nil, false
		}
		return nil, false
	case map[string]any:
		return nil, false
	default:
		return nil, false
	}
}

func compileCapabilityBindings(resolved Resolved) map[string][]string {
	out := map[string][]string{}
	out["observability.logger.handler"] = sortedHandlerTypes(resolved.Logging.Handlers)
	out["observability.logger.writer"] = sortedWriterTypes(resolved.Logging.Writers)
	if resolved.Telemetry.Tracer != "" {
		out["observability.otel.tracer_provider"] = []string{resolved.Telemetry.Tracer}
	}
	if resolved.Telemetry.Meter != "" {
		out["observability.otel.meter_provider"] = []string{resolved.Telemetry.Meter}
	}
	statsNames := dedupStrings(append(
		stats.ParseHandlerNames(resolved.Telemetry.Stats.Server),
		stats.ParseHandlerNames(resolved.Telemetry.Stats.Client)...,
	))
	if len(statsNames) > 0 {
		out["observability.stats.handler"] = statsNames
	}
	securityTypes := map[string]struct{}{}
	for _, spec := range resolved.Transports.SecurityProfiles {
		if spec.Type == "" {
			continue
		}
		securityTypes[spec.Type] = struct{}{}
	}
	if len(securityTypes) > 0 {
		names := make([]string, 0, len(securityTypes))
		for name := range securityTypes {
			names = append(names, name)
		}
		sort.Strings(names)
		out["security.profile.provider"] = names
	}
	if resolved.Transports.Rest != nil {
		schemes := slices.Clone(resolved.Transports.Rest.Marshaler.Support)
		if len(schemes) == 0 {
			schemes = []string{"jsonpb"}
		}
		out["marshaler.scheme"] = dedupStrings(schemes)
	}

	serverProtocols := dedupStrings(append([]string(nil), resolved.Server.Transports...))
	if len(serverProtocols) > 0 {
		out["transport.server.provider"] = serverProtocols
	}

	clientProtocols := []string{grpcprotocol.Protocol, rpchttp.Protocol}
	for _, cfg := range resolved.Clients.Services {
		for _, endpoint := range cfg.Remote.Endpoints {
			if endpoint.Protocol != "" {
				clientProtocols = append(clientProtocols, endpoint.Protocol)
			}
		}
	}
	out["transport.client.provider"] = dedupStrings(clientProtocols)

	out["rpc.interceptor.unary_server"] = dedupStrings(
		append([]string(nil), resolved.Server.Interceptors.Unary...),
	)
	out["rpc.interceptor.stream_server"] = dedupStrings(
		append([]string(nil), resolved.Server.Interceptors.Stream...),
	)
	clientUnary := append(
		[]string(nil),
		resolved.Root.Yggdrasil.Clients.Defaults.Interceptors.Unary...)
	clientStream := append(
		[]string(nil),
		resolved.Root.Yggdrasil.Clients.Defaults.Interceptors.Stream...)
	for _, cfg := range resolved.Clients.Services {
		clientUnary = append(clientUnary, cfg.Interceptors.Unary...)
		clientStream = append(clientStream, cfg.Interceptors.Stream...)
	}
	out["rpc.interceptor.unary_client"] = dedupStrings(clientUnary)
	out["rpc.interceptor.stream_client"] = dedupStrings(clientStream)

	restMiddlewares := []string{"marshaler"}
	if resolved.Transports.Rest != nil {
		restMiddlewares = append(restMiddlewares, resolved.Transports.Rest.Middleware.All...)
		restMiddlewares = append(restMiddlewares, resolved.Transports.Rest.Middleware.RPC...)
		restMiddlewares = append(restMiddlewares, resolved.Transports.Rest.Middleware.Web...)
	}
	out["transport.rest.middleware"] = dedupStrings(restMiddlewares)

	if resolved.Discovery.Registry.Type != "" {
		out["discovery.registry.provider"] = []string{resolved.Discovery.Registry.Type}
	}
	resolverTypes := make([]string, 0, len(resolved.Discovery.Resolvers))
	for _, spec := range resolved.Discovery.Resolvers {
		if spec.Type == "" {
			continue
		}
		resolverTypes = append(resolverTypes, spec.Type)
	}
	out["discovery.resolver.provider"] = dedupStrings(resolverTypes)

	balancerTypes := []string{balancer.DefaultBalancerType}
	for _, spec := range resolved.Balancers.Defaults {
		if spec.Type != "" {
			balancerTypes = append(balancerTypes, spec.Type)
		}
	}
	for _, items := range resolved.Balancers.Services {
		for _, spec := range items {
			if spec.Type != "" {
				balancerTypes = append(balancerTypes, spec.Type)
			}
		}
	}
	out["transport.balancer.provider"] = dedupStrings(balancerTypes)
	return out
}

func validateSecurityProfileReferences(resolved Resolved) error {
	defined := resolved.Transports.SecurityProfiles
	check := func(name, key string) error {
		if strings.TrimSpace(name) == "" {
			return nil
		}
		if _, ok := defined[name]; ok {
			return nil
		}
		return fmt.Errorf("%s references undefined security profile %q", key, name)
	}
	if err := check(
		resolved.Transports.GRPC.Server.SecurityProfile,
		"yggdrasil.transports.grpc.server.security_profile",
	); err != nil {
		return err
	}
	if err := check(
		resolved.Transports.GRPC.Client.Transport.SecurityProfile,
		"yggdrasil.transports.grpc.client.transport.security_profile",
	); err != nil {
		return err
	}
	for serviceName, cfg := range resolved.Transports.GRPC.ClientServices {
		if err := check(
			cfg.Transport.SecurityProfile,
			"yggdrasil.clients.services."+serviceName+".transports.grpc.transport.security_profile",
		); err != nil {
			return err
		}
	}
	if err := check(
		resolved.Transports.HTTP.Server.SecurityProfile,
		"yggdrasil.transports.http.server.security_profile",
	); err != nil {
		return err
	}
	if err := check(
		resolved.Transports.HTTP.Client.SecurityProfile,
		"yggdrasil.transports.http.client.security_profile",
	); err != nil {
		return err
	}
	for serviceName, cfg := range resolved.Transports.HTTP.ClientServices {
		if err := check(
			cfg.SecurityProfile,
			"yggdrasil.clients.services."+serviceName+".transports.http.security_profile",
		); err != nil {
			return err
		}
	}
	return nil
}

func sortedHandlerTypes(in map[string]logger.HandlerSpec) []string {
	keys := make([]string, 0, len(in))
	for _, spec := range in {
		if spec.Type == "" {
			continue
		}
		keys = append(keys, spec.Type)
	}
	if len(keys) == 0 {
		keys = append(keys, "text")
	}
	sort.Strings(keys)
	return dedupStrings(keys)
}

func sortedWriterTypes(in map[string]logger.WriterSpec) []string {
	keys := make([]string, 0, len(in))
	for _, spec := range in {
		if spec.Type == "" {
			continue
		}
		keys = append(keys, spec.Type)
	}
	if len(keys) == 0 {
		keys = append(keys, "console")
	}
	sort.Strings(keys)
	return dedupStrings(keys)
}
