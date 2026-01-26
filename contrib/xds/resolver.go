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
	vhosts  []*VirtualHost
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
	cfg          ResolverConfig
	ctx          context.Context
	cancel       context.CancelFunc
	mu           sync.RWMutex
	apps         map[string]*appInfo
	listeners    map[string]*listenerSnapshot
	routes       map[string]*routeSnapshot
	clusters     map[string]*clusterSnapshot
	endpoints    map[string]*edsSnapshot
	onUpdate     func(string, resolver.State)
	ads          *adsClient
	dependencies map[string]*resourceDependencies
}

type resourceDependencies struct {
	listeners map[string]bool
	routes    map[string]bool
	clusters  map[string]bool
}

// NewResolver creates a new xDS resolver
func NewResolver(name string, cfg ResolverConfig) (resolver.Resolver, error) {
	ctx, cancel := context.WithCancel(context.Background())

	core := &xdsCore{
		cfg:          cfg,
		ctx:          ctx,
		cancel:       cancel,
		apps:         make(map[string]*appInfo),
		listeners:    make(map[string]*listenerSnapshot),
		routes:       make(map[string]*routeSnapshot),
		clusters:     make(map[string]*clusterSnapshot),
		endpoints:    make(map[string]*edsSnapshot),
		onUpdate:     nil,
		ads:          nil,
		dependencies: make(map[string]*resourceDependencies),
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
		r.core.onUpdate = func(_ string, st resolver.State) {
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

func (r *xdsResolver) DelWatch(target string, _ resolver.Client) error {
	r.core.mu.Lock()
	defer r.core.mu.Unlock()

	delete(r.core.apps, target)
	r.core.reconcileSubscriptions()
	return nil
}

func (c *xdsCore) handleDiscoveryEvent(e discoveryEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch e.typ {
	case listenerAdded:
		ls := e.data.(*listenerSnapshot)
		c.listeners[e.name] = ls
		c.trackDependencies(e.name, ls)

	case routeAdded:
		rs := e.data.(*routeSnapshot)
		c.routes[e.name] = rs
		c.updateRouteDependencies(e.name, rs)

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

func (c *xdsCore) trackDependencies(listenerName string, ls *listenerSnapshot) {
	if ls == nil {
		return
	}
	if _, ok := c.dependencies[listenerName]; !ok {
		c.dependencies[listenerName] = &resourceDependencies{
			listeners: make(map[string]bool),
			routes:    make(map[string]bool),
			clusters:  make(map[string]bool),
		}
	}
	if ls.route != "" {
		c.dependencies[listenerName].routes[ls.route] = true
	}
}

func (c *xdsCore) updateRouteDependencies(_ string, rs *routeSnapshot) {
	if rs == nil {
		return
	}
	for _, vh := range rs.vhosts {
		for _, route := range vh.Routes {
			if route.Action != nil {
				if route.Action.Cluster != "" {
					for _, dep := range c.dependencies {
						dep.clusters[route.Action.Cluster] = true
					}
				}
				if route.Action.WeightedClusters != nil {
					for _, wc := range route.Action.WeightedClusters.Clusters {
						for _, dep := range c.dependencies {
							dep.clusters[wc.Name] = true
						}
					}
				}
			}
		}
	}
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
						for _, vh := range rs.vhosts {
							for _, route := range vh.Routes {
								if route.Action != nil {
									if route.Action.Cluster != "" {
										cdsNames = append(cdsNames, route.Action.Cluster)
									}
									if route.Action.WeightedClusters != nil {
										for _, wc := range route.Action.WeightedClusters.Clusters {
											cdsNames = append(cdsNames, wc.Name)
										}
									}
								}
							}
						}
					}
				}
			}

			if dep, ok := c.dependencies[listener]; ok {
				for cluster := range dep.clusters {
					if !contains(cdsNames, cluster) {
						cdsNames = append(cdsNames, cluster)
					}
				}
			}
		}
		for cluster := range app.clusters {
			cdsNames = append(cdsNames, cluster)
		}
	}

	edsNames = append(edsNames, cdsNames...)

	if c.ads != nil {
		c.ads.UpdateSubscriptions(ldsNames, rdsNames, cdsNames, edsNames)
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func (c *xdsCore) notifyApps() {
	for appName, app := range c.apps {
		var allEndpoints []*weightedEndpoint
		for listener := range app.listeners {
			if ls, ok := c.listeners[listener]; ok {
				if rs, ok := c.routes[ls.route]; ok {
					clusters := make(map[string]bool)
					for _, vh := range rs.vhosts {
						for _, route := range vh.Routes {
							if route.Action != nil {
								if route.Action.Cluster != "" {
									clusters[route.Action.Cluster] = true
								}
								if route.Action.WeightedClusters != nil {
									for _, wc := range route.Action.WeightedClusters.Clusters {
										clusters[wc.Name] = true
									}
								}
							}
						}
					}

					for cluster := range clusters {
						if es, ok := c.endpoints[cluster]; ok {
							for _, ep := range es.endpoints {
								we := &weightedEndpoint{
									endpoint: ep.endpoint,
									weight:   ep.weight,
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
			"xds_routes":   buildRouteConfig(app, c.routes, c.listeners),
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

func buildRouteConfig(
	app *appInfo,
	routes map[string]*routeSnapshot,
	listeners map[string]*listenerSnapshot,
) []*VirtualHost {
	var vhosts []*VirtualHost
	for listenerName := range app.listeners {
		if ls, ok := listeners[listenerName]; ok {
			if rs, ok := routes[ls.route]; ok {
				vhosts = append(vhosts, rs.vhosts...)
			}
		}
	}
	return vhosts
}

func buildClusterMap(app *appInfo) map[string]clusterPolicy {
	clusters := make(map[string]clusterPolicy)
	for cluster := range app.clusters {
		clusters[cluster] = clusterPolicy{}
	}
	return clusters
}
