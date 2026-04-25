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

package client

import (
	"context"
	"errors"

	"github.com/codesjoy/pkg/basic/xerror"
	"github.com/codesjoy/pkg/utils/xsync"
	"google.golang.org/genproto/googleapis/rpc/code"

	"github.com/codesjoy/yggdrasil/v3/remote"
	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
)

// Close closes the client.
func (c *client) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return ErrClientClosing
	}
	var multiErr error
	if c.resolver != nil {
		if err := c.resolver.DelWatch(c.appName, c); err != nil {
			multiErr = errors.Join(multiErr, err)
		}
	}
	if err := c.balancer.Close(); err != nil {
		multiErr = errors.Join(multiErr, err)
	}
	c.cancel()
	if err := c.remoteClientManager.Close(); err != nil {
		multiErr = errors.Join(multiErr, err)
	}
	return multiErr
}

// UpdateState implements resolver.ClientConn interface
func (c *client) UpdateState(state resolver.State) {
	select {
	case <-c.ctx.Done():
		return
	case c.stateChange <- state:
		return
	default:
	}

	select {
	case <-c.ctx.Done():
		return
	default:
	}

	select {
	case <-c.ctx.Done():
		return
	case <-c.stateChange:
	default:
	}

	select {
	case <-c.ctx.Done():
	case c.stateChange <- state:
	default:
	}
}

func (c *client) watchUpdateState() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case state := <-c.stateChange:
			c.updateState(state)
		}
	}
}

func (c *client) updateState(state resolver.State) {
	if c.balancerClient != nil {
		c.balancerClient.syncActiveEndpoints(state)
	}
	c.balancer.UpdateState(state)
	c.resolvedEvent.Fire()
}

func (c *client) updateConnectivityState(state remote.State) {
	c.channelState.Store(int32(state))
}

func (c *client) updateStaticState(cfg ServiceSettings) error {
	if len(cfg.Remote.Endpoints) == 0 {
		return errors.New("no endpoints provided")
	}
	state := resolver.BaseState{
		Attributes: cfg.Remote.Attributes,
		Endpoints:  make([]resolver.Endpoint, 0, len(cfg.Remote.Endpoints)),
	}
	for _, endpoint := range cfg.Remote.Endpoints {
		state.Endpoints = append(state.Endpoints, endpoint)
	}
	c.updateState(state)
	return nil
}

func (c *client) initResolverAndBalancer(cfg ServiceSettings) error {
	balancerName := cfg.Balancer
	if balancerName == "" {
		balancerName = "default"
	}
	bc := &balancerClient{
		cli:          c,
		serializer:   xsync.NewSerializer(c.ctx),
		remoteStates: make(map[string]remote.State),
		activeNames:  make(map[string]struct{}),
	}
	b, err := c.runtime.NewBalancer(c.appName, balancerName, bc)
	if err != nil {
		return err
	}
	c.balancerClient = bc
	c.balancer = b
	resolverName := cfg.Resolver
	if resolverName != "" {
		r, err := c.runtime.NewResolver(resolverName)
		if err != nil {
			return err
		}
		if r != nil {
			c.resolver = r
		}
	}
	return nil
}

// waitForResolved blocks until the client has received its initial state update
// (static or resolver-driven), or the context expires.
func (c *client) waitForResolved(ctx context.Context) error {
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

func (c *client) resolveNow() {
	rn, ok := c.resolver.(resolver.ResolveNower)
	if !ok {
		return
	}
	rn.ResolveNow()
}
