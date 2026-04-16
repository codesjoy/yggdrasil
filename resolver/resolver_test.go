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

package resolver

import (
	"errors"
	"sync"
	"testing"

	"github.com/codesjoy/yggdrasil/v2/config"
)

// mockResolver is a mock implementation of Resolver interface
type mockResolver struct {
	name    string
	watches map[string][]Client
	mu      sync.Mutex
	addErr  error
	delErr  error
}

func newMockResolver(name string) *mockResolver {
	return &mockResolver{
		name:    name,
		watches: make(map[string][]Client),
	}
}

func (m *mockResolver) AddWatch(app string, client Client) error {
	if m.addErr != nil {
		return m.addErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.watches[app] = append(m.watches[app], client)
	return nil
}

func (m *mockResolver) DelWatch(app string, client Client) error {
	if m.delErr != nil {
		return m.delErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	clients := m.watches[app]
	for i, c := range clients {
		if c == client {
			m.watches[app] = append(clients[:i], clients[i+1:]...)
			break
		}
	}
	return nil
}

func (m *mockResolver) Type() string {
	return m.name
}

func (m *mockResolver) GetWatches(app string) []Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.watches[app]
}

// mockClient is a mock implementation of Client interface
type mockClient struct {
	states []State
	mu     sync.Mutex
}

func newMockClient() *mockClient {
	return &mockClient{
		states: make([]State, 0),
	}
}

func (m *mockClient) UpdateState(state State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states = append(m.states, state)
}

func (m *mockClient) GetStates() []State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.states
}

func clearResolverCache(name string) {
	mu.Lock()
	defer mu.Unlock()
	delete(resolver, name)
}

func TestBaseEndpoint_Name(t *testing.T) {
	endpoint := BaseEndpoint{
		Address:  "localhost:8080",
		Protocol: "grpc",
	}

	expected := "grpc/localhost:8080"
	if endpoint.Name() != expected {
		t.Fatalf("expected name %q, got %q", expected, endpoint.Name())
	}
}

func TestBaseEndpoint_GetAddress(t *testing.T) {
	endpoint := BaseEndpoint{
		Address:  "localhost:8080",
		Protocol: "grpc",
	}

	if endpoint.GetAddress() != "localhost:8080" {
		t.Fatalf("expected address 'localhost:8080', got %q", endpoint.GetAddress())
	}
}

func TestBaseEndpoint_GetProtocol(t *testing.T) {
	endpoint := BaseEndpoint{
		Address:  "localhost:8080",
		Protocol: "grpc",
	}

	if endpoint.GetProtocol() != "grpc" {
		t.Fatalf("expected protocol 'grpc', got %q", endpoint.GetProtocol())
	}
}

func TestBaseEndpoint_GetAttributes(t *testing.T) {
	attrs := map[string]any{
		"weight": 100,
		"zone":   "us-east-1",
	}
	endpoint := BaseEndpoint{
		Address:    "localhost:8080",
		Protocol:   "grpc",
		Attributes: attrs,
	}

	result := endpoint.GetAttributes()
	if result["weight"] != 100 {
		t.Fatalf("expected weight 100, got %v", result["weight"])
	}
	if result["zone"] != "us-east-1" {
		t.Fatalf("expected zone 'us-east-1', got %v", result["zone"])
	}
}

func TestBaseEndpoint_GetAttributes_Nil(t *testing.T) {
	endpoint := BaseEndpoint{
		Address:  "localhost:8080",
		Protocol: "grpc",
	}

	result := endpoint.GetAttributes()
	if result != nil {
		t.Fatalf("expected nil attributes, got %v", result)
	}
}

func TestBaseState_GetEndpoints(t *testing.T) {
	endpoints := []Endpoint{
		BaseEndpoint{Address: "localhost:8080", Protocol: "grpc"},
		BaseEndpoint{Address: "localhost:8081", Protocol: "grpc"},
	}
	state := BaseState{
		Endpoints: endpoints,
	}

	result := state.GetEndpoints()
	if len(result) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(result))
	}
}

func TestBaseState_GetAttributes(t *testing.T) {
	attrs := map[string]any{
		"service": "test-service",
		"version": "1.0.0",
	}
	state := BaseState{
		Attributes: attrs,
	}

	result := state.GetAttributes()
	if result["service"] != "test-service" {
		t.Fatalf("expected service 'test-service', got %v", result["service"])
	}
	if result["version"] != "1.0.0" {
		t.Fatalf("expected version '1.0.0', got %v", result["version"])
	}
}

func TestBaseState_GetAttributes_Nil(t *testing.T) {
	state := BaseState{}

	result := state.GetAttributes()
	if result != nil {
		t.Fatalf("expected nil attributes, got %v", result)
	}
}

func TestRegisterBuilder(t *testing.T) {
	testBuilder := func(name string) (Resolver, error) {
		return newMockResolver(name), nil
	}

	RegisterBuilder("test_resolver", testBuilder)

	// Verify it was registered by checking the builder map
	mu.RLock()
	_, ok := builder["test_resolver"]
	mu.RUnlock()

	if !ok {
		t.Fatal("expected builder to be registered")
	}
}

