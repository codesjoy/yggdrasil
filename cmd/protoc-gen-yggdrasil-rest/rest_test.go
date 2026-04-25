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
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestServiceDesc_Execute(t *testing.T) {
	sd := &serviceDesc{
		HTTPPkg:        "http",
		ChiPkg:         "chi",
		MarshalerPkg:   "marshaler",
		StatusPkg:      "status",
		RestPkg:        "rest",
		SvrPkg:         "svr",
		CodePkg:        "code",
		InterceptorPkg: "interceptor",
		CtxPkg:         "context",
		IoPkg:          "io",
		ServiceType:    "Greeter",
		ServiceName:    "helloworld.Greeter",
		Methods: []*methodDesc{
			{
				Name:    "SayHello",
				Num:     0,
				Method:  "POST",
				Request: "HelloRequest",
				Path:    "/v1/greeter/say_hello",
				HasBody: true,
			},
		},
	}

	output := sd.execute()

	assert.Contains(t, output, "func local_handler_Greeter_SayHello_0")
	assert.Contains(t, output, "protoReq := &HelloRequest{}")
	assert.Contains(t, output, "var GreeterRestServiceDesc = svrRestServiceDesc")
	assert.Contains(t, output, `Method: "POST"`)
	assert.Contains(t, output, `Path: "/v1/greeter/say_hello"`)
}

func TestBuildPathVars_ProducesRenderableBindings(t *testing.T) {
	sd := &serviceDesc{
		ChiPkg: "chi.",
	}

	tests := []struct {
		path     string
		expected string
	}{
		{"{name}", `chi.URLParam(r, "params0")`},
		{"{name=*}", `chi.URLParam(r, "params0")`},
		{"/v1/{name}", `chi.URLParam(r, "params0")`},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			_, bindings, err := buildPathVars(tt.path)
			assert.NoError(t, err)
			if assert.Len(t, bindings, 1) {
				output := sd.renderPathValue(bindings[0].Segments)
				assert.Contains(t, output, tt.expected)
			}
		})
	}
}

func TestCamelCaseVars(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"name", "Name"},
		{"user_id", "UserId"},
		{"user.first_name", "User.FirstName"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, camelCaseVars(tt.input))
	}
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
						"github.com/codesjoy/yggdrasil/v3/cmd/protoc-gen-yggdrasil-rest;main",
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
								Options:    &descriptorpb.MethodOptions{},
							},
						},
					},
					{
						Name: proto.String("EmptyService"),
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

	// Set HTTP rule extension with additional bindings and custom method
	rule := &annotations.HttpRule{
		Pattern: &annotations.HttpRule_Post{
			Post: "/v1/greeter/say_hello",
		},
		Body: "*",
		AdditionalBindings: []*annotations.HttpRule{
			{
				Pattern: &annotations.HttpRule_Get{
					Get: "/v1/greeter/say_hello/{name}",
				},
			},
			{
				Pattern: &annotations.HttpRule_Custom{
					Custom: &annotations.CustomHttpPattern{
						Kind: "CUSTOM",
						Path: "/v1/greeter/custom",
					},
				},
				Body: "*",
			},
		},
	}
	proto.SetExtension(gen.Files[0].Services[0].Methods[0].Desc.Options(), annotations.E_Http, rule)

	// Mark service as deprecated
	gen.Files[0].Services[0].Desc.Options().(*descriptorpb.ServiceOptions).Deprecated = proto.Bool(
		true,
	)

	err = generateFiles(gen, gen.Files[0])
	assert.NoError(t, err)

	// Check if file was generated
	generatedFile := ""
	for _, f := range gen.Response().File {
		if strings.HasSuffix(f.GetName(), "test_rest.pb.go") {
			generatedFile = f.GetName()
		}
	}
	assert.NotEmpty(t, generatedFile)
}

