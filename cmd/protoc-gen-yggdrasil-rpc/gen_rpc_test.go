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

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestServiceDesc_Execute(t *testing.T) {
	sd := &serviceDesc{
		ServiceType:           "Greeter",
		ServiceName:           "helloworld.Greeter",
		LowerFirstServiceType: "greeter",
		Context:               "context.Context",
		Client:                "grpc.ClientConnInterface",
		Md:                    "metadata",
		Server:                "grpc.Server",
		Interceptor:           "grpc",
		Status:                "status",
		Code:                  "codes",
		Stream:                "grpc",
		FullServerName:        "helloworld.Greeter",
		Filename:              "helloworld.proto",
		NeedStream:            true,
		Methods: []*methodDesc{
			{
				Name:         "SayHello",
				Input:        "HelloRequest",
				Output:       "HelloResponse",
				ClientStream: false,
				ServerStream: false,
			},
			{
				Name:         "StreamHello",
				Input:        "HelloRequest",
				Output:       "HelloResponse",
				ClientStream: true,
				ServerStream: true,
			},
		},
	}

	output := sd.execute(tpl)

	assert.Contains(t, output, "type GreeterClient interface")
	assert.Contains(t, output, "SayHello(context.Context, *HelloRequest) (*HelloResponse, error)")
	assert.Contains(t, output, "StreamHello(context.Context) (GreeterStreamHelloClient, error)")
	assert.Contains(t, output, "type GreeterServer interface")
	assert.Contains(t, output, "var GreeterServiceDesc = grpc.ServerServiceDesc")
}

func TestGenerateFiles(t *testing.T) {
	gen, err := protogen.Options{}.New(&pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{"test.proto"},
		ProtoFile: []*descriptorpb.FileDescriptorProto{
			{
				Name:    proto.String("test.proto"),
				Package: proto.String("test"),
				Options: &descriptorpb.FileOptions{
					GoPackage: proto.String(
						"github.com/codesjoy/yggdrasil/v2/cmd/protoc-gen-yggdrasil-rpc;main",
					),
				},
				Service: []*descriptorpb.ServiceDescriptorProto{
					{
						Name:    proto.String("Greeter"),
						Options: &descriptorpb.ServiceOptions{},
						Method: []*descriptorpb.MethodDescriptorProto{
							{
								Name:       proto.String("SayHello"),
								InputType:  proto.String(".test.HelloRequest"),
								OutputType: proto.String(".test.HelloResponse"),
							},
						},
					},
				},
				MessageType: []*descriptorpb.DescriptorProto{
					{Name: proto.String("HelloRequest")},
					{Name: proto.String("HelloResponse")},
				},
			},
		},
	})
	assert.NoError(t, err)

	// Test with deprecated service
	gen.Files[0].Services[0].Desc.Options().(*descriptorpb.ServiceOptions).Deprecated = proto.Bool(
		true,
	)

	generateFiles(gen, gen.Files[0])

	// Check if file was generated
	generatedFile := ""
	for _, f := range gen.Response().File {
		if strings.HasSuffix(f.GetName(), "test_rpc.pb.go") {
			generatedFile = f.GetName()
		}
	}
	assert.NotEmpty(t, generatedFile)
}

func TestGenerateFiles_Complex(t *testing.T) {
	gen, err := protogen.Options{}.New(&pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{"test.proto"},
		ProtoFile: []*descriptorpb.FileDescriptorProto{
			{
				Name:    proto.String("test.proto"),
				Package: proto.String("test"),
				Options: &descriptorpb.FileOptions{
					GoPackage: proto.String(
						"github.com/codesjoy/yggdrasil/v2/cmd/protoc-gen-yggdrasil-rpc;main",
					),
				},
				Service: []*descriptorpb.ServiceDescriptorProto{
					{
						Name:    proto.String("Greeter"),
						Options: &descriptorpb.ServiceOptions{},
						Method: []*descriptorpb.MethodDescriptorProto{
							{
								Name:       proto.String("SayHello"),
								InputType:  proto.String(".test.HelloRequest"),
								OutputType: proto.String(".test.HelloResponse"),
							},
							{
								Name:            proto.String("StreamHello"),
								InputType:       proto.String(".test.HelloRequest"),
								OutputType:      proto.String(".test.HelloResponse"),
								ClientStreaming: proto.Bool(true),
								ServerStreaming: proto.Bool(true),
							},
						},
					},
				},
				MessageType: []*descriptorpb.DescriptorProto{
					{Name: proto.String("HelloRequest")},
					{Name: proto.String("HelloResponse")},
				},
			},
		},
	})
	assert.NoError(t, err)

	generateFiles(gen, gen.Files[0])

	// Check if file was generated
	generatedFile := ""
	for _, f := range gen.Response().File {
		if strings.HasSuffix(f.GetName(), "test_rpc.pb.go") {
			generatedFile = f.GetName()
		}
	}
	assert.NotEmpty(t, generatedFile)
}

func TestGenerateFiles_NoServices(t *testing.T) {
	gen, err := protogen.Options{}.New(&pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{"test.proto"},
		ProtoFile: []*descriptorpb.FileDescriptorProto{
			{
				Name:    proto.String("test.proto"),
				Package: proto.String("test"),
				Options: &descriptorpb.FileOptions{
					GoPackage: proto.String(
						"github.com/codesjoy/yggdrasil/v2/cmd/protoc-gen-yggdrasil-rpc;main",
					),
				},
			},
		},
	})
	assert.NoError(t, err)

	generateFiles(gen, gen.Files[0])

	// No file should be generated
	assert.Empty(t, gen.Response().File)
}
