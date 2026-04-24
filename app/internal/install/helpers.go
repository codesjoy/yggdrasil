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

package install

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"

	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
	"github.com/codesjoy/yggdrasil/v3/server"
)

func ImplementsHandler(handlerType, impl any) bool {
	if handlerType == nil || impl == nil {
		return false
	}
	want := reflect.TypeOf(handlerType)
	if want == nil || want.Kind() != reflect.Pointer {
		return false
	}
	got := reflect.TypeOf(impl)
	return got.Implements(want.Elem())
}

func BuildRESTRoutePrefix(prefixes []string) string {
	if len(prefixes) == 0 || strings.TrimSpace(prefixes[0]) == "" {
		return ""
	}
	return "/" + strings.TrimPrefix(prefixes[0], "/")
}

func RouteKey(method, path string) string {
	return strings.ToUpper(method) + " " + path
}

func NormalizeRawHTTPBinding(
	desc *server.RestRawHandlerDesc,
	method, path string,
	handler any,
) (*server.RestRawHandlerDesc, error) {
	if desc != nil {
		return desc, nil
	}
	method = strings.TrimSpace(method)
	path = strings.TrimSpace(path)
	if method == "" {
		return nil, ValidationError("raw http binding method is empty", nil)
	}
	if path == "" {
		return nil, ValidationError("raw http binding path is empty", nil)
	}
	normalizedHandler, ok := NormalizeRawHTTPHandler(handler)
	if !ok {
		return nil, ValidationError("raw http binding handler must be http.HandlerFunc", nil)
	}
	return &server.RestRawHandlerDesc{
		Method:  method,
		Path:    path,
		Handler: normalizedHandler,
	}, nil
}

func NormalizeRawHTTPHandler(handler any) (http.HandlerFunc, bool) {
	switch item := handler.(type) {
	case nil:
		return nil, true
	case http.HandlerFunc:
		return item, true
	case func(http.ResponseWriter, *http.Request):
		return http.HandlerFunc(item), true
	default:
		return nil, false
	}
}

func ValidationError(message string, cause error) error {
	return yassembly.NewError(yassembly.ErrInstallValidationFailed, "install", message, cause, nil)
}

func ConflictError(message string, cause error) error {
	return yassembly.NewError(yassembly.ErrInstallRegistrationConflict, "install", message, cause, nil)
}

func WrapError(err error) error {
	if err == nil {
		return nil
	}
	if structured, ok := err.(*yassembly.Error); ok {
		return structured
	}
	return ValidationError(fmt.Sprintf("install failed: %v", err), err)
}
