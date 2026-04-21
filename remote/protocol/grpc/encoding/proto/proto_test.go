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

package proto

import (
	"bytes"
	"sync"
	"testing"

	"github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc/encoding"
	"github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc/encoding/proto/codec_perf"
	"github.com/stretchr/testify/require"
)

func marshalAndUnmarshal(t *testing.T, codec encoding.Codec, expectedBody []byte) {
	t.Helper()

	p := &codec_perf.Buffer{}
	p.Body = expectedBody

	marshalledBytes, err := codec.Marshal(p)
	require.NoError(t, err)

	require.NoError(t, codec.Unmarshal(marshalledBytes, p))
	require.True(t, bytes.Equal(p.GetBody(), expectedBody))
}

func TestBasicProtoCodecMarshalAndUnmarshal(t *testing.T) {
	marshalAndUnmarshal(t, mustGetCodec(t), []byte{1, 2, 3})
}

// Try to catch possible race conditions around use of pools
func TestConcurrentUsage(t *testing.T) {
	const (
		numGoRoutines   = 100
		numMarshUnmarsh = 1000
	)

	// small, arbitrary byte slices
	protoBodies := [][]byte{
		[]byte("one"),
		[]byte("two"),
		[]byte("three"),
		[]byte("four"),
		[]byte("five"),
	}

	var wg sync.WaitGroup
	codec := mustGetCodec(t)

	for i := 0; i < numGoRoutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for k := 0; k < numMarshUnmarsh; k++ {
				marshalAndUnmarshal(t, codec, protoBodies[k%len(protoBodies)])
			}
		}()
	}

	wg.Wait()
}

// TestStaggeredMarshalAndUnmarshalUsingSamePool tries to catch potential errors in which slices get
// stomped on during reuse of a proto.Buffer.
func TestStaggeredMarshalAndUnmarshalUsingSamePool(t *testing.T) {
	codec1 := mustGetCodec(t)
	codec2 := mustGetCodec(t)

	expectedBody1 := []byte{1, 2, 3}
	expectedBody2 := []byte{4, 5, 6}

	proto1 := codec_perf.Buffer{Body: expectedBody1}
	proto2 := codec_perf.Buffer{Body: expectedBody2}

	var m1, m2 []byte
	var err error

	m1, err = codec1.Marshal(&proto1)
	require.NoError(t, err)
	m2, err = codec2.Marshal(&proto2)
	require.NoError(t, err)
	require.NoError(t, codec1.Unmarshal(m1, &proto1))
	require.NoError(t, codec2.Unmarshal(m2, &proto2))

	b1 := proto1.GetBody()
	b2 := proto2.GetBody()

	for i, v := range b1 {
		require.Equal(t, expectedBody1[i], v, "b1 index %d", i)
	}

	for i, v := range b2 {
		require.Equal(t, expectedBody2[i], v, "b2 index %d", i)
	}
}

func mustGetCodec(t *testing.T) encoding.Codec {
	t.Helper()

	codec := encoding.GetCodec(Name)
	require.NotNil(t, codec)
	return codec
}
