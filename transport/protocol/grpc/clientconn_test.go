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
	"errors"
	"fmt"
	"io"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ggrpc "google.golang.org/grpc"
	gkeepalive "google.golang.org/grpc/keepalive"
	gmetadata "google.golang.org/grpc/metadata"

	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
	"github.com/codesjoy/yggdrasil/v3/rpc/stream"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
)

// ---------------------------------------------------------------------------
// ClientConfig.setDefault tests
// ---------------------------------------------------------------------------

func TestClientConfig_SetDefault(t *testing.T) {
	cfg := &ClientConfig{}
	cfg.setDefault("test-service")

	assert.Equal(t, math.MaxInt32, cfg.MaxSendMsgSize)
	assert.Equal(t, 4*1024*1024, cfg.MaxRecvMsgSize)
	assert.Equal(t, 32*1024, cfg.Transport.WriteBufferSize)
	assert.Equal(t, 32*1024, cfg.Transport.ReadBufferSize)
	assert.Equal(t, "test-service", cfg.Transport.Authority)
}

func TestClientConfig_SetDefault_DoesNotOverwrite(t *testing.T) {
	cfg := &ClientConfig{
		MaxSendMsgSize: 1024,
		MaxRecvMsgSize: 2048,
		Transport: ClientTransportOptions{
			WriteBufferSize: 4096,
			ReadBufferSize:  8192,
			Authority:       "custom-authority",
		},
	}
	cfg.setDefault("test-service")

	assert.Equal(t, 1024, cfg.MaxSendMsgSize)
	assert.Equal(t, 2048, cfg.MaxRecvMsgSize)
	assert.Equal(t, 4096, cfg.Transport.WriteBufferSize)
	assert.Equal(t, 8192, cfg.Transport.ReadBufferSize)
	assert.Equal(t, "custom-authority", cfg.Transport.Authority)
}

// ---------------------------------------------------------------------------
// clientConn tests
// ---------------------------------------------------------------------------

func TestClientConn_Protocol(t *testing.T) {
	cc := &clientConn{}
	assert.Equal(t, "grpc", cc.Protocol())
}

func TestClientConn_State(t *testing.T) {
	cc := &clientConn{
		state: remote.Ready,
	}
	assert.Equal(t, remote.Ready, cc.State())
}

func TestClientConn_State_InitialIdle(t *testing.T) {
	cc := &clientConn{
		state: remote.Idle,
	}
	assert.Equal(t, remote.Idle, cc.State())
}

// ---------------------------------------------------------------------------
// changeStateUnlock tests
// ---------------------------------------------------------------------------

func TestClientConn_changeStateUnlock_CallbackFires(t *testing.T) {
	var capturedState remote.ClientState
	cc := &clientConn{
		state: remote.Ready,
		onStateChange: func(s remote.ClientState) {
			capturedState = s
		},
	}

	cc.mu.Lock()
	cc.changeStateUnlock(remote.TransientFailure, nil)
	cc.mu.Unlock()

	assert.Equal(t, remote.TransientFailure, cc.state)
	assert.Equal(t, remote.TransientFailure, capturedState.State)
	assert.Nil(t, capturedState.ConnectionError)
}

func TestClientConn_changeStateUnlock_CallbackFiresWithConnErr(t *testing.T) {
	var capturedState remote.ClientState
	cc := &clientConn{
		state: remote.Ready,
		onStateChange: func(s remote.ClientState) {
			capturedState = s
		},
	}

	connErr := assert.AnError
	cc.mu.Lock()
	cc.changeStateUnlock(remote.TransientFailure, connErr)
	cc.mu.Unlock()

	assert.Equal(t, remote.TransientFailure, cc.state)
	assert.Equal(t, remote.TransientFailure, capturedState.State)
	assert.ErrorIs(t, capturedState.ConnectionError, connErr)
}

