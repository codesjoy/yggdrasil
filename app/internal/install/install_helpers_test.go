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
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/server"
)

type testHandler interface {
	Handle(string)
}

type testHandlerImpl struct{}

func (*testHandlerImpl) Handle(_ string) {}

// --- ImplementsHandler ---

func TestImplementsHandler(t *testing.T) {
	t.Run("nil handlerType returns false", func(t *testing.T) {
		assert.False(t, ImplementsHandler(nil, &testHandlerImpl{}))
	})

	t.Run("nil impl returns false", func(t *testing.T) {
		assert.False(t, ImplementsHandler((*testHandler)(nil), nil))
	})

	t.Run("valid match returns true", func(t *testing.T) {
		assert.True(t, ImplementsHandler((*testHandler)(nil), &testHandlerImpl{}))
	})

	t.Run("mismatch returns false", func(t *testing.T) {
		assert.False(t, ImplementsHandler((*testHandler)(nil), "not a handler"))
	})

	t.Run("non-pointer handlerType returns false", func(t *testing.T) {
		assert.False(t, ImplementsHandler("string", &testHandlerImpl{}))
	})
}

// --- BuildRESTRoutePrefix ---

func TestBuildRESTRoutePrefix(t *testing.T) {
	t.Run("empty slice returns empty", func(t *testing.T) {
		assert.Equal(t, "", BuildRESTRoutePrefix(nil))
		assert.Equal(t, "", BuildRESTRoutePrefix([]string{}))
	})

	t.Run("single prefix adds leading slash", func(t *testing.T) {
		assert.Equal(t, "/api", BuildRESTRoutePrefix([]string{"api"}))
	})

	t.Run("prefix with leading slash deduped", func(t *testing.T) {
		assert.Equal(t, "/api", BuildRESTRoutePrefix([]string{"/api"}))
	})

	t.Run("blank prefix returns empty", func(t *testing.T) {
		assert.Equal(t, "", BuildRESTRoutePrefix([]string{"  "}))
	})
}

// --- RouteKey ---

func TestRouteKey(t *testing.T) {
	t.Run("normal method and path", func(t *testing.T) {
		assert.Equal(t, "GET /api/v1/test", RouteKey("GET", "/api/v1/test"))
	})

	t.Run("lowercase method is normalized", func(t *testing.T) {
		assert.Equal(t, "POST /path", RouteKey("post", "/path"))
	})
}

// --- NormalizeRawHTTPBinding ---

func TestNormalizeRawHTTPBinding(t *testing.T) {
	t.Run("non-nil desc returns directly", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {})
		desc := &server.RestRawHandlerDesc{
			Method:  "PUT",
			Path:    "/existing",
			Handler: handler,
		}
		got, err := NormalizeRawHTTPBinding(desc, "POST", "/other", nil)
		require.NoError(t, err)
		assert.Equal(t, desc, got)
	})

	t.Run("builds desc from parts", func(t *testing.T) {
		fn := func(http.ResponseWriter, *http.Request) {}
		got, err := NormalizeRawHTTPBinding(nil, " GET ", " /path ", fn)
		require.NoError(t, err)
		assert.Equal(t, "GET", got.Method)
		assert.Equal(t, "/path", got.Path)
		assert.NotNil(t, got.Handler)
	})

	t.Run("empty method returns validation error", func(t *testing.T) {
		_, err := NormalizeRawHTTPBinding(
			nil,
			"",
			"/path",
			func(http.ResponseWriter, *http.Request) {},
		)
		require.Error(t, err)
		var assemblyErr *yassembly.Error
		assert.ErrorAs(t, err, &assemblyErr)
		assert.Equal(t, yassembly.ErrInstallValidationFailed, assemblyErr.Code)
	})

	t.Run("empty path returns validation error", func(t *testing.T) {
		_, err := NormalizeRawHTTPBinding(
			nil,
			"GET",
			"  ",
			func(http.ResponseWriter, *http.Request) {},
		)
		require.Error(t, err)
		var assemblyErr *yassembly.Error
		assert.ErrorAs(t, err, &assemblyErr)
		assert.Equal(t, yassembly.ErrInstallValidationFailed, assemblyErr.Code)
	})

	t.Run("unsupported handler type returns error", func(t *testing.T) {
		_, err := NormalizeRawHTTPBinding(nil, "GET", "/path", "not a handler")
		require.Error(t, err)
		var assemblyErr *yassembly.Error
		assert.ErrorAs(t, err, &assemblyErr)
		assert.Equal(t, yassembly.ErrInstallValidationFailed, assemblyErr.Code)
	})
}

// --- NormalizeRawHTTPHandler ---

func TestNormalizeRawHTTPHandler(t *testing.T) {
	t.Run("nil returns nil handler and true", func(t *testing.T) {
		handler, ok := NormalizeRawHTTPHandler(nil)
		assert.True(t, ok)
		assert.Nil(t, handler)
	})

	t.Run("http.HandlerFunc passes through", func(t *testing.T) {
		original := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
		handler, ok := NormalizeRawHTTPHandler(original)
		assert.True(t, ok)
		assert.NotNil(t, handler)
	})

	t.Run("raw func is wrapped", func(t *testing.T) {
		fn := func(http.ResponseWriter, *http.Request) {}
		handler, ok := NormalizeRawHTTPHandler(fn)
		assert.True(t, ok)
		assert.NotNil(t, handler)
	})

	t.Run("unsupported type returns false", func(t *testing.T) {
		handler, ok := NormalizeRawHTTPHandler("string")
		assert.False(t, ok)
		assert.Nil(t, handler)
	})

	t.Run("integer returns false", func(t *testing.T) {
		handler, ok := NormalizeRawHTTPHandler(42)
		assert.False(t, ok)
		assert.Nil(t, handler)
	})
}

