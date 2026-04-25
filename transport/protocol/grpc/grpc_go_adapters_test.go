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
	stdtls "crypto/tls"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/code"
	gcodes "google.golang.org/grpc/codes"
	gconnectivity "google.golang.org/grpc/connectivity"
	gcredentials "google.golang.org/grpc/credentials"
	gmetadata "google.golang.org/grpc/metadata"
	gpeer "google.golang.org/grpc/peer"
	gstats "google.golang.org/grpc/stats"
	gstatus "google.golang.org/grpc/status"

	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	ymetadata "github.com/codesjoy/yggdrasil/v3/rpc/metadata"
	ystatus "github.com/codesjoy/yggdrasil/v3/rpc/status"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
	stats2 "github.com/codesjoy/yggdrasil/v3/transport/protocol/grpc/stats"
	"github.com/codesjoy/yggdrasil/v3/transport/support/peer"
	"github.com/codesjoy/yggdrasil/v3/transport/support/security"
)

func TestToGRPCMetadata(t *testing.T) {
	t.Run("empty returns nil", func(t *testing.T) {
		assert.Nil(t, toGRPCMetadata(nil))
	})
	t.Run("non-empty", func(t *testing.T) {
		md := ymetadata.Pairs("key1", "val1", "key2", "val2")
		got := toGRPCMetadata(md)
		require.Len(t, got, 2)
		assert.Equal(t, []string{"val1"}, got["key1"])
	})
}

func TestFromGRPCMetadata(t *testing.T) {
	t.Run("empty returns empty", func(t *testing.T) {
		got := fromGRPCMetadata(nil)
		assert.Empty(t, got)
	})
	t.Run("keys lowercased", func(t *testing.T) {
		md := gmetadata.Pairs("Key-1", "val1")
		got := fromGRPCMetadata(md)
		assert.Equal(t, []string{"val1"}, got["key-1"])
	})
}

func TestRemoteStateFromConnectivity(t *testing.T) {
	cases := []struct {
		input gconnectivity.State
		want  remote.State
	}{
		{gconnectivity.Idle, remote.Idle},
		{gconnectivity.Connecting, remote.Connecting},
		{gconnectivity.Ready, remote.Ready},
		{gconnectivity.TransientFailure, remote.TransientFailure},
		{gconnectivity.Shutdown, remote.Shutdown},
		{gconnectivity.State(99), remote.Idle},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, remoteStateFromConnectivity(tc.input))
	}
}

func TestToGRPCError(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		assert.Nil(t, toGRPCError(nil))
	})
	t.Run("context deadline exceeded", func(t *testing.T) {
		err := toGRPCError(context.DeadlineExceeded)
		require.Error(t, err)
	})
	t.Run("context canceled", func(t *testing.T) {
		err := toGRPCError(context.Canceled)
		require.Error(t, err)
	})
	t.Run("generic error", func(t *testing.T) {
		err := toGRPCError(io.EOF)
		require.Error(t, err)
	})
}

func TestGRPCTargetForEndpoint(t *testing.T) {
	assert.Equal(t, "passthrough:///127.0.0.1:8080", grpcTargetForEndpoint("127.0.0.1:8080"))
}

func TestGRPCConnectParams(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		cfg := &ClientConfig{}
		params := grpcConnectParams(cfg)
		assert.Equal(t, 120*time.Second, params.Backoff.MaxDelay)
		assert.Equal(t, minConnectTimeout, params.MinConnectTimeout)
	})
	t.Run("overrides", func(t *testing.T) {
		cfg := &ClientConfig{
			BackOffMaxDelay:   30 * time.Second,
			MinConnectTimeout: 10 * time.Second,
		}
		params := grpcConnectParams(cfg)
		assert.Equal(t, 30*time.Second, params.Backoff.MaxDelay)
		assert.Equal(t, 10*time.Second, params.MinConnectTimeout)
	})
}