func TestRegisterBuilder_Override(t *testing.T) {
	called := false
	testBuilder1 := func(name string) (Resolver, error) {
		return nil, errors.New("builder1")
	}
	testBuilder2 := func(name string) (Resolver, error) {
		called = true
		return newMockResolver(name), nil
	}

	RegisterBuilder("override_resolver", testBuilder1)
	RegisterBuilder("override_resolver", testBuilder2)

	mu.RLock()
	b := builder["override_resolver"]
	mu.RUnlock()

	_, _ = b("test")
	if !called {
		t.Fatal("expected second builder to be called")
	}
}

func TestGet_NotFoundType(t *testing.T) {
	_, err := Get("non_existent_resolver")
	if err == nil {
		t.Fatal("expected error for non-existent resolver")
	}
}

func TestMockResolver_AddWatch(t *testing.T) {
	r := newMockResolver("test")
	client := newMockClient()

	err := r.AddWatch("app1", client)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	watches := r.GetWatches("app1")
	if len(watches) != 1 {
		t.Fatalf("expected 1 watch, got %d", len(watches))
	}
}

func TestMockResolver_AddWatch_Error(t *testing.T) {
	r := newMockResolver("test")
	r.addErr = errors.New("add watch error")
	client := newMockClient()

	err := r.AddWatch("app1", client)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "add watch error" {
		t.Fatalf("expected 'add watch error', got %q", err.Error())
	}
}

func TestMockResolver_DelWatch(t *testing.T) {
	r := newMockResolver("test")
	client := newMockClient()

	_ = r.AddWatch("app1", client)
	err := r.DelWatch("app1", client)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	watches := r.GetWatches("app1")
	if len(watches) != 0 {
		t.Fatalf("expected 0 watches, got %d", len(watches))
	}
}

func TestMockResolver_DelWatch_Error(t *testing.T) {
	r := newMockResolver("test")
	r.delErr = errors.New("del watch error")
	client := newMockClient()

	err := r.DelWatch("app1", client)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "del watch error" {
		t.Fatalf("expected 'del watch error', got %q", err.Error())
	}
}

func TestMockResolver_Type(t *testing.T) {
	r := newMockResolver("test_name")
	if r.Type() != "test_name" {
		t.Fatalf("expected type 'test_name', got %q", r.Type())
	}
}

func TestMockClient_UpdateState(t *testing.T) {
	client := newMockClient()
	state := BaseState{
		Endpoints: []Endpoint{
			BaseEndpoint{Address: "localhost:8080", Protocol: "grpc"},
		},
	}

	client.UpdateState(state)

	states := client.GetStates()
	if len(states) != 1 {
		t.Fatalf("expected 1 state, got %d", len(states))
	}
}

func TestMockClient_UpdateState_Multiple(t *testing.T) {
	client := newMockClient()

	for i := 0; i < 5; i++ {
		state := BaseState{
			Endpoints: []Endpoint{
				BaseEndpoint{Address: "localhost:8080", Protocol: "grpc"},
			},
		}
		client.UpdateState(state)
	}

	states := client.GetStates()
	if len(states) != 5 {
		t.Fatalf("expected 5 states, got %d", len(states))
	}
}

func TestEndpointInterface(t *testing.T) {
	// Test that BaseEndpoint implements Endpoint interface
	var _ Endpoint = BaseEndpoint{}
	var _ Endpoint = &BaseEndpoint{}
}

func TestStateInterface(t *testing.T) {
	// Test that BaseState implements State interface
	var _ State = BaseState{}
	var _ State = &BaseState{}
}

func TestMockResolver_MultipleClients(t *testing.T) {
	r := newMockResolver("test")
	client1 := newMockClient()
	client2 := newMockClient()
	client3 := newMockClient()

	_ = r.AddWatch("app1", client1)
	_ = r.AddWatch("app1", client2)
	_ = r.AddWatch("app2", client3)

	watches1 := r.GetWatches("app1")
	if len(watches1) != 2 {
		t.Fatalf("expected 2 watches for app1, got %d", len(watches1))
	}

	watches2 := r.GetWatches("app2")
	if len(watches2) != 1 {
		t.Fatalf("expected 1 watch for app2, got %d", len(watches2))
	}
}

func TestMockResolver_DelWatch_NotFound(t *testing.T) {
	r := newMockResolver("test")
	client1 := newMockClient()
	client2 := newMockClient()

	_ = r.AddWatch("app1", client1)

	// Try to delete a client that was never added
	err := r.DelWatch("app1", client2)
	if err != nil {
		t.Fatalf("expected no error when deleting non-existent client, got %v", err)
	}

	// Original client should still be there
	watches := r.GetWatches("app1")
	if len(watches) != 1 {
		t.Fatalf("expected 1 watch, got %d", len(watches))
	}
}

