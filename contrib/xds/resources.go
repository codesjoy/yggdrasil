package xds

import (
	"fmt"

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
	data interface{}
}

type weightedRoute struct {
	cluster string
	weight  uint32
}

type clusterPolicy struct {
	lbPolicy    string
	maxRequests uint32
}

type weightedEndpoint struct {
	endpoint Endpoint
	weight   uint32
	priority uint32
	metadata map[string]string
}

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
		routes:  make(map[string]weightedRoute),
	}

	if r.VirtualHosts != nil {
		for _, vh := range r.VirtualHosts {
			if vh.Routes == nil {
				continue
			}
			if len(vh.Routes) > 0 {
				snapshot.routes["default"] = weightedRoute{
					cluster: "default",
					weight:  100,
				}
			}
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

	if c.MaxRequestsPerConnection != nil {
		snapshot.policy.maxRequests = c.MaxRequestsPerConnection.Value
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

					if ep.HostIdentifier != nil {
						if addr := ep.GetEndpoint().GetAddress(); addr != nil {
							endpoint.Address = addr.GetSocketAddress().GetAddress()
							endpoint.Port = int(addr.GetSocketAddress().GetPortValue())
						}
					}

					we := &weightedEndpoint{
						endpoint: endpoint,
						weight:   weight * localityWeight,
						priority: uint32(priority),
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
