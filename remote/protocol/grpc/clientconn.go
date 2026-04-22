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

// Package grpc implements the gRPC client.
package grpc

import (
	"context"
	"errors"
	"io"
	"math"
	"net"
	"sync"
	"time"

	ggrpc "google.golang.org/grpc"
	gresolver "google.golang.org/grpc/connectivity"
	gkeepalive "google.golang.org/grpc/keepalive"
	gmetadata "google.golang.org/grpc/metadata"

	"github.com/codesjoy/yggdrasil/v3/metadata"
	"github.com/codesjoy/yggdrasil/v3/remote"
	"github.com/codesjoy/yggdrasil/v3/remote/protocol/grpc/encoding"
	"github.com/codesjoy/yggdrasil/v3/resolver"
	"github.com/codesjoy/yggdrasil/v3/stats"
	"github.com/codesjoy/yggdrasil/v3/stream"
)

const (
	minConnectTimeout                  = 20 * time.Second
	defaultClientMaxReceiveMessageSize = 1024 * 1024 * 4
	defaultClientMaxSendMessageSize    = math.MaxInt32
	defaultWriteBufSize                = 32 * 1024
	defaultReadBufSize                 = 32 * 1024
)

func init() {
	remote.RegisterClientBuilder("grpc", newClient)
}

// Config defines the configuration for a client.
type Config struct {
	WaitConnTimeout   time.Duration          `mapstructure:"wait_conn_timeout" default:"500ms"`
	Transport         clientTransportOptions `mapstructure:"transport"`
	ConnectTimeout    time.Duration          `mapstructure:"connect_timeout" default:"3s"`
	MaxSendMsgSize    int                    `mapstructure:"max_send_msg_size"`
	MaxRecvMsgSize    int                    `mapstructure:"max_recv_msg_size"`
	Compressor        string                 `mapstructure:"compressor"`
	BackOffMaxDelay   time.Duration          `mapstructure:"back_off_max_delay" default:"5s"`
	MinConnectTimeout time.Duration          `mapstructure:"min_connect_timeout" default:"1s"`
	Network           string                 `mapstructure:"network" default:"tcp"`
}

func (cfg *Config) setDefault(serviceName string) {
	if cfg.MaxSendMsgSize == 0 {
		cfg.MaxSendMsgSize = defaultClientMaxSendMessageSize
	}
	if cfg.MaxRecvMsgSize == 0 {
		cfg.MaxRecvMsgSize = defaultClientMaxReceiveMessageSize
	}
	if cfg.Transport.WriteBufferSize == 0 {
		cfg.Transport.WriteBufferSize = defaultWriteBufSize
	}
	if cfg.Transport.ReadBufferSize == 0 {
		cfg.Transport.ReadBufferSize = defaultReadBufSize
	}
	if cfg.Transport.Authority == "" {
		cfg.Transport.Authority = serviceName
	}
}

type clientConn struct {
	ctx    context.Context
	cancel context.CancelFunc
	cfg    *Config

	mu sync.RWMutex

	conn     *ggrpc.ClientConn
	state    remote.State
	endpoint resolver.Endpoint

	onStateChange remote.OnStateChange
}

func newClient(
	ctx context.Context,
	serviceName string,
	endpoint resolver.Endpoint,
	statsHandler stats.Handler,
	onStateChange remote.OnStateChange,
) (remote.Client, error) {
	resolved := currentSettings()
	cfg := &resolved.Client
	if serviceCfg, ok := resolved.ClientServices[serviceName]; ok {
		cfg = &serviceCfg
	}
	cfg.setDefault(serviceName)

	dialOpts, err := buildClientDialOptions(cfg, serviceName, statsHandler)
	if err != nil {
		return nil, err
	}
	conn, err := ggrpc.NewClient(grpcTargetForEndpoint(endpoint.GetAddress()), dialOpts...)
	if err != nil {
		return nil, err
	}

	cc := &clientConn{
		cfg:           cfg,
		conn:          conn,
		state:         remoteStateFromConnectivity(conn.GetState()),
		endpoint:      endpoint,
		onStateChange: onStateChange,
	}
	cc.ctx, cc.cancel = context.WithCancel(ctx)
	go cc.watchConnectivity()
	return cc, nil
}

