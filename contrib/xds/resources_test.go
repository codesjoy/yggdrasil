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
	"testing"

	clusterType "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointType "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerType "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routeType "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestParseListener(t *testing.T) {
	tests := []struct {
		name     string
		listener *listenerType.Listener
		wantLen  int
	}{
		{
			name: "basic listener with HTTP filter",
			listener: &listenerType.Listener{
				Name: "test-listener",
				FilterChains: []*listenerType.FilterChain{
					{
						Filters: []*listenerType.Filter{
							{
								Name: "envoy.filters.network.http_connection_manager",
							},
						},
					},
				},
			},
			wantLen: 1,
		},
		{
			name:     "empty listener",
			listener: &listenerType.Listener{},
			wantLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evs := parseListener(tt.listener)
			if len(evs) != tt.wantLen {
				t.Errorf("parseListener() returned %d events, want %d", len(evs), tt.wantLen)
			}
		})
	}
}

func TestParseRoute(t *testing.T) {
	tests := []struct {
		name  string
		route *routeType.RouteConfiguration
		want  int
	}{
		{
			name: "basic route with single cluster",
			route: &routeType.RouteConfiguration{
				Name: "test-route",
				VirtualHosts: []*routeType.VirtualHost{
					{
						Routes: []*routeType.Route{
							{
								Match:  &routeType.RouteMatch{},
								Action: nil,
							},
						},
					},
				},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evs := parseRoute(tt.route)
			if len(evs) != tt.want {
				t.Errorf("parseRoute() returned %d events, want %d", len(evs), tt.want)
			}
		})
	}
}

func TestParseCluster(t *testing.T) {
	tests := []struct {
		name    string
		cluster *clusterType.Cluster
		wantLB  string
	}{
		{
			name: "round robin cluster",
			cluster: &clusterType.Cluster{
				Name:     "test-cluster",
				LbPolicy: clusterType.Cluster_ROUND_ROBIN,
			},
			wantLB: "round_robin",
		},
		{
			name: "random cluster",
			cluster: &clusterType.Cluster{
				Name:     "test-cluster",
				LbPolicy: clusterType.Cluster_RANDOM,
			},
			wantLB: "random",
		},
		{
			name: "least request cluster",
			cluster: &clusterType.Cluster{
				Name:     "test-cluster",
				LbPolicy: clusterType.Cluster_LEAST_REQUEST,
			},
			wantLB: "least_request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evs := parseCluster(tt.cluster)
			if len(evs) != 1 {
				t.Fatalf("parseCluster() returned %d events, want 1", len(evs))
			}
			if evs[0].typ != clusterAdded {
				t.Errorf("parseCluster() returned wrong event type")
			}
			snapshot, ok := evs[0].data.(*clusterSnapshot)
			if !ok {
				t.Fatalf("parseCluster() returned wrong data type")
			}
			if snapshot.policy.lbPolicy != tt.wantLB {
				t.Errorf(
					"parseCluster() returned lbPolicy = %v, want %v",
					snapshot.policy.lbPolicy,
					tt.wantLB,
				)
			}
		})
	}
}

func TestParseEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint *endpointType.ClusterLoadAssignment
		wantLen  int
	}{
		{
			name: "basic endpoint with locality",
			endpoint: &endpointType.ClusterLoadAssignment{
				ClusterName: "test-cluster",
				Endpoints: []*endpointType.LocalityLbEndpoints{
					{
						LbEndpoints: []*endpointType.LbEndpoint{
							{
								HostIdentifier: &endpointType.LbEndpoint_Endpoint{
									Endpoint: &endpointType.Endpoint{
										Address: &corev3.Address{
											Address: &corev3.Address_SocketAddress{
												SocketAddress: &corev3.SocketAddress{
													Address: "127.0.0.1",
													PortSpecifier: &corev3.SocketAddress_PortValue{
														PortValue: 8080,
													},
												},
											},
										},
									},
								},
								LoadBalancingWeight: wrapperspb.UInt32(10),
							},
						},
						LoadBalancingWeight: wrapperspb.UInt32(1),
						Priority:            0,
					},
				},
			},
			wantLen: 1,
		},
		{
			name:     "empty endpoint",
			endpoint: &endpointType.ClusterLoadAssignment{},
			wantLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evs := parseEndpoint(tt.endpoint)
			if len(evs) != tt.wantLen {
				t.Errorf("parseEndpoint() returned %d events, want %d", len(evs), tt.wantLen)
			}
		})
	}
}

