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
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/v2/logger/buffer"
)

// mockPrimitiveEncoder is a mock implementation of PrimitiveEncoder for testing
type mockPrimitiveEncoder struct {
	buf *buffer.Buffer
}

func newMockPrimitiveEncoder() *mockPrimitiveEncoder {
	return &mockPrimitiveEncoder{buf: buffer.Get()}
}

func (m *mockPrimitiveEncoder) AppendBool(v bool) {
	m.buf.AppendBool(v)
}

func (m *mockPrimitiveEncoder) AppendFloat64(v float64) {
	m.buf.AppendFloat(v, 64)
}

func (m *mockPrimitiveEncoder) AppendInt64(v int64) {
	m.buf.AppendInt(v)
}

func (m *mockPrimitiveEncoder) AppendString(v string) {
	m.buf.AppendString(v)
}

func (m *mockPrimitiveEncoder) AppendUint64(v uint64) {
	m.buf.AppendUint(v)
}

func (m *mockPrimitiveEncoder) String() string {
	return m.buf.String()
}

func (m *mockPrimitiveEncoder) Bytes() []byte {
	return m.buf.Bytes()
}

func TestEpochTimeEncoder(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	enc := newMockPrimitiveEncoder()

	EpochTimeEncoder(now, enc)

	result := enc.String()
	if result == "" {
		t.Error("EpochTimeEncoder produced empty output")
	}
}

func TestEpochMillisTimeEncoder(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	enc := newMockPrimitiveEncoder()

	EpochMillisTimeEncoder(now, enc)

	result := enc.String()
	if result == "" {
		t.Error("EpochMillisTimeEncoder produced empty output")
	}
}

func TestEpochNanosTimeEncoder(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	enc := newMockPrimitiveEncoder()

	EpochNanosTimeEncoder(now, enc)

	result := enc.String()
	if result == "" {
		t.Error("EpochNanosTimeEncoder produced empty output")
	}
}

func TestTimeEncoderOfLayout(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 30, 45, 0, time.UTC)
	enc := newMockPrimitiveEncoder()

	encoder := TimeEncoderOfLayout(time.RFC3339)
	encoder(now, enc)

	result := enc.String()
	expected := "2024-01-01T12:30:45Z"
	if result != expected {
		t.Errorf("TimeEncoderOfLayout = %s, want %s", result, expected)
	}
}

func TestTimeEncoderUnmarshalText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		wantNil  bool
		testTime time.Time
		wantOut  string
	}{
		{"second", "second", false, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), "1704067200"},
		{"millis", "millis", false, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), "1704067200000"},
		{
			"nanos",
			"nanos",
			false,
			time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			"1704067200000000000",
		},
		{
			"RFC3339",
			"RFC3339",
			false,
			time.Date(2024, 1, 1, 12, 30, 45, 0, time.UTC),
			"2024-01-01T12:30:45Z",
		},
		{
			"RFC822",
			"RFC822",
			false,
			time.Date(2024, 1, 1, 12, 30, 45, 0, time.UTC),
			"01 Jan 24 12:30 UTC",
		},
		{
			"RFC822Z",
			"RFC822Z",
			false,
			time.Date(2024, 1, 1, 12, 30, 45, 0, time.UTC),
			"01 Jan 24 12:30 +0000",
		},
		{
			"RFC850",
			"RFC850",
			false,
			time.Date(2024, 1, 1, 12, 30, 45, 0, time.UTC),
			"Monday, 01-Jan-24 12:30:45 UTC",
		},
		{
			"RFC1123",
			"RFC1123",
			false,
			time.Date(2024, 1, 1, 12, 30, 45, 0, time.UTC),
			"Mon, 01 Jan 2024 12:30:45 UTC",
		},
		{
			"RFC1123Z",
			"RFC1123Z",
			false,
			time.Date(2024, 1, 1, 12, 30, 45, 0, time.UTC),
			"Mon, 01 Jan 2024 12:30:45 +0000",
		},
		{
			"RFC3339Nano",
			"RFC3339Nano",
			false,
			time.Date(2024, 1, 1, 12, 30, 45, 123456789, time.UTC),
			"2024-01-01T12:30:45.123456789Z",
		},
		{"custom", "2006-01-02", false, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), "2024-01-01"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var enc TimeEncoder
			err := enc.UnmarshalText([]byte(tt.text))
			if err != nil {
				t.Errorf("TimeEncoder.UnmarshalText() error = %v", err)
				return
			}

			mockEnc := newMockPrimitiveEncoder()
			enc(tt.testTime, mockEnc)

			result := mockEnc.String()
			if result != tt.wantOut {
				t.Errorf("TimeEncoder output = %s, want %s", result, tt.wantOut)
			}
		})
	}
}

func TestSecondsDurationEncoder(t *testing.T) {
	d := 5 * time.Second
	enc := newMockPrimitiveEncoder()

	SecondsDurationEncoder(d, enc)

	result := enc.String()
	expected := "5"
	if result != expected {
		t.Errorf("SecondsDurationEncoder = %s, want %s", result, expected)
	}
}

func TestNanosDurationEncoder(t *testing.T) {
	d := 5 * time.Second
	enc := newMockPrimitiveEncoder()

	NanosDurationEncoder(d, enc)

	result := enc.String()
	expected := "5000000000"
	if result != expected {
		t.Errorf("NanosDurationEncoder = %s, want %s", result, expected)
	}
}

func TestMillisDurationEncoder(t *testing.T) {
	d := 5 * time.Second
	enc := newMockPrimitiveEncoder()

	MillisDurationEncoder(d, enc)

	result := enc.String()
	expected := "5000"
	if result != expected {
		t.Errorf("MillisDurationEncoder = %s, want %s", result, expected)
	}
}

func TestStringDurationEncoder(t *testing.T) {
	d := 5*time.Second + 100*time.Millisecond
	enc := newMockPrimitiveEncoder()

	StringDurationEncoder(d, enc)

	result := enc.String()
	expected := "5.1s"
	if result != expected {
		t.Errorf("StringDurationEncoder = %s, want %s", result, expected)
	}
}

func TestDurationEncoderUnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		input   time.Duration
		wantOut string
	}{
		{"string", "string", 5 * time.Second, "5s"},
		{"nanos", "nanos", 5 * time.Second, "5000000000"},
		{"ms", "ms", 5 * time.Second, "5000"},
		{"default", "unknown", 5 * time.Second, "5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var enc DurationEncoder
			err := enc.UnmarshalText([]byte(tt.text))
			if err != nil {
				t.Errorf("DurationEncoder.UnmarshalText() error = %v", err)
				return
			}

			mockEnc := newMockPrimitiveEncoder()
			enc(tt.input, mockEnc)

			result := mockEnc.String()
			if result != tt.wantOut {
				t.Errorf("DurationEncoder output = %s, want %s", result, tt.wantOut)
			}
		})
	}
}
