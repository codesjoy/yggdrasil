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
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/codesjoy/yggdrasil/v3/balancer"
	"github.com/codesjoy/yggdrasil/v3/interceptor"
	"github.com/codesjoy/yggdrasil/v3/metadata"
	"github.com/codesjoy/yggdrasil/v3/remote"
	"github.com/codesjoy/yggdrasil/v3/resolver"
	"github.com/codesjoy/yggdrasil/v3/stats"
	"github.com/codesjoy/yggdrasil/v3/stream"
)

type countingBackoff struct {
	calls int32
}

func (b *countingBackoff) Backoff(_ int) time.Duration {
	atomic.AddInt32(&b.calls, 1)
	return time.Millisecond
}

func (b *countingBackoff) Count() int32 {
	return atomic.LoadInt32(&b.calls)
}

// mockRemoteClient is a mock implementation of remote.Client.
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

// mockEndpoint is a mock implementation of resolver.Endpoint.
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

// mockClientStream is a mock implementation of stream.ClientStream.
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

func (m *mockClientStream) SendMsg(any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendCount++
	return m.sendErr
}

func (m *mockClientStream) RecvMsg(any) error {
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

// mockStatsHandler is a mock implementation of stats.Handler.
type mockStatsHandler struct{}

func newMockStatsHandler() stats.Handler {
	return &mockStatsHandler{}
}

func (m *mockStatsHandler) TagRPC(ctx context.Context, info stats.RPCTagInfo) context.Context {
	return ctx
}

func (m *mockStatsHandler) HandleRPC(context.Context, stats.RPCStats) {}

func (m *mockStatsHandler) TagChannel(ctx context.Context, info stats.ChanTagInfo) context.Context {
	return ctx
}

func (m *mockStatsHandler) HandleChannel(context.Context, stats.ChanStats) {}

// mockBalancer is a mock implementation of balancer.Balancer.
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

func (m *mockBalancer) Type() string {
	return "mock_balancer"
}

func (m *mockBalancer) UpdatePicker(picker balancer.Picker) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.picker = picker
}

// mockPicker is a mock implementation of balancer.Picker.
type mockPicker struct {
	results     []balancer.PickResult
	errors      []error
	callCount   int32
	returnIndex int32
}

func newMockPicker() *mockPicker {
	return &mockPicker{
		results: make([]balancer.PickResult, 0),
		errors:  make([]error, 0),
	}
}

func (m *mockPicker) AddResult(result balancer.PickResult, err error) {
	m.results = append(m.results, result)
	m.errors = append(m.errors, err)
}

func (m *mockPicker) Next(balancer.RPCInfo) (balancer.PickResult, error) {
	atomic.AddInt32(&m.callCount, 1)
	idx := atomic.AddInt32(&m.returnIndex, 1) - 1
	if int(idx) >= len(m.results) {
		idx = int32(len(m.results) - 1)
	}
	return m.results[idx], m.errors[idx]
}

func (m *mockPicker) GetCallCount() int32 {
	return atomic.LoadInt32(&m.callCount)
}

// mockPickResult is a mock implementation of balancer.PickResult.
type mockPickResult struct {
	client     remote.Client
	reportFunc func(err error)
}

func newMockPickResult(client remote.Client) *mockPickResult {
	return &mockPickResult{
		client: client,
	}
}

func (m *mockPickResult) RemoteClient() remote.Client {
	return m.client
}

func (m *mockPickResult) Report(err error) {
	if m.reportFunc != nil {
		m.reportFunc(err)
	}
}

type testRuntime struct {
	configs         map[string]ServiceSettings
	statsHandler    stats.Handler
	clientProviders map[string]remote.TransportClientProvider
	newResolver     func(name string) (resolver.Resolver, error)
	newBalancer     func(serviceName, balancerName string, cli balancer.Client) (balancer.Balancer, error)
	buildUnary      func(serviceName string, names []string) interceptor.UnaryClientInterceptor
	buildStream     func(serviceName string, names []string) interceptor.StreamClientInterceptor
}

func newTestRuntime() *testRuntime {
	return &testRuntime{
		configs:      map[string]ServiceSettings{},
		statsHandler: newMockStatsHandler(),
		clientProviders: map[string]remote.TransportClientProvider{
			"test": remote.NewTransportClientProvider("test", testClientBuilder),
		},
		newBalancer: func(serviceName, balancerName string, cli balancer.Client) (balancer.Balancer, error) {
			return newMockBalancer(), nil
		},
	}
}

func (r *testRuntime) ClientSettings(serviceName string) ServiceSettings {
	if r == nil {
		return ServiceSettings{}
	}
	return r.configs[serviceName]
}

func (r *testRuntime) ClientStatsHandler() stats.Handler {
	if r == nil || r.statsHandler == nil {
		return stats.NoOpHandler
	}
	return r.statsHandler
}

func (r *testRuntime) TransportClientProvider(protocol string) remote.TransportClientProvider {
	if r == nil {
		return nil
	}
	return r.clientProviders[protocol]
}

func (r *testRuntime) NewResolver(name string) (resolver.Resolver, error) {
	if r != nil && r.newResolver != nil {
		return r.newResolver(name)
	}
	if name == "" || name == resolver.DefaultResolverName {
		return nil, nil
	}
	return nil, fmt.Errorf("resolver %s not found", name)
}

func (r *testRuntime) NewBalancer(
	serviceName string,
	balancerName string,
	cli balancer.Client,
) (balancer.Balancer, error) {
	if r != nil && r.newBalancer != nil {
		return r.newBalancer(serviceName, balancerName, cli)
	}
	return nil, fmt.Errorf("balancer %s not found", balancerName)
}

func (r *testRuntime) BuildUnaryClientInterceptor(
	serviceName string,
	names []string,
) interceptor.UnaryClientInterceptor {
	if r != nil && r.buildUnary != nil {
		return r.buildUnary(serviceName, names)
	}
	return nil
}

func (r *testRuntime) BuildStreamClientInterceptor(
	serviceName string,
	names []string,
) interceptor.StreamClientInterceptor {
	if r != nil && r.buildStream != nil {
		return r.buildStream(serviceName, names)
	}
	return nil
}
