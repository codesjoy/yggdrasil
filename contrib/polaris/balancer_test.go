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
	"errors"
	"testing"

	"github.com/codesjoy/yggdrasil/v2/balancer"
	"github.com/codesjoy/yggdrasil/v2/remote"
	"github.com/codesjoy/yggdrasil/v2/resolver"
	"github.com/codesjoy/yggdrasil/v2/stream"
	polarisgo "github.com/polarismesh/polaris-go"
	"github.com/polarismesh/polaris-go/pkg/model"
)

type fakeRemoteClient struct {
	scheme string
}

func (f *fakeRemoteClient) NewStream(
	context.Context,
	*stream.Desc,
	string,
) (stream.ClientStream, error) {
	return nil, nil
}
func (f *fakeRemoteClient) Close() error   { return nil }
func (f *fakeRemoteClient) Scheme() string { return f.scheme }
func (f *fakeRemoteClient) State() remote.State {
	return remote.Ready
}
func (f *fakeRemoteClient) Connect() {}

type fakeBalancerClient struct {
	lastPicker balancer.Picker
}

func (f *fakeBalancerClient) UpdateState(state balancer.State) {
	f.lastPicker = state.Picker
}

func (f *fakeBalancerClient) NewRemoteClient(
	endpoint resolver.Endpoint,
	_ balancer.NewRemoteClientOptions,
) (remote.Client, error) {
	return &fakeRemoteClient{scheme: endpoint.Name()}, nil
}

type fakeRemoteClientWithState struct {
	scheme string
	state  remote.State
}

func (f *fakeRemoteClientWithState) NewStream(
	context.Context,
	*stream.Desc,
	string,
) (stream.ClientStream, error) {
	return nil, nil
}
func (f *fakeRemoteClientWithState) Close() error   { return nil }
func (f *fakeRemoteClientWithState) Scheme() string { return f.scheme }
func (f *fakeRemoteClientWithState) State() remote.State {
	return f.state
}
func (f *fakeRemoteClientWithState) Connect() {}

type fakeBalancerClientWithStates struct {
	lastPicker balancer.Picker
	stateByID  map[string]remote.State
}

func (f *fakeBalancerClientWithStates) UpdateState(state balancer.State) {
	f.lastPicker = state.Picker
}

func (f *fakeBalancerClientWithStates) NewRemoteClient(
	endpoint resolver.Endpoint,
	_ balancer.NewRemoteClientOptions,
) (remote.Client, error) {
	id, _ := endpoint.GetAttributes()["instance_id"].(string)
	st := remote.Ready
	if f.stateByID != nil {
		if s, ok := f.stateByID[id]; ok {
			st = s
		}
	}
	return &fakeRemoteClientWithState{scheme: endpoint.Name(), state: st}, nil
}

type fakeRouter struct {
	pickInstanceID string
}

func (f *fakeRouter) ProcessRouters(
	req *polarisgo.ProcessRoutersRequest,
) (*model.InstancesResponse, error) {
	return req.DstInstances.(*model.InstancesResponse), nil
}

func (f *fakeRouter) ProcessLoadBalance(
	req *polarisgo.ProcessLoadBalanceRequest,
) (*model.OneInstanceResponse, error) {
	dst := req.DstInstances.(*model.InstancesResponse)
	var picked model.Instance
	for _, inst := range dst.Instances {
		if inst.GetId() == f.pickInstanceID {
			picked = inst
			break
		}
	}
	return &model.OneInstanceResponse{
		InstancesResponse: model.InstancesResponse{Instances: []model.Instance{picked}},
	}, nil
}

type assertRouterNoNonReady struct {
	forbiddenID string
}

func (r *assertRouterNoNonReady) ProcessRouters(
	req *polarisgo.ProcessRoutersRequest,
) (*model.InstancesResponse, error) {
	dst := req.DstInstances.(*model.InstancesResponse)
	for _, inst := range dst.Instances {
		if inst != nil && inst.GetId() == r.forbiddenID {
			return nil, errors.New("non-ready instance leaked into routing candidates")
		}
	}
	return dst, nil
}

func (r *assertRouterNoNonReady) ProcessLoadBalance(
	req *polarisgo.ProcessLoadBalanceRequest,
) (*model.OneInstanceResponse, error) {
	dst := req.DstInstances.(*model.InstancesResponse)
	if len(dst.Instances) == 0 {
		return nil, errors.New("empty instances")
	}
	return &model.OneInstanceResponse{
		InstancesResponse: model.InstancesResponse{Instances: []model.Instance{dst.Instances[0]}},
	}, nil
}

