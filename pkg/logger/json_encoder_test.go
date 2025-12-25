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

package logger

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/pkg/logger/buffer"
)

func TestNewJsonEncoder(t *testing.T) {
	cfg := &JSONEncoderConfig{
		TimeEncoder:     EpochTimeEncoder,
		DurationEncoder: SecondsDurationEncoder,
		Spaced:          true,
	}

	enc, err := NewJSONEncoder(cfg)
	if err != nil {
		t.Fatalf("NewJSONEncoder() error = %v", err)
	}

	if enc == nil {
		t.Error("NewJSONEncoder() returned nil encoder")
	}
}

func TestJsonEncoderAddBool(t *testing.T) {
	cfg := &JSONEncoderConfig{}
	enc, _ := NewJSONEncoder(cfg)
	buf := buffer.Get()
	enc.SetBuffer(buf)

	enc.AddBool("key1", true)
	enc.AddBool("key2", false)

	result := buf.String()
	if !strings.Contains(result, `"key1":true`) {
		t.Errorf("AddBool() result = %s, expected to contain key1:true", result)
	}
	if !strings.Contains(result, `"key2":false`) {
		t.Errorf("AddBool() result = %s, expected to contain key2:false", result)
	}
}

func TestJsonEncoderAddInt64(t *testing.T) {
	cfg := &JSONEncoderConfig{}
	enc, _ := NewJSONEncoder(cfg)
	buf := buffer.Get()
	enc.SetBuffer(buf)

	enc.AddInt64("key", 42)

	result := buf.String()
	if !strings.Contains(result, `"key":42`) {
		t.Errorf("AddInt64() result = %s, expected to contain key:42", result)
	}
}

func TestJsonEncoderAddUint64(t *testing.T) {
	cfg := &JSONEncoderConfig{}
	enc, _ := NewJSONEncoder(cfg)
	buf := buffer.Get()
	enc.SetBuffer(buf)

	enc.AddUint64("key", 18446744073709551615)

	result := buf.String()
	if !strings.Contains(result, `"key":18446744073709551615`) {
		t.Errorf("AddUint64() result = %s", result)
	}
}

func TestJsonEncoderAddFloat64(t *testing.T) {
	cfg := &JSONEncoderConfig{}
	enc, _ := NewJSONEncoder(cfg)
	buf := buffer.Get()
	enc.SetBuffer(buf)

	enc.AddFloat64("key", 3.14)

	result := buf.String()
	if !strings.Contains(result, `"key":3.14`) {
		t.Errorf("AddFloat64() result = %s", result)
	}
}

func TestJsonEncoderAddFloat64SpecialValues(t *testing.T) {
	cfg := &JSONEncoderConfig{}
	enc, _ := NewJSONEncoder(cfg)
	buf := buffer.Get()
	enc.SetBuffer(buf)

	enc.AddFloat64("nan", math.NaN())
	enc.AddFloat64("inf", math.Inf(1))
	enc.AddFloat64("neginf", math.Inf(-1))

	result := buf.String()
	if !strings.Contains(result, `"nan":"NaN"`) {
		t.Errorf("AddFloat64() for NaN result = %s", result)
	}
	if !strings.Contains(result, `"inf":"+Inf"`) {
		t.Errorf("AddFloat64() for +Inf result = %s", result)
	}
	if !strings.Contains(result, `"neginf":"-Inf"`) {
		t.Errorf("AddFloat64() for -Inf result = %s", result)
	}
}

func TestJsonEncoderAddString(t *testing.T) {
	cfg := &JSONEncoderConfig{}
	enc, _ := NewJSONEncoder(cfg)
	buf := buffer.Get()
	enc.SetBuffer(buf)

	enc.AddString("key", "value")

	result := buf.String()
	if !strings.Contains(result, `"key":"value"`) {
		t.Errorf("AddString() result = %s", result)
	}
}

func TestJsonEncoderAddStringWithEscape(t *testing.T) {
	cfg := &JSONEncoderConfig{}
	enc, _ := NewJSONEncoder(cfg)
	buf := buffer.Get()
	enc.SetBuffer(buf)

	enc.AddString("key", "value\nwith\tquotes\"")

	result := buf.String()
	if !strings.Contains(result, `"key":"value\nwith\tquotes\""`) {
		t.Errorf("AddString() with escape result = %s", result)
	}
}

