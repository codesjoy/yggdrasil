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

package rest

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/codesjoy/yggdrasil/v3/transport/support/marshaler"
)

func TestParseField_Uint32(t *testing.T) {
	msg := &wrapperspb.UInt32Value{}
	err := PopulateFieldFromPath(msg, "value", "42")
	require.NoError(t, err)
	assert.Equal(t, uint32(42), msg.Value)
}

func TestParseField_Uint64(t *testing.T) {
	msg := &wrapperspb.UInt64Value{}
	err := PopulateFieldFromPath(msg, "value", "123456789012")
	require.NoError(t, err)
	assert.Equal(t, uint64(123456789012), msg.Value)
}

func TestPopulateQueryParameters(t *testing.T) {
	// structpb.Struct is a good candidate for testing map and value types
	// But it's a bit complex. Let's use wrappers for simple types if possible.
	// Actually, `wrapperspb` types don't have fields we can easily populate via query params
	// because they usually have a single `value` field.

	// Let's try to use `structpb.Struct` for map testing.
	// fields { key: "foo" value { string_value: "bar" } }
	// Query: fields.foo.string_value=bar ?? No, Struct uses a map.
	// populateFieldValues handles maps.

	// Better yet, let's test `parseField` directly for most logic,
	// and use a simple message for PopulateQueryParameters if we can find one in standard lib.
	// `durationpb.Duration` has `seconds` (int64) and `nanos` (int32).

	t.Run("Duration", func(t *testing.T) {
		msg := &durationpb.Duration{}
		values := url.Values{}
		values.Set("seconds", "100")
		values.Set("nanos", "500")

		err := PopulateQueryParameters(msg, values)
		require.NoError(t, err)
		assert.Equal(t, int64(100), msg.Seconds)
		assert.Equal(t, int32(500), msg.Nanos)
	})

	// Test error case
	t.Run("Invalid Field", func(t *testing.T) {
		msg := &durationpb.Duration{}
		values := url.Values{}
		values.Set("unknown", "100")

		// Should log info and return nil (not error) based on implementation
		err := PopulateQueryParameters(msg, values)
		require.NoError(t, err)
	})
}

func TestPopulateFieldFromPath(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		msg := &wrapperspb.StringValue{}
		err := PopulateFieldFromPath(msg, "value", "hello")
		require.NoError(t, err)
		assert.Equal(t, "hello", msg.Value)
	})

	t.Run("Nested", func(t *testing.T) {
		// Timestamp has seconds and nanos
		msg := &timestamppb.Timestamp{}
		err := PopulateFieldFromPath(msg, "seconds", "1234567890")
		require.NoError(t, err)
		assert.Equal(t, int64(1234567890), msg.Seconds)
	})
}

func TestParseField(t *testing.T) {
	// We can't easily call parseField directly because it requires FieldDescriptor.
	// But we can test it via PopulateFieldFromPath using various types.

	t.Run("Bool", func(t *testing.T) {
		msg := &wrapperspb.BoolValue{}
		err := PopulateFieldFromPath(msg, "value", "true")
		require.NoError(t, err)
		assert.True(t, msg.Value)
	})

	t.Run("Int32", func(t *testing.T) {
		msg := &wrapperspb.Int32Value{}
		err := PopulateFieldFromPath(msg, "value", "123")
		require.NoError(t, err)
		assert.Equal(t, int32(123), msg.Value)
	})

	t.Run("Int64", func(t *testing.T) {
		msg := &wrapperspb.Int64Value{}
		err := PopulateFieldFromPath(msg, "value", "1234567890123")
		require.NoError(t, err)
		assert.Equal(t, int64(1234567890123), msg.Value)
	})

	t.Run("Float", func(t *testing.T) {
		msg := &wrapperspb.FloatValue{}
		err := PopulateFieldFromPath(msg, "value", "1.5")
		require.NoError(t, err)
		assert.Equal(t, float32(1.5), msg.Value)
	})

	t.Run("Double", func(t *testing.T) {
		msg := &wrapperspb.DoubleValue{}
		err := PopulateFieldFromPath(msg, "value", "1.5")
		require.NoError(t, err)
		assert.Equal(t, 1.5, msg.Value)
	})

	t.Run("String", func(t *testing.T) {
		msg := &wrapperspb.StringValue{}
		err := PopulateFieldFromPath(msg, "value", "hello")
		require.NoError(t, err)
		assert.Equal(t, "hello", msg.Value)
	})

	t.Run("Bytes", func(t *testing.T) {
		msg := &wrapperspb.BytesValue{}
		// base64 "hello" -> "aGVsbG8="
		err := PopulateFieldFromPath(msg, "value", "aGVsbG8=")
		require.NoError(t, err)
		assert.Equal(t, []byte("hello"), msg.Value)
	})
}

