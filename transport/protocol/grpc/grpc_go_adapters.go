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
	"net"
	"strings"
	"time"

	"google.golang.org/genproto/googleapis/rpc/code"
	ggrpc "google.golang.org/grpc"
	gbackoff "google.golang.org/grpc/backoff"
	gcodes "google.golang.org/grpc/codes"
	gconnectivity "google.golang.org/grpc/connectivity"
	gcredentials "google.golang.org/grpc/credentials"
	ginsecure "google.golang.org/grpc/credentials/insecure"
	gkeepalive "google.golang.org/grpc/keepalive"
	gmetadata "google.golang.org/grpc/metadata"
	gpeer "google.golang.org/grpc/peer"
	gstats "google.golang.org/grpc/stats"
	gstatus "google.golang.org/grpc/status"

	"github.com/codesjoy/pkg/basic/xerror"

	ystats "github.com/codesjoy/yggdrasil/v3/observability/stats"
	ymetadata "github.com/codesjoy/yggdrasil/v3/rpc/metadata"
	ystatus "github.com/codesjoy/yggdrasil/v3/rpc/status"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
	stats2 "github.com/codesjoy/yggdrasil/v3/transport/protocol/grpc/stats"
	"github.com/codesjoy/yggdrasil/v3/transport/support/listenaddr"
	"github.com/codesjoy/yggdrasil/v3/transport/support/peer"
	"github.com/codesjoy/yggdrasil/v3/transport/support/security"
)

func toGRPCMetadata(md ymetadata.MD) gmetadata.MD {
	if len(md) == 0 {
		return nil
	}
	out := make(gmetadata.MD, len(md))
	for k, vs := range md {
		cp := make([]string, len(vs))
		copy(cp, vs)
		out[k] = cp
	}
	return out
}

func fromGRPCMetadata(md gmetadata.MD) ymetadata.MD {
	if len(md) == 0 {
		return ymetadata.MD{}
	}
	out := make(ymetadata.MD, len(md))
	for k, vs := range md {
		cp := make([]string, len(vs))
		copy(cp, vs)
		out[strings.ToLower(k)] = cp
	}
	return out
}

func remoteStateFromConnectivity(state gconnectivity.State) remote.State {
	switch state {
	case gconnectivity.Idle:
		return remote.Idle
	case gconnectivity.Connecting:
		return remote.Connecting
	case gconnectivity.Ready:
		return remote.Ready
	case gconnectivity.TransientFailure:
		return remote.TransientFailure
	case gconnectivity.Shutdown:
		return remote.Shutdown
	default:
		return remote.Idle
	}
}

func toGRPCError(err error) error {
	if err == nil {
		return nil
	}
	if st, ok := ystatus.CoverError(err); ok {
		return gstatus.FromProto(st.Status()).Err()
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return gstatus.Error(gcodes.DeadlineExceeded, err.Error())
	case errors.Is(err, context.Canceled):
		return gstatus.Error(gcodes.Canceled, err.Error())
	default:
		return gstatus.Error(gcodes.Unknown, err.Error())
	}
}

func toRPCErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, io.EOF):
		return err
	case errors.Is(err, context.DeadlineExceeded):
		return ystatus.WithCode(code.Code_DEADLINE_EXCEEDED, err)
	case errors.Is(err, context.Canceled):
		return ystatus.WithCode(code.Code_CANCELLED, err)
	case errors.Is(err, io.ErrUnexpectedEOF):
		return xerror.New(code.Code_INTERNAL, io.ErrUnexpectedEOF.Error())
	}
	if st, ok := gstatus.FromError(err); ok {
		return ystatus.FromProto(st.Proto())
	}
	if _, ok := ystatus.CoverError(err); ok {
		return err
	}
	return ystatus.WithCode(code.Code_UNKNOWN, err)
}

func grpcTargetForEndpoint(address string) string {
	return "passthrough:///" + address
}

