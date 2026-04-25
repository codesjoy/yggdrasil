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
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/codesjoy/pkg/basic/xerror"
	"google.golang.org/genproto/googleapis/rpc/code"

	"github.com/codesjoy/yggdrasil/v3/remote"
	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
)

const name = "round_robin"

// BuiltinProvider returns the built-in round_robin balancer provider.
func BuiltinProvider() Provider {
	return NewProvider(name, newRoundRobin)
}

func defaultProviders() map[string]Provider {
	return map[string]Provider{
		name: BuiltinProvider(),
	}
}

type rrBalancer struct {
	cli Client

	mu            sync.RWMutex
	remotesClient map[string]*remoteClientState
	resolverErr   error
	buildErr      error

	lastConnectionErr error
}

type remoteClientState struct {
	client  remote.Client
	state   remote.State
	lastErr error
}

func newRoundRobin(_ string, _ string, cli Client) (Balancer, error) {
	return &rrBalancer{
		cli:           cli,
		remotesClient: make(map[string]*remoteClientState),
	}, nil
}

// UpdateState updates the balancer state.
func (b *rrBalancer) UpdateState(state resolver.State) {
	endpoints := state.GetEndpoints()

	b.mu.Lock()
	if b.remotesClient == nil {
		b.mu.Unlock()
		return
	}

	remoteCli := make(map[string]*remoteClientState, len(endpoints))
	connectClients := make([]remote.Client, 0, len(endpoints))
	buildErrs := make([]string, 0)
	for _, item := range endpoints {
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
			buildErrs = append(buildErrs, fmt.Sprintf("%s: %v", item.Name(), err))
			continue
		}
		if cli != nil {
			clientState := &remoteClientState{client: cli, state: cli.State()}
			switch clientState.state {
			case remote.Idle, remote.Connecting:
				clientState.state = remote.Connecting
				connectClients = append(connectClients, cli)
			}
			remoteCli[item.Name()] = clientState
		}
	}
	needDelClients := make([]remote.Client, 0)
	for key, rc := range b.remotesClient {
		if _, ok := remoteCli[key]; !ok {
			needDelClients = append(needDelClients, rc.client)
		}
	}

	b.remotesClient = remoteCli
	b.buildErr = buildBuildError(buildErrs)
	if len(endpoints) == 0 {
		b.resolverErr = errors.New("produced zero addresses")
	} else {
		b.resolverErr = nil
	}
	if !b.hasTransientFailureLocked() {
		b.lastConnectionErr = nil
	}
	connectivityState, picker := b.buildStateLocked()
	b.mu.Unlock()

	for _, cli := range connectClients {
		cli.Connect()
	}

	b.cli.UpdateState(State{ConnectivityState: connectivityState, Picker: picker})

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
func (b *rrBalancer) UpdateRemoteClientState(state remote.ClientState) {
	var connectClient remote.Client

	b.mu.Lock()
	if b.remotesClient == nil {
		b.mu.Unlock()
		return
	}

	if state.Endpoint != nil {
		endpointState, ok := b.remotesClient[state.Endpoint.Name()]
		if !ok {
			b.mu.Unlock()
			return
		}
		if endpointState.state == remote.TransientFailure &&
			(state.State == remote.Connecting || state.State == remote.Idle) {
			if state.State == remote.Idle {
				connectClient = endpointState.client
			}
			b.mu.Unlock()
			if connectClient != nil {
				connectClient.Connect()
			}
			return
		}

		endpointState.state = state.State
		if state.State == remote.TransientFailure {
			endpointState.lastErr = state.ConnectionError
			if state.ConnectionError != nil {
				b.lastConnectionErr = state.ConnectionError
			}
		} else {
			endpointState.lastErr = nil
		}
		if state.State == remote.Idle {
			connectClient = endpointState.client
		}
		if !b.hasTransientFailureLocked() {
			b.lastConnectionErr = nil
		}
	}

	connectivityState, picker := b.buildStateLocked()
	b.mu.Unlock()

	if connectClient != nil {
		connectClient.Connect()
	}
	b.cli.UpdateState(State{ConnectivityState: connectivityState, Picker: picker})
}

// Close closes all managed connections.
func (b *rrBalancer) Close() error {
	b.mu.Lock()
	if b.remotesClient == nil {
		b.mu.Unlock()
		return nil
	}
	clients := make([]remote.Client, 0, len(b.remotesClient))
	for _, cli := range b.remotesClient {
		clients = append(clients, cli.client)
	}
	b.remotesClient = nil
	b.resolverErr = nil
	b.buildErr = nil
	b.lastConnectionErr = nil
	b.mu.Unlock()
	b.cli.UpdateState(State{ConnectivityState: remote.Shutdown, Picker: &rrPicker{}})
	var multiErr error
	for _, cli := range clients {
		if err := cli.Close(); err != nil {
			multiErr = errors.Join(multiErr, err)
		}
	}
	return multiErr
}