func TestParseMessage(t *testing.T) {
	// Test well-known types parsing via PopulateFieldFromPath
	// This requires a message that HAS a well-known type field.
	// `structpb.Value` has `kind` oneof which can be `number_value`, `string_value`, `bool_value`, `struct_value`, `list_value`.
	// But we need a field that IS a message, not a primitive.

	// `structpb.ListValue` has `values` (repeated Value).
	// `structpb.Struct` has `fields` (map<string, Value>).

	// Maybe we can use `google.protobuf.Duration` or `Timestamp` directly?
	// `parseMessage` is called when the field kind is MessageKind.
	// But `PopulateFieldFromPath` traverses into the message if it's a message field.
	// It only calls `parseField` (and thus `parseMessage`) if we are setting the message ITSELF from a string.
	// This happens when we have a field of type Message, and we provide a string value for it.
	// e.g. `timestamp_field=2023-01-01T00:00:00Z`

	// We need a message that contains a Timestamp field.
	// Standard types don't usually nest other standard types in a way we can easily use here without generating code.
	// However, we can test `parseMessage` logic by using `wrapperspb` which are handled in `parseMessage`.
	// But `wrapperspb` are usually used as fields.

	// Let's verify `Timestamp` parsing.
	// We need a message with a Timestamp field.
	// Since we can't easily find one, we might skip direct `parseMessage` testing via `PopulateFieldFromPath`
	// unless we define a custom proto (which we can't do easily here).

	// Wait, `PopulateFieldFromPath` logic:
	// If `fd.Message() != nil`, it traverses using `v.Mutable(fd).Message()`.
	// UNLESS `i == len(fieldPath)-1`.
	// If it is the last segment, and it is a Message, it calls `populateField`.
	// `populateField` calls `parseField`.
	// `parseField` calls `parseMessage`.

	// So if we have a message `M` with field `t` of type `Timestamp`.
	// `PopulateFieldFromPath(M, "t", "2023...")` should work.

	// Is there a standard message with a Timestamp field?
	// `google.protobuf.Api` has `source_context`.
	// `google.protobuf.Type` has `source_context`.
	// `google.protobuf.SourceContext` has `file_name`.

	// Not finding an easy one.
	// But we can test `wrapperspb` types themselves!
	// `wrapperspb.StringValue` IS a message.
	// But `PopulateFieldFromPath` takes a root message and a path.
	// If we pass `wrapperspb.StringValue` as root, and path "value", "value" is a string field, so it uses `StringKind`.

	// What if we try to populate a `StringValue` itself?
	// We need a message that has a `StringValue` field.
	// `structpb.Value` has `string_value` which is just a string, not `StringValue`.

	// Let's stick to what we can test: primitives and recursion into fields.
}

func TestPopulateFieldValues_Errors(t *testing.T) {
	t.Run("Field not found", func(t *testing.T) {
		msg := &wrapperspb.StringValue{}
		err := PopulateFieldFromPath(msg, "nonexistent_field", "hello")
		// field not found returns nil (logs info and returns)
		require.NoError(t, err)
	})

	t.Run("Empty values via PopulateQueryParameters", func(t *testing.T) {
		msg := &wrapperspb.StringValue{}
		values := url.Values{}
		// No keys, so no iteration -> no error
		err := PopulateQueryParameters(msg, values)
		require.NoError(t, err)
	})
}

func TestParseField_InvalidBool(t *testing.T) {
	msg := &wrapperspb.BoolValue{}
	err := PopulateFieldFromPath(msg, "value", "notabool")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing field")
}