func TestClientConn_changeStateUnlock_NoCallbackWhenSame(t *testing.T) {
	called := false
	cc := &clientConn{
		state: remote.Ready,
		onStateChange: func(s remote.ClientState) {
			called = true
		},
	}

	cc.mu.Lock()
	cc.changeStateUnlock(remote.Ready, nil)
	cc.mu.Unlock()

	assert.Equal(t, remote.Ready, cc.state)
	assert.False(t, called, "callback should not fire for same state")
}

func TestClientConn_changeStateUnlock_NoCallbackOnShutdown(t *testing.T) {
	called := false
	cc := &clientConn{
		state: remote.Ready,
		onStateChange: func(s remote.ClientState) {
			called = true
		},
	}

	cc.mu.Lock()
	cc.changeStateUnlock(remote.Shutdown, nil)
	cc.mu.Unlock()

	assert.Equal(t, remote.Shutdown, cc.state)
	assert.False(t, called, "callback should not fire for Shutdown")
}

func TestClientConn_changeStateUnlock_NoCallbackWhenNil(t *testing.T) {
	cc := &clientConn{
		state:         remote.Ready,
		onStateChange: nil,
	}

	cc.mu.Lock()
	cc.changeStateUnlock(remote.TransientFailure, nil)
	cc.mu.Unlock()

	assert.Equal(t, remote.TransientFailure, cc.state)
	// No panic means success.
}

func TestClientConn_changeStateUnlock_ConcurrentReads(t *testing.T) {
	var capturedStates []remote.State
	var mu sync.Mutex

	cc := &clientConn{
		state: remote.Idle,
		onStateChange: func(s remote.ClientState) {
			mu.Lock()
			capturedStates = append(capturedStates, s.State)
			mu.Unlock()
		},
	}

	// Simulate several state transitions.
	transitions := []remote.State{remote.Connecting, remote.Ready, remote.TransientFailure}
	for _, s := range transitions {
		cc.mu.Lock()
		cc.changeStateUnlock(s, nil)
		cc.mu.Unlock()
	}

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, capturedStates, 3)
	assert.Equal(t, remote.Connecting, capturedStates[0])
	assert.Equal(t, remote.Ready, capturedStates[1])
	assert.Equal(t, remote.TransientFailure, capturedStates[2])
}

// ---------------------------------------------------------------------------
// ClientProvider tests
// ---------------------------------------------------------------------------

func TestClientProvider_ReturnsProvider(t *testing.T) {
	provider := ClientProvider()
	require.NotNil(t, provider)
	assert.Equal(t, Protocol, provider.Protocol())
}

func TestClientProviderWithSettings_ReturnsProvider(t *testing.T) {
	provider := ClientProviderWithSettings(Settings{}, nil)
	require.NotNil(t, provider)
	assert.Equal(t, Protocol, provider.Protocol())
}

// ---------------------------------------------------------------------------
// buildClientDialOptions tests
// ---------------------------------------------------------------------------

func TestBuildClientDialOptions_Basic(t *testing.T) {
	cfg := &ClientConfig{}
	cfg.setDefault("test-svc")

	opts, err := buildClientDialOptions(cfg, "test-svc", nil)
	require.NoError(t, err)
	require.NotEmpty(t, opts)
}

func TestBuildClientDialOptions_WithAuthority(t *testing.T) {
	cfg := &ClientConfig{
		Transport: ClientTransportOptions{
			Authority: "my-authority",
		},
	}
	cfg.setDefault("test-svc")

	opts, err := buildClientDialOptions(cfg, "test-svc", nil)
	require.NoError(t, err)
	require.NotEmpty(t, opts)
}

func TestBuildClientDialOptions_WithUserAgent(t *testing.T) {
	cfg := &ClientConfig{
		Transport: ClientTransportOptions{
			UserAgent: "test-agent/1.0",
		},
	}
	cfg.setDefault("test-svc")

	opts, err := buildClientDialOptions(cfg, "test-svc", nil)
	require.NoError(t, err)
	require.NotEmpty(t, opts)
}

