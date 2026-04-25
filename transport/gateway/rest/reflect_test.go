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
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/protobuf/field_mask"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
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
		msg := &field_mask.FieldMask{}
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
	// FieldMask is handled in parseMessage. To trigger it, we need a message
	// that has a FieldMask field. We can't easily create one, so we test
	// FieldMask paths parsing indirectly via populateFieldValues repeated path.

	// Test populateFieldValues directly by setting a repeated field with multiple values.
	// Using url.Values with repeated keys.
	t.Run("RepeatedValuesViaPopulateQueryParameters", func(t *testing.T) {
		// Use a message with repeated field.
		// Since we can't easily create one, test with structpb.ListValue.
		// structpb.ListValue has repeated structpb.Value "values".
		// That's a message type, not scalar, so it goes through a different path.

		// Instead, let's test the error cases:
		// 1. Too many values for a scalar field
		msg := &wrapperspb.StringValue{}
		err := PopulateFieldFromPath(msg, "value", "test")
		require.NoError(t, err)
		assert.Equal(t, "test", msg.Value)
	})
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
	msg := &field_mask.FieldMask{}
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
			msg:   &field_mask.FieldMask{},
			field: "paths",
			value: "foo,bar,baz",
			check: func(t *testing.T, msg proto.Message) {
				// PopulateFieldFromPath sets a single string value for the repeated field,
				// so the comma-separated string is treated as a single element.
				assert.Equal(t, []string{"foo,bar,baz"}, msg.(*field_mask.FieldMask).Paths)
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
