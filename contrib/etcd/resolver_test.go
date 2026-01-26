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

package etcd

import (
	"context"
	"testing"
	"time"

	yregistry "github.com/codesjoy/yggdrasil/v2/registry"
	yresolver "github.com/codesjoy/yggdrasil/v2/resolver"
)

func TestResolver_AddWatch(t *testing.T) {
	ee := newEmbeddedEtcd(t)

	res, err := NewResolver("test", ResolverConfig{
		Client:    ClientConfig{Endpoints: []string{ee.endpoint}},
		Prefix:    "/yggdrasil/registry",
		Namespace: "default",
	})
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	t.Cleanup(func() { _ = res.cli.Close() })

	reg, err := NewRegistry(RegistryConfig{
		Client: ClientConfig{Endpoints: []string{ee.endpoint}},
		Prefix: "/yggdrasil/registry",
		TTL:    5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	inst := demoInstance{
		namespace: "default",
		name:      "svc",
		version:   "1.0.0",
		endpoints: []yregistry.Endpoint{demoEndpoint{scheme: "grpc", address: "127.0.0.1:9000"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := reg.Register(ctx, inst); err != nil {
		t.Fatalf("Register: %v", err)
	}

	stateCh := make(chan yresolver.State, 1)
	watcher := &mockWatcher{stateCh: stateCh}

	if err := res.AddWatch("svc", watcher); err != nil {
		t.Fatalf("AddWatch: %v", err)
	}

	select {
	case st := <-stateCh:
		eps := st.GetEndpoints()
		if len(eps) == 0 {
			t.Fatal("expected at least one endpoint")
		}
		if eps[0].GetAddress() != "127.0.0.1:9000" {
			t.Fatalf("expected address 127.0.0.1:9000, got %s", eps[0].GetAddress())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for resolver state")
	}
}

func TestResolver_DelWatch(t *testing.T) {
	ee := newEmbeddedEtcd(t)

	res, err := NewResolver("test", ResolverConfig{
		Client:    ClientConfig{Endpoints: []string{ee.endpoint}},
		Prefix:    "/yggdrasil/registry",
		Namespace: "default",
	})
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	t.Cleanup(func() { _ = res.cli.Close() })

	stateCh := make(chan yresolver.State, 1)
	watcher := &mockWatcher{stateCh: stateCh}

	if err := res.AddWatch("svc", watcher); err != nil {
		t.Fatalf("AddWatch: %v", err)
	}

	if err := res.DelWatch("svc", watcher); err != nil {
		t.Fatalf("DelWatch: %v", err)
	}

	select {
	case <-stateCh:
		t.Fatal("expected no state after DelWatch")
	case <-time.After(500 * time.Millisecond):
	}
}

type mockWatcher struct {
	stateCh chan yresolver.State
}

func (m *mockWatcher) UpdateState(st yresolver.State) {
	select {
	case m.stateCh <- st:
	default:
	}
}
