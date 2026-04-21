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
	"io"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v2/metadata"
	"github.com/codesjoy/yggdrasil/v2/stream"
)

func TestClientStreamReportAndRecvPaths(t *testing.T) {
	t.Run("unary stream copies metadata and reports only once", func(t *testing.T) {
		ctx := metadata.WithStreamContext(context.Background())
		st := newMockClientStream(ctx)
		st.header = metadata.Pairs("h-key", "h-val")
		st.trailer = metadata.Pairs("t-key", "t-val")

		var reportCalls int32
		var reportedErr error
		cs := &clientStream{
			desc:         &stream.Desc{ServerStreams: false},
			ClientStream: st,
			report: func(err error) {
				atomic.AddInt32(&reportCalls, 1)
				reportedErr = err
			},
		}

		require.NoError(t, cs.RecvMsg(new(any)))
		require.NoError(t, cs.RecvMsg(new(any)))

		st.SetSendErr(errors.New("send failed"))
		require.Error(t, cs.SendMsg("x"))

		require.Equal(t, int32(1), atomic.LoadInt32(&reportCalls))
		require.NoError(t, reportedErr)

		header, ok := metadata.FromHeaderCtx(ctx)
		require.True(t, ok)
		require.Equal(t, "h-val", header.Get("h-key")[0])
		trailer, ok := metadata.FromTrailerCtx(ctx)
		require.True(t, ok)
		require.Equal(t, "t-val", trailer.Get("t-key")[0])
	})

	t.Run("server stream EOF reports success", func(t *testing.T) {
		st := newMockClientStream(context.Background())
		st.SetRecvErr(io.EOF)

		var calls int32
		var gotErr error
		cs := &clientStream{
			desc:         &stream.Desc{ServerStreams: true},
			ClientStream: st,
			report: func(err error) {
				atomic.AddInt32(&calls, 1)
				gotErr = err
			},
		}

		err := cs.RecvMsg(new(any))
		require.ErrorIs(t, err, io.EOF)
		require.Equal(t, int32(1), atomic.LoadInt32(&calls))
		require.NoError(t, gotErr)
	})

	t.Run("non EOF recv reports error", func(t *testing.T) {
		want := errors.New("recv failed")
		st := newMockClientStream(context.Background())
		st.SetRecvErr(want)

		var calls int32
		var gotErr error
		cs := &clientStream{
			desc:         &stream.Desc{ServerStreams: true},
			ClientStream: st,
			report: func(err error) {
				atomic.AddInt32(&calls, 1)
				gotErr = err
			},
		}

		err := cs.RecvMsg(new(any))
		require.ErrorIs(t, err, want)
		require.Equal(t, int32(1), atomic.LoadInt32(&calls))
		require.ErrorIs(t, gotErr, want)
	})
}
