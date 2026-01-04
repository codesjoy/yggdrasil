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
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/codesjoy/yggdrasil/pkg/remote"
	"github.com/codesjoy/yggdrasil/pkg/resolver"
	"github.com/codesjoy/yggdrasil/pkg/status"
	"google.golang.org/genproto/googleapis/rpc/code"
)

const name = "round_robin"

func init() {
	RegisterBuilder(name, newRoundRobin)
}

type rrBalancer struct {
	cli Client

	mu            sync.RWMutex
	remotesClient map[string]remote.Client
}

func newRoundRobin(_ string, cli Client) (Balancer, error) {
	return &rrBalancer{
		cli:           cli,
		remotesClient: make(map[string]remote.Client),
	}, nil
}

// UpdateState updates the balancer state.
func (b *rrBalancer) UpdateState(state resolver.State) {
	b.mu.Lock()
	// Check if balancer is closed
	if b.remotesClient == nil {
		b.mu.Unlock()
		return
	}
	remoteCli := make(map[string]remote.Client, len(state.GetEndpoints()))
	for _, item := range state.GetEndpoints() {
		if cli, ok := b.remotesClient[item.Name()]; ok {
			remoteCli[item.Name()] = cli
			continue
		}
		cli, err := b.cli.NewRemoteClient(
			item,
			NewRemoteClientOptions{StateListener: b.UpdateRemoteClientState},
		)
		if err != nil {
			slog.Error("new remote client error", slog.Any("error", err))
			continue
		}
		if cli != nil {
			remoteCli[item.Name()] = cli
			// Start connection for new client
			cli.Connect()
		}
	}
	needDelClients := make([]remote.Client, 0)
	for key := range b.remotesClient {
		if rc, ok := remoteCli[key]; !ok {
			needDelClients = append(needDelClients, rc)
		}
	}
	// Update the connection map before generating picker
	b.remotesClient = remoteCli
	picker := b.buildPicker()
	b.mu.Unlock()

	// Call UpdateState outside of lock to avoid potential deadlock
	b.cli.UpdateState(State{Picker: picker})

	// Remove old clients through ConnManager outside of lock
	for _, rc := range needDelClients {
		if err := rc.Close(); err != nil {
			slog.Warn(
				"remove remote client error",
				slog.String("name", name),
				slog.Any("error", err),
			)
		}
	}
}

// UpdateRemoteClientState updates the state of a remote client.
func (b *rrBalancer) UpdateRemoteClientState(_ remote.ClientState) {
	b.mu.RLock()
	// Check if balancer is closed
	if b.remotesClient == nil {
		b.mu.RUnlock()
		return
	}
	picker := b.buildPicker()
	b.mu.RUnlock()
	// Call UpdateState outside of lock to avoid potential deadlock
	b.cli.UpdateState(State{Picker: picker})
}

// Close closes all managed connections.
func (b *rrBalancer) Close() error {
	b.mu.Lock()
	clients := make([]remote.Client, 0, len(b.remotesClient))
	for _, cli := range b.remotesClient {
		clients = append(clients, cli)
	}
	b.remotesClient = nil
	picker := b.buildPicker()
	b.mu.Unlock()
	b.cli.UpdateState(State{Picker: picker})
	var multiErr error
	for _, cli := range clients {
		if err := cli.Close(); err != nil {
			multiErr = errors.Join(multiErr, err)
		}
	}
	return multiErr
}

// Name returns the name of the balancer.
func (b *rrBalancer) Name() string {
	return name
}

// buildPicker creates a new picker based on current ready clients
// Must be called with at least a read lock held
func (b *rrBalancer) buildPicker() *rrPicker {
	picker := &rrPicker{endpoint: make([]remote.Client, 0, len(b.remotesClient))}
	for _, item := range b.remotesClient {
		if item.State() != remote.Ready {
			continue
		}
		picker.endpoint = append(picker.endpoint, item)
	}
	return picker
}

type rrPicker struct {
	idx      int64 // accessed atomically, must be 64-bit aligned
	endpoint []remote.Client
}

// Next returns the next available remote client.
func (r *rrPicker) Next(ri RPCInfo) (PickResult, error) {
	endpoints := r.endpoint
	if len(endpoints) == 0 {
		return nil, status.New(code.Code_UNAVAILABLE, "not found endpoint")
	}
	// Use atomic operations for thread-safe round-robin
	idx := int(atomic.AddInt64(&r.idx, 1)-1) % len(r.endpoint)
	res := &pickResult{endpoint: r.endpoint[idx], ctx: ri.Ctx}
	return res, nil
}

type pickResult struct {
	ctx      context.Context
	endpoint remote.Client
}

// RemoteClient returns the remote client for the picker.
func (p *pickResult) RemoteClient() remote.Client {
	return p.endpoint
}

// Report reports the result of the picker.
func (p *pickResult) Report(err error) {
	// Log the result for debugging/monitoring purposes
	if err != nil {
		slog.Debug("rpc call failed",
			slog.String("endpoint", p.endpoint.Scheme()),
			slog.Any("error", err),
		)
	}
	// Future: This can be extended to support:
	// - Circuit breaker integration
	// - Adaptive load balancing (e.g., least-loaded, P2C)
	// - Metrics collection for monitoring
}
