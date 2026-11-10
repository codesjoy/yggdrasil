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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/interop/grpc_testing"
	gstatus "google.golang.org/grpc/status"

	ymetadata "github.com/codesjoy/yggdrasil/v3/rpc/metadata"
	ystatus "github.com/codesjoy/yggdrasil/v3/rpc/status"
	"github.com/codesjoy/yggdrasil/v3/rpc/stream"
)

func TestInteropCancellation(t *testing.T) {
	t.Run("grpc-go client to yggdrasil server", func(t *testing.T) {
		healthDisabled := false
		srv := startYGGServer(t, Settings{
			Server: ServerConfig{
				Network:       "tcp",
				Address:       "127.0.0.1:0",
				HealthEnabled: &healthDisabled,
			},
		}, handleYGGInterop, nil)
		conn := dialGRPCGo(t, srv.Info().Address)

		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan error, 1)
		go func() {
			result <- conn.Invoke(
				ctx,
				interopSlowMethod,
				&grpc_testing.Empty{},
				&grpc_testing.Empty{},
			)
		}()
		time.AfterFunc(20*time.Millisecond, cancel)

		select {
		case err := <-result:
			assert.Equal(t, codes.Canceled, gstatus.Code(err))
		case <-time.After(2 * time.Second):
			t.Fatal("canceled grpc-go call did not return")
		}
	})

	t.Run("yggdrasil client to grpc-go server", func(t *testing.T) {
		address := startGRPCGoServer(t, grpcGoInteropService{})
		client := newYGGClient(t, address, "", nil)

		ctx, cancel := context.WithCancel(ymetadata.WithStreamContext(context.Background()))
		call, err := client.NewStream(
			ctx,
			&stream.Desc{},
			grpc_testing.TestService_UnaryCall_FullMethodName,
		)
		require.NoError(t, err)
		require.NoError(t, call.SendMsg(&grpc_testing.SimpleRequest{FillServerId: true}))

		result := make(chan error, 1)
		go func() {
			result <- call.RecvMsg(new(grpc_testing.SimpleResponse))
		}()
		time.AfterFunc(20*time.Millisecond, cancel)

		select {
		case err := <-result:
			require.Error(t, err)
			status, ok := ystatus.CoverError(err)
			require.True(t, ok)
			assert.Equal(t, code.Code_CANCELLED, status.Code())
		case <-time.After(2 * time.Second):
			t.Fatal("canceled yggdrasil call did not return")
		}
	})
}
