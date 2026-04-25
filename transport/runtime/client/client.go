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

// Package client provides a client implementation for the Yggdrasil framework.
package client

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"

	"github.com/codesjoy/pkg/basic/xerror"
	"github.com/codesjoy/pkg/utils/xgo"
	"github.com/codesjoy/pkg/utils/xsync"
	"google.golang.org/genproto/googleapis/rpc/code"

	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	"github.com/codesjoy/yggdrasil/v3/internal/backoff"
	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	"github.com/codesjoy/yggdrasil/v3/rpc/interceptor"
	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
	"github.com/codesjoy/yggdrasil/v3/rpc/stream"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/client/balancer"
)

// ErrClientClosing is returned when the client is closing.
var ErrClientClosing = xerror.New(code.Code_CANCELLED, "the client is closing")

// Client is the client interface.
type Client interface {
	Invoke(ctx context.Context, method string, args, reply interface{}) error
	NewStream(ctx context.Context, desc *stream.Desc, method string) (stream.ClientStream, error)
	Close() error
}

// Runtime exposes the App-scoped runtime dependencies needed by the client package.
type Runtime interface {
	ClientSettings(serviceName string) ServiceSettings
	ClientStatsHandler() stats.Handler
	TransportClientProvider(protocol string) remote.TransportClientProvider
	NewResolver(name string) (resolver.Resolver, error)
	NewBalancer(serviceName, balancerName string, cli balancer.Client) (balancer.Balancer, error)
	BuildUnaryClientInterceptor(
		serviceName string,
		names []string,
	) interceptor.UnaryClientInterceptor
	BuildStreamClientInterceptor(
		serviceName string,
		names []string,
	) interceptor.StreamClientInterceptor
}

type clientStream struct {
	desc *stream.Desc
	stream.ClientStream
	report   func(err error)
	reported atomic.Bool
}

func (c *clientStream) reportResult(err error) {
	if c.report == nil {
		return
	}
	if c.reported.CompareAndSwap(false, true) {
		c.report(err)
	}
}

func (c *clientStream) SendMsg(m interface{}) error {
	err := c.ClientStream.SendMsg(m)
	if err != nil && err != io.EOF {
		c.reportResult(err)
	}
	return err
}

func (c *clientStream) RecvMsg(m interface{}) error {
	err := c.ClientStream.RecvMsg(m)
	if !c.desc.ServerStreams {
		if header, _ := c.Header(); header != nil {
			_ = metadata.SetHeader(c.Context(), header)
		}
		if trailer := c.Trailer(); trailer != nil {
			_ = metadata.SetTrailer(c.Context(), trailer)
		}
	}
	if err == nil && !c.desc.ServerStreams {
		c.reportResult(nil)
		return nil
	}
	if err == io.EOF && c.desc.ServerStreams {
		c.reportResult(nil)
		return err
	}
	if err != nil && err != io.EOF {
		c.reportResult(err)
	}
	return err
}

type client struct {
	ctx    context.Context
	cancel context.CancelFunc

	appName  string
	fastFail bool

	resolver resolver.Resolver
	balancer balancer.Balancer

	unaryInterceptor  interceptor.UnaryClientInterceptor
	streamInterceptor interceptor.StreamClientInterceptor
	statsHandler      stats.Handler

	pickerSnap atomic.Pointer[pickerSnap]

	streamBackoff backoff.Strategy
	stateChange   chan resolver.State
	resolvedEvent *xsync.Event
	channelState  atomic.Int32

	remoteClientManager *remoteClientManager
	balancerClient      *balancerClient
	closed              atomic.Bool
	runtime             Runtime
}

// New creates a new client from one explicit runtime snapshot.
func New(ctx context.Context, appName string, runtimeSnapshot Runtime) (_ Client, err error) {
	if runtimeSnapshot == nil {
		return nil, errors.New("client runtime is required")
	}
	cfg := runtimeSnapshot.ClientSettings(appName)
	statsHandler := runtimeSnapshot.ClientStatsHandler()
	cli := &client{
		appName:       appName,
		fastFail:      cfg.FastFail,
		statsHandler:  statsHandler,
		stateChange:   make(chan resolver.State, 1),
		resolvedEvent: xsync.NewEvent(),
		runtime:       runtimeSnapshot,
	}
	cli.ctx, cli.cancel = context.WithCancel(ctx)
	cli.channelState.Store(int32(remote.Idle))

	cli.remoteClientManager = newRemoteClientManager(
		cli.ctx,
		appName,
		statsHandler,
		runtimeSnapshot,
	)
	watchRegistered := false
	defer func() {
		if err == nil {
			return
		}
		if watchRegistered && cli.resolver != nil {
			if cleanupErr := cli.resolver.DelWatch(cli.appName, cli); cleanupErr != nil {
				slog.Error("failed to clean up resolver watch", slog.Any("error", cleanupErr))
			}
		}
		if cli.balancer != nil {
			if cleanupErr := cli.balancer.Close(); cleanupErr != nil {
				slog.Error("failed to clean up balancer", slog.Any("error", cleanupErr))
			}
		}
		if cli.remoteClientManager != nil {
			if cleanupErr := cli.remoteClientManager.Close(); cleanupErr != nil {
				slog.Error(
					"failed to clean up remote client manager",
					slog.Any("error", cleanupErr),
				)
			}
		}
		if cli.cancel != nil {
			cli.cancel()
		}
	}()

	cli.streamBackoff = backoff.Exponential{Config: cfg.Backoff}
	cli.pickerSnap.Store(&pickerSnap{
		picker:     nil,
		blockingCh: make(chan struct{}),
	})
	if err = cli.initResolverAndBalancer(cfg); err != nil {
		return nil, err
	}
	cli.initInterceptor()
	if cli.resolver != nil {
		if err = cli.resolver.AddWatch(cli.appName, cli); err != nil {
			return nil, err
		}
		watchRegistered = true
		xgo.Go(cli.watchUpdateState)
	} else {
		if err = cli.updateStaticState(cfg); err != nil {
			return nil, err
		}
	}
	return cli, nil
}
