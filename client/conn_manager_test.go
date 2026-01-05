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

package client

import (
	"context"
	"sync"
	"testing"

	"github.com/codesjoy/yggdrasil/v2/remote"
	"github.com/codesjoy/yggdrasil/v2/resolver"
	"github.com/codesjoy/yggdrasil/v2/stats"
	"github.com/codesjoy/yggdrasil/v2/stream"
)

// testClientBuilder is a mock client builder for testing
func testClientBuilder(
	ctx context.Context,
	appName string,
	endpoint resolver.Endpoint,
	statsHandler stats.Handler,
	stateListener func(remote.ClientState),
) (remote.Client, error) {
	return newMockRemoteClient(endpoint.Name(), remote.Ready), nil
}

func init() {
	// Register the test client builder
	remote.RegisterClientBuilder("test", testClientBuilder)
}

func TestNewRemoteClientManager(t *testing.T) {
	ctx := context.Background()
	manager := newRemoteClientManager(ctx, "test-app", newMockStatsHandler())

	if manager == nil {
		t.Fatal("expected manager to be non-nil")
	}
	if manager.appName != "test-app" {
		t.Fatalf("expected appName 'test-app', got %q", manager.appName)
	}
	if manager.remoteClients == nil {
		t.Fatal("expected remoteClients map to be initialized")
	}
}

func TestRemoteClientManager_GetOrCreate_NewClient(t *testing.T) {
	ctx := context.Background()
	manager := newRemoteClientManager(ctx, "test-app", newMockStatsHandler())

	endpoint := newMockEndpoint("endpoint1", "localhost:8080", "test")
	stateListener := func(state remote.ClientState) {}

	client, err := manager.GetOrCreate(endpoint, stateListener)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client == nil {
		t.Fatal("expected client to be non-nil")
	}
}

func TestRemoteClientManager_GetOrCreate_ExistingClient(t *testing.T) {
	ctx := context.Background()
	manager := newRemoteClientManager(ctx, "test-app", newMockStatsHandler())

	endpoint := newMockEndpoint("endpoint1", "localhost:8080", "test")
	stateListener := func(state remote.ClientState) {}

	// Create first client
	client1, err := manager.GetOrCreate(endpoint, stateListener)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Get same client again
	client2, err := manager.GetOrCreate(endpoint, stateListener)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should be the same client
	if client1 != client2 {
		t.Fatal("expected same client instance")
	}
}

func TestRemoteClientManager_GetOrCreate_AfterClose(t *testing.T) {
	ctx := context.Background()
	manager := newRemoteClientManager(ctx, "test-app", newMockStatsHandler())

	// Close the manager
	_ = manager.Close()

	endpoint := newMockEndpoint("endpoint1", "localhost:8080", "test")
	stateListener := func(state remote.ClientState) {}

	_, err := manager.GetOrCreate(endpoint, stateListener)
	if err == nil {
		t.Fatal("expected error after close")
	}
}

func TestRemoteClientManager_GetOrCreate_NoBuilder(t *testing.T) {
	ctx := context.Background()
	manager := newRemoteClientManager(ctx, "test-app", newMockStatsHandler())

	// Use a protocol without a registered builder
	endpoint := newMockEndpoint("endpoint1", "localhost:8080", "unknown_protocol")
	stateListener := func(state remote.ClientState) {}

	_, err := manager.GetOrCreate(endpoint, stateListener)
	if err == nil {
		t.Fatal("expected error for unknown protocol")
	}
}

func TestRemoteClientManager_Remove(t *testing.T) {
	ctx := context.Background()
	manager := newRemoteClientManager(ctx, "test-app", newMockStatsHandler())

	endpoint := newMockEndpoint("endpoint1", "localhost:8080", "test")
	stateListener := func(state remote.ClientState) {}

	// Create client
	_, err := manager.GetOrCreate(endpoint, stateListener)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Remove client
	err = manager.Remove(endpoint.Name())
	if err != nil {
		t.Fatalf("expected no error on remove, got %v", err)
	}

	// Verify client is removed by checking map directly
	manager.mu.RLock()
	_, exists := manager.remoteClients[endpoint.Name()]
	manager.mu.RUnlock()

	if exists {
		t.Fatal("expected client to be removed")
	}
}

