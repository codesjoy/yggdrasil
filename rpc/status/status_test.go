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

package status

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/codesjoy/pkg/basic/xerror"
	"github.com/stretchr/testify/assert"
	"google.golang.org/genproto/googleapis/rpc/code"

	istatus "github.com/codesjoy/yggdrasil/v3/internal/status"
)

type testReason struct {
	reason string
	domain string
	code   code.Code
}

func (r testReason) Reason() string {
	return r.reason
}

func (r testReason) Domain() string {
	return r.domain
}

func (r testReason) Code() code.Code {
	return r.code
}

func TestCoverError(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		st, ok := CoverError(nil)
		assert.Nil(t, st)
		assert.True(t, ok)
	})

	t.Run("plain error fallback", func(t *testing.T) {
		st, ok := CoverError(errors.New("plain"))
		assert.False(t, ok)
		assert.NotNil(t, st)
		assert.Equal(t, code.Code_UNKNOWN, st.Code())
		assert.Equal(t, "plain", st.Message())
	})

	t.Run("status passthrough", func(t *testing.T) {
		src := istatus.WithCode(code.Code_NOT_FOUND, errors.New("missing"))
		st, ok := CoverError(fmt.Errorf("wrapped: %w", src))
		assert.True(t, ok)
		assert.Equal(t, code.Code_NOT_FOUND, st.Code())
	})

	t.Run("xerror conversion", func(t *testing.T) {
		reason := testReason{
			reason: "INVALID_INPUT",
			domain: "demo.domain",
			code:   code.Code_INVALID_ARGUMENT,
		}
		xerr := xerror.WrapWithReason(errors.New("invalid payload"), reason, "", map[string]string{
			"field": "name",
		})

		st, ok := CoverError(fmt.Errorf("wrapped: %w", xerr))
		assert.True(t, ok)
		assert.Equal(t, code.Code_INVALID_ARGUMENT, st.Code())
		assert.Equal(t, "wrapped: invalid payload", st.Message())
		info := st.ErrorInfo()
		assert.NotNil(t, info)
		assert.Equal(t, "INVALID_INPUT", info.GetReason())
		assert.Equal(t, "demo.domain", info.GetDomain())
		assert.Equal(t, "name", info.GetMetadata()["field"])
	})
}

func TestFromErrorCode(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		assert.Nil(t, FromErrorCode(nil, code.Code_INTERNAL))
	})

	t.Run("fallback code for plain error", func(t *testing.T) {
		st := FromErrorCode(errors.New("boom"), code.Code_PERMISSION_DENIED)
		assert.Equal(t, code.Code_PERMISSION_DENIED, st.Code())
	})

	t.Run("keep xerror code", func(t *testing.T) {
		xerr := xerror.Wrap(errors.New("db timeout"), code.Code_UNAVAILABLE, "")
		st := FromErrorCode(xerr, code.Code_INTERNAL)
		assert.Equal(t, code.Code_UNAVAILABLE, st.Code())
	})
}

func TestFromError(t *testing.T) {
	t.Run("plain error", func(t *testing.T) {
		st := FromError(errors.New("plain"))
		assert.Equal(t, code.Code_UNKNOWN, st.Code())
		assert.Equal(t, "plain", st.Message())
	})
}

func TestFromContextError(t *testing.T) {
	t.Run("deadline exceeded", func(t *testing.T) {
		st := FromContextError(context.DeadlineExceeded)
		assert.Equal(t, code.Code_DEADLINE_EXCEEDED, st.Code())
		assert.Equal(t, int32(http.StatusGatewayTimeout), st.HTTPCode())
	})

	t.Run("cancelled", func(t *testing.T) {
		st := FromContextError(context.Canceled)
		assert.Equal(t, code.Code_CANCELLED, st.Code())
	})
}
