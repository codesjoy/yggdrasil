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

package balancer

import (
	"errors"
	"testing"

	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
)

// --- aggregateConnectivityStateLocked tests ---

func TestAggregateState_AllTransientFailure(t *testing.T) {
	cli := newMockBalancerClient()
	b, _ := newRoundRobin("test", "default", cli)
	rr := b.(*rrBalancer)

	rr.mu.Lock()
	rr.remotesClient = map[string]*remoteClientState{
		"e1": {state: remote.TransientFailure},
		"e2": {state: remote.TransientFailure},
	}
	state := rr.aggregateConnectivityStateLocked()
	rr.mu.Unlock()

	if state != remote.TransientFailure {
		t.Fatalf("expected TransientFailure, got %v", state)
	}
}

func TestAggregateState_MixedReadyAndTF(t *testing.T) {
	cli := newMockBalancerClient()
	b, _ := newRoundRobin("test", "default", cli)
	rr := b.(*rrBalancer)

	rr.mu.Lock()
	rr.remotesClient = map[string]*remoteClientState{
		"e1": {state: remote.Ready},
		"e2": {state: remote.TransientFailure},
	}
	state := rr.aggregateConnectivityStateLocked()
	rr.mu.Unlock()

	if state != remote.Ready {
		t.Fatalf("expected Ready (priority over TF), got %v", state)
	}
}

func TestAggregateState_AllConnecting(t *testing.T) {
	cli := newMockBalancerClient()
	b, _ := newRoundRobin("test", "default", cli)
	rr := b.(*rrBalancer)

	rr.mu.Lock()
	rr.remotesClient = map[string]*remoteClientState{
		"e1": {state: remote.Connecting},
		"e2": {state: remote.Connecting},
	}
	state := rr.aggregateConnectivityStateLocked()
	rr.mu.Unlock()

	if state != remote.Connecting {
		t.Fatalf("expected Connecting, got %v", state)
	}
}

func TestAggregateState_AllIdle(t *testing.T) {
	cli := newMockBalancerClient()
	b, _ := newRoundRobin("test", "default", cli)
	rr := b.(*rrBalancer)

	rr.mu.Lock()
	rr.remotesClient = map[string]*remoteClientState{
		"e1": {state: remote.Idle},
		"e2": {state: remote.Idle},
	}
	state := rr.aggregateConnectivityStateLocked()
	rr.mu.Unlock()

	if state != remote.Idle {
		t.Fatalf("expected Idle, got %v", state)
	}
}

func TestAggregateState_EmptyWithResolverErr(t *testing.T) {
	cli := newMockBalancerClient()
	b, _ := newRoundRobin("test", "default", cli)
	rr := b.(*rrBalancer)

	rr.mu.Lock()
	rr.remotesClient = map[string]*remoteClientState{}
	rr.resolverErr = errors.New("resolver failed")
	state := rr.aggregateConnectivityStateLocked()
	rr.mu.Unlock()

	if state != remote.TransientFailure {
		t.Fatalf("expected TransientFailure with resolverErr, got %v", state)
	}
}

func TestAggregateState_EmptyNoErrors(t *testing.T) {
	cli := newMockBalancerClient()
	b, _ := newRoundRobin("test", "default", cli)
	rr := b.(*rrBalancer)

	rr.mu.Lock()
	rr.remotesClient = map[string]*remoteClientState{}
	rr.resolverErr = nil
	rr.buildErr = nil
	state := rr.aggregateConnectivityStateLocked()
	rr.mu.Unlock()

	if state != remote.Idle {
		t.Fatalf("expected Idle with no errors, got %v", state)
	}
}

// --- transientFailureErrorLocked table-driven tests ---

