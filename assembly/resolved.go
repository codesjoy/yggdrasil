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

package assembly

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/observability/logger"
	"github.com/codesjoy/yggdrasil/v3/remote/transport/grpc"
	"github.com/codesjoy/yggdrasil/v3/remote/transport/rpchttp"
)

func (p *planner) buildEffectiveResolved() (settings.Resolved, error) {
	effective, err := cloneResolved(p.input.Resolved)
	if err != nil {
		return settings.Resolved{}, err
	}

	if value, ok := p.selectedDefaults[capLoggerHandler]; ok {
		spec := effective.Logging.Handlers["default"]
		spec.Type = value
		effective.Logging.Handlers["default"] = spec
	}
	if value, ok := p.selectedDefaults[capLoggerWriter]; ok {
		spec := effective.Logging.Writers["default"]
		spec.Type = value
		effective.Logging.Writers["default"] = spec
	}
	if value, ok := p.selectedDefaults[capTracer]; ok {
		effective.Telemetry.Tracer = value
	}
	if value, ok := p.selectedDefaults[capMeter]; ok {
		effective.Telemetry.Meter = value
	}
	if value, ok := p.selectedDefaults[capRegistry]; ok {
		effective.Discovery.Registry.Type = value
	}

	applyServerChain(&effective, chainUnaryServer, p.selectedChains[chainUnaryServer])
	applyServerChain(&effective, chainStreamServer, p.selectedChains[chainStreamServer])
	applyClientChain(&effective, chainUnaryClient, p.selectedChains[chainUnaryClient])
	applyClientChain(&effective, chainStreamClient, p.selectedChains[chainStreamClient])
	applyRESTChain(&effective, chainRESTAll, p.selectedChains[chainRESTAll])
	applyRESTChain(&effective, chainRESTRPC, p.selectedChains[chainRESTRPC])
	applyRESTChain(&effective, chainRESTWeb, p.selectedChains[chainRESTWeb])
	return effective, nil
}

func cloneResolved(resolved settings.Resolved) (settings.Resolved, error) {
	var root settings.Root
	data, err := json.Marshal(resolved.Root)
	if err != nil {
		return settings.Resolved{}, err
	}
	if err := json.Unmarshal(data, &root); err != nil {
		return settings.Resolved{}, err
	}
	return settings.Compile(root)
}

func applyServerChain(target *settings.Resolved, path string, chain Chain) {
	if target == nil {
		return
	}
	switch path {
	case chainUnaryServer:
		target.Server.Interceptors.Unary = append([]string(nil), chain.Items...)
		target.OrderedExtensions.UnaryServer = append([]string(nil), chain.Items...)
	case chainStreamServer:
		target.Server.Interceptors.Stream = append([]string(nil), chain.Items...)
		target.OrderedExtensions.StreamServer = append([]string(nil), chain.Items...)
	}
}

func applyClientChain(target *settings.Resolved, path string, chain Chain) {
	if target == nil {
		return
	}
	switch path {
	case chainUnaryClient:
		target.OrderedExtensions.UnaryClient = append([]string(nil), chain.Items...)
		target.Root.Yggdrasil.Clients.Defaults.Interceptors.Unary = append([]string(nil), chain.Items...)
		for name, cfg := range target.Clients.Services {
			if len(cfg.Interceptors.Unary) != 0 {
				continue
			}
			cfg.Interceptors.Unary = append([]string(nil), chain.Items...)
			target.Clients.Services[name] = cfg
		}
	case chainStreamClient:
		target.OrderedExtensions.StreamClient = append([]string(nil), chain.Items...)
		target.Root.Yggdrasil.Clients.Defaults.Interceptors.Stream = append([]string(nil), chain.Items...)
		for name, cfg := range target.Clients.Services {
			if len(cfg.Interceptors.Stream) != 0 {
				continue
			}
			cfg.Interceptors.Stream = append([]string(nil), chain.Items...)
			target.Clients.Services[name] = cfg
		}
	}
}

func applyRESTChain(target *settings.Resolved, path string, chain Chain) {
	if target == nil || target.Transports.Rest == nil {
		return
	}
	switch path {
	case chainRESTAll:
		target.Transports.Rest.Middleware.All = append([]string(nil), chain.Items...)
		target.OrderedExtensions.RestAll = append([]string(nil), chain.Items...)
	case chainRESTRPC:
		target.Transports.Rest.Middleware.RPC = append([]string(nil), chain.Items...)
		target.OrderedExtensions.RestRPC = append([]string(nil), chain.Items...)
	case chainRESTWeb:
		target.Transports.Rest.Middleware.Web = append([]string(nil), chain.Items...)
		target.OrderedExtensions.RestWeb = append([]string(nil), chain.Items...)
	}
}

