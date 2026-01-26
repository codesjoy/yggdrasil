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
)

func TestRegistry_RegisterDeregister(t *testing.T) {
	ee := newEmbeddedEtcd(t)

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

	key, _, err := reg.buildKeyValue(inst)
	if err != nil {
		t.Fatalf("buildKeyValue: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := reg.Register(ctx, inst); err != nil {
		t.Fatalf("Register: %v", err)
	}

	getCtx, getCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer getCancel()
	resp, err := reg.cli.Get(getCtx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(resp.Kvs) != 1 {
		t.Fatalf("expected key exists, got %d", len(resp.Kvs))
	}

	if err := reg.Deregister(ctx, inst); err != nil {
		t.Fatalf("Deregister: %v", err)
	}
	resp, err = reg.cli.Get(getCtx, key)
	if err != nil {
		t.Fatalf("Get after deregister: %v", err)
	}
	if len(resp.Kvs) != 0 {
		t.Fatalf("expected key removed, got %d", len(resp.Kvs))
	}
}

func TestRegistry_KeepAlive(t *testing.T) {
	ee := newEmbeddedEtcd(t)

	reg, err := NewRegistry(RegistryConfig{
		Client:        ClientConfig{Endpoints: []string{ee.endpoint}},
		Prefix:        "/yggdrasil/registry",
		TTL:           3 * time.Second,
		KeepAlive:     boolPtr(true),
		RetryInterval: 500 * time.Millisecond,
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

	key, _, err := reg.buildKeyValue(inst)
	if err != nil {
		t.Fatalf("buildKeyValue: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := reg.Register(ctx, inst); err != nil {
		t.Fatalf("Register: %v", err)
	}

	time.Sleep(5 * time.Second)

	getCtx, getCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer getCancel()
	resp, err := reg.cli.Get(getCtx, key)
	if err != nil {
		t.Fatalf("Get after keepalive: %v", err)
	}
	if len(resp.Kvs) != 1 {
		t.Fatalf("expected key still exists, got %d", len(resp.Kvs))
	}
}

func boolPtr(b bool) *bool {
	return &b
}
