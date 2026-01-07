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
	"regexp"
	"strings"
	"text/template"
)

var restTemplate = `
{{range $method := .Methods }}
func local_handler_{{$.ServiceType}}_{{ .Name }}_{{.Num}}(w {{$.HTTPPkg}}ResponseWriter, r *{{$.HTTPPkg}}Request, server interface{}, unaryInt {{$.InterceptorPkg}}UnaryServerInterceptor) (interface{}, error) {
		protoReq := &{{$method.Request}}{}
		{{if $method.HasBody }}
			inbound := {{$.MarshalerPkg}}InboundFromContext(r.Context())
			if err := inbound.NewDecoder(r.Body).Decode(protoReq{{$method.Body}}); err != nil && err != {{$.IoPkg}}EOF {
				return nil, {{$.StatusPkg}}WithCode({{$.CodePkg}}Code_INVALID_ARGUMENT, err)
			}
		{{else -}}
			if err := {{$.RestPkg}}PopulateQueryParameters(protoReq, r.URL.Query()); err != nil {
				return nil,  {{$.StatusPkg}}WithCode({{$.CodePkg}}Code_INVALID_ARGUMENT, err)
			}
		{{end -}}

		{{- range  $key, $value := .PathVars}}
			if val := {{parsePathValues $value }}; len(val) == 0 {
				return nil, {{$.StatusPkg}}New({{$.CodePkg}}Code_INVALID_ARGUMENT, "not found {{$key}}")
			} else if err := {{$.RestPkg}}PopulateFieldFromPath(protoReq, {{$key | printf "%q"}}, val); err != nil {
				return nil, {{$.StatusPkg}}WithCode({{$.CodePkg}}Code_INVALID_ARGUMENT, err)
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

type methodDesc struct {
	Name    string
	Num     int
	Method  string
	Request string

	PathVars map[string]string
	Path     string

	Body    string
	HasBody bool
}

func (s *serviceDesc) execute() string {
	buf := new(bytes.Buffer)
	tmpl, err := template.New("http-rest").
		Funcs(template.FuncMap{"parsePathValues": s.parsePathValues}).
		Parse(strings.TrimSpace(restTemplate))
	if err != nil {
		panic(err)
	}
	if err = tmpl.Execute(buf, s); err != nil {
		panic(err)
	}
	return buf.String()
}

func (s *serviceDesc) parsePathValues(path string) string {
	subPattern0 := regexp.MustCompile(`(?i)^{params[0-9]+}$`)
	if subPattern0.MatchString(path) {
		path = fmt.Sprintf(
			`%sURLParam(r, "%s")`,
			s.ChiPkg,
			strings.TrimRight(strings.TrimLeft(path, "{"), "}"),
		)
		return path
	}
	path = subPattern0.ReplaceAllStringFunc(path, func(subStr string) string {
		params := pathPattern.FindStringSubmatch(subStr)
		return fmt.Sprintf(`%sURLParam(r, "%s")+"/`, s.ChiPkg, params[1])
	})
	subPattern1 := regexp.MustCompile(`(?i)/{(params[0-9]+)}/`)
	path = subPattern1.ReplaceAllStringFunc(path, func(subStr string) string {
		params := pathPattern.FindStringSubmatch(subStr)
		return fmt.Sprintf(`/"+%sURLParam(r, "%s")+"/`, s.ChiPkg, params[1])
	})
	subPattern2 := regexp.MustCompile(`(?i)/{(params[0-9]+)}`)
	path = subPattern2.ReplaceAllStringFunc(path, func(subStr string) string {
		params := pathPattern.FindStringSubmatch(subStr)
		return fmt.Sprintf(`/"+%sURLParam(r, "%s")+"`, s.ChiPkg, params[1])
	})
	path = fmt.Sprintf(`"%s"`, path)
	path = strings.TrimRight(path, `+""`) // nolint:staticcheck
	return path
}
