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

package convert

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestConvertTimestamp(t *testing.T) {
	specs := []struct {
		name    string
		input   string
		output  *timestamppb.Timestamp
		wanterr bool
	}{
		{
			name:  "a valid RFC3339 timestamp",
			input: `"2016-05-10T10:19:13.123Z"`,
			output: &timestamppb.Timestamp{
				Seconds: 1462875553,
				Nanos:   123000000,
			},
			wanterr: false,
		},
		{
			name:  "a valid RFC3339 timestamp without double quotation",
			input: "2016-05-10T10:19:13.123Z",
			output: &timestamppb.Timestamp{
				Seconds: 1462875553,
				Nanos:   123000000,
			},
			wanterr: false,
		},
		{
			name:    "invalid timestamp",
			input:   `"05-10-2016T10:19:13.123Z"`,
			output:  nil,
			wanterr: true,
		},
		{
			name:    "JSON number",
			input:   "123",
			output:  nil,
			wanterr: true,
		},
		{
			name:    "JSON bool",
			input:   "true",
			output:  nil,
			wanterr: true,
		},
	}

	for _, spec := range specs {
		t.Run(spec.name, func(t *testing.T) {
			ts, err := Timestamp(spec.input)
			switch {
			case err != nil && !spec.wanterr:
				t.Errorf("got unexpected error\n%#v", err)
			case err == nil && spec.wanterr:
				t.Errorf("did not error when expecte")
			case !proto.Equal(ts, spec.output):
				t.Errorf(
					"when testing %s; got\n%#v\nexpected\n%#v",
					spec.name,
					ts,
					spec.output,
				)
			}
		})
	}
}

func TestString(t *testing.T) {
	got, err := String("hello")
	require.NoError(t, err)
	assert.Equal(t, "hello", got)
}

func TestStringSlice(t *testing.T) {
	got, err := StringSlice("a,b,c", ",")
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, got)
}

func TestBool(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		got, err := Bool("true")
		require.NoError(t, err)
		assert.True(t, got)
	})
	t.Run("false", func(t *testing.T) {
		got, err := Bool("false")
		require.NoError(t, err)
		assert.False(t, got)
	})
	t.Run("invalid", func(t *testing.T) {
		_, err := Bool("maybe")
		require.Error(t, err)
	})
}

func TestBoolSlice(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := BoolSlice("true,false,true", ",")
		require.NoError(t, err)
		assert.Equal(t, []bool{true, false, true}, got)
	})
	t.Run("invalid element", func(t *testing.T) {
		_, err := BoolSlice("true,maybe", ",")
		require.Error(t, err)
	})
}

func TestFloat64(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := Float64("3.14")
		require.NoError(t, err)
		assert.InDelta(t, 3.14, got, 0.001)
	})
	t.Run("invalid", func(t *testing.T) {
		_, err := Float64("abc")
		require.Error(t, err)
	})
}

func TestFloat64Slice(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := Float64Slice("1.1,2.2,3.3", ",")
		require.NoError(t, err)
		assert.InDelta(t, 1.1, got[0], 0.001)
		assert.InDelta(t, 2.2, got[1], 0.001)
		assert.InDelta(t, 3.3, got[2], 0.001)
	})
	t.Run("invalid element", func(t *testing.T) {
		_, err := Float64Slice("1.1,abc", ",")
		require.Error(t, err)
	})
}

func TestFloat32(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := Float32("3.14")
		require.NoError(t, err)
		assert.InDelta(t, float32(3.14), got, 0.001)
	})
	t.Run("invalid", func(t *testing.T) {
		_, err := Float32("abc")
		require.Error(t, err)
	})
}

func TestFloat32Slice(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := Float32Slice("1.1,2.2", ",")
		require.NoError(t, err)
		assert.Len(t, got, 2)
	})
	t.Run("invalid element", func(t *testing.T) {
		_, err := Float32Slice("1.1,abc", ",")
		require.Error(t, err)
	})
}

func TestInt64(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := Int64("42")
		require.NoError(t, err)
		assert.Equal(t, int64(42), got)
	})
	t.Run("invalid", func(t *testing.T) {
		_, err := Int64("abc")
		require.Error(t, err)
	})
}

