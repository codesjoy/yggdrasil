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
	"fmt"
	"math"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/codesjoy/yggdrasil/v2/logger/buffer"
)

// For JSON-escaping; see jsonHandler.safeAddString below.
const _hex = "0123456789abcdef"

var _jsonPool = sync.Pool{New: func() interface{} {
	return &jsonEncoder{}
}}

func getJSONEncoder() *jsonEncoder {
	return _jsonPool.Get().(*jsonEncoder)
}

func putJSONEncoder(enc *jsonEncoder) {
	enc.buf = nil
	enc.spaced = false
	_jsonPool.Put(enc)
}

type jsonEncoder struct {
	encodeTime     TimeEncoder
	encodeDuration DurationEncoder

	spaced bool // include spaces after colons and commas

	buf *buffer.Buffer
}

// JSONEncoderConfig is the configuration of jsonEncoder.
type JSONEncoderConfig struct {
	TimeEncoder     TimeEncoder     `yaml:"timeEncoder"     json:"timeEncoder"`
	DurationEncoder DurationEncoder `yaml:"durationEncoder" json:"durationEncoder"`
	Spaced          bool            `yaml:"spaced"          json:"spaced"`
}

// NewJSONEncoder returns a new jsonEncoder.
func NewJSONEncoder(cfg *JSONEncoderConfig) (ObjectEncoder, error) {
	enc := &jsonEncoder{
		spaced:         cfg.Spaced,
		encodeTime:     cfg.TimeEncoder,
		encodeDuration: cfg.DurationEncoder,
	}
	return enc, nil
}

func (enc *jsonEncoder) Get() ObjectEncoder {
	newEnc := getJSONEncoder()
	newEnc.encodeTime = enc.encodeTime
	newEnc.encodeDuration = enc.encodeDuration
	newEnc.spaced = enc.spaced
	return enc
}

func (enc *jsonEncoder) Free() {
	putJSONEncoder(enc)
}

func (enc *jsonEncoder) SetBuffer(buf *buffer.Buffer) {
	enc.buf = buf
}

func (enc *jsonEncoder) SetDurationEncoder(de DurationEncoder) {
	enc.encodeDuration = de
}

func (enc *jsonEncoder) SetTimeEncoder(te TimeEncoder) {
	enc.encodeTime = te
}

func (enc *jsonEncoder) AddBool(key string, val bool) {
	enc.addKey(key)
	enc.AppendBool(val)
}

func (enc *jsonEncoder) AddDuration(key string, val time.Duration) {
	enc.addKey(key)
	enc.appendDuration(val)
}

func (enc *jsonEncoder) AddFloat64(key string, val float64) {
	enc.addKey(key)
	enc.AppendFloat64(val)
}

func (enc *jsonEncoder) AddInt64(key string, val int64) {
	enc.addKey(key)
	enc.AppendInt64(val)
}

func (enc *jsonEncoder) AddString(key, val string) {
	enc.addKey(key)
	enc.AppendString(val)
}

func (enc *jsonEncoder) AddTime(key string, val time.Time) {
	enc.addKey(key)
	enc.appendTime(val)
}

func (enc *jsonEncoder) AddUint64(key string, val uint64) {
	enc.addKey(key)
	enc.AppendUint64(val)
}

func (enc *jsonEncoder) AddAny(key string, val any) {
	enc.addKey(key)
	if err := enc.appendAny(val); err != nil {
		enc.AddString(fmt.Sprintf("%sError", key), err.Error())
	}
}

func (enc *jsonEncoder) OpenNamespace(key string) {
	enc.addKey(key)
	enc.buf.AppendByte('{')
}

func (enc *jsonEncoder) CloseNamespace(num int) {
	for i := 0; i < num; i++ {
		enc.buf.AppendByte('}')
	}
}

func (enc *jsonEncoder) AppendBool(val bool) {
	enc.addElementSeparator()
	enc.buf.AppendBool(val)
}

func (enc *jsonEncoder) AppendInt64(val int64) {
	enc.addElementSeparator()
	enc.buf.AppendInt(val)
}

func (enc *jsonEncoder) AppendString(val string) {
	enc.addElementSeparator()
	enc.buf.AppendByte('"')
	enc.safeAddString(val)
	enc.buf.AppendByte('"')
}

func (enc *jsonEncoder) AppendUint64(val uint64) {
	enc.addElementSeparator()
	enc.buf.AppendUint(val)
}