func TestBuildPathVars(t *testing.T) {
	tests := []struct {
		path     string
		expected string
		vars     []pathVarBinding
		wantErr  string
	}{
		{
			path:     "/v1/{name}",
			expected: "/v1/{params0}",
			vars: []pathVarBinding{
				pathBinding("name", paramSegment("params0")),
			},
		},
		{
			path:     "/v1/{name=*}",
			expected: "/v1/{params0}",
			vars: []pathVarBinding{
				pathBinding("name", paramSegment("params0")),
			},
		},
		{
			path:     "/v1/{name=users/*}",
			expected: "/v1/users/{params1}",
			vars: []pathVarBinding{
				pathBinding("name", literalSegment("users"), paramSegment("params1")),
			},
		},
		{
			path:     "/v1/{name=users/*/items/*}",
			expected: "/v1/users/{params1}/items/{params2}",
			vars: []pathVarBinding{
				pathBinding("name",
					literalSegment("users"),
					paramSegment("params1"),
					literalSegment("items"),
					paramSegment("params2"),
				),
			},
		},
		{
			path:     "/v1/{name=organizations/*/settings}",
			expected: "/v1/organizations/{params1}/settings",
			vars: []pathVarBinding{
				pathBinding("name",
					literalSegment("organizations"),
					paramSegment("params1"),
					literalSegment("settings"),
				),
			},
		},
		{
			path:     "/v1/{name=organizations/*/applications/*/settings}",
			expected: "/v1/organizations/{params1}/applications/{params2}/settings",
			vars: []pathVarBinding{
				pathBinding("name",
					literalSegment("organizations"),
					paramSegment("params1"),
					literalSegment("applications"),
					paramSegment("params2"),
					literalSegment("settings"),
				),
			},
		},
		{
			path:     "/v1/{name=a/b/c}",
			expected: "/v1/a/b/c",
			vars: []pathVarBinding{
				pathBinding("name",
					literalSegment("a"),
					literalSegment("b"),
					literalSegment("c"),
				),
			},
		},
		{
			path:     "/v1/{name=a/b}",
			expected: "/v1/a/b",
			vars: []pathVarBinding{
				pathBinding("name",
					literalSegment("a"),
					literalSegment("b"),
				),
			},
		},
		{
			path:     "/v1/{name=a/*/b/*/c}",
			expected: "/v1/a/{params1}/b/{params2}/c",
			vars: []pathVarBinding{
				pathBinding("name",
					literalSegment("a"),
					paramSegment("params1"),
					literalSegment("b"),
					paramSegment("params2"),
					literalSegment("c"),
				),
			},
		},
		{
			path:     "/v1/{parent=organizations/*}/settings/{name}",
			expected: "/v1/organizations/{params1}/settings/{params2}",
			vars: []pathVarBinding{
				pathBinding("parent",
					literalSegment("organizations"),
					paramSegment("params1"),
				),
				pathBinding("name", paramSegment("params2")),
			},
		},
		{
			path:    "/v1/{na me}",
			wantErr: `invalid binding "{na me}"`,
		},
		{
			path:    "/v1/{ resource.name =organizations/*}",
			wantErr: `invalid binding "{ resource.name =organizations/*}"`,
		},
		{
			path:    "/v1/{name=/**}",
			wantErr: "contains an empty path segment",
		},
		{
			path:    "/v1/{name=/a/b/}",
			wantErr: "contains an empty path segment",
		},
		{
			path:    "/v1/{name=a/**/b}",
			wantErr: `unsupported multi-segment wildcard "**"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			path, vars, err := buildPathVars(tt.path)
			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, path)
			assert.Equal(t, tt.vars, vars)
		})
	}
}

func TestServiceDesc_RenderPathValue(t *testing.T) {
	sd := &serviceDesc{
		ChiPkg: "chi.",
	}

	tests := []struct {
		name     string
		segments []pathBindingSegment
		expected string
	}{
		{
			name:     "single param",
			segments: []pathBindingSegment{paramSegment("params0")},
			expected: `chi.URLParam(r, "params0")`,
		},
		{
			name: "literal and params",
			segments: []pathBindingSegment{
				literalSegment("users"),
				paramSegment("params0"),
				literalSegment("items"),
				paramSegment("params1"),
			},
			expected: `"users/" + chi.URLParam(r, "params0") + "/items/" + chi.URLParam(r, "params1")`,
		},
		{
			name: "singleton path",
			segments: []pathBindingSegment{
				literalSegment("organizations"),
				paramSegment("params1"),
				literalSegment("settings"),
			},
			expected: `"organizations/" + chi.URLParam(r, "params1") + "/settings"`,
		},
		{
			name: "nested singleton path",
			segments: []pathBindingSegment{
				literalSegment("organizations"),
				paramSegment("params1"),
				literalSegment("applications"),
				paramSegment("params2"),
				literalSegment("settings"),
			},
			expected: `"organizations/" + chi.URLParam(r, "params1") + "/applications/" + chi.URLParam(r, "params2") + "/settings"`,
		},
		{
			name: "fully literal",
			segments: []pathBindingSegment{
				literalSegment("a"),
				literalSegment("b"),
				literalSegment("c"),
			},
			expected: `"a/b/c"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := sd.renderPathValue(tt.segments)
			assert.Equal(t, tt.expected, output)
		})
	}
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
						"github.com/codesjoy/yggdrasil/v3/cmd/protoc-gen-yggdrasil-rest;main",
					),
				},
			},
		},
	})
	assert.NoError(t, err)

	err = generateFiles(gen, gen.Files[0])
	assert.NoError(t, err)

	// No file should be generated
	assert.Empty(t, gen.Response().File)
}

