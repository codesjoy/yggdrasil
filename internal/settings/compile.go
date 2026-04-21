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
	"github.com/codesjoy/yggdrasil/v2/balancer"
	"github.com/codesjoy/yggdrasil/v2/client"
	"github.com/codesjoy/yggdrasil/v2/logger"
	grpcprotocol "github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc"
	protocolhttp "github.com/codesjoy/yggdrasil/v2/remote/protocol/http"
	"github.com/codesjoy/yggdrasil/v2/resolver"
)

// Compile normalizes the raw framework root into per-module resolved settings.
func Compile(root Root) (Resolved, error) {
	fw := root.Yggdrasil
	resolved := Resolved{
		Root:      root,
		Server:    fw.Server,
		Logging:   fw.Logging,
		Discovery: fw.Discovery,
		Balancers: fw.Balancers,
		Telemetry: fw.Telemetry,
		Admin:     fw.Admin,
		Clients: client.Settings{
			Services: map[string]client.ServiceConfig{},
		},
		Transports: ResolvedTransports{
			GRPC: grpcprotocol.Settings{
				Client:         fw.Transports.GRPC.Client,
				ClientServices: map[string]grpcprotocol.Config{},
				Server:         fw.Transports.GRPC.Server,
			},
			HTTP: protocolhttp.Settings{
				Client:         fw.Transports.HTTP.Client,
				ClientServices: map[string]protocolhttp.ClientConfig{},
				Server:         fw.Transports.HTTP.Server,
			},
			Rest:                   fw.Transports.HTTP.Rest,
			GRPCCredentials:        cloneNestedMap(fw.Transports.GRPC.Credentials),
			GRPCServiceCredentials: map[string]map[string]map[string]any{},
		},
	}

	normalizeLogging(&resolved.Logging)
	ensureCollections(&resolved)

	resolved.Server.RestEnabled = resolved.Transports.Rest != nil
	for serviceName, spec := range fw.Clients.Services {
		resolved.Clients.Services[serviceName] = mergeClientServiceConfig(
			fw.Clients.Defaults.ServiceConfig,
			spec.ServiceConfig,
		)
		resolved.Transports.GRPC.ClientServices[serviceName] = mergeGRPCClientConfig(
			fw.Transports.GRPC.Client,
			spec.Transports.GRPC.Config,
		)
		resolved.Transports.HTTP.ClientServices[serviceName] = mergeHTTPClientConfig(
			fw.Transports.HTTP.Client,
			spec.Transports.HTTP,
		)
		if len(spec.Transports.GRPC.Credentials) != 0 {
			resolved.Transports.GRPCServiceCredentials[serviceName] = cloneNestedMap(
				spec.Transports.GRPC.Credentials,
			)
		}
	}

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
}
