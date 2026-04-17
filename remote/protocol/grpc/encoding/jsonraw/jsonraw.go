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

// Package jsonraw defines a JSON []byte passthrough codec for gRPC.
package jsonraw

import (
	"fmt"

	"github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc/encoding"

	gmem "google.golang.org/grpc/mem"
)

// Name is the registered name of the json raw codec.
const Name = "jsonraw"

func init() {
	encoding.RegisterCodec(codec{})
}

type codec struct{}

func (codec) Marshal(v interface{}) ([]byte, error) {
	b, ok := v.([]byte)
	if !ok {
		return nil, fmt.Errorf("failed to marshal, message is %T, want []byte", v)
	}
	return b, nil
}

func (codec) Unmarshal(data []byte, v interface{}) error {
	b, ok := v.(*[]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal, message is %T, want *[]byte", v)
	}
	*b = data
	return nil
}

func (codec) MarshalV2(v interface{}) (gmem.BufferSlice, error) {
	b, ok := v.([]byte)
	if !ok {
		return nil, fmt.Errorf("failed to marshal, message is %T, want []byte", v)
	}
	if len(b) == 0 {
		return nil, nil
	}
	data := b
	return gmem.BufferSlice{gmem.NewBuffer(&data, nil)}, nil
}

func (codec) UnmarshalV2(data gmem.BufferSlice, v interface{}) error {
	b, ok := v.(*[]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal, message is %T, want *[]byte", v)
	}
	*b = data.Materialize()
	return nil
}

func (codec) Name() string {
	return Name
}
