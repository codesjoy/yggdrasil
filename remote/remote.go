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
	"sync"

	"github.com/codesjoy/yggdrasil/v2/resolver"
	"github.com/codesjoy/yggdrasil/v2/stats"
)

// ClientState is the state of the client.
type ClientState struct {
	Endpoint resolver.Endpoint
	State    State
}

// OnStateChange is the callback function when the state of the client changes.
type OnStateChange func(state ClientState)

// ClientBuilder is the interface that wraps the Build method.
type ClientBuilder func(context.Context, string, resolver.Endpoint, stats.Handler, OnStateChange) (Client, error)

// ServerInfo is the information of the server.
type ServerInfo struct {
	Protocol   string
	Address    string
	Attributes map[string]string
}

// ServerBuilder is the interface that wraps the Build method.
type ServerBuilder func(handle MethodHandle) (Server, error)

var (
	mu            sync.RWMutex
	clientBuilder = map[string]ClientBuilder{}
	serverBuilder = map[string]ServerBuilder{}
)

// RegisterClientBuilder registers a client builder for the given protocol.
func RegisterClientBuilder(protocol string, builder ClientBuilder) {
	mu.Lock()
	defer mu.Unlock()
	clientBuilder[protocol] = builder
}

// GetClientBuilder returns the client builder for the given protocol.
func GetClientBuilder(protocol string) ClientBuilder {
	mu.RLock()
	defer mu.RUnlock()
	builder, ok := clientBuilder[protocol]
	if !ok {
		return nil
	}
	return builder
}

// RegisterServerBuilder registers a server builder for the given protocol.
func RegisterServerBuilder(protocol string, builder ServerBuilder) {
	mu.Lock()
	defer mu.Unlock()
	serverBuilder[protocol] = builder
}

// GetServerBuilder returns the server builder for the given protocol.
func GetServerBuilder(protocol string) ServerBuilder {
	mu.RLock()
	defer mu.RUnlock()
	builder, ok := serverBuilder[protocol]
	if !ok {
		return nil
	}
	return builder
}
