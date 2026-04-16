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

// Package client provides balancer client
package client

import (
	"context"
	"log/slog"

	"github.com/codesjoy/pkg/utils/xsync"

	"github.com/codesjoy/yggdrasil/v2/balancer"
	"github.com/codesjoy/yggdrasil/v2/remote"
	"github.com/codesjoy/yggdrasil/v2/resolver"
)

type balancerClient struct {
	cli          *client
	serializer   *xsync.Serializer
	remoteStates map[string]remote.State
}

// UpdateState updates the state of the client
func (bc *balancerClient) UpdateState(state balancer.State) {
	bc.cli.updatePicker(state.Picker)
	bc.cli.updateConnectivityState(state.ConnectivityState)
}

// NewRemoteClient creates a new remote client using centralized ConnManager
func (bc *balancerClient) NewRemoteClient(
	endpoint resolver.Endpoint,
	ops balancer.NewRemoteClientOptions,
) (remote.Client, error) {
	rc, err := bc.cli.remoteClientManager.GetOrCreate(
		endpoint,
		bc.createStateListener(ops.StateListener),
	)
	if err != nil {
		return nil, err
	}
	return rc, nil
}

func (bc *balancerClient) createStateListener(f func(remote.ClientState)) func(remote.ClientState) {
	if f == nil {
		return func(remote.ClientState) {}
	}
	run := func(state remote.ClientState) {
		prevState := bc.rememberRemoteState(state)
		bc.maybeResolveNow(prevState, state)
		f(state)
	}
	if bc.serializer == nil {
		return run
	}
	return func(state remote.ClientState) {
		if err := bc.serializer.Submit(func(_ context.Context) {
			run(state)
		}); err != nil {
			slog.Error("createStateListener failed", slog.Any("error", err))
		}
	}
}

func (bc *balancerClient) rememberRemoteState(state remote.ClientState) remote.State {
	if state.Endpoint == nil {
		return remote.Shutdown
	}
	if bc.remoteStates == nil {
		bc.remoteStates = make(map[string]remote.State)
	}
	name := state.Endpoint.Name()
	prevState := bc.remoteStates[name]
	bc.remoteStates[name] = state.State
	return prevState
}

func (bc *balancerClient) maybeResolveNow(prevState remote.State, state remote.ClientState) {
	switch {
	case state.State == remote.TransientFailure && state.ConnectionError != nil:
		bc.cli.resolveNow()
	case state.State == remote.Idle && prevState == remote.Ready:
		bc.cli.resolveNow()
	}
}
