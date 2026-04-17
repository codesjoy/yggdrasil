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

package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	stpb "google.golang.org/genproto/googleapis/rpc/status"

	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/remote"
	"github.com/codesjoy/yggdrasil/v2/resolver"
	"github.com/codesjoy/yggdrasil/v2/stats"
	"github.com/codesjoy/yggdrasil/v2/stream"
)

func blockingStopTestMethodHandle(entered chan<- struct{}, release <-chan struct{}) remote.MethodHandle {
	return func(ss remote.ServerStream) {
		var (
			reply any
			err   error
		)
		defer func() {
			ss.Finish(reply, err)
		}()

		if err = ss.Start(false, false); err != nil {
			return
		}
		var req stpb.Status
		if err = ss.RecvMsg(&req); err != nil {
			return
		}
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		reply = &stpb.Status{Message: req.Message}
	}
}

func TestServerStopRespectsContextDeadline(t *testing.T) {
	require.NoError(t, config.Set(config.Join(config.KeyBase, "remote", "protocol", scheme, "network"), "tcp"))
	require.NoError(t, config.Set(config.Join(config.KeyBase, "remote", "protocol", scheme, "address"), "127.0.0.1:0"))

	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	srv, err := newServer(blockingStopTestMethodHandle(entered, release))
	require.NoError(t, err)
	require.NoError(t, srv.Start())

	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- srv.Handle()
	}()

	endpoint := resolver.BaseEndpoint{
		Address:  srv.Info().Address,
		Protocol: scheme,
	}
	cli, err := newClient(
		context.Background(),
		"grpc-stop-deadline",
		endpoint,
		stats.NoOpHandler,
		func(remote.ClientState) {},
	)
	require.NoError(t, err)
	cc := cli.(*clientConn)
	cc.Connect()
	require.Eventually(t, func() bool {
		return cc.State() == remote.Ready
	}, 5*time.Second, 20*time.Millisecond)

	cs, err := cc.NewStream(context.Background(), &stream.Desc{}, "/stop.Test/Unary")
	require.NoError(t, err)
	require.NoError(t, cs.SendMsg(&stpb.Status{Message: "ping"}))

	recvErrCh := make(chan error, 1)
	go func() {
		var reply stpb.Status
		recvErrCh <- cs.RecvMsg(&reply)
	}()

	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("blocking rpc did not start in time")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	err = srv.Stop(stopCtx)
	elapsed := time.Since(start)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, elapsed, 250*time.Millisecond)

	close(release)
	require.NoError(t, cc.Close())
	select {
	case <-recvErrCh:
	case <-time.After(2 * time.Second):
		t.Fatal("client recv did not finish in time")
	}
	select {
	case err := <-serveErrCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("grpc server did not stop in time")
	}
}