func grpcConnectParams(cfg *ClientConfig) ggrpc.ConnectParams {
	params := ggrpc.ConnectParams{
		Backoff: gbackoff.Config{
			BaseDelay:  1.0 * time.Second,
			Multiplier: 1.6,
			Jitter:     0.2,
			MaxDelay:   120 * time.Second,
		},
		MinConnectTimeout: minConnectTimeout,
	}
	if cfg.BackOffMaxDelay > 0 {
		params.Backoff.MaxDelay = cfg.BackOffMaxDelay
	}
	if cfg.MinConnectTimeout > 0 {
		params.MinConnectTimeout = cfg.MinConnectTimeout
	}
	return params
}

type grpcAuthInfo struct {
	base security.AuthInfo
}

func (a grpcAuthInfo) AuthType() string {
	return a.base.AuthType()
}

func (a grpcAuthInfo) GetCommonAuthInfo() gcredentials.CommonAuthInfo {
	type commonInfo interface {
		GetCommonAuthInfo() security.CommonAuthInfo
	}
	ci, ok := a.base.(commonInfo)
	if !ok {
		return gcredentials.CommonAuthInfo{}
	}
	level := ci.GetCommonAuthInfo().SecurityLevel
	out := gcredentials.CommonAuthInfo{}
	switch level {
	case security.NoSecurity:
		out.SecurityLevel = gcredentials.NoSecurity
	case security.IntegrityOnly:
		out.SecurityLevel = gcredentials.IntegrityOnly
	case security.PrivacyAndIntegrity:
		out.SecurityLevel = gcredentials.PrivacyAndIntegrity
	}
	return out
}

type transportCredentialsBridge struct {
	base security.ConnAuthenticator
}

func (b transportCredentialsBridge) ClientHandshake(
	ctx context.Context,
	authority string,
	rawConn net.Conn,
) (net.Conn, gcredentials.AuthInfo, error) {
	conn, authInfo, err := b.base.ClientHandshake(ctx, authority, rawConn)
	if err != nil {
		return nil, nil, err
	}
	if authInfo == nil {
		return conn, nil, nil
	}
	return conn, grpcAuthInfo{base: authInfo}, nil
}

func (b transportCredentialsBridge) ServerHandshake(
	rawConn net.Conn,
) (net.Conn, gcredentials.AuthInfo, error) {
	conn, authInfo, err := b.base.ServerHandshake(rawConn)
	if err != nil {
		return nil, nil, err
	}
	if authInfo == nil {
		return conn, nil, nil
	}
	return conn, grpcAuthInfo{base: authInfo}, nil
}

func (b transportCredentialsBridge) Info() gcredentials.ProtocolInfo {
	info := b.base.Info()
	return gcredentials.ProtocolInfo{
		ProtocolVersion:  info.ProtocolVersion,
		SecurityProtocol: info.SecurityProtocol,
		SecurityVersion:  info.SecurityVersion,
		ServerName:       info.ServerName,
	}
}

func (b transportCredentialsBridge) Clone() gcredentials.TransportCredentials {
	return transportCredentialsBridge{base: b.base.Clone()}
}

func (b transportCredentialsBridge) OverrideServerName(serverName string) error {
	return b.base.OverrideServerName(serverName)
}

func buildTransportCredentials(
	profileName string,
	serviceName string,
	client bool,
	authority string,
) (gcredentials.TransportCredentials, error) {
	return buildTransportCredentialsWithProfiles(nil, profileName, serviceName, client, authority)
}

func buildTransportCredentialsWithProfiles(
	profiles map[string]security.Profile,
	profileName string,
	serviceName string,
	client bool,
	authority string,
) (gcredentials.TransportCredentials, error) {
	if profileName == "" {
		return ginsecure.NewCredentials(), nil
	}
	var profile security.Profile
	if profiles != nil {
		profile = profiles[profileName]
	}
	if profile == nil {
		return nil, fmt.Errorf("security profile %q not found", profileName)
	}
	side := security.SideServer
	if client {
		side = security.SideClient
	}
	material, err := profile.Build(security.BuildSpec{
		Protocol:    Protocol,
		Side:        side,
		ServiceName: serviceName,
		Authority:   authority,
	})
	if err != nil {
		return nil, err
	}
	switch material.Mode {
	case security.ModeInsecure:
		return ginsecure.NewCredentials(), nil
	case security.ModeTLS:
		cfg := material.ServerTLS
		if client {
			cfg = material.ClientTLS
		}
		if cfg == nil {
			return nil, fmt.Errorf("security profile %q returned nil tls config", profileName)
		}
		return gcredentials.NewTLS(cfg), nil
	case security.ModeLocal:
		if material.ConnAuth == nil {
			return nil, fmt.Errorf(
				"security profile %q returned nil connection authenticator",
				profileName,
			)
		}
		return transportCredentialsBridge{base: material.ConnAuth}, nil
	default:
		return nil, fmt.Errorf(
			"security profile %q returned unsupported mode %q",
			profileName,
			material.Mode,
		)
	}
}