func TestNormalizeListenAddress(t *testing.T) {
	t.Run("empty address uses port 0", func(t *testing.T) {
		addr, err := normalizeListenAddress("tcp", "")
		require.NoError(t, err)
		assert.Contains(t, addr, ":0")
	})
	t.Run("host:port", func(t *testing.T) {
		addr, err := normalizeListenAddress("tcp", "127.0.0.1:8080")
		require.NoError(t, err)
		assert.Equal(t, "127.0.0.1:8080", addr)
	})
	t.Run("invalid address", func(t *testing.T) {
		_, err := normalizeListenAddress("tcp", "invalid-no-port")
		require.Error(t, err)
	})
}

func TestGRPCAuthInfo(t *testing.T) {
	t.Run("AuthType", func(t *testing.T) {
		ai := grpcAuthInfo{base: testAuthInfo{authType: "test"}}
		assert.Equal(t, "test", ai.AuthType())
	})
	t.Run("GetCommonAuthInfo no CommonAuthInfo", func(t *testing.T) {
		ai := grpcAuthInfo{base: bareAuthInfoNoCommon{}}
		info := ai.GetCommonAuthInfo()
		// Default CommonAuthInfo has zero SecurityLevel
		assert.NotNil(t, info)
	})
}

func TestTransportCredentialsBridge(t *testing.T) {
	tc := &transportCredentialsBridge{base: &mockTransportCredentials{}}

	t.Run("Info", func(t *testing.T) {
		info := tc.Info()
		assert.Equal(t, "test-proto", info.SecurityProtocol)
	})
	t.Run("OverrideServerName", func(t *testing.T) {
		err := tc.OverrideServerName("new-name")
		require.NoError(t, err)
	})
	t.Run("Clone", func(t *testing.T) {
		cloned := tc.Clone()
		require.NotNil(t, cloned)
	})
}

func TestBuildTransportCredentials(t *testing.T) {
	t.Run("empty profile returns insecure", func(t *testing.T) {
		creds, err := buildTransportCredentials("", "svc", true, "")
		require.NoError(t, err)
		require.NotNil(t, creds)
	})
	t.Run("insecure profile", func(t *testing.T) {
		creds, err := buildTransportCredentialsWithProfiles(map[string]security.Profile{
			"insecure": mockProfile{
				name:     "insecure",
				material: security.Material{Mode: security.ModeInsecure},
			},
		}, "insecure", "svc", true, "")
		require.NoError(t, err)
		require.NotNil(t, creds)
	})
	t.Run("local profile", func(t *testing.T) {
		creds, err := buildTransportCredentialsWithProfiles(map[string]security.Profile{
			"local": mockProfile{
				name: "local",
				material: security.Material{
					Mode:     security.ModeLocal,
					ConnAuth: &mockTransportCredentials{},
				},
			},
		}, "local", "svc", true, "")
		require.NoError(t, err)
		require.NotNil(t, creds)
	})
	t.Run("unknown profile returns error", func(t *testing.T) {
		_, err := buildTransportCredentials("unknown", "svc", true, "")
		require.Error(t, err)
		assert.ErrorContains(t, err, "not found")
	})
	t.Run("custom profile", func(t *testing.T) {
		called := false
		profiles := map[string]security.Profile{
			"custom": mockProfile{
				name: "custom",
				buildFn: func(spec security.BuildSpec) (security.Material, error) {
					called = true
					return security.Material{
						Mode:     security.ModeLocal,
						ConnAuth: &mockTransportCredentials{},
					}, nil
				},
			},
		}
		creds, err := buildTransportCredentialsWithProfiles(profiles, "custom", "svc", true, "")
		require.NoError(t, err)
		require.NotNil(t, creds)
		assert.True(t, called)
	})
	t.Run("local profile missing conn auth", func(t *testing.T) {
		_, err := buildTransportCredentialsWithProfiles(map[string]security.Profile{
			"bad": mockProfile{name: "bad", material: security.Material{Mode: security.ModeLocal}},
		}, "bad", "svc", true, "")
		require.Error(t, err)
		assert.ErrorContains(t, err, "nil connection authenticator")
	})
}