func TestDecodeDiscoveryResponse(t *testing.T) {
	t.Run("decode listener", func(t *testing.T) {
		listener := &listenerType.Listener{
			Name: "test-listener",
			FilterChains: []*listenerType.FilterChain{
				{
					Filters: []*listenerType.Filter{
						{
							Name: "envoy.filters.network.http_connection_manager",
						},
					},
				},
			},
		}
		anyListener, err := anypb.New(listener)
		if err != nil {
			t.Fatalf("failed to create Any: %v", err)
		}

		evs, err := decodeDiscoveryResponse(typeURLListener, []*anypb.Any{anyListener})
		if err != nil {
			t.Fatalf("decodeDiscoveryResponse() error = %v", err)
		}
		if len(evs) != 1 {
			t.Errorf("decodeDiscoveryResponse() returned %d events, want 1", len(evs))
		}
	})

	t.Run("decode route", func(t *testing.T) {
		route := &routeType.RouteConfiguration{
			Name: "test-route",
			VirtualHosts: []*routeType.VirtualHost{
				{
					Routes: []*routeType.Route{
						{
							Match:  &routeType.RouteMatch{},
							Action: nil,
						},
					},
				},
			},
		}
		anyRoute, err := anypb.New(route)
		if err != nil {
			t.Fatalf("failed to create Any: %v", err)
		}

		evs, err := decodeDiscoveryResponse(typeURLRoute, []*anypb.Any{anyRoute})
		if err != nil {
			t.Fatalf("decodeDiscoveryResponse() error = %v", err)
		}
		if len(evs) != 1 {
			t.Errorf("decodeDiscoveryResponse() returned %d events, want 1", len(evs))
		}
	})

	t.Run("decode cluster", func(t *testing.T) {
		cluster := &clusterType.Cluster{
			Name:     "test-cluster",
			LbPolicy: clusterType.Cluster_ROUND_ROBIN,
		}
		anyCluster, err := anypb.New(cluster)
		if err != nil {
			t.Fatalf("failed to create Any: %v", err)
		}

		evs, err := decodeDiscoveryResponse(typeURLCluster, []*anypb.Any{anyCluster})
		if err != nil {
			t.Fatalf("decodeDiscoveryResponse() error = %v", err)
		}
		if len(evs) != 1 {
			t.Errorf("decodeDiscoveryResponse() returned %d events, want 1", len(evs))
		}
	})

	t.Run("decode endpoint", func(t *testing.T) {
		endpoint := &endpointType.ClusterLoadAssignment{
			ClusterName: "test-cluster",
			Endpoints: []*endpointType.LocalityLbEndpoints{
				{
					LbEndpoints: []*endpointType.LbEndpoint{
						{
							HostIdentifier: &endpointType.LbEndpoint_Endpoint{
								Endpoint: &endpointType.Endpoint{
									Address: &corev3.Address{
										Address: &corev3.Address_SocketAddress{
											SocketAddress: &corev3.SocketAddress{
												Address: "127.0.0.1",
												PortSpecifier: &corev3.SocketAddress_PortValue{
													PortValue: 8080,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		anyEndpoint, err := anypb.New(endpoint)
		if err != nil {
			t.Fatalf("failed to create Any: %v", err)
		}

		evs, err := decodeDiscoveryResponse(typeURLEndpoint, []*anypb.Any{anyEndpoint})
		if err != nil {
			t.Fatalf("decodeDiscoveryResponse() error = %v", err)
		}
		if len(evs) != 1 {
			t.Errorf("decodeDiscoveryResponse() returned %d events, want 1", len(evs))
		}
	})

	t.Run("unknown type URL", func(t *testing.T) {
		_, err := decodeDiscoveryResponse("unknown/type/url", []*anypb.Any{{}})
		if err == nil {
			t.Error("decodeDiscoveryResponse() expected error for unknown type URL")
		}
	})
}