func TestInt64Slice(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := Int64Slice("1,2,3", ",")
		require.NoError(t, err)
		assert.Equal(t, []int64{1, 2, 3}, got)
	})
	t.Run("invalid element", func(t *testing.T) {
		_, err := Int64Slice("1,abc", ",")
		require.Error(t, err)
	})
}

func TestInt32(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := Int32("42")
		require.NoError(t, err)
		assert.Equal(t, int32(42), got)
	})
	t.Run("invalid", func(t *testing.T) {
		_, err := Int32("abc")
		require.Error(t, err)
	})
}

func TestInt32Slice(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := Int32Slice("1,2,3", ",")
		require.NoError(t, err)
		assert.Equal(t, []int32{1, 2, 3}, got)
	})
	t.Run("invalid element", func(t *testing.T) {
		_, err := Int32Slice("1,abc", ",")
		require.Error(t, err)
	})
}

func TestUint64(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := Uint64("42")
		require.NoError(t, err)
		assert.Equal(t, uint64(42), got)
	})
	t.Run("invalid", func(t *testing.T) {
		_, err := Uint64("-1")
		require.Error(t, err)
	})
}

func TestUint64Slice(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := Uint64Slice("1,2,3", ",")
		require.NoError(t, err)
		assert.Equal(t, []uint64{1, 2, 3}, got)
	})
	t.Run("invalid element", func(t *testing.T) {
		_, err := Uint64Slice("1,-1", ",")
		require.Error(t, err)
	})
}

func TestUint32(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := Uint32("42")
		require.NoError(t, err)
		assert.Equal(t, uint32(42), got)
	})
	t.Run("invalid", func(t *testing.T) {
		_, err := Uint32("-1")
		require.Error(t, err)
	})
}

func TestUint32Slice(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := Uint32Slice("1,2,3", ",")
		require.NoError(t, err)
		assert.Equal(t, []uint32{1, 2, 3}, got)
	})
	t.Run("invalid element", func(t *testing.T) {
		_, err := Uint32Slice("1,-1", ",")
		require.Error(t, err)
	})
}

func TestBytes(t *testing.T) {
	t.Run("standard base64", func(t *testing.T) {
		encoded := base64.StdEncoding.EncodeToString([]byte("hello"))
		got, err := Bytes(encoded)
		require.NoError(t, err)
		assert.Equal(t, []byte("hello"), got)
	})
	t.Run("url base64", func(t *testing.T) {
		encoded := base64.URLEncoding.EncodeToString([]byte("world"))
		got, err := Bytes(encoded)
		require.NoError(t, err)
		assert.Equal(t, []byte("world"), got)
	})
	t.Run("invalid", func(t *testing.T) {
		_, err := Bytes("!!!invalid!!!")
		require.Error(t, err)
	})
}

func TestBytesSlice(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		enc1 := base64.StdEncoding.EncodeToString([]byte("aa"))
		enc2 := base64.StdEncoding.EncodeToString([]byte("bb"))
		got, err := BytesSlice(enc1+","+enc2, ",")
		require.NoError(t, err)
		assert.Equal(t, [][]byte{[]byte("aa"), []byte("bb")}, got)
	})
	t.Run("invalid element", func(t *testing.T) {
		_, err := BytesSlice("valid_base64,!!!invalid!!!", ",")
		require.Error(t, err)
	})
}

func TestEnum(t *testing.T) {
	enumMap := map[string]int32{"FOO": 1, "BAR": 2}
	t.Run("by name", func(t *testing.T) {
		got, err := Enum("FOO", enumMap)
		require.NoError(t, err)
		assert.Equal(t, int32(1), got)
	})
	t.Run("by numeric value", func(t *testing.T) {
		got, err := Enum("2", enumMap)
		require.NoError(t, err)
		assert.Equal(t, int32(2), got)
	})
	t.Run("invalid name", func(t *testing.T) {
		_, err := Enum("BAZ", enumMap)
		require.Error(t, err)
	})
	t.Run("invalid number", func(t *testing.T) {
		_, err := Enum("99", enumMap)
		require.Error(t, err)
	})
	t.Run("non-numeric non-name", func(t *testing.T) {
		_, err := Enum("abc", enumMap)
		require.Error(t, err)
	})
}

