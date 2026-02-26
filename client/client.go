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
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sync/atomic"
	"time"

	"github.com/codesjoy/pkg/basic/xerror"
	"github.com/codesjoy/yggdrasil/v2/balancer"
	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/interceptor"
	"github.com/codesjoy/yggdrasil/v2/internal/backoff"
	"github.com/codesjoy/yggdrasil/v2/metadata"
	"github.com/codesjoy/yggdrasil/v2/resolver"
	"github.com/codesjoy/yggdrasil/v2/stats"
	"github.com/codesjoy/yggdrasil/v2/stream"
	"github.com/codesjoy/yggdrasil/v2/utils/xarray"
	"github.com/codesjoy/yggdrasil/v2/utils/xgo"
	"github.com/codesjoy/yggdrasil/v2/utils/xsync"
	"google.golang.org/genproto/googleapis/rpc/code"
)

// ErrClientClosing is returned when the client is closing.
var ErrClientClosing = xerror.New(code.Code_CANCELLED, "the client is closing")

// Client is the client interface.
type Client interface {
	// Invoke performs a unary RPC and returns after the response is received into reply.
	Invoke(ctx context.Context, method string, args, reply interface{}) error
	// NewStream begins a streaming RPC.
	NewStream(
		ctx context.Context,
		desc *stream.Desc,
		method string,
	) (stream.ClientStream, error)
	// Close destroy the client resource.
	Close() error
}

type clientStream struct {
	desc *stream.Desc
	stream.ClientStream
	report func(err error)
}

func (c *clientStream) SendMsg(m interface{}) error {
	err := c.ClientStream.SendMsg(m)
	if err != nil && err != io.EOF {
		c.report(err)
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
	if err != nil && err != io.EOF && !c.desc.ServerStreams {
		c.report(err)
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

	remoteClientManager *remoteClientManager
	closed              atomic.Bool
}

// NewClient creates a new client.
func NewClient(ctx context.Context, appName string) (Client, error) {
	cfgKey := config.Join(config.KeyBase, "client", fmt.Sprintf("{%s}", appName))
	cfg := config.ValueToValues(config.Get(cfgKey))
	statsHandler := stats.GetClientHandler()
	cli := &client{
		appName:       appName,
		fastFail:      cfg.Get("fastFail").Bool(false),
		statsHandler:  statsHandler,
		stateChange:   make(chan resolver.State, 1),
		resolvedEvent: xsync.NewEvent(),
	}
	cli.ctx, cli.cancel = context.WithCancel(ctx)

	cli.remoteClientManager = newRemoteClientManager(cli.ctx, appName, statsHandler)

	boCfg := backoff.Config{}
	if err := cfg.Get(config.Join("backoff")).Scan(&boCfg); err != nil {
		return nil, err
	}
	cli.streamBackoff = backoff.Exponential{Config: boCfg}
	// Initialize pickerSnap to avoid nil pointer panic on first updatePicker call
	cli.pickerSnap.Store(&pickerSnap{
		picker:     nil,
		blockingCh: make(chan struct{}),
	})
	if err := cli.initResolverAndBalancer(cfg); err != nil {
		return nil, err
	}
	cli.initInterceptor()
	if cli.resolver != nil {
		xgo.Go(cli.watchUpdateState)
		if err := cli.resolver.AddWatch(cli.appName, cli); err != nil {
			return nil, err
		}
	} else {
		if err := cli.updateStaticState(cfg); err != nil {
			return nil, err
		}
	}
	return cli, nil
}

// Invoke performs a unary RPC and returns after the response is received into reply.
func (c *client) Invoke(ctx context.Context, method string, args, reply interface{}) error {
	ctx = metadata.WithStreamContext(ctx)
	if c.unaryInterceptor != nil {
		return c.unaryInterceptor(ctx, method, args, reply, c.invoke)
	}
	return c.invoke(ctx, method, args, reply)
}

// NewStream creates a new stream.
func (c *client) NewStream(
	ctx context.Context,
	desc *stream.Desc,
	method string,
) (stream.ClientStream, error) {
	if c.streamInterceptor != nil {
		return c.streamInterceptor(ctx, desc, method, c.newStream)
	}
	return c.newStream(ctx, desc, method)
}

// Close closes the client.
func (c *client) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return ErrClientClosing
	}
	var multiErr error
	// Remove resolver watch first to stop receiving updates
	if c.resolver != nil {
		if err := c.resolver.DelWatch(c.appName, c); err != nil {
			multiErr = errors.Join(multiErr, err)
		}
	}
	// Close balancer (no longer manages connections)
	if err := c.balancer.Close(); err != nil {
		multiErr = errors.Join(multiErr, err)
	}
	// Cancel context to stop watchUpdateState goroutine
	c.cancel()
	// Close stateChange channel
	close(c.stateChange)
	// Close all connections through centralized manager
	if err := c.remoteClientManager.Close(); err != nil {
		multiErr = errors.Join(multiErr, err)
	}
	return multiErr
}

