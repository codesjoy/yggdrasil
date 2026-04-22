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
	"math"
	"net"
	"sync"
	"time"

	ggrpc "google.golang.org/grpc"
	gcredentials "google.golang.org/grpc/credentials"
	gkeepalive "google.golang.org/grpc/keepalive"

	"github.com/codesjoy/yggdrasil/v3/metadata"
	"github.com/codesjoy/yggdrasil/v3/remote"
	"github.com/codesjoy/yggdrasil/v3/remote/protocol/grpc/encoding"
	"github.com/codesjoy/yggdrasil/v3/stats"
)

func init() {
	remote.RegisterServerBuilder("grpc", newServer)
}

const (
	defaultServerMaxReceiveMessageSize = 1024 * 1024 * 4
	defaultServerMaxSendMessageSize    = math.MaxInt32
)

type serverOptions struct {
	Network               string                       `mapstructure:"network"`
	Address               string                       `mapstructure:"address"`
	CredsProto            string                       `mapstructure:"creds_proto"`
	CodeProto             string                       `mapstructure:"code_proto"`
	MaxConcurrentStreams  uint32                       `mapstructure:"max_concurrent_streams"`
	MaxReceiveMessageSize int                          `mapstructure:"max_receive_message_size"`
	MaxSendMessageSize    int                          `mapstructure:"max_send_message_size"`
	KeepaliveParams       gkeepalive.ServerParameters  `mapstructure:"keepalive_params"`
	KeepalivePolicy       gkeepalive.EnforcementPolicy `mapstructure:"keepalive_policy"`
	InitialWindowSize     int32                        `mapstructure:"initial_window_size"`
	InitialConnWindowSize int32                        `mapstructure:"initial_conn_window_size"`
	WriteBufferSize       int                          `mapstructure:"write_buffer_size"`
	ReadBufferSize        int                          `mapstructure:"read_buffer_size"`
	ConnectionTimeout     time.Duration                `mapstructure:"connection_timeout"`
	MaxHeaderListSize     *uint32                      `mapstructure:"max_header_list_size"`
	HeaderTableSize       *uint32                      `mapstructure:"header_table_size"`

	Attr map[string]string `mapstructure:"attr"`

	creds gcredentials.TransportCredentials
	codec encoding.Codec
}

func (opts *serverOptions) SetDefault() error {
	if opts.Network == "" {
		opts.Network = "tcp"
	}
	address, err := normalizeListenAddress(opts.Network, opts.Address)
	if err != nil {
		return err
	}
	opts.Address = address
	if opts.Attr == nil {
		opts.Attr = make(map[string]string)
	}
	if opts.MaxReceiveMessageSize == 0 {
		opts.MaxReceiveMessageSize = defaultServerMaxReceiveMessageSize
	}
	if opts.MaxSendMessageSize == 0 {
		opts.MaxSendMessageSize = defaultServerMaxSendMessageSize
	}
	if opts.WriteBufferSize == 0 {
		opts.WriteBufferSize = defaultWriteBufSize
	}
	if opts.ReadBufferSize == 0 {
		opts.ReadBufferSize = defaultReadBufSize
	}
	if opts.ConnectionTimeout == 0 {
		opts.ConnectionTimeout = 120 * time.Second
	}
	if opts.CredsProto != "" {
		creds, err := buildTransportCredentials(opts.CredsProto, "", false)
		if err != nil {
			return err
		}
		opts.creds = creds
	}
	if opts.CodeProto != "" {
		opts.codec = encoding.GetCodec(opts.CodeProto)
		if opts.codec == nil {
			return errors.New("grpc: configured codec is not registered")
		}
	}
	return nil
}

type server struct {
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
	address   string
	lis       net.Listener
	serve     bool
	stopped   bool
	stoppedCh chan struct{}

	opts         serverOptions
	handle       remote.MethodHandle
	statsHandler stats.Handler
	grpcServer   *ggrpc.Server
}

