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

package marshaler

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestJSONPb(t *testing.T) {
	m, err := NewJSONPbMarshaler()
	require.NoError(t, err)
	assert.Equal(t, "application/json", m.ContentType(nil))

	t.Run("Marshal Proto", func(t *testing.T) {
		msg := wrapperspb.String("hello")
		data, err := m.Marshal(msg)
		require.NoError(t, err)
		assert.Equal(t, `"hello"`, string(data))
	})

	t.Run("Unmarshal Proto", func(t *testing.T) {
		data := []byte(`"world"`)
		msg := &wrapperspb.StringValue{}
		err := m.Unmarshal(data, msg)
		require.NoError(t, err)
		assert.Equal(t, "world", msg.Value)
	})

	t.Run("Marshal Non-Proto", func(t *testing.T) {
		v := map[string]string{"foo": "bar"}
		data, err := m.Marshal(v)
		require.NoError(t, err)
		assert.JSONEq(t, `{"foo": "bar"}`, string(data))
	})

	t.Run("Unmarshal Non-Proto", func(t *testing.T) {
		data := []byte(`{"foo": "baz"}`)
		var v map[string]string
		err := m.Unmarshal(data, &v)
		require.NoError(t, err)
		assert.Equal(t, "baz", v["foo"])
	})

	t.Run("Encoder/Decoder", func(t *testing.T) {
		var buf bytes.Buffer
		enc := m.NewEncoder(&buf)
		msg := wrapperspb.String("encode")
		err := enc.Encode(msg)
		require.NoError(t, err)

		dec := m.NewDecoder(&buf)
		var res wrapperspb.StringValue
		err = dec.Decode(&res)
		require.NoError(t, err)
		assert.Equal(t, "encode", res.Value)
	})
}

func TestProtoMarshaller(t *testing.T) {
	m := &ProtoMarshaller{}
	assert.Equal(t, "application/octet-stream", m.ContentType(nil))

	t.Run("Marshal/Unmarshal", func(t *testing.T) {
		msg := wrapperspb.String("proto")
		data, err := m.Marshal(msg)
		require.NoError(t, err)

		res := &wrapperspb.StringValue{}
		err = m.Unmarshal(data, res)
		require.NoError(t, err)
		assert.Equal(t, "proto", res.Value)
	})

	t.Run("Non-Proto Error", func(t *testing.T) {
		_, err := m.Marshal("string")
		assert.Error(t, err)

		err = m.Unmarshal([]byte{}, "string")
		assert.Error(t, err)
	})

	t.Run("Encoder/Decoder", func(t *testing.T) {
		var buf bytes.Buffer
		enc := m.NewEncoder(&buf)
		msg := wrapperspb.String("encode_proto")
		err := enc.Encode(msg)
		require.NoError(t, err)

		dec := m.NewDecoder(&buf)
		var res wrapperspb.StringValue
		err = dec.Decode(&res)
		require.NoError(t, err)
		assert.Equal(t, "encode_proto", res.Value)
	})
}

func TestRegistry(t *testing.T) {
	RegisterJSONPbBuilder()
	RegisterProtoBuilder()

	reg := BuildMarshalerRegistry("jsonpb", "proto")

	t.Run("Default", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil) // nolint:noctx
		in, out := reg.GetMarshaler(req)
		assert.IsType(t, &JSONPb{}, in)
		assert.IsType(t, &JSONPb{}, out)
	})

	t.Run("Accept Header Uses MIME", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil) // nolint:noctx
		req.Header.Set("Accept", "application/octet-stream")
		_, out := reg.GetMarshaler(req)
		assert.IsType(t, &ProtoMarshaller{}, out)
	})

	t.Run("Content-Type Header Uses MIME", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/", nil) // nolint:noctx
		req.Header.Set("Content-Type", "application/octet-stream")
		in, _ := reg.GetMarshaler(req)
		assert.IsType(t, &ProtoMarshaller{}, in)
	})

	t.Run("Alias Fallback Remains Supported", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/", nil) // nolint:noctx
		req.Header.Set("Accept", "jsonpb")
		req.Header.Set("Content-Type", "proto")
		in, out := reg.GetMarshaler(req)
		assert.IsType(t, &ProtoMarshaller{}, in)
		assert.IsType(t, &JSONPb{}, out)
	})

	t.Run("JSON Media Types Normalize", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil) // nolint:noctx
		req.Header.Set("Accept", "application/json; charset=utf-8")
		_, out := reg.GetMarshaler(req)
		assert.IsType(t, &JSONPb{}, out)
	})
}

func TestContext(t *testing.T) {
	ctx := context.Background()
	m := &ProtoMarshaller{}

	ctx = WithInboundContext(ctx, m)
	assert.Equal(t, m, InboundFromContext(ctx))

	ctx = WithOutboundContext(ctx, m)
	assert.Equal(t, m, OutboundFromContext(ctx))

	// Test defaults
	ctx = context.Background()
	assert.NotNil(t, InboundFromContext(ctx)) // Should return defaultMarshaler
	assert.NotNil(t, OutboundFromContext(ctx))
}
