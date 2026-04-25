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
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
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

func TestProtoMarshaler(t *testing.T) {
	m := &ProtoMarshaler{}
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

	t.Run("Accept Header Uses MIME", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil) // nolint:noctx
		req.Header.Set("Accept", "application/octet-stream")
		_, out := reg.GetMarshaler(req)
		assert.IsType(t, &ProtoMarshaler{}, out)
	})

	t.Run("Content-Type Header Uses MIME", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/", nil) // nolint:noctx
		req.Header.Set("Content-Type", "application/octet-stream")
		in, _ := reg.GetMarshaler(req)
		assert.IsType(t, &ProtoMarshaler{}, in)
	})

	t.Run("Alias Fallback Remains Supported", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/", nil) // nolint:noctx
		req.Header.Set("Accept", "jsonpb")
		req.Header.Set("Content-Type", "proto")
		in, out := reg.GetMarshaler(req)
		assert.IsType(t, &ProtoMarshaler{}, in)
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
	m := &ProtoMarshaler{}

	ctx = WithInboundContext(ctx, m)
	assert.Equal(t, m, InboundFromContext(ctx))

	ctx = WithOutboundContext(ctx, m)
	assert.Equal(t, m, OutboundFromContext(ctx))

	// Test defaults
	ctx = context.Background()
	assert.NotNil(t, InboundFromContext(ctx)) // Should return defaultMarshaler
	assert.NotNil(t, OutboundFromContext(ctx))
}

func TestBuildMarshaler_Error(t *testing.T) {
	_, err := BuildMarshaler("unknown")
	assert.Error(t, err)
}

func TestBuildMarshalerWithConfig(t *testing.T) {
	t.Run("jsonpb with config", func(t *testing.T) {
		m, err := BuildMarshalerWithConfig("jsonpb", nil)
		require.NoError(t, err)
		assert.NotNil(t, m)
	})
	t.Run("proto fallback", func(t *testing.T) {
		m, err := BuildMarshalerWithConfig("proto", nil)
		require.NoError(t, err)
		assert.NotNil(t, m)
	})
}

func TestBuildMarshalerWithBuilders(t *testing.T) {
	t.Run("jsonpb with builders", func(t *testing.T) {
		m, err := BuildMarshalerWithBuilders(nil, "jsonpb", nil)
		require.NoError(t, err)
		assert.NotNil(t, m)
	})
	t.Run("custom builder", func(t *testing.T) {
		custom := func() (Marshaler, error) { return &ProtoMarshaler{}, nil }
		builders := map[string]MarshalerBuilder{"custom": custom}
		m, err := BuildMarshalerWithBuilders(builders, "custom", nil)
		require.NoError(t, err)
		assert.NotNil(t, m)
	})
	t.Run("fallback to builtin", func(t *testing.T) {
		m, err := BuildMarshalerWithBuilders(nil, "proto", nil)
		require.NoError(t, err)
		assert.NotNil(t, m)
	})
}

func TestMarshalerForContentType(t *testing.T) {
	t.Run("empty returns nil", func(t *testing.T) {
		assert.Nil(t, MarshalerForContentType(""))
	})
	t.Run("json returns JSONPb", func(t *testing.T) {
		m := MarshalerForContentType("application/json")
		assert.NotNil(t, m)
		assert.IsType(t, &JSONPb{}, m)
	})
	t.Run("json with charset returns JSONPb", func(t *testing.T) {
		m := MarshalerForContentType("application/json; charset=utf-8")
		assert.NotNil(t, m)
		assert.IsType(t, &JSONPb{}, m)
	})
	t.Run("octet-stream returns proto", func(t *testing.T) {
		m := MarshalerForContentType("application/octet-stream")
		assert.NotNil(t, m)
		assert.IsType(t, &ProtoMarshaler{}, m)
	})
}

func TestMarshalerForValue(t *testing.T) {
	t.Run("nil returns proto", func(t *testing.T) {
		assert.IsType(t, &ProtoMarshaler{}, MarshalerForValue(nil))
	})
	t.Run("proto message returns proto", func(t *testing.T) {
		assert.IsType(t, &ProtoMarshaler{}, MarshalerForValue(wrapperspb.String("test")))
	})
	t.Run("non-proto returns jsonpb", func(t *testing.T) {
		assert.IsType(t, &JSONPb{}, MarshalerForValue("string"))
	})
}

