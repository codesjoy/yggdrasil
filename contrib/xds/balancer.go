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

package xds

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	mrand "math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/codesjoy/yggdrasil/v2/balancer"
	"github.com/codesjoy/yggdrasil/v2/metadata"
	"github.com/codesjoy/yggdrasil/v2/remote"
	"github.com/codesjoy/yggdrasil/v2/resolver"
)

const name = "xds"

var (
	_ = fmt.Sscanf
	_ = time.Now
)

func init() {
	balancer.RegisterBuilder(name, newXdsBalancer)
}

type xdsBalancer struct {
	name string
	cli  balancer.Client

	mu               sync.RWMutex
	remotesClient    map[string]remote.Client
	vhosts           []*VirtualHost
	clusterPolicies  map[string]clusterPolicy
	endpoints        map[string][]*weightedEndpoint
	circuitBreakers  map[string]*CircuitBreaker
	outlierDetectors map[string]*OutlierDetector
	rateLimiters     map[string]*RateLimiter
	inFlight         map[string]*int32
	rng              *mrand.Rand
}

func newXdsBalancer(name string, _ string, cli balancer.Client) (balancer.Balancer, error) {
	//nolint:gosec // G404: Weak random is acceptable for load balancing selection (non-cryptographic use)
	return &xdsBalancer{
		name:             name,
		cli:              cli,
		remotesClient:    make(map[string]remote.Client),
		vhosts:           make([]*VirtualHost, 0),
		clusterPolicies:  make(map[string]clusterPolicy),
		endpoints:        make(map[string][]*weightedEndpoint),
		circuitBreakers:  make(map[string]*CircuitBreaker),
		outlierDetectors: make(map[string]*OutlierDetector),
		rateLimiters:     make(map[string]*RateLimiter),
		inFlight:         make(map[string]*int32),
		rng:              mrand.New(mrand.NewSource(time.Now().UnixNano())),
	}, nil
}

func (b *xdsBalancer) UpdateState(state resolver.State) {
	b.mu.Lock()
	defer b.mu.Unlock()

	endpoints := state.GetEndpoints()
	if endpoints == nil {
		return
	}

	remoteCli := make(map[string]remote.Client, len(endpoints))
	for _, item := range endpoints {
		endpointKey := item.GetAddress()
		if endpointKey == "" {
			endpointKey = item.Name()
		}

		if cli, ok := b.remotesClient[endpointKey]; ok {
			remoteCli[endpointKey] = cli
			continue
		}
		cli, err := b.cli.NewRemoteClient(
			item,
			balancer.NewRemoteClientOptions{StateListener: b.UpdateRemoteClientState},
		)
		if err != nil {
			slog.Error("new remote client error", slog.Any("error", err))
			continue
		}
		if cli != nil {
			remoteCli[endpointKey] = cli
			cli.Connect()
		}
	}

	needDelClients := make([]remote.Client, 0)
	for key, rc := range b.remotesClient {
		if _, ok := remoteCli[key]; !ok {
			needDelClients = append(needDelClients, rc)
		}
	}

	b.remotesClient = remoteCli

	attributes := state.GetAttributes()
	if vhosts, ok := attributes["xds_routes"].([]*VirtualHost); ok {
		b.vhosts = vhosts
	}

	if clusters, ok := attributes["xds_clusters"].(map[string]clusterPolicy); ok {
		for name, policy := range clusters {
			b.clusterPolicies[name] = policy
			if policy.circuitBreaker != nil {
				b.circuitBreakers[name] = NewCircuitBreaker(policy.circuitBreaker)
			}
			if policy.outlierDetection != nil {
				// Clean up old detector if exists
				if old, ok := b.outlierDetectors[name]; ok {
					old.Stop()
				}
				od := NewOutlierDetector(policy.outlierDetection)
				b.outlierDetectors[name] = od
				od.Start()
			}
			if policy.rateLimiter != nil {
				if old, ok := b.rateLimiters[name]; ok {
					old.Stop()
				}
				rl := NewRateLimiter(policy.rateLimiter)
				b.rateLimiters[name] = rl
			}
		}
	}

	b.endpoints = make(map[string][]*weightedEndpoint)
	for _, ep := range endpoints {
		addr := ep.GetAddress()
		attrs := ep.GetAttributes()

		if addr == "" {
			continue
		}

		host := addr
		port := 0
		if len(addr) > 0 {
			for i := len(addr) - 1; i >= 0; i-- {
				if addr[i] == ':' {
					host = addr[:i]
					_, _ = fmt.Sscanf(addr[i+1:], "%d", &port)
					break
				}
			}
		}

		we := &weightedEndpoint{
			endpoint: Endpoint{
				Address: host,
				Port:    port,
			},
			weight:   1,
			priority: 0,
			metadata: make(map[string]string),
		}

		if weight, ok := attrs["weight"].(uint32); ok {
			we.weight = weight
		}
		if priority, ok := attrs["priority"].(uint32); ok {
			we.priority = priority
		}
		if md, ok := attrs["metadata"].(map[string]string); ok {
			we.metadata = md
		}

		clusterKey := "default"
		for key := range b.clusterPolicies {
			clusterKey = key
			break
		}
		b.endpoints[clusterKey] = append(b.endpoints[clusterKey], we)

		key := addr
		if _, ok := b.inFlight[key]; !ok {
			val := int32(0)
			b.inFlight[key] = &val
		}
	}

	picker := b.buildPicker()
	b.mu.Unlock()
	b.cli.UpdateState(balancer.State{Picker: picker})
	b.mu.Lock()

	for _, rc := range needDelClients {
		if err := rc.Close(); err != nil {
			slog.Warn(
				"remove remote client error",
				slog.String("name", name),
				slog.Any("error", err),
			)
		}
	}
}

