package xds

import (
	"context"
	"fmt"
	"sync"

	"github.com/codesjoy/yggdrasil/v2/resolver"
)

type xdsResolver struct {
	name string
	cfg  ResolverConfig
	core *xdsCore
}

type appInfo struct {
	name      string
	listeners map[string]bool
	routes    map[string]bool
	clusters  map[string]bool
}

type listenerSnapshot struct {
	version string
	route   string
}

type routeSnapshot struct {
	version string
	routes  map[string]weightedRoute
}

type clusterSnapshot struct {
	version string
	policy  clusterPolicy
}

type edsSnapshot struct {
	version   string
	endpoints []*weightedEndpoint
}

type xdsCore struct {
	cfg       ResolverConfig
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.RWMutex
	apps      map[string]*appInfo
	listeners map[string]*listenerSnapshot
	routes    map[string]*routeSnapshot
	clusters  map[string]*clusterSnapshot
	endpoints map[string]*edsSnapshot
	onUpdate  func(string, resolver.State)
	ads       *adsClient
}

type xdsResolverClient struct {
	resolver *xdsResolver
}

func (c *xdsResolverClient) UpdateState(state resolver.State) {
	c.resolver.notifyState(state)
}

func NewResolver(name string, cfg ResolverConfig) (resolver.Resolver, error) {
	ctx, cancel := context.WithCancel(context.Background())

	core := &xdsCore{
		cfg:       cfg,
		ctx:       ctx,
		cancel:    cancel,
		apps:      make(map[string]*appInfo),
		listeners: make(map[string]*listenerSnapshot),
		routes:    make(map[string]*routeSnapshot),
		clusters:  make(map[string]*clusterSnapshot),
		endpoints: make(map[string]*edsSnapshot),
		onUpdate:  nil,
		ads:       nil,
	}

	return &xdsResolver{
		name: name,
		cfg:  cfg,
		core: core,
	}, nil
}

func (r *xdsResolver) Type() string {
	return "xds"
}

func (r *xdsResolver) AddWatch(target string, client resolver.Client) error {
	r.core.mu.Lock()
	defer r.core.mu.Unlock()

	if r.core.onUpdate != nil {
		r.core.onUpdate = func(s string, st resolver.State) {
			client.UpdateState(st)
		}
	}

	app, ok := r.core.apps[target]
	if !ok {
		app = &appInfo{
			name:      target,
			listeners: make(map[string]bool),
			routes:    make(map[string]bool),
			clusters:  make(map[string]bool),
		}
		r.core.apps[target] = app
	}

	listenerName, ok := r.cfg.ServiceMap[target]
	if !ok {
		listenerName = target
	}
	app.listeners[listenerName] = true

	if r.core.ads == nil {
		ads, err := newADSClient(r.core.cfg, r.core.handleDiscoveryEvent)
		if err != nil {
			return err
		}
		if err := ads.Start(); err != nil {
			return err
		}
		r.core.ads = ads
	}

	r.core.reconcileSubscriptions()
	return nil
}

func (r *xdsResolver) DelWatch(target string, client resolver.Client) error {
	r.core.mu.Lock()
	defer r.core.mu.Unlock()

	delete(r.core.apps, target)
	r.core.reconcileSubscriptions()
	return nil
}

func (r *xdsResolver) notifyState(state resolver.State) {
	if r.core.onUpdate != nil {
		r.core.onUpdate("", state)
	}
}

func (c *xdsCore) handleDiscoveryEvent(e discoveryEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch e.typ {
	case listenerAdded:
		ls := e.data.(*listenerSnapshot)
		c.listeners[e.name] = ls

	case routeAdded:
		rs := e.data.(*routeSnapshot)
		c.routes[e.name] = rs

	case clusterAdded:
		cs := e.data.(*clusterSnapshot)
		c.clusters[e.name] = cs

	case endpointAdded:
		es := e.data.(*edsSnapshot)
		c.endpoints[e.name] = es
	}

	c.reconcileSubscriptions()
	c.notifyApps()
}

func (c *xdsCore) reconcileSubscriptions() {
	var ldsNames, rdsNames, cdsNames, edsNames []string

	for _, app := range c.apps {
		for listener := range app.listeners {
			ldsNames = append(ldsNames, listener)
			if ls, ok := c.listeners[listener]; ok {
				if ls.route != "" {
					rdsNames = append(rdsNames, ls.route)
					if rs, ok := c.routes[ls.route]; ok {
						for cluster := range rs.routes {
							cdsNames = append(cdsNames, cluster)
						}
					}
				}
			}
		}
		for cluster := range app.clusters {
			cdsNames = append(cdsNames, cluster)
		}
	}

	for _, cluster := range cdsNames {
		edsNames = append(edsNames, cluster)
	}

	if c.ads != nil {
		c.ads.UpdateSubscriptions(ldsNames, rdsNames, cdsNames, edsNames)
	}
}

func (c *xdsCore) notifyApps() {
	for appName, app := range c.apps {
		var allEndpoints []*weightedEndpoint
		for listener := range app.listeners {
			if ls, ok := c.listeners[listener]; ok {
				if rs, ok := c.routes[ls.route]; ok {
					for cluster, route := range rs.routes {
						if es, ok := c.endpoints[cluster]; ok {
							for _, ep := range es.endpoints {
								we := &weightedEndpoint{
									endpoint: ep.endpoint,
									weight:   ep.weight * route.weight,
									priority: ep.priority,
									metadata: ep.metadata,
								}
								allEndpoints = append(allEndpoints, we)
							}
						}
					}
				}
			}
		}

		endpoints := make([]resolver.Endpoint, 0, len(allEndpoints))
		for _, we := range allEndpoints {
			attrs := map[string]any{
				"weight":   we.weight,
				"priority": we.priority,
				"metadata": we.metadata,
			}
			endpoints = append(endpoints, resolver.BaseEndpoint{
				Address:    fmt.Sprintf("%s:%d", we.endpoint.Address, we.endpoint.Port),
				Protocol:   c.cfg.Protocol,
				Attributes: attrs,
			})
		}

		attrs := map[string]any{
			"xds_routes":   buildRouteMap(app),
			"xds_clusters": buildClusterMap(app),
		}

		state := resolver.BaseState{
			Endpoints:  endpoints,
			Attributes: attrs,
		}

		if c.onUpdate != nil {
			c.onUpdate(appName, state)
		}
	}
}

func buildRouteMap(app *appInfo) map[string][]weightedRoute {
	routes := make(map[string][]weightedRoute)
	for listener := range app.listeners {
		routes[listener] = []weightedRoute{}
	}
	return routes
}

func buildClusterMap(app *appInfo) map[string]clusterPolicy {
	clusters := make(map[string]clusterPolicy)
	for cluster := range app.clusters {
		clusters[cluster] = clusterPolicy{}
	}
	return clusters
}
