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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gmem "google.golang.org/grpc/mem"

	yencoding "github.com/codesjoy/yggdrasil/v3/transport/protocol/grpc/encoding"
)

// ---------------------------------------------------------------------------
// Mock codecs for testing
// ---------------------------------------------------------------------------

type mockV1Codec struct {
	name          string
	marshalFunc   func(v interface{}) ([]byte, error)
	unmarshalFunc func(data []byte, v interface{}) error
}

func (c *mockV1Codec) Name() string { return c.name }

func (c *mockV1Codec) Marshal(v interface{}) ([]byte, error) {
	if c.marshalFunc != nil {
		return c.marshalFunc(v)
	}
	return []byte("mock"), nil
}

func (c *mockV1Codec) Unmarshal(data []byte, v interface{}) error {
	if c.unmarshalFunc != nil {
		return c.unmarshalFunc(data, v)
	}
	return nil
}

// mockLocalCodecV2 implements both yencoding.Codec and the localCodecV2 interface.
type mockLocalCodecV2 struct {
	mockV1Codec
	marshalV2Func   func(v interface{}) (gmem.BufferSlice, error)
	unmarshalV2Func func(data gmem.BufferSlice, v interface{}) error
}

func (c *mockLocalCodecV2) MarshalV2(v interface{}) (gmem.BufferSlice, error) {
	if c.marshalV2Func != nil {
		return c.marshalV2Func(v)
	}
	data := []byte("mockV2")
	return gmem.BufferSlice{gmem.NewBuffer(&data, nil)}, nil
}

func (c *mockLocalCodecV2) UnmarshalV2(data gmem.BufferSlice, v interface{}) error {
	if c.unmarshalV2Func != nil {
		return c.unmarshalV2Func(data, v)
	}
	return nil
}

// ---------------------------------------------------------------------------
// officialCodecV1Bridge tests
// ---------------------------------------------------------------------------

func TestOfficialCodecV1Bridge_Name(t *testing.T) {
	codec := &mockV1Codec{name: "test-v1"}
	bridge := officialCodecV1Bridge{codec: codec}
	assert.Equal(t, "test-v1", bridge.Name())
}

func TestOfficialCodecV1Bridge_Name_DelegatesToWrappedCodec(t *testing.T) {
	// Verify that the bridge delegates Name() to the wrapped codec.
	codec := &mockV1Codec{name: "my-codec"}
	bridge := officialCodecV1Bridge{codec: codec}
	assert.Equal(t, "my-codec", bridge.Name())
}

func TestOfficialCodecV1Bridge_Marshal(t *testing.T) {
	codec := &mockV1Codec{
		name: "test-marshal",
		marshalFunc: func(v interface{}) ([]byte, error) {
			return []byte("marshaled-data"), nil
		},
	}
	bridge := officialCodecV1Bridge{codec: codec}

	bs, err := bridge.Marshal("test")
	require.NoError(t, err)
	require.Len(t, bs, 1)
	assert.Equal(t, "marshaled-data", string(bs[0].ReadOnlyData()))
}

func TestOfficialCodecV1Bridge_Marshal_Error(t *testing.T) {
	expectedErr := errors.New("marshal failed")
	codec := &mockV1Codec{
		name: "err-codec",
		marshalFunc: func(v interface{}) ([]byte, error) {
			return nil, expectedErr
		},
	}
	bridge := officialCodecV1Bridge{codec: codec}

	bs, err := bridge.Marshal("anything")
	require.ErrorIs(t, err, expectedErr)
	assert.Nil(t, bs)
}

func TestOfficialCodecV1Bridge_Marshal_Empty(t *testing.T) {
	codec := &mockV1Codec{
		name: "empty-codec",
		marshalFunc: func(v interface{}) ([]byte, error) {
			return []byte{}, nil
		},
	}
	bridge := officialCodecV1Bridge{codec: codec}

	bs, err := bridge.Marshal("anything")
	require.NoError(t, err)
	assert.Nil(t, bs)
}

func TestOfficialCodecV1Bridge_Unmarshal(t *testing.T) {
	called := false
	codec := &mockV1Codec{
		name: "test-unmarshal",
		unmarshalFunc: func(data []byte, v interface{}) error {
			called = true
			assert.Equal(t, "test", string(data))
			return nil
		},
	}
	bridge := officialCodecV1Bridge{codec: codec}

	data := []byte("test")
	bs := gmem.BufferSlice{gmem.NewBuffer(&data, nil)}
	err := bridge.Unmarshal(bs, nil)
	require.NoError(t, err)
	assert.True(t, called)
}

