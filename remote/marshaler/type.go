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
	"io"
	"net/http"
)

// Marshaler defines a conversion between byte sequence and RPC payloads / fields.
type Marshaler interface {
	// Marshal marshals "v" into byte sequence.
	Marshal(v interface{}) ([]byte, error)
	// Unmarshal unmarshals "data" into "v".
	// "v" must be a pointer value.
	Unmarshal(data []byte, v interface{}) error
	// NewDecoder returns a Decoder which reads byte sequence from "r".
	NewDecoder(r io.Reader) Decoder
	// NewEncoder returns an Encoder which writes bytes sequence into "w".
	NewEncoder(w io.Writer) Encoder
	// ContentType returns the Content-Type which this marshaler is responsible for.
	// The parameter describes the type which is being marshalled, which can sometimes
	// affect the content type returned.
	ContentType(v interface{}) string
}

// Decoder decodes a byte sequence
type Decoder interface {
	Decode(v interface{}) error
}

// Encoder encodes RPC payloads / fields into byte sequence.
type Encoder interface {
	Encode(v interface{}) error
}

// DecoderFunc adapts an decoder function into Decoder.
type DecoderFunc func(v interface{}) error

// Decode delegates invocations to the underlying function itself.
func (f DecoderFunc) Decode(v interface{}) error { return f(v) }

// EncoderFunc adapts an encoder function into Encoder
type EncoderFunc func(v interface{}) error

// Encode delegates invocations to the underlying function itself.
func (f EncoderFunc) Encode(v interface{}) error { return f(v) }

// MarshallerBuilder builds a marshaler for a given request.
type MarshallerBuilder func() (Marshaler, error)

// Registry defines a registry for marshaler.
type Registry interface {
	GetMarshaler(r *http.Request) (inbound Marshaler, outbound Marshaler)
}
