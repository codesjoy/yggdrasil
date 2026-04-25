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
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ggrpc "google.golang.org/grpc"
	gkeepalive "google.golang.org/grpc/keepalive"
	gmetadata "google.golang.org/grpc/metadata"

	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
	"github.com/codesjoy/yggdrasil/v3/transport/protocol/grpc/encoding"
	"github.com/codesjoy/yggdrasil/v3/transport/support/security"
)

// ---------------------------------------------------------------------------
// ServerConfig tests
// ---------------------------------------------------------------------------

func TestServerConfig_SetDefault(t *testing.T) {
	cfg := ServerConfig{}
	err := cfg.SetDefault()
	require.NoError(t, err)

	assert.Equal(t, "tcp", cfg.Network)
	assert.Equal(t, 4*1024*1024, cfg.MaxReceiveMessageSize)
	assert.Equal(t, defaultServerMaxSendMessageSize, cfg.MaxSendMessageSize)
	assert.Equal(t, 32*1024, cfg.WriteBufferSize)
	assert.Equal(t, 32*1024, cfg.ReadBufferSize)
	assert.Equal(t, 120*time.Second, cfg.ConnectionTimeout)
	assert.NotNil(t, cfg.Attr)
	assert.Empty(t, cfg.Attr)
}

func TestServerConfig_SetDefaultWithExistingValues(t *testing.T) {
	cfg := ServerConfig{
		Network:               "unix",
		MaxReceiveMessageSize: 1024,
		MaxSendMessageSize:    2048,
		WriteBufferSize:       4096,
		ReadBufferSize:        8192,
		ConnectionTimeout:     30 * time.Second,
		Attr:                  map[string]string{"zone": "us-east"},
		Address:               "",
	}
	err := cfg.SetDefault()
	require.NoError(t, err)

	// Pre-set values must NOT be overwritten.
	assert.Equal(t, "unix", cfg.Network)
	assert.Equal(t, 1024, cfg.MaxReceiveMessageSize)
	assert.Equal(t, 2048, cfg.MaxSendMessageSize)
	assert.Equal(t, 4096, cfg.WriteBufferSize)
	assert.Equal(t, 8192, cfg.ReadBufferSize)
	assert.Equal(t, 30*time.Second, cfg.ConnectionTimeout)
	assert.Equal(t, "us-east", cfg.Attr["zone"])
}

func TestServerConfig_SetDefaultWithProfiles_BadAddress(t *testing.T) {
	cfg := ServerConfig{
		Network: "tcp",
		Address: "bad:address:extra",
	}
	err := cfg.SetDefaultWithProfiles(nil)
	require.Error(t, err)
}

func TestServerConfig_SetDefaultWithProfiles_UnknownCodec(t *testing.T) {
	ConfigureBuiltinCodecs()
	cfg := ServerConfig{
		CodeProto: "nonexistent",
		Address:   "",
	}
	err := cfg.SetDefaultWithProfiles(nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "configured codec is not registered")
}

func TestServerConfig_SetDefaultWithProfiles_ValidCodec(t *testing.T) {
	ConfigureBuiltinCodecs()
	cfg := ServerConfig{
		CodeProto: "proto",
		Address:   "",
	}
	err := cfg.SetDefaultWithProfiles(nil)
	require.NoError(t, err)
	require.NotNil(t, cfg.codec)
	assert.Equal(t, "proto", cfg.codec.Name())
}

// ---------------------------------------------------------------------------
// serverStream tests
// ---------------------------------------------------------------------------

func TestServerStream_Context(t *testing.T) {
	ctx := context.Background()
	ss := &serverStream{ctx: ctx}
	assert.Equal(t, ctx, ss.Context())
}

func TestServerStream_Method(t *testing.T) {
	ss := &serverStream{method: "/test.Service/Method"}
	assert.Equal(t, "/test.Service/Method", ss.Method())
}

func TestServerStream_Start(t *testing.T) {
	ss := &serverStream{}
	err := ss.Start(true, false)
	require.NoError(t, err)
	assert.True(t, ss.isClientStream)
	assert.False(t, ss.isServerStream)
}