func TestEnumSlice(t *testing.T) {
	enumMap := map[string]int32{"FOO": 1, "BAR": 2}
	t.Run("valid", func(t *testing.T) {
		got, err := EnumSlice("FOO,BAR", ",", enumMap)
		require.NoError(t, err)
		assert.Equal(t, []int32{1, 2}, got)
	})
	t.Run("invalid element", func(t *testing.T) {
		_, err := EnumSlice("FOO,BAZ", ",", enumMap)
		require.Error(t, err)
	})
}

func TestStringValue(t *testing.T) {
	got, err := StringValue("hello")
	require.NoError(t, err)
	assert.Equal(t, "hello", got.Value)
}

func TestFloatValue(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := FloatValue("3.14")
		require.NoError(t, err)
		assert.InDelta(t, float32(3.14), got.Value, 0.001)
	})
	t.Run("invalid", func(t *testing.T) {
		_, err := FloatValue("abc")
		require.Error(t, err)
	})
}

func TestDoubleValue(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := DoubleValue("3.14")
		require.NoError(t, err)
		assert.InDelta(t, 3.14, got.Value, 0.001)
	})
	t.Run("invalid", func(t *testing.T) {
		_, err := DoubleValue("abc")
		require.Error(t, err)
	})
}

func TestBoolValue(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := BoolValue("true")
		require.NoError(t, err)
		assert.True(t, got.Value)
	})
	t.Run("invalid", func(t *testing.T) {
		_, err := BoolValue("maybe")
		require.Error(t, err)
	})
}

func TestInt32Value(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := Int32Value("42")
		require.NoError(t, err)
		assert.Equal(t, int32(42), got.Value)
	})
	t.Run("invalid", func(t *testing.T) {
		_, err := Int32Value("abc")
		require.Error(t, err)
	})
}

func TestUInt32Value(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := UInt32Value("42")
		require.NoError(t, err)
		assert.Equal(t, uint32(42), got.Value)
	})
	t.Run("invalid", func(t *testing.T) {
		_, err := UInt32Value("-1")
		require.Error(t, err)
	})
}

func TestInt64Value(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := Int64Value("42")
		require.NoError(t, err)
		assert.Equal(t, int64(42), got.Value)
	})
	t.Run("invalid", func(t *testing.T) {
		_, err := Int64Value("abc")
		require.Error(t, err)
	})
}

func TestUInt64Value(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := UInt64Value("42")
		require.NoError(t, err)
		assert.Equal(t, uint64(42), got.Value)
	})
	t.Run("invalid", func(t *testing.T) {
		_, err := UInt64Value("-1")
		require.Error(t, err)
	})
}

func TestBytesValue(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		encoded := base64.StdEncoding.EncodeToString([]byte("hello"))
		got, err := BytesValue(encoded)
		require.NoError(t, err)
		assert.Equal(t, []byte("hello"), got.Value)
	})
	t.Run("invalid", func(t *testing.T) {
		_, err := BytesValue("!!!invalid!!!")
		require.Error(t, err)
	})
}

func TestConvertDuration(t *testing.T) {
	specs := []struct {
		name    string
		input   string
		output  *durationpb.Duration
		wanterr bool
	}{
		{
			name:  "a valid duration",
			input: `"123.456s"`,
			output: &durationpb.Duration{
				Seconds: 123,
				Nanos:   456000000,
			},
			wanterr: false,
		},
		{
			name:  "a valid duration without double quotation",
			input: "123.456s",
			output: &durationpb.Duration{
				Seconds: 123,
				Nanos:   456000000,
			},
			wanterr: false,
		},
		{
			name:    "invalid duration",
			input:   `"123years"`,
			output:  nil,
			wanterr: true,
		},
		{
			name:    "JSON number",
			input:   "123",
			output:  nil,
			wanterr: true,
		},
		{
			name:    "JSON bool",
			input:   "true",
			output:  nil,
			wanterr: true,
		},
	}

	for _, spec := range specs {
		t.Run(spec.name, func(t *testing.T) {
			ts, err := Duration(spec.input)
			switch {
			case err != nil && !spec.wanterr:
				t.Errorf("got unexpected error\n%#v", err)
			case err == nil && spec.wanterr:
				t.Errorf("did not error when expecte")
			case !proto.Equal(ts, spec.output):
				t.Errorf(
					"when testing %s; got\n%#v\nexpected\n%#v",
					spec.name,
					ts,
					spec.output,
				)
			}
		})
	}
}
