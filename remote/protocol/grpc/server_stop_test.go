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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v2/metadata"
	"github.com/codesjoy/yggdrasil/v2/remote/peer"
	transport2 "github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc/transport"
	"github.com/codesjoy/yggdrasil/v2/status"
)

type blockingServerTransport struct {
	drained bool
}

func (b *blockingServerTransport) HandleStreams(context.Context, func(*transport2.Stream)) {}

func (b *blockingServerTransport) WriteHeader(*transport2.Stream, metadata.MD) error {
	return nil
}

func (b *blockingServerTransport) Write(*transport2.Stream, []byte, []byte, *transport2.Options) error {
	return nil
}

func (b *blockingServerTransport) WriteStatus(*transport2.Stream, *status.Status) error {
	return nil
}

func (b *blockingServerTransport) Close() {}

func (b *blockingServerTransport) Peer() *peer.Peer {
	return &peer.Peer{}
}

func (b *blockingServerTransport) Drain() {
	b.drained = true
}

func TestServerStopRespectsContextDeadline(t *testing.T) {
	s := &server{
		serve:     true,
		stoppedCh: make(chan struct{}),
		conns:     make(map[transport2.ServerTransport]bool),
	}
	s.cv = sync.NewCond(&s.mu)
	s.ctx, s.cancel = context.WithCancel(context.Background())

	st := &blockingServerTransport{}
	s.conns[st] = true

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := s.Stop(ctx)
	elapsed := time.Since(start)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.True(t, st.drained)
	require.Less(t, elapsed, 250*time.Millisecond)

	s.removeConn(st)

	select {
	case <-s.stoppedCh:
	case <-time.After(2 * time.Second):
		t.Fatal("grpc server did not finish stopping after connection removal")
	}
}
