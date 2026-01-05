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

package stats

import (
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/v2/metadata"
	"github.com/stretchr/testify/assert"
)

// TestRPCTagInfoBase tests RPCTagInfoBase methods
func TestRPCTagInfoBase(t *testing.T) {
	t.Run("get full method", func(t *testing.T) {
		info := &RPCTagInfoBase{
			FullMethod: "/package.service/method",
		}

		assert.Equal(t, "/package.service/method", info.GetFullMethod())
	})

	t.Run("implements RPCTagInfo interface", func(t *testing.T) {
		var info RPCTagInfo = &RPCTagInfoBase{
			FullMethod: "/test.service/rpc",
		}

		assert.Equal(t, "/test.service/rpc", info.GetFullMethod())
	})
}

// TestRPCBeginBase tests RPCBeginBase methods
func TestRPCBeginBase(t *testing.T) {
	t.Run("client RPC begin", func(t *testing.T) {
		begin := &RPCBeginBase{
			Client:       true,
			BeginTime:    time.Now(),
			ClientStream: false,
			ServerStream: false,
			Protocol:     "grpc",
		}

		assert.True(t, begin.IsClient())
		assert.False(t, begin.IsClientStream())
		assert.False(t, begin.IsServerStream())
		assert.Equal(t, "grpc", begin.GetProtocol())
		assert.NotZero(t, begin.GetBeginTime())
		assert.Implements(t, (*RPCStats)(nil), begin)
		assert.Implements(t, (*RPCBegin)(nil), begin)
	})

	t.Run("server streaming RPC begin", func(t *testing.T) {
		begin := &RPCBeginBase{
			Client:       false,
			BeginTime:    time.Now(),
			ClientStream: false,
			ServerStream: true,
			Protocol:     "grpc",
		}

		assert.False(t, begin.IsClient())
		assert.False(t, begin.IsClientStream())
		assert.True(t, begin.IsServerStream())
		assert.Equal(t, "grpc", begin.GetProtocol())
	})

	t.Run("bidirectional streaming RPC begin", func(t *testing.T) {
		begin := &RPCBeginBase{
			Client:       true,
			BeginTime:    time.Now(),
			ClientStream: true,
			ServerStream: true,
			Protocol:     "http",
		}

		assert.True(t, begin.IsClient())
		assert.True(t, begin.IsClientStream())
		assert.True(t, begin.IsServerStream())
		assert.Equal(t, "http", begin.GetProtocol())
	})
}

// TestRPCInPayloadBase tests RPCInPayloadBase methods
func TestRPCInPayloadBase(t *testing.T) {
	t.Run("create and get fields", func(t *testing.T) {
		payload := &RPCInPayloadBase{
			Client:        true,
			Payload:       "test payload",
			Data:          []byte("serialized data"),
			TransportSize: 100,
			RecvTime:      time.Now(),
			Protocol:      "grpc",
		}

		assert.True(t, payload.IsClient())
		assert.Equal(t, "test payload", payload.GetPayload())
		assert.Equal(t, []byte("serialized data"), payload.GetData())
		assert.Equal(t, 100, payload.GetTransportSize())
		assert.NotZero(t, payload.GetRecvTime())
		assert.Equal(t, "grpc", payload.GetProtocol())
		assert.Implements(t, (*RPCStats)(nil), payload)
		assert.Implements(t, (*RPCInPayload)(nil), payload)
	})
}

// TestRPCInHeaderBase tests RPCInHeaderBase methods
func TestRPCInHeaderBase(t *testing.T) {
	t.Run("client inbound header", func(t *testing.T) {
		header := &RPCClientInHeaderBase{
			RPCInHeaderBase: RPCInHeaderBase{
				Header:        metadata.MD{"key": []string{"value"}},
				Protocol:      "grpc",
				TransportSize: 50,
			},
		}

		assert.Equal(t, metadata.MD{"key": []string{"value"}}, header.GetHeader())
		assert.Equal(t, "grpc", header.GetProtocol())
		assert.Equal(t, 50, header.GetTransportSize())
		assert.Implements(t, (*RPCStats)(nil), header)
		assert.Implements(t, (*RPCInHeader)(nil), header)
		assert.Implements(t, (*RPCClientInHeader)(nil), header)
	})

	t.Run("server inbound header", func(t *testing.T) {
		header := &RPCServerInHeaderBase{
			RPCInHeaderBase: RPCInHeaderBase{
				Header:        metadata.MD{"auth": []string{"token"}},
				Protocol:      "grpc",
				TransportSize: 60,
			},
			FullMethod:     "/service/method",
			RemoteEndpoint: "127.0.0.1:8080",
			LocalEndpoint:  "127.0.0.1:3000",
		}

		assert.Equal(t, metadata.MD{"auth": []string{"token"}}, header.GetHeader())
		assert.Equal(t, "grpc", header.GetProtocol())
		assert.Equal(t, 60, header.GetTransportSize())
		assert.Equal(t, "/service/method", header.GetFullMethod())
		assert.Equal(t, "127.0.0.1:8080", header.GetRemoteEndpoint())
		assert.Equal(t, "127.0.0.1:3000", header.GetLocalEndpoint())
		assert.Implements(t, (*RPCServerInHeader)(nil), header)
	})
}

