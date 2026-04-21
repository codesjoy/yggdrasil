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

// Package resolver provides a way to resolve application information.
package resolver

import (
	"fmt"
	"sync"
)

// BaseEndpoint is the base endpoint.
type BaseEndpoint struct {
	Address    string         `mapstructure:"address"`
	Protocol   string         `mapstructure:"protocol"`
	Attributes map[string]any `mapstructure:"attributes"`
}

// Name returns the name of the endpoint.
func (be BaseEndpoint) Name() string {
	return fmt.Sprintf("%s/%s", be.Protocol, be.Address)
}

// GetAddress returns the address of the endpoint.
func (be BaseEndpoint) GetAddress() string {
	return be.Address
}

// GetProtocol returns the protocol of the endpoint.
func (be BaseEndpoint) GetProtocol() string {
	return be.Protocol
}

// GetAttributes returns the attributes of the endpoint.
func (be BaseEndpoint) GetAttributes() map[string]any {
	return be.Attributes
}

// BaseState is the base state.
type BaseState struct {
	Attributes map[string]any
	Endpoints  []Endpoint
}

// GetAttributes returns the attributes of the application.
func (bs BaseState) GetAttributes() map[string]any {
	return bs.Attributes
}

// GetEndpoints returns the list of endpoints.
func (bs BaseState) GetEndpoints() []Endpoint {
	return bs.Endpoints
}

// Endpoint is the endpoint of the application.
type Endpoint interface {
	Name() string
	// GetAddress returns the address of the endpoint.
	GetAddress() string
	// GetProtocol returns the protocol of the endpoint.
	GetProtocol() string
	// GetAttributes returns the attributes of the endpoint.
	GetAttributes() map[string]any
}

// State is the state of the application.
type State interface {
	// GetEndpoints returns the list of endpoints.
	GetEndpoints() []Endpoint
	// GetAttributes returns the attributes of the application.
	GetAttributes() map[string]any
}

// Client defines the interface for a client.
type Client interface {
	// UpdateState updates the state of the client.
	UpdateState(state State)
}

// Resolver defines the interface for a resolver.
type Resolver interface {
	// AddWatch add a watch for the given application.
	AddWatch(string, Client) error
	// DelWatch deletes a watch for the given application.
	DelWatch(string, Client) error
	// Type returns the type of the resolver.
	Type() string
}

// ResolveNower is implemented by resolvers that can trigger an immediate
// re-resolution outside the normal watch/update flow.
type ResolveNower interface {
	ResolveNow()
}

const (
	// DefaultResolverName is the default resolver name
	DefaultResolverName = "default"
	// NoResolverType indicates no dynamic resolver should be used
	NoResolverType = ""
)

// Builder is a function that creates a resolver.
type Builder func(name string) (Resolver, error)

// Spec describes a resolver extension envelope.
type Spec struct {
	Type   string         `mapstructure:"type"`
	Config map[string]any `mapstructure:"config"`
}

var (
	resolver = map[string]Resolver{}
	builder  = map[string]Builder{}
	mu       sync.RWMutex
	specs    = map[string]Spec{}
)

// RegisterBuilder registers a resolver builder.
func RegisterBuilder(typeName string, f func(string) (Resolver, error)) {
	mu.Lock()
	defer mu.Unlock()
	builder[typeName] = f
}

// HasBuilder reports whether a resolver builder exists.
func HasBuilder(typeName string) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, ok := builder[typeName]
	return ok
}

// Configure replaces the configured resolver specs.
func Configure(next map[string]Spec) {
	mu.Lock()
	defer mu.Unlock()
	if next == nil {
		next = map[string]Spec{}
	}
	specs = next
	resolver = map[string]Resolver{}
}

// CurrentSpec returns the configured resolver spec by name.
func CurrentSpec(name string) Spec {
	mu.RLock()
	defer mu.RUnlock()
	return specs[name]
}

// Get returns the resolver by name.
func Get(name string) (Resolver, error) {
	mu.RLock()
	if r, ok := resolver[name]; ok {
		mu.RUnlock()
		return r, nil
	}
	mu.RUnlock()
	mu.Lock()
	defer mu.Unlock()
	if r, ok := resolver[name]; ok {
		return r, nil
	}
	spec := specs[name]
	typeName := spec.Type

	// Handle "default" resolver without configuration
	if typeName == "" {
		if name == DefaultResolverName {
			// Return nil to indicate no dynamic resolver (use static endpoints)
			return nil, nil
		}
		return nil, fmt.Errorf("not found resolver type, name: %s", name)
	}

	f, ok := builder[typeName]
	if !ok {
		return nil, fmt.Errorf("not found resolver builder, type: %s", typeName)
	}
	r, err := f(name)
	if err != nil {
		return nil, err
	}
	// Cache the resolver for future use
	resolver[name] = r
	return r, nil
}
