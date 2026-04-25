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

package rpchttp

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	"github.com/codesjoy/yggdrasil/v3/rpc/stream"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
	"github.com/codesjoy/yggdrasil/v3/transport/support/marshaler"
)

func TestClientConnNewStreamRejectsStreamingDescriptors(t *testing.T) {
	cc := &clientConn{
		state:        remote.Ready,
		endpoint:     resolver.BaseEndpoint{Address: "127.0.0.1:8080", Protocol: Protocol},
		hc:           &http.Client{},
		statsHandler: stats.NoOpHandler,
	}

	_, err := cc.NewStream(
		context.Background(),
		&stream.Desc{ClientStreams: true},
		"/pkg.Service/Method",
	)
	require.ErrorContains(t, err, "does not support streaming")
}

func TestSelectMarshalersUseContentNegotiation(t *testing.T) {
	inbound := selectInboundMarshaler(nil, marshaler.ContentTypeJSON)
	require.IsType(t, &marshaler.JSONPb{}, inbound)

	outbound := selectOutboundMarshaler(nil, marshaler.ContentTypeJSON, &marshaler.ProtoMarshaler{})
	require.IsType(t, &marshaler.JSONPb{}, outbound)

	fallback := selectOutboundMarshaler(nil, "", &marshaler.ProtoMarshaler{})
	require.IsType(t, &marshaler.ProtoMarshaler{}, fallback)
}

func TestClientConn_Protocol(t *testing.T) {
	cc := &clientConn{
		ctx:          context.Background(),
		state:        remote.Ready,
		endpoint:     resolver.BaseEndpoint{Address: "127.0.0.1:8080", Protocol: Protocol},
		hc:           &http.Client{},
		statsHandler: stats.NoOpHandler,
	}
	require.Equal(t, "http", cc.Protocol())
}

func TestClientConn_State(t *testing.T) {
	cc := &clientConn{
		ctx:          context.Background(),
		state:        remote.Ready,
		endpoint:     resolver.BaseEndpoint{Address: "127.0.0.1:8080", Protocol: Protocol},
		hc:           &http.Client{},
		statsHandler: stats.NoOpHandler,
	}
	require.Equal(t, remote.Ready, cc.State())
}

func TestClientConn_Close(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cc := &clientConn{
		ctx:          ctx,
		cancel:       cancel,
		state:        remote.Ready,
		endpoint:     resolver.BaseEndpoint{Address: "127.0.0.1:8080", Protocol: Protocol},
		hc:           &http.Client{},
		statsHandler: stats.NoOpHandler,
	}

	// First Close should transition to Shutdown.
	err := cc.Close()
	require.NoError(t, err)
	require.Equal(t, remote.Shutdown, cc.State())

	// Second Close should be a no-op and return nil.
	err = cc.Close()
	require.NoError(t, err)
	require.Equal(t, remote.Shutdown, cc.State())
}

func TestClientConn_Connect(t *testing.T) {
	t.Run("transitions to Ready and fires callback", func(t *testing.T) {
		var captured remote.ClientState
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		cc := &clientConn{
			ctx:          ctx,
			cancel:       cancel,
			state:        remote.TransientFailure,
			endpoint:     resolver.BaseEndpoint{Address: "127.0.0.1:8080", Protocol: Protocol},
			hc:           &http.Client{},
			statsHandler: stats.NoOpHandler,
			onStateChange: func(state remote.ClientState) {
				captured = state
			},
		}

		cc.Connect()
		require.Equal(t, remote.Ready, cc.State())
		require.Equal(t, remote.Ready, captured.State)
		require.Equal(t, "127.0.0.1:8080", captured.Endpoint.GetAddress())
	})

	t.Run("no-op when Shutdown", func(t *testing.T) {
		var callbackFired bool
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		cc := &clientConn{
			ctx:          ctx,
			cancel:       cancel,
			state:        remote.Shutdown,
			endpoint:     resolver.BaseEndpoint{Address: "127.0.0.1:8080", Protocol: Protocol},
			hc:           &http.Client{},
			statsHandler: stats.NoOpHandler,
			onStateChange: func(state remote.ClientState) {
				callbackFired = true
			},
		}

		cc.Connect()
		require.Equal(t, remote.Shutdown, cc.State())
		require.False(t, callbackFired)
	})
}

