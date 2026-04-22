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

package grpc

import (
	"strings"

	grpcencoding "google.golang.org/grpc/encoding"
	gmem "google.golang.org/grpc/mem"

	yencoding "github.com/codesjoy/yggdrasil/v3/remote/protocol/grpc/encoding"
)

type localCodecV2 interface {
	yencoding.Codec
	MarshalV2(v interface{}) (gmem.BufferSlice, error)
	UnmarshalV2(data gmem.BufferSlice, v interface{}) error
}

type officialCodecV1Bridge struct {
	codec grpcencoding.Codec
}

func (b officialCodecV1Bridge) Name() string {
	return b.codec.Name()
}

func (b officialCodecV1Bridge) Marshal(v interface{}) (gmem.BufferSlice, error) {
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

func (b officialCodecV1Bridge) Unmarshal(data gmem.BufferSlice, v interface{}) error {
	if len(data) == 0 {
		return b.codec.Unmarshal(nil, v)
	}
	if len(data) == 1 {
		return b.codec.Unmarshal(data[0].ReadOnlyData(), v)
	}
	return b.codec.Unmarshal(data.Materialize(), v)
}

type localCodecV2Bridge struct {
	codec yencoding.Codec
}

func (b localCodecV2Bridge) Name() string {
	return b.codec.Name()
}

func (b localCodecV2Bridge) Marshal(v interface{}) (gmem.BufferSlice, error) {
	if codec, ok := b.codec.(localCodecV2); ok {
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

func (b localCodecV2Bridge) Unmarshal(data gmem.BufferSlice, v interface{}) error {
	if codec, ok := b.codec.(localCodecV2); ok {
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

func grpcCodecV2BySubtype(contentSubtype string) grpcencoding.CodecV2 {
	contentSubtype = strings.ToLower(contentSubtype)
	if contentSubtype == "" {
		return nil
	}
	if codec := grpcencoding.GetCodecV2(contentSubtype); codec != nil {
		return codec
	}
	if codec := grpcencoding.GetCodec(contentSubtype); codec != nil {
		return officialCodecV1Bridge{codec: codec}
	}
	return nil
}

func grpcCodecV2ForLocal(codec yencoding.Codec) grpcencoding.CodecV2 {
	if codec == nil {
		return nil
	}
	if codec.Name() != "" {
		if registered := grpcCodecV2BySubtype(codec.Name()); registered != nil {
			return registered
		}
	}
	return localCodecV2Bridge{codec: codec}
}
