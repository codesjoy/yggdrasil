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
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	stpb "google.golang.org/genproto/googleapis/rpc/status"

	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/internal/backoff"
	"github.com/codesjoy/yggdrasil/v2/remote"
	"github.com/codesjoy/yggdrasil/v2/resolver"
	"github.com/codesjoy/yggdrasil/v2/stats"
	"github.com/codesjoy/yggdrasil/v2/stream"
)

func newTestBackoffStrategy() backoff.Strategy {
	return backoff.Exponential{Config: backoff.Config{
		BaseDelay:  time.Millisecond,
		Multiplier: 1.0,
		Jitter:     0,
		MaxDelay:   time.Millisecond,
	}}
}

func TestClientConnOnCloseDoesNotReconnectAfterShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cc := &clientConn{
		ctx:  ctx,
		cfg:  &Config{},
		bs:   newTestBackoffStrategy(),
		addr: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1},
		onStateChange: func(remote.ClientState) {
		},
		state: remote.Shutdown,
	}

	cc.onClose()
	assert.Equal(t, remote.Shutdown, cc.State())
}

func TestClientConnOnCloseMovesToIdle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cc := &clientConn{
		ctx:  ctx,
		cfg:  &Config{},
		bs:   newTestBackoffStrategy(),
		addr: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1},
		onStateChange: func(remote.ClientState) {
		},
		state: remote.Ready,
	}

	cc.onClose()
	assert.Equal(t, remote.Idle, cc.State())
}

func reserveClientConnTestAddr(t *testing.T) string {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := lis.Addr().String()
	require.NoError(t, lis.Close())
	return addr
}

func lateStartTestMethodHandle(ss remote.ServerStream) {
	var (
		reply any
		err   error
	)
	defer func() {
		ss.Finish(reply, err)
	}()

	err = ss.Start(false, false)
	if err != nil {
		return
	}

	var req stpb.Status
	err = ss.RecvMsg(&req)
	if err != nil {
		return
	}

	req.Message += ":ok"
	reply = &req
}

func TestClientConnReconnectsAfterExplicitReconnect(t *testing.T) {
	addr := reserveClientConnTestAddr(t)
	require.NoError(t, config.Set(config.Join(config.KeyBase, "remote", "protocol", scheme, "network"), "tcp"))
	require.NoError(t, config.Set(config.Join(config.KeyBase, "remote", "protocol", scheme, "address"), addr))

	cli, err := newClient(
		context.Background(),
		"late-start-clientconn",
		resolver.BaseEndpoint{Address: addr, Protocol: scheme},
		stats.NoOpHandler,
		func(remote.ClientState) {},
	)
	require.NoError(t, err)

	cc := cli.(*clientConn)
	cc.bs = newTestBackoffStrategy()
	cc.cfg.MinConnectTimeout = time.Millisecond
	go cc.Connect()

	require.Eventually(t, func() bool {
		cc.mu.RLock()
		defer cc.mu.RUnlock()
		return cc.backoffIdx > 0
	}, time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		return cc.State() == remote.Idle
	}, time.Second, 10*time.Millisecond)

	srv, err := newServer(lateStartTestMethodHandle)
	require.NoError(t, err)
	require.NoError(t, srv.Start())

	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- srv.Handle()
	}()

	defer func() {
		require.NoError(t, cc.Close())
		require.NoError(t, srv.Stop(context.Background()))
		select {
		case err := <-serveErrCh:
			require.NoError(t, err)
		case <-time.After(2 * time.Second):
			t.Fatal("grpc server did not stop in time")
		}
	}()

	time.Sleep(20 * time.Millisecond)
	require.Equal(t, remote.Idle, cc.State())

	go cc.Connect()
	require.Eventually(t, func() bool {
		return cc.State() == remote.Ready
	}, 2*time.Second, 10*time.Millisecond)

	cs, err := cc.NewStream(context.Background(), &stream.Desc{}, "/late.Test/Unary")
	require.NoError(t, err)
	require.NoError(t, cs.SendMsg(&stpb.Status{Message: "ping"}))

	var reply stpb.Status
	require.NoError(t, cs.RecvMsg(&reply))
	assert.Equal(t, "ping:ok", reply.Message)
}
