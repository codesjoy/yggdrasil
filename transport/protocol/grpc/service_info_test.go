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
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ggrpc "google.golang.org/grpc"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	v1reflectiongrpc "google.golang.org/grpc/reflection/grpc_reflection_v1"
	v1alphareflectiongrpc "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"

	"github.com/codesjoy/yggdrasil/v3/transport/protocol/grpc/encoding/raw"
)

func newServiceInfoTestServer(config ServerConfig) *server {
	grpcServer := ggrpc.NewServer()
	s := &server{
		grpcServer: grpcServer,
		opts:       config,
	}
	s.registerStandardServices()
	return s
}

func TestServerConfigStandardServiceDefaults(t *testing.T) {
	config := ServerConfig{Address: "127.0.0.1:0"}
	require.NoError(t, config.SetDefault())
	assert.True(t, config.healthEnabled())
	assert.False(t, config.ReflectionEnabled)

	disabled := false
	config.HealthEnabled = &disabled
	require.NoError(t, config.SetDefault())
	assert.False(t, config.healthEnabled())
}

func TestServerConfigRejectsForcedCodecWithStandardServices(t *testing.T) {
	config := ServerConfig{
		Address:   "127.0.0.1:0",
		CodeProto: raw.Name,
	}
	err := config.SetDefault()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "require the proto codec")

	disabled := false
	config.HealthEnabled = &disabled
	require.NoError(t, config.SetDefault())
}

func TestServerRegisterServiceInfo(t *testing.T) {
	disabled := false
	s := newServiceInfoTestServer(ServerConfig{HealthEnabled: &disabled})
	require.NoError(t, s.RegisterServiceInfo("example.Service"))
	assert.Contains(t, s.serviceInfo.GetServiceInfo(), "example.Service")

	err := s.RegisterServiceInfo("example.Service")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestServerRegisterServiceInfoRejectsReservedNames(t *testing.T) {
	s := newServiceInfoTestServer(ServerConfig{ReflectionEnabled: true})

	err := s.RegisterServiceInfo(healthgrpc.Health_ServiceDesc.ServiceName)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reserved")

	err = s.RegisterServiceInfo(v1reflectiongrpc.ServerReflection_ServiceDesc.ServiceName)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reserved")

	err = s.RegisterServiceInfo(v1alphareflectiongrpc.ServerReflection_ServiceDesc.ServiceName)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reserved")
}

func TestServerHealthStatusLifecycle(t *testing.T) {
	s := newServiceInfoTestServer(ServerConfig{ReflectionEnabled: true})
	require.NoError(t, s.RegisterServiceInfo("example.Service"))

	check := func(service string) healthgrpc.HealthCheckResponse_ServingStatus {
		resp, err := s.health.Check(
			context.Background(),
			&healthgrpc.HealthCheckRequest{Service: service},
		)
		require.NoError(t, err)
		return resp.GetStatus()
	}

	assert.Equal(t, healthgrpc.HealthCheckResponse_NOT_SERVING, check(""))
	assert.Equal(
		t,
		healthgrpc.HealthCheckResponse_NOT_SERVING,
		check("example.Service"),
	)
	assert.Equal(
		t,
		healthgrpc.HealthCheckResponse_NOT_SERVING,
		check(healthgrpc.Health_ServiceDesc.ServiceName),
	)
	assert.Equal(
		t,
		healthgrpc.HealthCheckResponse_NOT_SERVING,
		check(v1reflectiongrpc.ServerReflection_ServiceDesc.ServiceName),
	)
	assert.Equal(
		t,
		healthgrpc.HealthCheckResponse_NOT_SERVING,
		check(v1alphareflectiongrpc.ServerReflection_ServiceDesc.ServiceName),
	)
	s.setHealthServing()
	assert.Equal(t, healthgrpc.HealthCheckResponse_SERVING, check(""))
	assert.Equal(t, healthgrpc.HealthCheckResponse_SERVING, check("example.Service"))
	assert.Equal(
		t,
		healthgrpc.HealthCheckResponse_SERVING,
		check(healthgrpc.Health_ServiceDesc.ServiceName),
	)
	assert.Equal(
		t,
		healthgrpc.HealthCheckResponse_SERVING,
		check(v1reflectiongrpc.ServerReflection_ServiceDesc.ServiceName),
	)
	s.health.Shutdown()
	assert.Equal(t, healthgrpc.HealthCheckResponse_NOT_SERVING, check("example.Service"))
	assert.Equal(
		t,
		healthgrpc.HealthCheckResponse_NOT_SERVING,
		check(v1alphareflectiongrpc.ServerReflection_ServiceDesc.ServiceName),
	)
}

func TestDynamicServiceInfoMergesRegisteredAndDynamicServices(t *testing.T) {
	s := newServiceInfoTestServer(ServerConfig{
		HealthEnabled:     boolPointer(false),
		ReflectionEnabled: true,
	})
	require.NoError(t, s.RegisterServiceInfo("example.Service"))

	info := s.serviceInfo.GetServiceInfo()
	assert.Contains(t, info, "example.Service")
	assert.Contains(t, info, v1reflectiongrpc.ServerReflection_ServiceDesc.ServiceName)
	assert.Contains(t, info, v1alphareflectiongrpc.ServerReflection_ServiceDesc.ServiceName)
}

func TestServerRegisterServiceInfoConcurrent(t *testing.T) {
	s := newServiceInfoTestServer(ServerConfig{})
	const serviceCount = 32

	var wg sync.WaitGroup
	errs := make(chan error, serviceCount)
	for i := 0; i < serviceCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			errs <- s.RegisterServiceInfo(fmt.Sprintf("example.Service%d", index))
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	info := s.serviceInfo.GetServiceInfo()
	assert.Len(t, info, serviceCount+1)
	for i := 0; i < serviceCount; i++ {
		assert.Contains(t, info, fmt.Sprintf("example.Service%d", i))
	}
}

func TestServerRegisterServiceInfoAfterServeSetsServing(t *testing.T) {
	s := newServiceInfoTestServer(ServerConfig{})
	s.serving.Store(true)

	require.NoError(t, s.RegisterServiceInfo("example.AfterServe"))
	resp, err := s.health.Check(
		context.Background(),
		&healthgrpc.HealthCheckRequest{Service: "example.AfterServe"},
	)
	require.NoError(t, err)
	assert.Equal(t, healthgrpc.HealthCheckResponse_SERVING, resp.GetStatus())
}

func boolPointer(value bool) *bool {
	return &value
}