func TestBuildHTTPRule_Errors(t *testing.T) {
	gen, err := protogen.Options{}.New(&pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{"test.proto"},
		ProtoFile: []*descriptorpb.FileDescriptorProto{
			{
				Name:    proto.String("test.proto"),
				Package: proto.String("test"),
				Options: &descriptorpb.FileOptions{
					GoPackage: proto.String(
						"github.com/codesjoy/yggdrasil/v3/cmd/protoc-gen-yggdrasil-rest;main",
					),
				},
				MessageType: []*descriptorpb.DescriptorProto{
					{Name: proto.String("HelloRequest")},
				},
			},
		},
	})
	assert.NoError(t, err)

	g := gen.NewGeneratedFile("test.go", gen.Files[0].GoImportPath)
	m := &protogen.Method{
		GoName: "SayHello",
		Input:  gen.Files[0].Messages[0],
	}

	// Test GET with body
	rule := &annotations.HttpRule{
		Pattern: &annotations.HttpRule_Get{
			Get: "/v1/test",
		},
		Body: "some_body",
	}
	_, err = buildHTTPRule(g, m, rule)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "body should not be declared")

	// Test POST without body
	rule = &annotations.HttpRule{
		Pattern: &annotations.HttpRule_Post{
			Post: "/v1/test",
		},
		Body: "",
	}
	_, err = buildHTTPRule(g, m, rule)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not declare a body")

	rule = &annotations.HttpRule{
		Pattern: &annotations.HttpRule_Get{
			Get: "/v1/{name=organizations/**}",
		},
	}
	_, err = buildHTTPRule(g, m, rule)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `unsupported multi-segment wildcard "**"`)

	rule = &annotations.HttpRule{
		Pattern: &annotations.HttpRule_Get{
			Get: "/v1/{na me}",
		},
	}
	_, err = buildHTTPRule(g, m, rule)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `invalid binding "{na me}"`)
}

