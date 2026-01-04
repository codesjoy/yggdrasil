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

package server

import (
	"context"
	"net/http"

	"github.com/codesjoy/yggdrasil/pkg/interceptor"
	"github.com/codesjoy/yggdrasil/pkg/stream"
)

type methodHandler func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor interceptor.UnaryServerInterceptor) (interface{}, error)

// MethodDesc represents an RPC service's method specification.
type MethodDesc struct {
	MethodName string
	Handler    methodHandler
}

// ServiceDesc represents an RPC service's specification.
type ServiceDesc struct {
	ServiceName string
	// The pointer to the service interface. Used to check whether the user
	// provided implementation satisfies the interface requirements.
	HandlerType interface{}
	Methods     []MethodDesc
	Streams     []stream.Desc
	Metadata    interface{}
}

// ServiceInfo represents an RPC service's information.
type ServiceInfo struct {
	// Contains the implementation for the methods in this service.
	ServiceImpl interface{}
	Methods     map[string]*MethodDesc
	Streams     map[string]*stream.Desc
	Metadata    interface{}
}

type methodInfo struct {
	MethodName    string `json:"methodName"`
	ServerStreams bool   `json:"serverStreams"`
	ClientStreams bool   `json:"clientStreams"`
}

// RestMethodHandler represents a REST method handler.
type RestMethodHandler func(w http.ResponseWriter, r *http.Request, srv interface{}, interceptor interceptor.UnaryServerInterceptor) (interface{}, error)

// RestServiceDesc represents a REST service's specification.
type RestServiceDesc struct {
	HandlerType interface{}
	Methods     []RestMethodDesc
}

// RestMethodDesc represents a REST method specification.
type RestMethodDesc struct {
	Method  string
	Path    string
	Handler RestMethodHandler
}

type restRouterInfo struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

// RestRawHandlerDesc represents a raw REST handler specification.
type RestRawHandlerDesc struct {
	Method  string
	Path    string
	Handler http.HandlerFunc
}
