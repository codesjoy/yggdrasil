package http

import (
	"context"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/metadata"
	"github.com/codesjoy/yggdrasil/v2/remote"
	"github.com/codesjoy/yggdrasil/v2/resolver"
	"github.com/codesjoy/yggdrasil/v2/stats"
	"github.com/codesjoy/yggdrasil/v2/status"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/code"
	stpb "google.golang.org/genproto/googleapis/rpc/status"
)

func startHTTPTestServer(t *testing.T, handle remote.MethodHandle) (addr string, stop func()) {
	t.Helper()
	_ = config.Set(config.Join(config.KeyBase, "remote", "protocol", scheme, "server", "address"), "127.0.0.1:0")
	_ = config.Set(config.Join(config.KeyBase, "remote", "protocol", scheme, "server", "network"), "tcp")
	svr, err := newServer(handle)
	require.NoError(t, err)

	require.NoError(t, svr.Start())
	addr = svr.Info().Address
	require.NotEmpty(t, addr)

	done := make(chan struct{})
	go func() {
		_ = svr.Handle()
		close(done)
	}()

	stop = func() {
		_ = svr.Stop()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("server did not stop")
		}
	}
	return addr, stop
}

func newHTTPTestClient(t *testing.T, addr string) remote.Client {
	t.Helper()
	ep := resolver.BaseEndpoint{
		Address:  addr,
		Protocol: scheme,
	}
	cli, err := newClient(context.Background(), "test", ep, stats.NoOpHandler, nil)
	require.NoError(t, err)
	return cli
}

func TestHTTPUnaryProto(t *testing.T) {
	addr, stop := startHTTPTestServer(t, func(ss remote.ServerStream) {
		require.NoError(t, ss.Start(false, false))
		var in stpb.Status
		require.NoError(t, ss.RecvMsg(&in))
		require.NoError(t, ss.SetHeader(metadata.Pairs("h1", "v1")))
		ss.SetTrailer(metadata.Pairs("t1", "v2"))
		in.Message = in.Message + ":ok"
		ss.Finish(&in, nil)
	})
	t.Cleanup(stop)

	cli := newHTTPTestClient(t, addr)
	defer cli.Close()

	cs, err := cli.NewStream(context.Background(), nil, "/test/echo")
	require.NoError(t, err)

	req := &stpb.Status{Code: int32(code.Code_OK), Message: "hello"}
	resp := &stpb.Status{}
	require.NoError(t, cs.SendMsg(req))
	require.NoError(t, cs.RecvMsg(resp))
	require.Equal(t, int32(code.Code_OK), resp.Code)
	require.Equal(t, "hello:ok", resp.Message)

	h, err := cs.Header()
	require.NoError(t, err)
	require.Equal(t, []string{"v1"}, h.Get("h1"))
	require.Equal(t, []string{"v2"}, cs.Trailer().Get("t1"))
}

func TestHTTPUnaryJSONPbGeneric(t *testing.T) {
	addr, stop := startHTTPTestServer(t, func(ss remote.ServerStream) {
		require.NoError(t, ss.Start(false, false))
		var in stpb.Status
		require.NoError(t, ss.RecvMsg(&in))
		in.Message = in.Message + ":json"
		ss.Finish(&in, nil)
	})
	t.Cleanup(stop)

	cli := newHTTPTestClient(t, addr)
	defer cli.Close()

	cs, err := cli.NewStream(context.Background(), nil, "/test/echo")
	require.NoError(t, err)

	req := map[string]any{"code": 3, "message": "hello"}
	resp := map[string]any{}
	require.NoError(t, cs.SendMsg(req))
	require.NoError(t, cs.RecvMsg(&resp))
	require.Equal(t, float64(3), resp["code"])
	require.Equal(t, "hello:json", resp["message"])
}

func TestHTTPUnaryErrorMapping(t *testing.T) {
	addr, stop := startHTTPTestServer(t, func(ss remote.ServerStream) {
		require.NoError(t, ss.Start(false, false))
		ss.Finish(nil, status.New(code.Code_INVALID_ARGUMENT, "bad"))
	})
	t.Cleanup(stop)

	cli := newHTTPTestClient(t, addr)
	defer cli.Close()

	cs, err := cli.NewStream(context.Background(), nil, "/test/bad")
	require.NoError(t, err)

	req := &stpb.Status{Code: int32(code.Code_OK), Message: "x"}
	resp := &stpb.Status{}
	require.NoError(t, cs.SendMsg(req))
	err = cs.RecvMsg(resp)
	require.Error(t, err)
	require.True(t, status.IsCode(err, code.Code_INVALID_ARGUMENT))

	h, _ := cs.Header()
	_, ok := h["content-type"]
	require.False(t, ok)
}