func TestClientConn_NewStream_EmptyMethod(t *testing.T) {
	cc := &clientConn{
		ctx:          context.Background(),
		state:        remote.Ready,
		endpoint:     resolver.BaseEndpoint{Address: "127.0.0.1:8080", Protocol: Protocol},
		hc:           &http.Client{},
		codec:        marshalerSet{},
		statsHandler: stats.NoOpHandler,
	}
	_, err := cc.NewStream(context.Background(), nil, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty method")
}

func TestClientConn_NewStream_ServerStreamingRejected(t *testing.T) {
	cc := &clientConn{
		ctx:          context.Background(),
		state:        remote.Ready,
		endpoint:     resolver.BaseEndpoint{Address: "127.0.0.1:8080", Protocol: Protocol},
		hc:           &http.Client{},
		statsHandler: stats.NoOpHandler,
	}
	_, err := cc.NewStream(
		context.Background(),
		&stream.Desc{ServerStreams: true},
		"/pkg.Service/Method",
	)
	require.ErrorContains(t, err, "does not support streaming")
}

func TestClientConn_NewStream_Success(t *testing.T) {
	cc := &clientConn{
		ctx:          context.Background(),
		state:        remote.Ready,
		endpoint:     resolver.BaseEndpoint{Address: "127.0.0.1:8080", Protocol: Protocol},
		hc:           &http.Client{},
		codec:        marshalerSet{},
		statsHandler: stats.NoOpHandler,
	}
	s, err := cc.NewStream(context.Background(), nil, "/pkg.Service/Method")
	require.NoError(t, err)
	require.NotNil(t, s)
}

func TestClientConn_NewStream_MethodNormalization(t *testing.T) {
	cc := &clientConn{
		ctx:          context.Background(),
		state:        remote.Ready,
		endpoint:     resolver.BaseEndpoint{Address: "127.0.0.1:8080", Protocol: Protocol},
		hc:           &http.Client{},
		codec:        marshalerSet{},
		statsHandler: stats.NoOpHandler,
	}
	// Method without leading slash should be normalized.
	s, err := cc.NewStream(context.Background(), nil, "pkg.Service/Method")
	require.NoError(t, err)
	require.NotNil(t, s)
	// Verify the method is accessible via the stream context (internal check).
	cs := s.(*httpClientStream)
	require.Equal(t, "/pkg.Service/Method", cs.method)
}

func TestClientProvider(t *testing.T) {
	provider := ClientProvider()
	require.NotNil(t, provider)
	require.Equal(t, "http", provider.Protocol())
}

func TestClientProviderWithSettings(t *testing.T) {
	ep := resolver.BaseEndpoint{Address: "127.0.0.1:8080", Protocol: Protocol}

	t.Run("creates client with default settings", func(t *testing.T) {
		provider := ClientProviderWithSettings(Settings{}, nil, nil)
		cc, err := provider.NewClient(context.Background(), "test.Service", ep, nil, nil)
		require.NoError(t, err)
		require.NotNil(t, cc)
		require.Equal(t, "http", cc.Protocol())
		require.Equal(t, remote.Ready, cc.State())
	})

	t.Run("creates client with custom timeout", func(t *testing.T) {
		provider := ClientProviderWithSettings(Settings{
			Client: ClientConfig{
				Timeout: 5e9, // 5 seconds in nanoseconds
			},
		}, nil, nil)
		cc, err := provider.NewClient(
			context.Background(),
			"test.Service",
			ep,
			stats.NoOpHandler,
			nil,
		)
		require.NoError(t, err)
		require.NotNil(t, cc)
	})

	t.Run("creates client with service-specific config", func(t *testing.T) {
		provider := ClientProviderWithSettings(Settings{
			ClientServices: map[string]ClientConfig{
				"test.Service": {
					Timeout: 3e9, // 3 seconds
				},
			},
		}, nil, nil)
		cc, err := provider.NewClient(
			context.Background(),
			"test.Service",
			ep,
			stats.NoOpHandler,
			nil,
		)
		require.NoError(t, err)
		require.NotNil(t, cc)
	})

	t.Run("fires onStateChange callback", func(t *testing.T) {
		var captured remote.ClientState
		provider := ClientProviderWithSettings(Settings{}, nil, nil)
		cc, err := provider.NewClient(
			context.Background(),
			"test.Service",
			ep,
			stats.NoOpHandler,
			func(state remote.ClientState) {
				captured = state
			},
		)
		require.NoError(t, err)
		require.NotNil(t, cc)
		// The initial state change callback should have fired during construction.
		require.Equal(t, remote.Ready, captured.State)
	})
}