func buildClientDialOptions(cfg *Config, serviceName string, statsHandler stats.Handler) ([]ggrpc.DialOption, error) {
	creds, err := buildTransportCredentials(cfg.Transport.CredsProto, serviceName, true)
	if err != nil {
		return nil, err
	}
	dialer := func(ctx context.Context, address string) (net.Conn, error) {
		if cfg.ConnectTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, cfg.ConnectTimeout)
			defer cancel()
		}
		return (&net.Dialer{}).DialContext(ctx, cfg.Network, address)
	}

	opts := []ggrpc.DialOption{
		ggrpc.WithTransportCredentials(creds),
		ggrpc.WithContextDialer(dialer),
		ggrpc.WithConnectParams(grpcConnectParams(cfg)),
		ggrpc.WithStatsHandler(newStatsHandlerBridge(statsHandler)),
		ggrpc.WithDisableHealthCheck(),
		ggrpc.WithDefaultServiceConfig(`{"loadBalancingConfig":[{"pick_first":{}}]}`),
		ggrpc.WithDefaultCallOptions(
			ggrpc.MaxCallSendMsgSize(cfg.MaxSendMsgSize),
			ggrpc.MaxCallRecvMsgSize(cfg.MaxRecvMsgSize),
		),
	}
	if cfg.Transport.Authority != "" {
		opts = append(opts, ggrpc.WithAuthority(cfg.Transport.Authority))
	}
	if cfg.Transport.UserAgent != "" {
		opts = append(opts, ggrpc.WithUserAgent(cfg.Transport.UserAgent))
	}
	if cfg.Transport.KeepaliveParams != (gkeepalive.ClientParameters{}) {
		opts = append(opts, ggrpc.WithKeepaliveParams(cfg.Transport.KeepaliveParams))
	}
	if cfg.Transport.InitialWindowSize > 0 {
		opts = append(opts, ggrpc.WithInitialWindowSize(cfg.Transport.InitialWindowSize))
	}
	if cfg.Transport.InitialConnWindowSize > 0 {
		opts = append(opts, ggrpc.WithInitialConnWindowSize(cfg.Transport.InitialConnWindowSize))
	}
	if cfg.Transport.WriteBufferSize > 0 {
		opts = append(opts, ggrpc.WithWriteBufferSize(cfg.Transport.WriteBufferSize))
	}
	if cfg.Transport.ReadBufferSize > 0 {
		opts = append(opts, ggrpc.WithReadBufferSize(cfg.Transport.ReadBufferSize))
	}
	if cfg.Transport.MaxHeaderListSize != nil {
		opts = append(opts, ggrpc.WithMaxHeaderListSize(*cfg.Transport.MaxHeaderListSize))
	}
	return opts, nil
}

func (cc *clientConn) watchConnectivity() {
	state := cc.conn.GetState()
	for {
		if state == gresolver.Shutdown {
			cc.mu.Lock()
			cc.changeStateUnlock(remote.Shutdown, nil)
			cc.mu.Unlock()
			return
		}
		if !cc.conn.WaitForStateChange(cc.ctx, state) {
			return
		}
		state = cc.conn.GetState()

		var connErr error
		if state == gresolver.TransientFailure {
			connErr = errors.New("grpc connection entered transient failure")
		}

		cc.mu.Lock()
		cc.changeStateUnlock(remoteStateFromConnectivity(state), connErr)
		cc.mu.Unlock()
	}
}

