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

package yggdrasil

import (
	"github.com/codesjoy/yggdrasil/v2/balancer"
	"github.com/codesjoy/yggdrasil/v2/client"
	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/governor"
	"github.com/codesjoy/yggdrasil/v2/internal/settings"
	"github.com/codesjoy/yggdrasil/v2/logger"
	"github.com/codesjoy/yggdrasil/v2/registry"
	ytls "github.com/codesjoy/yggdrasil/v2/remote/credentials/tls"
	grpcprotocol "github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc"
	protocolhttp "github.com/codesjoy/yggdrasil/v2/remote/protocol/http"
	"github.com/codesjoy/yggdrasil/v2/remote/rest"
	"github.com/codesjoy/yggdrasil/v2/remote/rest/middleware"
	"github.com/codesjoy/yggdrasil/v2/resolver"
	"github.com/codesjoy/yggdrasil/v2/server"
	"github.com/codesjoy/yggdrasil/v2/stats"
)

func refreshResolvedSettings(opts *options) error {
	catalog := settings.NewCatalog(opts.configManager)
	root, err := catalog.Root().Current()
	if err != nil {
		return err
	}
	resolved, err := settings.Compile(root)
	if err != nil {
		return err
	}
	applyResolvedSettings(opts.configManager, resolved)
	opts.resolvedSettings = resolved
	return nil
}

func applyResolvedSettings(manager *config.Manager, resolved settings.Resolved) {
	logger.Configure(resolved.Logging)
	registry.Configure(resolved.Discovery.Registry)
	resolver.Configure(resolved.Discovery.Resolvers)
	balancer.Configure(resolved.Balancers.Defaults, resolved.Balancers.Services)
	client.Configure(resolved.Clients)
	server.Configure(resolved.Server)
	grpcprotocol.Configure(resolved.Transports.GRPC)
	protocolhttp.Configure(resolved.Transports.HTTP)
	rest.Configure(resolved.Transports.Rest)
	stats.Configure(resolved.Telemetry.Stats)
	governor.Configure(resolved.Admin.Governor, manager)

	var tlsGlobal ytls.BuilderConfig
	if raw, ok := resolved.Transports.GRPCCredentials["tls"]; ok {
		_ = settings.DecodePayload(&tlsGlobal, raw)
	}
	tlsServices := map[string]ytls.BuilderConfig{}
	for serviceName, specs := range resolved.Transports.GRPCServiceCredentials {
		raw, ok := specs["tls"]
		if !ok {
			continue
		}
		cfg := ytls.BuilderConfig{}
		_ = settings.DecodePayload(&cfg, raw)
		tlsServices[serviceName] = cfg
	}
	ytls.Configure(tlsGlobal, tlsServices)

	if resolved.Transports.Rest != nil {
		middleware.ConfigureMarshaler(
			resolved.Transports.Rest.Marshaler.Support,
			resolved.Transports.Rest.Marshaler.Config.JSONPB,
		)
	} else {
		middleware.ConfigureMarshaler(nil, nil)
	}
}
