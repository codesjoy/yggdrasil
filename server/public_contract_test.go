package server

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/rpc/interceptor"
	"github.com/codesjoy/yggdrasil/v3/internal/constant"
)

type mockEndpoint struct {
	scheme   string
	address  string
	metadata map[string]string
	kind     constant.ServerKind
}

func (m *mockEndpoint) Scheme() string {
	return m.scheme
}

func (m *mockEndpoint) Address() string {
	return m.address
}

func (m *mockEndpoint) Metadata() map[string]string {
	return m.metadata
}

func (m *mockEndpoint) Kind() constant.ServerKind {
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
		scheme:   "grpc",
		address:  "localhost:8080",
		metadata: map[string]string{"version": "1.0"},
		kind:     constant.ServerKindRPC,
	}
	require.Equal(t, "grpc", endpoint.Scheme())
	require.Equal(t, "localhost:8080", endpoint.Address())
	require.Equal(t, "1.0", endpoint.Metadata()["version"])
	require.Equal(t, constant.ServerKindRPC, endpoint.Kind())

	server := &mockServer{}
	server.RegisterService(&ServiceDesc{ServiceName: "test.service", HandlerType: (*TestService)(nil)}, &TestServiceImpl{})
	server.RegisterRestService(&RestServiceDesc{HandlerType: (*TestService)(nil)}, &TestServiceImpl{})
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
		scheme:   "grpc",
		address:  "localhost:8080",
		svrKind:  constant.ServerKindRPC,
		metadata: nil,
	}
	assert.Nil(t, info.Metadata())
}