func TestTransientFailureError_AllCombinations(t *testing.T) {
	connErr := errors.New("connection error")
	resErr := errors.New("resolver error")
	buildErr := errors.New("build error")

	tests := []struct {
		name         string
		hasTF        bool
		lastConnErr  error
		resolverErr  error
		buildErr     error
		wantContains string
	}{
		{
			name:         "TF_with_connErr_and_resolverErr",
			hasTF:        true,
			lastConnErr:  connErr,
			resolverErr:  resErr,
			wantContains: "last connection error",
		},
		{
			name:         "TF_with_connErr_and_buildErr",
			hasTF:        true,
			lastConnErr:  connErr,
			buildErr:     buildErr,
			wantContains: "last connection error",
		},
		{
			name:         "TF_with_connErr_only",
			hasTF:        true,
			lastConnErr:  connErr,
			wantContains: "last connection error",
		},
		{
			name:         "no_TF_resolverErr_and_buildErr",
			resolverErr:  resErr,
			buildErr:     buildErr,
			wantContains: "last resolver error",
		},
		{
			name:         "no_TF_resolverErr_only",
			resolverErr:  resErr,
			wantContains: "last resolver error",
		},
		{
			name:         "no_TF_buildErr_only",
			buildErr:     buildErr,
			wantContains: "build error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := newMockBalancerClient()
			b, _ := newRoundRobin("test", "default", cli)
			rr := b.(*rrBalancer)

			rr.mu.Lock()
			if tt.hasTF {
				rr.remotesClient = map[string]*remoteClientState{
					"e1": {state: remote.TransientFailure},
				}
			} else {
				rr.remotesClient = map[string]*remoteClientState{
					"e1": {state: remote.Ready},
				}
			}
			rr.lastConnectionErr = tt.lastConnErr
			rr.resolverErr = tt.resolverErr
			rr.buildErr = tt.buildErr
			err := rr.transientFailureErrorLocked()
			rr.mu.Unlock()

			if err == nil {
				t.Fatal("expected non-nil error")
			}
		})
	}
}

// --- UpdateRemoteClientState edge cases ---

func TestUpdateRemoteClientState_TFToConnecting(t *testing.T) {
	cli := newMockBalancerClient()
	b, _ := newRoundRobin("test", "default", cli)
	rr := b.(*rrBalancer)

	client := newMockRemoteClient("e1", remote.TransientFailure)
	rr.mu.Lock()
	rr.remotesClient = map[string]*remoteClientState{
		"e1": {client: client, state: remote.TransientFailure, lastErr: errors.New("dial failed")},
	}
	rr.mu.Unlock()

	initialUpdates := cli.GetStateUpdates()
	rr.UpdateRemoteClientState(remote.ClientState{
		Endpoint: newMockEndpoint("e1", "localhost:8080", "grpc"),
		State:    remote.Connecting,
	})

	// TF -> Connecting should return early without publishing
	if cli.GetStateUpdates() != initialUpdates {
		t.Fatal("expected no published state for TF->Connecting")
	}
}

func TestUpdateRemoteClientState_TFRecordsError(t *testing.T) {
	cli := newMockBalancerClient()
	b, _ := newRoundRobin("test", "default", cli)
	rr := b.(*rrBalancer)

	client := newMockRemoteClient("e1", remote.TransientFailure)
	connErr := errors.New("connection refused")
	rr.mu.Lock()
	rr.remotesClient = map[string]*remoteClientState{
		"e1": {client: client, state: remote.Connecting},
	}
	rr.mu.Unlock()

	rr.UpdateRemoteClientState(remote.ClientState{
		Endpoint:        newMockEndpoint("e1", "localhost:8080", "grpc"),
		State:           remote.TransientFailure,
		ConnectionError: connErr,
	})

	rr.mu.Lock()
	lastErr := rr.lastConnectionErr
	epState := rr.remotesClient["e1"]
	rr.mu.Unlock()

	if lastErr == nil || lastErr.Error() != "connection refused" {
		t.Fatalf("expected lastConnectionErr to be set, got %v", lastErr)
	}
	if epState.lastErr == nil || epState.lastErr.Error() != "connection refused" {
		t.Fatalf("expected endpoint lastErr to be set, got %v", epState.lastErr)
	}
}