func compileCapabilityBindings(resolved settings.Resolved) map[string][]string {
	out := map[string][]string{}
	out[capLoggerHandler] = sortedHandlerTypes(resolved.Logging.Handlers)
	out[capLoggerWriter] = sortedWriterTypes(resolved.Logging.Writers)
	if resolved.Telemetry.Tracer != "" {
		out[capTracer] = []string{resolved.Telemetry.Tracer}
	}
	if resolved.Telemetry.Meter != "" {
		out[capMeter] = []string{resolved.Telemetry.Meter}
	}
	statsNames := dedupStrings(append(
		parseStatsHandlerNames(resolved.Telemetry.Stats.Server),
		parseStatsHandlerNames(resolved.Telemetry.Stats.Client)...,
	))
	if len(statsNames) > 0 {
		out[capStatsHandler] = statsNames
	}
	credentialNames := map[string]struct{}{}
	if resolved.Transports.GRPC.Server.CredsProto != "" {
		credentialNames[resolved.Transports.GRPC.Server.CredsProto] = struct{}{}
	}
	if resolved.Transports.GRPC.Client.Transport.CredsProto != "" {
		credentialNames[resolved.Transports.GRPC.Client.Transport.CredsProto] = struct{}{}
	}
	for _, cfg := range resolved.Transports.GRPC.ClientServices {
		if cfg.Transport.CredsProto != "" {
			credentialNames[cfg.Transport.CredsProto] = struct{}{}
		}
	}
	for name := range resolved.Transports.GRPCCredentials {
		credentialNames[name] = struct{}{}
	}
	for _, items := range resolved.Transports.GRPCServiceCredentials {
		for name := range items {
			credentialNames[name] = struct{}{}
		}
	}
	if len(credentialNames) > 0 {
		out[capCredentials] = sortedKeys(credentialNames)
	}
	if resolved.Transports.Rest != nil {
		schemes := append([]string(nil), resolved.Transports.Rest.Marshaler.Support...)
		if len(schemes) == 0 {
			schemes = []string{"jsonpb"}
		}
		out[capMarshaler] = dedupStrings(schemes)
	}
	serverProtocols := dedupStrings(append([]string(nil), resolved.Server.Transports...))
	if len(serverProtocols) > 0 {
		out[capServerTrans] = serverProtocols
	}
	clientProtocols := []string{grpc.Protocol, rpchttp.Protocol}
	for _, cfg := range resolved.Clients.Services {
		for _, endpoint := range cfg.Remote.Endpoints {
			if endpoint.Protocol != "" {
				clientProtocols = append(clientProtocols, endpoint.Protocol)
			}
		}
	}
	out[capClientTrans] = dedupStrings(clientProtocols)
	out[capUnaryServer] = dedupStrings(append([]string(nil), resolved.Server.Interceptors.Unary...))
	out[capStreamServer] = dedupStrings(append([]string(nil), resolved.Server.Interceptors.Stream...))
	var clientUnary []string
	var clientStream []string
	for _, cfg := range resolved.Clients.Services {
		clientUnary = append(clientUnary, cfg.Interceptors.Unary...)
		clientStream = append(clientStream, cfg.Interceptors.Stream...)
	}
	if len(clientUnary) > 0 {
		out[capUnaryClient] = dedupStrings(clientUnary)
	}
	if len(clientStream) > 0 {
		out[capStreamClient] = dedupStrings(clientStream)
	}
	restMiddlewares := []string{"marshaler"}
	if resolved.Transports.Rest != nil {
		restMiddlewares = append(restMiddlewares, resolved.Transports.Rest.Middleware.All...)
		restMiddlewares = append(restMiddlewares, resolved.Transports.Rest.Middleware.RPC...)
		restMiddlewares = append(restMiddlewares, resolved.Transports.Rest.Middleware.Web...)
	}
	out[capRESTMW] = dedupStrings(restMiddlewares)
	if resolved.Discovery.Registry.Type != "" {
		out[capRegistry] = []string{resolved.Discovery.Registry.Type}
	}
	var resolverTypes []string
	for _, spec := range resolved.Discovery.Resolvers {
		if spec.Type != "" {
			resolverTypes = append(resolverTypes, spec.Type)
		}
	}
	if len(resolverTypes) > 0 {
		out[capResolver] = dedupStrings(resolverTypes)
	}
	balancerTypes := []string{"round_robin"}
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
	out[capBalancer] = dedupStrings(balancerTypes)
	return out
}

func sortedHandlerTypes(in map[string]logger.HandlerSpec) []string {
	keys := make([]string, 0, len(in))
	for _, spec := range in {
		if spec.Type == "" {
			continue
		}
		keys = append(keys, normalizedHandlerType(spec.Type))
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

func normalizedHandlerType(typeName string) string {
	if typeName == "console" {
		return "text"
	}
	return typeName
}

func parseStatsHandlerNames(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, item := range parts {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}
