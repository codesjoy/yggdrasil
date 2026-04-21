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
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v2/interceptor"
	"github.com/codesjoy/yggdrasil/v2/metadata"
	"github.com/codesjoy/yggdrasil/v2/stream"
)

func TestServerProcessUnaryRPC(t *testing.T) {
	t.Run("start error finishes with error", func(t *testing.T) {
		ss := &testServerStream{startErr: errors.New("start failed")}
		s := &server{}
		s.processUnaryRPC(&MethodDesc{
			MethodName: "Unary",
			Handler: func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor interceptor.UnaryServerInterceptor) (interface{}, error) {
				t.Fatal("handler should not be called")
				return nil, nil
			},
		}, &ServiceInfo{ServiceImpl: &TestServiceImpl{}}, ss)

		require.ErrorContains(t, ss.finishErr, "start failed")
		require.Nil(t, ss.finishReply)
	})

	t.Run("success propagates metadata and interceptor", func(t *testing.T) {
		var interceptorCalled bool
		s := &server{
			unaryInterceptor: func(ctx context.Context, req interface{}, info *interceptor.UnaryServerInfo, handler interceptor.UnaryHandler) (interface{}, error) {
				interceptorCalled = true
				require.Equal(t, "/svc/Unary", info.FullMethod)
				return handler(ctx, req)
			},
		}
		ss := &testServerStream{method: "/svc/Unary"}
		desc := &MethodDesc{
			MethodName: "Unary",
			Handler: func(srv interface{}, ctx context.Context, dec func(interface{}) error, unary interceptor.UnaryServerInterceptor) (interface{}, error) {
				if unary != nil {
					return unary(ctx, "req", &interceptor.UnaryServerInfo{
						Server:     srv,
						FullMethod: "/svc/Unary",
					}, func(ctx context.Context, req any) (any, error) {
						require.NoError(t, metadata.SetHeader(ctx, metadata.Pairs("h", "v")))
						require.NoError(t, metadata.SetTrailer(ctx, metadata.Pairs("t", "tv")))
						return "reply", nil
					})
				}
				return nil, nil
			},
		}

		s.processUnaryRPC(desc, &ServiceInfo{ServiceImpl: &TestServiceImpl{}}, ss)

		require.True(t, interceptorCalled)
		require.False(t, ss.startClientStream)
		require.False(t, ss.startServerStream)
		require.Equal(t, "reply", ss.finishReply)
		require.NoError(t, ss.finishErr)
		require.Equal(t, "v", ss.header.Get("h")[0])
		require.Equal(t, "tv", ss.trailer.Get("t")[0])
	})
}

func TestServerProcessStreamRPC(t *testing.T) {
	t.Run("start error finishes with error", func(t *testing.T) {
		ss := &testServerStream{startErr: errors.New("start failed")}
		s := &server{
			streamInterceptor: func(srv interface{}, ss stream.ServerStream, info *interceptor.StreamServerInfo, handler stream.Handler) error {
				t.Fatal("interceptor should not be called")
				return nil
			},
		}
		s.processStreamRPC(&stream.Desc{
			StreamName:    "Stream",
			ClientStreams: true,
			ServerStreams: true,
			Handler: func(srv interface{}, ss stream.ServerStream) error {
				return nil
			},
		}, &ServiceInfo{ServiceImpl: &TestServiceImpl{}}, ss)
		require.ErrorContains(t, ss.finishErr, "start failed")
	})

	t.Run("success calls interceptor and handler", func(t *testing.T) {
		var interceptorCalled bool
		var handlerCalled bool
		ss := &testServerStream{method: "/svc/Stream"}
		s := &server{
			streamInterceptor: func(srv interface{}, ss stream.ServerStream, info *interceptor.StreamServerInfo, handler stream.Handler) error {
				interceptorCalled = true
				require.Equal(t, "/svc/Stream", info.FullMethod)
				require.True(t, info.IsClientStream)
				require.True(t, info.IsServerStream)
				return handler(srv, ss)
			},
		}
		s.processStreamRPC(&stream.Desc{
			StreamName:    "Stream",
			ClientStreams: true,
			ServerStreams: true,
			Handler: func(srv interface{}, ss stream.ServerStream) error {
				handlerCalled = true
				return nil
			},
		}, &ServiceInfo{ServiceImpl: &TestServiceImpl{}}, ss)

		require.True(t, interceptorCalled)
		require.True(t, handlerCalled)
		require.True(t, ss.startClientStream)
		require.True(t, ss.startServerStream)
		require.NoError(t, ss.finishErr)
		require.Nil(t, ss.finishReply)
	})
}