// TestRPCInTrailerBase tests RPCInTrailerBase methods
func TestRPCInTrailerBase(t *testing.T) {
	t.Run("create and get fields", func(t *testing.T) {
		trailer := &RPCInTrailerBase{
			Client:        true,
			Trailer:       metadata.MD{"trailer-key": []string{"trailer-value"}},
			TransportSize: 30,
			Protocol:      "grpc",
		}

		assert.True(t, trailer.IsClient())
		assert.Equal(t, metadata.MD{"trailer-key": []string{"trailer-value"}}, trailer.GetTrailer())
		assert.Equal(t, 30, trailer.GetTransportSize())
		assert.Equal(t, "grpc", trailer.GetProtocol())
		assert.Implements(t, (*RPCStats)(nil), trailer)
		assert.Implements(t, (*RPCInTrailer)(nil), trailer)
	})
}

// TestRPCOutPayloadBase tests RPCOutPayloadBase methods
func TestRPCOutPayloadBase(t *testing.T) {
	t.Run("create and get fields", func(t *testing.T) {
		payload := &RPCOutPayloadBase{
			Client:        false,
			Payload:       "response",
			Data:          []byte("response data"),
			TransportSize: 200,
			SendTime:      time.Now(),
			Protocol:      "grpc",
		}

		assert.False(t, payload.IsClient())
		assert.Equal(t, "response", payload.GetPayload())
		assert.Equal(t, []byte("response data"), payload.GetData())
		assert.Equal(t, 200, payload.GetTransportSize())
		assert.NotZero(t, payload.GetSendTime())
		assert.Equal(t, "grpc", payload.GetProtocol())
		assert.Implements(t, (*RPCStats)(nil), payload)
		assert.Implements(t, (*RPCOutPayload)(nil), payload)
	})
}

// TestOutHeaderBase tests OutHeaderBase methods
func TestOutHeaderBase(t *testing.T) {
	t.Run("client outbound header", func(t *testing.T) {
		header := &OutHeaderBase{
			Client:         true,
			Header:         metadata.MD{"out-key": []string{"out-value"}},
			TransportSize:  70,
			FullMethod:     "/service/clientMethod",
			RemoteEndpoint: "server:8080",
			LocalEndpoint:  "client:3000",
			Protocol:       "grpc",
		}

		assert.True(t, header.IsClient())
		assert.Equal(t, metadata.MD{"out-key": []string{"out-value"}}, header.GetHeader())
		assert.Equal(t, 70, header.GetTransportSize())
		assert.Equal(t, "/service/clientMethod", header.GetFullMethod())
		assert.Equal(t, "server:8080", header.GetRemoteEndpoint())
		assert.Equal(t, "client:3000", header.GetLocalEndpoint())
		assert.Equal(t, "grpc", header.GetProtocol())
		assert.Implements(t, (*RPCStats)(nil), header)
		assert.Implements(t, (*RPCOutHeader)(nil), header)
	})
}

// TestOutTrailerBase tests OutTrailerBase methods
func TestOutTrailerBase(t *testing.T) {
	t.Run("server outbound trailer", func(t *testing.T) {
		trailer := &OutTrailerBase{
			Client:        false,
			Trailer:       metadata.MD{"server-trailer": []string{"value"}},
			TransportSize: 40,
		}

		assert.False(t, trailer.IsClient())
		assert.Equal(t, metadata.MD{"server-trailer": []string{"value"}}, trailer.GetTrailer())
		assert.Equal(t, 40, trailer.GetTransportSize())
		assert.Implements(t, (*RPCStats)(nil), trailer)
		assert.Implements(t, (*RPCOutTrailer)(nil), trailer)
	})
}

// TestRPCEndBase tests RPCEndBase methods
func TestRPCEndBase(t *testing.T) {
	t.Run("successful RPC end", func(t *testing.T) {
		beginTime := time.Now()
		endTime := beginTime.Add(100 * time.Millisecond)

		end := &RPCEndBase{
			Client:    true,
			BeginTime: beginTime,
			EndTime:   endTime,
			Err:       nil,
			Protocol:  "grpc",
		}

		assert.True(t, end.IsClient())
		assert.Equal(t, beginTime, end.GetBeginTime())
		assert.Equal(t, endTime, end.GetEndTime())
		assert.NoError(t, end.Error())
		assert.Equal(t, "grpc", end.GetProtocol())
		assert.Implements(t, (*RPCStats)(nil), end)
		assert.Implements(t, (*RPCEnd)(nil), end)
	})

	t.Run("failed RPC end", func(t *testing.T) {
		beginTime := time.Now()
		endTime := beginTime.Add(50 * time.Millisecond)
		err := assert.AnError

		end := &RPCEndBase{
			Client:    false,
			BeginTime: beginTime,
			EndTime:   endTime,
			Err:       err,
			Protocol:  "http",
		}

		assert.False(t, end.IsClient())
		assert.Equal(t, beginTime, end.GetBeginTime())
		assert.Equal(t, endTime, end.GetEndTime())
		assert.Equal(t, err, end.Error())
		assert.Equal(t, "http", end.GetProtocol())
	})
}

// TestRPCStatsInterface tests RPCStats interface implementations
func TestRPCStatsInterface(t *testing.T) {
	t.Run("all types implement RPCStats", func(t *testing.T) {
		stats := []RPCStats{
			&RPCBeginBase{},
			&RPCInPayloadBase{},
			&RPCInHeaderBase{},
			&RPCInTrailerBase{},
			&RPCOutPayloadBase{},
			&OutHeaderBase{},
			&OutTrailerBase{},
			&RPCEndBase{},
		}

		for i, stat := range stats {
			assert.Implements(t, (*RPCStats)(nil), stat, "index %d should implement RPCStats", i)
		}
	})
}
