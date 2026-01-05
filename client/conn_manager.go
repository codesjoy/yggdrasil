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
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/codesjoy/yggdrasil/v2/remote"
	"github.com/codesjoy/yggdrasil/v2/resolver"
	"github.com/codesjoy/yggdrasil/v2/stats"
)

// remoteClientManager manages all remote client  centrally.
// It provides a single point of control for connection lifecycle management.
type remoteClientManager struct {
	ctx          context.Context
	appName      string
	statsHandler stats.Handler

	mu            sync.RWMutex
	remoteClients map[string]*rcWrapper // key: endpoint name
	closed        bool
}

// NewRemoteClientManager creates a new remote client manager
func newRemoteClientManager(
	ctx context.Context,
	appName string,
	statsHandler stats.Handler,
) *remoteClientManager {
	return &remoteClientManager{
		ctx:           ctx,
		appName:       appName,
		statsHandler:  statsHandler,
		remoteClients: make(map[string]*rcWrapper),
	}
}

// GetOrCreate returns an existing connection or creates a new one for the endpoint.
// The stateListener is called when the connection state changes.
func (m *remoteClientManager) GetOrCreate(
	endpoint resolver.Endpoint,
	stateListener func(remote.ClientState),
) (remote.Client, error) {
	name := endpoint.Name()

	// Fast path: check if connection exists with read lock
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return nil, errors.New("connection manager is closed")
	}
	if rc, ok := m.remoteClients[name]; ok {
		m.mu.RUnlock()
		return rc, nil
	}
	m.mu.RUnlock()

	// Slow path: create new connection with write lock
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if m.closed {
		return nil, errors.New("connection manager is closed")
	}
	if rc, ok := m.remoteClients[name]; ok {
		return rc, nil
	}

	// Create new connection
	builder := remote.GetClientBuilder(endpoint.GetProtocol())
	if builder == nil {
		return nil, fmt.Errorf("no client builder found for protocol %s", endpoint.GetProtocol())
	}

	rc, err := builder(m.ctx, m.appName, endpoint, m.statsHandler, stateListener)
	if err != nil {
		slog.Error("failed to build client",
			slog.String("protocol", endpoint.GetProtocol()),
			slog.String("address", endpoint.GetAddress()),
			slog.Any("error", err),
		)
		return nil, err
	}

	remoteClient := &rcWrapper{
		name:                endpoint.Name(),
		remoteClientManager: m,
		Client:              rc,
	}

	m.remoteClients[name] = remoteClient
	return remoteClient, nil
}

// Remove removes and closes a connection by endpoint name.
func (m *remoteClientManager) Remove(name string) error {
	m.mu.Lock()
	conn, ok := m.remoteClients[name]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	delete(m.remoteClients, name)
	m.mu.Unlock()

	// Close outside of lock to avoid blocking other operations
	return conn.Close()
}

// Close closes all managed connections
func (m *remoteClientManager) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}

	m.closed = true
	clients := make([]remote.Client, 0, len(m.remoteClients))
	for _, conn := range m.remoteClients {
		clients = append(clients, conn)
	}
	m.remoteClients = nil
	m.mu.Unlock()

	// Close all connections outside of lock
	var multiErr error
	for _, cli := range clients {
		if err := cli.Close(); err != nil {
			multiErr = errors.Join(multiErr, err)
		}
	}

	return multiErr
}

type rcWrapper struct {
	name string
	remote.Client
	remoteClientManager *remoteClientManager
}

func (r *rcWrapper) Close() error {
	return r.remoteClientManager.Remove(r.name)
}

func (r *rcWrapper) Connect() {
	go r.Client.Connect()
}
