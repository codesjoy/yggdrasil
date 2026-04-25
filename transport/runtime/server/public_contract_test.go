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
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/rpc/interceptor"
)

type mockEndpoint struct {
	protocol string
	address  string
	metadata map[string]string
	kind     EndpointKind
}

func (m *mockEndpoint) Protocol() string {
	return m.protocol
}

func (m *mockEndpoint) Address() string {
	return m.address
}

func (m *mockEndpoint) Metadata() map[string]string {
	return m.metadata
}

func (m *mockEndpoint) Kind() EndpointKind {
	return m.kind
}

type mockServer struct {
	services          []*ServiceDesc
	restServices      []*RestServiceDesc
	rawHandlers       []*RestRawHandlerDesc
	endpoints         []Endpoint
	serveCalled       bool
	stopCalled        bool
	startFlagProvided bool
}

func (m *mockServer) RegisterService(sd *ServiceDesc, _ interface{}) {
	m.services = append(m.services, sd)
}

func (m *mockServer) RegisterRestService(sd *RestServiceDesc, _ interface{}, _ ...string) {
	m.restServices = append(m.restServices, sd)
}

func (m *mockServer) RegisterRestRawHandlers(sd ...*RestRawHandlerDesc) {
	m.rawHandlers = append(m.rawHandlers, sd...)
}

func (m *mockServer) Serve(startFlag chan<- struct{}) error {
	m.serveCalled = true
	if startFlag != nil {
		startFlag <- struct{}{}
		m.startFlagProvided = true
	}
	return nil
}

func (m *mockServer) Stop(context.Context) error {
	m.stopCalled = true
	return nil
}

func (m *mockServer) Endpoints() []Endpoint {
	return m.endpoints
}

func TestEndpointAndServerInterfaces(t *testing.T) {
	var _ Endpoint = (*mockEndpoint)(nil)
	var _ Endpoint = (*serverInfo)(nil)
	var _ Server = (*mockServer)(nil)

	endpoint := &mockEndpoint{
		protocol: "grpc",
		address:  "localhost:8080",
		metadata: map[string]string{"version": "1.0"},
		kind:     EndpointKindRPC,
	}
	require.Equal(t, "grpc", endpoint.Protocol())
	require.Equal(t, "localhost:8080", endpoint.Address())
	require.Equal(t, "1.0", endpoint.Metadata()["version"])
	require.Equal(t, EndpointKindRPC, endpoint.Kind())

	server := &mockServer{}
	server.RegisterService(
		&ServiceDesc{ServiceName: "test.service", HandlerType: (*TestService)(nil)},
		&TestServiceImpl{},
	)
	server.RegisterRestService(
		&RestServiceDesc{HandlerType: (*TestService)(nil)},
		&TestServiceImpl{},
	)
	server.RegisterRestRawHandlers(&RestRawHandlerDesc{Method: http.MethodGet, Path: "/health"})
	server.endpoints = []Endpoint{endpoint}

	startFlag := make(chan struct{}, 1)
	require.NoError(t, server.Serve(startFlag))
	<-startFlag
	require.NoError(t, server.Stop(context.Background()))

	require.True(t, server.serveCalled)
	require.True(t, server.stopCalled)
	require.True(t, server.startFlagProvided)
	require.Len(t, server.services, 1)
	require.Len(t, server.restServices, 1)
	require.Len(t, server.rawHandlers, 1)
	require.Equal(t, endpoint, server.Endpoints()[0])
}

func TestHandlerTypesInvokeProvidedCallbacks(t *testing.T) {
	var methodCalled bool
	method := MethodDesc{
		MethodName: "TestMethod",
		Handler: func(srv interface{}, ctx context.Context, dec func(interface{}) error, unary interceptor.UnaryServerInterceptor) (interface{}, error) {
			methodCalled = true
			return "handled", nil
		},
	}

	response, err := method.Handler(nil, context.Background(), nil, nil)
	require.NoError(t, err)
	require.True(t, methodCalled)
	require.Equal(t, "handled", response)

	var restCalled bool
	restMethod := RestMethodDesc{
		Method: http.MethodGet,
		Path:   "/test",
		Handler: func(w http.ResponseWriter, r *http.Request, srv interface{}, unary interceptor.UnaryServerInterceptor) (interface{}, error) {
			restCalled = true
			return "rest handled", nil
		},
	}

	response, err = restMethod.Handler(nil, nil, nil, nil)
	require.NoError(t, err)
	require.True(t, restCalled)
	require.Equal(t, "rest handled", response)
}

func TestPublicDescriptorZeroValues(t *testing.T) {
	serviceDesc := &ServiceDesc{
		ServiceName: "empty.service",
		HandlerType: (*TestService)(nil),
		Methods:     []MethodDesc{},
		Streams:     nil,
		Metadata:    nil,
	}
	assert.Empty(t, serviceDesc.Methods)
	assert.Nil(t, serviceDesc.Streams)
	assert.Nil(t, serviceDesc.Metadata)

	restDesc := &RestServiceDesc{
		HandlerType: (*TestService)(nil),
		Methods:     nil,
	}
	assert.Nil(t, restDesc.Methods)

	info := &serverInfo{
		protocol: "grpc",
		address:  "localhost:8080",
		svrKind:  EndpointKindRPC,
		metadata: nil,
	}
	assert.Nil(t, info.Metadata())
}