func TestOfficialCodecV1Bridge_Unmarshal_Empty(t *testing.T) {
	called := false
	codec := &mockV1Codec{
		name: "unmarshal-empty",
		unmarshalFunc: func(data []byte, v interface{}) error {
			called = true
			assert.Nil(t, data)
			return nil
		},
	}
	bridge := officialCodecV1Bridge{codec: codec}

	err := bridge.Unmarshal(gmem.BufferSlice{}, nil)
	require.NoError(t, err)
	assert.True(t, called)
}

func TestOfficialCodecV1Bridge_Unmarshal_MultiBuffer(t *testing.T) {
	// With multiple buffers, Materialize() should be called which concatenates them.
	called := false
	codec := &mockV1Codec{
		name: "multi-buffer",
		unmarshalFunc: func(data []byte, v interface{}) error {
			called = true
			// Materialize concatenates the two buffers
			assert.Equal(t, "helloworld", string(data))
			return nil
		},
	}
	bridge := officialCodecV1Bridge{codec: codec}

	part1 := []byte("hello")
	part2 := []byte("world")
	bs := gmem.BufferSlice{
		gmem.NewBuffer(&part1, nil),
		gmem.NewBuffer(&part2, nil),
	}
	err := bridge.Unmarshal(bs, nil)
	require.NoError(t, err)
	assert.True(t, called)
}

// ---------------------------------------------------------------------------
// localCodecV2Bridge tests
// ---------------------------------------------------------------------------

func TestLocalCodecV2Bridge_Name(t *testing.T) {
	codec := &mockV1Codec{name: "test-local"}
	bridge := localCodecV2Bridge{codec: codec}
	assert.Equal(t, "test-local", bridge.Name())
}

func TestLocalCodecV2Bridge_Marshal_V1Path(t *testing.T) {
	// mockV1Codec does NOT implement localCodecV2, so the v1 path is taken.
	codec := &mockV1Codec{
		name: "v1-path",
		marshalFunc: func(v interface{}) ([]byte, error) {
			return []byte("v1data"), nil
		},
	}
	bridge := localCodecV2Bridge{codec: codec}

	bs, err := bridge.Marshal("test")
	require.NoError(t, err)
	require.Len(t, bs, 1)
	assert.Equal(t, "v1data", string(bs[0].ReadOnlyData()))
}

func TestLocalCodecV2Bridge_Marshal_V1PathEmpty(t *testing.T) {
	codec := &mockV1Codec{
		name: "v1-empty",
		marshalFunc: func(v interface{}) ([]byte, error) {
			return []byte{}, nil
		},
	}
	bridge := localCodecV2Bridge{codec: codec}

	bs, err := bridge.Marshal("test")
	require.NoError(t, err)
	assert.Nil(t, bs)
}

func TestLocalCodecV2Bridge_Marshal_V1PathError(t *testing.T) {
	expectedErr := errors.New("v1 marshal error")
	codec := &mockV1Codec{
		name: "v1-err",
		marshalFunc: func(v interface{}) ([]byte, error) {
			return nil, expectedErr
		},
	}
	bridge := localCodecV2Bridge{codec: codec}

	bs, err := bridge.Marshal("test")
	require.ErrorIs(t, err, expectedErr)
	assert.Nil(t, bs)
}

func TestLocalCodecV2Bridge_Marshal_V2Path(t *testing.T) {
	codec := &mockLocalCodecV2{
		mockV1Codec: mockV1Codec{name: "v2-path"},
	}
	bridge := localCodecV2Bridge{codec: codec}

	bs, err := bridge.Marshal("test")
	require.NoError(t, err)
	require.Len(t, bs, 1)
	assert.Equal(t, "mockV2", string(bs[0].ReadOnlyData()))
}

func TestLocalCodecV2Bridge_Unmarshal_V1Path(t *testing.T) {
	called := false
	codec := &mockV1Codec{
		name: "v1-unmarshal",
		unmarshalFunc: func(data []byte, v interface{}) error {
			called = true
			assert.Equal(t, "single", string(data))
			return nil
		},
	}
	bridge := localCodecV2Bridge{codec: codec}

	data := []byte("single")
	bs := gmem.BufferSlice{gmem.NewBuffer(&data, nil)}
	err := bridge.Unmarshal(bs, nil)
	require.NoError(t, err)
	assert.True(t, called)
}

func TestLocalCodecV2Bridge_Unmarshal_V1PathEmpty(t *testing.T) {
	called := false
	codec := &mockV1Codec{
		name: "v1-unmarshal-empty",
		unmarshalFunc: func(data []byte, v interface{}) error {
			called = true
			assert.Nil(t, data)
			return nil
		},
	}
	bridge := localCodecV2Bridge{codec: codec}

	err := bridge.Unmarshal(gmem.BufferSlice{}, nil)
	require.NoError(t, err)
	assert.True(t, called)
}

