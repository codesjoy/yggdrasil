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

type Callbacks struct {
	signal   chan struct{}
	fetches  int32
	requests int32
}

func NewCallbacks() *Callbacks {
	return &Callbacks{
		signal: make(chan struct{}),
	}
}

func (cb *Callbacks) OnStreamOpen(ctx context.Context, id int64, typ string) error {
	slog.Info("Stream opened", "id", id, "type", typ)
	return nil
}

func (cb *Callbacks) OnStreamClosed(id int64, node *corev3.Node) {
	slog.Info("Stream closed", "id", id)
}

func (cb *Callbacks) OnStreamRequest(id int64, req *discoverygrpc.DiscoveryRequest) error {
	atomic.AddInt32(&cb.requests, 1)
	slog.Info("Stream request", "id", id, "node", req.GetNode().GetId(), "resources", req.GetResourceNames(), "version", req.GetVersionInfo(), "type", req.GetTypeUrl())
	return nil
}

func (cb *Callbacks) OnStreamResponse(ctx context.Context, id int64, req *discoverygrpc.DiscoveryRequest, resp *discoverygrpc.DiscoveryResponse) {
	slog.Info("Stream response", "id", id, "version", resp.GetVersionInfo(), "type", resp.GetTypeUrl(), "resources", len(resp.GetResources()))
}

func (cb *Callbacks) OnFetchRequest(ctx context.Context, req *discoverygrpc.DiscoveryRequest) error {
	atomic.AddInt32(&cb.fetches, 1)
	slog.Info("Fetch request", "node", req.GetNode().GetId(), "resources", req.GetResourceNames(), "version", req.GetVersionInfo(), "type", req.GetTypeUrl())
	return nil
}

func (cb *Callbacks) OnFetchResponse(req *discoverygrpc.DiscoveryRequest, resp *discoverygrpc.DiscoveryResponse) {
	slog.Info("Fetch response", "version", resp.GetVersionInfo(), "type", resp.GetTypeUrl(), "resources", len(resp.GetResources()))
}

func (cb *Callbacks) OnDeltaStreamOpen(ctx context.Context, id int64, typ string) error {
	slog.Info("Delta stream opened", "id", id, "type", typ)
	return nil
}

func (cb *Callbacks) OnDeltaStreamClosed(id int64, node *corev3.Node) {
	slog.Info("Delta stream closed", "id", id)
}

func (cb *Callbacks) OnStreamDeltaRequest(id int64, req *discoverygrpc.DeltaDiscoveryRequest) error {
	slog.Info("Delta stream request", "id", id, "node", req.GetNode().GetId(), "type", req.GetTypeUrl())
	return nil
}

func (cb *Callbacks) OnStreamDeltaResponse(id int64, req *discoverygrpc.DeltaDiscoveryRequest, resp *discoverygrpc.DeltaDiscoveryResponse) {
	slog.Info("Delta stream response", "id", id, "type", resp.GetTypeUrl(), "resources", len(resp.GetResources()))
}

type Server struct {
	grpcServer *grpc.Server
	xdsServer  server.Server
	cache      cache.SnapshotCache
	port       uint
}

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

func (s *Server) Stop() {
	slog.Info("Stopping xDS server...")
	s.grpcServer.GracefulStop()
}