func newServer(handle remote.MethodHandle) (remote.Server, error) {
	opts := currentSettings().Server
	if err := opts.SetDefault(); err != nil {
		return nil, err
	}

	s := &server{
		stoppedCh:    make(chan struct{}),
		opts:         opts,
		handle:       handle,
		statsHandler: stats.GetServerHandler(),
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.grpcServer = ggrpc.NewServer(s.serverOptions()...)
	return s, nil
}

func (s *server) serverOptions() []ggrpc.ServerOption {
	opts := []ggrpc.ServerOption{
		ggrpc.UnknownServiceHandler(func(_ interface{}, stream ggrpc.ServerStream) error {
			return s.handleUnknown(stream)
		}),
		ggrpc.StatsHandler(newStatsHandlerBridge(s.statsHandler)),
		ggrpc.MaxRecvMsgSize(s.opts.MaxReceiveMessageSize),
		ggrpc.MaxSendMsgSize(s.opts.MaxSendMessageSize),
		ggrpc.ConnectionTimeout(s.opts.ConnectionTimeout),
	}
	if s.opts.creds != nil {
		opts = append(opts, ggrpc.Creds(s.opts.creds))
	}
	if s.opts.MaxConcurrentStreams > 0 {
		opts = append(opts, ggrpc.MaxConcurrentStreams(s.opts.MaxConcurrentStreams))
	}
	if s.opts.KeepaliveParams != (gkeepalive.ServerParameters{}) {
		opts = append(opts, ggrpc.KeepaliveParams(s.opts.KeepaliveParams))
	}
	if s.opts.KeepalivePolicy != (gkeepalive.EnforcementPolicy{}) {
		opts = append(opts, ggrpc.KeepaliveEnforcementPolicy(s.opts.KeepalivePolicy))
	}
	if s.opts.InitialWindowSize > 0 {
		opts = append(opts, ggrpc.InitialWindowSize(s.opts.InitialWindowSize))
	}
	if s.opts.InitialConnWindowSize > 0 {
		opts = append(opts, ggrpc.InitialConnWindowSize(s.opts.InitialConnWindowSize))
	}
	if s.opts.WriteBufferSize > 0 {
		opts = append(opts, ggrpc.WriteBufferSize(s.opts.WriteBufferSize))
	}
	if s.opts.ReadBufferSize > 0 {
		opts = append(opts, ggrpc.ReadBufferSize(s.opts.ReadBufferSize))
	}
	if s.opts.MaxHeaderListSize != nil {
		opts = append(opts, ggrpc.MaxHeaderListSize(*s.opts.MaxHeaderListSize))
	}
	if s.opts.HeaderTableSize != nil {
		opts = append(opts, ggrpc.HeaderTableSize(*s.opts.HeaderTableSize))
	}
	if s.opts.codec != nil {
		opts = append(opts, ggrpc.ForceServerCodecV2(grpcCodecV2ForLocal(s.opts.codec)))
	}
	return opts
}

func (s *server) handleUnknown(stream ggrpc.ServerStream) error {
	ss := &serverStream{
		ctx:    buildIncomingContext(stream.Context()),
		stream: stream,
		method: methodFromServerStream(stream),
	}
	s.handle(ss)
	if err := ss.applyContextMetadata(); err != nil {
		return toGRPCError(err)
	}
	if ss.finishErr != nil {
		return toGRPCError(ss.finishErr)
	}
	if !ss.isClientStream && !ss.isServerStream && ss.finishReply != nil {
		if err := stream.SendMsg(ss.finishReply); err != nil {
			return err
		}
	}
	return nil
}

func (s *server) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	if !s.serve {
		if !s.stopped {
			s.stopped = true
			close(s.stoppedCh)
		}
		stoppedCh := s.stoppedCh
		s.mu.Unlock()
		select {
		case <-stoppedCh:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if s.stopped {
		stoppedCh := s.stoppedCh
		s.mu.Unlock()
		select {
		case <-stoppedCh:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	s.stopped = true
	stoppedCh := s.stoppedCh
	s.mu.Unlock()

	done := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		s.cancel()
		close(stoppedCh)
		return nil
	case <-ctx.Done():
		s.grpcServer.Stop()
		s.cancel()
		close(stoppedCh)
		return ctx.Err()
	}
}

func (s *server) Info() remote.ServerInfo {
	return remote.ServerInfo{
		Address:    s.address,
		Protocol:   scheme,
		Attributes: s.opts.Attr,
	}
}

func (s *server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return errors.New("server had already stopped")
	}
	if s.serve {
		return errors.New("server had already serve")
	}
	ctx, cancel := context.WithTimeout(s.ctx, time.Second)
	defer cancel()
	lis, err := (&net.ListenConfig{}).Listen(ctx, s.opts.Network, s.opts.Address)
	if err != nil {
		return err
	}
	s.lis = lis
	s.address = lis.Addr().String()
	s.serve = true
	return nil
}

func (s *server) Handle() error {
	err := s.grpcServer.Serve(s.lis)
	if errors.Is(err, ggrpc.ErrServerStopped) {
		return nil
	}
	return err
}

type serverStream struct {
	ctx    context.Context
	stream ggrpc.ServerStream
	method string

	isClientStream bool
	isServerStream bool
	headerApplied  bool
	trailerApplied bool

	finishReply any
	finishErr   error
}

func (ss *serverStream) Context() context.Context {
	return ss.ctx
}

func (ss *serverStream) SendHeader(md metadata.MD) error {
	if err := ss.applyPendingHeader(); err != nil {
		return err
	}
	return toRPCErr(ss.stream.SendHeader(toGRPCMetadata(md)))
}

func (ss *serverStream) SetTrailer(md metadata.MD) {
	if md.Len() == 0 {
		return
	}
	ss.stream.SetTrailer(toGRPCMetadata(md))
}

func (ss *serverStream) SetHeader(md metadata.MD) error {
	if md.Len() == 0 {
		return nil
	}
	return toRPCErr(ss.stream.SetHeader(toGRPCMetadata(md)))
}

func (ss *serverStream) SendMsg(m interface{}) error {
	if err := ss.applyPendingHeader(); err != nil {
		return err
	}
	return toRPCErr(ss.stream.SendMsg(m))
}

func (ss *serverStream) RecvMsg(m interface{}) error {
	err := ss.stream.RecvMsg(m)
	if err == nil || errors.Is(err, io.EOF) {
		return err
	}
	return toRPCErr(err)
}

func (ss *serverStream) Method() string {
	return ss.method
}

func (ss *serverStream) Start(isClientStream, isServerStream bool) error {
	ss.isClientStream = isClientStream
	ss.isServerStream = isServerStream
	return nil
}

func (ss *serverStream) Finish(reply any, err error) {
	if err := ss.applyPendingTrailer(); err != nil && ss.finishErr == nil {
		ss.finishErr = err
		return
	}
	ss.finishReply = reply
	ss.finishErr = err
}

func (ss *serverStream) applyContextMetadata() error {
	if err := ss.applyPendingHeader(); err != nil {
		return err
	}
	return ss.applyPendingTrailer()
}

func (ss *serverStream) applyPendingHeader() error {
	if ss.headerApplied {
		return nil
	}
	md, ok := metadata.FromHeaderCtx(ss.ctx)
	if !ok || md.Len() == 0 {
		return nil
	}
	ss.headerApplied = true
	return ss.SetHeader(md)
}

func (ss *serverStream) applyPendingTrailer() error {
	if ss.trailerApplied {
		return nil
	}
	md, ok := metadata.FromTrailerCtx(ss.ctx)
	if !ok || md.Len() == 0 {
		return nil
	}
	ss.trailerApplied = true
	ss.SetTrailer(md)
	return nil
}

func methodFromServerStream(stream ggrpc.ServerStream) string {
	method, ok := ggrpc.MethodFromServerStream(stream)
	if !ok {
		return ""
	}
	return method
}
