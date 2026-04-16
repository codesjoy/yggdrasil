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

package protocolhttp

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/codesjoy/pkg/basic/xerror"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/code"
	stpb "google.golang.org/genproto/googleapis/rpc/status"

	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/metadata"
	"github.com/codesjoy/yggdrasil/v2/remote"
	"github.com/codesjoy/yggdrasil/v2/resolver"
	"github.com/codesjoy/yggdrasil/v2/stats"
	"github.com/codesjoy/yggdrasil/v2/status"
)

func startHTTPTestServer(t *testing.T, handle remote.MethodHandle) (addr string, stop func()) {
	t.Helper()
	addr = reserveHTTPTestAddr(t)
	stop = startHTTPTestServerAtAddr(t, addr, handle)
	return addr, stop
}

func reserveHTTPTestAddr(t *testing.T) string {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := lis.Addr().String()
	require.NoError(t, lis.Close())
	return addr
}

func startHTTPTestServerAtAddr(t *testing.T, addr string, handle remote.MethodHandle) func() {
	t.Helper()
	_ = config.Set(
		config.Join(config.KeyBase, "remote", "protocol", scheme, "server", "address"),
		addr,
	)
	_ = config.Set(
		config.Join(config.KeyBase, "remote", "protocol", scheme, "server", "network"),
		"tcp",
	)
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

	return func() {
		_ = svr.Stop(context.Background())
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("server did not stop")
		}
	}
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
		in.Message += ":ok"
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
		in.Message += ":json"
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
		ss.Finish(nil, xerror.New(code.Code_INVALID_ARGUMENT, "bad"))
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
	require.Equal(t, code.Code_INVALID_ARGUMENT, status.FromError(err).Code())

	h, _ := cs.Header()
	_, ok := h["content-type"]
	require.False(t, ok)
}

func TestHTTPClientStateRemainsReadyAcrossRequestFailures(t *testing.T) {
	addr, stop := startHTTPTestServer(t, func(ss remote.ServerStream) {
		require.NoError(t, ss.Start(false, false))
		ss.Finish(&stpb.Status{Code: int32(code.Code_OK), Message: "ok"}, nil)
	})

	cli := newHTTPTestClient(t, addr)
	require.Equal(t, remote.Ready, cli.State())

	stop()
	defer cli.Close() // nolint:errcheck

	cs, err := cli.NewStream(context.Background(), nil, "/test/echo")
	require.NoError(t, err)

	req := &stpb.Status{Code: int32(code.Code_OK), Message: "hello"}
	resp := &stpb.Status{}
	require.NoError(t, cs.SendMsg(req))
	err = cs.RecvMsg(resp)
	require.Error(t, err)
	require.Equal(t, remote.Ready, cli.State())
}

func TestHTTPClientRecoversWhenServerStartsLater(t *testing.T) {
	addr := reserveHTTPTestAddr(t)
	cli := newHTTPTestClient(t, addr)
	defer cli.Close()

	cs, err := cli.NewStream(context.Background(), nil, "/test/echo")
	require.NoError(t, err)
	require.NoError(t, cs.SendMsg(&stpb.Status{Code: int32(code.Code_OK), Message: "hello"}))

	var reply stpb.Status
	err = cs.RecvMsg(&reply)
	require.Error(t, err)
	require.Equal(t, remote.Ready, cli.State())

	stop := startHTTPTestServerAtAddr(t, addr, func(ss remote.ServerStream) {
		require.NoError(t, ss.Start(false, false))
		var in stpb.Status
		require.NoError(t, ss.RecvMsg(&in))
		in.Message += ":ok"
		ss.Finish(&in, nil)
	})
	t.Cleanup(stop)

	cs, err = cli.NewStream(context.Background(), nil, "/test/echo")
	require.NoError(t, err)
	require.NoError(t, cs.SendMsg(&stpb.Status{Code: int32(code.Code_OK), Message: "hello"}))
	require.NoError(t, cs.RecvMsg(&reply))
	require.Equal(t, "hello:ok", reply.Message)
	require.Equal(t, remote.Ready, cli.State())
}

func TestHTTPStopUsesContextDeadline(t *testing.T) {
	_ = config.Set(
		config.Join(config.KeyBase, "remote", "protocol", scheme, "server", "address"),
		"127.0.0.1:0",
	)
	_ = config.Set(
		config.Join(config.KeyBase, "remote", "protocol", scheme, "server", "network"),
		"tcp",
	)

	started := make(chan struct{})
	release := make(chan struct{})
	svr, err := newServer(func(ss remote.ServerStream) {
		require.NoError(t, ss.Start(false, false))
		close(started)
		<-release
		ss.Finish(&stpb.Status{Code: int32(code.Code_OK), Message: "ok"}, nil)
	})
	require.NoError(t, err)
	require.NoError(t, svr.Start())

	done := make(chan struct{})
	go func() {
		_ = svr.Handle()
		close(done)
	}()

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://"+svr.Info().Address+"/test/block",
		bytes.NewReader([]byte(`{"code":0,"message":"block"}`)),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	reqDone := make(chan struct{})
	go func() {
		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp != nil {
			_ = resp.Body.Close()
		}
		close(reqDone)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not reach handler")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	start := time.Now()
	err = svr.Stop(stopCtx)
	elapsed := time.Since(start)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, elapsed, 250*time.Millisecond)

	close(release)

	select {
	case <-reqDone:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not complete after release")
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop after request release")
	}
}
