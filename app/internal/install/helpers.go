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
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/server"
)

// ImplementsHandler checks whether impl satisfies the handlerType interface.
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

// BuildRESTRoutePrefix joins the prefix list into a single route prefix.
func BuildRESTRoutePrefix(prefixes []string) string {
	if len(prefixes) == 0 || strings.TrimSpace(prefixes[0]) == "" {
		return ""
	}
	return "/" + strings.TrimPrefix(prefixes[0], "/")
}

// RouteKey builds a unique route key from method and path.
func RouteKey(method, path string) string {
	return strings.ToUpper(method) + " " + path
}

// NormalizeRawHTTPBinding fills a RestRawHandlerDesc from loose parts, validating inputs.
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

// NormalizeRawHTTPHandler adapts handler into http.HandlerFunc if possible.
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

// ValidationError creates a structured install validation error.
func ValidationError(message string, cause error) error {
	return yassembly.NewError(yassembly.ErrInstallValidationFailed, "install", message, cause, nil)
}

// ConflictError creates a structured install conflict error.
func ConflictError(message string, cause error) error {
	return yassembly.NewError(
		yassembly.ErrInstallRegistrationConflict,
		"install",
		message,
		cause,
		nil,
	)
}

// WrapError wraps an install error into a structured assembly error.
func WrapError(err error) error {
	if err == nil {
		return nil
	}
	if structured, ok := err.(*yassembly.Error); ok {
		return structured
	}
	return ValidationError(fmt.Sprintf("install failed: %v", err), err)
}

// ValidateRPCBinding validates one RPC binding and returns the resolved descriptor and display name.
func ValidateRPCBinding(
	transportConfigured bool,
	bindingServiceName string,
	desc any,
	impl any,
) (*server.ServiceDesc, string, error) {
	if !transportConfigured {
		return nil, "", ValidationError(
			"rpc bindings require at least one configured server transport",
			nil,
		)
	}
	serviceDesc, ok := desc.(*server.ServiceDesc)
	if !ok || serviceDesc == nil {
		return nil, "", ValidationError("rpc binding desc must be *server.ServiceDesc", nil)
	}
	serviceName := strings.TrimSpace(bindingServiceName)
	if serviceName == "" {
		serviceName = serviceDesc.ServiceName
	}
	if impl == nil {
		return nil, "", ValidationError(
			fmt.Sprintf("rpc service %q implementation is nil", serviceName),
			nil,
		)
	}
	if !ImplementsHandler(serviceDesc.HandlerType, impl) {
		return nil, "", ValidationError(
			fmt.Sprintf("rpc service %q handler does not satisfy interface", serviceName),
			nil,
		)
	}
	return serviceDesc, serviceName, nil
}

// ValidateRESTBinding validates one REST binding and returns the resolved descriptor and display name.
func ValidateRESTBinding(
	restEnabled bool,
	bindingName string,
	desc any,
	impl any,
) (*server.RestServiceDesc, string, error) {
	if !restEnabled {
		return nil, "", ValidationError(
			"rest bindings require yggdrasil.transports.http.rest",
			nil,
		)
	}
	restDesc, ok := desc.(*server.RestServiceDesc)
	if !ok || restDesc == nil {
		return nil, "", ValidationError(
			"rest binding desc must be *server.RestServiceDesc",
			nil,
		)
	}
	name := strings.TrimSpace(bindingName)
	if name == "" {
		if handlerType := reflect.TypeOf(restDesc.HandlerType); handlerType != nil {
			name = handlerType.String()
		} else {
			name = "rest"
		}
	}
	if impl == nil {
		return nil, "", ValidationError(
			fmt.Sprintf("rest binding %q implementation is nil", name),
			nil,
		)
	}
	if !ImplementsHandler(restDesc.HandlerType, impl) {
		return nil, "", ValidationError(
			fmt.Sprintf("rest binding %q handler does not satisfy interface", name),
			nil,
		)
	}
	return restDesc, name, nil
}

// ValidateRawHTTPBinding validates and normalizes one raw HTTP binding.
func ValidateRawHTTPBinding(
	restEnabled bool,
	desc *server.RestRawHandlerDesc,
	method string,
	path string,
	handler any,
) (*server.RestRawHandlerDesc, error) {
	if !restEnabled {
		return nil, ValidationError(
			"raw http bindings require yggdrasil.transports.http.rest",
			nil,
		)
	}
	normalized, err := NormalizeRawHTTPBinding(desc, method, path, handler)
	if err != nil {
		return nil, err
	}
	if normalized.Handler == nil {
		return nil, ValidationError("raw http binding handler is nil", nil)
	}
	return normalized, nil
}

// CheckServiceConflict reports whether the RPC service key is already installed.
func CheckServiceConflict(installed map[string]struct{}, key string, displayName string) error {
	if _, exists := installed[key]; exists {
		return ConflictError(
			fmt.Sprintf("rpc service %q already installed", displayName),
			nil,
		)
	}
	return nil
}

// CheckRouteConflict reports whether the route is already installed.
func CheckRouteConflict(kind string, installed map[string]struct{}, method, path string) error {
	if _, exists := installed[RouteKey(method, path)]; exists {
		return ConflictError(
			fmt.Sprintf("%s route %s %s already installed", kind, method, path),
			nil,
		)
	}
	return nil
}
