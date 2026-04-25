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

package assembly

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestError_NilReceiver(t *testing.T) {
	var e *Error
	require.Equal(t, "", e.Error())
}

func TestError_EmptyCode(t *testing.T) {
	e := &Error{Code: "", Message: "something broke"}
	require.Equal(t, "something broke", e.Error())
}

func TestError_EmptyMessage(t *testing.T) {
	e := &Error{Code: ErrInvalidMode, Message: ""}
	require.Equal(t, "InvalidMode", e.Error())
}

func TestError_FullFormat(t *testing.T) {
	e := &Error{Code: ErrInvalidMode, Message: "mode not supported"}
	require.Equal(t, "InvalidMode: mode not supported", e.Error())
}

func TestError_Unwrap_NilReceiver(t *testing.T) {
	var e *Error
	require.Nil(t, e.Unwrap())
}

func TestError_Unwrap_WithCause(t *testing.T) {
	cause := errors.New("root cause")
	e := &Error{Code: ErrComposeFailed, Message: "failed", Cause: cause}
	require.Equal(t, cause, e.Unwrap())
}

func TestError_Unwrap_NoCause(t *testing.T) {
	e := &Error{Code: ErrComposeFailed, Message: "failed"}
	require.Nil(t, e.Unwrap())
}

func TestNewError_Fields(t *testing.T) {
	cause := errors.New("root")
	ctx := map[string]string{"key": "value"}
	e := NewError(ErrConflictingOverride, "plan", "conflict", cause, ctx)
	require.Equal(t, ErrConflictingOverride, e.Code)
	require.Equal(t, "plan", e.Stage)
	require.Equal(t, "conflict", e.Message)
	require.Equal(t, cause, e.Cause)
	require.Equal(t, ctx, e.Context)
}
