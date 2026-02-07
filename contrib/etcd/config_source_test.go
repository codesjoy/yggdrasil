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

	"github.com/codesjoy/yggdrasil/v2/config/source"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestConfigSource_BlobRead(t *testing.T) {
	ee := newEmbeddedEtcd(t)

	cli, err := newClient(ClientConfig{Endpoints: []string{ee.endpoint}})
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	t.Cleanup(func() { _ = cli.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = cli.Put(ctx, "/test/config", "foo: bar")
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	src, err := NewConfigSource(ConfigSourceConfig{
		Client: ClientConfig{Endpoints: []string{ee.endpoint}},
		Key:    "/test/config",
		Mode:   ConfigSourceModeBlob,
	})
	if err != nil {
		t.Fatalf("NewConfigSource: %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })

	data, err := src.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if data.Priority() != source.PriorityRemote {
		t.Fatalf("expected PriorityRemote, got %v", data.Priority())
	}
	var m map[string]any
	if err := data.Unmarshal(&m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if m["foo"] != "bar" {
		t.Fatalf("expected foo=bar, got %v", m)
	}
}

func TestConfigSource_KVRead(t *testing.T) {
	ee := newEmbeddedEtcd(t)

	cli, err := newClient(ClientConfig{Endpoints: []string{ee.endpoint}})
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	t.Cleanup(func() { _ = cli.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ops := []clientv3.Op{
		clientv3.OpPut("/test/config/a", "1"),
		clientv3.OpPut("/test/config/b", "2"),
		clientv3.OpPut("/test/config/c/d", "3"),
	}
	_, err = cli.Txn(ctx).Then(ops...).Commit()
	if err != nil {
		t.Fatalf("Txn: %v", err)
	}

	src, err := NewConfigSource(ConfigSourceConfig{
		Client: ClientConfig{Endpoints: []string{ee.endpoint}},
		Prefix: "/test/config",
		Mode:   ConfigSourceModeKV,
	})
	if err != nil {
		t.Fatalf("NewConfigSource: %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })

	data, err := src.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if data.Priority() != source.PriorityRemote {
		t.Fatalf("expected PriorityRemote, got %v", data.Priority())
	}
	var m map[string]any
	if err := data.Unmarshal(&m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if m["a"] != 1 {
		t.Fatalf("expected a=1, got %v", m["a"])
	}
	if m["b"] != 2 {
		t.Fatalf("expected b=2, got %v", m["b"])
	}
	c, ok := m["c"].(map[string]any)
	if !ok || c["d"] != 3 {
		t.Fatalf("expected c.d=3, got %v", m["c"])
	}
}

func TestConfigSource_Watch(t *testing.T) {
	ee := newEmbeddedEtcd(t)

	cli, err := newClient(ClientConfig{Endpoints: []string{ee.endpoint}})
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	t.Cleanup(func() { _ = cli.Close() })

	src, err := NewConfigSource(ConfigSourceConfig{
		Client: ClientConfig{Endpoints: []string{ee.endpoint}},
		Key:    "/test/config/watch",
		Mode:   ConfigSourceModeBlob,
		Watch:  boolPtr(true),
	})
	if err != nil {
		t.Fatalf("NewConfigSource: %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })

	ch, err := src.Watch()
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = cli.Put(ctx, "/test/config/watch", "updated")
	}()

	select {
	case data := <-ch:
		if data.Priority() != source.PriorityRemote {
			t.Fatalf("expected PriorityRemote, got %v", data.Priority())
		}
		var m map[string]any
		if err := data.Unmarshal(&m); err == nil {
			t.Fatalf("expected parse error for plain text, got nil")
		}
		if string(data.Data()) != "updated" {
			t.Fatalf("expected updated, got %s", string(data.Data()))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for watch event")
	}
}
