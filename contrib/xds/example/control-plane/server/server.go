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

package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync/atomic"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	clusterservice "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	endpointservice "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	listenerservice "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	routeservice "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"google.golang.org/grpc"
)

const (
	grpcMaxConcurrentStreams = 1000000
)

// Callbacks implements callbacks for xDS server
type Callbacks struct {
	signal   chan struct{}
	fetches  int32
	requests int32
}

// NewCallbacks creates a new Callbacks instance
func NewCallbacks() *Callbacks {
	return &Callbacks{
		signal: make(chan struct{}),
	}
}

// OnStreamOpen is called when a new stream is opened
func (cb *Callbacks) OnStreamOpen(ctx context.Context, id int64, typ string) error {
	slog.Info("Stream opened", "id", id, "type", typ)
	return nil
}

// OnStreamClosed is called when a stream is closed
func (cb *Callbacks) OnStreamClosed(id int64, node *corev3.Node) {
	slog.Info("Stream closed", "id", id)
}

// OnStreamRequest is called when a stream request is received
func (cb *Callbacks) OnStreamRequest(id int64, req *discoverygrpc.DiscoveryRequest) error {
	atomic.AddInt32(&cb.requests, 1)
	slog.Info(
		"Stream request",
		"id",
		id,
		"node",
		req.GetNode().GetId(),
		"resources",
		req.GetResourceNames(),
		"version",
		req.GetVersionInfo(),
		"type",
		req.GetTypeUrl(),
	)
	return nil
}

// OnStreamResponse is called when a stream response is sent
func (cb *Callbacks) OnStreamResponse(
	ctx context.Context,
	id int64,
	req *discoverygrpc.DiscoveryRequest,
	resp *discoverygrpc.DiscoveryResponse,
) {
	slog.Info(
		"Stream response",
		"id",
		id,
		"version",
		resp.GetVersionInfo(),
		"type",
		resp.GetTypeUrl(),
		"resources",
		len(resp.GetResources()),
	)
}

// OnFetchRequest is called when a fetch request is received
func (cb *Callbacks) OnFetchRequest(
	ctx context.Context,
	req *discoverygrpc.DiscoveryRequest,
) error {
	atomic.AddInt32(&cb.fetches, 1)
	slog.Info(
		"Fetch request",
		"node",
		req.GetNode().GetId(),
		"resources",
		req.GetResourceNames(),
		"version",
		req.GetVersionInfo(),
		"type",
		req.GetTypeUrl(),
	)
	return nil
}

// OnFetchResponse is called when a fetch response is sent
func (cb *Callbacks) OnFetchResponse(
	req *discoverygrpc.DiscoveryRequest,
	resp *discoverygrpc.DiscoveryResponse,
) {
	slog.Info(
		"Fetch response",
		"version",
		resp.GetVersionInfo(),
		"type",
		resp.GetTypeUrl(),
		"resources",
		len(resp.GetResources()),
	)
}

// OnDeltaStreamOpen is called when a delta stream is opened
func (cb *Callbacks) OnDeltaStreamOpen(ctx context.Context, id int64, typ string) error {
	slog.Info("Delta stream opened", "id", id, "type", typ)
	return nil
}

// OnDeltaStreamClosed is called when a delta stream is closed
func (cb *Callbacks) OnDeltaStreamClosed(id int64, node *corev3.Node) {
	slog.Info("Delta stream closed", "id", id)
}

// OnStreamDeltaRequest is called when a delta stream request is received
func (cb *Callbacks) OnStreamDeltaRequest(
	id int64,
	req *discoverygrpc.DeltaDiscoveryRequest,
) error {
	slog.Info(
		"Delta stream request",
		"id",
		id,
		"node",
		req.GetNode().GetId(),
		"type",
		req.GetTypeUrl(),
	)
	return nil
}

// OnStreamDeltaResponse is called when a delta stream response is sent
func (cb *Callbacks) OnStreamDeltaResponse(
	id int64,
	req *discoverygrpc.DeltaDiscoveryRequest,
	resp *discoverygrpc.DeltaDiscoveryResponse,
) {
	slog.Info(
		"Delta stream response",
		"id",
		id,
		"type",
		resp.GetTypeUrl(),
		"resources",
		len(resp.GetResources()),
	)
}

// Server represents an xDS control plane server
type Server struct {
	grpcServer *grpc.Server
	xdsServer  server.Server
	cache      cache.SnapshotCache
	port       uint
}

// NewServer creates a new xDS server
func NewServer(port uint, cache cache.SnapshotCache) *Server {
	callbacks := NewCallbacks()
	xdsServer := server.NewServer(context.Background(), cache, callbacks)

	grpcServer := grpc.NewServer(
		grpc.MaxConcurrentStreams(grpcMaxConcurrentStreams),
	)

	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(grpcServer, xdsServer)
	endpointservice.RegisterEndpointDiscoveryServiceServer(grpcServer, xdsServer)
	clusterservice.RegisterClusterDiscoveryServiceServer(grpcServer, xdsServer)
	routeservice.RegisterRouteDiscoveryServiceServer(grpcServer, xdsServer)
	listenerservice.RegisterListenerDiscoveryServiceServer(grpcServer, xdsServer)

	return &Server{
		grpcServer: grpcServer,
		xdsServer:  xdsServer,
		cache:      cache,
		port:       port,
	}
}

// Run starts the xDS server
func (s *Server) Run() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	slog.Info("xDS server listening", "port", s.port)

	if err := s.grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}

// Stop gracefully stops the xDS server
func (s *Server) Stop() {
	slog.Info("Stopping xDS server...")
	s.grpcServer.GracefulStop()
}
