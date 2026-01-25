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
