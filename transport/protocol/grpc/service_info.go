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
	"fmt"
	"sync"

	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	v1reflectiongrpc "google.golang.org/grpc/reflection/grpc_reflection_v1"
	v1alphareflectiongrpc "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
)

type dynamicServiceInfo struct {
	server *ggrpc.Server

	mu       sync.RWMutex
	services map[string]struct{}
}

func newDynamicServiceInfo(server *ggrpc.Server) *dynamicServiceInfo {
	return &dynamicServiceInfo{
		server:   server,
		services: map[string]struct{}{},
	}
}

// GetServiceInfo implements grpc/reflection.ServiceInfoProvider.
func (s *dynamicServiceInfo) GetServiceInfo() map[string]ggrpc.ServiceInfo {
	out := s.server.GetServiceInfo()
	if out == nil {
		out = map[string]ggrpc.ServiceInfo{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for name := range s.services {
		out[name] = ggrpc.ServiceInfo{}
	}
	return out
}

func (s *dynamicServiceInfo) register(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.services[name]; exists {
		return fmt.Errorf("grpc service info %q is already registered", name)
	}
	s.services[name] = struct{}{}
	return nil
}

func (s *dynamicServiceInfo) snapshot() map[string]struct{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]struct{}, len(s.services))
	for name := range s.services {
		out[name] = struct{}{}
	}
	return out
}

func (s *server) registerServiceInfo(name string) error {
	if _, reserved := s.reserved[name]; reserved {
		return fmt.Errorf("grpc service name %q is reserved by a built-in service", name)
	}
	if s.serviceInfo == nil {
		return nil
	}
	if err := s.serviceInfo.register(name); err != nil {
		return err
	}
	if s.health != nil {
		status := healthgrpc.HealthCheckResponse_NOT_SERVING
		if s.serving.Load() {
			status = healthgrpc.HealthCheckResponse_SERVING
		}
		s.health.SetServingStatus(name, status)
	}
	return nil
}

// RegisterServiceInfo implements transport.ServiceInfoRegistrar.
func (s *server) RegisterServiceInfo(serviceName string) error {
	return s.registerServiceInfo(serviceName)
}

func (s *server) setHealthServing() {
	if s.health == nil {
		return
	}
	s.health.SetServingStatus("", healthgrpc.HealthCheckResponse_SERVING)
	for name := range s.serviceInfo.snapshot() {
		s.health.SetServingStatus(name, healthgrpc.HealthCheckResponse_SERVING)
	}
	for name := range s.reserved {
		s.health.SetServingStatus(name, healthgrpc.HealthCheckResponse_SERVING)
	}
}

func (s *server) registerStandardServices() {
	s.reserved = map[string]struct{}{}
	s.serviceInfo = newDynamicServiceInfo(s.grpcServer)
	if s.opts.healthEnabled() {
		s.health = health.NewServer()
		s.health.SetServingStatus("", healthgrpc.HealthCheckResponse_NOT_SERVING)
		healthgrpc.RegisterHealthServer(s.grpcServer, s.health)
		s.reserved[healthgrpc.Health_ServiceDesc.ServiceName] = struct{}{}
		s.health.SetServingStatus(
			healthgrpc.Health_ServiceDesc.ServiceName,
			healthgrpc.HealthCheckResponse_NOT_SERVING,
		)
	}
	if !s.opts.ReflectionEnabled {
		return
	}
	opts := reflection.ServerOptions{Services: s.serviceInfo}
	v1reflectiongrpc.RegisterServerReflectionServer(
		s.grpcServer,
		reflection.NewServerV1(opts),
	)
	v1alphareflectiongrpc.RegisterServerReflectionServer(
		s.grpcServer,
		reflection.NewServer(opts),
	)
	s.reserved[v1reflectiongrpc.ServerReflection_ServiceDesc.ServiceName] = struct{}{}
	s.reserved[v1alphareflectiongrpc.ServerReflection_ServiceDesc.ServiceName] = struct{}{}
	if s.health != nil {
		s.health.SetServingStatus(
			v1reflectiongrpc.ServerReflection_ServiceDesc.ServiceName,
			healthgrpc.HealthCheckResponse_NOT_SERVING,
		)
		s.health.SetServingStatus(
			v1alphareflectiongrpc.ServerReflection_ServiceDesc.ServiceName,
			healthgrpc.HealthCheckResponse_NOT_SERVING,
		)
	}
}
