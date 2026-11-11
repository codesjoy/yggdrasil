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

	"github.com/codesjoy/yggdrasil/v3/internal/settings"
)

func (p *planner) buildEffectiveResolved() (settings.Resolved, error) {
	effective, err := cloneResolved(p.input.Resolved)
	if err != nil {
		return settings.Resolved{}, err
	}
	declared := cloneBindings(p.input.Resolved.DeclaredCapabilityBindings)

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
	effective.CapabilityBindings = settings.CompileCapabilityBindings(effective)
	effective.DeclaredCapabilityBindings = declared
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
		target.Root.Yggdrasil.Clients.Defaults.Interceptors.Unary = append(
			[]string(nil),
			chain.Items...)
		for name, cfg := range target.Clients.Services {
			if len(cfg.Interceptors.Unary) != 0 {
				continue
			}
			cfg.Interceptors.Unary = append([]string(nil), chain.Items...)
			target.Clients.Services[name] = cfg
		}
	case chainStreamClient:
		target.OrderedExtensions.StreamClient = append([]string(nil), chain.Items...)
		target.Root.Yggdrasil.Clients.Defaults.Interceptors.Stream = append(
			[]string(nil),
			chain.Items...)
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
