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
	"errors"
	"io"

	"google.golang.org/protobuf/proto"
)

func init() {
	RegisterMarshallerBuilder("proto", func() (Marshaler, error) {
		return &ProtoMarshaller{}, nil
	})
}

// ProtoMarshaller is a Marshaller which marshals/unmarshals into/from serialize proto bytes
type ProtoMarshaller struct{}

// ContentType always returns "application/octet-stream".
func (*ProtoMarshaller) ContentType(_ interface{}) string {
	return "application/octet-stream"
}

// Marshal marshals "value" into Proto
func (*ProtoMarshaller) Marshal(value interface{}) ([]byte, error) {
	message, ok := value.(proto.Message)
	if !ok {
		return nil, errors.New("unable to marshal non proto field")
	}
	return proto.Marshal(message)
}

// Unmarshal unmarshals proto "data" into "value"
func (*ProtoMarshaller) Unmarshal(data []byte, value interface{}) error {
	message, ok := value.(proto.Message)
	if !ok {
		return errors.New("unable to unmarshal non proto field")
	}
	return proto.Unmarshal(data, message)
}

// NewDecoder returns a Decoder which reads proto stream from "reader".
func (marshaller *ProtoMarshaller) NewDecoder(reader io.Reader) Decoder {
	return DecoderFunc(func(value interface{}) error {
		buffer, err := io.ReadAll(reader)
		if err != nil {
			return err
		}
		return marshaller.Unmarshal(buffer, value)
	})
}

// NewEncoder returns an Encoder which writes proto stream into "writer".
func (marshaller *ProtoMarshaller) NewEncoder(writer io.Writer) Encoder {
	return EncoderFunc(func(value interface{}) error {
		buffer, err := marshaller.Marshal(value)
		if err != nil {
			return err
		}
		if _, err := writer.Write(buffer); err != nil {
			return err
		}

		return nil
	})
}