func TestBuildClientDialOptions_UnknownSecurityProfile(t *testing.T) {
	cfg := &ClientConfig{
		Transport: ClientTransportOptions{
			SecurityProfile: "unknown-profile",
		},
	}
	cfg.setDefault("test-svc")

	_, err := buildClientDialOptions(cfg, "test-svc", nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "not found")
}

// ---------------------------------------------------------------------------
// buildClientDialOptionsWithBuilders optional branches
// ---------------------------------------------------------------------------

func TestBuildClientDialOptions_WithKeepaliveParams(t *testing.T) {
	cfg := &ClientConfig{
		Transport: ClientTransportOptions{
			KeepaliveParams: gkeepalive.ClientParameters{
				Time:    30 * time.Second,
				Timeout: 10 * time.Second,
			},
		},
	}
	cfg.setDefault("test-svc")

	opts, err := buildClientDialOptions(cfg, "test-svc", nil)
	require.NoError(t, err)
	require.NotEmpty(t, opts)
}

func TestBuildClientDialOptions_WithInitialWindowSize(t *testing.T) {
	cfg := &ClientConfig{
		Transport: ClientTransportOptions{
			InitialWindowSize: 65535,
		},
	}
	cfg.setDefault("test-svc")

	opts, err := buildClientDialOptions(cfg, "test-svc", nil)
	require.NoError(t, err)
	require.NotEmpty(t, opts)
}

func TestBuildClientDialOptions_WithInitialConnWindowSize(t *testing.T) {
	cfg := &ClientConfig{
		Transport: ClientTransportOptions{
			InitialConnWindowSize: 131072,
		},
	}
	cfg.setDefault("test-svc")

	opts, err := buildClientDialOptions(cfg, "test-svc", nil)
	require.NoError(t, err)
	require.NotEmpty(t, opts)
}

func TestBuildClientDialOptions_WithMaxHeaderListSize(t *testing.T) {
	maxHdr := uint32(8192)
	cfg := &ClientConfig{
		Transport: ClientTransportOptions{
			MaxHeaderListSize: &maxHdr,
		},
	}
	cfg.setDefault("test-svc")

	opts, err := buildClientDialOptions(cfg, "test-svc", nil)
	require.NoError(t, err)
	require.NotEmpty(t, opts)
}

func TestBuildClientDialOptions_WithConnectTimeout(t *testing.T) {
	cfg := &ClientConfig{
		ConnectTimeout: 5 * time.Second,
	}
	cfg.setDefault("test-svc")

	opts, err := buildClientDialOptions(cfg, "test-svc", nil)
	require.NoError(t, err)
	require.NotEmpty(t, opts)
}

func TestBuildClientDialOptions_WithCompressor(t *testing.T) {
	cfg := &ClientConfig{
		Compressor: "gzip",
	}
	cfg.setDefault("test-svc")

	opts, err := buildClientDialOptions(cfg, "test-svc", nil)
	require.NoError(t, err)
	require.NotEmpty(t, opts)
}

// ---------------------------------------------------------------------------
// testEndpoint — resolver.Endpoint mock
// ---------------------------------------------------------------------------

type testEndpoint struct {
	name    string
	address string
	proto   string
}

func (e testEndpoint) Name() string                  { return e.name }
func (e testEndpoint) GetAddress() string            { return e.address }
func (e testEndpoint) GetProtocol() string           { return e.proto }
func (e testEndpoint) GetAttributes() map[string]any { return nil }

// ---------------------------------------------------------------------------
// clientConn.Close tests
// ---------------------------------------------------------------------------

func TestClientConn_Close_Normal(t *testing.T) {
	ConfigureBuiltinCodecs()
	cfg := &ClientConfig{}
	cfg.setDefault("test-svc")

	conn, err := ggrpc.NewClient(
		grpcTargetForEndpoint("127.0.0.1:0"),
		buildInsecureCreds(),
	)
	require.NoError(t, err)

	cc := &clientConn{
		cfg:   cfg,
		conn:  conn,
		state: remote.Idle,
	}
	cc.ctx, cc.cancel = context.WithCancel(context.Background())

	err = cc.Close()
	require.NoError(t, err)
	assert.Equal(t, remote.Shutdown, cc.State())
}

