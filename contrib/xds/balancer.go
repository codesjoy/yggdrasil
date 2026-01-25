package xds

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
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

	mu              sync.RWMutex
	remotesClient   map[string]remote.Client
	route           *routeTable
	clusterPolicies map[string]clusterPolicy
	endpoints       map[string][]*weightedEndpoint
	inFlight        map[string]*int32
	rng             *rand.Rand
}

type routeTable struct {
	routes []weightedRoute
}

func newXdsBalancer(name string, _ string, cli balancer.Client) (balancer.Balancer, error) {
	return &xdsBalancer{
		name:            name,
		cli:             cli,
		remotesClient:   make(map[string]remote.Client),
		route:           &routeTable{},
		clusterPolicies: make(map[string]clusterPolicy),
		endpoints:       make(map[string][]*weightedEndpoint),
		inFlight:        make(map[string]*int32),
		rng:             rand.New(rand.NewSource(time.Now().UnixNano())),
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
		if cli, ok := b.remotesClient[item.Name()]; ok {
			remoteCli[item.Name()] = cli
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
			remoteCli[item.Name()] = cli
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
	if routes, ok := attributes["xds_routes"].(map[string][]weightedRoute); ok {
		for _, rs := range routes {
			b.route.routes = append(b.route.routes, rs...)
		}
	}

	if clusters, ok := attributes["xds_clusters"].(map[string]clusterPolicy); ok {
		for name, policy := range clusters {
			b.clusterPolicies[name] = policy
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
					fmt.Sscanf(addr[i+1:], "%d", &port)
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

		key := fmt.Sprintf("%s", addr)
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
				headers[string(k)] = v[0]
			}
		}
	}

	path, ok := headers[":path"]
	if !ok {
		path = ""
	}

	cluster := p.balancer.selectCluster(path, headers)
	if cluster == "" {
		return nil, balancer.ErrNoAvailableInstance
	}

	ep := p.balancer.selectEndpoint(cluster)
	if ep == nil {
		return nil, balancer.ErrNoAvailableInstance
	}

	key := fmt.Sprintf("%s:%d", ep.endpoint.Address, ep.endpoint.Port)
	cli, ok := p.balancer.remotesClient[key]
	if !ok {
		return nil, balancer.ErrNoAvailableInstance
	}

	if cli.State() != remote.Ready {
		return nil, balancer.ErrNoAvailableInstance
	}

	return &pickResult{
		endpoint:    cli,
		ctx:         ri.Ctx,
		balancer:    p.balancer,
		inflightKey: key,
	}, nil
}

func (p *xdsBalancer) selectCluster(path string, headers map[string]string) string {
	if p.route == nil || len(p.route.routes) == 0 {
		return ""
	}

	totalWeight := uint32(0)
	for _, route := range p.route.routes {
		totalWeight += route.weight
	}

	if totalWeight == 0 {
		if len(p.route.routes) > 0 {
			return p.route.routes[0].cluster
		}
		return ""
	}

	r := p.rng.Uint32() % totalWeight
	accumWeight := uint32(0)

	for _, route := range p.route.routes {
		accumWeight += route.weight
		if r < accumWeight {
			return route.cluster
		}
	}

	return p.route.routes[0].cluster
}

func (p *xdsBalancer) selectEndpoint(cluster string) *weightedEndpoint {
	endpoints, ok := p.endpoints[cluster]
	if !ok || len(endpoints) == 0 {
		return nil
	}

	priorityGroups := make(map[uint32][]*weightedEndpoint)
	for _, ep := range endpoints {
		priorityGroups[ep.priority] = append(priorityGroups[ep.priority], ep)
	}

	for priority := uint32(0); priority <= 10; priority++ {
		group, ok := priorityGroups[priority]
		if !ok || len(group) == 0 {
			continue
		}

		policy, ok := p.clusterPolicies[cluster]
		if !ok {
			policy = clusterPolicy{lbPolicy: "round_robin"}
		}

		switch policy.lbPolicy {
		case "random":
			return p.selectRandom(group)
		case "least_request":
			return p.selectLeastRequest(group)
		default:
			return p.selectRoundRobin(group)
		}
	}

	return nil
}

func (p *xdsBalancer) selectRoundRobin(endpoints []*weightedEndpoint) *weightedEndpoint {
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

	r := p.rng.Uint32() % totalWeight
	accumWeight := uint32(0)

	for _, ep := range endpoints {
		accumWeight += ep.weight
		if r < accumWeight {
			return ep
		}
	}

	return endpoints[0]
}

func (p *xdsBalancer) selectRandom(endpoints []*weightedEndpoint) *weightedEndpoint {
	if len(endpoints) == 0 {
		return nil
	}

	return endpoints[p.rng.Intn(len(endpoints))]
}

func (p *xdsBalancer) selectLeastRequest(endpoints []*weightedEndpoint) *weightedEndpoint {
	if len(endpoints) == 0 {
		return nil
	}

	minInFlight := int32(-1)
	var selected *weightedEndpoint

	for _, ep := range endpoints {
		key := fmt.Sprintf("%s:%d", ep.endpoint.Address, ep.endpoint.Port)
		var inFlight int32
		if val, ok := p.inFlight[key]; ok && val != nil {
			inFlight = atomic.LoadInt32(val)
		}

		if minInFlight == -1 || inFlight < minInFlight {
			minInFlight = inFlight
			selected = ep
		}
	}

	if selected != nil {
		key := fmt.Sprintf("%s:%d", selected.endpoint.Address, selected.endpoint.Port)
		if val, ok := p.inFlight[key]; ok && val != nil {
			atomic.AddInt32(val, 1)
		}
	}

	return selected
}

type pickResult struct {
	ctx         context.Context
	endpoint    remote.Client
	balancer    *xdsBalancer
	inflightKey string
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

	if p.balancer != nil && p.inflightKey != "" {
		p.balancer.mu.Lock()
		defer p.balancer.mu.Unlock()
		if val := p.balancer.inFlight[p.inflightKey]; val != nil {
			if atomic.LoadInt32(val) > 0 {
				atomic.AddInt32(val, -1)
			}
		}
	}
}
