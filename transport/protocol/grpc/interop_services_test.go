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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/interop/grpc_testing"
	v1reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
	v1alphareflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	gstatus "google.golang.org/grpc/status"

	remote "github.com/codesjoy/yggdrasil/v3/transport"
)

const testInteropServiceName = "grpc.testing.TestService"

func registerInteropServiceInfo(t *testing.T, srv remote.Server) {
	t.Helper()
	registrar, ok := srv.(remote.ServiceInfoRegistrar)
	require.True(t, ok)
	require.NoError(t, registrar.RegisterServiceInfo(testInteropServiceName))
}

func TestInteropHealthAndReflection(t *testing.T) {
	srv := startYGGServer(t, Settings{
		Server: ServerConfig{
			Network:           "tcp",
			Address:           "127.0.0.1:0",
			ReflectionEnabled: true,
		},
	}, handleYGGInterop, nil)
	registerInteropServiceInfo(t, srv.Server)
	conn := dialGRPCGo(t, srv.Info().Address)

	t.Run("health check and list", func(t *testing.T) {
		client := healthgrpc.NewHealthClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		overall, err := client.Check(ctx, &healthgrpc.HealthCheckRequest{})
		require.NoError(t, err)
		assert.Equal(t, healthgrpc.HealthCheckResponse_SERVING, overall.GetStatus())

		service, err := client.Check(ctx, &healthgrpc.HealthCheckRequest{
			Service: testInteropServiceName,
		})
		require.NoError(t, err)
		assert.Equal(t, healthgrpc.HealthCheckResponse_SERVING, service.GetStatus())

		list, err := client.List(ctx, &healthgrpc.HealthListRequest{})
		require.NoError(t, err)
		assert.Equal(
			t,
			healthgrpc.HealthCheckResponse_SERVING,
			list.GetStatuses()[testInteropServiceName].GetStatus(),
		)
		assert.Equal(
			t,
			healthgrpc.HealthCheckResponse_SERVING,
			list.GetStatuses()[healthgrpc.Health_ServiceDesc.ServiceName].GetStatus(),
		)

		_, err = client.Check(ctx, &healthgrpc.HealthCheckRequest{Service: "missing.Service"})
		assert.Equal(t, codes.NotFound, gstatus.Code(err))
	})

	t.Run("reflection v1", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client := v1reflectionpb.NewServerReflectionClient(conn)
		stream, err := client.ServerReflectionInfo(ctx)
		require.NoError(t, err)

		require.NoError(t, stream.Send(&v1reflectionpb.ServerReflectionRequest{
			MessageRequest: &v1reflectionpb.ServerReflectionRequest_ListServices{
				ListServices: "",
			},
		}))
		resp, err := stream.Recv()
		require.NoError(t, err)
		services := resp.GetListServicesResponse().GetService()
		names := make(map[string]struct{}, len(services))
		for _, service := range services {
			names[service.GetName()] = struct{}{}
		}
		assert.Contains(t, names, testInteropServiceName)
		assert.Contains(t, names, healthgrpc.Health_ServiceDesc.ServiceName)
		assert.Contains(
			t,
			names,
			v1reflectionpb.ServerReflection_ServiceDesc.ServiceName,
		)

		require.NoError(t, stream.Send(&v1reflectionpb.ServerReflectionRequest{
			MessageRequest: &v1reflectionpb.ServerReflectionRequest_FileContainingSymbol{
				FileContainingSymbol: testInteropServiceName,
			},
		}))
		resp, err = stream.Recv()
		require.NoError(t, err)
		assert.NotEmpty(t, resp.GetFileDescriptorResponse().GetFileDescriptorProto())
	})

	t.Run("reflection v1alpha", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client := v1alphareflectionpb.NewServerReflectionClient(conn)
		stream, err := client.ServerReflectionInfo(ctx)
		require.NoError(t, err)
		//nolint:staticcheck // The v1alpha compatibility surface must remain exercised.
		require.NoError(t, stream.Send(&v1alphareflectionpb.ServerReflectionRequest{
			MessageRequest: &v1alphareflectionpb.ServerReflectionRequest_ListServices{
				ListServices: "",
			},
		}))
		resp, err := stream.Recv()
		require.NoError(t, err)
		assert.NotEmpty(t, resp.GetListServicesResponse().GetService()) //nolint:staticcheck

		//nolint:staticcheck // The v1alpha compatibility surface must remain exercised.
		require.NoError(t, stream.Send(&v1alphareflectionpb.ServerReflectionRequest{
			MessageRequest: &v1alphareflectionpb.ServerReflectionRequest_FileContainingSymbol{
				FileContainingSymbol: testInteropServiceName,
			},
		}))
		resp, err = stream.Recv()
		require.NoError(t, err)
		//nolint:staticcheck // The v1alpha compatibility surface must remain exercised.
		assert.NotEmpty(
			t,
			resp.GetFileDescriptorResponse().GetFileDescriptorProto(),
		)
	})
}

func TestInteropHealthWatchTransitionsOnStop(t *testing.T) {
	srv := startYGGServer(t, Settings{
		Server: ServerConfig{
			Network: "tcp",
			Address: "127.0.0.1:0",
		},
	}, handleYGGInterop, nil)
	registerInteropServiceInfo(t, srv.Server)
	conn := dialGRPCGo(t, srv.Info().Address)
	client := healthgrpc.NewHealthClient(conn)

	ctx, cancel := context.WithCancel(context.Background())
	watch, err := client.Watch(ctx, &healthgrpc.HealthCheckRequest{
		Service: testInteropServiceName,
	})
	require.NoError(t, err)
	first, err := watch.Recv()
	require.NoError(t, err)
	require.Equal(t, healthgrpc.HealthCheckResponse_SERVING, first.GetStatus())

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	stopDone := make(chan error, 1)
	go func() {
		stopDone <- srv.stop(stopCtx)
	}()

	next, err := watch.Recv()
	require.NoError(t, err)
	assert.Equal(t, healthgrpc.HealthCheckResponse_NOT_SERVING, next.GetStatus())
	cancel()
	require.NoError(t, <-stopDone)
	srv.wait(t)
}

func TestInteropHealthAndReflectionDisabled(t *testing.T) {
	healthDisabled := false
	srv := startYGGServer(t, Settings{
		Server: ServerConfig{
			Network:       "tcp",
			Address:       "127.0.0.1:0",
			HealthEnabled: &healthDisabled,
		},
	}, handleYGGInterop, nil)
	conn := dialGRPCGo(t, srv.Info().Address)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := healthgrpc.NewHealthClient(conn).Check(
		ctx,
		&healthgrpc.HealthCheckRequest{},
	)
	assert.Equal(t, codes.Unimplemented, gstatus.Code(err))

	reflection := v1reflectionpb.NewServerReflectionClient(conn)
	stream, err := reflection.ServerReflectionInfo(ctx)
	require.NoError(t, err)
	require.NoError(t, stream.Send(&v1reflectionpb.ServerReflectionRequest{
		MessageRequest: &v1reflectionpb.ServerReflectionRequest_ListServices{
			ListServices: "",
		},
	}))
	_, err = stream.Recv()
	assert.Equal(t, codes.Unimplemented, gstatus.Code(err))
}

func TestInteropServiceNamesMatchGeneratedDescriptors(t *testing.T) {
	assert.Equal(t, grpc_testing.TestService_ServiceDesc.ServiceName, testInteropServiceName)
}