func (b *xdsBalancer) UpdateRemoteClientState(_ remote.ClientState) {
	b.mu.RLock()
	picker := b.buildPicker()
	b.mu.RUnlock()
	b.cli.UpdateState(balancer.State{Picker: picker})
}

func (b *xdsBalancer) Close() error {
	b.mu.Lock()
	clients := make([]remote.Client, 0, len(b.remotesClient))
	for _, cli := range b.remotesClient {
		clients = append(clients, cli)
	}
	// Stop all outlier detectors
	for _, od := range b.outlierDetectors {
		od.Stop()
	}
	for _, rl := range b.rateLimiters {
		rl.Stop()
	}

	b.remotesClient = nil
	picker := b.buildPicker()
	b.mu.Unlock()
	b.cli.UpdateState(balancer.State{Picker: picker})
	var multiErr error
	for _, cli := range clients {
		if err := cli.Close(); err != nil {
			multiErr = errors.Join(multiErr, err)
		}
	}
	return multiErr
}

func (b *xdsBalancer) Type() string {
	return name
}

// BalancerStats contains statistics for the xDS balancer
type BalancerStats struct {
	CircuitBreakers  map[string]CircuitBreakerStats
	OutlierDetectors map[string]map[string]any
	RateLimiters     map[string]RateLimiterStats
}

func (b *xdsBalancer) GetStats() BalancerStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	cbStats := make(map[string]CircuitBreakerStats)
	for name, cb := range b.circuitBreakers {
		cbStats[name] = cb.GetStats()
	}

	odStats := make(map[string]map[string]interface{})
	for name, od := range b.outlierDetectors {
		odStats[name] = od.GetStats()
	}

	rlStats := make(map[string]RateLimiterStats)
	for name, rl := range b.rateLimiters {
		rlStats[name] = rl.GetStats()
	}

	return BalancerStats{
		CircuitBreakers:  cbStats,
		OutlierDetectors: odStats,
		RateLimiters:     rlStats,
	}
}

func (b *xdsBalancer) buildPicker() *xdsPicker {
	return &xdsPicker{
		balancer: b,
	}
}

type xdsPicker struct {
	balancer *xdsBalancer
}

