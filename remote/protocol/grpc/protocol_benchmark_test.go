package grpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/remote"
	jsonrawenc "github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc/encoding/jsonraw"
	"github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc/encoding/proto/codec_perf"
	rawenc "github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc/encoding/raw"
	"github.com/codesjoy/yggdrasil/v2/resolver"
	"github.com/codesjoy/yggdrasil/v2/stats"
	"github.com/codesjoy/yggdrasil/v2/stream"
)

// Compare these benchmarks against the last commit that still contained the
// legacy transport to establish the pre-delete baseline:
//
//	go test -run '^$' -bench '^BenchmarkProtocol' -benchmem -count=10 ./remote/protocol/grpc
const (
	benchmarkProtoUnaryMethod = "/bench.proto/Unary"
	benchmarkProtoBidiMethod  = "/bench.proto/Bidi"
	benchmarkRawUnaryMethod   = "/bench.raw/Unary"
	benchmarkJSONUnaryMethod  = "/bench.jsonraw/Unary"
)

func BenchmarkProtocolProtoUnaryHotEndpoint(b *testing.B) {
	cc := newBenchmarkClientConn(b, "", protoBenchmarkMethodHandle(nil))
	payload := bytes.Repeat([]byte("x"), 128)

	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		msg := &codec_perf.Buffer{Body: append([]byte(nil), payload...)}
		var out codec_perf.Buffer
		for pb.Next() {
			cs, err := cc.NewStream(context.Background(), &stream.Desc{}, benchmarkProtoUnaryMethod)
			if err != nil {
				panic(err)
			}
			if err := cs.SendMsg(msg); err != nil {
				panic(err)
			}
			out.Reset()
			if err := cs.RecvMsg(&out); err != nil {
				panic(err)
			}
		}
	})
}