func TestNormalizeContentType(t *testing.T) {
	assert.Equal(t, "application/json", NormalizeContentType("application/json"))
	assert.Equal(t, "application/json", NormalizeContentType("Application/JSON; charset=utf-8"))
	assert.Equal(t, "", NormalizeContentType(""))
	assert.Equal(t, "application/octet-stream", NormalizeContentType("application/octet-stream"))
}

func TestCanonicalContentTypeForScheme(t *testing.T) {
	assert.Equal(t, ContentTypeJSON, CanonicalContentTypeForScheme("jsonpb"))
	assert.Equal(t, ContentTypeJSON, CanonicalContentTypeForScheme("JSONPB"))
	assert.Equal(t, ContentTypeProto, CanonicalContentTypeForScheme("proto"))
	assert.Equal(t, "", CanonicalContentTypeForScheme("unknown"))
}

func TestHasMarshalerBuilder(t *testing.T) {
	assert.True(t, HasMarshalerBuilder("jsonpb"))
	assert.True(t, HasMarshalerBuilder("proto"))
	assert.False(t, HasMarshalerBuilder("unknown"))
}

func TestRegistryErrors(t *testing.T) {
	mr := NewRegistry()
	err := mr.Register("test", nil)
	assert.Error(t, err)
	err = mr.Register("", &ProtoMarshaler{})
	assert.Error(t, err)
}

func TestBuildMarshalerRegistryWithBuilders(t *testing.T) {
	reg := BuildMarshalerRegistryWithBuilders(nil, nil, "jsonpb", "proto")
	require.NotNil(t, reg)
	req, _ := http.NewRequest("GET", "/", nil) // nolint:noctx
	in, out := reg.GetMarshaler(req)
	assert.NotNil(t, in)
	assert.NotNil(t, out)
}

// testProtoEnum implements protoEnum for testing enum marshal/unmarshal paths.
type testProtoEnum int32

const (
	testEnumAlpha testProtoEnum = iota + 1
	testEnumBeta
)

func (e testProtoEnum) String() string {
	switch e {
	case testEnumAlpha:
		return "ALPHA"
	case testEnumBeta:
		return "BETA"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", int(e))
	}
}

func (e testProtoEnum) EnumDescriptor() ([]byte, []int) { return nil, nil }

// errorWriter is an io.Writer that always returns an error.
type errorWriter struct {
	err error
}

func (w *errorWriter) Write(p []byte) (int, error) { return 0, w.err }

// ---------------------------------------------------------------------------
// marshalNonProtoField branch tests
// ---------------------------------------------------------------------------

func TestJSONPb_MarshalNonProto_Nil(t *testing.T) {
	m, err := NewJSONPbMarshaler()
	require.NoError(t, err)
	data, err := m.Marshal(nil)
	require.NoError(t, err)
	assert.Equal(t, "null", string(data))
}

func TestJSONPb_MarshalNonProto_NilPointer(t *testing.T) {
	m, err := NewJSONPbMarshaler()
	require.NoError(t, err)
	var p *int
	data, err := m.Marshal(p)
	require.NoError(t, err)
	assert.Equal(t, "null", string(data))
}

func TestJSONPb_MarshalNonProto_NilSlice_EmitUnpopulated(t *testing.T) {
	m := NewJSONPbMarshalerWithConfig(&JSONPbConfig{
		MarshalOptions: struct {
			Multiline         bool   `mapstructure:"multiline"`
			Indent            string `mapstructure:"indent"`
			AllowPartial      bool   `mapstructure:"allow_partial"`
			UseProtoNames     bool   `mapstructure:"use_proto_names"`
			UseEnumNumbers    bool   `mapstructure:"use_enum_numbers"`
			EmitUnpopulated   bool   `mapstructure:"emit_unpopulated"`
			EmitDefaultValues bool   `mapstructure:"emit_default_values"`
		}{
			EmitUnpopulated: true,
		},
	})
	var sl []string
	data, err := m.Marshal(sl)
	require.NoError(t, err)
	assert.Equal(t, "[]", string(data))
}

