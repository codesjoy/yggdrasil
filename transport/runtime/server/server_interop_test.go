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

package server

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ggrpc "google.golang.org/grpc"
	ginsecure "google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/interop/grpc_testing"
	gmetadata "google.golang.org/grpc/metadata"

	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	"github.com/codesjoy/yggdrasil/v3/rpc/interceptor"
	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
	grpctransport "github.com/codesjoy/yggdrasil/v3/transport/protocol/grpc"
)

type runtimeMetadataService struct {
	grpc_testing.UnimplementedTestServiceServer
}

func TestRuntimeServerGRPCGoMetadataIsNotDuplicated(t *testing.T) {
	healthDisabled := false
	runtime := newTestRuntime()
	runtime.settings.Transports = []string{grpctransport.Protocol}
	runtime.serverProviders[grpctransport.Protocol] = grpctransport.ServerProviderWithSettings(
		grpctransport.Settings{
			Server: grpctransport.ServerConfig{
				Network:       "tcp",
				Address:       "127.0.0.1:0",
				HealthEnabled: &healthDisabled,
			},
		},
		stats.NoOpHandler,
		nil,
	)

	srv, err := newServer(runtime)
	require.NoError(t, err)
	srv.RegisterService(&ServiceDesc{
		ServiceName: grpc_testing.TestService_ServiceDesc.ServiceName,
		HandlerType: (*grpc_testing.TestServiceServer)(nil),
		Methods: []MethodDesc{{
			MethodName: "UnaryCall",
			Handler: func(
				srv interface{},
				ctx context.Context,
				dec func(interface{}) error,
				_ interceptor.UnaryServerInterceptor,
			) (interface{}, error) {
				if err := dec(&grpc_testing.SimpleRequest{}); err != nil {
					return nil, err
				}
				_ = metadata.SetHeader(ctx, metadata.Pairs("x-header", "one"))
				_ = metadata.SetHeader(ctx, metadata.Pairs("x-header", "two"))
				_ = metadata.SetTrailer(ctx, metadata.Pairs("x-trailer", "one"))
				_ = metadata.SetTrailer(ctx, metadata.Pairs("x-trailer", "two"))
				return &grpc_testing.SimpleResponse{Username: "ok"}, nil
			},
		}},
	}, &runtimeMetadataService{})

	startFlag := make(chan struct{})
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- srv.Serve(startFlag)
	}()
	select {
	case <-startFlag:
	case <-time.After(5 * time.Second):
		t.Fatal("runtime server did not start")
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Stop(stopCtx)
		select {
		case err := <-serveDone:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Error("runtime server did not stop")
		}
	})

	endpoints := srv.Endpoints()
	require.Len(t, endpoints, 1)
	conn, err := ggrpc.NewClient(
		endpoints[0].Address(),
		ggrpc.WithTransportCredentials(ginsecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	var header, trailer gmetadata.MD
	resp, err := grpc_testing.NewTestServiceClient(conn).UnaryCall(
		context.Background(),
		&grpc_testing.SimpleRequest{},
		ggrpc.Header(&header),
		ggrpc.Trailer(&trailer),
	)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.GetUsername())
	assert.Equal(t, []string{"one", "two"}, header.Get("x-header"))
	assert.Equal(t, []string{"one", "two"}, trailer.Get("x-trailer"))
}