func TestFromGRPCPeer(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		assert.Nil(t, fromGRPCPeer(nil, "grpc"))
	})
	t.Run("with TCP addr", func(t *testing.T) {
		p := &gpeer.Peer{
			Addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
		}
		got := fromGRPCPeer(p, "grpc")
		require.NotNil(t, got)
		assert.Equal(t, "127.0.0.1", got.RemoteIP)
		assert.Equal(t, "grpc", got.Protocol)
	})
}

func TestAddrString(t *testing.T) {
	assert.Equal(t, "", addrString(nil))
	assert.Equal(
		t,
		"127.0.0.1:8080",
		addrString(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}),
	)
}

func TestStatsHandlerBridge(t *testing.T) {
	t.Run("nil handler returns nil", func(t *testing.T) {
		assert.Nil(t, newStatsHandlerBridge(nil))
	})
}

func TestToRPCErr(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		assert.Nil(t, toRPCErr(nil))
	})
	t.Run("io.EOF is returned as-is", func(t *testing.T) {
		err := toRPCErr(io.EOF)
		require.ErrorIs(t, err, io.EOF)
	})
	t.Run("context deadline exceeded", func(t *testing.T) {
		err := toRPCErr(context.DeadlineExceeded)
		require.Error(t, err)
	})
	t.Run("context canceled", func(t *testing.T) {
		err := toRPCErr(context.Canceled)
		require.Error(t, err)
	})
	t.Run("io.ErrUnexpectedEOF", func(t *testing.T) {
		err := toRPCErr(io.ErrUnexpectedEOF)
		require.Error(t, err)
	})
	t.Run("gRPC status error", func(t *testing.T) {
		grpcErr := gstatus.Error(gcodes.NotFound, "item not found")
		err := toRPCErr(grpcErr)
		require.Error(t, err)
	})
	t.Run("generic error wrapped as unknown", func(t *testing.T) {
		err := toRPCErr(errors.New("something went wrong"))
		require.Error(t, err)
	})
}

func TestSetCallInfoCodec(t *testing.T) {
	t.Run("default uses proto codec", func(t *testing.T) {
		c := defaultCallInfo()
		err := setCallInfoCodec(c)
		require.NoError(t, err)
		require.NotNil(t, c.codec)
		assert.Equal(t, "proto", c.codec.Name())
	})
	t.Run("empty content subtype set", func(t *testing.T) {
		c := &callInfo{contentSubtypeSet: true, contentSubtype: ""}
		err := setCallInfoCodec(c)
		require.Error(t, err)
		assert.ErrorContains(t, err, "content-subtype cannot be empty")
	})
}

// Test helper types

type testAuthInfo struct {
	authType string
	security.CommonAuthInfo
}

func (t testAuthInfo) AuthType() string { return t.authType }

type bareAuthInfoNoCommon struct{}

func (bareAuthInfoNoCommon) AuthType() string { return "bare" }

type mockTransportCredentials struct{}

func (m *mockTransportCredentials) ClientHandshake(
	ctx context.Context,
	authority string,
	conn net.Conn,
) (net.Conn, security.AuthInfo, error) {
	return conn, nil, nil
}

func (m *mockTransportCredentials) ServerHandshake(
	conn net.Conn,
) (net.Conn, security.AuthInfo, error) {
	return conn, nil, nil
}

func (m *mockTransportCredentials) Info() security.ProtocolInfo {
	return security.ProtocolInfo{SecurityProtocol: "test-proto"}
}

func (m *mockTransportCredentials) Clone() security.ConnAuthenticator {
	return &mockTransportCredentials{}
}
func (m *mockTransportCredentials) OverrideServerName(string) error { return nil }

