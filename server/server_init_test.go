package server

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/interceptor"
	"github.com/codesjoy/yggdrasil/v3/remote"
	restserver "github.com/codesjoy/yggdrasil/v3/server/rest"
	"github.com/codesjoy/yggdrasil/v3/stream"
)

func TestNewAndInitRemoteServer(t *testing.T) {
	t.Run("new server without transports", func(t *testing.T) {
		srv, err := New(newTestRuntime())
		require.NoError(t, err)
		require.NotNil(t, srv)
	})

	t.Run("new server with rest enabled", func(t *testing.T) {
		runtime := newTestRuntime()
		runtime.settings.RestEnabled = true
		runtime.restConfig = &restserver.Config{}

		srv, err := New(runtime)
		require.NoError(t, err)
		require.NotNil(t, srv)

		inner := srv.(*server)
		require.True(t, inner.restEnable)
		require.NotNil(t, inner.restSvr)
	})

	t.Run("init remote server unknown protocol", func(t *testing.T) {
		runtime := newTestRuntime()
		runtime.settings.Transports = []string{"missing-protocol"}

		s := newTestServer()
		s.runtime = runtime
		err := s.initRemoteServer()
		require.ErrorContains(t, err, "server transport provider for protocol missing-protocol not found")
	})

	t.Run("init remote server builder error", func(t *testing.T) {
		runtime := newTestRuntime()
		runtime.serverProviders["test-builder-error"] = remote.NewTransportServerProvider(
			"test-builder-error",
			func(remote.MethodHandle) (remote.Server, error) {
				return nil, errors.New("build failed")
			},
		)
		runtime.settings.Transports = []string{"test-builder-error"}

		s := newTestServer()
		s.runtime = runtime
		err := s.initRemoteServer()
		require.ErrorContains(t, err, "fault to new test-builder-error remote server")
	})

	t.Run("init remote server builder success", func(t *testing.T) {
		runtime := newTestRuntime()
		runtime.serverProviders["test-builder-success"] = remote.NewTransportServerProvider(
			"test-builder-success",
			func(remote.MethodHandle) (remote.Server, error) {
				return &testRemoteServer{
					info: remote.ServerInfo{
						Protocol: "test-builder-success",
						Address:  "127.0.0.1:9000",
					},
				}, nil
			},
		)
		runtime.settings.Transports = []string{"test-builder-success"}

		s := newTestServer()
		s.runtime = runtime
		require.NoError(t, s.initRemoteServer())
		require.Len(t, s.servers, 1)
		require.Equal(t, "test-builder-success", s.servers[0].Info().Protocol)
	})
}

func TestInitInterceptorDedupsConfiguredNames(t *testing.T) {
	var unaryBuildCalls int32
	var streamBuildCalls int32

	runtime := newTestRuntime()
	runtime.settings.Interceptors = InterceptorSettings{
		Unary:  []string{"server-dedup-unary", "server-dedup-unary"},
		Stream: []string{"server-dedup-stream", "server-dedup-stream"},
	}
	runtime.unaryProviders["server-dedup-unary"] = interceptor.NewUnaryServerInterceptorProvider(
		"server-dedup-unary",
		func() interceptor.UnaryServerInterceptor {
			atomic.AddInt32(&unaryBuildCalls, 1)
			return func(ctx context.Context, req interface{}, info *interceptor.UnaryServerInfo, handler interceptor.UnaryHandler) (interface{}, error) {
				return handler(ctx, req)
			}
		},
	)
	runtime.streamProviders["server-dedup-stream"] = interceptor.NewStreamServerInterceptorProvider(
		"server-dedup-stream",
		func() interceptor.StreamServerInterceptor {
			atomic.AddInt32(&streamBuildCalls, 1)
			return func(srv interface{}, ss stream.ServerStream, info *interceptor.StreamServerInfo, handler stream.Handler) error {
				return handler(srv, ss)
			}
		},
	)

	s := newTestServer()
	s.runtime = runtime
	s.initInterceptor()

	require.NotNil(t, s.unaryInterceptor)
	require.NotNil(t, s.streamInterceptor)
	require.Equal(t, int32(1), atomic.LoadInt32(&unaryBuildCalls))
	require.Equal(t, int32(1), atomic.LoadInt32(&streamBuildCalls))
}

func TestNewTwiceDoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		first, err := New(newTestRuntime())
		assert.NoError(t, err)
		assert.NotNil(t, first)

		second, err := New(newTestRuntime())
		assert.NoError(t, err)
		assert.NotNil(t, second)
	})
}
