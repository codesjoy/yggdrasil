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
	"net/url"
	"testing"
	"time"

	"go.etcd.io/etcd/server/v3/embed"
)

type embeddedEtcd struct {
	etcd     *embed.Etcd
	endpoint string
}

func newEmbeddedEtcd(t *testing.T) *embeddedEtcd {
	t.Helper()
	cfg := embed.NewConfig()
	cfg.Dir = t.TempDir()
	peerURL, _ := url.Parse("http://127.0.0.1:0")
	clientURL, _ := url.Parse("http://127.0.0.1:0")
	advClientURL, _ := url.Parse("http://127.0.0.1:0")
	cfg.ListenPeerUrls = []url.URL{*peerURL}
	cfg.ListenClientUrls = []url.URL{*clientURL}
	cfg.AdvertiseClientUrls = []url.URL{*advClientURL}
	etcdSrv, err := embed.StartEtcd(cfg)
	if err != nil {
		t.Fatalf("failed to start embedded etcd: %v", err)
	}
	t.Cleanup(func() { etcdSrv.Close() })

	select {
	case <-etcdSrv.Server.ReadyNotify():
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for etcd to become ready")
	}
	return &embeddedEtcd{
		etcd:     etcdSrv,
		endpoint: etcdSrv.Clients[0].Addr().String(),
	}
}