type mockProfile struct {
	name     string
	material security.Material
	buildFn  func(security.BuildSpec) (security.Material, error)
}

func (p mockProfile) Name() string { return p.name }

func (p mockProfile) Type() string { return p.name }

func (p mockProfile) Build(spec security.BuildSpec) (security.Material, error) {
	if p.buildFn != nil {
		return p.buildFn(spec)
	}
	return p.material, nil
}

// ---------------------------------------------------------------------------
// recordingHandler — mock for ystats.Handler that records all calls
// ---------------------------------------------------------------------------

type rpcCall struct {
	method string
	info   interface{}
}

type recordingHandler struct {
	rpcCalls  []rpcCall
	connCalls []string // "begin" / "end"
}

func (h *recordingHandler) TagRPC(ctx context.Context, info stats.RPCTagInfo) context.Context {
	h.rpcCalls = append(h.rpcCalls, rpcCall{method: "TagRPC", info: info})
	return ctx
}

func (h *recordingHandler) HandleRPC(ctx context.Context, rs stats.RPCStats) {
	h.rpcCalls = append(h.rpcCalls, rpcCall{method: "HandleRPC", info: rs})
}

func (h *recordingHandler) TagChannel(ctx context.Context, info stats.ChanTagInfo) context.Context {
	h.connCalls = append(h.connCalls, "TagChannel")
	return ctx
}

func (h *recordingHandler) HandleChannel(ctx context.Context, cs stats.ChanStats) {
	switch cs.(type) {
	case stats.ChanBegin:
		h.connCalls = append(h.connCalls, "begin")
	case stats.ChanEnd:
		h.connCalls = append(h.connCalls, "end")
	}
}

// ---------------------------------------------------------------------------
// errorTransportCredentials — mock for testing error/nil-auth paths
// ---------------------------------------------------------------------------

type errorTransportCredentials struct {
	err      error
	authInfo security.AuthInfo
}

func (e *errorTransportCredentials) ClientHandshake(
	_ context.Context,
	_ string,
	conn net.Conn,
) (net.Conn, security.AuthInfo, error) {
	if e.err != nil {
		return nil, nil, e.err
	}
	return conn, e.authInfo, nil
}

func (e *errorTransportCredentials) ServerHandshake(
	conn net.Conn,
) (net.Conn, security.AuthInfo, error) {
	if e.err != nil {
		return nil, nil, e.err
	}
	return conn, e.authInfo, nil
}

func (e *errorTransportCredentials) Info() security.ProtocolInfo {
	return security.ProtocolInfo{SecurityProtocol: "error-proto"}
}

func (e *errorTransportCredentials) Clone() security.ConnAuthenticator {
	return e
}

func (e *errorTransportCredentials) OverrideServerName(string) error { return nil }

// ---------------------------------------------------------------------------
// statsHandlerBridge tests
// ---------------------------------------------------------------------------

func TestStatsHandlerBridge_TagRPC(t *testing.T) {
	t.Run("nil info returns ctx unchanged", func(t *testing.T) {
		h := &recordingHandler{}
		b := &statsHandlerBridge{handler: h}
		ctx := context.Background()
		got := b.TagRPC(ctx, nil)
		assert.Equal(t, ctx, got)
		assert.Empty(t, h.rpcCalls)
	})
	t.Run("valid info delegates to handler", func(t *testing.T) {
		h := &recordingHandler{}
		b := &statsHandlerBridge{handler: h}
		ctx := context.Background()
		got := b.TagRPC(ctx, &gstats.RPCTagInfo{FullMethodName: "/test/Method"})
		assert.Equal(t, ctx, got)
		require.Len(t, h.rpcCalls, 1)
		assert.Equal(t, "TagRPC", h.rpcCalls[0].method)
	})
}

