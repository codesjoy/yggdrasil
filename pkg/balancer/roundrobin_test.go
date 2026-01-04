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
	"sync"
	"sync/atomic"
	"testing"

	"github.com/codesjoy/yggdrasil/pkg/remote"
	"github.com/codesjoy/yggdrasil/pkg/resolver"
	"github.com/codesjoy/yggdrasil/pkg/stream"
)

// mockRemoteClient is a mock implementation of remote.Client
type mockRemoteClient struct {
	name      string
	state     remote.State
	scheme    string
	closed    bool
	connected bool
	mu        sync.Mutex
}

func newMockRemoteClient(name string, state remote.State) *mockRemoteClient {
	return &mockRemoteClient{
		name:   name,
		state:  state,
		scheme: "mock://" + name,
	}
}

func (m *mockRemoteClient) NewStream(
	ctx context.Context,
	desc *stream.Desc,
	method string,
) (stream.ClientStream, error) {
	return nil, nil
}

func (m *mockRemoteClient) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockRemoteClient) Scheme() string {
	return m.scheme
}

func (m *mockRemoteClient) State() remote.State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

func (m *mockRemoteClient) SetState(state remote.State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = state
}

func (m *mockRemoteClient) Connect() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = true
}

func (m *mockRemoteClient) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

func (m *mockRemoteClient) IsConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connected
}

// mockBalancerClient is a mock implementation of balancer.Client
type mockBalancerClient struct {
	mu           sync.Mutex
	state        State
	stateUpdates int
	remoteClient *mockRemoteClient
}

func newMockBalancerClient() *mockBalancerClient {
	return &mockBalancerClient{}
}

func (m *mockBalancerClient) UpdateState(state State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = state
	m.stateUpdates++
}

func (m *mockBalancerClient) NewRemoteClient(
	endpoint resolver.Endpoint,
	ops NewRemoteClientOptions,
) (remote.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.remoteClient = newMockRemoteClient(endpoint.Name(), remote.Ready)
	return m.remoteClient, nil
}

func (m *mockBalancerClient) GetStateUpdates() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stateUpdates
}

func (m *mockBalancerClient) GetState() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

// mockEndpoint is a mock implementation of resolver.Endpoint
type mockEndpoint struct {
	name       string
	address    string
	protocol   string
	attributes map[string]any
}

func newMockEndpoint(name, address, protocol string) *mockEndpoint {
	return &mockEndpoint{
		name:       name,
		address:    address,
		protocol:   protocol,
		attributes: make(map[string]any),
	}
}

func (m *mockEndpoint) Name() string {
	return m.name
}

func (m *mockEndpoint) GetAddress() string {
	return m.address
}

func (m *mockEndpoint) GetProtocol() string {
	return m.protocol
}

func (m *mockEndpoint) GetAttributes() map[string]any {
	return m.attributes
}

// mockState is a mock implementation of resolver.State
type mockState struct {
	endpoints  []resolver.Endpoint
	attributes map[string]any
}

func newMockState(endpoints []resolver.Endpoint) *mockState {
	return &mockState{
		endpoints:  endpoints,
		attributes: make(map[string]any),
	}
}

func (m *mockState) GetEndpoints() []resolver.Endpoint {
	return m.endpoints
}

func (m *mockState) GetAttributes() map[string]any {
	return m.attributes
}

func TestNewRoundRobin(t *testing.T) {
	cli := newMockBalancerClient()
	balancer, err := newRoundRobin("test", cli)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if balancer == nil {
		t.Fatal("expected balancer to be non-nil")
	}
	if balancer.Name() != "round_robin" {
		t.Fatalf("expected name 'round_robin', got %q", balancer.Name())
	}
}