func TestJSONPb_MarshalNonProto_NilSlice_Default(t *testing.T) {
	// EmitUnpopulated=false to get "null" for nil slice
	m := NewJSONPbMarshalerWithConfig(&JSONPbConfig{
		MarshalOptions: struct {
			Multiline         bool   `mapstructure:"multiline"`
			Indent            string `mapstructure:"indent"`
			AllowPartial      bool   `mapstructure:"allow_partial"`
			UseProtoNames     bool   `mapstructure:"use_proto_names"`
			UseEnumNumbers    bool   `mapstructure:"use_enum_numbers"`
			EmitUnpopulated   bool   `mapstructure:"emit_unpopulated"`
			EmitDefaultValues bool   `mapstructure:"emit_default_values"`
		}{
			EmitUnpopulated: false,
		},
	})
	var sl []string
	data, err := m.Marshal(sl)
	require.NoError(t, err)
	assert.Equal(t, "null", string(data))
}

func TestJSONPb_MarshalNonProto_SliceOfProto(t *testing.T) {
	m, err := NewJSONPbMarshaler()
	require.NoError(t, err)
	sl := []proto.Message{wrapperspb.String("a"), wrapperspb.String("b")}
	data, err := m.Marshal(sl)
	require.NoError(t, err)
	assert.Equal(t, `["a","b"]`, string(data))
}

func TestJSONPb_MarshalNonProto_SliceOfEnum_Numbers(t *testing.T) {
	m := NewJSONPbMarshalerWithConfig(&JSONPbConfig{
		MarshalOptions: struct {
			Multiline         bool   `mapstructure:"multiline"`
			Indent            string `mapstructure:"indent"`
			AllowPartial      bool   `mapstructure:"allow_partial"`
			UseProtoNames     bool   `mapstructure:"use_proto_names"`
			UseEnumNumbers    bool   `mapstructure:"use_enum_numbers"`
			EmitUnpopulated   bool   `mapstructure:"emit_unpopulated"`
			EmitDefaultValues bool   `mapstructure:"emit_default_values"`
		}{
			UseEnumNumbers: true,
		},
	})
	sl := []testProtoEnum{testEnumAlpha, testEnumBeta}
	data, err := m.Marshal(sl)
	require.NoError(t, err)
	assert.Equal(t, `[1,2]`, string(data))
}

func TestJSONPb_MarshalNonProto_SliceOfEnum_Strings(t *testing.T) {
	m, err := NewJSONPbMarshaler()
	require.NoError(t, err)
	// Default UseEnumNumbers is false with EmitUnpopulated=true
	sl := []testProtoEnum{testEnumAlpha, testEnumBeta}
	data, err := m.Marshal(sl)
	require.NoError(t, err)
	assert.Equal(t, `["ALPHA","BETA"]`, string(data))
}

func TestJSONPb_MarshalNonProto_Enum_UseStrings(t *testing.T) {
	m, err := NewJSONPbMarshaler()
	require.NoError(t, err)
	data, err := m.Marshal(testEnumAlpha)
	require.NoError(t, err)
	assert.Equal(t, `"ALPHA"`, string(data))
}

func TestJSONPb_MarshalNonProto_Enum_UseNumbers(t *testing.T) {
	m := NewJSONPbMarshalerWithConfig(&JSONPbConfig{
		MarshalOptions: struct {
			Multiline         bool   `mapstructure:"multiline"`
			Indent            string `mapstructure:"indent"`
			AllowPartial      bool   `mapstructure:"allow_partial"`
			UseProtoNames     bool   `mapstructure:"use_proto_names"`
			UseEnumNumbers    bool   `mapstructure:"use_enum_numbers"`
			EmitUnpopulated   bool   `mapstructure:"emit_unpopulated"`
			EmitDefaultValues bool   `mapstructure:"emit_default_values"`
		}{
			UseEnumNumbers: true,
		},
	})
	data, err := m.Marshal(testEnumAlpha)
	require.NoError(t, err)
	assert.Equal(t, "1", string(data))
}

