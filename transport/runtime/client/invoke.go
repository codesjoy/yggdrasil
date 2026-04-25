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
	"time"

	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
	"github.com/codesjoy/yggdrasil/v3/rpc/stream"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/client/balancer"
)

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