func TestClientConn_Close_AlreadyClosed(t *testing.T) {
	ConfigureBuiltinCodecs()
	cfg := &ClientConfig{}
	cfg.setDefault("test-svc")

	conn, err := ggrpc.NewClient(
		grpcTargetForEndpoint("127.0.0.1:0"),
		buildInsecureCreds(),
	)
	require.NoError(t, err)

	cc := &clientConn{
		cfg:   cfg,
		conn:  conn,
		state: remote.Shutdown,
	}
	cc.ctx, cc.cancel = context.WithCancel(context.Background())

	err = cc.Close()
	require.Error(t, err)
	assert.ErrorContains(t, err, "remote client closed")
}

func buildInsecureCreds() ggrpc.DialOption {
	return ggrpc.WithTransportCredentials(
		&transportCredentialsBridge{base: &mockTransportCredentials{}},
	)
}

// ---------------------------------------------------------------------------
// clientConn.Connect test
// ---------------------------------------------------------------------------

func TestClientConn_Connect(t *testing.T) {
	ConfigureBuiltinCodecs()
	cfg := &ClientConfig{}
	cfg.setDefault("test-svc")

	conn, err := ggrpc.NewClient(
		grpcTargetForEndpoint("127.0.0.1:0"),
		buildInsecureCreds(),
	)
	require.NoError(t, err)
	defer conn.Close()

	cc := &clientConn{
		cfg:  cfg,
		conn: conn,
	}
	cc.ctx, cc.cancel = context.WithCancel(context.Background())
	defer cc.cancel()

	// Connect is a simple delegation, should not panic
	cc.Connect()
}

// ---------------------------------------------------------------------------
// clientConn.NewStream tests
// ---------------------------------------------------------------------------

func TestClientConn_NewStream_NilDesc(t *testing.T) {
	// NewStream with nil desc should default to empty Desc
	// We can't easily test the full flow without a real server,
	// but we can test the option building path.
	ConfigureBuiltinCodecs()
	cfg := &ClientConfig{}
	cfg.setDefault("test-svc")

	// Create a real clientConn pointing at a bogus address
	conn, err := ggrpc.NewClient(
		grpcTargetForEndpoint("127.0.0.1:0"),
		buildInsecureCreds(),
	)
	require.NoError(t, err)
	defer conn.Close()

	cc := &clientConn{
		cfg:  cfg,
		conn: conn,
	}
	cc.ctx, cc.cancel = context.WithCancel(context.Background())
	defer cc.cancel()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = cc.NewStream(ctx, nil, "/test.Service/Method")
	// Expected to fail since there's no server, but the function should run without panicking
	require.Error(t, err)
}

func TestClientConn_NewStream_WithCallOptions(t *testing.T) {
	ConfigureBuiltinCodecs()
	cfg := &ClientConfig{
		Compressor: "gzip",
	}
	cfg.setDefault("test-svc")

	conn, err := ggrpc.NewClient(
		grpcTargetForEndpoint("127.0.0.1:0"),
		buildInsecureCreds(),
	)
	require.NoError(t, err)
	defer conn.Close()

	cc := &clientConn{
		cfg:  cfg,
		conn: conn,
	}
	cc.ctx, cc.cancel = context.WithCancel(context.Background())
	defer cc.cancel()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Add call options to context
	ctx = WithCallOptions(ctx, CallContentSubtype("proto"))

	_, err = cc.NewStream(
		ctx,
		&stream.Desc{ServerStreams: true, ClientStreams: true},
		"/test.Service/Method",
	)
	require.Error(t, err)
}

