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
	"fmt"
	"regexp"

	clusterType "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointType "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerType "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routeType "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"google.golang.org/protobuf/types/known/anypb"
)

type discoveryEventType int

const (
	listenerAdded discoveryEventType = iota
	routeAdded
	clusterAdded
	endpointAdded
)

type discoveryEvent struct {
	typ  discoveryEventType
	name string
	data any
}

type clusterPolicy struct {
	lbPolicy         string
	maxRequests      uint32
	circuitBreaker   *CircuitBreakerConfig
	outlierDetection *OutlierDetectionConfig
	rateLimiter      *RateLimiterConfig
}

type weightedEndpoint struct {
	endpoint Endpoint
	weight   uint32
	priority uint32
	metadata map[string]string
}

// Endpoint represents a service endpoint
type Endpoint struct {
	Address string
	Port    int
}

const (
	typeURLListener = "type.googleapis.com/envoy.config.listener.v3.Listener"
	typeURLRoute    = "type.googleapis.com/envoy.config.route.v3.RouteConfiguration"
	typeURLCluster  = "type.googleapis.com/envoy.config.cluster.v3.Cluster"
	typeURLEndpoint = "type.googleapis.com/envoy.config.endpoint.v3.ClusterLoadAssignment"
)

func decodeDiscoveryResponse(typeURL string, resources []*anypb.Any) ([]discoveryEvent, error) {
	var evs []discoveryEvent

	for _, res := range resources {
		switch typeURL {
		case typeURLListener:
			l := &listenerType.Listener{}
			if err := res.UnmarshalTo(l); err != nil {
				return nil, fmt.Errorf("failed to unmarshal listener: %w", err)
			}
			evs = append(evs, parseListener(l)...)

		case typeURLRoute:
			r := &routeType.RouteConfiguration{}
			if err := res.UnmarshalTo(r); err != nil {
				return nil, fmt.Errorf("failed to unmarshal route: %w", err)
			}
			evs = append(evs, parseRoute(r)...)

		case typeURLCluster:
			c := &clusterType.Cluster{}
			if err := res.UnmarshalTo(c); err != nil {
				return nil, fmt.Errorf("failed to unmarshal cluster: %w", err)
			}
			evs = append(evs, parseCluster(c)...)

		case typeURLEndpoint:
			e := &endpointType.ClusterLoadAssignment{}
			if err := res.UnmarshalTo(e); err != nil {
				return nil, fmt.Errorf("failed to unmarshal endpoint: %w", err)
			}
			evs = append(evs, parseEndpoint(e)...)

		default:
			return nil, fmt.Errorf("unknown type URL: %s", typeURL)
		}
	}

	return evs, nil
}

func parseListener(l *listenerType.Listener) []discoveryEvent {
	var evs []discoveryEvent

	if l == nil || l.Name == "" {
		return evs
	}

	snapshot := &listenerSnapshot{
		version: "",
		route:   "",
	}

	if l.FilterChains != nil {
		for _, fc := range l.FilterChains {
			if fc.Filters == nil {
				continue
			}
			for _, f := range fc.Filters {
				if f.Name == "envoy.filters.network.http_connection_manager" {
					snapshot.route = l.Name
				}
			}
		}
	}

	evs = append(evs, discoveryEvent{
		typ:  listenerAdded,
		name: l.Name,
		data: snapshot,
	})

	return evs
}

func parseRoute(r *routeType.RouteConfiguration) []discoveryEvent {
	var evs []discoveryEvent

	if r == nil || r.Name == "" {
		return evs
	}

	snapshot := &routeSnapshot{
		version: "",
		vhosts:  make([]*VirtualHost, 0),
	}

	if r.VirtualHosts != nil {
		snapshot.vhosts = make([]*VirtualHost, 0, len(r.VirtualHosts))
		for _, vh := range r.VirtualHosts {
			v := &VirtualHost{
				Name:    vh.Name,
				Domains: vh.Domains,
				Routes:  make([]*Route, 0, len(vh.Routes)),
			}
			for _, route := range vh.Routes {
				match := parseRouteMatch(route.Match)
				action := parseRouteAction(route.GetRoute())
				v.Routes = append(v.Routes, &Route{
					Match:  match,
					Action: action,
				})
			}
			snapshot.vhosts = append(snapshot.vhosts, v)
		}
	}

	evs = append(evs, discoveryEvent{
		typ:  routeAdded,
		name: r.Name,
		data: snapshot,
	})

	return evs
}

func parseCluster(c *clusterType.Cluster) []discoveryEvent {
	var evs []discoveryEvent

	if c == nil || c.Name == "" {
		return evs
	}

	snapshot := &clusterSnapshot{
		version: "",
		policy: clusterPolicy{
			lbPolicy:    "round_robin",
			maxRequests: 0,
		},
	}

	switch c.LbPolicy {
	case clusterType.Cluster_ROUND_ROBIN:
		snapshot.policy.lbPolicy = "round_robin"
	case clusterType.Cluster_RANDOM:
		snapshot.policy.lbPolicy = "random"
	case clusterType.Cluster_LEAST_REQUEST:
		snapshot.policy.lbPolicy = "least_request"
	default:
		snapshot.policy.lbPolicy = "round_robin"
	}

	//nolint:staticcheck // SA1019: MaxRequestsPerConnection is deprecated but still used in older xDS configs
	if c.MaxRequestsPerConnection != nil { //nolint:staticcheck
		snapshot.policy.maxRequests = c.MaxRequestsPerConnection.Value
	}

	// Parse Circuit Breaker thresholds
	if c.CircuitBreakers != nil && len(c.CircuitBreakers.Thresholds) > 0 {
		threshold := c.CircuitBreakers.Thresholds[0]
		snapshot.policy.circuitBreaker = &CircuitBreakerConfig{
			MaxConnections:     threshold.MaxConnections.GetValue(),
			MaxPendingRequests: threshold.MaxPendingRequests.GetValue(),
			MaxRequests:        threshold.MaxRequests.GetValue(),
			MaxRetries:         threshold.MaxRetries.GetValue(),
		}
	}

	evs = append(evs, discoveryEvent{
		typ:  clusterAdded,
		name: c.Name,
		data: snapshot,
	})

	return evs
}

