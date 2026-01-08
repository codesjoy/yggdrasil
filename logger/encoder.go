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
	"time"

	"github.com/codesjoy/yggdrasil/v2/logger/buffer"
)

// TimeEncoder serializes a time.Time to a primitive type.
type TimeEncoder func(time.Time, PrimitiveEncoder)

// UnmarshalText unmarshals text to a TimeEncoder. "string" is unmarshaled to
func (e *TimeEncoder) UnmarshalText(text []byte) error {
	switch string(text) {
	case "second":
		*e = EpochTimeEncoder
	case "millis":
		*e = EpochMillisTimeEncoder
	case "nanos":
		*e = EpochNanosTimeEncoder
	case "RFC822":
		*e = TimeEncoderOfLayout(time.RFC822)
	case "RFC822Z":
		*e = TimeEncoderOfLayout(time.RFC822Z)
	case "RFC850":
		*e = TimeEncoderOfLayout(time.RFC850)
	case "RFC1123":
		*e = TimeEncoderOfLayout(time.RFC1123)
	case "RFC1123Z":
		*e = TimeEncoderOfLayout(time.RFC1123Z)
	case "RFC3339", "":
		*e = TimeEncoderOfLayout(time.RFC3339)
	case "RFC3339Nano":
		*e = TimeEncoderOfLayout(time.RFC3339Nano)
	default:
		*e = TimeEncoderOfLayout(string(text))
	}
	return nil
}

// EpochTimeEncoder serializes a time.Time to a floating-point number of seconds
// since the Unix epoch.
func EpochTimeEncoder(t time.Time, enc PrimitiveEncoder) {
	nanos := t.UnixNano()
	sec := float64(nanos) / float64(time.Second)
	enc.AppendFloat64(sec)
}

// EpochMillisTimeEncoder serializes a time.Time to a floating-point number of
// milliseconds since the Unix epoch.
func EpochMillisTimeEncoder(t time.Time, enc PrimitiveEncoder) {
	nanos := t.UnixNano()
	millis := float64(nanos) / float64(time.Millisecond)
	enc.AppendFloat64(millis)
}

// EpochNanosTimeEncoder serializes a time.Time to an integer number of
// nanoseconds since the Unix epoch.
func EpochNanosTimeEncoder(t time.Time, enc PrimitiveEncoder) {
	enc.AppendInt64(t.UnixNano())
}

// TimeEncoderOfLayout returns TimeEncoder which serializes a time.Time using
// given layout.
func TimeEncoderOfLayout(layout string) TimeEncoder {
	return func(t time.Time, enc PrimitiveEncoder) {
		enc.AppendString(t.Format(layout))
	}
}

// DurationEncoder serializes a time.Duration to a primitive type.
type DurationEncoder func(time.Duration, PrimitiveEncoder)

// UnmarshalText unmarshals text to a DurationEncoder. "string" is unmarshaled
// to StringDurationEncoder, and anything else is unmarshaled to
// NanosDurationEncoder.
func (e *DurationEncoder) UnmarshalText(text []byte) error {
	switch string(text) {
	case "string":
		*e = StringDurationEncoder
	case "nanos":
		*e = NanosDurationEncoder
	case "ms":
		*e = MillisDurationEncoder
	case "second":
		*e = SecondsDurationEncoder
	default:
		*e = SecondsDurationEncoder
	}
	return nil
}

// SecondsDurationEncoder serializes a time.Duration to a floating-point number of seconds elapsed.
func SecondsDurationEncoder(d time.Duration, enc PrimitiveEncoder) {
	enc.AppendFloat64(float64(d) / float64(time.Second))
}

// NanosDurationEncoder serializes a time.Duration to an integer number of
// nanoseconds elapsed.
func NanosDurationEncoder(d time.Duration, enc PrimitiveEncoder) {
	enc.AppendInt64(int64(d))
}

// MillisDurationEncoder serializes a time.Duration to an integer number of
// milliseconds elapsed.
func MillisDurationEncoder(d time.Duration, enc PrimitiveEncoder) {
	enc.AppendInt64(d.Nanoseconds() / 1e6)
}

// StringDurationEncoder serializes a time.Duration using its built-in String
// method.
func StringDurationEncoder(d time.Duration, enc PrimitiveEncoder) {
	enc.AppendString(d.String())
}

// PrimitiveEncoder is the interface that encoders must implement.
type PrimitiveEncoder interface {
	AppendBool(bool)
	AppendFloat64(float64)
	AppendInt64(int64)
	AppendString(string)
	AppendUint64(uint64)
}

// ObjectEncoder is the interface that encoders must implement.
type ObjectEncoder interface {
	AddBool(key string, value bool)
	AddDuration(key string, value time.Duration)
	AddFloat64(key string, value float64)
	AddInt64(key string, value int64)
	AddString(key, value string)
	AddTime(key string, value time.Time)
	AddUint64(key string, value uint64)
	AddAny(key string, val any)

	// OpenNamespace opens an isolated namespace where all subsequent fields will
	// be added.
	OpenNamespace(key string)
	// CloseNamespace closes the last n namespaces.
	CloseNamespace(n int)

	// SetBuffer set the buffer.
	SetBuffer(buffer *buffer.Buffer)

	// SetDurationEncoder set the duration encoder.
	SetDurationEncoder(de DurationEncoder)
	// SetTimeEncoder set the time encoder.
	SetTimeEncoder(te TimeEncoder)

	// Get and Free are used to manage the lifecycle of the attr encoder.
	Get() ObjectEncoder
	Free()
}