func TestClientConn_NewStream_WithOutgoingMetadata(t *testing.T) {
	ConfigureBuiltinCodecs()
	cfg := &ClientConfig{}
	cfg.setDefault("test-svc")

	conn, err := ggrpc.NewClient(
		grpcTargetForEndpoint("127.0.0.1:0"),
		buildInsecureCreds(),
	)
	require.NoError(t, err)
	defer conn.Close()

	cc := &clientConn{
		cfg:  cfg,
		conn: conn,
	}
	cc.ctx, cc.cancel = context.WithCancel(context.Background())
	defer cc.cancel()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Add outgoing metadata
	ctx = metadata.WithOutContext(ctx, metadata.Pairs("x-custom", "val"))

	_, err = cc.NewStream(ctx, &stream.Desc{}, "/test.Service/Method")
	require.Error(t, err)
}

func TestClientConn_NewStream_BadCallOption(t *testing.T) {
	ConfigureBuiltinCodecs()
	cfg := &ClientConfig{}
	cfg.setDefault("test-svc")

	conn, err := ggrpc.NewClient(
		grpcTargetForEndpoint("127.0.0.1:0"),
		buildInsecureCreds(),
	)
	require.NoError(t, err)
	defer conn.Close()

	cc := &clientConn{
		cfg:  cfg,
		conn: conn,
	}
	cc.ctx, cc.cancel = context.WithCancel(context.Background())
	defer cc.cancel()

	// ForceCodec(nil) should cause an error
	ctx := WithCallOptions(context.Background(), ForceCodec(nil))
	_, err = cc.NewStream(ctx, &stream.Desc{}, "/test.Service/Method")
	require.Error(t, err)
	assert.ErrorContains(t, err, "forced codec cannot be nil")
}

// ---------------------------------------------------------------------------
// clientStream tests (with mock)
// ---------------------------------------------------------------------------

type mockClientStream struct {
	ggrpc.ClientStream
	header    gmetadata.MD
	trailer   gmetadata.MD
	headerErr error
	closeErr  error
	sendErr   error
	recvErr   error
	ctx       context.Context
}

func (m *mockClientStream) Header() (gmetadata.MD, error) {
	if m.headerErr != nil {
		return nil, m.headerErr
	}
	return m.header, nil
}

func (m *mockClientStream) Trailer() gmetadata.MD {
	return m.trailer
}

func (m *mockClientStream) CloseSend() error {
	return m.closeErr
}

func (m *mockClientStream) Context() context.Context {
	if m.ctx != nil {
		return m.ctx
	}
	return context.Background()
}

func (m *mockClientStream) SendMsg(interface{}) error {
	return m.sendErr
}

func (m *mockClientStream) RecvMsg(interface{}) error {
	return m.recvErr
}

func TestClientStream_Header_Success(t *testing.T) {
	cs := &clientStream{
		ctx:          context.Background(),
		ClientStream: &mockClientStream{header: gmetadata.Pairs("x-hdr", "val")},
	}
	md, err := cs.Header()
	require.NoError(t, err)
	assert.Equal(t, []string{"val"}, md.Get("x-hdr"))
}

func TestClientStream_Header_Error(t *testing.T) {
	cs := &clientStream{
		ctx:          context.Background(),
		ClientStream: &mockClientStream{headerErr: io.EOF},
	}
	_, err := cs.Header()
	require.Error(t, err)
}

func TestClientStream_Trailer(t *testing.T) {
	cs := &clientStream{
		ctx:          context.Background(),
		ClientStream: &mockClientStream{trailer: gmetadata.Pairs("x-trailer", "val")},
	}
	md := cs.Trailer()
	assert.Equal(t, []string{"val"}, md.Get("x-trailer"))
}

func TestClientStream_CloseSend_Success(t *testing.T) {
	cs := &clientStream{
		ctx:          context.Background(),
		ClientStream: &mockClientStream{},
	}
	err := cs.CloseSend()
	assert.NoError(t, err)
}

func TestClientStream_CloseSend_Error(t *testing.T) {
	cs := &clientStream{
		ctx:          context.Background(),
		ClientStream: &mockClientStream{closeErr: errors.New("close fail")},
	}
	err := cs.CloseSend()
	require.Error(t, err)
}

func TestClientStream_Context(t *testing.T) {
	ctx := context.Background()
	cs := &clientStream{
		ctx:          ctx,
		ClientStream: &mockClientStream{},
	}
	assert.Equal(t, ctx, cs.Context())
}