// UpdateState implements resolver.ClientConn interface
func (c *client) UpdateState(state resolver.State) {
	// Check if client is closing
	select {
	case <-c.ctx.Done():
		return
	default:
	}

	// Try to send the state, if channel is full, drain it first then send
	select {
	case c.stateChange <- state:
		return
	default:
		// Channel is full, try to drain the old value
		select {
		case <-c.stateChange:
		default:
		}
		// Now try to send again
		select {
		case c.stateChange <- state:
		case <-c.ctx.Done():
			// Client is closing, ignore the update
		}
	}
}

func (c *client) watchUpdateState() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case state, ok := <-c.stateChange:
			if !ok {
				return
			}
			c.updateState(state)
		}
	}
}

func (c *client) updateState(state resolver.State) {
	defer c.resolvedEvent.Fire()
	c.balancer.UpdateState(state)
}

func (c *client) updateStaticState(cfg config.Values) error {
	var endpoints []resolver.BaseEndpoint
	if err := cfg.Get(config.Join("remote", "endpoints")).Scan(&endpoints); err != nil {
		return err
	}
	if len(endpoints) == 0 {
		return errors.New("no endpoints provided")
	}
	attrs := cfg.Get(config.Join("remote", "attributes")).Map(map[string]any{})
	state := resolver.BaseState{
		Attributes: attrs,
		Endpoints:  make([]resolver.Endpoint, 0, len(endpoints)),
	}
	for _, endpoint := range endpoints {
		state.Endpoints = append(state.Endpoints, endpoint)
	}
	c.updateState(state)
	return nil
}

func (c *client) initResolverAndBalancer(cfg config.Values) error {
	balancerName := cfg.Get("balancer").String("default")
	b, err := balancer.New(
		c.appName,
		balancerName,
		&balancerClient{cli: c, serializer: xsync.NewCallbackSerializer(c.ctx)},
	)
	if err != nil {
		return err
	}
	c.balancer = b
	resolverName := cfg.Get("resolver").String("")
	if resolverName != "" {
		r, err := resolver.Get(resolverName)
		if err != nil {
			return err
		}
		// r can be nil for "default" with no config (use static endpoints)
		if r != nil {
			c.resolver = r
		}
		// If r is nil (no dynamic resolver), will use updateStaticState() below
	}
	return nil
}