func TestRemoteClientManager_Remove_NotFound(t *testing.T) {
	ctx := context.Background()
	manager := newRemoteClientManager(ctx, "test-app", newMockStatsHandler())

	// Remove non-existent client should not error
	err := manager.Remove("non_existent")
	if err != nil {
		t.Fatalf("expected no error for non-existent client, got %v", err)
	}
}

func TestRemoteClientManager_Close(t *testing.T) {
	ctx := context.Background()
	manager := newRemoteClientManager(ctx, "test-app", newMockStatsHandler())

	endpoint1 := newMockEndpoint("endpoint1", "localhost:8080", "test")
	endpoint2 := newMockEndpoint("endpoint2", "localhost:8081", "test")
	stateListener := func(state remote.ClientState) {}

	// Create multiple clients
	_, _ = manager.GetOrCreate(endpoint1, stateListener)
	_, _ = manager.GetOrCreate(endpoint2, stateListener)

	// Close manager
	err := manager.Close()
	if err != nil {
		t.Fatalf("expected no error on close, got %v", err)
	}

	// Verify manager is closed
	if !manager.closed {
		t.Fatal("expected manager to be closed")
	}

	// Verify clients map is nil
	if manager.remoteClients != nil {
		t.Fatal("expected remoteClients to be nil after close")
	}
}

func TestRemoteClientManager_Close_Idempotent(t *testing.T) {
	ctx := context.Background()
	manager := newRemoteClientManager(ctx, "test-app", newMockStatsHandler())

	// Close multiple times should not error
	err := manager.Close()
	if err != nil {
		t.Fatalf("expected no error on first close, got %v", err)
	}

	err = manager.Close()
	if err != nil {
		t.Fatalf("expected no error on second close, got %v", err)
	}
}

func TestRemoteClientManager_Concurrent(t *testing.T) {
	ctx := context.Background()
	manager := newRemoteClientManager(ctx, "test-app", newMockStatsHandler())

	var wg sync.WaitGroup
	numGoroutines := 50

	// Concurrent GetOrCreate
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			endpoint := newMockEndpoint("endpoint1", "localhost:8080", "test")
			stateListener := func(state remote.ClientState) {}
			_, _ = manager.GetOrCreate(endpoint, stateListener)
		}(i)
	}

	wg.Wait()

	// Should only have one client
	manager.mu.RLock()
	count := len(manager.remoteClients)
	manager.mu.RUnlock()

	if count != 1 {
		t.Fatalf("expected 1 client, got %d", count)
	}
}

func TestRcWrapper_Close(t *testing.T) {
	ctx := context.Background()
	manager := newRemoteClientManager(ctx, "test-app", newMockStatsHandler())

	endpoint := newMockEndpoint("endpoint1", "localhost:8080", "test")
	stateListener := func(state remote.ClientState) {}

	client, err := manager.GetOrCreate(endpoint, stateListener)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Close through wrapper
	err = client.Close()
	if err != nil {
		t.Fatalf("expected no error on close, got %v", err)
	}

	// Verify client is removed from manager
	manager.mu.RLock()
	_, exists := manager.remoteClients[endpoint.Name()]
	manager.mu.RUnlock()

	if exists {
		t.Fatal("expected client to be removed from manager")
	}
}

func TestRcWrapper_Connect(t *testing.T) {
	ctx := context.Background()
	manager := newRemoteClientManager(ctx, "test-app", newMockStatsHandler())

	endpoint := newMockEndpoint("endpoint1", "localhost:8080", "test")
	stateListener := func(state remote.ClientState) {}

	client, err := manager.GetOrCreate(endpoint, stateListener)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Connect should not panic
	wrapper := client.(*rcWrapper)
	wrapper.Connect()
}

func TestRcWrapper_NewStream(t *testing.T) {
	ctx := context.Background()
	manager := newRemoteClientManager(ctx, "test-app", newMockStatsHandler())

	endpoint := newMockEndpoint("endpoint1", "localhost:8080", "test")
	stateListener := func(state remote.ClientState) {}

	client, err := manager.GetOrCreate(endpoint, stateListener)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	desc := &stream.Desc{ServerStreams: false, ClientStreams: false}
	st, err := client.NewStream(ctx, desc, "/test/method")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if st == nil {
		t.Fatal("expected stream to be non-nil")
	}
}