func parseEndpoint(e *endpointType.ClusterLoadAssignment) []discoveryEvent {
	var evs []discoveryEvent

	if e == nil || e.ClusterName == "" {
		return evs
	}

	snapshot := &edsSnapshot{
		version:   "",
		endpoints: make([]*weightedEndpoint, 0),
	}

	if e.Endpoints != nil {
		for _, localityLb := range e.Endpoints {
			locality := localityLb.GetLocality()
			localityWeight := localityLb.GetLoadBalancingWeight().GetValue()
			if localityWeight == 0 {
				localityWeight = 1
			}
			priority := localityLb.GetPriority()

			if localityLb.LbEndpoints != nil {
				for _, ep := range localityLb.LbEndpoints {
					var endpoint Endpoint
					weight := ep.GetLoadBalancingWeight().GetValue()
					if weight == 0 {
						weight = 1
					}

					metadata := make(map[string]string)
					if locality != nil {
						if locality.Region != "" {
							metadata["region"] = locality.Region
						}
						if locality.Zone != "" {
							metadata["zone"] = locality.Zone
						}
						if locality.SubZone != "" {
							metadata["sub_zone"] = locality.SubZone
						}
					}

					healthStatus := "UNKNOWN"
					switch ep.HealthStatus {
					case 0:
						healthStatus = "HEALTHY"
					case 1:
						healthStatus = "UNHEALTHY"
					case 2:
						healthStatus = "DRAINING"
					case 3:
						healthStatus = "TIMEOUT"
					case 4:
						healthStatus = "DEGRADED"
					}
					metadata["health"] = healthStatus

					if ep.HostIdentifier != nil {
						if addr := ep.GetEndpoint().GetAddress(); addr != nil {
							endpoint.Address = addr.GetSocketAddress().GetAddress()
							endpoint.Port = int(addr.GetSocketAddress().GetPortValue())
						}
					}

					we := &weightedEndpoint{
						endpoint: endpoint,
						weight:   weight * localityWeight,
						priority: priority,
						metadata: metadata,
					}

					snapshot.endpoints = append(snapshot.endpoints, we)
				}
			}
		}
	}

	evs = append(evs, discoveryEvent{
		typ:  endpointAdded,
		name: e.ClusterName,
		data: snapshot,
	})

	return evs
}

func parseRouteMatch(m *routeType.RouteMatch) *RouteMatch {
	if m == nil {
		return nil
	}

	rm := &RouteMatch{}

	if m.PathSpecifier != nil {
		switch p := m.PathSpecifier.(type) {
		case *routeType.RouteMatch_Prefix:
			rm.Prefix = p.Prefix
		case *routeType.RouteMatch_Path:
			rm.Path = p.Path
		case *routeType.RouteMatch_SafeRegex:
			if p.SafeRegex != nil && p.SafeRegex.Regex != "" {
				if r, err := regexp.Compile(p.SafeRegex.Regex); err == nil {
					rm.Regex = r
				}
			}
		}
	}

	if len(m.Headers) > 0 {
		for _, h := range m.Headers {
			hm := &HeaderMatcher{
				Name: h.Name,
			}

			switch spec := h.HeaderMatchSpecifier.(type) {
			case *routeType.HeaderMatcher_ExactMatch:
				hm.ExactMatch = spec.ExactMatch //nolint:staticcheck
			case *routeType.HeaderMatcher_SafeRegexMatch:
				if spec.SafeRegexMatch != nil && spec.SafeRegexMatch.Regex != "" { //nolint:staticcheck
					if r, err := regexp.Compile(spec.SafeRegexMatch.Regex); err == nil { //nolint:staticcheck
						hm.RegexMatch = r
					}
				}
			case *routeType.HeaderMatcher_PresentMatch:
				hm.Present = spec.PresentMatch
			case *routeType.HeaderMatcher_PrefixMatch:
				hm.PrefixMatch = spec.PrefixMatch //nolint:staticcheck
			case *routeType.HeaderMatcher_SuffixMatch:
				hm.SuffixMatch = spec.SuffixMatch //nolint:staticcheck
			}

			rm.Headers = append(rm.Headers, hm)
		}
	}

	return rm
}

func parseRouteAction(r *routeType.RouteAction) *RouteAction {
	if r == nil {
		return nil
	}

	ra := &RouteAction{}

	switch c := r.ClusterSpecifier.(type) {
	case *routeType.RouteAction_Cluster:
		ra.Cluster = c.Cluster
	case *routeType.RouteAction_WeightedClusters:
		if c.WeightedClusters != nil {
			wc := &WeightedClusters{
				TotalWeight: 0,
			}
			if c.WeightedClusters.TotalWeight != nil { //nolint:staticcheck
				wc.TotalWeight = c.WeightedClusters.TotalWeight.Value //nolint:staticcheck
			}
			for _, cl := range c.WeightedClusters.Clusters {
				w := uint32(0)
				if cl.Weight != nil {
					w = cl.Weight.Value
				}
				wc.Clusters = append(wc.Clusters, &WeightedCluster{
					Name:   cl.Name,
					Weight: w,
				})
			}
			ra.WeightedClusters = wc
		}
	}

	return ra
}
