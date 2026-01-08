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

package main

import (
	"strings"
	"testing"

	"github.com/codesjoy/yggdrasil/v2/proto/codesjoy/yggdrasil/reason"
	"github.com/stretchr/testify/assert"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestReasons_Execute(t *testing.T) {
	reasons := &Reasons{
		Domain:      "test.package",
		CodePackage: "code",
		Reason: []ReasonWrapper{
			{
				Name: "ErrorReason",
				Codes: map[int32]code.Code{
					0: code.Code_OK,
					1: code.Code_CANCELLED,
				},
			},
		},
	}

	output := reasons.execute()

	expected := `var ErrorReason_code = map[int32]codeCode{
	0: codeCode_OK,
	1: codeCode_CANCELLED,
}

func (r ErrorReason) Reason() string {
	return ErrorReason_name[int32(r)]
}

func (r ErrorReason) Domain() string {
	return "test.package"
}

func (r ErrorReason) Code() codeCode {
	return ErrorReason_code[int32(r)]
}`

	assert.Equal(t, expected, output)
}

func TestGenerateFile(t *testing.T) {
	gen, err := protogen.Options{}.New(&pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{"test.proto"},
		ProtoFile: []*descriptorpb.FileDescriptorProto{
			{
				Name:    proto.String("test.proto"),
				Package: proto.String("test"),
				Options: &descriptorpb.FileOptions{
					GoPackage: proto.String(
						"github.com/codesjoy/yggdrasil/v2/cmd/protoc-gen-yggdrasil-reason;main",
					),
				},
				EnumType: []*descriptorpb.EnumDescriptorProto{
					{
						Name: proto.String("ErrorReason"),
						Value: []*descriptorpb.EnumValueDescriptorProto{
							{
								Name:    proto.String("OK"),
								Number:  proto.Int32(0),
								Options: &descriptorpb.EnumValueOptions{},
							},
						},
						Options: &descriptorpb.EnumOptions{},
					},
				},
			},
		},
	})
	assert.NoError(t, err)

	// Set extension for the enum
	proto.SetExtension(gen.Files[0].Enums[0].Desc.Options(), reason.E_DefaultReason, int32(1))
	// Set extension for the enum value
	proto.SetExtension(gen.Files[0].Enums[0].Values[0].Desc.Options(), reason.E_Code, code.Code_OK)

	file := gen.Files[0]
	generateFile(gen, file)

	// Check if file was generated
	generatedFile := ""
	for _, f := range gen.Response().File {
		if strings.HasSuffix(f.GetName(), "test_reason.pb.go") {
			generatedFile = f.GetName()
		}
	}
	assert.NotEmpty(t, generatedFile)
}

func TestGenerateFileContent_Skip(t *testing.T) {
	gen, err := protogen.Options{}.New(&pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{"test.proto"},
		ProtoFile: []*descriptorpb.FileDescriptorProto{
			{
				Name:    proto.String("test.proto"),
				Package: proto.String("test"),
				Options: &descriptorpb.FileOptions{
					GoPackage: proto.String(
						"github.com/codesjoy/yggdrasil/v2/cmd/protoc-gen-yggdrasil-reason;main",
					),
				},
			},
		},
	})
	assert.NoError(t, err)

	file := gen.Files[0]
	g := gen.NewGeneratedFile("test_reason.pb.go", file.GoImportPath)
	generateFileContent(gen, file, g)

	// Since there are no enums with the extension, it should be skipped
	// But we can verify it doesn't panic.
}