func TestJSONPb_MarshalNonProto_MapWithProtoValues(t *testing.T) {
	m, err := NewJSONPbMarshaler()
	require.NoError(t, err)
	mv := map[string]proto.Message{
		"a": wrapperspb.String("x"),
		"b": wrapperspb.String("y"),
	}
	data, err := m.Marshal(mv)
	require.NoError(t, err)
	assert.JSONEq(t, `{"a":"x","b":"y"}`, string(data))
}

// ---------------------------------------------------------------------------
// decodeNonProtoField branch tests
// ---------------------------------------------------------------------------

func TestDecodeNonProtoField_NonPointerError(t *testing.T) {
	m, err := NewJSONPbMarshaler()
	require.NoError(t, err)
	err = m.Unmarshal([]byte(`42`), "not-a-pointer")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a pointer")
}

func TestDecodeNonProtoField_PointerChain(t *testing.T) {
	m, err := NewJSONPbMarshaler()
	require.NoError(t, err)
	v := 0
	pv := &v
	ppv := &pv
	err = m.Unmarshal([]byte(`42`), ppv)
	require.NoError(t, err)
	assert.Equal(t, 42, **ppv)
}

func TestDecodeNonProtoField_PointerToProto(t *testing.T) {
	m, err := NewJSONPbMarshaler()
	require.NoError(t, err)
	var p *wrapperspb.StringValue
	err = m.Unmarshal([]byte(`"hello"`), &p)
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, "hello", p.Value)
}

func TestDecodeNonProtoField_Map(t *testing.T) {
	m, err := NewJSONPbMarshaler()
	require.NoError(t, err)
	var v map[string]string
	err = m.Unmarshal([]byte(`{"foo":"bar"}`), &v)
	require.NoError(t, err)
	assert.Equal(t, "bar", v["foo"])
}

func TestDecodeNonProtoField_Slice(t *testing.T) {
	m, err := NewJSONPbMarshaler()
	require.NoError(t, err)
	var v []string
	err = m.Unmarshal([]byte(`["a","b","c"]`), &v)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, v)
}

func TestDecodeNonProtoField_ByteSlice(t *testing.T) {
	m, err := NewJSONPbMarshaler()
	require.NoError(t, err)
	original := []byte("hello, world")
	encoded := base64.StdEncoding.EncodeToString(original)
	var v []byte
	err = m.Unmarshal([]byte(`"`+encoded+`"`), &v)
	require.NoError(t, err)
	assert.Equal(t, original, v)
}

func TestDecodeNonProtoField_Enum_Float64(t *testing.T) {
	m, err := NewJSONPbMarshaler()
	require.NoError(t, err)
	var e testProtoEnum
	err = m.Unmarshal([]byte(`1`), &e)
	require.NoError(t, err)
	assert.Equal(t, testEnumAlpha, e)
}

func TestDecodeNonProtoField_Enum_StringError(t *testing.T) {
	m, err := NewJSONPbMarshaler()
	require.NoError(t, err)
	var e testProtoEnum
	err = m.Unmarshal([]byte(`"ALPHA"`), &e)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "symbolic enum")
}

func TestDecodeNonProtoField_Enum_DefaultError(t *testing.T) {
	m, err := NewJSONPbMarshaler()
	require.NoError(t, err)
	var e testProtoEnum
	err = m.Unmarshal([]byte(`true`), &e)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot assign")
}

// ---------------------------------------------------------------------------
// Other low-coverage function tests
// ---------------------------------------------------------------------------