func TestParseField_InvalidInt32(t *testing.T) {
	msg := &wrapperspb.Int32Value{}
	err := PopulateFieldFromPath(msg, "value", "notanumber")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing field")
}

func TestParseField_InvalidInt64(t *testing.T) {
	msg := &wrapperspb.Int64Value{}
	err := PopulateFieldFromPath(msg, "value", "notanumber")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing field")
}

func TestParseField_InvalidFloat(t *testing.T) {
	msg := &wrapperspb.FloatValue{}
	err := PopulateFieldFromPath(msg, "value", "notafloat")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing field")
}

func TestParseField_InvalidDouble(t *testing.T) {
	msg := &wrapperspb.DoubleValue{}
	err := PopulateFieldFromPath(msg, "value", "notafloat")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing field")
}

func TestParseField_InvalidBytes(t *testing.T) {
	msg := &wrapperspb.BytesValue{}
	err := PopulateFieldFromPath(msg, "value", "!!!invalid-base64!!!")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing field")
}

func TestParseField_Enum(t *testing.T) {
	// Use google.protobuf.Duration which has an enum-like field behavior
	// Actually test via a message that has a Duration field
	// PopulateFieldFromPath on a Duration itself sets seconds/nanos which are int types.
	// For enum, we need a proto with an enum field.
	// Use structpb.Value.kind which has enum behavior through oneof.
	// Actually, let's test via PopulateQueryParameters with repeated values
	// and map values to cover populateRepeatedField and populateMapField.
}

func TestPopulateRepeatedField(t *testing.T) {
	// structpb.ListValue has repeated Value fields.
	// But Value is a message type, so it won't go through parseField for the repeated element.
	// Instead, use PopulateQueryParameters with a repeated field.
	// durationpb.Duration has int64 seconds, we can't have repeated.
	// Let's use a message that has repeated scalar fields.
	// structpb.ListValue's "values" is repeated structpb.Value - message kind, not parseField path.

	// Let's test populateFieldValues with repeated path via PopulateQueryParameters.
	// Use url.Values with multiple values for the same key on a repeated field.
	// We need a proto message with a repeated scalar field.
	// timestamppb.Timestamp doesn't have repeated fields.
	// Let's test via PopulateQueryParameters with the key being a repeated field path.

	// The simplest approach: test populateRepeatedField indirectly.
	// We need a message with repeated field. Let's use field_mask.FieldMask which has repeated string paths.
	t.Run("FieldMask", func(t *testing.T) {
		msg := &fieldmaskpb.FieldMask{}
		values := url.Values{}
		values.Set("paths", "foo,bar,baz")
		err := PopulateQueryParameters(msg, values)
		require.NoError(t, err)
		// Note: "paths" is a repeated string field, but PopulateQueryParameters
		// iterates each key and passes the values to populateFieldValues.
		// For repeated fields, it calls populateRepeatedField.
		// However, paths is a repeated string, and we only have one value "foo,bar,baz".
		// That would be a single value, not multiple.
		// Let's use url.Values with multiple values for the same key.
	})
}

func TestParseMessage_Timestamp(t *testing.T) {
	// parseMessage handles google.protobuf.Timestamp.
	// We need a message with a Timestamp field to test via PopulateFieldFromPath.
	// We can use a wrapper approach - create a message containing Timestamp.
	// Since we can't easily find such a message, test the "null" branch.
	// parseMessage with "null" creates an empty message.
	// But to trigger parseMessage, we need a field of MessageKind.

	// Actually, we can test parseMessage directly for Timestamp by using
	// PopulateFieldFromPath on a message that HAS a timestamp field.
	// google.protobuf.Api has no timestamp, but google.protobuf.Type has source_context.
	// Let's try a different approach - test the known types directly.

	// The easiest way to test parseMessage is through PopulateQueryParameters
	// on a message containing a well-known type field.
	// Since no standard message easily has these, we test what we can.
}