func TestRRBalancer_UpdateState(t *testing.T) {
	cli := newMockBalancerClient()
	balancer, _ := newRoundRobin("test", cli)

	endpoints := []resolver.Endpoint{
		newMockEndpoint("endpoint1", "localhost:8080", "grpc"),
		newMockEndpoint("endpoint2", "localhost:8081", "grpc"),
	}
	state := newMockState(endpoints)

	balancer.UpdateState(state)

	// Verify that UpdateState was called on the client
	if cli.GetStateUpdates() == 0 {
		t.Fatal("expected client UpdateState to be called")
	}
}

func TestRRBalancer_Close(t *testing.T) {
	cli := newMockBalancerClient()
	balancer, _ := newRoundRobin("test", cli)

	endpoints := []resolver.Endpoint{
		newMockEndpoint("endpoint1", "localhost:8080", "grpc"),
	}
	state := newMockState(endpoints)
	balancer.UpdateState(state)

	err := balancer.Close()
	if err != nil {
		t.Fatalf("expected no error on close, got %v", err)
	}

	// Verify that UpdateState was called after close (to update picker)
	updates := cli.GetStateUpdates()
	if updates < 2 {
		t.Fatalf("expected at least 2 state updates, got %d", updates)
	}
}

func TestRRBalancer_UpdateState_AfterClose(t *testing.T) {
	cli := newMockBalancerClient()
	balancer, _ := newRoundRobin("test", cli)

	// Close the balancer first
	_ = balancer.Close()

	initialUpdates := cli.GetStateUpdates()

	// Try to update state after close
	endpoints := []resolver.Endpoint{
		newMockEndpoint("endpoint1", "localhost:8080", "grpc"),
	}
	state := newMockState(endpoints)
	balancer.UpdateState(state)

	// Should not have additional updates after close
	if cli.GetStateUpdates() != initialUpdates {
		t.Fatal("expected no state updates after close")
	}
}

func TestRRPicker_Next_Empty(t *testing.T) {
	picker := &rrPicker{endpoint: []remote.Client{}}

	_, err := picker.Next(RPCInfo{Ctx: context.Background(), Method: "test"})
	if err == nil {
		t.Fatal("expected error for empty picker")
	}
}

func TestRRPicker_Next_SingleEndpoint(t *testing.T) {
	client := newMockRemoteClient("test", remote.Ready)
	picker := &rrPicker{endpoint: []remote.Client{client}}

	result, err := picker.Next(RPCInfo{Ctx: context.Background(), Method: "test"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.RemoteClient() != client {
		t.Fatal("expected to get the same client")
	}
}

func TestRRPicker_Next_RoundRobin(t *testing.T) {
	client1 := newMockRemoteClient("test1", remote.Ready)
	client2 := newMockRemoteClient("test2", remote.Ready)
	client3 := newMockRemoteClient("test3", remote.Ready)

	picker := &rrPicker{endpoint: []remote.Client{client1, client2, client3}}

	// First round
	results := make([]remote.Client, 6)
	for i := 0; i < 6; i++ {
		result, err := picker.Next(RPCInfo{Ctx: context.Background(), Method: "test"})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		results[i] = result.RemoteClient()
	}

	// Verify round-robin behavior
	if results[0] != client1 {
		t.Error("expected first pick to be client1")
	}
	if results[1] != client2 {
		t.Error("expected second pick to be client2")
	}
	if results[2] != client3 {
		t.Error("expected third pick to be client3")
	}
	if results[3] != client1 {
		t.Error("expected fourth pick to be client1 (wrap around)")
	}
	if results[4] != client2 {
		t.Error("expected fifth pick to be client2")
	}
	if results[5] != client3 {
		t.Error("expected sixth pick to be client3")
	}
}

func TestRRPicker_Next_Concurrent(t *testing.T) {
	numClients := 10
	clients := make([]remote.Client, numClients)
	for i := 0; i < numClients; i++ {
		clients[i] = newMockRemoteClient("test", remote.Ready)
	}

	picker := &rrPicker{endpoint: clients}

	var wg sync.WaitGroup
	numGoroutines := 100
	picksPerGoroutine := 100

	counts := make([]int64, numClients)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < picksPerGoroutine; j++ {
				result, err := picker.Next(RPCInfo{Ctx: context.Background(), Method: "test"})
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				// Find which client was picked
				for k, c := range clients {
					if result.RemoteClient() == c {
						atomic.AddInt64(&counts[k], 1)
						break
					}
				}
			}
		}()
	}

	wg.Wait()

	// Verify all picks were made
	totalPicks := int64(0)
	for _, count := range counts {
		totalPicks += count
	}
	expectedTotal := int64(numGoroutines * picksPerGoroutine)
	if totalPicks != expectedTotal {
		t.Fatalf("expected %d total picks, got %d", expectedTotal, totalPicks)
	}
}