func TestNewJSONPbMarshalerWithConfig_NonNil(t *testing.T) {
	cfg := &JSONPbConfig{
		MarshalOptions: struct {
			Multiline         bool   `mapstructure:"multiline"`
			Indent            string `mapstructure:"indent"`
			AllowPartial      bool   `mapstructure:"allow_partial"`
			UseProtoNames     bool   `mapstructure:"use_proto_names"`
			UseEnumNumbers    bool   `mapstructure:"use_enum_numbers"`
			EmitUnpopulated   bool   `mapstructure:"emit_unpopulated"`
			EmitDefaultValues bool   `mapstructure:"emit_default_values"`
		}{
			Multiline:         true,
			Indent:            "  ",
			AllowPartial:      true,
			UseProtoNames:     true,
			UseEnumNumbers:    true,
			EmitUnpopulated:   true,
			EmitDefaultValues: true,
		},
		UnmarshalOptions: struct {
			AllowPartial   bool `mapstructure:"allow_partial"`
			DiscardUnknown bool `mapstructure:"discard_unknown"`
			RecursionLimit int  `mapstructure:"recursion_limit"`
		}{
			AllowPartial:   true,
			DiscardUnknown: true,
			RecursionLimit: 100,
		},
	}
	m := NewJSONPbMarshalerWithConfig(cfg)
	require.NotNil(t, m)
	assert.True(t, m.Multiline)
	assert.Equal(t, "  ", m.Indent)
	assert.True(t, m.MarshalOptions.AllowPartial)
	assert.True(t, m.UseProtoNames)
	assert.True(t, m.UseEnumNumbers)
	assert.True(t, m.EmitUnpopulated)
	assert.True(t, m.EmitDefaultValues)
	assert.True(t, m.UnmarshalOptions.AllowPartial)
	assert.True(t, m.DiscardUnknown)
	assert.Equal(t, 100, m.RecursionLimit)
}

func TestJSONPb_MarshalWithIndent(t *testing.T) {
	m := NewJSONPbMarshalerWithConfig(&JSONPbConfig{
		MarshalOptions: struct {
			Multiline         bool   `mapstructure:"multiline"`
			Indent            string `mapstructure:"indent"`
			AllowPartial      bool   `mapstructure:"allow_partial"`
			UseProtoNames     bool   `mapstructure:"use_proto_names"`
			UseEnumNumbers    bool   `mapstructure:"use_enum_numbers"`
			EmitUnpopulated   bool   `mapstructure:"emit_unpopulated"`
			EmitDefaultValues bool   `mapstructure:"emit_default_values"`
		}{
			Indent: "  ",
		},
	})
	v := map[string]string{"key": "value"}
	data, err := m.Marshal(v)
	require.NoError(t, err)
	// With indent, the output should contain newlines
	assert.Contains(t, string(data), "\n")
}

func TestJSONPb_Delimiter(t *testing.T) {
	m := NewJSONPbMarshalerWithConfig(nil)
	assert.Equal(t, []byte("\n"), m.Delimiter())
}

func TestBuildMarshalerRegistry_ErrorPath(t *testing.T) {
	// Passing an unknown scheme should not panic; it logs a warning and continues
	reg := BuildMarshalerRegistry("unknown_scheme", "jsonpb")
	require.NotNil(t, reg)
	// jsonpb should still be registered
	req, _ := http.NewRequest("GET", "/", nil) // nolint:noctx
	in, out := reg.GetMarshaler(req)
	assert.NotNil(t, in)
	assert.NotNil(t, out)
	assert.IsType(t, &JSONPb{}, in)
}

func TestBuildMarshalerRegistryWithBuilders_ErrorPath(t *testing.T) {
	// Passing an unknown scheme should not panic; only "proto" is successfully registered
	reg := BuildMarshalerRegistryWithBuilders(nil, nil, "unknown_scheme", "proto")
	require.NotNil(t, reg)
	req, _ := http.NewRequest("POST", "/", nil) // nolint:noctx
	req.Header.Set("Content-Type", "application/octet-stream")
	in, _ := reg.GetMarshaler(req)
	assert.IsType(t, &ProtoMarshaler{}, in)
}

func TestRegistry_HeaderValues_CommaSeparated(t *testing.T) {
	reg := BuildMarshalerRegistry("jsonpb", "proto")

	// Multiple comma-separated values in a single Accept header
	req, _ := http.NewRequest("GET", "/", nil) // nolint:noctx
	req.Header.Set("Accept", "text/html, application/octet-stream")
	_, out := reg.GetMarshaler(req)
	assert.IsType(t, &ProtoMarshaler{}, out)

	// Multiple comma-separated values in Content-Type
	req2, _ := http.NewRequest("POST", "/", nil) // nolint:noctx
	req2.Header.Set("Content-Type", "application/json")
	in, _ := reg.GetMarshaler(req2)
	assert.IsType(t, &JSONPb{}, in)
}