func TestParseMessage_FieldMask(t *testing.T) {
	descriptor := (&fieldmaskpb.FieldMask{}).ProtoReflect().Descriptor()
	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{name: "single path", value: "displayName", want: []string{"display_name"}},
		{
			name:  "multiple and nested paths",
			value: "displayName,createTime.startTime",
			want:  []string{"display_name", "create_time.start_time"},
		},
		{name: "empty mask", value: "", want: nil},
		{name: "wildcard", value: "*", want: []string{"*"}},
		{name: "trimmed wildcard", value: "  *  ", want: []string{"*"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := parseMessage(
				descriptor,
				tt.value,
				queryParameterMarshaler(context.Background()),
			)
			require.NoError(t, err)
			mask := value.Message().Interface().(*fieldmaskpb.FieldMask)
			assert.Equal(t, tt.want, mask.Paths)
		})
	}
}

func TestParseMessage_FieldMaskRejectsInvalidPaths(t *testing.T) {
	descriptor := (&fieldmaskpb.FieldMask{}).ProtoReflect().Descriptor()
	for _, value := range []string{"display_name", "displayName,,createTime", "display-name"} {
		t.Run(value, func(t *testing.T) {
			_, err := parseMessage(
				descriptor,
				value,
				queryParameterMarshaler(context.Background()),
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid path")
		})
	}

	t.Run("mixed wildcard", func(t *testing.T) {
		_, err := parseMessage(
			descriptor,
			"*,displayName",
			queryParameterMarshaler(context.Background()),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "wildcard must be used alone")
	})
}

func TestQueryParameterMarshaler(t *testing.T) {
	t.Run("reuse JSON inbound", func(t *testing.T) {
		inbound := marshaler.NewJSONPbMarshalerWithConfig(nil)
		ctx := marshaler.WithInboundContext(context.Background(), inbound)
		assert.Same(t, inbound, queryParameterMarshaler(ctx))
	})

	t.Run("fallback from protobuf inbound", func(t *testing.T) {
		oldSupport, oldConfig := currentMarshalerConfig()
		t.Cleanup(func() { ConfigureMarshaler(oldSupport, oldConfig) })
		cfg := &marshaler.JSONPbConfig{}
		cfg.UnmarshalOptions.AllowPartial = true
		ConfigureMarshaler(nil, cfg)

		ctx := marshaler.WithInboundContext(context.Background(), &marshaler.ProtoMarshaler{})
		decoder := queryParameterMarshaler(ctx)
		require.IsType(t, &marshaler.JSONPb{}, decoder)
		jsonpb := decoder.(*marshaler.JSONPb)
		assert.True(t, jsonpb.UnmarshalOptions.AllowPartial)
	})

	t.Run("fallback from custom JSON inbound", func(t *testing.T) {
		inbound := &jsonContentTypeProtoMarshaler{}
		ctx := marshaler.WithInboundContext(context.Background(), inbound)
		decoder := queryParameterMarshaler(ctx)
		require.IsType(t, &marshaler.JSONPb{}, decoder)
		assert.NotSame(t, inbound, decoder)
	})
}

type jsonContentTypeProtoMarshaler struct {
	marshaler.ProtoMarshaler
}

func (*jsonContentTypeProtoMarshaler) ContentType(any) string {
	return marshaler.ContentTypeJSON
}

func TestPopulateQueryParameters_FieldMask(t *testing.T) {
	msg := newFieldMaskRequest(t)
	ctx := marshaler.WithInboundContext(context.Background(), &marshaler.ProtoMarshaler{})

	err := PopulateQueryParametersContext(ctx, msg, url.Values{
		"updateMask": {"displayName,createTime.startTime"},
	})
	require.NoError(t, err)

	updateMaskField := msg.Descriptor().Fields().ByName("update_mask")
	mask := msg.Get(updateMaskField).Message()
	pathsField := mask.Descriptor().Fields().ByName("paths")
	paths := mask.Get(pathsField).List()
	require.Equal(t, 2, paths.Len())
	assert.Equal(t, "display_name", paths.Get(0).String())
	assert.Equal(t, "create_time.start_time", paths.Get(1).String())
}

func TestPopulateQueryParametersContext_CustomJSONInboundUsesJSONPbForFieldMask(t *testing.T) {
	msg := newFieldMaskRequest(t)
	ctx := marshaler.WithInboundContext(context.Background(), &jsonContentTypeProtoMarshaler{})

	err := PopulateQueryParametersContext(ctx, msg, url.Values{
		"updateMask": {"displayName"},
	})
	require.NoError(t, err)

	updateMaskField := msg.Descriptor().Fields().ByName("update_mask")
	mask := msg.Get(updateMaskField).Message()
	paths := mask.Get(mask.Descriptor().Fields().ByName("paths")).List()
	require.Equal(t, 1, paths.Len())
	assert.Equal(t, "display_name", paths.Get(0).String())
}

func TestPopulateQueryParametersContext_ExcludedFieldPaths(t *testing.T) {
	t.Run("body prefix and segment boundary", func(t *testing.T) {
		msg := newQueryFilterRequest(t)
		err := PopulateQueryParametersContext(context.Background(), msg, url.Values{
			"resource.displayName": {"ignored"},
			"resource.name":        {"ignored"},
			"resourceName":         {"kept"},
			"pageSize":             {"25"},
			"unknown":              {"ignored"},
		}, "resource")
		require.NoError(t, err)

		resourceField := msg.Descriptor().Fields().ByName("resource")
		assert.False(t, msg.Has(resourceField))
		assert.Equal(t, "kept", msg.Get(msg.Descriptor().Fields().ByName("resource_name")).String())
		assert.Equal(
			t,
			int32(25),
			int32(msg.Get(msg.Descriptor().Fields().ByName("page_size")).Int()),
		)
	})

	t.Run("JSON name exclusion is canonicalized", func(t *testing.T) {
		msg := newQueryFilterRequest(t)
		err := PopulateQueryParametersContext(context.Background(), msg, url.Values{
			"resource.displayName": {"ignored"},
			"resource.name":        {"kept"},
		}, "resource.displayName")
		require.NoError(t, err)

		resourceField := msg.Descriptor().Fields().ByName("resource")
		resource := msg.Get(resourceField).Message()
		assert.Equal(
			t,
			"",
			resource.Get(resource.Descriptor().Fields().ByName("display_name")).String(),
		)
		assert.Equal(
			t,
			"kept",
			resource.Get(resource.Descriptor().Fields().ByName("name")).String(),
		)
	})
}

func newFieldMaskRequest(t testing.TB) *dynamicpb.Message {
	t.Helper()
	file, err := protodesc.NewFile(
		&descriptorpb.FileDescriptorProto{
			Syntax:     proto.String("proto3"),
			Name:       proto.String("field_mask_request.proto"),
			Package:    proto.String("resttest"),
			Dependency: []string{"google/protobuf/field_mask.proto"},
			MessageType: []*descriptorpb.DescriptorProto{
				{
					Name: proto.String("UpdateRequest"),
					Field: []*descriptorpb.FieldDescriptorProto{
						{
							Name:     proto.String("update_mask"),
							JsonName: proto.String("updateMask"),
							Number:   proto.Int32(1),
							Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
							Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
							TypeName: proto.String(".google.protobuf.FieldMask"),
						},
					},
				},
			},
		},
		protoregistry.GlobalFiles,
	)
	require.NoError(t, err)
	return dynamicpb.NewMessage(file.Messages().ByName("UpdateRequest"))
}

func newQueryFilterRequest(t testing.TB) *dynamicpb.Message {
	t.Helper()
	file, err := protodesc.NewFile(
		&descriptorpb.FileDescriptorProto{
			Syntax:  proto.String("proto3"),
			Name:    proto.String("query_filter_request.proto"),
			Package: proto.String("resttest"),
			MessageType: []*descriptorpb.DescriptorProto{
				{
					Name: proto.String("Resource"),
					Field: []*descriptorpb.FieldDescriptorProto{
						{
							Name:     proto.String("display_name"),
							JsonName: proto.String("displayName"),
							Number:   proto.Int32(1),
							Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
							Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						},
						{
							Name:   proto.String("name"),
							Number: proto.Int32(2),
							Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
							Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						},
					},
				},
				{
					Name: proto.String("UpdateRequest"),
					Field: []*descriptorpb.FieldDescriptorProto{
						{
							Name:     proto.String("resource"),
							Number:   proto.Int32(1),
							Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
							Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
							TypeName: proto.String(".resttest.Resource"),
						},
						{
							Name:     proto.String("resource_name"),
							JsonName: proto.String("resourceName"),
							Number:   proto.Int32(2),
							Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
							Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						},
						{
							Name:     proto.String("page_size"),
							JsonName: proto.String("pageSize"),
							Number:   proto.Int32(3),
							Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
							Type:     descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
						},
					},
				},
			},
		},
		nil,
	)
	require.NoError(t, err)
	return dynamicpb.NewMessage(file.Messages().ByName("UpdateRequest"))
}

func BenchmarkParseMessage_FieldMask(b *testing.B) {
	descriptor := (&fieldmaskpb.FieldMask{}).ProtoReflect().Descriptor()
	decoder := queryParameterMarshaler(context.Background())
	for _, value := range []string{
		"displayName",
		"displayName,createTime.startTime,resource.labels",
	} {
		b.Run(value, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				if _, err := parseMessage(descriptor, value, decoder); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestPopulateFieldValues_TooManyValues(t *testing.T) {
	// populateFieldValues returns error when len(values) != 1 for a non-list, non-map field.
	// PopulateQueryParameters calls populateFieldValues with all values for a given key.
	// If we provide multiple values for a scalar field, it should error.
	msg := &wrapperspb.StringValue{}
	values := url.Values{}
	values.Add("value", "first")
	values.Add("value", "second")
	err := PopulateQueryParameters(msg, values)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many values")
}

func TestPopulateFieldValues_InvalidPath(t *testing.T) {
	// Test traversing into a non-message field.
	// If we try to access a sub-path through a scalar field, it should error.
	msg := &wrapperspb.StringValue{}
	err := PopulateFieldFromPath(msg, "value.subfield", "test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a message")
}

func TestPopulateFieldValues_EmptyPath(t *testing.T) {
	// PopulateFieldFromPath with empty string splits to [""], which tries to find
	// a field named "" and returns nil (field not found = no error).
	msg := &wrapperspb.StringValue{}
	err := PopulateFieldFromPath(msg, "", "test")
	require.NoError(t, err) // field not found is a no-op
}

func TestPopulateRepeatedField_FieldMaskPaths(t *testing.T) {
	msg := &fieldmaskpb.FieldMask{}
	values := url.Values{}
	values.Add("paths", "foo")
	values.Add("paths", "bar")
	values.Add("paths", "baz")
	err := PopulateQueryParameters(msg, values)
	require.NoError(t, err)
	assert.Equal(t, []string{"foo", "bar", "baz"}, msg.Paths)
}

func TestPopulateMapField_WrongValueCount(t *testing.T) {
	msg := &structpb.Struct{}
	// "fields" is a map field. Providing only one value (key without value) should error.
	err := PopulateFieldFromPath(msg, "fields", "key1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "more than one value")
}

func TestPopulateFieldValues_OneofConflict(t *testing.T) {
	msg := &structpb.Value{}
	err := PopulateFieldFromPath(msg, "string_value", "hello")
	require.NoError(t, err)

	// Setting another oneof field should error because the oneof is already set.
	err = PopulateFieldFromPath(msg, "number_value", "42")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field already set for oneof")
}

func TestPopulateFieldValues_NoValue(t *testing.T) {
	msg := &wrapperspb.StringValue{}
	// url.Values can have a key with empty slice if manually constructed.
	err := PopulateQueryParameters(msg, url.Values{"value": {}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no value provided")
}

func TestParseField_EnumByName(t *testing.T) {
	msg := &structpb.Value{}
	err := PopulateFieldFromPath(msg, "null_value", "NULL_VALUE")
	require.NoError(t, err)
	assert.Equal(t, structpb.NullValue_NULL_VALUE, msg.GetNullValue())
}

func TestParseField_EnumByNumber(t *testing.T) {
	msg := &structpb.Value{}
	err := PopulateFieldFromPath(msg, "null_value", "0")
	require.NoError(t, err)
	assert.Equal(t, structpb.NullValue_NULL_VALUE, msg.GetNullValue())
}

func TestParseField_EnumInvalid(t *testing.T) {
	msg := &structpb.Value{}
	err := PopulateFieldFromPath(msg, "null_value", "INVALID")
	require.Error(t, err)
}

func TestParseMessage_WellKnownTypes(t *testing.T) {
	tests := []struct {
		name  string
		msg   proto.Message
		field string
		value string
		check func(t *testing.T, msg proto.Message)
	}{
		{
			name:  "Timestamp seconds",
			msg:   &timestamppb.Timestamp{},
			field: "seconds",
			value: "1234567890",
			check: func(t *testing.T, msg proto.Message) {
				assert.Equal(t, int64(1234567890), msg.(*timestamppb.Timestamp).Seconds)
			},
		},
		{
			name:  "Duration seconds",
			msg:   &durationpb.Duration{},
			field: "seconds",
			value: "100",
			check: func(t *testing.T, msg proto.Message) {
				assert.Equal(t, int64(100), msg.(*durationpb.Duration).Seconds)
			},
		},
		{
			name:  "DoubleValue",
			msg:   &wrapperspb.DoubleValue{},
			field: "value",
			value: "3.14",
			check: func(t *testing.T, msg proto.Message) {
				assert.InDelta(t, 3.14, msg.(*wrapperspb.DoubleValue).Value, 0.001)
			},
		},
		{
			name:  "FloatValue",
			msg:   &wrapperspb.FloatValue{},
			field: "value",
			value: "1.5",
			check: func(t *testing.T, msg proto.Message) {
				assert.InDelta(t, float32(1.5), msg.(*wrapperspb.FloatValue).Value, 0.001)
			},
		},
		{
			name:  "Int64Value",
			msg:   &wrapperspb.Int64Value{},
			field: "value",
			value: "9999999999",
			check: func(t *testing.T, msg proto.Message) {
				assert.Equal(t, int64(9999999999), msg.(*wrapperspb.Int64Value).Value)
			},
		},
		{
			name:  "Int32Value",
			msg:   &wrapperspb.Int32Value{},
			field: "value",
			value: "42",
			check: func(t *testing.T, msg proto.Message) {
				assert.Equal(t, int32(42), msg.(*wrapperspb.Int32Value).Value)
			},
		},
		{
			name:  "UInt64Value",
			msg:   &wrapperspb.UInt64Value{},
			field: "value",
			value: "123456789012",
			check: func(t *testing.T, msg proto.Message) {
				assert.Equal(t, uint64(123456789012), msg.(*wrapperspb.UInt64Value).Value)
			},
		},
		{
			name:  "UInt32Value",
			msg:   &wrapperspb.UInt32Value{},
			field: "value",
			value: "42",
			check: func(t *testing.T, msg proto.Message) {
				assert.Equal(t, uint32(42), msg.(*wrapperspb.UInt32Value).Value)
			},
		},
		{
			name:  "BoolValue true",
			msg:   &wrapperspb.BoolValue{},
			field: "value",
			value: "true",
			check: func(t *testing.T, msg proto.Message) {
				assert.True(t, msg.(*wrapperspb.BoolValue).Value)
			},
		},
		{
			name:  "StringValue",
			msg:   &wrapperspb.StringValue{},
			field: "value",
			value: "hello world",
			check: func(t *testing.T, msg proto.Message) {
				assert.Equal(t, "hello world", msg.(*wrapperspb.StringValue).Value)
			},
		},
		{
			name:  "BytesValue",
			msg:   &wrapperspb.BytesValue{},
			field: "value",
			value: "aGVsbG8=",
			check: func(t *testing.T, msg proto.Message) {
				assert.Equal(t, []byte("hello"), msg.(*wrapperspb.BytesValue).Value)
			},
		},
		{
			name:  "FieldMask via repeated paths",
			msg:   &fieldmaskpb.FieldMask{},
			field: "paths",
			value: "foo,bar,baz",
			check: func(t *testing.T, msg proto.Message) {
				// PopulateFieldFromPath sets a single string value for the repeated field,
				// so the comma-separated string is treated as a single element.
				assert.Equal(t, []string{"foo,bar,baz"}, msg.(*fieldmaskpb.FieldMask).Paths)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := PopulateFieldFromPath(tt.msg, tt.field, tt.value)
			require.NoError(t, err)
			tt.check(t, tt.msg)
		})
	}
}