func TestUpdateRemoteClientState_NonTransientClearsLastError(t *testing.T) {
	cli := newMockBalancerClient()
	b, _ := newRoundRobin("test", "default", cli)
	rr := b.(*rrBalancer)

	client := newMockRemoteClient("e1", remote.TransientFailure)
	rr.mu.Lock()
	rr.remotesClient = map[string]*remoteClientState{
		"e1": {client: client, state: remote.TransientFailure, lastErr: errors.New("old error")},
	}
	rr.lastConnectionErr = errors.New("old error")
	rr.mu.Unlock()

	rr.UpdateRemoteClientState(remote.ClientState{
		Endpoint: newMockEndpoint("e1", "localhost:8080", "grpc"),
		State:    remote.Ready,
	})

	rr.mu.Lock()
	epState := rr.remotesClient["e1"]
	rr.mu.Unlock()

	if epState.lastErr != nil {
		t.Fatalf("expected endpoint lastErr to be cleared, got %v", epState.lastErr)
	}
}

func TestUpdateRemoteClientState_UnknownEndpoint(t *testing.T) {
	cli := newMockBalancerClient()
	b, _ := newRoundRobin("test", "default", cli)
	rr := b.(*rrBalancer)

	rr.mu.Lock()
	rr.remotesClient = map[string]*remoteClientState{
		"e1": {client: newMockRemoteClient("e1", remote.Ready), state: remote.Ready},
	}
	rr.mu.Unlock()

	initialUpdates := cli.GetStateUpdates()
	rr.UpdateRemoteClientState(remote.ClientState{
		Endpoint: newMockEndpoint("unknown", "localhost:8080", "grpc"),
		State:    remote.Ready,
	})

	// Unknown endpoint should be a no-op, no state update
	if cli.GetStateUpdates() != initialUpdates {
		t.Fatal("expected no state update for unknown endpoint")
	}
}

func TestUpdateState_PreservesExistingClient(t *testing.T) {
	cli := newMockBalancerClient()
	b, _ := newRoundRobin("test", "default", cli)

	// First update with e1
	ep1 := newMockEndpoint("e1", "localhost:8080", "grpc")
	b.UpdateState(newMockState([]resolver.Endpoint{ep1}))
	firstClient := cli.GetRemoteClient("e1")

	// Second update with e1 + e2
	ep2 := newMockEndpoint("e2", "localhost:8081", "grpc")
	b.UpdateState(newMockState([]resolver.Endpoint{ep1, ep2}))
	secondClient := cli.GetRemoteClient("e1")

	if firstClient != secondClient {
		t.Fatal("expected same client for e1 after second update")
	}
}

func TestUpdateState_NewClientIdleBecomesConnecting(t *testing.T) {
	cli := newMockBalancerClient()
	// Pre-register an idle client
	idleClient := newMockRemoteClient("e1", remote.Idle)
	cli.remoteClients["e1"] = idleClient

	b, _ := newRoundRobin("test", "default", cli)
	ep1 := newMockEndpoint("e1", "localhost:8080", "grpc")
	b.UpdateState(newMockState([]resolver.Endpoint{ep1}))

	if !idleClient.IsConnected() {
		t.Fatal("expected Idle client to be connected (set to Connecting)")
	}
}

func TestClose_DoubleClose(t *testing.T) {
	cli := newMockBalancerClient()
	b, _ := newRoundRobin("test", "default", cli)

	ep1 := newMockEndpoint("e1", "localhost:8080", "grpc")
	b.UpdateState(newMockState([]resolver.Endpoint{ep1}))

	// First close
	err := b.Close()
	if err != nil {
		t.Fatalf("expected no error on first close, got %v", err)
	}

	// Second close should be a no-op
	err = b.Close()
	if err != nil {
		t.Fatalf("expected no error on second close, got %v", err)
	}
}