func (p *xdsPicker) Next(ri balancer.RPCInfo) (balancer.PickResult, error) {
	p.balancer.mu.RLock()
	defer p.balancer.mu.RUnlock()

	headers := make(map[string]string)
	md, ok := metadata.FromOutContext(ri.Ctx)
	if ok {
		for k, v := range md {
			if len(v) > 0 {
				headers[k] = v[0]
			}
		}
	}

	path, ok := headers[":path"]
	if !ok {
		path = ""
	}

	cluster, circuitBreaker, rateLimiter := p.selectCluster(path, headers)
	if cluster == "" {
		return nil, balancer.ErrNoAvailableInstance
	}

	if rateLimiter != nil && !rateLimiter.Allow() {
		return nil, ErrRateLimitExceeded()
	}

	if circuitBreaker != nil {
		if !circuitBreaker.TryAcquire(ResourceRequest) {
			return nil, errors.New("circuit breaker open: max requests reached")
		}
	}

	ep := p.balancer.selectEndpoint(cluster, p.balancer.outlierDetectors[cluster])
	if ep == nil {
		if circuitBreaker != nil {
			circuitBreaker.Release(ResourceRequest)
		}
		return nil, balancer.ErrNoAvailableInstance
	}

	key := fmt.Sprintf("%s:%d", ep.endpoint.Address, ep.endpoint.Port)
	cli, ok := p.balancer.remotesClient[key]
	if !ok {
		if circuitBreaker != nil {
			circuitBreaker.Release(ResourceRequest)
		}
		return nil, balancer.ErrNoAvailableInstance
	}

	if cli.State() != remote.Ready {
		if circuitBreaker != nil {
			circuitBreaker.Release(ResourceRequest)
		}
		return nil, balancer.ErrNoAvailableInstance
	}

	return &pickResult{
		endpoint:        cli,
		ctx:             ri.Ctx,
		balancer:        p.balancer,
		inflightKey:     key,
		circuitBreaker:  circuitBreaker,
		rateLimiter:     rateLimiter,
		outlierDetector: p.balancer.outlierDetectors[cluster],
	}, nil
}

func (p *xdsPicker) selectCluster(
	path string,
	headers map[string]string,
) (string, *CircuitBreaker, *RateLimiter) {
	action := MatchRoute(p.balancer.vhosts, path, headers)
	if action == nil {
		return "", nil, nil
	}

	var cluster string
	if action.WeightedClusters != nil && len(action.WeightedClusters.Clusters) > 0 {
		cluster = p.balancer.selectWeightedCluster(action.WeightedClusters)
	} else {
		cluster = action.Cluster
	}

	var cb *CircuitBreaker
	var rl *RateLimiter
	if cluster != "" {
		cb = p.balancer.circuitBreakers[cluster]
		rl = p.balancer.rateLimiters[cluster]
	}

	return cluster, cb, rl
}

func (b *xdsBalancer) selectWeightedCluster(wc *WeightedClusters) string {
	if wc.TotalWeight == 0 {
		if len(wc.Clusters) > 0 {
			return wc.Clusters[0].Name
		}
		return ""
	}

	r := b.rng.Uint32() % wc.TotalWeight
	accumWeight := uint32(0)

	for _, c := range wc.Clusters {
		accumWeight += c.Weight
		if r < accumWeight {
			return c.Name
		}
	}

	return wc.Clusters[0].Name
}