func BenchmarkProtocolProtoUnary64KiB(b *testing.B) {
	cc := newBenchmarkClientConn(b, "", protoBenchmarkMethodHandle(nil))
	payload := bytes.Repeat([]byte("x"), 64<<10)
	msg := &codec_perf.Buffer{Body: payload}
	var out codec_perf.Buffer

	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cs, err := cc.NewStream(context.Background(), &stream.Desc{}, benchmarkProtoUnaryMethod)
		if err != nil {
			b.Fatal(err)
		}
		if err := cs.SendMsg(msg); err != nil {
			b.Fatal(err)
		}
		out.Reset()
		if err := cs.RecvMsg(&out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProtocolProtoBidi64KiB(b *testing.B) {
	cc := newBenchmarkClientConn(b, "", protoBenchmarkMethodHandle(nil))
	payload := bytes.Repeat([]byte("x"), 64<<10)
	msg := &codec_perf.Buffer{Body: payload}
	var out codec_perf.Buffer

	cs, err := cc.NewStream(
		context.Background(),
		&stream.Desc{ServerStreams: true, ClientStreams: true},
		benchmarkProtoBidiMethod,
	)
	require.NoError(b, err)
	b.Cleanup(func() { _ = cs.CloseSend() })

	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := cs.SendMsg(msg); err != nil {
			b.Fatal(err)
		}
		out.Reset()
		if err := cs.RecvMsg(&out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProtocolRawUnarySmall(b *testing.B) {
	cc := newBenchmarkClientConn(b, "", rawReplyMethodHandle(benchmarkRawUnaryMethod, []byte("pong")))
	payload := []byte("ping")
	var out []byte

	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cs, err := cc.NewStream(
			WithCallOptions(context.Background(), CallContentSubtype(rawenc.Name)),
			&stream.Desc{},
			benchmarkRawUnaryMethod,
		)
		if err != nil {
			b.Fatal(err)
		}
		if err := cs.SendMsg(payload); err != nil {
			b.Fatal(err)
		}
		out = out[:0]
		if err := cs.RecvMsg(&out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProtocolJSONRawUnarySmall(b *testing.B) {
	reply := []byte(`{"message":"pong"}`)
	cc := newBenchmarkClientConn(b, "", rawReplyMethodHandle(benchmarkJSONUnaryMethod, reply))
	payload := []byte(`{"message":"ping"}`)
	var out []byte

	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cs, err := cc.NewStream(
			WithCallOptions(context.Background(), CallContentSubtype(jsonrawenc.Name)),
			&stream.Desc{},
			benchmarkJSONUnaryMethod,
		)
		if err != nil {
			b.Fatal(err)
		}
		if err := cs.SendMsg(payload); err != nil {
			b.Fatal(err)
		}
		out = out[:0]
		if err := cs.RecvMsg(&out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProtocolRawReceive1MiB(b *testing.B) {
	reply := bytes.Repeat([]byte("x"), 1<<20)
	cc := newBenchmarkClientConn(b, "", rawReplyMethodHandle(benchmarkRawUnaryMethod, reply))
	payload := []byte("ping")
	var out []byte

	b.ReportAllocs()
	b.SetBytes(int64(len(reply)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cs, err := cc.NewStream(
			WithCallOptions(context.Background(), CallContentSubtype(rawenc.Name)),
			&stream.Desc{},
			benchmarkRawUnaryMethod,
		)
		if err != nil {
			b.Fatal(err)
		}
		if err := cs.SendMsg(payload); err != nil {
			b.Fatal(err)
		}
		out = out[:0]
		if err := cs.RecvMsg(&out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProtocolJSONRawReceive1MiB(b *testing.B) {
	reply := []byte(`{"message":"` + strings.Repeat("x", (1<<20)-len(`{"message":""}`)) + `"}`)
	cc := newBenchmarkClientConn(b, "", rawReplyMethodHandle(benchmarkJSONUnaryMethod, reply))
	payload := []byte(`{"message":"ping"}`)
	var out []byte

	b.ReportAllocs()
	b.SetBytes(int64(len(reply)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cs, err := cc.NewStream(
			WithCallOptions(context.Background(), CallContentSubtype(jsonrawenc.Name)),
			&stream.Desc{},
			benchmarkJSONUnaryMethod,
		)
		if err != nil {
			b.Fatal(err)
		}
		if err := cs.SendMsg(payload); err != nil {
			b.Fatal(err)
		}
		out = out[:0]
		if err := cs.RecvMsg(&out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProtocolProtoHotspotConcurrentStreams(b *testing.B) {
	cc := newBenchmarkClientConn(b, "", protoBenchmarkMethodHandle(nil))
	payload := bytes.Repeat([]byte("x"), 256)

	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		cs, err := cc.NewStream(
			context.Background(),
			&stream.Desc{ServerStreams: true, ClientStreams: true},
			benchmarkProtoBidiMethod,
		)
		if err != nil {
			panic(err)
		}
		defer func() { _ = cs.CloseSend() }()

		msg := &codec_perf.Buffer{Body: append([]byte(nil), payload...)}
		var out codec_perf.Buffer
		for pb.Next() {
			if err := cs.SendMsg(msg); err != nil {
				panic(err)
			}
			out.Reset()
			if err := cs.RecvMsg(&out); err != nil {
				panic(err)
			}
		}
	})
}

func newBenchmarkClientConn(
	b *testing.B,
	compressor string,
	handle remote.MethodHandle,
) *clientConn {
	b.Helper()

	serviceName := sanitizeServiceName(fmt.Sprintf("%s-%s", b.Name(), compressor))
	configPrefix := fmt.Sprintf("{%s}", serviceName)

	require.NoError(b, config.Set(config.Join(config.KeyBase, "remote", "protocol", scheme, "network"), "tcp"))
	require.NoError(b, config.Set(config.Join(config.KeyBase, "remote", "protocol", scheme, "address"), "127.0.0.1:0"))
	require.NoError(b, config.Set(config.Join(config.KeyBase, "remote", "protocol", scheme, "codeProto"), ""))
	require.NoError(
		b,
		config.Set(config.Join(config.KeyBase, "client", configPrefix, "protocol_config", scheme, "network"), "tcp"),
	)
	require.NoError(
		b,
		config.Set(config.Join(config.KeyBase, "client", configPrefix, "protocol_config", scheme, "compressor"), compressor),
	)

	srv, err := newServer(handle)
	require.NoError(b, err)
	require.NoError(b, srv.Start())

	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- srv.Handle()
	}()

	endpoint := resolver.BaseEndpoint{Address: srv.Info().Address, Protocol: scheme}
	cli, err := newClient(
		context.Background(),
		serviceName,
		endpoint,
		stats.NoOpHandler,
		func(remote.ClientState) {},
	)
	require.NoError(b, err)
	cc := cli.(*clientConn)
	cc.Connect()
	require.Eventually(b, func() bool {
		return cc.State() == remote.Ready
	}, 5*time.Second, 20*time.Millisecond)

	b.Cleanup(func() {
		_ = cc.Close()
		require.NoError(b, srv.Stop(context.Background()))
		select {
		case err := <-serveErrCh:
			require.NoError(b, err)
		case <-time.After(2 * time.Second):
			b.Fatal("grpc server did not stop in time")
		}
	})

	return cc
}

func protoBenchmarkMethodHandle(fixedReply []byte) remote.MethodHandle {
	return func(ss remote.ServerStream) {
		var (
			reply any
			err   error
		)
		defer func() { ss.Finish(reply, err) }()

		switch ss.Method() {
		case benchmarkProtoUnaryMethod:
			err = ss.Start(false, false)
			if err != nil {
				return
			}
			var req codec_perf.Buffer
			if err = ss.RecvMsg(&req); err != nil {
				return
			}
			body := req.Body
			if fixedReply != nil {
				body = fixedReply
			}
			reply = &codec_perf.Buffer{Body: append([]byte(nil), body...)}
		case benchmarkProtoBidiMethod:
			err = ss.Start(true, true)
			if err != nil {
				return
			}
			for {
				var req codec_perf.Buffer
				err = ss.RecvMsg(&req)
				if err == io.EOF {
					err = nil
					return
				}
				if err != nil {
					return
				}
				body := req.Body
				if fixedReply != nil {
					body = fixedReply
				}
				if err = ss.SendMsg(&codec_perf.Buffer{Body: append([]byte(nil), body...)}); err != nil {
					return
				}
			}
		default:
			err = fmt.Errorf("unknown method %s", ss.Method())
		}
	}
}

func rawReplyMethodHandle(method string, replyPayload []byte) remote.MethodHandle {
	return func(ss remote.ServerStream) {
		var err error
		defer func() { ss.Finish(replyPayload, err) }()

		if ss.Method() != method {
			err = fmt.Errorf("unknown method %s", ss.Method())
			return
		}
		if err = ss.Start(false, false); err != nil {
			return
		}
		var req []byte
		err = ss.RecvMsg(&req)
	}
}