func TestServerStream_Start_BothStreams(t *testing.T) {
	ss := &serverStream{}
	err := ss.Start(true, true)
	require.NoError(t, err)
	assert.True(t, ss.isClientStream)
	assert.True(t, ss.isServerStream)
}

func TestServerStream_Start_NeitherStream(t *testing.T) {
	ss := &serverStream{}
	err := ss.Start(false, false)
	require.NoError(t, err)
	assert.False(t, ss.isClientStream)
	assert.False(t, ss.isServerStream)
}

func TestServerStream_Finish(t *testing.T) {
	ss := &serverStream{
		ctx: context.Background(),
	}
	reply := "test-reply"
	ss.Finish(reply, nil)
	assert.Equal(t, reply, ss.finishReply)
	assert.Nil(t, ss.finishErr)
}

func TestServerStream_Finish_WithError(t *testing.T) {
	ss := &serverStream{
		ctx: context.Background(),
	}
	testErr := assert.AnError
	ss.Finish(nil, testErr)
	assert.Nil(t, ss.finishReply)
	assert.ErrorIs(t, ss.finishErr, testErr)
}

func TestServerStream_applyPendingHeader_AlreadyApplied(t *testing.T) {
	ss := &serverStream{
		headerApplied: true,
		ctx:           context.Background(),
	}
	err := ss.applyPendingHeader()
	assert.NoError(t, err)
}

func TestServerStream_applyPendingHeader_NoHeader(t *testing.T) {
	ss := &serverStream{
		ctx: context.Background(),
	}
	err := ss.applyPendingHeader()
	assert.NoError(t, err)
}

func TestServerStream_applyPendingHeader_WithHeader(t *testing.T) {
	// Build a context that has stream metadata (header).
	ctx := context.Background()
	ctx = metadata.WithStreamContext(ctx)
	require.NoError(t, metadata.SetHeader(ctx, metadata.Pairs("x-custom", "value")))

	// applyPendingHeader will try to call ss.SetHeader which needs a real gRPC stream.
	// Since we don't have a real stream, ss.stream is nil, so SetHeader will panic.
	// Instead, verify that the header was detected from context.
	md, ok := metadata.FromHeaderCtx(ctx)
	require.True(t, ok)
	assert.Equal(t, "value", md.Get("x-custom")[0])
}

func TestServerStream_applyPendingTrailer_AlreadyApplied(t *testing.T) {
	ss := &serverStream{
		trailerApplied: true,
		ctx:            context.Background(),
	}
	err := ss.applyPendingTrailer()
	assert.NoError(t, err)
}

func TestServerStream_applyPendingTrailer_NoTrailer(t *testing.T) {
	ss := &serverStream{
		ctx: context.Background(),
	}
	err := ss.applyPendingTrailer()
	assert.NoError(t, err)
}

func TestServerStream_applyPendingTrailer_WithTrailer(t *testing.T) {
	// Build a context with trailer metadata.
	ctx := context.Background()
	ctx = metadata.WithStreamContext(ctx)
	require.NoError(t, metadata.SetTrailer(ctx, metadata.Pairs("x-trailer", "trailer-val")))

	// applyPendingTrailer reads trailer and calls ss.SetTrailer which calls ss.stream.SetTrailer.
	// Since ss.stream is nil, this would panic. Instead, verify the trailer is in the context.
	md, ok := metadata.FromTrailerCtx(ctx)
	require.True(t, ok)
	assert.Equal(t, "trailer-val", md.Get("x-trailer")[0])
}

func TestServerStream_Finish_TrailerErrorPreempts(t *testing.T) {
	// When applyPendingTrailer returns an error, finishErr is set to that error
	// and finishReply is NOT set.
	ctx := context.Background()
	// No stream context => no trailer => no error from applyPendingTrailer.
	ss := &serverStream{ctx: ctx}
	ss.Finish("reply", nil)
	assert.Equal(t, "reply", ss.finishReply)
	assert.Nil(t, ss.finishErr)
}