func TestGenerateFiles_ExpandsSingletonRoute(t *testing.T) {
	methodSets = make(map[string]int)

	gen := newTestPlugin(t, &descriptorpb.FileDescriptorProto{
		Name:    proto.String("test.proto"),
		Package: proto.String("test"),
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String(
				"github.com/codesjoy/yggdrasil/v3/cmd/protoc-gen-yggdrasil-rest;main",
			),
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: proto.String("SettingsService"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       proto.String("GetSettings"),
						InputType:  proto.String(".test.GetSettingsRequest"),
						OutputType: proto.String(".test.GetSettingsResponse"),
						Options:    &descriptorpb.MethodOptions{},
					},
				},
			},
		},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("GetSettingsRequest"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("name"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					},
				},
			},
			{Name: proto.String("GetSettingsResponse")},
		},
	})
	proto.SetExtension(
		gen.Files[0].Services[0].Methods[0].Desc.Options(),
		annotations.E_Http,
		&annotations.HttpRule{
			Pattern: &annotations.HttpRule_Get{
				Get: "/v1/{name=organizations/*/settings}",
			},
		},
	)

	err := generateFiles(gen, gen.Files[0])
	assert.NoError(t, err)

	output := generatedFileContent(t, gen, "test_rest.pb.go")
	assert.Contains(t, output, `Path:    "/v1/organizations/{params1}/settings"`)
	assert.Contains(t, output, `PopulateFieldFromPath(protoReq, "name", val)`)
	assert.Contains(t, output, `"organizations/" + v5.URLParam(r, "params1") + "/settings"`)
}

func TestGenerateFiles_NamedBodyPatchIncludesQueryBeforePathPopulation(t *testing.T) {
	methodSets = make(map[string]int)

	gen := newTestPlugin(t, &descriptorpb.FileDescriptorProto{
		Name:    proto.String("test.proto"),
		Package: proto.String("test"),
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String(
				"github.com/codesjoy/yggdrasil/v3/cmd/protoc-gen-yggdrasil-rest;main",
			),
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: proto.String("ResourcesService"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       proto.String("UpdateResource"),
						InputType:  proto.String(".test.UpdateResourceRequest"),
						OutputType: proto.String(".test.UpdateResourceResponse"),
						Options:    &descriptorpb.MethodOptions{},
					},
				},
			},
		},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("Resource"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("name"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					},
				},
			},
			{
				Name: proto.String("UpdateResourceRequest"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("resource"),
						Number:   proto.Int32(1),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".test.Resource"),
					},
					{
						Name:   proto.String("update_mask"),
						Number: proto.Int32(2),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					},
				},
			},
			{Name: proto.String("UpdateResourceResponse")},
		},
	})
	proto.SetExtension(
		gen.Files[0].Services[0].Methods[0].Desc.Options(),
		annotations.E_Http,
		&annotations.HttpRule{
			Pattern: &annotations.HttpRule_Patch{
				Patch: "/v1/{resource.name=organizations/*/settings}",
			},
			Body: "resource",
		},
	)

	err := generateFiles(gen, gen.Files[0])
	assert.NoError(t, err)

	output := generatedFileContent(t, gen, "test_rest.pb.go")
	decodeIdx := strings.Index(output, `Decode(protoReq.Resource)`)
	queryIdx := strings.Index(output, `PopulateQueryParameters(protoReq, r.URL.Query())`)
	pathIdx := strings.Index(output, `PopulateFieldFromPath(protoReq, "resource.name", val)`)
	assert.NotEqual(t, -1, decodeIdx)
	assert.NotEqual(t, -1, queryIdx)
	assert.NotEqual(t, -1, pathIdx)
	assert.True(t, decodeIdx < queryIdx)
	assert.True(t, queryIdx < pathIdx)
	assert.Contains(t, output, `Path:    "/v1/organizations/{params1}/settings"`)
}

