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

// Package remote provides remote functionality for yggdrasil.
package remote

import (
	"context"

	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	"github.com/codesjoy/yggdrasil/v3/observability/stats"
)

// ClientState is the state of the client.
type ClientState struct {
	Endpoint        resolver.Endpoint
	State           State
	ConnectionError error
}

// OnStateChange is the callback function when the state of the client changes.
type OnStateChange func(state ClientState)

// ClientBuilder is the interface that wraps the Build method.
type ClientBuilder func(context.Context, string, resolver.Endpoint, stats.Handler, OnStateChange) (Client, error)

// TransportClientProvider provides client transport construction for one protocol.
type TransportClientProvider interface {
	Protocol() string
	NewClient(context.Context, string, resolver.Endpoint, stats.Handler, OnStateChange) (Client, error)
}

// ServerInfo is the information of the server.
type ServerInfo struct {
	Protocol   string
	Address    string
	Attributes map[string]string
}

// ServerBuilder is the interface that wraps the Build method.
type ServerBuilder func(handle MethodHandle) (Server, error)

// TransportServerProvider provides server transport construction for one protocol.
type TransportServerProvider interface {
	Protocol() string
	NewServer(handle MethodHandle) (Server, error)
}

type transportClientProvider struct {
	protocol string
	builder  ClientBuilder
}

func (p transportClientProvider) Protocol() string { return p.protocol }

func (p transportClientProvider) NewClient(
	ctx context.Context,
	serviceName string,
	endpoint resolver.Endpoint,
	statsHandler stats.Handler,
	onStateChange OnStateChange,
) (Client, error) {
	return p.builder(ctx, serviceName, endpoint, statsHandler, onStateChange)
}

type transportServerProvider struct {
	protocol string
	builder  ServerBuilder
}

func (p transportServerProvider) Protocol() string { return p.protocol }

func (p transportServerProvider) NewServer(handle MethodHandle) (Server, error) {
	return p.builder(handle)
}

// NewTransportClientProvider builds a client transport provider from a builder.
func NewTransportClientProvider(protocol string, builder ClientBuilder) TransportClientProvider {
	return transportClientProvider{
		protocol: protocol,
		builder:  builder,
	}
}

// NewTransportServerProvider builds a server transport provider from a builder.
func NewTransportServerProvider(protocol string, builder ServerBuilder) TransportServerProvider {
	return transportServerProvider{
		protocol: protocol,
		builder:  builder,
	}
}
