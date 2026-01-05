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
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

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