func TestStatsHandlerBridge_HandleRPC(t *testing.T) {
	ctx := context.Background()

	t.Run("Begin", func(t *testing.T) {
		h := &recordingHandler{}
		b := &statsHandlerBridge{handler: h}
		b.HandleRPC(ctx, &gstats.Begin{Client: true, BeginTime: time.Now()})
		require.Len(t, h.rpcCalls, 1)
		assert.Equal(t, "HandleRPC", h.rpcCalls[0].method)
	})

	t.Run("InHeader client", func(t *testing.T) {
		h := &recordingHandler{}
		b := &statsHandlerBridge{handler: h}
		b.HandleRPC(
			ctx,
			&gstats.InHeader{
				Client:      true,
				Header:      gmetadata.Pairs("k", "v"),
				WireLength:  10,
				Compression: "gzip",
			},
		)
		require.Len(t, h.rpcCalls, 1)
		call := h.rpcCalls[0]
		_, ok := call.info.(*stats2.ClientInHeader)
		assert.True(t, ok, "expected ClientInHeader")
	})

	t.Run("InHeader server", func(t *testing.T) {
		h := &recordingHandler{}
		b := &statsHandlerBridge{handler: h}
		b.HandleRPC(ctx, &gstats.InHeader{
			Client:     false,
			Header:     gmetadata.Pairs("k", "v"),
			WireLength: 20,
			FullMethod: "/svc/Method",
			RemoteAddr: &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 1234},
			LocalAddr:  &net.TCPAddr{IP: net.ParseIP("10.0.0.2"), Port: 5678},
		})
		require.Len(t, h.rpcCalls, 1)
		_, ok := h.rpcCalls[0].info.(*stats2.ServerInHeader)
		assert.True(t, ok, "expected ServerInHeader")
	})

	t.Run("OutHeader", func(t *testing.T) {
		h := &recordingHandler{}
		b := &statsHandlerBridge{handler: h}
		b.HandleRPC(ctx, &gstats.OutHeader{
			Client:     true,
			Header:     gmetadata.Pairs("k", "v"),
			FullMethod: "/svc/Method",
		})
		require.Len(t, h.rpcCalls, 1)
		_, ok := h.rpcCalls[0].info.(*stats2.OutHeader)
		assert.True(t, ok, "expected OutHeader")
	})

	t.Run("InTrailer", func(t *testing.T) {
		h := &recordingHandler{}
		b := &statsHandlerBridge{handler: h}
		b.HandleRPC(ctx, &gstats.InTrailer{Client: true, Trailer: gmetadata.Pairs("tk", "tv")})
		require.Len(t, h.rpcCalls, 1)
	})

	t.Run("OutTrailer", func(t *testing.T) {
		h := &recordingHandler{}
		b := &statsHandlerBridge{handler: h}
		b.HandleRPC(ctx, &gstats.OutTrailer{Client: false, Trailer: gmetadata.Pairs("tk", "tv")})
		require.Len(t, h.rpcCalls, 1)
	})

	t.Run("InPayload", func(t *testing.T) {
		h := &recordingHandler{}
		b := &statsHandlerBridge{handler: h}
		b.HandleRPC(ctx, &gstats.InPayload{Client: true, Payload: "data", RecvTime: time.Now()})
		require.Len(t, h.rpcCalls, 1)
		_, ok := h.rpcCalls[0].info.(*stats2.InPayload)
		assert.True(t, ok)
	})

	t.Run("OutPayload", func(t *testing.T) {
		h := &recordingHandler{}
		b := &statsHandlerBridge{handler: h}
		b.HandleRPC(ctx, &gstats.OutPayload{Client: false, Payload: "data", SentTime: time.Now()})
		require.Len(t, h.rpcCalls, 1)
		_, ok := h.rpcCalls[0].info.(*stats2.OutPayload)
		assert.True(t, ok)
	})

	t.Run("End", func(t *testing.T) {
		h := &recordingHandler{}
		b := &statsHandlerBridge{handler: h}
		b.HandleRPC(ctx, &gstats.End{Client: true, BeginTime: time.Now(), EndTime: time.Now()})
		require.Len(t, h.rpcCalls, 1)
	})
}