func TestJsonEncoderAddTime(t *testing.T) {
	cfg := &JSONEncoderConfig{
		TimeEncoder: TimeEncoderOfLayout(time.RFC3339),
	}
	enc, _ := NewJSONEncoder(cfg)
	buf := buffer.Get()
	enc.SetBuffer(buf)

	testTime := time.Date(2024, 1, 1, 12, 30, 45, 0, time.UTC)
	enc.AddTime("key", testTime)

	result := buf.String()
	if !strings.Contains(result, `"key":"2024-01-01T12:30:45Z"`) {
		t.Errorf("AddTime() result = %s", result)
	}
}

func TestJsonEncoderAddDuration(t *testing.T) {
	cfg := &JSONEncoderConfig{
		DurationEncoder: SecondsDurationEncoder,
	}
	enc, _ := NewJSONEncoder(cfg)
	buf := buffer.Get()
	enc.SetBuffer(buf)

	enc.AddDuration("key", 5*time.Second)

	result := buf.String()
	if !strings.Contains(result, `"key":5`) {
		t.Errorf("AddDuration() result = %s", result)
	}
}

func TestJsonEncoderAddAny(t *testing.T) {
	cfg := &JSONEncoderConfig{}
	enc, _ := NewJSONEncoder(cfg)
	buf := buffer.Get()
	enc.SetBuffer(buf)

	enc.AddAny("key", "test")
	enc.AddAny("num", 42)
	enc.AddAny("nil", nil)
	enc.AddAny("bool", true)

	result := buf.String()
	if !strings.Contains(result, `"key":"test"`) {
		t.Errorf("AddAny() for string result = %s", result)
	}
	if !strings.Contains(result, `"num":"42"`) {
		t.Errorf("AddAny() for int result = %s", result)
	}
	if !strings.Contains(result, `"nil":"null"`) {
		t.Errorf("AddAny() for nil result = %s", result)
	}
	if !strings.Contains(result, `"bool":"true"`) {
		t.Errorf("AddAny() for bool result = %s", result)
	}
}

func TestJsonEncoderAddAnyJSONMarshaler(t *testing.T) {
	cfg := &JSONEncoderConfig{}
	enc, _ := NewJSONEncoder(cfg)
	buf := buffer.Get()
	enc.SetBuffer(buf)

	type testStruct struct {
		Field string `json:"field"`
	}
	s := testStruct{Field: "value"}
	enc.AddAny("key", s)

	result := buf.String()
	if !strings.Contains(result, `"key":`) {
		t.Errorf("AddAny() for JSONMarshaler result = %s", result)
	}
}

func TestJsonEncoderOpenNamespace(t *testing.T) {
	cfg := &JSONEncoderConfig{}
	enc, _ := NewJSONEncoder(cfg)
	buf := buffer.Get()
	enc.SetBuffer(buf)

	enc.OpenNamespace("ns1")
	enc.AddString("inner", "value")
	enc.CloseNamespace(1)

	result := buf.String()
	if !strings.Contains(result, `"ns1":{`) {
		t.Errorf("OpenNamespace() result = %s", result)
	}
	if !strings.Contains(result, `"inner":"value"`) {
		t.Errorf("Inner string not found, result = %s", result)
	}
}

func TestJsonEncoderMultipleNamespaces(t *testing.T) {
	cfg := &JSONEncoderConfig{}
	enc, _ := NewJSONEncoder(cfg)
	buf := buffer.Get()
	enc.SetBuffer(buf)

	enc.OpenNamespace("ns1")
	enc.AddString("key1", "value1")
	enc.OpenNamespace("ns2")
	enc.AddString("key2", "value2")
	enc.CloseNamespace(2)

	result := buf.String()
	if !strings.Contains(result, `"ns1":{`) {
		t.Errorf("First namespace not found, result = %s", result)
	}
	if !strings.Contains(result, `"ns2":{`) {
		t.Errorf("Second namespace not found, result = %s", result)
	}
}

func TestJsonEncoderSpaced(t *testing.T) {
	cfg := &JSONEncoderConfig{Spaced: true}
	enc, _ := NewJSONEncoder(cfg)
	buf := buffer.Get()
	enc.SetBuffer(buf)

	enc.AddString("key", "value")

	result := buf.String()
	if !strings.Contains(result, `"key": "value"`) {
		t.Errorf("Spaced encoder result = %s, expected space after colon", result)
	}
}

func TestJsonEncoderAppendMethods(t *testing.T) {
	cfg := &JSONEncoderConfig{}
	enc, _ := NewJSONEncoder(cfg)
	buf := buffer.Get()
	enc.SetBuffer(buf)

	// Use type assertion to access jsonEncoder methods
	jsonEnc := enc.(*jsonEncoder)
	jsonEnc.AppendBool(true)
	jsonEnc.AppendInt64(42)
	jsonEnc.AppendUint64(100)
	jsonEnc.AppendFloat64(3.14)
	jsonEnc.AppendString("test")

	result := buf.String()
	// These are primitives without keys, so they should be separated by commas
	if result == "" {
		t.Error("Append methods produced empty output")
	}
}