func (enc *jsonEncoder) AppendFloat64(val float64) {
	enc.addElementSeparator()
	switch {
	case math.IsNaN(val):
		enc.buf.AppendString(`"NaN"`)
	case math.IsInf(val, 1):
		enc.buf.AppendString(`"+Inf"`)
	case math.IsInf(val, -1):
		enc.buf.AppendString(`"-Inf"`)
	default:
		enc.buf.AppendFloat(val, 64)
	}
}

func (enc *jsonEncoder) appendDuration(val time.Duration) {
	cur := enc.buf.Len()
	if e := enc.encodeDuration; e != nil {
		e(val, enc)
	}
	if cur == enc.buf.Len() {
		// User-supplied encodeDuration is a no-op. Fall back to nanoseconds to keep
		// JSON valid.
		enc.AppendInt64(int64(val))
	}
}

func (enc *jsonEncoder) appendTime(val time.Time) {
	cur := enc.buf.Len()
	if e := enc.encodeTime; e != nil {
		e(val, enc)
	}
	if cur == enc.buf.Len() {
		// User-supplied encodeTime is a no-op. Fall back to nanos since epoch to keep
		// output JSON valid.
		enc.AppendInt64(val.UnixNano())
	}
}

func (enc *jsonEncoder) appendAny(val any) error {
	enc.addElementSeparator()
	if val == nil {
		enc.AppendString("null")
		return nil
	}

	_, ok := val.(json.Marshaler)
	if !ok {
		enc.AppendString(fmt.Sprintf("%v", val))
		return nil
	}

	e := json.NewEncoder(enc.buf)
	e.SetEscapeHTML(false)
	if err := e.Encode(val); err != nil {
		return err
	}
	return nil
}

func (enc *jsonEncoder) addKey(key string) {
	enc.addElementSeparator()
	enc.buf.AppendByte('"')
	enc.safeAddString(key)
	enc.buf.AppendByte('"')
	enc.buf.AppendByte(':')
	if enc.spaced {
		enc.buf.AppendByte(' ')
	}
}

func (enc *jsonEncoder) addElementSeparator() {
	last := enc.buf.Len() - 1
	if last < 0 {
		return
	}
	switch enc.buf.Bytes()[last] {
	case '{', '[', ':', ',', ' ':
		return
	default:
		enc.buf.AppendByte(',')
		if enc.spaced {
			enc.buf.AppendByte(' ')
		}
	}
}

// safeAddString JSON-escapes a string and appends it to the internal buffer.
// Unlike the standard library's encoder, it doesn't attempt to protect the
// user from browser vulnerabilities or JSONP-related problems.
func (enc *jsonEncoder) safeAddString(s string) {
	for i := 0; i < len(s); {
		if enc.tryAddRuneSelf(s[i]) {
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if enc.tryAddRuneError(r, size) {
			i++
			continue
		}
		enc.buf.AppendString(s[i : i+size])
		i += size
	}
}

// safeAddByteString is no-alloc equivalent of safeAddString(string(s)) for s []byte.
// nolint:unused
func (enc *jsonEncoder) safeAddByteString(s []byte) {
	for i := 0; i < len(s); {
		if enc.tryAddRuneSelf(s[i]) {
			i++
			continue
		}
		r, size := utf8.DecodeRune(s[i:])
		if enc.tryAddRuneError(r, size) {
			i++
			continue
		}
		enc.buf.AppendBytes(s[i : i+size])
		i += size
	}
}

// tryAddRuneSelf appends b if it is valid UTF-8 character represented in a single byte.
func (enc *jsonEncoder) tryAddRuneSelf(b byte) bool {
	if b >= utf8.RuneSelf {
		return false
	}
	if 0x20 <= b && b != '\\' && b != '"' {
		enc.buf.AppendByte(b)
		return true
	}
	switch b {
	case '\\', '"':
		enc.buf.AppendByte('\\')
		enc.buf.AppendByte(b)
	case '\n':
		enc.buf.AppendByte('\\')
		enc.buf.AppendByte('n')
	case '\r':
		enc.buf.AppendByte('\\')
		enc.buf.AppendByte('r')
	case '\t':
		enc.buf.AppendByte('\\')
		enc.buf.AppendByte('t')
	default:
		// Encode bytes < 0x20, except for the escape sequences above.
		enc.buf.AppendString(`\u00`)
		enc.buf.AppendByte(_hex[b>>4])
		enc.buf.AppendByte(_hex[b&0xF])
	}
	return true
}

func (enc *jsonEncoder) tryAddRuneError(r rune, size int) bool {
	if r == utf8.RuneError && size == 1 {
		enc.buf.AppendString(`\ufffd`)
		return true
	}
	return false
}
