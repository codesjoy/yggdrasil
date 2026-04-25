/*
 *
 * Copyright 2017 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

// Package encoding defines the interface for the compressor and codec, and
// functions to register and retrieve compressors and codecs.
//
// # Experimental
//
// Notice: This package is EXPERIMENTAL and may be changed or removed in a
// later release.
package encoding

import (
	"io"
	"strings"

	grpcencoding "google.golang.org/grpc/encoding"
	gmem "google.golang.org/grpc/mem"
)

// Identity specifies the optional encoding for uncompressed streams.
// It is intended for grpc internal use only.
const Identity = grpcencoding.Identity

// Compressor is used for compressing and decompressing when sending or
// receiving messages.
type Compressor interface {
	// Compress writes the data written to wc to w after compressing it.  If an
	// reason occurs while initializing the compressor, that reason is returned
	// instead.
	Compress(w io.Writer) (io.WriteCloser, error)
	// Decompress reads data from r, decompresses it, and provides the
	// uncompressed data via the returned io.Reader.  If an reason occurs while
	// initializing the decompressor, that reason is returned instead.
	Decompress(r io.Reader) (io.Reader, error)
	// Name is the name of the compression codec and is used to set the content
	// coding header.  The result must be static; the result cannot change
	// between calls.
	Name() string
	// If a Compressor implements
	// DecompressedSize(compressedBytes []byte) int, gRPC will call it
	// to determine the size of the buffer allocated for the result of decompression.
	// Return -1 to indicate unknown size.
	//
	// Experimental
	//
	// Notice: This API is EXPERIMENTAL and may be changed or removed in a
	// later release.
}

// RegisterCompressor registers the compressor with gRPC by its name.  It can
// be activated when sending an RPC via grpc.UseCompressor().  It will be
// automatically accessed when receiving a message based on the content coding
// header.  Servers also use it to send a response with the same encoding as
// the request.
//
// NOTE: this function must only be called during initialization time (i.e. in
// an init() function), and is not thread-safe.  If multiple Compressors are
// registered with the same name, the one registered last will take effect.
func RegisterCompressor(c Compressor) {
	grpcencoding.RegisterCompressor(c)
}

// GetCompressor returns Compressor for the given compressor name.
func GetCompressor(name string) Compressor {
	return grpcencoding.GetCompressor(name)
}

// Codec defines the interface gRPC uses to encode and decode messages.  Note
// that implementations of this interface must be thread safe; a Codec's
// methods can be called from concurrent goroutines.
type Codec interface {
	// Marshal returns the wire format of v.
	Marshal(v interface{}) ([]byte, error)
	// Unmarshal parses the wire format into v.
	Unmarshal(data []byte, v interface{}) error
	// Name returns the name of the Codec implementation. The returned string
	// will be used as part of content type in transmission.  The result must be
	// static; the result cannot change between calls.
	Name() string
}

type codecV2 interface {
	Codec
	MarshalV2(v interface{}) (gmem.BufferSlice, error)
	UnmarshalV2(data gmem.BufferSlice, v interface{}) error
}

// RegisterCodec registers the provided Codec for use with all gRPC clients and
// servers.
//
// The Codec will be stored and looked up by result of its Name() method, which
// should match the content-subtype of the encoding handled by the Codec.  This
// is case-insensitive, and is stored and looked up as lowercase.  If the
// result of calling Name() is an empty string, RegisterCodec will panic. See
// Content-Type on
// https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md#requests for
// more details.
//
// NOTE: this function must only be called during initialization time (i.e. in
// an init() function), and is not thread-safe.  If multiple Compressors are
// registered with the same name, the one registered last will take effect.
func RegisterCodec(codec Codec) {
	if codec == nil {
		panic("cannot register a nil Codec")
	}
	if codec.Name() == "" {
		panic("cannot register Codec with empty string result for Name()")
	}
	grpcencoding.RegisterCodecV2(asGRPCCodecV2(codec))
}

// GetCodec gets a registered Codec by content-subtype, or nil if no Codec is
// registered for the content-subtype.
//
// The content-subtype is expected to be lowercase.
func GetCodec(contentSubtype string) Codec {
	contentSubtype = strings.ToLower(contentSubtype)
	if codec := grpcencoding.GetCodec(contentSubtype); codec != nil {
		return codec
	}
	if codec := grpcencoding.GetCodecV2(contentSubtype); codec != nil {
		return grpcCodecV1Bridge{codec: codec}
	}
	return nil
}

func asGRPCCodecV2(codec Codec) grpcencoding.CodecV2 {
	if codec == nil {
		return nil
	}
	return grpcCodecBridge{codec: codec}
}

type grpcCodecBridge struct {
	codec Codec
}

func (b grpcCodecBridge) Name() string {
	return b.codec.Name()
}

func (b grpcCodecBridge) Marshal(v interface{}) (gmem.BufferSlice, error) {
	if codec, ok := b.codec.(codecV2); ok {
		return codec.MarshalV2(v)
	}
	data, err := b.codec.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	buf := data
	return gmem.BufferSlice{gmem.NewBuffer(&buf, nil)}, nil
}

func (b grpcCodecBridge) Unmarshal(data gmem.BufferSlice, v interface{}) error {
	if codec, ok := b.codec.(codecV2); ok {
		return codec.UnmarshalV2(data, v)
	}
	if len(data) == 0 {
		return b.codec.Unmarshal(nil, v)
	}
	if len(data) == 1 {
		return b.codec.Unmarshal(data[0].ReadOnlyData(), v)
	}
	return b.codec.Unmarshal(data.Materialize(), v)
}

type grpcCodecV1Bridge struct {
	codec grpcencoding.CodecV2
}

func (b grpcCodecV1Bridge) Name() string {
	return b.codec.Name()
}

func (b grpcCodecV1Bridge) Marshal(v interface{}) ([]byte, error) {
	data, err := b.codec.Marshal(v)
	if err != nil {
		return nil, err
	}
	defer data.Free()
	if len(data) == 0 {
		return nil, nil
	}
	return data.Materialize(), nil
}

func (b grpcCodecV1Bridge) Unmarshal(data []byte, v interface{}) error {
	if len(data) == 0 {
		return b.codec.Unmarshal(nil, v)
	}
	buf := data
	bufferSlice := gmem.BufferSlice{gmem.NewBuffer(&buf, nil)}
	defer bufferSlice.Free()
	return b.codec.Unmarshal(bufferSlice, v)
}