func (b *xdsBalancer) selectEndpoint(cluster string, od *OutlierDetector) *weightedEndpoint {
	endpoints, ok := b.endpoints[cluster]
	if !ok || len(endpoints) == 0 {
		return nil
	}

	// Filter healthy endpoints (not ejected)
	var healthyEndpoints []*weightedEndpoint
	for _, ep := range endpoints {
		key := fmt.Sprintf("%s:%d", ep.endpoint.Address, ep.endpoint.Port)
		if od != nil && od.IsEjected(key) {
			continue
		}
		healthyEndpoints = append(healthyEndpoints, ep)
	}

	if len(healthyEndpoints) == 0 {
		return nil // All endpoints ejected
	}

	priorityGroups := make(map[uint32][]*weightedEndpoint)
	for _, ep := range healthyEndpoints {
		priorityGroups[ep.priority] = append(priorityGroups[ep.priority], ep)
	}

	for priority := uint32(0); priority <= 10; priority++ {
		group, ok := priorityGroups[priority]
		if !ok || len(group) == 0 {
			continue
		}

		policy, ok := b.clusterPolicies[cluster]
		if !ok {
			policy = clusterPolicy{lbPolicy: "round_robin"}
		}

		switch policy.lbPolicy {
		case "random":
			return b.selectRandom(group)
		case "least_request":
			return b.selectLeastRequest(group)
		default:
			return b.selectRoundRobin(group)
		}
	}

	return nil
}

func (b *xdsBalancer) selectRoundRobin(endpoints []*weightedEndpoint) *weightedEndpoint {
	if len(endpoints) == 0 {
		return nil
	}

	totalWeight := uint32(0)
	for _, ep := range endpoints {
		totalWeight += ep.weight
	}

	if totalWeight == 0 {
		return endpoints[0]
	}

	r := b.rng.Uint32() % totalWeight
	accumWeight := uint32(0)

	for _, ep := range endpoints {
		accumWeight += ep.weight
		if r < accumWeight {
			return ep
		}
	}

	return endpoints[0]
}

func (b *xdsBalancer) selectRandom(endpoints []*weightedEndpoint) *weightedEndpoint {
	if len(endpoints) == 0 {
		return nil
	}

	return endpoints[b.rng.Intn(len(endpoints))]
}

func (b *xdsBalancer) selectLeastRequest(endpoints []*weightedEndpoint) *weightedEndpoint {
	if len(endpoints) == 0 {
		return nil
	}

	minInFlight := int32(-1)
	var selected *weightedEndpoint

	for _, ep := range endpoints {
		key := fmt.Sprintf("%s:%d", ep.endpoint.Address, ep.endpoint.Port)
		var inFlight int32
		if val, ok := b.inFlight[key]; ok && val != nil {
			inFlight = atomic.LoadInt32(val)
		}

		if minInFlight == -1 || inFlight < minInFlight {
			minInFlight = inFlight
			selected = ep
		}
	}

	if selected != nil {
		key := fmt.Sprintf("%s:%d", selected.endpoint.Address, selected.endpoint.Port)
		if val, ok := b.inFlight[key]; ok && val != nil {
			atomic.AddInt32(val, 1)
		}
	}

	return selected
}

type pickResult struct {
	ctx             context.Context
	endpoint        remote.Client
	balancer        *xdsBalancer
	inflightKey     string
	circuitBreaker  *CircuitBreaker
	rateLimiter     *RateLimiter
	outlierDetector *OutlierDetector
}

func (p *pickResult) RemoteClient() remote.Client {
	return p.endpoint
}

func (p *pickResult) Report(err error) {
	if err != nil {
		slog.Debug("rpc call failed",
			slog.String("endpoint", p.endpoint.Scheme()),
			slog.Any("error", err),
		)
	}

	p.balancer.mu.Lock()
	defer p.balancer.mu.Unlock()

	// Release in-flight count
	if p.inflightKey != "" {
		if val := p.balancer.inFlight[p.inflightKey]; val != nil {
			if atomic.LoadInt32(val) > 0 {
				atomic.AddInt32(val, -1)
			}
		}
	}

	// Release circuit breaker
	if p.circuitBreaker != nil {
		p.circuitBreaker.Release(ResourceRequest)
	}

	// Report to outlier detector
	if p.outlierDetector != nil {
		// Default to 500 for errors, 200 for success
		statusCode := 200
		if err != nil {
			statusCode = 500
		}
		// Try to extract key from remote client address or similar if possible.
		// Here we need the address corresponding to the key.
		// Since we have inflightKey which is "address:port", we can use that.
		p.outlierDetector.ReportResult(p.inflightKey, err, statusCode)
	}
}