func TestLocalCodecV2Bridge_Unmarshal_V1PathMultiBuffer(t *testing.T) {
	called := false
	codec := &mockV1Codec{
		name: "v1-multi",
		unmarshalFunc: func(data []byte, v interface{}) error {
			called = true
			assert.Equal(t, "ab", string(data))
			return nil
		},
	}
	bridge := localCodecV2Bridge{codec: codec}

	part1 := []byte("a")
	part2 := []byte("b")
	bs := gmem.BufferSlice{
		gmem.NewBuffer(&part1, nil),
		gmem.NewBuffer(&part2, nil),
	}
	err := bridge.Unmarshal(bs, nil)
	require.NoError(t, err)
	assert.True(t, called)
}

func TestLocalCodecV2Bridge_Unmarshal_V2Path(t *testing.T) {
	called := false
	codec := &mockLocalCodecV2{
		mockV1Codec: mockV1Codec{name: "v2-unmarshal"},
		unmarshalV2Func: func(data gmem.BufferSlice, v interface{}) error {
			called = true
			return nil
		},
	}
	bridge := localCodecV2Bridge{codec: codec}

	data := []byte("v2data")
	bs := gmem.BufferSlice{gmem.NewBuffer(&data, nil)}
	err := bridge.Unmarshal(bs, nil)
	require.NoError(t, err)
	assert.True(t, called)
}

func TestLocalCodecV2Bridge_Unmarshal_V2PathError(t *testing.T) {
	expectedErr := errors.New("v2 unmarshal error")
	codec := &mockLocalCodecV2{
		mockV1Codec: mockV1Codec{name: "v2-unmarshal-err"},
		unmarshalV2Func: func(data gmem.BufferSlice, v interface{}) error {
			return expectedErr
		},
	}
	bridge := localCodecV2Bridge{codec: codec}

	data := []byte("v2data")
	bs := gmem.BufferSlice{gmem.NewBuffer(&data, nil)}
	err := bridge.Unmarshal(bs, nil)
	require.ErrorIs(t, err, expectedErr)
}

// ---------------------------------------------------------------------------
// grpcCodecV2BySubtype tests
// ---------------------------------------------------------------------------

func TestGRPCCodecV2BySubtype(t *testing.T) {
	t.Run("empty string returns nil", func(t *testing.T) {
		assert.Nil(t, grpcCodecV2BySubtype(""))
	})

	t.Run("proto returns non-nil", func(t *testing.T) {
		// The proto codec is registered globally by the init() in encoding/proto.
		codec := grpcCodecV2BySubtype("proto")
		require.NotNil(t, codec)
		assert.Equal(t, "proto", codec.Name())
	})

	t.Run("unknown returns nil", func(t *testing.T) {
		assert.Nil(t, grpcCodecV2BySubtype("nonexistent-codec-xyz"))
	})

	t.Run("case insensitive lookup", func(t *testing.T) {
		// "Proto" is lowercased internally, so it should still find the proto codec.
		codec := grpcCodecV2BySubtype("Proto")
		require.NotNil(t, codec)
		assert.Equal(t, "proto", codec.Name())
	})
}

// ---------------------------------------------------------------------------
// grpcCodecV2ForLocal tests
// ---------------------------------------------------------------------------

func TestGRPCCodecV2ForLocal_Nil(t *testing.T) {
	assert.Nil(t, grpcCodecV2ForLocal(nil))
}

func TestGRPCCodecV2ForLocal_RegisteredName(t *testing.T) {
	// Use a codec whose name matches a globally-registered codec.
	codec := &mockV1Codec{name: "proto"}
	result := grpcCodecV2ForLocal(codec)
	require.NotNil(t, result)
	assert.Equal(t, "proto", result.Name())
}

func TestGRPCCodecV2ForLocal_Unregistered(t *testing.T) {
	// A codec with a name that is not registered globally falls back to localCodecV2Bridge.
	codec := &mockV1Codec{name: "my-custom-unregistered-codec"}
	result := grpcCodecV2ForLocal(codec)
	require.NotNil(t, result)
	assert.Equal(t, "my-custom-unregistered-codec", result.Name())
	// Verify it's the bridge type by checking it works correctly.
	_, ok := result.(localCodecV2Bridge)
	assert.True(t, ok, "expected localCodecV2Bridge for unregistered codec")
}

func TestGRPCCodecV2ForLocal_EmptyName(t *testing.T) {
	// Empty name should skip the global lookup and return a localCodecV2Bridge.
	codec := &mockV1Codec{name: ""}
	result := grpcCodecV2ForLocal(codec)
	require.NotNil(t, result)
	_, ok := result.(localCodecV2Bridge)
	assert.True(t, ok, "expected localCodecV2Bridge for codec with empty name")
}

// Verify that the mock types satisfy the expected interfaces at compile time.
var (
	_ yencoding.Codec = (*mockV1Codec)(nil)
	_ yencoding.Codec = (*mockLocalCodecV2)(nil)
)
