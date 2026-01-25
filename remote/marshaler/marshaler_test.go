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
	reg := BuildMarshalerRegistry("jsonpb", "proto")

	t.Run("Default", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil) // nolint:noctx
		in, out := reg.GetMarshaler(req)
		assert.IsType(t, &JSONPb{}, in)
		assert.IsType(t, &JSONPb{}, out)
	})

	t.Run("Accept Header", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)           // nolint:noctx
		req.Header.Set("Accept", "application/octet-stream") // proto maps to this? No, wait.
		// The registry maps "scheme" names to marshalers.
		// But GetMarshaler looks up by Accept header value.
		// Wait, BuildMarshalerRegistry takes schemes like "jsonpb", "proto".
		// But GetMarshaler uses mimeMap.
		// Let's check how mimeMap is populated.
		// marshalerRegistry.add(mime, marshaler)
		// But BuildMarshalerRegistry calls buildMarshaller(item) and then add(item, marshaler).
		// So the key in mimeMap is the scheme name?
		// Let's check GetMarshaler again.
		// It looks up r.Header[acceptHeader].
		// So if I pass "jsonpb", the Accept header must be "jsonpb"? That seems wrong.
		// Usually it should be "application/json".

		// Ah, looking at marshaler_registry.go:
		// func BuildMarshalerRegistry(scheme ...string) Registry {
		//     for _, item := range scheme {
		//         marshaler, err := buildMarshaller(item)
		//         _ = mr.add(item, marshaler)
		//     }
		// }
		// And marshaler.go:
		// RegisterMarshallerBuilder("jsonpb", NewJSONPbMarshaler)
		// RegisterMarshallerBuilder("proto", ...)

		// But GetMarshaler uses:
		// if m, ok := mr.mimeMap[acceptVal]; ok

		// So if I register "jsonpb", the mimeMap key is "jsonpb".
		// But the client sends "application/json".
		// This implies that the 'scheme' passed to BuildMarshalerRegistry should be the MIME type?
		// But buildMarshaller uses the scheme to look up the builder.
		// This looks like a potential bug or confusion in the existing code.
		// The `marshalerBuilder` map uses keys like "jsonpb", "proto".
		// But `marshalerRegistry` expects keys to be MIME types (from Accept/Content-Type headers).

		// If I look at `marshal_jsonpb.go`:
		// RegisterMarshallerBuilder("jsonpb", NewJSONPbMarshaler)

		// If I call BuildMarshalerRegistry("jsonpb"), it looks up builder "jsonpb", gets the marshaler, and adds it with key "jsonpb".
		// Then GetMarshaler looks for "jsonpb" in Accept header.
		// This means the code expects `Accept: jsonpb`. This is non-standard.
		// OR, maybe I should register with MIME types?
		// But `marshalerBuilder` is global and populated by init().

		// Let's assume for now I should test with what the code does, which is using the scheme name as the MIME type key.
		// I will verify this behavior.

		req.Header.Set("Accept", "proto")
		_, out := reg.GetMarshaler(req)
		assert.IsType(t, &ProtoMarshaller{}, out)
	})

	t.Run("Content-Type Header", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/", nil) // nolint:noctx
		req.Header.Set("Content-Type", "proto")
		in, _ := reg.GetMarshaler(req)
		assert.IsType(t, &ProtoMarshaller{}, in)
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