func TestPickResult_Report(t *testing.T) {
	client := newMockRemoteClient("test", remote.Ready)
	result := &pickResult{
		ctx:      context.Background(),
		endpoint: client,
	}

	// Report should not panic
	result.Report(nil)
	result.Report(context.DeadlineExceeded)
}

func TestPickResult_RemoteClient(t *testing.T) {
	client := newMockRemoteClient("test", remote.Ready)
	result := &pickResult{
		ctx:      context.Background(),
		endpoint: client,
	}

	if result.RemoteClient() != client {
		t.Fatal("expected RemoteClient to return the endpoint")
	}
}

func TestRRBalancer_UpdateRemoteClientState(t *testing.T) {
	cli := newMockBalancerClient()
	balancer, _ := newRoundRobin("test", cli)

	// First add some endpoints
	endpoints := []resolver.Endpoint{
		newMockEndpoint("endpoint1", "localhost:8080", "grpc"),
	}
	state := newMockState(endpoints)
	balancer.UpdateState(state)

	initialUpdates := cli.GetStateUpdates()

	// Update remote client state
	rrBal := balancer.(*rrBalancer)
	rrBal.UpdateRemoteClientState(remote.ClientState{State: remote.Ready})

	// Should trigger another state update
	if cli.GetStateUpdates() <= initialUpdates {
		t.Fatal("expected state update after UpdateRemoteClientState")
	}
}

func TestRRBalancer_UpdateRemoteClientState_AfterClose(t *testing.T) {
	cli := newMockBalancerClient()
	balancer, _ := newRoundRobin("test", cli)

	// Close the balancer
	_ = balancer.Close()

	initialUpdates := cli.GetStateUpdates()

	// Try to update remote client state after close
	rrBal := balancer.(*rrBalancer)
	rrBal.UpdateRemoteClientState(remote.ClientState{State: remote.Ready})

	// Should not trigger state update after close
	if cli.GetStateUpdates() != initialUpdates {
		t.Fatal("expected no state update after close")
	}
}

func TestRRBalancer_BuildPicker_OnlyReadyClients(t *testing.T) {
	cli := newMockBalancerClient()
	balancer, _ := newRoundRobin("test", cli)
	rrBal := balancer.(*rrBalancer)

	// Manually add clients with different states
	rrBal.mu.Lock()
	rrBal.remotesClient = map[string]remote.Client{
		"ready1":     newMockRemoteClient("ready1", remote.Ready),
		"ready2":     newMockRemoteClient("ready2", remote.Ready),
		"idle":       newMockRemoteClient("idle", remote.Idle),
		"connecting": newMockRemoteClient("connecting", remote.Connecting),
	}
	picker := rrBal.buildPicker()
	rrBal.mu.Unlock()

	// Picker should only have ready clients
	if len(picker.endpoint) != 2 {
		t.Fatalf("expected 2 ready endpoints, got %d", len(picker.endpoint))
	}
}

func TestRRBalancer_Name(t *testing.T) {
	cli := newMockBalancerClient()
	balancer, _ := newRoundRobin("test", cli)

	if balancer.Name() != "round_robin" {
		t.Fatalf("expected name 'round_robin', got %q", balancer.Name())
	}
}