func TestPolarisBalancer_RoutingPickByInstanceID(t *testing.T) {
	bc := &fakeBalancerClient{}
	pb := &polarisBalancer{
		serviceName:      "svc",
		cli:              bc,
		remoteByName:     make(map[string]remote.Client),
		remoteByInstance: make(map[string]remote.Client),
		governance: governanceConfig{
			Namespace: "default",
			Routing: routingConfig{
				Enable: true,
			},
		},
		router:    &fakeRouter{pickInstanceID: "ins-2"},
		routerErr: nil,
	}

	ins1 := &fakeInstance{
		namespace: "default",
		service:   "svc",
		id:        "ins-1",
		host:      "127.0.0.1",
		port:      9000,
		protocol:  "grpc",
		healthy:   true,
		weight:    100,
		metadata:  map[string]string{},
	}
	ins2 := &fakeInstance{
		namespace: "default",
		service:   "svc",
		id:        "ins-2",
		host:      "127.0.0.1",
		port:      9001,
		protocol:  "grpc",
		healthy:   true,
		weight:    100,
		metadata:  map[string]string{},
	}
	resp := &model.InstancesResponse{
		ServiceInfo: model.ServiceInfo{Service: "svc", Namespace: "default"},
		Instances:   []model.Instance{ins1, ins2},
	}

	state := resolver.BaseState{
		Attributes: map[string]any{"polaris_instances_response": resp},
		Endpoints: []resolver.Endpoint{
			resolver.BaseEndpoint{
				Address:    "127.0.0.1:9000",
				Protocol:   "grpc",
				Attributes: map[string]any{"instance_id": "ins-1"},
			},
			resolver.BaseEndpoint{
				Address:    "127.0.0.1:9001",
				Protocol:   "grpc",
				Attributes: map[string]any{"instance_id": "ins-2"},
			},
		},
	}
	pb.UpdateState(state)

	if bc.lastPicker == nil {
		t.Fatal("expected picker to be updated")
	}
	pr, err := bc.lastPicker.Next(
		balancer.RPCInfo{Ctx: context.Background(), Method: "/svc/method"},
	)
	if err != nil {
		t.Fatalf("picker Next err: %v", err)
	}
	if pr.RemoteClient().Scheme() != "grpc/127.0.0.1:9001" {
		t.Fatalf("unexpected remote picked: %s", pr.RemoteClient().Scheme())
	}
}

func TestPolarisBalancer_RoutingFiltersNonReadyInstancesBeforeCallingRouter(t *testing.T) {
	bc := &fakeBalancerClientWithStates{
		stateByID: map[string]remote.State{
			"ins-1": remote.Ready,
			"ins-2": remote.Connecting,
		},
	}
	pb := &polarisBalancer{
		serviceName:      "svc",
		cli:              bc,
		remoteByName:     make(map[string]remote.Client),
		remoteByInstance: make(map[string]remote.Client),
		governance: governanceConfig{
			Namespace: "default",
			Routing: routingConfig{
				Enable: true,
			},
		},
		router:    &assertRouterNoNonReady{forbiddenID: "ins-2"},
		routerErr: nil,
	}

	ins1 := &fakeInstance{
		namespace: "default",
		service:   "svc",
		id:        "ins-1",
		host:      "127.0.0.1",
		port:      9000,
		protocol:  "grpc",
		healthy:   true,
		weight:    100,
		metadata:  map[string]string{},
	}
	ins2 := &fakeInstance{
		namespace: "default",
		service:   "svc",
		id:        "ins-2",
		host:      "127.0.0.1",
		port:      9001,
		protocol:  "grpc",
		healthy:   true,
		weight:    100,
		metadata:  map[string]string{},
	}
	resp := &model.InstancesResponse{
		ServiceInfo: model.ServiceInfo{Service: "svc", Namespace: "default"},
		Instances:   []model.Instance{ins1, ins2},
	}

	state := resolver.BaseState{
		Attributes: map[string]any{"polaris_instances_response": resp},
		Endpoints: []resolver.Endpoint{
			resolver.BaseEndpoint{
				Address:    "127.0.0.1:9000",
				Protocol:   "grpc",
				Attributes: map[string]any{"instance_id": "ins-1"},
			},
			resolver.BaseEndpoint{
				Address:    "127.0.0.1:9001",
				Protocol:   "grpc",
				Attributes: map[string]any{"instance_id": "ins-2"},
			},
		},
	}
	pb.UpdateState(state)

	pr, err := bc.lastPicker.Next(
		balancer.RPCInfo{Ctx: context.Background(), Method: "/svc/method"},
	)
	if err != nil {
		t.Fatalf("picker Next err: %v", err)
	}
	if pr.RemoteClient().Scheme() != "grpc/127.0.0.1:9000" {
		t.Fatalf("unexpected remote picked: %s", pr.RemoteClient().Scheme())
	}
}