func fromGRPCPeer(p *gpeer.Peer, protocol string) *peer.Peer {
	if p == nil {
		return nil
	}
	out := &peer.Peer{
		Addr:      p.Addr,
		LocalAddr: p.LocalAddr,
		Protocol:  protocol,
	}
	if tcp, ok := p.Addr.(*net.TCPAddr); ok {
		out.RemoteIP = tcp.IP.String()
	}
	if auth, ok := p.AuthInfo.(grpcAuthInfo); ok {
		out.AuthInfo = auth.base
	}
	return out
}

type statsHandlerBridge struct {
	handler ystats.Handler
}

func newStatsHandlerBridge(handler ystats.Handler) gstats.Handler {
	if handler == nil {
		return nil
	}
	return &statsHandlerBridge{handler: handler}
}

func (b *statsHandlerBridge) TagRPC(ctx context.Context, info *gstats.RPCTagInfo) context.Context {
	if info == nil {
		return ctx
	}
	return b.handler.TagRPC(ctx, &ystats.RPCTagInfoBase{FullMethod: info.FullMethodName})
}

func (b *statsHandlerBridge) HandleRPC(ctx context.Context, rs gstats.RPCStats) {
	switch s := rs.(type) {
	case *gstats.Begin:
		b.handler.HandleRPC(ctx, &ystats.RPCBeginBase{
			Client:       s.Client,
			BeginTime:    s.BeginTime,
			ClientStream: s.IsClientStream,
			ServerStream: s.IsServerStream,
			Protocol:     Protocol,
		})
	case *gstats.InHeader:
		header := fromGRPCMetadata(s.Header)
		if s.Client {
			b.handler.HandleRPC(ctx, &stats2.ClientInHeader{
				RPCClientInHeaderBase: ystats.RPCClientInHeaderBase{
					RPCInHeaderBase: ystats.RPCInHeaderBase{
						Header:        header,
						Protocol:      Protocol,
						TransportSize: s.WireLength,
					},
				},
				Compression: s.Compression,
			})
			return
		}
		b.handler.HandleRPC(ctx, &stats2.ServerInHeader{
			RPCServerInHeaderBase: ystats.RPCServerInHeaderBase{
				RPCInHeaderBase: ystats.RPCInHeaderBase{
					Header:        header,
					Protocol:      Protocol,
					TransportSize: s.WireLength,
				},
				FullMethod:     s.FullMethod,
				RemoteEndpoint: addrString(s.RemoteAddr),
				LocalEndpoint:  addrString(s.LocalAddr),
			},
			Compression: s.Compression,
		})
	case *gstats.OutHeader:
		b.handler.HandleRPC(ctx, &stats2.OutHeader{
			OutHeaderBase: ystats.OutHeaderBase{
				Client:         s.Client,
				Header:         fromGRPCMetadata(s.Header),
				FullMethod:     s.FullMethod,
				RemoteEndpoint: addrString(s.RemoteAddr),
				LocalEndpoint:  addrString(s.LocalAddr),
				Protocol:       Protocol,
			},
			Compression: s.Compression,
		})
	case *gstats.InTrailer:
		b.handler.HandleRPC(ctx, &ystats.RPCInTrailerBase{
			Client:        s.Client,
			Trailer:       fromGRPCMetadata(s.Trailer),
			TransportSize: s.WireLength,
			Protocol:      Protocol,
		})
	case *gstats.OutTrailer:
		b.handler.HandleRPC(ctx, &ystats.OutTrailerBase{
			Client:        s.Client,
			Trailer:       fromGRPCMetadata(s.Trailer),
			TransportSize: s.WireLength, //nolint:staticcheck // SA1019: WireLength is the only available field for transport size in OutTrailer
		})
	case *gstats.InPayload:
		b.handler.HandleRPC(ctx, &stats2.InPayload{
			RPCInPayloadBase: ystats.RPCInPayloadBase{
				Client:        s.Client,
				Payload:       s.Payload,
				TransportSize: s.WireLength,
				RecvTime:      s.RecvTime,
				Protocol:      Protocol,
			},
			CompressedLength: s.CompressedLength,
		})
	case *gstats.OutPayload:
		b.handler.HandleRPC(ctx, &stats2.OutPayload{
			RPCOutPayloadBase: ystats.RPCOutPayloadBase{
				Client:        s.Client,
				Payload:       s.Payload,
				TransportSize: s.WireLength,
				SendTime:      s.SentTime,
				Protocol:      Protocol,
			},
			CompressedLength: s.CompressedLength,
		})
	case *gstats.End:
		b.handler.HandleRPC(ctx, &ystats.RPCEndBase{
			Client:    s.Client,
			BeginTime: s.BeginTime,
			EndTime:   s.EndTime,
			Err:       toRPCErr(s.Error),
			Protocol:  Protocol,
		})
	}
}

