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

package polaris

import (
	"context"
	"testing"
	"time"

	polarisgo "github.com/polarismesh/polaris-go"
	"github.com/polarismesh/polaris-go/pkg/model"
)

type fakeConsumer struct {
	resp *model.InstancesResponse
	err  error
}

func (f *fakeConsumer) GetInstances(
	req *polarisgo.GetInstancesRequest,
) (*model.InstancesResponse, error) {
	return f.resp, f.err
}

type fakeCBStatus struct{}

func (s *fakeCBStatus) GetCircuitBreaker() string            { return "" }
func (s *fakeCBStatus) GetStatus() model.Status              { return 0 }
func (s *fakeCBStatus) GetStartTime() time.Time              { return time.Time{} }
func (s *fakeCBStatus) GetFallbackInfo() *model.FallbackInfo { return nil }
func (s *fakeCBStatus) SetFallbackInfo(*model.FallbackInfo)  {}

type fakeInstance struct {
	namespace string
	service   string
	id        string
	host      string
	port      uint32
	protocol  string
	version   string
	weight    int
	priority  uint32
	metadata  map[string]string
	healthy   bool
	isolated  bool
	region    string
	zone      string
	campus    string
	revision  string
	ttl       int64
}

func (i *fakeInstance) GetInstanceKey() model.InstanceKey { return model.InstanceKey{} }
func (i *fakeInstance) GetNamespace() string              { return i.namespace }
func (i *fakeInstance) GetService() string                { return i.service }
func (i *fakeInstance) GetId() string                     { return i.id } // nolint:revive
func (i *fakeInstance) GetHost() string                   { return i.host }
func (i *fakeInstance) GetPort() uint32                   { return i.port }
func (i *fakeInstance) GetVpcId() string                  { return "" } // nolint:revive
func (i *fakeInstance) GetProtocol() string               { return i.protocol }
func (i *fakeInstance) GetVersion() string                { return i.version }
func (i *fakeInstance) GetWeight() int                    { return i.weight }
func (i *fakeInstance) GetPriority() uint32               { return i.priority }
func (i *fakeInstance) GetMetadata() map[string]string    { return i.metadata }
func (i *fakeInstance) GetLogicSet() string               { return "" }
func (i *fakeInstance) GetCircuitBreakerStatus() model.CircuitBreakerStatus {
	return &fakeCBStatus{}
}
func (i *fakeInstance) IsHealthy() bool           { return i.healthy }
func (i *fakeInstance) IsIsolated() bool          { return i.isolated }
func (i *fakeInstance) IsEnableHealthCheck() bool { return false }
func (i *fakeInstance) GetRegion() string         { return i.region }
func (i *fakeInstance) GetZone() string           { return i.zone }
func (i *fakeInstance) GetIDC() string            { return i.campus }
func (i *fakeInstance) GetCampus() string         { return i.campus }
func (i *fakeInstance) GetRevision() string       { return i.revision }
func (i *fakeInstance) GetTtl() int64             { return i.ttl } // nolint:revive
func (i *fakeInstance) SetHealthy(status bool)    { i.healthy = status }
func (i *fakeInstance) DeepClone() model.Instance { return i }

func TestResolver_FetchState_FiltersProtocols(t *testing.T) {
	r := &Resolver{
		name: "default",
		cfg: ResolverConfig{
			Namespace: "ns",
			Protocols: []string{"grpc"},
		},
		api: &fakeConsumer{
			resp: &model.InstancesResponse{
				Revision: "rev",
				Instances: []model.Instance{
					&fakeInstance{
						namespace: "ns",
						service:   "svc",
						id:        "i1",
						host:      "127.0.0.1",
						port:      8080,
						protocol:  "grpc",
						version:   "v1",
						weight:    100,
						priority:  0,
						metadata:  map[string]string{"k": "v"},
					},
					&fakeInstance{
						namespace: "ns",
						service:   "svc",
						id:        "i2",
						host:      "127.0.0.1",
						port:      8081,
						protocol:  "http",
					},
				},
			},
		},
	}

	state, err := r.fetchState(context.Background(), "svc")
	if err != nil {
		t.Fatalf("fetchState() error = %v", err)
	}
	endpoints := state.GetEndpoints()
	if len(endpoints) != 1 {
		t.Fatalf("endpoints len = %d, want 1", len(endpoints))
	}
	ep := endpoints[0]
	if ep.GetProtocol() != "grpc" || ep.GetAddress() != "127.0.0.1:8080" {
		t.Fatalf(
			"endpoint = (%s, %s), want (grpc, 127.0.0.1:8080)",
			ep.GetProtocol(),
			ep.GetAddress(),
		)
	}
	attrs := ep.GetAttributes()
	if attrs["instance_id"] != "i1" || attrs["k"] != "v" {
		t.Fatalf("attrs = %+v, want instance_id=i1 and k=v", attrs)
	}
}