func TestProtoMarshaler_NewEncoder_WriteError(t *testing.T) {
	m := &ProtoMarshaler{}
	w := &errorWriter{err: errors.New("write failed")}
	enc := m.NewEncoder(w)
	err := enc.Encode(wrapperspb.String("test"))
	assert.Error(t, err)
	assert.Equal(t, "write failed", err.Error())
}

func TestJSONPb_Encoder_WriteError(t *testing.T) {
	m, err := NewJSONPbMarshaler()
	require.NoError(t, err)
	w := &errorWriter{err: errors.New("write failed")}
	enc := m.NewEncoder(w)
	err = enc.Encode(wrapperspb.String("test"))
	assert.Error(t, err)
	assert.Equal(t, "write failed", err.Error())
}

func TestJSONPb_Marshal_NonProtoWithIndent(t *testing.T) {
	m := NewJSONPbMarshalerWithConfig(&JSONPbConfig{
		MarshalOptions: struct {
			Multiline         bool   `mapstructure:"multiline"`
			Indent            string `mapstructure:"indent"`
			AllowPartial      bool   `mapstructure:"allow_partial"`
			UseProtoNames     bool   `mapstructure:"use_proto_names"`
			UseEnumNumbers    bool   `mapstructure:"use_enum_numbers"`
			EmitUnpopulated   bool   `mapstructure:"emit_unpopulated"`
			EmitDefaultValues bool   `mapstructure:"emit_default_values"`
		}{
			Indent: "  ",
		},
	})
	// map → non-proto path with indent
	v := map[string]string{"a": "1", "b": "2"}
	data, err := m.Marshal(v)
	require.NoError(t, err)
	assert.Contains(t, string(data), "\n")

	// Verify indent applied via marshalTo non-proto indent branch
	m2 := NewJSONPbMarshalerWithConfig(&JSONPbConfig{
		MarshalOptions: struct {
			Multiline         bool   `mapstructure:"multiline"`
			Indent            string `mapstructure:"indent"`
			AllowPartial      bool   `mapstructure:"allow_partial"`
			UseProtoNames     bool   `mapstructure:"use_proto_names"`
			UseEnumNumbers    bool   `mapstructure:"use_enum_numbers"`
			EmitUnpopulated   bool   `mapstructure:"emit_unpopulated"`
			EmitDefaultValues bool   `mapstructure:"emit_default_values"`
		}{
			Indent: " ",
		},
	})
	var buf bytes.Buffer
	err = m2.NewEncoder(&buf).Encode(map[string]string{"x": "y"})
	require.NoError(t, err)
	// Encoder writes delimiter after content
	body := buf.String()
	assert.True(t, strings.Contains(body, "\n"), "expected newline delimiter in encoder output")
}

func TestMarshalerForContentType_JSONVariant(t *testing.T) {
	// Content type with json substring
	m := MarshalerForContentType("text/json")
	assert.NotNil(t, m)
	assert.IsType(t, &JSONPb{}, m)
}

func TestNormalizeContentType_InvalidMedia(t *testing.T) {
	// Valid media type is parsed by mime.ParseMediaType and lowercased
	result := NormalizeContentType("  Application/JSON; charset=utf-8  ")
	assert.Equal(t, "application/json", result)

	// Truly invalid media type falls back to lower+trim
	result2 := NormalizeContentType("not@valid/media")
	assert.Equal(t, "not@valid/media", result2)
}

func TestBuildMarshalerWithConfig_UnknownScheme(t *testing.T) {
	// Unknown scheme falls through to BuildMarshaler which returns error
	_, err := BuildMarshalerWithConfig("unknown", nil)
	assert.Error(t, err)
}

func TestBuildMarshalerWithBuilders_CustomBuilderError(t *testing.T) {
	// Custom builder that returns an error
	errBuilder := func() (Marshaler, error) { return nil, errors.New("builder failed") }
	builders := map[string]MarshalerBuilder{"bad": errBuilder}
	_, err := BuildMarshalerWithBuilders(builders, "bad", nil)
	assert.Error(t, err)
	assert.Equal(t, "builder failed", err.Error())
}
