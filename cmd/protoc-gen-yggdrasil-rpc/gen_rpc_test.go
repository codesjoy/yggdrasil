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
	"sort"
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
		HasUnaryMethods:       true,
		Methods: []*methodDesc{
			{
				Name:         "SayHello",
				Input:        "HelloRequest",
				Output:       "HelloResponse",
				IsUnary:      true,
				ClientStream: false,
				ServerStream: false,
			},
			{
				Name:               "StreamHello",
				Input:              "HelloRequest",
				Output:             "HelloResponse",
				ClientStream:       true,
				ServerStream:       true,
				IsBidi:             true,
				StreamIndex:        0,
				IsClientStreamOnly: false,
				IsServerStreamOnly: false,
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
	assert.Empty(t, gen.Response().File)
}

func TestGenerateFiles_UnaryOnly_HasInterceptorImport(t *testing.T) {
	content := generateRPCContent(t, newService("Greeter",
		newMethod("SayHello", "HelloRequest", "HelloResponse", false, false),
	))

	assert.Contains(t, content, "interceptor \"github.com/codesjoy/yggdrasil/v2/interceptor\"")
	assert.Contains(t, content, "UnaryServerInterceptor")
	assert.Contains(t, content, "SayHello(context.Context, *HelloRequest) (*HelloResponse, error)")
}

func TestGenerateFiles_StreamingOnly_NoInterceptorImport(t *testing.T) {
	content := generateRPCContent(t, newService("Greeter",
		newMethod("Bidi", "BidiRequest", "BidiResponse", true, true),
		newMethod("Upload", "UploadRequest", "UploadResponse", true, false),
		newMethod("Watch", "WatchRequest", "WatchResponse", false, true),
	))

	assert.NotContains(t, content, "interceptor \"github.com/codesjoy/yggdrasil/v2/interceptor\"")
	assert.NotContains(t, content, "UnaryServerInterceptor")
}

func TestGenerateFiles_ClientStreamOnly_HasCloseAndRecv(t *testing.T) {
	content := generateRPCContent(t, newService("Greeter",
		newMethod("Upload", "UploadRequest", "UploadResponse", true, false),
	))

	assert.Contains(t, content, "Upload(context.Context) (GreeterUploadClient, error)")
	assert.Contains(t, content, "type GreeterUploadClient interface {")
	assert.Contains(t, content, "Send(*UploadRequest) error")
	assert.Contains(t, content, "CloseAndRecv() (*UploadResponse, error)")
	assert.Contains(
		t,
		content,
		"func (x *greeterUploadClient) CloseAndRecv() (*UploadResponse, error)",
	)
	assert.NotContains(t, content, "func (x *greeterUploadClient) Recv() (*UploadResponse, error)")
}

func TestGenerateFiles_ServerStreamOnly_NoCloseAndRecv(t *testing.T) {
	content := generateRPCContent(t, newService("Greeter",
		newMethod("Watch", "WatchRequest", "WatchResponse", false, true),
	))

	assert.Contains(t, content, "Watch(context.Context, *WatchRequest) (GreeterWatchClient, error)")
	assert.Contains(t, content, "type GreeterWatchClient interface {")
	assert.Contains(t, content, "Recv() (*WatchResponse, error)")
	assert.NotContains(t, content, "CloseAndRecv() (*WatchResponse, error)")
	assert.NotContains(t, content, "func (x *greeterWatchClient) Send(m *WatchRequest) error")
}

func TestGenerateFiles_StreamIndexMatchesDescriptorOrder(t *testing.T) {
	content := generateRPCContent(t, newService("Greeter",
		newMethod("ServerFirst", "ServerFirstRequest", "ServerFirstResponse", false, true),
		newMethod("UnaryMiddle", "UnaryMiddleRequest", "UnaryMiddleResponse", false, false),
		newMethod("BidiThird", "BidiThirdRequest", "BidiThirdResponse", true, true),
		newMethod("ClientFourth", "ClientFourthRequest", "ClientFourthResponse", true, false),
		newMethod("ServerFifth", "ServerFifthRequest", "ServerFifthResponse", false, true),
	))

	assert.Contains(
		t,
		content,
		"NewStream(ctx, &GreeterServiceDesc.Streams[0], \"/test.Greeter/ServerFirst\")",
	)
	assert.Contains(
		t,
		content,
		"NewStream(ctx, &GreeterServiceDesc.Streams[1], \"/test.Greeter/BidiThird\")",
	)
	assert.Contains(
		t,
		content,
		"NewStream(ctx, &GreeterServiceDesc.Streams[2], \"/test.Greeter/ClientFourth\")",
	)
	assert.Contains(
		t,
		content,
		"NewStream(ctx, &GreeterServiceDesc.Streams[3], \"/test.Greeter/ServerFifth\")",
	)
}

func generateRPCContent(t *testing.T, services ...*descriptorpb.ServiceDescriptorProto) string {
	t.Helper()

	gen := newTestPlugin(t, services...)
	generateFiles(gen, gen.Files[0])

	for _, f := range gen.Response().File {
		if strings.HasSuffix(f.GetName(), "test_rpc.pb.go") {
			return f.GetContent()
		}
	}
	t.Fatalf("no generated rpc file found")
	return ""
}

func newTestPlugin(
	t *testing.T,
	services ...*descriptorpb.ServiceDescriptorProto,
) *protogen.Plugin {
	t.Helper()

	gen, err := protogen.Options{}.New(&pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{"test.proto"},
		ProtoFile: []*descriptorpb.FileDescriptorProto{
			{
				Name:        proto.String("test.proto"),
				Package:     proto.String("test"),
				MessageType: collectMessageTypes(services...),
				Options: &descriptorpb.FileOptions{
					GoPackage: proto.String(
						"github.com/codesjoy/yggdrasil/v2/cmd/protoc-gen-yggdrasil-rpc;main",
					),
				},
				Service: services,
			},
		},
	})
	assert.NoError(t, err)
	return gen
}

