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

// Package xds provides xDS (Envoy discovery service) integration for service discovery and configuration.
// It implements the ADS (Aggregated Discovery Service) protocol for dynamic configuration updates.
package xds

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"slices"
	"sync"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

type subscriptions struct {
	lds []string
	rds []string
	cds []string
	eds []string
}

type typeWatchState struct {
	version   string
	nonce     string
	resources map[string]*anypb.Any
}

type adsClient struct {
	cfg        ResolverConfig
	mu         sync.Mutex
	ctx        context.Context
	cancel     context.CancelFunc
	conn       *grpc.ClientConn
	client     discoveryv3.AggregatedDiscoveryServiceClient
	stream     discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesClient
	node       *corev3.Node
	sub        subscriptions
	typeState  map[string]*typeWatchState
	handle     func(discoveryEvent)
	sendCh     chan *discoveryv3.DiscoveryRequest
	retries    int
	maxRetries int
}

func newADSClient(cfg ResolverConfig, handle func(discoveryEvent)) (*adsClient, error) {
	ctx, cancel := context.WithCancel(context.Background())

	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 10
	}

	metadataMap := make(map[string]any)
	for k, v := range cfg.Node.Metadata {
		metadataMap[k] = v
	}
	metadata, err := structpb.NewStruct(metadataMap)
	if err != nil {
		cancel() // Clean up context on error
		return nil, err
	}

	node := &corev3.Node{
		Id:       cfg.Node.ID,
		Cluster:  cfg.Node.Cluster,
		Metadata: metadata,
	}

	return &adsClient{
		cfg:        cfg,
		ctx:        ctx,
		cancel:     cancel,
		node:       node,
		sub:        subscriptions{},
		typeState:  make(map[string]*typeWatchState),
		handle:     handle,
		sendCh:     make(chan *discoveryv3.DiscoveryRequest, 100),
		retries:    0,
		maxRetries: maxRetries,
	}, nil
}

func (c *adsClient) Start() error {
	if c.cfg.Server.Address == "" {
		return fmt.Errorf("xDS server address is empty")
	}
	go c.run()
	return nil
}

func (c *adsClient) run() {
	backoff := time.Second
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		if err := c.connect(); err != nil {
			log.Printf("[xDS] connection failed: %v, retrying in %v", err, backoff)
			select {
			case <-c.ctx.Done():
				return
			case <-time.After(backoff):
				if c.shouldReconnect() {
					c.incrementRetry()
					backoff = time.Duration(c.retries) * time.Second
				} else {
					return
				}
			}
		} else {
			c.resetRetries()
			backoff = time.Second
		}
	}
}

func (c *adsClient) connect() error {
	c.mu.Lock()
	opts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(1024*1024*10),
			grpc.MaxCallSendMsgSize(1024*1024*10),
		),
	}

	if c.cfg.Server.TLS.Enable {
		//nolint:gosec // G402: MinVersion is intentionally not set to allow configuration flexibility
		tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
		if c.cfg.Server.TLS.CertFile != "" && c.cfg.Server.TLS.KeyFile != "" {
			cert, err := tls.LoadX509KeyPair(c.cfg.Server.TLS.CertFile, c.cfg.Server.TLS.KeyFile)
			if err != nil {
				c.mu.Unlock()
				return fmt.Errorf("failed to load TLS cert pair: %w", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
		if c.cfg.Server.TLS.CAFile != "" {
			caCert, err := os.ReadFile(c.cfg.Server.TLS.CAFile)
			if err != nil {
				c.mu.Unlock()
				return fmt.Errorf("failed to read CA file: %w", err)
			}
			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)
			tlsConfig.RootCAs = caCertPool
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	c.mu.Unlock()

	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.Server.Timeout)
	defer cancel()

	//nolint:staticcheck // SA1019: DialContext is deprecated but still supported in gRPC 1.x
	// Dial with timeout
	conn, err := grpc.DialContext(ctx, c.cfg.Server.Address, opts...)
	if err != nil {
		return err
	}
	defer conn.Close() //nolint

	client := discoveryv3.NewAggregatedDiscoveryServiceClient(conn)
	// Use the main context for the stream, not the dial timeout context
	stream, err := client.StreamAggregatedResources(c.ctx)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.client = client
	c.stream = stream
	c.mu.Unlock()

	// Re-send subscriptions
	c.resendSubscriptions()

	// Start send loop in background
	// We need a way to stop sendLoop when this connection dies, but sendLoop reads from c.sendCh which is persistent.
	// One way is to have sendLoop handle stream errors.
	// Actually, having two loops (send/recv) is tricky if we want to synchronize closing.
	// Simpler: merge them or control them via a per-connection context?
	// But c.sendCh is shared.
	// Let's spawn sendLoop and return when stream fails.

	errCh := make(chan error, 2)

	go func() {
		errCh <- c.sendLoop(stream)
	}()
	go func() {
		errCh <- c.watchResources(stream)
	}()

	// Wait for one to fail
	select {
	case <-c.ctx.Done():
		return nil
	case err := <-errCh:
		// Attempt to close stream to unblock the other loop
		_ = stream.CloseSend()
		return err
	}
}

func (c *adsClient) resendSubscriptions() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.sub.lds) > 0 {
		c.sendSubscriptionRequestLocked(resource.ListenerType)
	}
	if len(c.sub.rds) > 0 {
		c.sendSubscriptionRequestLocked(resource.RouteType)
	}
	if len(c.sub.cds) > 0 {
		c.sendSubscriptionRequestLocked(resource.ClusterType)
	}
	if len(c.sub.eds) > 0 {
		c.sendSubscriptionRequestLocked(resource.EndpointType)
	}
}