func (cc *clientConn) NewStream(
	ctx context.Context,
	desc *stream.Desc,
	method string,
) (stream.ClientStream, error) {
	if desc == nil {
		desc = &stream.Desc{}
	}
	c := defaultCallInfo()
	c.maxSendMessageSize = &cc.cfg.MaxSendMsgSize
	c.maxReceiveMessageSize = &cc.cfg.MaxRecvMsgSize
	if err := applyCallOptions(c, callOptionsFromContext(ctx)); err != nil {
		return nil, err
	}
	if err := setCallInfoCodec(c); err != nil {
		return nil, err
	}

	callOpts := []ggrpc.CallOption{
		ggrpc.MaxCallSendMsgSize(*c.maxSendMessageSize),
		ggrpc.MaxCallRecvMsgSize(*c.maxReceiveMessageSize),
	}
	if c.contentSubtype != "" {
		callOpts = append(callOpts, ggrpc.CallContentSubtype(c.contentSubtype))
	}
	if c.forceCodec && c.codec != nil {
		callOpts = append(callOpts, ggrpc.ForceCodecV2(grpcCodecV2ForLocal(c.codec)))
	}
	if cc.cfg.Compressor != "" && cc.cfg.Compressor != encoding.Identity {
		callOpts = append(callOpts, ggrpc.UseCompressor(cc.cfg.Compressor))
	}

	if md, ok := metadata.FromOutContext(ctx); ok {
		ctx = gmetadata.NewOutgoingContext(ctx, toGRPCMetadata(md))
	}

	grpcStream, err := cc.conn.NewStream(
		ctx,
		&ggrpc.StreamDesc{
			StreamName:    desc.StreamName,
			ServerStreams: desc.ServerStreams,
			ClientStreams: desc.ClientStreams,
		},
		method,
		callOpts...,
	)
	if err != nil {
		return nil, toRPCErr(err)
	}
	return &clientStream{
		ctx:          ctx,
		ClientStream: grpcStream,
	}, nil
}

func (cc *clientConn) Close() error {
	cc.mu.Lock()
	if cc.state == remote.Shutdown {
		cc.mu.Unlock()
		return errors.New("remote client closed")
	}
	cc.changeStateUnlock(remote.Shutdown, nil)
	cc.mu.Unlock()

	cc.cancel()
	return cc.conn.Close()
}

func (cc *clientConn) Scheme() string {
	return scheme
}

func (cc *clientConn) State() remote.State {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.state
}

func (cc *clientConn) Connect() {
	cc.conn.Connect()
}

func (cc *clientConn) changeStateUnlock(s remote.State, connErr error) {
	state := cc.state
	cc.state = s
	if state != s && s != remote.Shutdown && cc.onStateChange != nil {
		cc.onStateChange(remote.ClientState{
			Endpoint:        cc.endpoint,
			State:           s,
			ConnectionError: connErr,
		})
	}
}

type clientStream struct {
	ctx context.Context
	ggrpc.ClientStream
}

func (cs *clientStream) Header() (metadata.MD, error) {
	md, err := cs.ClientStream.Header()
	if err != nil {
		return metadata.MD{}, toRPCErr(err)
	}
	return fromGRPCMetadata(md), nil
}

func (cs *clientStream) Trailer() metadata.MD {
	return fromGRPCMetadata(cs.ClientStream.Trailer())
}

func (cs *clientStream) CloseSend() error {
	if err := cs.ClientStream.CloseSend(); err != nil {
		return toRPCErr(err)
	}
	return nil
}

func (cs *clientStream) Context() context.Context {
	return cs.ctx
}

func (cs *clientStream) SendMsg(m interface{}) error {
	if err := cs.ClientStream.SendMsg(m); err != nil {
		if errors.Is(err, io.EOF) {
			return err
		}
		return toRPCErr(err)
	}
	return nil
}

func (cs *clientStream) RecvMsg(m interface{}) error {
	if err := cs.ClientStream.RecvMsg(m); err != nil {
		if errors.Is(err, io.EOF) {
			return err
		}
		return toRPCErr(err)
	}
	return nil
}