func (c *client) newStream(
	ctx context.Context,
	desc *stream.Desc,
	method string,
) (stream.ClientStream, error) {
	if err := c.waitForResolved(ctx); err != nil {
		return nil, err
	}
	pickInfo := &balancer.RPCInfo{
		Ctx:    ctx,
		Method: method,
	}
	retries := 0
	for {
		r, err := c.pick(c.fastFail, pickInfo)
		if err != nil {
			if errors.Is(err, balancer.ErrNoAvailableInstance) {
				// Add backoff and context check for ErrNoAvailableInstance
				t := time.NewTimer(c.streamBackoff.Backoff(retries))
				select {
				case <-c.ctx.Done():
					t.Stop()
					return nil, ErrClientClosing
				case <-ctx.Done():
					t.Stop()
					return nil, xerror.New(
						code.Code_DEADLINE_EXCEEDED,
						"context done while waiting for available instance",
					)
				case <-t.C:
					retries++
					continue
				}
			}
			return nil, err
		}

		st, err := r.RemoteClient().NewStream(ctx, desc, method)
		if err == nil {
			return &clientStream{
				desc:         desc,
				ClientStream: st,
				report:       r.Report,
			}, nil
		}
		r.Report(err)
		t := time.NewTimer(c.streamBackoff.Backoff(retries))
		select {
		case <-c.ctx.Done():
			t.Stop()
			return nil, ErrClientClosing
		case <-ctx.Done():
			t.Stop()
			return nil, err
		case <-t.C:
			retries++
		}
	}
}

func (c *client) invoke(ctx context.Context, method string, args, reply interface{}) error {
	cs, err := c.newStream(
		ctx,
		&stream.Desc{ServerStreams: false, ClientStreams: false},
		method,
	)
	if err != nil {
		return err
	}
	if err = cs.SendMsg(args); err != nil {
		return err
	}
	err = cs.RecvMsg(reply)
	return err
}

func (c *client) initInterceptor() {
	serviceNameKey := fmt.Sprintf("{%s}", c.appName)

	unaryNames := append(
		loadInterceptorNames(config.Join(config.KeyBase, "client", "interceptor", "unary")),
		loadInterceptorNames(
			config.Join(config.KeyBase, "client", serviceNameKey, "interceptor", "unary"),
		)...,
	)
	unaryNames = xarray.DelDupStable(
		slices.DeleteFunc(unaryNames, func(s string) bool { return s == "" }),
	)
	c.unaryInterceptor = interceptor.ChainUnaryClientInterceptors(c.appName, unaryNames)

	steamNames := append(
		loadInterceptorNames(config.Join(config.KeyBase, "client", "interceptor", "stream")),
		loadInterceptorNames(
			config.Join(config.KeyBase, "client", serviceNameKey, "interceptor", "stream"),
		)...,
	)
	steamNames = xarray.DelDupStable(
		slices.DeleteFunc(steamNames, func(s string) bool { return s == "" }),
	)
	c.streamInterceptor = interceptor.ChainStreamClientInterceptors(c.appName, steamNames)
}

func loadInterceptorNames(key string) []string {
	val := config.Get(key)
	names := val.StringSlice()

	var raw any
	_ = val.Scan(&raw)
	switch v := raw.(type) {
	case nil, []string, []interface{}:
		return names
	case string:
		if len(v) > 0 {
			slog.Warn("interceptor config value is string, fallback parser is applied", slog.String("key", key))
		}
		return names
	default:
		slog.Warn(
			"interceptor config value is not list-like, fallback to empty list",
			slog.String("key", key),
			slog.String("type", fmt.Sprintf("%T", v)),
		)
		return names
	}
}

// waitForResolved blocks until the resolver provides addresses or the
// context expires, whichever happens first.
func (c *client) waitForResolved(ctx context.Context) error {
	// This is on the RPC path, so we use a fast path to avoid the
	// more-expensive "select" below after the resolver has returned once.
	if c.resolvedEvent.HasFired() {
		return nil
	}
	select {
	case <-c.resolvedEvent.Done():
		return nil
	case <-ctx.Done():
		switch {
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			return xerror.Wrap(ctx.Err(), code.Code_DEADLINE_EXCEEDED, "")
		case errors.Is(ctx.Err(), context.Canceled):
			return xerror.Wrap(ctx.Err(), code.Code_CANCELLED, "")
		default:
			return xerror.Wrap(ctx.Err(), code.Code_UNKNOWN, "")
		}
	case <-c.ctx.Done():
		return ErrClientClosing
	}
}