func TestServerHandleStreamDispatch(t *testing.T) {
	t.Run("dispatches unary and stream handlers", func(t *testing.T) {
		var unaryCalled bool
		var streamCalled bool
		s := &server{
			services: map[string]*ServiceInfo{
				"svc": {
					ServiceImpl: &TestServiceImpl{},
					Methods: map[string]*MethodDesc{
						"Unary": {
							MethodName: "Unary",
							Handler: func(srv interface{}, ctx context.Context, dec func(interface{}) error, unary interceptor.UnaryServerInterceptor) (interface{}, error) {
								unaryCalled = true
								return "ok", nil
							},
						},
					},
					Streams: map[string]*stream.Desc{
						"Stream": {
							StreamName:    "Stream",
							ClientStreams: true,
							ServerStreams: true,
							Handler: func(srv interface{}, ss stream.ServerStream) error {
								streamCalled = true
								return nil
							},
						},
					},
				},
			},
			streamInterceptor: func(srv interface{}, ss stream.ServerStream, info *interceptor.StreamServerInfo, handler stream.Handler) error {
				return handler(srv, ss)
			},
		}

		s.handleStream(&testServerStream{method: "/svc/Unary"})
		s.handleStream(&testServerStream{method: "/svc/Stream"})

		require.True(t, unaryCalled)
		require.True(t, streamCalled)
	})

	t.Run("malformed method and missing service/method", func(t *testing.T) {
		s := &server{
			services: map[string]*ServiceInfo{
				"svc": {Methods: map[string]*MethodDesc{}, Streams: map[string]*stream.Desc{}},
			},
		}

		cases := []*testServerStream{
			{method: "malformed"},
			{method: "/missing/Unary"},
			{method: "/svc/Missing"},
		}
		for _, ss := range cases {
			s.handleStream(ss)
			require.Error(t, ss.finishErr)
			require.Nil(t, ss.finishReply)
		}
	})
}

type mockHandleStream struct {
	method string
	ctx    context.Context
}

func (m *mockHandleStream) Method() string {
	return m.method
}

func (m *mockHandleStream) Start(bool, bool) error {
	return nil
}

func (m *mockHandleStream) Finish(any, error) {}

func (m *mockHandleStream) SetHeader(metadata.MD) error {
	return nil
}

func (m *mockHandleStream) SendHeader(metadata.MD) error {
	return nil
}

func (m *mockHandleStream) SetTrailer(metadata.MD) {}

func (m *mockHandleStream) Context() context.Context {
	if m.ctx != nil {
		return m.ctx
	}
	return context.Background()
}

func (m *mockHandleStream) SendMsg(any) error {
	return nil
}

func (m *mockHandleStream) RecvMsg(any) error {
	return nil
}

func TestHandleStreamConcurrentWithLateRegister(t *testing.T) {
	s := &server{
		services: map[string]*ServiceInfo{
			"existing.service": {
				ServiceImpl: nil,
				Methods:     map[string]*MethodDesc{},
				Streams:     map[string]*stream.Desc{},
			},
		},
		servicesDesc: map[string][]methodInfo{},
		state:        serverStateRunning,
	}
	desc := &ServiceDesc{
		ServiceName: "late.service",
		HandlerType: (*TestService)(nil),
	}
	impl := &TestServiceImpl{}

	const workers = 200
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.handleStream(&mockHandleStream{method: "/existing.service/missing"})
		}()
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.RegisterService(desc, impl)
		}()
	}
	wg.Wait()

	s.mu.RLock()
	_, exists := s.services["late.service"]
	registerErr := s.registerErr
	s.mu.RUnlock()
	assert.False(t, exists)
	assert.Error(t, registerErr)
}
