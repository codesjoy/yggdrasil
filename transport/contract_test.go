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

package transport_test

import (
	"context"
	"testing"
	"time"

	"github.com/codesjoy/pkg/basic/xerror"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
	ystatus "github.com/codesjoy/yggdrasil/v3/rpc/status"
	"github.com/codesjoy/yggdrasil/v3/rpc/stream"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
	grpctransport "github.com/codesjoy/yggdrasil/v3/transport/protocol/grpc"
	rpchttp "github.com/codesjoy/yggdrasil/v3/transport/protocol/rpchttp"
)

func TestClientServerContracts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		client       remote.TransportClientProvider
		server       remote.TransportServerProvider
		initialState remote.State
	}{
		{
			name: "grpc",
			client: grpctransport.ClientProviderWithSettings(grpctransport.Settings{
				Client: grpctransport.ClientConfig{Network: "tcp"},
			}, nil),
			server: grpctransport.ServerProviderWithSettings(grpctransport.Settings{
				Server: grpctransport.ServerConfig{Network: "tcp", Address: "127.0.0.1:0"},
			}, stats.NoOpHandler, nil),
			initialState: remote.Idle,
		},
		{
			name:   "http",
			client: rpchttp.ClientProviderWithSettings(rpchttp.Settings{}, nil, nil),
			server: rpchttp.ServerProviderWithSettings(rpchttp.Settings{
				Server: rpchttp.ServerConfig{Network: "tcp", Address: "127.0.0.1:0"},
			}, stats.NoOpHandler, nil, nil),
			initialState: remote.Ready,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runClientServerContract(t, tc.client, tc.server, tc.initialState)
		})
	}
}

func runClientServerContract(
	t *testing.T,
	clientProvider remote.TransportClientProvider,
	serverProvider remote.TransportServerProvider,
	initialState remote.State,
) {
	t.Helper()

	const (
		unaryMethod = "/pkg.Service/Unary"
		errorMethod = "/pkg.Service/Error"
	)

	server, err := serverProvider.NewServer(func(ss remote.ServerStream) {
		switch ss.Method() {
		case unaryMethod:
			if err := ss.Start(false, false); err != nil {
				ss.Finish(nil, err)
				return
			}
			inbound, ok := metadata.FromInContext(ss.Context())
			if !ok || len(inbound.Get("x-client")) != 1 ||
				inbound.Get("x-client")[0] != "contract" {
				ss.Finish(nil, xerror.New(code.Code_INVALID_ARGUMENT, "missing inbound metadata"))
				return
			}
			if err := ss.SetHeader(metadata.Pairs("x-header", "seen")); err != nil {
				ss.Finish(nil, err)
				return
			}
			ss.SetTrailer(metadata.Pairs("x-trailer", "done"))

			req := &wrapperspb.StringValue{}
			if err := ss.RecvMsg(req); err != nil {
				ss.Finish(nil, err)
				return
			}
			ss.Finish(wrapperspb.String("echo:"+req.Value), nil)
		case errorMethod:
			if err := ss.Start(false, false); err != nil {
				ss.Finish(nil, err)
				return
			}
			req := &wrapperspb.StringValue{}
			if err := ss.RecvMsg(req); err != nil {
				ss.Finish(nil, err)
				return
			}
			ss.Finish(nil, xerror.New(code.Code_INVALID_ARGUMENT, "boom"))
		default:
			ss.Finish(nil, xerror.New(code.Code_UNIMPLEMENTED, "unknown method"))
		}
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	handleDone := make(chan error, 1)
	go func() {
		handleDone <- server.Handle()
	}()
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Stop(stopCtx)
		select {
		case <-handleDone:
		case <-time.After(5 * time.Second):
			t.Fatalf("server handle did not exit")
		}
	})

	client, err := clientProvider.NewClient(
		context.Background(),
		"svc",
		resolver.BaseEndpoint{
			Protocol: clientProvider.Protocol(),
			Address:  server.Info().Address,
		},
		stats.NoOpHandler,
		nil,
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Close()
	})

	if client.Protocol() != clientProvider.Protocol() {
		t.Fatalf("protocol mismatch: got %q want %q", client.Protocol(), clientProvider.Protocol())
	}
	if client.State() != initialState {
		t.Fatalf("initial state mismatch: got %v want %v", client.State(), initialState)
	}

	callCtx := metadata.WithOutContext(context.Background(), metadata.Pairs("x-client", "contract"))
	unaryStream, err := client.NewStream(callCtx, &stream.Desc{}, unaryMethod)
	if err != nil {
		t.Fatalf("new unary stream: %v", err)
	}
	if err := unaryStream.SendMsg(wrapperspb.String("ping")); err != nil {
		t.Fatalf("send unary request: %v", err)
	}

	var reply wrapperspb.StringValue
	if err := unaryStream.RecvMsg(&reply); err != nil {
		t.Fatalf("recv unary reply: %v", err)
	}
	if reply.Value != "echo:ping" {
		t.Fatalf("unexpected unary reply: %q", reply.Value)
	}
	header, err := unaryStream.Header()
	if err != nil {
		t.Fatalf("read header: %v", err)
	}
	if got := header.Get("x-header"); len(got) != 1 || got[0] != "seen" {
		t.Fatalf("unexpected header: %#v", header)
	}
	trailer := unaryStream.Trailer()
	if got := trailer.Get("x-trailer"); len(got) != 1 || got[0] != "done" {
		t.Fatalf("unexpected trailer: %#v", trailer)
	}

	errorStream, err := client.NewStream(callCtx, &stream.Desc{}, errorMethod)
	if err != nil {
		t.Fatalf("new error stream: %v", err)
	}
	if err := errorStream.SendMsg(wrapperspb.String("bad")); err != nil {
		t.Fatalf("send error request: %v", err)
	}
	var ignored wrapperspb.StringValue
	err = errorStream.RecvMsg(&ignored)
	if err == nil {
		t.Fatal("expected error response")
	}
	st, ok := ystatus.CoverError(err)
	if !ok {
		t.Fatalf("expected status error, got %v", err)
	}
	if st.Code() != code.Code_INVALID_ARGUMENT {
		t.Fatalf("unexpected error code: %v", st.Code())
	}

	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
	if client.State() != remote.Shutdown {
		t.Fatalf("expected shutdown state, got %v", client.State())
	}
}
