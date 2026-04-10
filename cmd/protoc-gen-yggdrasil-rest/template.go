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
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/template"
)

// restTemplate is the Go text/template that generates the REST handler functions
// and service descriptor for a single protobuf service. Each method produces:
//  1. A local handler function that decodes the request body, populates query
//     parameters, extracts path values, and delegates to the service impl.
//  2. An entry in the RestServiceDesc method list.
//
// Template execution order per request:
//
//	Decode(body) → PopulateQueryParameters → PopulateFieldFromPath → handler
var restTemplate = `
{{range $method := .Methods }}
func local_handler_{{$.ServiceType}}_{{ .Name }}_{{.Num}}(w {{$.HTTPPkg}}ResponseWriter, r *{{$.HTTPPkg}}Request, server interface{}, unaryInt {{$.InterceptorPkg}}UnaryServerInterceptor) (interface{}, error) {
		protoReq := &{{$method.Request}}{}
		{{if $method.HasBody }}
			{{if $method.Body }}
				// Initialize nested body field if it's a pointer
				if protoReq{{$method.Body}} == nil {
					protoReq{{$method.Body}} = &{{$method.BodyType}}{}
				}
			{{end}}
			inbound := {{$.MarshalerPkg}}InboundFromContext(r.Context())
			if err := inbound.NewDecoder(r.Body).Decode(protoReq{{$method.Body}}); err != nil && err != {{$.IoPkg}}EOF {
				return nil, {{$.StatusPkg}}Wrap(err, {{$.CodePkg}}Code_INVALID_ARGUMENT, "")
			}
		{{end -}}
		{{if $method.HasQueryParams }}
			if err := {{$.RestPkg}}PopulateQueryParameters(protoReq, r.URL.Query()); err != nil {
				return nil,  {{$.StatusPkg}}Wrap(err, {{$.CodePkg}}Code_INVALID_ARGUMENT, "")
			}
		{{end -}}

		{{- range  $binding := .PathBindings}}
			if val := {{renderPathValue $binding.Segments }}; len(val) == 0 {
				return nil, {{$.StatusPkg}}New({{$.CodePkg}}Code_INVALID_ARGUMENT, "not found {{$binding.FieldPath}}")
			} else if err := {{$.RestPkg}}PopulateFieldFromPath(protoReq, {{$binding.FieldPath | printf "%q"}}, val); err != nil {
				return nil, {{$.StatusPkg}}Wrap(err, {{$.CodePkg}}Code_INVALID_ARGUMENT, "")
			}
		{{- end}}

		if unaryInt == nil {
			return  server.({{$.ServiceType}}Server).{{$method.Name}}(r.Context(), protoReq)
		}

		info := &interceptor.UnaryServerInfo{
			Server:     server,
			FullMethod: "{{$.ServiceName}}/{{ .Name }}",
		}
		handler := func(ctx {{$.CtxPkg}}Context, req interface{}) (interface{}, error) {
			return server.({{$.ServiceType}}Server).{{$method.Name}}(ctx, req.(*{{$method.Request}}))
		}
		return unaryInt(r.Context(), protoReq, info, handler)
}
{{end -}}

var {{$.ServiceType}}RestServiceDesc = {{$.SvrPkg}}RestServiceDesc{
	HandlerType: (*{{$.ServiceType}}Server)(nil),
	Methods: []{{$.SvrPkg}}RestMethodDesc{
		{{range $method := .Methods -}}
		{
			Method: "{{$method.Method}}",
			Path: "{{$method.Path}}",
			Handler:    local_handler_{{$.ServiceType}}_{{ .Name }}_{{.Num}},
		},
		{{end -}}
	},
}
`

type serviceDesc struct {
	HTTPPkg        string
	ChiPkg         string
	MarshalerPkg   string
	StatusPkg      string
	RestPkg        string
	SvrPkg         string
	CodePkg        string
	InterceptorPkg string
	CtxPkg         string
	IoPkg          string

	ServiceType string
	ServiceName string
	Methods     []*methodDesc
}

// methodDesc holds all information needed to generate a single REST handler
// function and its corresponding route registration entry.
type methodDesc struct {
	Name    string
	Num     int    // overload counter for methods with multiple HTTP bindings
	Method  string // HTTP verb (GET, POST, PUT, PATCH, DELETE, CUSTOM)
	Request string // qualified Go type name of the request message

	// PathBindings contains the parsed path variable bindings. Each binding
	// maps a protobuf field path to a sequence of literal and param segments.
	PathBindings []pathVarBinding
	Path         string // rewritten route path with {paramsN} placeholders

	Body           string // dot-prefixed CamelCase field accessor (e.g. ".Resource") or empty
	BodyType       string // qualified Go type name of the body field message, for nil-initialization
	HasBody        bool   // true when the HTTP rule declares a body
	HasQueryParams bool   // true when query parameters should be populated (body="" or body="field")
}

func (s *serviceDesc) execute() string {
	buf := new(bytes.Buffer)
	tmpl, err := template.New("http-rest").
		Funcs(template.FuncMap{"renderPathValue": s.renderPathValue}).
		Parse(strings.TrimSpace(restTemplate))
	if err != nil {
		panic(err)
	}
	if err = tmpl.Execute(buf, s); err != nil {
		panic(err)
	}
	return buf.String()
}

// renderPathValue converts a slice of pathBindingSegments into a Go
// expression string that, when executed in the generated handler, reconstructs
// the bound path value at runtime using chi.URLParam calls and string literals.
//
// Example: [literal("orgs"), param("params1"), literal("settings")]
// → `"orgs/" + chi.URLParam(r, "params1") + "/settings"`
func (s *serviceDesc) renderPathValue(segments []pathBindingSegment) string {
	if len(segments) == 0 {
		return `""`
	}

	parts := make([]string, 0, len(segments)*2)

	// Accumulate consecutive literal segments (with "/" separators) into a
	// single quoted string, flushing when a param segment is encountered.
	var literal strings.Builder
	flushLiteral := func() {
		if literal.Len() == 0 {
			return
		}
		parts = append(parts, strconv.Quote(literal.String()))
		literal.Reset()
	}

	for idx, segment := range segments {
		// Insert "/" between segments. The separator is accumulated into the
		// literal buffer so that adjacent literals are merged cleanly.
		if idx > 0 {
			literal.WriteString("/")
		}
		if segment.Param != "" {
			flushLiteral()
			parts = append(parts, fmt.Sprintf(`%sURLParam(r, %q)`, s.ChiPkg, segment.Param))
			continue
		}
		literal.WriteString(segment.Literal)
	}
	// Flush any trailing literal text.
	flushLiteral()

	return strings.Join(parts, " + ")
}
