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

package lifecycle

import (
	"context"
	"sync"

	"github.com/stretchr/testify/mock"

	"github.com/codesjoy/yggdrasil/v3/discovery/registry"
	yserver "github.com/codesjoy/yggdrasil/v3/transport/runtime/server"
)

type mockRegistry struct {
	mock.Mock
	registered   bool
	deregistered bool
}

func (m *mockRegistry) Register(ctx context.Context, instance registry.Instance) error {
	args := m.Called(ctx, instance)
	m.registered = true
	return args.Error(0)
}

func (m *mockRegistry) Deregister(ctx context.Context, instance registry.Instance) error {
	args := m.Called(ctx, instance)
	m.deregistered = true
	return args.Error(0)
}

func (m *mockRegistry) Type() string {
	args := m.Called()
	return args.String(0)
}

type mockInternalServer struct {
	mock.Mock
	started bool
	stopped bool
}

func (m *mockInternalServer) Serve() error {
	args := m.Called()
	m.started = true
	return args.Error(0)
}

func (m *mockInternalServer) Stop(ctx context.Context) error {
	args := m.Called(ctx)
	m.stopped = true
	return args.Error(0)
}

type blockingInternalServer struct {
	stopCtx context.Context
}

func (b *blockingInternalServer) Serve() error {
	return nil
}

func (b *blockingInternalServer) Stop(ctx context.Context) error {
	b.stopCtx = ctx
	<-ctx.Done()
	return ctx.Err()
}

type blockingAppServer struct {
	stopCtx context.Context
	endpts  []yserver.Endpoint
}

func (b *blockingAppServer) RegisterService(*yserver.ServiceDesc, interface{})                    {}
func (b *blockingAppServer) RegisterRestService(*yserver.RestServiceDesc, interface{}, ...string) {}
func (b *blockingAppServer) RegisterRestRawHandlers(...*yserver.RestRawHandlerDesc)               {}

func (b *blockingAppServer) Serve(chan<- struct{}) error {
	return nil
}

func (b *blockingAppServer) Stop(ctx context.Context) error {
	b.stopCtx = ctx
	<-ctx.Done()
	return ctx.Err()
}

func (b *blockingAppServer) Endpoints() []yserver.Endpoint {
	return b.endpts
}

type runningAppServer struct {
	stopCtx  context.Context
	stopCh   chan struct{}
	stopOnce sync.Once
}

func (r *runningAppServer) RegisterService(*yserver.ServiceDesc, interface{})                    {}
func (r *runningAppServer) RegisterRestService(*yserver.RestServiceDesc, interface{}, ...string) {}
func (r *runningAppServer) RegisterRestRawHandlers(...*yserver.RestRawHandlerDesc)               {}

func (r *runningAppServer) Serve(startFlag chan<- struct{}) error {
	if startFlag != nil {
		startFlag <- struct{}{}
	}
	if r.stopCh == nil {
		return nil
	}
	<-r.stopCh
	return nil
}

func (r *runningAppServer) Stop(ctx context.Context) error {
	r.stopCtx = ctx
	r.stopOnce.Do(func() {
		if r.stopCh == nil {
			r.stopCh = make(chan struct{})
		}
		close(r.stopCh)
	})
	return nil
}

func (r *runningAppServer) Endpoints() []yserver.Endpoint {
	return nil
}

type failingInternalServer struct {
	serveErr error
	stopCtx  context.Context
}

func (f *failingInternalServer) Serve() error {
	return f.serveErr
}

func (f *failingInternalServer) Stop(ctx context.Context) error {
	f.stopCtx = ctx
	return nil
}

type stubEndpoint struct {
	protocol string
	address  string
	metadata map[string]string
	kind     yserver.EndpointKind
}

func (e stubEndpoint) Protocol() string {
	return e.protocol
}

func (e stubEndpoint) Address() string {
	return e.address
}

func (e stubEndpoint) Metadata() map[string]string {
	return e.metadata
}

func (e stubEndpoint) Kind() yserver.EndpointKind {
	return e.kind
}

func createMockRegistry() *mockRegistry {
	return &mockRegistry{}
}

func createMockInternalServer() *mockInternalServer {
	return &mockInternalServer{}
}