func TestClientStream_SendMsg_Success(t *testing.T) {
	cs := &clientStream{
		ctx:          context.Background(),
		ClientStream: &mockClientStream{},
	}
	err := cs.SendMsg("payload")
	assert.NoError(t, err)
}

func TestClientStream_SendMsg_EOF(t *testing.T) {
	cs := &clientStream{
		ctx:          context.Background(),
		ClientStream: &mockClientStream{sendErr: io.EOF},
	}
	err := cs.SendMsg("payload")
	assert.ErrorIs(t, err, io.EOF)
}

func TestClientStream_SendMsg_OtherError(t *testing.T) {
	cs := &clientStream{
		ctx:          context.Background(),
		ClientStream: &mockClientStream{sendErr: errors.New("send fail")},
	}
	err := cs.SendMsg("payload")
	require.Error(t, err)
	assert.NotErrorIs(t, err, io.EOF)
}

func TestClientStream_RecvMsg_Success(t *testing.T) {
	cs := &clientStream{
		ctx:          context.Background(),
		ClientStream: &mockClientStream{},
	}
	err := cs.RecvMsg(nil)
	assert.NoError(t, err)
}

func TestClientStream_RecvMsg_EOF(t *testing.T) {
	cs := &clientStream{
		ctx:          context.Background(),
		ClientStream: &mockClientStream{recvErr: io.EOF},
	}
	err := cs.RecvMsg(nil)
	assert.ErrorIs(t, err, io.EOF)
}

func TestClientStream_RecvMsg_OtherError(t *testing.T) {
	cs := &clientStream{
		ctx:          context.Background(),
		ClientStream: &mockClientStream{recvErr: errors.New("recv fail")},
	}
	err := cs.RecvMsg(nil)
	require.Error(t, err)
	assert.NotErrorIs(t, err, io.EOF)
}

// ---------------------------------------------------------------------------
// watchConnectivity integration tests
// ---------------------------------------------------------------------------

func TestClientConn_WatchConnectivity_ShutdownOnClose(t *testing.T) {
	ConfigureBuiltinCodecs()
	cfg := &ClientConfig{}
	cfg.setDefault("test-svc")

	conn, err := ggrpc.NewClient(
		grpcTargetForEndpoint("127.0.0.1:0"),
		buildInsecureCreds(),
	)
	require.NoError(t, err)

	var stateChanges []remote.State
	var mu sync.Mutex
	ep := testEndpoint{name: "test", address: "127.0.0.1:0", proto: Protocol}

	cc := &clientConn{
		cfg:      cfg,
		conn:     conn,
		state:    remote.Idle,
		endpoint: ep,
		onStateChange: func(s remote.ClientState) {
			mu.Lock()
			stateChanges = append(stateChanges, s.State)
			mu.Unlock()
		},
	}
	cc.ctx, cc.cancel = context.WithCancel(context.Background())

	// Start watching
	go cc.watchConnectivity()

	// Close triggers Shutdown
	time.Sleep(50 * time.Millisecond)
	require.NoError(t, cc.Close())

	// Wait for watchConnectivity to observe the shutdown
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, remote.Shutdown, cc.state)
}

// ---------------------------------------------------------------------------
// Full clientConn creation via ClientProvider
// ---------------------------------------------------------------------------

func TestClientConn_CreateViaProvider(t *testing.T) {
	ConfigureBuiltinCodecs()
	provider := ClientProviderWithSettings(Settings{}, nil)

	ep := testEndpoint{name: "test-svc", address: "127.0.0.1:0", proto: Protocol}
	client, err := provider.NewClient(
		context.Background(),
		"test-svc",
		ep,
		stats.NoOpHandler,
		func(s remote.ClientState) {},
	)
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, Protocol, client.Protocol())

	// Close the client
	require.NoError(t, client.Close())

	// Double close should error
	err = client.Close()
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// fmt import used (keep compiler happy)
// ---------------------------------------------------------------------------

var _ = fmt.Sprintf