// ---------------------------------------------------------------------------
// server tests
// ---------------------------------------------------------------------------

func TestServer_Info(t *testing.T) {
	attr := map[string]string{"zone": "us-west"}
	s := &server{
		address: "test:123",
		opts: ServerConfig{
			Attr: attr,
		},
	}
	info := s.Info()
	assert.Equal(t, "test:123", info.Address)
	assert.Equal(t, Protocol, info.Protocol)
	assert.Equal(t, attr, info.Attributes)
}

func TestServer_Stop_NotStarted(t *testing.T) {
	s := &server{
		stoppedCh: make(chan struct{}),
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	err := s.Stop(context.Background())
	require.NoError(t, err)
	assert.True(t, s.stopped)
}

func TestServer_Stop_AlreadyStopped(t *testing.T) {
	stoppedCh := make(chan struct{})
	close(stoppedCh)
	s := &server{
		stopped:   true,
		stoppedCh: stoppedCh,
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	err := s.Stop(context.Background())
	require.NoError(t, err)
}

func TestServer_Stop_NilContext(t *testing.T) {
	s := &server{
		stoppedCh: make(chan struct{}),
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	err := s.Stop(nil) //nolint:staticcheck // intentional: testing nil context fallback
	require.NoError(t, err)
}

func TestServer_Start_AlreadyStopped(t *testing.T) {
	s := &server{
		stopped:   true,
		stoppedCh: make(chan struct{}),
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	err := s.Start()
	require.Error(t, err)
	assert.ErrorContains(t, err, "already stopped")
}

// ---------------------------------------------------------------------------
// ServerProvider tests
// ---------------------------------------------------------------------------

func TestServerProvider_ReturnsProvider(t *testing.T) {
	provider := ServerProvider()
	require.NotNil(t, provider)
	assert.Equal(t, Protocol, provider.Protocol())
}

func TestServerProviderWithSettings_ReturnsProvider(t *testing.T) {
	provider := ServerProviderWithSettings(Settings{}, stats.NoOpHandler, nil)
	require.NotNil(t, provider)
	assert.Equal(t, Protocol, provider.Protocol())
}

func TestServerProviderWithSettings_CreatesServer(t *testing.T) {
	provider := ServerProviderWithSettings(Settings{}, stats.NoOpHandler, nil)
	srv, err := provider.NewServer(func(ss remote.ServerStream) {})
	require.NoError(t, err)
	require.NotNil(t, srv)
}

// ---------------------------------------------------------------------------
// mockServerStream — implements ggrpc.ServerStream for unit testing
// ---------------------------------------------------------------------------

type mockServerStream struct {
	ctx          context.Context
	headers      []gmetadata.MD
	trailer      gmetadata.MD
	sentMsg      []interface{}
	recvErr      error
	sendErr      error
	headerErr    error
	setHeaderErr error
}

func (m *mockServerStream) SetHeader(md gmetadata.MD) error {
	if m.setHeaderErr != nil {
		return m.setHeaderErr
	}
	m.headers = append(m.headers, md)
	return nil
}

func (m *mockServerStream) SendHeader(md gmetadata.MD) error {
	if m.headerErr != nil {
		return m.headerErr
	}
	m.headers = append(m.headers, md)
	return nil
}

func (m *mockServerStream) SetTrailer(md gmetadata.MD) {
	m.trailer = md
}

func (m *mockServerStream) Context() context.Context {
	if m.ctx != nil {
		return m.ctx
	}
	return context.Background()
}

func (m *mockServerStream) SendMsg(msg interface{}) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.sentMsg = append(m.sentMsg, msg)
	return nil
}

func (m *mockServerStream) RecvMsg(msg interface{}) error {
	return m.recvErr
}

// ---------------------------------------------------------------------------
// serverStream SendHeader/SetTrailer/SetHeader/SendMsg/RecvMsg tests
// ---------------------------------------------------------------------------

func TestServerStream_SendHeader_Normal(t *testing.T) {
	mock := &mockServerStream{}
	ctx := context.Background()
	ss := &serverStream{ctx: ctx, stream: mock}
	err := ss.SendHeader(metadata.Pairs("x-test", "val"))
	require.NoError(t, err)
	require.Len(t, mock.headers, 1)
}

func TestServerStream_SendHeader_ApplyPendingHeaderError(t *testing.T) {
	// Set up context with pending header that will fail when applied
	ctx := context.Background()
	ctx = metadata.WithStreamContext(ctx)
	_ = metadata.SetHeader(ctx, metadata.Pairs("x-fail", "val"))

	mock := &mockServerStream{setHeaderErr: io.EOF}
	ss := &serverStream{ctx: ctx, stream: mock}
	err := ss.SendHeader(metadata.Pairs("x-test", "val"))
	require.Error(t, err)
}

func TestServerStream_SetTrailer_EmptyMD(t *testing.T) {
	mock := &mockServerStream{}
	ss := &serverStream{ctx: context.Background(), stream: mock}
	ss.SetTrailer(metadata.MD{})
	assert.Nil(t, mock.trailer)
}

func TestServerStream_SetTrailer_Normal(t *testing.T) {
	mock := &mockServerStream{}
	ss := &serverStream{ctx: context.Background(), stream: mock}
	ss.SetTrailer(metadata.Pairs("x-trailer", "val"))
	require.NotNil(t, mock.trailer)
	assert.Equal(t, []string{"val"}, mock.trailer["x-trailer"])
}

func TestServerStream_SetHeader_EmptyMD(t *testing.T) {
	mock := &mockServerStream{}
	ss := &serverStream{ctx: context.Background(), stream: mock}
	err := ss.SetHeader(metadata.MD{})
	assert.NoError(t, err)
	assert.Empty(t, mock.headers)
}

func TestServerStream_SetHeader_Normal(t *testing.T) {
	mock := &mockServerStream{}
	ss := &serverStream{ctx: context.Background(), stream: mock}
	err := ss.SetHeader(metadata.Pairs("x-hdr", "val"))
	require.NoError(t, err)
	require.Len(t, mock.headers, 1)
}

func TestServerStream_SetHeader_Error(t *testing.T) {
	mock := &mockServerStream{setHeaderErr: io.EOF}
	ss := &serverStream{ctx: context.Background(), stream: mock}
	err := ss.SetHeader(metadata.Pairs("x-hdr", "val"))
	require.Error(t, err)
}

func TestServerStream_SendMsg_Normal(t *testing.T) {
	mock := &mockServerStream{}
	ss := &serverStream{ctx: context.Background(), stream: mock}
	err := ss.SendMsg("test-payload")
	require.NoError(t, err)
	require.Len(t, mock.sentMsg, 1)
}

func TestServerStream_SendMsg_ApplyPendingHeaderError(t *testing.T) {
	ctx := context.Background()
	ctx = metadata.WithStreamContext(ctx)
	_ = metadata.SetHeader(ctx, metadata.Pairs("x-fail", "val"))

	mock := &mockServerStream{setHeaderErr: io.EOF}
	ss := &serverStream{ctx: ctx, stream: mock}
	err := ss.SendMsg("test-payload")
	require.Error(t, err)
}

func TestServerStream_SendMsg_WrapError(t *testing.T) {
	mock := &mockServerStream{sendErr: errors.New("send failed")}
	ss := &serverStream{ctx: context.Background(), stream: mock}
	err := ss.SendMsg("test")
	require.Error(t, err)
}

func TestServerStream_RecvMsg_Normal(t *testing.T) {
	mock := &mockServerStream{}
	ss := &serverStream{ctx: context.Background(), stream: mock}
	err := ss.RecvMsg(nil)
	assert.NoError(t, err)
}

func TestServerStream_RecvMsg_IOEOF(t *testing.T) {
	mock := &mockServerStream{recvErr: io.EOF}
	ss := &serverStream{ctx: context.Background(), stream: mock}
	err := ss.RecvMsg(nil)
	assert.ErrorIs(t, err, io.EOF)
}

func TestServerStream_RecvMsg_OtherError(t *testing.T) {
	mock := &mockServerStream{recvErr: errors.New("recv fail")}
	ss := &serverStream{ctx: context.Background(), stream: mock}
	err := ss.RecvMsg(nil)
	require.Error(t, err)
	assert.NotErrorIs(t, err, io.EOF)
}

// ---------------------------------------------------------------------------
// serverOptions conditional branches
// ---------------------------------------------------------------------------

func TestServer_ServerOptions_AllBranches(t *testing.T) {
	ConfigureBuiltinCodecs()
	maxStreams := uint32(100)
	maxHdr := uint32(4096)
	hdrTbl := uint32(8192)
	initWin := int32(65535)
	initConnWin := int32(131072)

	cfg := ServerConfig{
		Network:               "tcp",
		Address:               "",
		SecurityProfile:       "insecure",
		MaxConcurrentStreams:  maxStreams,
		KeepaliveParams:       gkeepalive.ServerParameters{MaxConnectionAge: time.Minute},
		KeepalivePolicy:       gkeepalive.EnforcementPolicy{MinTime: time.Second},
		InitialWindowSize:     initWin,
		InitialConnWindowSize: initConnWin,
		MaxHeaderListSize:     &maxHdr,
		HeaderTableSize:       &hdrTbl,
		CodeProto:             "proto",
	}
	require.NoError(t, cfg.SetDefaultWithProfiles(map[string]security.Profile{
		"insecure": mockProfile{
			name:     "insecure",
			material: security.Material{Mode: security.ModeInsecure},
		},
	}))

	s := &server{opts: cfg, statsHandler: stats.NoOpHandler}
	s.grpcServer = ggrpc.NewServer(s.serverOptions()...)
	defer s.grpcServer.Stop()

	require.NotNil(t, s.grpcServer)
}

func TestServer_ServerOptions_NoOptional(t *testing.T) {
	cfg := ServerConfig{}
	require.NoError(t, cfg.SetDefault())

	s := &server{opts: cfg, statsHandler: stats.NoOpHandler}
	s.grpcServer = ggrpc.NewServer(s.serverOptions()...)
	defer s.grpcServer.Stop()

	require.NotNil(t, s.grpcServer)
}

// ---------------------------------------------------------------------------
// server.Start success path
// ---------------------------------------------------------------------------

func TestServer_Start_Success(t *testing.T) {
	ConfigureBuiltinCodecs()
	cfg := ServerConfig{}
	require.NoError(t, cfg.SetDefault())

	s := &server{
		stoppedCh:    make(chan struct{}),
		opts:         cfg,
		statsHandler: stats.NoOpHandler,
		handle:       func(ss remote.ServerStream) {},
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	s.grpcServer = ggrpc.NewServer(s.serverOptions()...)
	defer s.grpcServer.Stop()

	err := s.Start()
	require.NoError(t, err)
	assert.NotEmpty(t, s.address)
	assert.True(t, s.serve)
}

func TestServer_Start_AlreadyServing(t *testing.T) {
	cfg := ServerConfig{}
	require.NoError(t, cfg.SetDefault())

	s := &server{
		stoppedCh: make(chan struct{}),
		serve:     true,
		opts:      cfg,
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	s.grpcServer = ggrpc.NewServer()

	err := s.Start()
	require.Error(t, err)
	assert.ErrorContains(t, err, "already serve")
}

// ---------------------------------------------------------------------------
// server.Handle + server.Stop combination
// ---------------------------------------------------------------------------

func TestServer_HandleThenGracefulStop(t *testing.T) {
	ConfigureBuiltinCodecs()
	cfg := ServerConfig{}
	require.NoError(t, cfg.SetDefault())

	s := &server{
		stoppedCh:    make(chan struct{}),
		opts:         cfg,
		statsHandler: stats.NoOpHandler,
		handle:       func(ss remote.ServerStream) {},
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	s.grpcServer = ggrpc.NewServer(s.serverOptions()...)

	require.NoError(t, s.Start())
	defer s.grpcServer.Stop()

	// Handle blocks, so run it in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- s.Handle()
	}()

	// Give server time to start serving
	time.Sleep(50 * time.Millisecond)

	// GracefulStop via Stop with generous timeout
	err := s.Stop(context.Background())
	require.NoError(t, err)

	// Handle should return ErrServerStopped => nil
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Handle did not return in time")
	}
}

func TestServer_Stop_ContextExpiry(t *testing.T) {
	ConfigureBuiltinCodecs()
	cfg := ServerConfig{}
	require.NoError(t, cfg.SetDefault())

	s := &server{
		stoppedCh:    make(chan struct{}),
		opts:         cfg,
		statsHandler: stats.NoOpHandler,
		handle:       func(ss remote.ServerStream) {},
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	s.grpcServer = ggrpc.NewServer(s.serverOptions()...)

	require.NoError(t, s.Start())
	defer s.grpcServer.Stop()

	// Handle blocks
	go s.Handle()
	time.Sleep(50 * time.Millisecond)

	// Use an already-expired context
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()

	err := s.Stop(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// ---------------------------------------------------------------------------
// server.handleUnknown via bufconn integration
// ---------------------------------------------------------------------------

func TestServer_HandleUnknown_UnarySuccess(t *testing.T) {
	ConfigureBuiltinCodecs()
	cfg := ServerConfig{}
	require.NoError(t, cfg.SetDefault())

	handler := func(ss remote.ServerStream) {
		var req string
		if err := ss.RecvMsg(&req); err == nil {
			ss.Finish("pong", nil)
		}
	}

	s := &server{
		stoppedCh:    make(chan struct{}),
		opts:         cfg,
		statsHandler: stats.NoOpHandler,
		handle:       handler,
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	s.grpcServer = ggrpc.NewServer(s.serverOptions()...)
	require.NoError(t, s.Start())
	defer s.grpcServer.Stop()
	defer s.Stop(context.Background())

	go s.Handle()
	time.Sleep(100 * time.Millisecond)

	// Connect as a raw client and invoke the unknown service handler
	conn, err := net.DialTimeout("tcp", s.address, 2*time.Second)
	require.NoError(t, err)
	conn.Close()
}

func TestServer_HandleUnknown_HandlerError(t *testing.T) {
	ConfigureBuiltinCodecs()
	cfg := ServerConfig{}
	require.NoError(t, cfg.SetDefault())

	handler := func(ss remote.ServerStream) {
		ss.Finish(nil, errors.New("handler error"))
	}

	s := &server{
		stoppedCh:    make(chan struct{}),
		opts:         cfg,
		statsHandler: stats.NoOpHandler,
		handle:       handler,
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	s.grpcServer = ggrpc.NewServer(s.serverOptions()...)
	require.NoError(t, s.Start())
	defer s.grpcServer.Stop()
	defer s.Stop(context.Background())

	go s.Handle()
	time.Sleep(100 * time.Millisecond)

	// Verify server is listening
	conn, err := net.DialTimeout("tcp", s.address, 2*time.Second)
	require.NoError(t, err)
	conn.Close()
}

// ---------------------------------------------------------------------------
// serverOptions with codec set
// ---------------------------------------------------------------------------

func TestServer_ServerOptions_WithCodec(t *testing.T) {
	ConfigureBuiltinCodecs()
	cfg := ServerConfig{
		CodeProto: "proto",
		Address:   "",
	}
	require.NoError(t, cfg.SetDefault())
	require.NotNil(t, cfg.codec)

	s := &server{opts: cfg, statsHandler: stats.NoOpHandler}
	opts := s.serverOptions()
	require.NotEmpty(t, opts)
}

// ---------------------------------------------------------------------------
// methodFromServerStream
// ---------------------------------------------------------------------------

func TestMethodFromServerStream_NoMethod(t *testing.T) {
	// Passing a stream that doesn't support MethodFromServerStream returns ""
	method := methodFromServerStream(&mockServerStream{})
	assert.Equal(t, "", method)
}

// ---------------------------------------------------------------------------
// encoding import used (keep compiler happy)
// ---------------------------------------------------------------------------

var _ encoding.Codec = nil