func TestStatsHandlerBridge_TagConn(t *testing.T) {
	t.Run("nil info returns ctx unchanged", func(t *testing.T) {
		h := &recordingHandler{}
		b := &statsHandlerBridge{handler: h}
		ctx := context.Background()
		got := b.TagConn(ctx, nil)
		assert.Equal(t, ctx, got)
		assert.Empty(t, h.connCalls)
	})
	t.Run("valid info delegates to handler", func(t *testing.T) {
		h := &recordingHandler{}
		b := &statsHandlerBridge{handler: h}
		ctx := context.Background()
		got := b.TagConn(ctx, &gstats.ConnTagInfo{
			RemoteAddr: &net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 80},
			LocalAddr:  &net.TCPAddr{IP: net.ParseIP("5.6.7.8"), Port: 90},
		})
		assert.Equal(t, ctx, got)
		require.Contains(t, h.connCalls, "TagChannel")
	})
}

func TestStatsHandlerBridge_HandleConn(t *testing.T) {
	t.Run("ConnBegin", func(t *testing.T) {
		h := &recordingHandler{}
		b := &statsHandlerBridge{handler: h}
		b.HandleConn(context.Background(), &gstats.ConnBegin{Client: true})
		require.Contains(t, h.connCalls, "begin")
	})
	t.Run("ConnEnd", func(t *testing.T) {
		h := &recordingHandler{}
		b := &statsHandlerBridge{handler: h}
		b.HandleConn(context.Background(), &gstats.ConnEnd{Client: false})
		require.Contains(t, h.connCalls, "end")
	})
}

// ---------------------------------------------------------------------------
// buildIncomingContext tests
// ---------------------------------------------------------------------------

func TestBuildIncomingContext(t *testing.T) {
	t.Run("no metadata or peer", func(t *testing.T) {
		ctx := buildIncomingContext(context.Background())
		assert.NotNil(t, ctx)
	})

	t.Run("with gRPC incoming metadata", func(t *testing.T) {
		ctx := gmetadata.NewIncomingContext(context.Background(), gmetadata.Pairs("k", "v"))
		ctx = buildIncomingContext(ctx)
		md, ok := ymetadata.FromInContext(ctx)
		assert.True(t, ok)
		assert.Equal(t, []string{"v"}, md.Get("k"))
	})

	t.Run("with gRPC peer", func(t *testing.T) {
		ctx := gpeer.NewContext(context.Background(), &gpeer.Peer{
			Addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9090},
		})
		ctx = buildIncomingContext(ctx)
		p, ok := peer.FromContext(ctx)
		require.True(t, ok)
		require.NotNil(t, p)
		assert.Equal(t, "127.0.0.1", p.RemoteIP)
	})

	t.Run("with both metadata and peer", func(t *testing.T) {
		ctx := gmetadata.NewIncomingContext(context.Background(), gmetadata.Pairs("x", "y"))
		ctx = gpeer.NewContext(ctx, &gpeer.Peer{
			Addr: &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 8080},
		})
		ctx = buildIncomingContext(ctx)
		_, ok := ymetadata.FromInContext(ctx)
		assert.True(t, ok)
		p, ok := peer.FromContext(ctx)
		require.True(t, ok)
		require.NotNil(t, p)
		assert.Equal(t, "10.0.0.1", p.RemoteIP)
	})
}

// ---------------------------------------------------------------------------
// transportCredentialsBridge ClientHandshake/ServerHandshake tests
// ---------------------------------------------------------------------------