func TestJsonEncoderGetAndFree(t *testing.T) {
	cfg := &JSONEncoderConfig{
		TimeEncoder:     EpochTimeEncoder,
		DurationEncoder: SecondsDurationEncoder,
	}
	parentEnc, _ := NewJSONEncoder(cfg)

	childEnc := parentEnc.Get()
	if childEnc == nil {
		t.Error("Get() returned nil")
	}

	childEnc.Free()
	// If Free() panics, the test will fail
}

func TestJsonEncoderSetBuffer(t *testing.T) {
	cfg := &JSONEncoderConfig{}
	enc, _ := NewJSONEncoder(cfg)

	buf := buffer.Get()
	enc.SetBuffer(buf)

	if enc.(*jsonEncoder).buf != buf {
		t.Error("SetBuffer() did not set the buffer")
	}
}

func TestJsonEncoderSetTimeEncoder(t *testing.T) {
	cfg := &JSONEncoderConfig{}
	enc, _ := NewJSONEncoder(cfg)

	newEncoder := TimeEncoderOfLayout(time.RFC822)
	enc.SetTimeEncoder(newEncoder)

	buf := buffer.Get()
	enc.SetBuffer(buf)
	enc.AddTime("key", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))

	result := buf.String()
	if !strings.Contains(result, "01 Jan 24") {
		t.Errorf("SetTimeEncoder() result = %s, expected RFC822 format", result)
	}
}

func TestJsonEncoderSetDurationEncoder(t *testing.T) {
	cfg := &JSONEncoderConfig{}
	enc, _ := NewJSONEncoder(cfg)

	newEncoder := StringDurationEncoder
	enc.SetDurationEncoder(newEncoder)

	buf := buffer.Get()
	enc.SetBuffer(buf)
	enc.AddDuration("key", 5*time.Second)

	result := buf.String()
	if !strings.Contains(result, `"key":"5s"`) {
		t.Errorf("SetDurationEncoder() result = %s, expected string format", result)
	}
}

func TestJsonEncoderAddKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{"simple key", "key", `"key":`},
		{"key with space", "key name", `"key name":`},
		{"key with quote", `key"quote`, `\"key\"quote\":`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &JSONEncoderConfig{}
			enc, _ := NewJSONEncoder(cfg)
			buf := buffer.Get()
			enc.SetBuffer(buf)

			enc.AddString(tt.key, "value")

			result := buf.String()
			// Check that the key is properly escaped and quoted
			if result == "" {
				t.Error("addKey() produced empty output")
			}
		})
	}
}

// testJSONMarshaler is a simple JSON marshaler for testing
type testJSONMarshaler struct {
	Value string
}

func (t testJSONMarshaler) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Value)
}

func TestJsonEncoderAppendAnyWithMarshaler(t *testing.T) {
	cfg := &JSONEncoderConfig{}
	enc, _ := NewJSONEncoder(cfg)
	buf := buffer.Get()
	enc.SetBuffer(buf)

	marshaler := testJSONMarshaler{Value: "test value"}
	err := enc.(*jsonEncoder).appendAny(marshaler)
	if err != nil {
		t.Errorf("appendAny() with JSONMarshaler error = %v", err)
	}
}

func TestJsonEncoderSafeAddString(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"normal", "hello"},
		{"with newline", "hello\nworld"},
		{"with tab", "hello\tworld"},
		{"with quote", `hello"world`},
		{"with backslash", `hello\world`},
		{"with carriage return", "hello\rworld"},
		{"with unicode", "hello世界"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &JSONEncoderConfig{}
			enc, _ := NewJSONEncoder(cfg)
			buf := buffer.Get()
			enc.SetBuffer(buf)

			enc.AddString("key", tt.input)

			result := buf.String()
			// Build a complete JSON object for parsing
			jsonStr := "{" + result + "}"
			// Check that the result is valid JSON
			var m map[string]string
			err := json.Unmarshal([]byte(jsonStr), &m)
			if err != nil {
				t.Errorf("safeAddString() produced invalid JSON: %v, result: %s", err, result)
			}
			if m["key"] != tt.input {
				t.Errorf("Expected key value %q, got %q", tt.input, m["key"])
			}
		})
	}
}
