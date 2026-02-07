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

package k8s //nolint:staticcheck // SA1019: corev1.Endpoints and related types are deprecated in v1.33+, test kept for backward compatibility with older Kubernetes clusters

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEndpointsToState(t *testing.T) {
	r := &Resolver{cfg: ResolverConfig{Protocol: "grpc"}}
	//nolint:staticcheck // SA1019: corev1.Endpoints is deprecated in v1.33+, test kept for backward compatibility with older Kubernetes clusters
	ep := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{Name: "test-svc", Namespace: "default"},
		//nolint:staticcheck // SA1019: corev1.EndpointSubset is deprecated in v1.33+, test kept for backward compatibility with older Kubernetes clusters
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP:        "10.0.0.1",
						TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "pod-1"},
					},
					{IP: "10.0.0.2"},
				},
				Ports: []corev1.EndpointPort{
					{Name: "grpc", Port: 9090},
				},
			},
		},
	}
	state := r.endpointsToState(ep)
	if state.GetAttributes()["service"] != "test-svc" {
		t.Fatalf("expected service name test-svc, got %v", state.GetAttributes()["service"])
	}
	eps := state.GetEndpoints()
	if len(eps) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(eps))
	}
	if eps[0].GetAddress() != "10.0.0.1:9090" {
		t.Fatalf("expected address 10.0.0.1:9090, got %s", eps[0].GetAddress())
	}
}

func TestEndpointSlicesToState(t *testing.T) {
	r := &Resolver{cfg: ResolverConfig{Protocol: "http"}}
	portName := "http"
	portNum := int32(8080)
	addr1 := "10.0.0.3"
	addr2 := "10.0.0.4"
	slices := []discoveryv1.EndpointSlice{
		{
			ObjectMeta:  metav1.ObjectMeta{Name: "test-svc-abc", Namespace: "default"},
			AddressType: discoveryv1.AddressTypeIPv4,
			Ports: []discoveryv1.EndpointPort{
				{Name: &portName, Port: &portNum},
			},
			Endpoints: []discoveryv1.Endpoint{
				{
					Addresses: []string{addr1, addr2},
					TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "pod-2"},
					NodeName:  strPtr("node-1"),
					Zone:      strPtr("zone-a"),
				},
			},
		},
	}
	state := r.endpointSlicesToState(slices)
	eps := state.GetEndpoints()
	if len(eps) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(eps))
	}
	if eps[0].GetAddress() != "10.0.0.3:8080" {
		t.Fatalf("expected address 10.0.0.3:8080, got %s", eps[0].GetAddress())
	}
	attrs := eps[0].GetAttributes()
	if attrs["nodeName"] != "node-1" {
		t.Fatalf("expected nodeName node-1, got %v", attrs["nodeName"])
	}
	if attrs["zone"] != "zone-a" {
		t.Fatalf("expected zone zone-a, got %v", attrs["zone"])
	}
}

func strPtr(s string) *string {
	return &s
}