// sendSubscriptionRequestLocked assumes c.mu is held
func (c *adsClient) sendSubscriptionRequestLocked(typeURL string) {
	var resources []string
	switch typeURL {
	case resource.ListenerType:
		resources = c.sub.lds
	case resource.RouteType:
		resources = c.sub.rds
	case resource.ClusterType:
		resources = c.sub.cds
	case resource.EndpointType:
		resources = c.sub.eds
	}

	state := c.typeState[typeURL]
	if state == nil {
		state = &typeWatchState{
			resources: make(map[string]*anypb.Any),
		}
		c.typeState[typeURL] = state
	}

	req := &discoveryv3.DiscoveryRequest{
		Node:          c.node,
		TypeUrl:       typeURL,
		ResourceNames: resources,
		VersionInfo:   state.version,
		ResponseNonce: state.nonce,
	}

	// Non-blocking send or drop if full?
	// Ideally blocking but we are in a lock.
	// However, sendCh is buffered.
	select {
	case c.sendCh <- req:
	default:
		log.Printf("[xDS] send buffer full, dropping subscription request for %s", typeURL)
	}
}

func (c *adsClient) sendLoop(
	stream discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesClient,
) error {
	for {
		select {
		case <-c.ctx.Done():
			return nil
		case <-stream.Context().Done():
			return stream.Context().Err()
		case req := <-c.sendCh:
			if err := stream.Send(req); err != nil {
				return err
			}
		}
	}
}

func (c *adsClient) watchResources(
	stream discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesClient,
) error {
	for {
		resp, err := stream.Recv()
		if err != nil {
			return err
		}

		c.handleResponse(resp)
	}
}

func (c *adsClient) shouldReconnect() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.retries < c.maxRetries
}

func (c *adsClient) incrementRetry() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.retries++
}

func (c *adsClient) resetRetries() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.retries = 0
}

func (c *adsClient) Close() {
	c.cancel()
	if c.stream != nil {
		_ = c.stream.CloseSend()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
	close(c.sendCh)
}

func (c *adsClient) UpdateSubscriptions(lds, rds, cds, eds []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	slices.Sort(lds)
	slices.Sort(rds)
	slices.Sort(cds)
	slices.Sort(eds)

	newSub := subscriptions{
		lds: lds,
		rds: rds,
		cds: cds,
		eds: eds,
	}

	if subscriptionsEqual(c.sub, newSub) {
		return
	}

	c.sub = newSub

	c.sendSubscriptionRequestLocked(resource.ListenerType)
	c.sendSubscriptionRequestLocked(resource.RouteType)
	c.sendSubscriptionRequestLocked(resource.ClusterType)
	c.sendSubscriptionRequestLocked(resource.EndpointType)
}

func (c *adsClient) handleResponse(resp *discoveryv3.DiscoveryResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()

	state := c.typeState[resp.TypeUrl]
	if state == nil {
		state = &typeWatchState{
			resources: make(map[string]*anypb.Any),
		}
		c.typeState[resp.TypeUrl] = state
	}

	version := resp.VersionInfo
	nonce := resp.Nonce

	events, err := decodeDiscoveryResponse(resp.TypeUrl, resp.Resources)
	if err != nil {
		log.Printf("[xDS] failed to decode response: %v", err)
		c.sendNACK(resp.TypeUrl, version, nonce, err.Error())
		return
	}

	for _, event := range events {
		if c.handle != nil {
			c.handle(event)
		}
	}

	state.version = version
	state.nonce = nonce

	for _, res := range resp.Resources {
		state.resources[getResourceName(res)] = res
	}

	c.sendACK(resp.TypeUrl, version, nonce)
}

func (c *adsClient) sendACK(typeURL, version, nonce string) {
	req := &discoveryv3.DiscoveryRequest{
		Node:          c.node,
		TypeUrl:       typeURL,
		VersionInfo:   version,
		ResponseNonce: nonce,
	}

	// Use select to avoid blocking if channel is full/closed
	select {
	case c.sendCh <- req:
	default:
	}
}

func (c *adsClient) sendNACK(typeURL, version, nonce, errMsg string) {
	req := &discoveryv3.DiscoveryRequest{
		Node:          c.node,
		TypeUrl:       typeURL,
		VersionInfo:   version,
		ResponseNonce: nonce,
		ErrorDetail: &status.Status{
			Message: errMsg,
		},
	}

	select {
	case c.sendCh <- req:
	default:
	}
}

func subscriptionsEqual(a, b subscriptions) bool {
	if !slices.Equal(a.lds, b.lds) {
		return false
	}
	if !slices.Equal(a.rds, b.rds) {
		return false
	}
	if !slices.Equal(a.cds, b.cds) {
		return false
	}
	if !slices.Equal(a.eds, b.eds) {
		return false
	}
	return true
}

func getResourceName(msg *anypb.Any) string {
	if msg == nil {
		return ""
	}
	return string(msg.Value)
}
