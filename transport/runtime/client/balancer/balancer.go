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

// Package balancer implements load balancing algorithms for client requests.
package balancer

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
)

// ErrNoAvailableInstance is returned when no available instance is found
var ErrNoAvailableInstance = fmt.Errorf("no available instance")

const (
	// DefaultBalancerName is the default balancer name used when not specified
	DefaultBalancerName = "default"
	// DefaultBalancerType is the default balancer type fallback
	DefaultBalancerType = "round_robin"
)

// RPCInfo contains information about the RPC request
type RPCInfo struct {
	// Context of the RPC request
	Ctx context.Context
	// Method of the RPC request
	Method string
}

// PickResult contains the result of the pick operation
type PickResult interface {
	RemoteClient() remote.Client
	// Report reports the result of the RPC request
	Report(err error)
}

// Picker is the interface that wraps the Next method
type Picker interface {
	// Next returns the next instance to be picked
	Next(RPCInfo) (PickResult, error)
}

// Balancer is the interface that wraps the GetPicker method
type Balancer interface {
	// UpdateState updates the balancer
	UpdateState(resolver.State)
	// Close closes the balancer
	Close() error
	// Type returns the type of the balancer.
	Type() string
}

// State is the state of the balancer
type State struct {
	ConnectivityState remote.State
	Picker            Picker
}

// NewRemoteClientOptions is the options for NewRemoteClient
type NewRemoteClientOptions struct {
	// StateListener is called when the state of the subconn changes.  If nil,
	// Balancer.UpdateSubConnState will be called instead.  Will never be
	// invoked until after Connect() is called on the SubConn created with
	// these options.
	StateListener func(remote.ClientState)
}

// Client is the interface that wraps the UpdateState method
type Client interface {
	// UpdateState updates the state of the client
	UpdateState(state State)
	// NewRemoteClient creates a new remote client
	NewRemoteClient(endpoint resolver.Endpoint, ops NewRemoteClientOptions) (remote.Client, error)
}

// Builder is the function that creates a balancer.
type Builder func(serviceName, balancerName string, cli Client) (Balancer, error)

// Provider is the capability-oriented balancer provider.
type Provider interface {
	Type() string
	New(serviceName, balancerName string, cli Client) (Balancer, error)
}

type provider struct {
	typeName string
	builder  Builder
}

func (p provider) Type() string { return p.typeName }

func (p provider) New(serviceName, balancerName string, cli Client) (Balancer, error) {
	return p.builder(serviceName, balancerName, cli)
}

// NewProvider wraps a builder as a capability provider.
func NewProvider(typeName string, builder Builder) Provider {
	return provider{
		typeName: typeName,
		builder:  builder,
	}
}

var (
	providers = defaultProviders()
	mu        sync.RWMutex
)

// ConfigureProviders replaces all balancer providers.
func ConfigureProviders(next []Provider) error {
	target := map[string]Provider{}
	for _, item := range next {
		if item == nil {
			continue
		}
		typeName := item.Type()
		if typeName == "" {
			return errors.New("balancer provider type is empty")
		}
		if _, exists := target[typeName]; exists {
			return fmt.Errorf("duplicate balancer provider for type %q", typeName)
		}
		target[typeName] = item
	}
	mu.Lock()
	providers = target
	mu.Unlock()
	return nil
}

// GetProvider returns a balancer provider by type.
func GetProvider(typeName string) (Provider, bool) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := providers[typeName]
	return p, ok
}
