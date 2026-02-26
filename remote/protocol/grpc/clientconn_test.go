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
	"net"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/v2/internal/backoff"
	"github.com/codesjoy/yggdrasil/v2/remote"
	"github.com/stretchr/testify/assert"
)

func newTestBackoffStrategy() backoff.Strategy {
	return backoff.Exponential{Config: backoff.Config{
		BaseDelay:  time.Millisecond,
		Multiplier: 1.0,
		Jitter:     0,
		MaxDelay:   time.Millisecond,
	}}
}

func TestClientConnOnCloseDoesNotReconnectAfterShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cc := &clientConn{
		ctx:  ctx,
		cfg:  &Config{},
		bs:   newTestBackoffStrategy(),
		addr: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1},
		onStateChange: func(remote.ClientState) {
		},
		state: remote.Shutdown,
	}

	cc.onClose()
	assert.Equal(t, remote.Shutdown, cc.State())
}

func TestClientConnOnCloseMovesToRecoverableState(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cc := &clientConn{
		ctx:  ctx,
		cfg:  &Config{},
		bs:   newTestBackoffStrategy(),
		addr: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1},
		onStateChange: func(remote.ClientState) {
		},
		state: remote.Ready,
	}

	cc.onClose()
	assert.Equal(t, remote.TransientFailure, cc.State())
}