func TestGenerateFiles_BodyStarSkipsQueryParsing(t *testing.T) {
	methodSets = make(map[string]int)

	gen := newTestPlugin(t, &descriptorpb.FileDescriptorProto{
		Name:    proto.String("test.proto"),
		Package: proto.String("test"),
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String(
				"github.com/codesjoy/yggdrasil/v3/cmd/protoc-gen-yggdrasil-rest;main",
			),
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: proto.String("ActionsService"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       proto.String("DeactivateResource"),
						InputType:  proto.String(".test.DeactivateResourceRequest"),
						OutputType: proto.String(".test.DeactivateResourceResponse"),
						Options:    &descriptorpb.MethodOptions{},
					},
				},
			},
		},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("DeactivateResourceRequest"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("name"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					},
					{
						Name:   proto.String("reason"),
						Number: proto.Int32(2),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					},
				},
			},
			{Name: proto.String("DeactivateResourceResponse")},
		},
	})
	proto.SetExtension(
		gen.Files[0].Services[0].Methods[0].Desc.Options(),
		annotations.E_Http,
		&annotations.HttpRule{
			Pattern: &annotations.HttpRule_Post{
				Post: "/v1/{name=organizations/*}:deactivate",
			},
			Body: "*",
		},
	)

	err := generateFiles(gen, gen.Files[0])
	assert.NoError(t, err)

	output := generatedFileContent(t, gen, "test_rest.pb.go")
	assert.Contains(t, output, `Decode(protoReq)`)
	assert.NotContains(t, output, `PopulateQueryParameters(protoReq, r.URL.Query())`)
	assert.Contains(t, output, `PopulateFieldFromPath(protoReq, "name", val)`)
}

func TestGenerateFiles_OmitsRestImportWhenHelpersAreUnused(t *testing.T) {
	methodSets = make(map[string]int)

	gen := newTestPlugin(t, &descriptorpb.FileDescriptorProto{
		Name:    proto.String("test.proto"),
		Package: proto.String("test"),
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String(
				"github.com/codesjoy/yggdrasil/v3/cmd/protoc-gen-yggdrasil-rest;main",
			),
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: proto.String("ActionsService"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       proto.String("RunAction"),
						InputType:  proto.String(".test.RunActionRequest"),
						OutputType: proto.String(".test.RunActionResponse"),
						Options:    &descriptorpb.MethodOptions{},
					},
				},
			},
		},
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("RunActionRequest")},
			{Name: proto.String("RunActionResponse")},
		},
	})
	proto.SetExtension(
		gen.Files[0].Services[0].Methods[0].Desc.Options(),
		annotations.E_Http,
		&annotations.HttpRule{
			Pattern: &annotations.HttpRule_Post{
				Post: "/v1/actions:run",
			},
			Body: "*",
		},
	)

	err := generateFiles(gen, gen.Files[0])
	assert.NoError(t, err)

	output := generatedFileContent(t, gen, "test_rest.pb.go")
	assert.NotContains(t, output, `"github.com/codesjoy/yggdrasil/v3/transport/gateway/rest"`)
	assert.NotContains(t, output, `PopulateQueryParameters(`)
	assert.NotContains(t, output, `PopulateFieldFromPath(`)
}

// literalSegment creates a pathBindingSegment representing a static path component.
func literalSegment(value string) pathBindingSegment {
	return pathBindingSegment{Literal: value}
}

// paramSegment creates a pathBindingSegment representing a runtime route parameter.
func paramSegment(value string) pathBindingSegment {
	return pathBindingSegment{Param: value}
}

// pathBinding is a test helper that constructs a pathVarBinding from a field
// path and a variadic list of segments.
func pathBinding(field string, segments ...pathBindingSegment) pathVarBinding {
	return pathVarBinding{
		FieldPath: field,
		Segments:  segments,
	}
}

// newTestPlugin creates a protogen.Plugin from a single FileDescriptorProto
// for use in end-to-end generation tests.
func newTestPlugin(t *testing.T, file *descriptorpb.FileDescriptorProto) *protogen.Plugin {
	t.Helper()

	gen, err := protogen.Options{}.New(&pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{file.GetName()},
		ProtoFile:      []*descriptorpb.FileDescriptorProto{file},
	})
	assert.NoError(t, err)
	return gen
}

// generatedFileContent returns the content of the generated file matching the
// given suffix (e.g. "test_rest.pb.go").
func generatedFileContent(t *testing.T, gen *protogen.Plugin, suffix string) string {
	t.Helper()

	for _, f := range gen.Response().File {
		if strings.HasSuffix(f.GetName(), suffix) || strings.HasSuffix(f.GetName(), "_rest.pb.go") {
			return f.GetContent()
		}
	}
	t.Fatalf("generated file with suffix %q not found", suffix)
	return ""
}