func TestTransportCredentialsBridge_ClientHandshake(t *testing.T) {
	t.Run("base returns error", func(t *testing.T) {
		tc := &transportCredentialsBridge{base: &errorTransportCredentials{err: io.EOF}}
		_, _, err := tc.ClientHandshake(context.Background(), "test", nil)
		require.ErrorIs(t, err, io.EOF)
	})

	t.Run("base returns nil authInfo", func(t *testing.T) {
		tc := &transportCredentialsBridge{base: &errorTransportCredentials{}}
		conn, authInfo, err := tc.ClientHandshake(context.Background(), "test", nil)
		require.NoError(t, err)
		assert.Nil(t, conn)
		assert.Nil(t, authInfo)
	})

	t.Run("base returns valid authInfo", func(t *testing.T) {
		innerAuth := testAuthInfo{authType: "test"}
		tc := &transportCredentialsBridge{base: &errorTransportCredentials{authInfo: innerAuth}}
		_, authInfo, err := tc.ClientHandshake(context.Background(), "test", nil)
		require.NoError(t, err)
		require.NotNil(t, authInfo)
		gai, ok := authInfo.(grpcAuthInfo)
		require.True(t, ok)
		assert.Equal(t, "test", gai.AuthType())
	})
}

func TestTransportCredentialsBridge_ServerHandshake(t *testing.T) {
	t.Run("base returns error", func(t *testing.T) {
		tc := &transportCredentialsBridge{base: &errorTransportCredentials{err: io.EOF}}
		_, _, err := tc.ServerHandshake(nil)
		require.ErrorIs(t, err, io.EOF)
	})

	t.Run("base returns nil authInfo", func(t *testing.T) {
		tc := &transportCredentialsBridge{base: &errorTransportCredentials{}}
		_, authInfo, err := tc.ServerHandshake(nil)
		require.NoError(t, err)
		assert.Nil(t, authInfo)
	})

	t.Run("base returns valid authInfo", func(t *testing.T) {
		innerAuth := testAuthInfo{authType: "test"}
		tc := &transportCredentialsBridge{base: &errorTransportCredentials{authInfo: innerAuth}}
		_, authInfo, err := tc.ServerHandshake(nil)
		require.NoError(t, err)
		require.NotNil(t, authInfo)
		gai, ok := authInfo.(grpcAuthInfo)
		require.True(t, ok)
		assert.Equal(t, "test", gai.AuthType())
	})
}

// ---------------------------------------------------------------------------
// grpcAuthInfo.GetCommonAuthInfo security level branches
// ---------------------------------------------------------------------------

func TestGRPCAuthInfo_GetCommonAuthInfo_SecurityLevels(t *testing.T) {
	t.Run("NoSecurity", func(t *testing.T) {
		ai := grpcAuthInfo{
			base: testAuthInfo{
				CommonAuthInfo: security.CommonAuthInfo{SecurityLevel: security.NoSecurity},
			},
		}
		info := ai.GetCommonAuthInfo()
		assert.Equal(t, gcredentials.NoSecurity, info.SecurityLevel)
	})

	t.Run("IntegrityOnly", func(t *testing.T) {
		ai := grpcAuthInfo{
			base: testAuthInfo{
				CommonAuthInfo: security.CommonAuthInfo{SecurityLevel: security.IntegrityOnly},
			},
		}
		info := ai.GetCommonAuthInfo()
		assert.Equal(t, gcredentials.IntegrityOnly, info.SecurityLevel)
	})

	t.Run("PrivacyAndIntegrity", func(t *testing.T) {
		ai := grpcAuthInfo{
			base: testAuthInfo{
				CommonAuthInfo: security.CommonAuthInfo{
					SecurityLevel: security.PrivacyAndIntegrity,
				},
			},
		}
		info := ai.GetCommonAuthInfo()
		assert.Equal(t, gcredentials.PrivacyAndIntegrity, info.SecurityLevel)
	})
}

// ---------------------------------------------------------------------------
// setCallInfoCodec additional branches
// ---------------------------------------------------------------------------

func TestSetCallInfoCodec_ContentSubtypeRegistered(t *testing.T) {
	// contentSubtype set to a registered codec name
	c := &callInfo{contentSubtype: "proto", contentSubtypeSet: true}
	err := setCallInfoCodec(c)
	require.NoError(t, err)
	require.NotNil(t, c.codec)
	assert.Equal(t, "proto", c.codec.Name())
}

