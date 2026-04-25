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

package encoding

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gmem "google.golang.org/grpc/mem"

	// Trigger gzip compressor registration
	_ "github.com/codesjoy/yggdrasil/v3/transport/protocol/grpc/encoding/gzip"
)

func TestRegisterAndGetCodec(t *testing.T) {
	RegisterCodec(&testCodec{name: "test-codec-unit"})
	c := GetCodec("test-codec-unit")
	require.NotNil(t, c)
	assert.Equal(t, "test-codec-unit", c.Name())
}

func TestGetCodec_CaseInsensitive(t *testing.T) {
	RegisterCodec(&testCodec{name: "Test-Upper"})
	c := GetCodec("test-upper")
	require.NotNil(t, c)
	assert.Equal(t, "Test-Upper", c.Name())
}

func TestGetCodec_NotFound(t *testing.T) {
	assert.Nil(t, GetCodec("nonexistent"))
}

func TestRegisterCodec_NilPanics(t *testing.T) {
	assert.Panics(t, func() { RegisterCodec(nil) })
}

func TestRegisterCodec_EmptyNamePanics(t *testing.T) {
	assert.Panics(t, func() { RegisterCodec(&testCodec{name: ""}) })
}

func TestCodecBridge_MarshalUnmarshal(t *testing.T) {
	bridge := grpcCodecBridge{codec: &testCodec{name: "bridge-test"}}
	data, err := bridge.Marshal([]byte("hello"))
	require.NoError(t, err)
	assert.NotNil(t, data)

	var got []byte
	err = bridge.Unmarshal(data, &got)
	require.NoError(t, err)
}

func TestCodecBridge_MarshalEmpty(t *testing.T) {
	bridge := grpcCodecBridge{codec: &testCodec{name: "empty-test"}}
	data, err := bridge.Marshal([]byte{})
	require.NoError(t, err)
	assert.Nil(t, data)
}

func TestCodecBridgeV1_MarshalUnmarshal(t *testing.T) {
	bridge := grpcCodecV1Bridge{codec: grpcCodecBridge{codec: &testCodec{name: "v1-bridge"}}}
	data, err := bridge.Marshal([]byte("test"))
	require.NoError(t, err)
	assert.Equal(t, []byte("test"), data)

	var got []byte
	err = bridge.Unmarshal([]byte("test"), &got)
	require.NoError(t, err)
}

func TestCodecBridgeV1_MarshalEmpty(t *testing.T) {
	bridge := grpcCodecV1Bridge{codec: grpcCodecBridge{codec: &testCodec{name: "v1-empty"}}}
	data, err := bridge.Marshal([]byte{})
	require.NoError(t, err)
	assert.Nil(t, data)
}

// testCodec implements Codec for testing
type testCodec struct {
	name string
}

func (c *testCodec) Marshal(v interface{}) ([]byte, error) {
	b, ok := v.([]byte)
	if !ok {
		return nil, nil
	}
	return b, nil
}

func (c *testCodec) Unmarshal(data []byte, v interface{}) error {
	b, ok := v.(*[]byte)
	if !ok {
		return nil
	}
	*b = data
	return nil
}

func (c *testCodec) Name() string { return c.name }

// Ensure testCodec implements codecV2 via embedding
func (c *testCodec) MarshalV2(v interface{}) (gmem.BufferSlice, error) {
	b, ok := v.([]byte)
	if !ok {
		return nil, nil
	}
	if len(b) == 0 {
		return nil, nil
	}
	data := b
	return gmem.BufferSlice{gmem.NewBuffer(&data, nil)}, nil
}

func (c *testCodec) UnmarshalV2(data gmem.BufferSlice, v interface{}) error {
	b, ok := v.(*[]byte)
	if !ok {
		return nil
	}
	*b = data.Materialize()
	return nil
}

// v1OnlyCodec implements only the Codec interface (not codecV2),
// used to test grpcCodecBridge v1 fallback paths.
type v1OnlyCodec struct {
	name string
}

func (c *v1OnlyCodec) Marshal(v interface{}) ([]byte, error) {
	b, ok := v.([]byte)
	if !ok {
		return b, nil
	}
	return b, nil
}

func (c *v1OnlyCodec) Unmarshal(data []byte, v interface{}) error {
	b, ok := v.(*[]byte)
	if !ok {
		return nil
	}
	*b = data
	return nil
}

func (c *v1OnlyCodec) Name() string { return c.name }

// mockCompressor implements Compressor for testing RegisterCompressor/GetCompressor.
type mockCompressor struct{}

func (mockCompressor) Compress(w io.Writer) (io.WriteCloser, error) { return nil, nil }
func (mockCompressor) Decompress(r io.Reader) (io.Reader, error)    { return r, nil }
func (mockCompressor) Name() string                                 { return "mock-comp" }

func TestGetCompressor_Found(t *testing.T) {
	// gzip is registered via the blank import
	c := GetCompressor("gzip")
	assert.NotNil(t, c)
}

func TestGetCompressor_NotFound(t *testing.T) {
	c := GetCompressor("nonexistent-compressor")
	assert.Nil(t, c)
}

func TestRegisterCompressor_GetCompressor_RoundTrip(t *testing.T) {
	RegisterCompressor(mockCompressor{})
	c := GetCompressor("mock-comp")
	require.NotNil(t, c)
	assert.Equal(t, "mock-comp", c.Name())
}

func TestAsGRPCCodecV2_Nil(t *testing.T) {
	result := asGRPCCodecV2(nil)
	assert.Nil(t, result)
}

func TestCodecBridgeV1Only_Marshal(t *testing.T) {
	bridge := grpcCodecBridge{codec: &v1OnlyCodec{name: "v1-only"}}
	data, err := bridge.Marshal([]byte("hello"))
	require.NoError(t, err)
	require.Len(t, data, 1)
	assert.Equal(t, "hello", string(data[0].ReadOnlyData()))
}

func TestCodecBridgeV1Only_MarshalEmpty(t *testing.T) {
	bridge := grpcCodecBridge{codec: &v1OnlyCodec{name: "v1-empty"}}
	data, err := bridge.Marshal([]byte{})
	require.NoError(t, err)
	assert.Nil(t, data)
}

func TestCodecBridgeV1Only_Unmarshal_SingleBuffer(t *testing.T) {
	bridge := grpcCodecBridge{codec: &v1OnlyCodec{name: "v1-unmarshal"}}
	buf := []byte("data")
	bs := gmem.BufferSlice{gmem.NewBuffer(&buf, nil)}

	var got []byte
	err := bridge.Unmarshal(bs, &got)
	require.NoError(t, err)
	assert.Equal(t, "data", string(got))
}

func TestCodecBridgeV1Only_Unmarshal_EmptyBuffer(t *testing.T) {
	bridge := grpcCodecBridge{codec: &v1OnlyCodec{name: "v1-unmarshal-empty"}}
	bs := gmem.BufferSlice{}

	var got []byte
	err := bridge.Unmarshal(bs, &got)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestCodecBridgeV1Only_Unmarshal_MultiBuffer(t *testing.T) {
	bridge := grpcCodecBridge{codec: &v1OnlyCodec{name: "v1-unmarshal-multi"}}
	b1 := []byte("hel")
	b2 := []byte("lo")
	bs := gmem.BufferSlice{
		gmem.NewBuffer(&b1, nil),
		gmem.NewBuffer(&b2, nil),
	}

	var got []byte
	err := bridge.Unmarshal(bs, &got)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(got))
}
