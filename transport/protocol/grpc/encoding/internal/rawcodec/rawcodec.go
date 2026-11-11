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

// Package rawcodec provides the shared []byte passthrough gRPC codec.
package rawcodec

import (
	"fmt"

	gmem "google.golang.org/grpc/mem"
)

// Codec is a named []byte passthrough codec.
type Codec struct {
	name string
}

// New creates a []byte passthrough codec with the given registered name.
func New(name string) Codec {
	return Codec{name: name}
}

// Marshal returns the input bytes unchanged.
func (Codec) Marshal(value any) ([]byte, error) {
	data, ok := value.([]byte)
	if !ok {
		return nil, fmt.Errorf("failed to marshal, message is %T, want []byte", value)
	}
	return data, nil
}

// Unmarshal copies the input bytes into the target.
func (Codec) Unmarshal(data []byte, target any) error {
	out, ok := target.(*[]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal, message is %T, want *[]byte", target)
	}
	*out = data
	return nil
}

// MarshalV2 returns the input bytes as a gRPC buffer slice.
func (Codec) MarshalV2(value any) (gmem.BufferSlice, error) {
	data, ok := value.([]byte)
	if !ok {
		return nil, fmt.Errorf("failed to marshal, message is %T, want []byte", value)
	}
	if len(data) == 0 {
		return nil, nil
	}
	return gmem.BufferSlice{gmem.NewBuffer(&data, nil)}, nil
}

// UnmarshalV2 materializes a gRPC buffer slice into the target.
func (Codec) UnmarshalV2(data gmem.BufferSlice, target any) error {
	out, ok := target.(*[]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal, message is %T, want *[]byte", target)
	}
	*out = data.Materialize()
	return nil
}

// Name returns the registered codec name.
func (c Codec) Name() string {
	return c.name
}