func TestSetCallInfoCodec_ContentSubtypeNotRegistered(t *testing.T) {
	// contentSubtype set but codec not registered
	c := &callInfo{contentSubtype: "nonexistent-codec", contentSubtypeSet: true}
	err := setCallInfoCodec(c)
	require.Error(t, err)
	assert.ErrorContains(t, err, "no codec registered")
}

// ---------------------------------------------------------------------------
// fromGRPCPeer grpcAuthInfo branch
// ---------------------------------------------------------------------------

func TestFromGRPCPeer_WithGRPCAuthInfo(t *testing.T) {
	innerAuth := testAuthInfo{authType: "test"}
	p := &gpeer.Peer{
		Addr:     &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 443},
		AuthInfo: grpcAuthInfo{base: innerAuth},
	}
	got := fromGRPCPeer(p, "grpc")
	require.NotNil(t, got)
	assert.Equal(t, "192.168.1.1", got.RemoteIP)
	require.NotNil(t, got.AuthInfo)
	assert.Equal(t, "test", got.AuthInfo.AuthType())
}

func TestFromGRPCPeer_NonTCPAddr(t *testing.T) {
	p := &gpeer.Peer{
		Addr: &net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 53},
	}
	got := fromGRPCPeer(p, "grpc")
	require.NotNil(t, got)
	assert.Empty(t, got.RemoteIP) // UDPAddr is not TCPAddr
}

// ---------------------------------------------------------------------------
// toGRPCError / toRPCErr CoverError branch
// ---------------------------------------------------------------------------

func TestToGRPCError_CoverError(t *testing.T) {
	st := ystatus.New(code.Code_NOT_FOUND, "item not found")
	err := toGRPCError(st.Err())
	require.Error(t, err)
	grpcSt, ok := gstatus.FromError(err)
	require.True(t, ok)
	assert.Equal(t, gcodes.NotFound, grpcSt.Code())
}

func TestToRPCErr_CoverError(t *testing.T) {
	// Create a ystatus-compatible error
	st := ystatus.New(code.Code_NOT_FOUND, "not found")
	err := toRPCErr(st.Err())
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// buildTransportCredentialsWithBuilders — tls builtin
// ---------------------------------------------------------------------------

func TestBuildTransportCredentials_TLS(t *testing.T) {
	creds, err := buildTransportCredentialsWithProfiles(map[string]security.Profile{
		"tls": mockProfile{
			name: "tls",
			material: security.Material{
				Mode:      security.ModeTLS,
				ClientTLS: &stdtls.Config{},
				ServerTLS: &stdtls.Config{},
			},
		},
	}, "tls", "svc", true, "")
	require.NoError(t, err)
	require.NotNil(t, creds)
}

// ---------------------------------------------------------------------------
// normalizeListenAddress additional cases
// ---------------------------------------------------------------------------

func TestNormalizeListenAddress_EmptyHost(t *testing.T) {
	addr, err := normalizeListenAddress("tcp", ":9090")
	require.NoError(t, err)
	assert.Contains(t, addr, ":9090")
}

func TestNormalizeListenAddress_IPv6Host(t *testing.T) {
	addr, err := normalizeListenAddress("tcp", "[::1]:8080")
	require.NoError(t, err)
	assert.Contains(t, addr, "8080")
}

// ---------------------------------------------------------------------------
// grpcConnectParams edge
// ---------------------------------------------------------------------------

func TestGRPCConnectParams_ZeroBackOffMaxDelay(t *testing.T) {
	cfg := &ClientConfig{BackOffMaxDelay: 0, MinConnectTimeout: 0}
	params := grpcConnectParams(cfg)
	assert.Equal(t, 120*time.Second, params.Backoff.MaxDelay)
}
