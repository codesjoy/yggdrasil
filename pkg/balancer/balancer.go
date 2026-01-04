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
	"fmt"
	"sync"

	"github.com/codesjoy/yggdrasil/pkg/remote"
	"github.com/codesjoy/yggdrasil/pkg/resolver"
)

// ErrNoAvailableInstance is returned when no available instance is found
var ErrNoAvailableInstance = fmt.Errorf("no available instance")

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
	// Name returns the name of the balancer
	Name() string
}

// State is the state of the balancer
type State struct {
	Picker Picker
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

// Builder is the function that creates a balancer
type Builder func(string, Client) (Balancer, error)

var (
	builder = map[string]Builder{}
	mu      sync.RWMutex
)

// GetBuilder returns the balancer builder
func GetBuilder(name string) (Builder, error) {
	mu.RLock()
	defer mu.RUnlock()
	f, ok := builder[name]
	if !ok {
		return nil, fmt.Errorf("not found balancer builder, name: %s", name)
	}
	return f, nil
}

// RegisterBuilder registers a balancer builder
func RegisterBuilder(name string, f Builder) {
	mu.Lock()
	defer mu.Unlock()
	builder[name] = f
}
