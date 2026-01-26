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

	yregistry "github.com/codesjoy/yggdrasil/v2/registry"
	polarisgo "github.com/polarismesh/polaris-go"
	"github.com/polarismesh/polaris-go/pkg/model"
)

type fakeProvider struct {
	registerReqs   []*polarisgo.InstanceRegisterRequest
	deregisterReqs []*polarisgo.InstanceDeRegisterRequest
	nextID         string
}

func (f *fakeProvider) RegisterInstance(
	instance *polarisgo.InstanceRegisterRequest,
) (*model.InstanceRegisterResponse, error) {
	f.registerReqs = append(f.registerReqs, instance)
	return &model.InstanceRegisterResponse{InstanceID: f.nextID}, nil
}

func (f *fakeProvider) Deregister(instance *polarisgo.InstanceDeRegisterRequest) error {
	f.deregisterReqs = append(f.deregisterReqs, instance)
	return nil
}

type testEndpoint struct {
	scheme   string
	address  string
	metadata map[string]string
}

func (e testEndpoint) Scheme() string              { return e.scheme }
func (e testEndpoint) Address() string             { return e.address }
func (e testEndpoint) Metadata() map[string]string { return e.metadata }

type testInstance struct {
	region    string
	zone      string
	campus    string
	namespace string
	name      string
	version   string
	metadata  map[string]string
	endpoints []yregistry.Endpoint
}

func (i testInstance) Region() string                  { return i.region }
func (i testInstance) Zone() string                    { return i.zone }
func (i testInstance) Campus() string                  { return i.campus }
func (i testInstance) Namespace() string               { return i.namespace }
func (i testInstance) Name() string                    { return i.name }
func (i testInstance) Version() string                 { return i.version }
func (i testInstance) Metadata() map[string]string     { return i.metadata }
func (i testInstance) Endpoints() []yregistry.Endpoint { return i.endpoints }

func TestRegistry_RegisterAndDeregister(t *testing.T) {
	fp := &fakeProvider{nextID: "instance-1"}
	r := &Registry{
		cfg: RegistryConfig{
			ServiceToken:  "token",
			TTL:           5 * time.Second,
			AutoHeartbeat: true,
		},
		api:        fp,
		registered: map[string]registeredInstance{},
	}
	inst := testInstance{
		region:    "r",
		zone:      "z",
		campus:    "c",
		namespace: "ns",
		name:      "svc",
		version:   "v1",
		metadata:  map[string]string{"k1": "v1"},
		endpoints: []yregistry.Endpoint{
			testEndpoint{
				scheme:   "grpc",
				address:  "127.0.0.1:8080",
				metadata: map[string]string{"k2": "v2"},
			},
		},
	}

	if err := r.Register(context.Background(), inst); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if got := len(fp.registerReqs); got != 1 {
		t.Fatalf("registerReqs len = %d, want 1", got)
	}
	req := fp.registerReqs[0]
	if req.Service != "svc" || req.Namespace != "ns" || req.Host != "127.0.0.1" ||
		req.Port != 8080 {
		t.Fatalf("unexpected request base fields: %+v", req.InstanceRegisterRequest)
	}
	if req.Protocol == nil || *req.Protocol != "grpc" {
		t.Fatalf("Protocol = %v, want grpc", req.Protocol)
	}
	if req.TTL == nil || *req.TTL != 5 {
		t.Fatalf("TTL = %v, want 5", req.TTL)
	}
	if !req.AutoHeartbeat {
		t.Fatalf("AutoHeartbeat = false, want true")
	}
	if req.Version == nil || *req.Version != "v1" {
		t.Fatalf("Version = %v, want v1", req.Version)
	}
	if req.Location == nil || req.Location.Region != "r" || req.Location.Zone != "z" ||
		req.Location.Campus != "c" {
		t.Fatalf("Location = %+v, want r/z/c", req.Location)
	}
	if req.Metadata["k1"] != "v1" || req.Metadata["k2"] != "v2" {
		t.Fatalf("Metadata = %+v, want merged", req.Metadata)
	}

	if err := r.Deregister(context.Background(), inst); err != nil {
		t.Fatalf("Deregister() error = %v", err)
	}
	if got := len(fp.deregisterReqs); got != 1 {
		t.Fatalf("deregisterReqs len = %d, want 1", got)
	}
	dreq := fp.deregisterReqs[0]
	if dreq.InstanceID != "instance-1" || dreq.Service != "svc" || dreq.Namespace != "ns" {
		t.Fatalf("unexpected deregister request: %+v", dreq.InstanceDeRegisterRequest)
	}
}