func TestMockResolver_Concurrent(t *testing.T) {
	r := newMockResolver("test")
	var wg sync.WaitGroup

	// Add watches concurrently
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(int) {
			defer wg.Done()
			client := newMockClient()
			_ = r.AddWatch("app1", client)
		}(i)
	}

	wg.Wait()

	watches := r.GetWatches("app1")
	if len(watches) != 100 {
		t.Fatalf("expected 100 watches, got %d", len(watches))
	}
}

func TestGet_WithDefault(t *testing.T) {
	RegisterBuilder("mock", func(name string) (Resolver, error) {
		return newMockResolver("mock"), nil
	})

	tests := []struct {
		name         string
		resolverName string
		setupConfig  func() error
		wantNil      bool
		wantErr      bool
	}{
		{
			name:         "default resolver with no config returns nil (static mode)",
			resolverName: "default",
			setupConfig:  func() error { return nil },
			wantNil:      true,
			wantErr:      false,
		},
		{
			name:         "custom resolver without config returns error",
			resolverName: "custom",
			setupConfig:  func() error { return nil },
			wantNil:      true, // When error is returned, resolver should be nil
			wantErr:      true,
		},
		{
			name:         "configured resolver returns resolver",
			resolverName: "my-resolver",
			setupConfig: func() error {
				return config.Set(
					config.Join(config.KeyBase, "resolver", "my-resolver", "type"),
					"mock",
				)
			},
			wantNil: false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearResolverCache(tt.resolverName)
			t.Cleanup(func() { clearResolverCache(tt.resolverName) })

			// Reset config for this specific resolver
			_ = config.Set(config.Join(config.KeyBase, "resolver", tt.resolverName, "type"), "")

			if err := tt.setupConfig(); err != nil {
				t.Fatalf("setupConfig failed: %v", err)
			}

			got, err := Get(tt.resolverName)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (got == nil) != tt.wantNil {
				t.Errorf("Get() nil = %v, wantNil %v", got == nil, tt.wantNil)
			}
		})
	}
}

func TestGet_ReturnsCachedResolverInstance(t *testing.T) {
	typeName := "cached_instance_resolver"
	resolverName := "cached-instance"
	clearResolverCache(resolverName)
	t.Cleanup(func() { clearResolverCache(resolverName) })

	builderCalls := 0
	RegisterBuilder(typeName, func(name string) (Resolver, error) {
		builderCalls++
		return newMockResolver(name), nil
	})

	requireConfig := func(key string, val any) {
		t.Helper()
		if err := config.Set(key, val); err != nil {
			t.Fatalf("config.Set(%s) failed: %v", key, err)
		}
	}

	requireConfig(config.Join(config.KeyBase, "resolver", resolverName, "type"), typeName)

	r1, err := Get(resolverName)
	if err != nil {
		t.Fatalf("first Get failed: %v", err)
	}
	r2, err := Get(resolverName)
	if err != nil {
		t.Fatalf("second Get failed: %v", err)
	}

	if r1 != r2 {
		t.Fatal("expected Get to return cached resolver instance")
	}
	if builderCalls != 1 {
		t.Fatalf("expected builder to be called once, got %d", builderCalls)
	}
}

func TestGet_ConfigChangeDoesNotRebuildCachedResolver(t *testing.T) {
	typeName := "configurable_resolver"
	resolverName := "configurable-resolver"
	clearResolverCache(resolverName)
	t.Cleanup(func() { clearResolverCache(resolverName) })

	builderCalls := 0
	RegisterBuilder(typeName, func(name string) (Resolver, error) {
		builderCalls++
		marker := config.Get(config.Join(config.KeyBase, "resolver", name, "marker")).String("")
		return newMockResolver(marker), nil
	})

	requireConfig := func(key string, val any) {
		t.Helper()
		if err := config.Set(key, val); err != nil {
			t.Fatalf("config.Set(%s) failed: %v", key, err)
		}
	}

	requireConfig(config.Join(config.KeyBase, "resolver", resolverName, "type"), typeName)
	requireConfig(config.Join(config.KeyBase, "resolver", resolverName, "marker"), "v1")

	r1, err := Get(resolverName)
	if err != nil {
		t.Fatalf("first Get failed: %v", err)
	}

	requireConfig(config.Join(config.KeyBase, "resolver", resolverName, "marker"), "v2")

	r2, err := Get(resolverName)
	if err != nil {
		t.Fatalf("second Get failed: %v", err)
	}

	if r1 != r2 {
		t.Fatal("expected resolver to stay cached after config change")
	}
	if r1.Type() != "v1" {
		t.Fatalf("expected first resolver type v1, got %q", r1.Type())
	}
	if r2.Type() != "v1" {
		t.Fatalf("expected cached resolver type to remain v1, got %q", r2.Type())
	}
	if builderCalls != 1 {
		t.Fatalf("expected builder to be called once, got %d", builderCalls)
	}
}