func collectMessageTypes(
	services ...*descriptorpb.ServiceDescriptorProto,
) []*descriptorpb.DescriptorProto {
	names := make(map[string]struct{})
	for _, service := range services {
		for _, method := range service.Method {
			in := protoTypeName(method.GetInputType())
			if in != "" {
				names[in] = struct{}{}
			}
			out := protoTypeName(method.GetOutputType())
			if out != "" {
				names[out] = struct{}{}
			}
		}
	}

	keys := make([]string, 0, len(names))
	for name := range names {
		keys = append(keys, name)
	}
	sort.Strings(keys)

	messages := make([]*descriptorpb.DescriptorProto, 0, len(keys))
	for _, name := range keys {
		messages = append(messages, &descriptorpb.DescriptorProto{Name: proto.String(name)})
	}
	return messages
}

func protoTypeName(full string) string {
	if full == "" {
		return ""
	}
	lastDot := strings.LastIndex(full, ".")
	if lastDot == -1 || lastDot+1 >= len(full) {
		return full
	}
	return full[lastDot+1:]
}

func newService(
	name string,
	methods ...*descriptorpb.MethodDescriptorProto,
) *descriptorpb.ServiceDescriptorProto {
	return &descriptorpb.ServiceDescriptorProto{
		Name:    proto.String(name),
		Options: &descriptorpb.ServiceOptions{},
		Method:  methods,
	}
}

func newMethod(
	name string,
	input string,
	output string,
	clientStreaming bool,
	serverStreaming bool,
) *descriptorpb.MethodDescriptorProto {
	method := &descriptorpb.MethodDescriptorProto{
		Name:       proto.String(name),
		InputType:  proto.String(".test." + input),
		OutputType: proto.String(".test." + output),
	}
	if clientStreaming {
		method.ClientStreaming = proto.Bool(true)
	}
	if serverStreaming {
		method.ServerStreaming = proto.Bool(true)
	}
	return method
}