// --- ValidationError ---

func TestValidationError(t *testing.T) {
	err := ValidationError("something is wrong", errors.New("cause"))
	require.Error(t, err)
	var assemblyErr *yassembly.Error
	assert.ErrorAs(t, err, &assemblyErr)
	assert.Equal(t, yassembly.ErrInstallValidationFailed, assemblyErr.Code)
	assert.Equal(t, "something is wrong", assemblyErr.Message)
	assert.Equal(t, "install", assemblyErr.Stage)
}

// --- ConflictError ---

func TestConflictError(t *testing.T) {
	err := ConflictError("duplicate entry", nil)
	require.Error(t, err)
	var assemblyErr *yassembly.Error
	assert.ErrorAs(t, err, &assemblyErr)
	assert.Equal(t, yassembly.ErrInstallRegistrationConflict, assemblyErr.Code)
	assert.Equal(t, "duplicate entry", assemblyErr.Message)
	assert.Equal(t, "install", assemblyErr.Stage)
}

// --- WrapError ---

func TestWrapError(t *testing.T) {
	t.Run("nil error returns nil", func(t *testing.T) {
		assert.Nil(t, WrapError(nil))
	})

	t.Run("assembly.Error passes through", func(t *testing.T) {
		original := yassembly.NewError(
			yassembly.ErrInstallRegistrationConflict,
			"install",
			"conflict",
			nil,
			nil,
		)
		wrapped := WrapError(original)
		assert.Equal(t, original, wrapped)
	})

	t.Run("plain error is wrapped as validation error", func(t *testing.T) {
		wrapped := WrapError(errors.New("plain error"))
		require.Error(t, wrapped)
		var assemblyErr *yassembly.Error
		assert.ErrorAs(t, wrapped, &assemblyErr)
		assert.Equal(t, yassembly.ErrInstallValidationFailed, assemblyErr.Code)
		assert.Contains(t, assemblyErr.Message, "plain error")
	})
}

func TestValidateRPCBinding(t *testing.T) {
	desc := &server.ServiceDesc{
		ServiceName: "svc",
		HandlerType: (*testHandler)(nil),
	}

	t.Run("valid binding", func(t *testing.T) {
		gotDesc, serviceName, err := ValidateRPCBinding(true, "", desc, &testHandlerImpl{})
		require.NoError(t, err)
		assert.Equal(t, desc, gotDesc)
		assert.Equal(t, "svc", serviceName)
	})

	t.Run("missing transport returns error", func(t *testing.T) {
		_, _, err := ValidateRPCBinding(false, "", desc, &testHandlerImpl{})
		require.Error(t, err)
	})

	t.Run("invalid desc returns error", func(t *testing.T) {
		_, _, err := ValidateRPCBinding(true, "", "bad", &testHandlerImpl{})
		require.Error(t, err)
	})

	t.Run("invalid impl returns error", func(t *testing.T) {
		_, _, err := ValidateRPCBinding(true, "", desc, "bad")
		require.Error(t, err)
	})
}

func TestValidateRESTBinding(t *testing.T) {
	desc := &server.RestServiceDesc{
		HandlerType: (*testHandler)(nil),
		Methods: []server.RestMethodDesc{
			{Method: "GET", Path: "/users"},
		},
	}

	t.Run("valid binding", func(t *testing.T) {
		gotDesc, name, err := ValidateRESTBinding(true, "", desc, &testHandlerImpl{})
		require.NoError(t, err)
		assert.Equal(t, desc, gotDesc)
		assert.NotEmpty(t, name)
	})

	t.Run("rest disabled returns error", func(t *testing.T) {
		_, _, err := ValidateRESTBinding(false, "", desc, &testHandlerImpl{})
		require.Error(t, err)
	})

	t.Run("invalid desc returns error", func(t *testing.T) {
		_, _, err := ValidateRESTBinding(true, "", "bad", &testHandlerImpl{})
		require.Error(t, err)
	})

	t.Run("invalid impl returns error", func(t *testing.T) {
		_, _, err := ValidateRESTBinding(true, "", desc, "bad")
		require.Error(t, err)
	})
}

func TestValidateRawHTTPBindingStateful(t *testing.T) {
	t.Run("rest disabled returns error", func(t *testing.T) {
		_, err := ValidateRawHTTPBinding(
			false,
			nil,
			"GET",
			"/users",
			func(http.ResponseWriter, *http.Request) {},
		)
		require.Error(t, err)
	})

	t.Run("nil handler returns error", func(t *testing.T) {
		_, err := ValidateRawHTTPBinding(true, nil, "GET", "/users", nil)
		require.Error(t, err)
	})
}

func TestCheckServiceConflict(t *testing.T) {
	t.Run("no conflict", func(t *testing.T) {
		err := CheckServiceConflict(map[string]struct{}{}, "svc", "svc")
		require.NoError(t, err)
	})

	t.Run("duplicate conflict", func(t *testing.T) {
		err := CheckServiceConflict(map[string]struct{}{"svc": {}}, "svc", "svc")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already installed")
	})
}

func TestCheckRouteConflict(t *testing.T) {
	t.Run("no conflict", func(t *testing.T) {
		err := CheckRouteConflict("rest", map[string]struct{}{}, "GET", "/users")
		require.NoError(t, err)
	})

	t.Run("duplicate conflict", func(t *testing.T) {
		err := CheckRouteConflict(
			"raw http",
			map[string]struct{}{RouteKey("GET", "/users"): {}},
			"GET",
			"/users",
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already installed")
	})
}