func (b *statsHandlerBridge) TagConn(
	ctx context.Context,
	info *gstats.ConnTagInfo,
) context.Context {
	if info == nil {
		return ctx
	}
	return b.handler.TagChannel(ctx, &ystats.ChanTagInfoBase{
		RemoteEndpoint: addrString(info.RemoteAddr),
		LocalEndpoint:  addrString(info.LocalAddr),
		Protocol:       Protocol,
	})
}

func (b *statsHandlerBridge) HandleConn(ctx context.Context, cs gstats.ConnStats) {
	switch s := cs.(type) {
	case *gstats.ConnBegin:
		b.handler.HandleChannel(ctx, &ystats.ChanBeginBase{Client: s.Client})
	case *gstats.ConnEnd:
		b.handler.HandleChannel(ctx, &ystats.ChanEndBase{Client: s.Client})
	}
}

func addrString(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	return addr.String()
}

// ClientTransportOptions configures low-level gRPC client transport settings.
type ClientTransportOptions struct {
	UserAgent             string                      `mapstructure:"user_agent"`
	SecurityProfile       string                      `mapstructure:"security_profile"`
	Authority             string                      `mapstructure:"authority"`
	KeepaliveParams       gkeepalive.ClientParameters `mapstructure:"keepalive_params"`
	InitialWindowSize     int32                       `mapstructure:"initial_window_size"`
	InitialConnWindowSize int32                       `mapstructure:"initial_conn_window_size"`
	WriteBufferSize       int                         `mapstructure:"write_buffer_size"`
	ReadBufferSize        int                         `mapstructure:"read_buffer_size"`
	MaxHeaderListSize     *uint32                     `mapstructure:"max_header_list_size"`
}

func buildIncomingContext(ctx context.Context) context.Context {
	if md, ok := gmetadata.FromIncomingContext(ctx); ok {
		ctx = ymetadata.WithInContext(ctx, fromGRPCMetadata(md))
	}
	if p, ok := gpeer.FromContext(ctx); ok {
		ctx = peer.WithContext(ctx, fromGRPCPeer(p, Protocol))
	}
	return ymetadata.WithStreamContext(ctx)
}

func normalizeListenAddress(network, address string) (string, error) {
	host, port := "", "0"
	if address != "" {
		var err error
		host, port, err = net.SplitHostPort(address)
		if err != nil {
			return "", err
		}
		_ = network
	}
	host, err := listenaddr.NormalizeListenHost(host)
	if err != nil {
		return "", err
	}
	return net.JoinHostPort(host, port), nil
}
