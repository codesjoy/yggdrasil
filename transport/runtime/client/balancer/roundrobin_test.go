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
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"google.golang.org/genproto/googleapis/rpc/code"

	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	ygstatus "github.com/codesjoy/yggdrasil/v3/rpc/status"
	"github.com/codesjoy/yggdrasil/v3/rpc/stream"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
)

// mockRemoteClient is a mock implementation of remote.Client
type mockRemoteClient struct {
	name      string
	state     remote.State
	protocol  string
	closed    bool
	connected bool
	connects  int
	mu        sync.Mutex
}

func newMockRemoteClient(name string, state remote.State) *mockRemoteClient {
	return &mockRemoteClient{
		name:     name,
		state:    state,
		protocol: "mock",
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

func (m *mockRemoteClient) Protocol() string {
	return m.protocol
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
	m.connects++
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

func (m *mockRemoteClient) ConnectCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connects
}

// mockBalancerClient is a mock implementation of balancer.Client
type mockBalancerClient struct {
	mu            sync.Mutex
	state         State
	stateUpdates  int
	remoteClients map[string]*mockRemoteClient
	remoteErrs    map[string]error
}

func newMockBalancerClient() *mockBalancerClient {
	return &mockBalancerClient{
		remoteClients: make(map[string]*mockRemoteClient),
		remoteErrs:    make(map[string]error),
	}
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
	if err, ok := m.remoteErrs[endpoint.Name()]; ok {
		return nil, err
	}
	if rc, ok := m.remoteClients[endpoint.Name()]; ok {
		return rc, nil
	}
	rc := newMockRemoteClient(endpoint.Name(), remote.Ready)
	m.remoteClients[endpoint.Name()] = rc
	return rc, nil
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

func (m *mockBalancerClient) GetRemoteClient(name string) *mockRemoteClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.remoteClients[name]
}

func (m *mockBalancerClient) SetRemoteClientErr(name string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err == nil {
		delete(m.remoteErrs, name)
		return
	}
	m.remoteErrs[name] = err
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
	balancer, err := newRoundRobin("test", "default", cli)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if balancer == nil {
		t.Fatal("expected balancer to be non-nil")
	}
	if balancer.Type() != "round_robin" {
		t.Fatalf("expected type 'round_robin', got %q", balancer.Type())
	}
}

func TestRRBalancer_UpdateState(t *testing.T) {
	cli := newMockBalancerClient()
	balancer, _ := newRoundRobin("test", "default", cli)

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
	if cli.GetState().ConnectivityState != remote.Ready {
		t.Fatalf("expected ready connectivity state, got %v", cli.GetState().ConnectivityState)
	}
}

func TestRRBalancer_UpdateState_RemovedClientClosed(t *testing.T) {
	cli := newMockBalancerClient()
	balancer, _ := newRoundRobin("test", "default", cli)

	endpoint1 := newMockEndpoint("endpoint1", "localhost:8080", "grpc")
	endpoint2 := newMockEndpoint("endpoint2", "localhost:8081", "grpc")
	balancer.UpdateState(newMockState([]resolver.Endpoint{endpoint1, endpoint2}))

	removedClient := cli.GetRemoteClient("endpoint2")
	if removedClient == nil {
		t.Fatal("expected endpoint2 client to exist")
	}

	balancer.UpdateState(newMockState([]resolver.Endpoint{endpoint1}))

	if !removedClient.IsClosed() {
		t.Fatal("expected removed client to be closed")
	}
	if cli.GetRemoteClient("endpoint1") == nil {
		t.Fatal("expected remaining client to stay registered")
	}
}

func TestRRBalancer_Close(t *testing.T) {
	cli := newMockBalancerClient()
	balancer, _ := newRoundRobin("test", "default", cli)

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

func TestRRBalancer_UpdateState_AllRemoteClientBuildsFailPublishesErrorPicker(t *testing.T) {
	cli := newMockBalancerClient()
	cli.SetRemoteClientErr("endpoint1", errors.New("dial endpoint1"))
	cli.SetRemoteClientErr("endpoint2", errors.New("dial endpoint2"))

	balancer, _ := newRoundRobin("test", "default", cli)
	balancer.UpdateState(newMockState([]resolver.Endpoint{
		newMockEndpoint("endpoint1", "localhost:8080", "grpc"),
		newMockEndpoint("endpoint2", "localhost:8081", "grpc"),
	}))

	_, err := cli.GetState().Picker.Next(RPCInfo{Ctx: context.Background(), Method: "test"})
	if err == nil {
		t.Fatal("expected picker error")
	}
	st, ok := ygstatus.CoverError(err)
	if !ok {
		t.Fatalf("expected status error, got %T", err)
	}
	if st.Code() != code.Code_UNAVAILABLE {
		t.Fatalf("expected unavailable, got %v", st.Code())
	}
	if !strings.Contains(err.Error(), "endpoint1: dial endpoint1") {
		t.Fatalf("expected endpoint1 error in %q", err.Error())
	}
	if !strings.Contains(err.Error(), "endpoint2: dial endpoint2") {
		t.Fatalf("expected endpoint2 error in %q", err.Error())
	}
	if cli.GetState().ConnectivityState != remote.TransientFailure {
		t.Fatalf(
			"expected transient failure connectivity state, got %v",
			cli.GetState().ConnectivityState,
		)
	}
}

func TestRRBalancer_UpdateState_AfterClose(t *testing.T) {
	cli := newMockBalancerClient()
	balancer, _ := newRoundRobin("test", "default", cli)

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
	balancer, _ := newRoundRobin("test", "default", cli)

	// First add some endpoints
	endpoints := []resolver.Endpoint{
		newMockEndpoint("endpoint1", "localhost:8080", "grpc"),
	}
	state := newMockState(endpoints)
	balancer.UpdateState(state)

	initialUpdates := cli.GetStateUpdates()

	// Update remote client state
	rrBal := balancer.(*rrBalancer)
	rrBal.UpdateRemoteClientState(remote.ClientState{
		Endpoint: newMockEndpoint("endpoint1", "localhost:8080", "grpc"),
		State:    remote.Ready,
	})

	// Should trigger another state update
	if cli.GetStateUpdates() <= initialUpdates {
		t.Fatal("expected state update after UpdateRemoteClientState")
	}
}

func TestRRBalancer_UpdateRemoteClientState_TransientFailureToIdleReconnectsWithoutPublishingState(
	t *testing.T,
) {
	cli := newMockBalancerClient()
	balancer, _ := newRoundRobin("test", "default", cli)
	rrBal := balancer.(*rrBalancer)

	client := newMockRemoteClient("endpoint1", remote.TransientFailure)
	rrBal.mu.Lock()
	rrBal.remotesClient = map[string]*remoteClientState{
		"endpoint1": {
			client:  client,
			state:   remote.TransientFailure,
			lastErr: errors.New("dial failed"),
		},
	}
	rrBal.lastConnectionErr = errors.New("dial failed")
	rrBal.mu.Unlock()

	initialUpdates := cli.GetStateUpdates()
	rrBal.UpdateRemoteClientState(remote.ClientState{
		Endpoint: newMockEndpoint("endpoint1", "localhost:8080", "grpc"),
		State:    remote.Idle,
	})

	if client.ConnectCount() != 1 {
		t.Fatalf("expected one reconnect attempt, got %d", client.ConnectCount())
	}
	if cli.GetStateUpdates() != initialUpdates {
		t.Fatal("expected no published state update for transient failure to idle transition")
	}
}

func TestRRBalancer_UpdateRemoteClientState_AfterClose(t *testing.T) {
	cli := newMockBalancerClient()
	balancer, _ := newRoundRobin("test", "default", cli)

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
	balancer, _ := newRoundRobin("test", "default", cli)
	rrBal := balancer.(*rrBalancer)

	// Manually add clients with different states
	rrBal.mu.Lock()
	rrBal.remotesClient = map[string]*remoteClientState{
		"ready1": {client: newMockRemoteClient("ready1", remote.Ready), state: remote.Ready},
		"ready2": {client: newMockRemoteClient("ready2", remote.Ready), state: remote.Ready},
		"idle":   {client: newMockRemoteClient("idle", remote.Idle), state: remote.Idle},
		"connecting": {
			client: newMockRemoteClient("connecting", remote.Connecting),
			state:  remote.Connecting,
		},
	}
	picker := rrBal.buildPicker()
	rrBal.mu.Unlock()

	// Picker should only have ready clients
	if len(picker.endpoint) != 2 {
		t.Fatalf("expected 2 ready endpoints, got %d", len(picker.endpoint))
	}
}

func TestRRBalancer_UpdateState_ZeroAddressesPublishesTransientFailure(t *testing.T) {
	cli := newMockBalancerClient()
	balancer, _ := newRoundRobin("test", "default", cli)

	balancer.UpdateState(newMockState(nil))

	state := cli.GetState()
	if state.ConnectivityState != remote.TransientFailure {
		t.Fatalf("expected transient failure, got %v", state.ConnectivityState)
	}
	_, err := state.Picker.Next(RPCInfo{Ctx: context.Background(), Method: "test"})
	if err == nil {
		t.Fatal("expected zero-address picker error")
	}
	if !strings.Contains(err.Error(), "produced zero addresses") {
		t.Fatalf("expected zero-address error, got %q", err.Error())
	}
}

func TestRRBalancer_Type(t *testing.T) {
	cli := newMockBalancerClient()
	balancer, _ := newRoundRobin("test", "default", cli)

	if balancer.Type() != "round_robin" {
		t.Fatalf("expected type 'round_robin', got %q", balancer.Type())
	}
}
