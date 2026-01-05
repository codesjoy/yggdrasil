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

	"github.com/codesjoy/yggdrasil/v2/balancer"
	"github.com/codesjoy/yggdrasil/v2/metadata"
	"github.com/codesjoy/yggdrasil/v2/remote"
	"github.com/codesjoy/yggdrasil/v2/resolver"
	"github.com/codesjoy/yggdrasil/v2/stats"
	"github.com/codesjoy/yggdrasil/v2/stream"
)

// mockRemoteClient is a mock implementation of remote.Client
type mockRemoteClient struct {
	name      string
	state     remote.State
	scheme    string
	closed    bool
	connected bool
	mu        sync.Mutex

	newStreamFunc func(ctx context.Context, desc *stream.Desc, method string) (stream.ClientStream, error)
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
	if m.newStreamFunc != nil {
		return m.newStreamFunc(ctx, desc, method)
	}
	return &mockClientStream{ctx: ctx}, nil
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

// mockClientStream is a mock implementation of stream.ClientStream
type mockClientStream struct {
	ctx       context.Context
	header    metadata.MD
	trailer   metadata.MD
	sendErr   error
	recvErr   error
	closed    bool
	mu        sync.Mutex
	sendCount int
	recvCount int
}

func newMockClientStream(ctx context.Context) *mockClientStream {
	return &mockClientStream{
		ctx:     ctx,
		header:  make(metadata.MD),
		trailer: make(metadata.MD),
	}
}

func (m *mockClientStream) Header() (metadata.MD, error) {
	return m.header, nil
}

func (m *mockClientStream) Trailer() metadata.MD {
	return m.trailer
}

func (m *mockClientStream) CloseSend() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockClientStream) Context() context.Context {
	return m.ctx
}

func (m *mockClientStream) SendMsg(msg interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendCount++
	return m.sendErr
}

func (m *mockClientStream) RecvMsg(msg interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recvCount++
	return m.recvErr
}

func (m *mockClientStream) SetSendErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendErr = err
}

func (m *mockClientStream) SetRecvErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recvErr = err
}

// mockStatsHandler is a mock implementation of stats.Handler
type mockStatsHandler struct{}

func newMockStatsHandler() stats.Handler {
	return &mockStatsHandler{}
}

func (m *mockStatsHandler) TagRPC(ctx context.Context, info stats.RPCTagInfo) context.Context {
	return ctx
}

func (m *mockStatsHandler) HandleRPC(ctx context.Context, s stats.RPCStats) {}

func (m *mockStatsHandler) TagChannel(ctx context.Context, info stats.ChanTagInfo) context.Context {
	return ctx
}

func (m *mockStatsHandler) HandleChannel(ctx context.Context, s stats.ChanStats) {}

// mockBalancer is a mock implementation of balancer.Balancer
type mockBalancer struct {
	mu          sync.Mutex
	state       resolver.State
	closed      bool
	picker      balancer.Picker
	updateCount int
}

func newMockBalancer() *mockBalancer {
	return &mockBalancer{}
}

func (m *mockBalancer) UpdateState(state resolver.State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = state
	m.updateCount++
}

func (m *mockBalancer) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockBalancer) Name() string {
	return "mock_balancer"
}

func (m *mockBalancer) UpdatePicker(picker balancer.Picker) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.picker = picker
}

// mockResolver is a mock implementation of resolver.Resolver
type mockResolver struct {
	mu         sync.Mutex
	watchers   map[string]resolver.Client
	addCount   int
	delCount   int
	updateFunc func(resolver.Client)
}

func newMockResolver() *mockResolver {
	return &mockResolver{
		watchers: make(map[string]resolver.Client),
	}
}

func (m *mockResolver) AddWatch(appName string, watcher resolver.Client) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.watchers[appName] = watcher
	m.addCount++
	if m.updateFunc != nil {
		m.updateFunc(watcher)
	}
	return nil
}

func (m *mockResolver) DelWatch(appName string, watcher resolver.Client) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.watchers[appName]; ok {
		delete(m.watchers, appName)
		m.delCount++
	}
	return nil
}

func (m *mockResolver) Name() string {
	return "mock_resolver"
}