// Type returns the type of the balancer.
func (b *rrBalancer) Type() string {
	return name
}

// buildPicker creates a new picker based on current ready clients
// Must be called with at least a read lock held
func (b *rrBalancer) buildPicker() *rrPicker {
	picker := &rrPicker{endpoint: make([]remote.Client, 0, len(b.remotesClient))}
	for _, item := range b.remotesClient {
		if item.state != remote.Ready {
			continue
		}
		picker.endpoint = append(picker.endpoint, item.client)
	}
	return picker
}

func (b *rrBalancer) buildStateLocked() (remote.State, Picker) {
	if b.remotesClient == nil {
		return remote.Shutdown, &rrPicker{}
	}
	state := b.aggregateConnectivityStateLocked()
	if state != remote.TransientFailure {
		return state, b.buildPicker()
	}
	return state, &errPicker{
		err: b.transientFailureErrorLocked(),
	}
}

func (b *rrBalancer) aggregateConnectivityStateLocked() remote.State {
	if len(b.remotesClient) == 0 {
		if b.resolverErr != nil || b.buildErr != nil {
			return remote.TransientFailure
		}
		return remote.Idle
	}

	var numReady, numConnecting, numIdle, numTransientFailure int
	for _, state := range b.remotesClient {
		switch state.state {
		case remote.Ready:
			numReady++
		case remote.Connecting:
			numConnecting++
		case remote.Idle:
			numIdle++
		case remote.TransientFailure:
			numTransientFailure++
		}
	}

	switch {
	case numReady > 0:
		return remote.Ready
	case numConnecting > 0:
		return remote.Connecting
	case numIdle > 0:
		return remote.Idle
	case numTransientFailure > 0:
		return remote.TransientFailure
	default:
		return remote.TransientFailure
	}
}

func (b *rrBalancer) transientFailureErrorLocked() error {
	hasTF := b.hasTransientFailureLocked()
	switch {
	case hasTF && b.lastConnectionErr != nil && b.resolverErr != nil:
		return fmt.Errorf("last connection error: %v; last resolver error: %v", b.lastConnectionErr, b.resolverErr)
	case hasTF && b.lastConnectionErr != nil && b.buildErr != nil:
		return fmt.Errorf("last connection error: %v; last build error: %v", b.lastConnectionErr, b.buildErr)
	case hasTF && b.lastConnectionErr != nil:
		return fmt.Errorf("last connection error: %v", b.lastConnectionErr)
	case b.resolverErr != nil && b.buildErr != nil:
		return xerror.New(
			code.Code_UNAVAILABLE,
			fmt.Sprintf("last resolver error: %v; last build error: %v", b.resolverErr, b.buildErr),
		)
	case b.resolverErr != nil:
		return xerror.New(code.Code_UNAVAILABLE, fmt.Sprintf("last resolver error: %v", b.resolverErr))
	case b.buildErr != nil:
		return b.buildErr
	default:
		return ErrNoAvailableInstance
	}
}

func (b *rrBalancer) hasTransientFailureLocked() bool {
	for _, state := range b.remotesClient {
		if state.state == remote.TransientFailure {
			return true
		}
	}
	return false
}

func buildBuildError(buildErrs []string) error {
	if len(buildErrs) == 0 {
		return nil
	}
	return xerror.New(
		code.Code_UNAVAILABLE,
		fmt.Sprintf(
			"failed to create remote clients for resolved endpoints: %s",
			strings.Join(buildErrs, "; "),
		),
	)
}

type rrPicker struct {
	idx      int64 // accessed atomically, must be 64-bit aligned
	endpoint []remote.Client
}

type errPicker struct {
	err error
}

func (p *errPicker) Next(RPCInfo) (PickResult, error) {
	return nil, p.err
}

// Next returns the next available remote client.
func (r *rrPicker) Next(ri RPCInfo) (PickResult, error) {
	endpoints := r.endpoint
	if len(endpoints) == 0 {
		return nil, ErrNoAvailableInstance
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
	if err != nil {
		slog.Debug("rpc call failed",
			slog.String("endpoint", p.endpoint.Scheme()),
			slog.Any("error", err),
		)
		return
	}
	slog.Debug("rpc call succeeded",
		slog.String("endpoint", p.endpoint.Scheme()),
	)
}
