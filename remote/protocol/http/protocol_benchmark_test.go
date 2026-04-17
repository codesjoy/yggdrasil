package protocolhttp

import (
	"context"
	"net"
	"testing"
	"time"

	stpb "google.golang.org/genproto/googleapis/rpc/status"

	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/remote"
	"github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc/encoding/proto/codec_perf"
	"github.com/codesjoy/yggdrasil/v2/resolver"
	"github.com/codesjoy/yggdrasil/v2/stats"
)

func benchmarkHTTPServer(b *testing.B, handle remote.MethodHandle) (string, func()) {
	b.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	addr := lis.Addr().String()
	_ = lis.Close()

	if err := config.Set(config.Join(config.KeyBase, "remote", "protocol", scheme, "server", "address"), addr); err != nil {
		b.Fatal(err)
	}
	if err := config.Set(config.Join(config.KeyBase, "remote", "protocol", scheme, "server", "network"), "tcp"); err != nil {
		b.Fatal(err)
	}

	svr, err := newServer(handle)
	if err != nil {
		b.Fatal(err)
	}
	if err := svr.Start(); err != nil {
		b.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		_ = svr.Handle()
		close(done)
	}()

	return svr.Info().Address, func() {
		_ = svr.Stop(context.Background())
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			b.Fatal("server did not stop")
		}
	}
}

func benchmarkHTTPClient(b *testing.B, addr string) remote.Client {
	b.Helper()
	ep := resolver.BaseEndpoint{Address: addr, Protocol: scheme}
	cli, err := newClient(context.Background(), "bench", ep, stats.NoOpHandler, nil)
	if err != nil {
		b.Fatal(err)
	}
	return cli
}

func BenchmarkProtocolHTTPUnarySmallProto(b *testing.B) {
	addr, stop := benchmarkHTTPServer(b, func(ss remote.ServerStream) {
		if err := ss.Start(false, false); err != nil {
			panic(err)
		}
		var in stpb.Status
		if err := ss.RecvMsg(&in); err != nil {
			panic(err)
		}
		ss.Finish(&in, nil)
	})
	b.Cleanup(stop)

	cli := benchmarkHTTPClient(b, addr)
	b.Cleanup(func() { _ = cli.Close() })

	req := &stpb.Status{Code: 3, Message: "hello"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cs, err := cli.NewStream(context.Background(), nil, "/bench/echo")
		if err != nil {
			b.Fatal(err)
		}
		var out stpb.Status
		if err := cs.SendMsg(req); err != nil {
			b.Fatal(err)
		}
		if err := cs.RecvMsg(&out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProtocolHTTPUnary64KiBProto(b *testing.B) {
	payload := make([]byte, 64<<10)
	for i := range payload {
		payload[i] = byte(i)
	}

	addr, stop := benchmarkHTTPServer(b, func(ss remote.ServerStream) {
		if err := ss.Start(false, false); err != nil {
			panic(err)
		}
		var in codec_perf.Buffer
		if err := ss.RecvMsg(&in); err != nil {
			panic(err)
		}
		ss.Finish(&in, nil)
	})
	b.Cleanup(stop)

	cli := benchmarkHTTPClient(b, addr)
	b.Cleanup(func() { _ = cli.Close() })

	req := &codec_perf.Buffer{Body: payload}
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cs, err := cli.NewStream(context.Background(), nil, "/bench/echo")
		if err != nil {
			b.Fatal(err)
		}
		var out codec_perf.Buffer
		if err := cs.SendMsg(req); err != nil {
			b.Fatal(err)
		}
		if err := cs.RecvMsg(&out); err != nil {
			b.Fatal(err)
		}
	}
}
