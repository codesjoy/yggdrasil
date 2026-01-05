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

	"github.com/stretchr/testify/assert"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
)

// TestStatus_Reason tests Reason method
func TestStatus_Reason(t *testing.T) {
	t.Run("Reason returns nil when no reason", func(t *testing.T) {
		st := WithCode(code.Code_NOT_FOUND, errors.New("not found"))
		reason := st.ErrorInfo()
		assert.Nil(t, reason)
	})

	t.Run("Reason returns ErrorInfo when present", func(t *testing.T) {
		st := WithCode(code.Code_NOT_FOUND, errors.New("not found")).
			WithDetails(&errdetails.ErrorInfo{
				Reason:   "test reason",
				Domain:   "test domain",
				Metadata: map[string]string{"key": "value"},
			})

		reason := st.ErrorInfo()
		assert.NotNil(t, reason)
		assert.Equal(t, "test reason", reason.Reason)
		assert.Equal(t, "test domain", reason.Domain)
	})

	t.Run("Reason on nil status", func(t *testing.T) {
		var st *Status
		reason := st.ErrorInfo()
		assert.Nil(t, reason)
	})
}

// TestFromReason tests FromReason function
func TestFromReason(t *testing.T) {
	t.Run("from reason", func(t *testing.T) {
		err := errors.New("test error")
		reason := &testReason{
			reason: "TEST_REASON",
			domain: "test.domain",
			code:   code.Code_INVALID_ARGUMENT,
		}
		metadata := map[string]string{"key": "value"}

		st := FromReason(err, reason, metadata)
		assert.NotNil(t, st)
		assert.Equal(t, code.Code_INVALID_ARGUMENT, st.Code())

		errorInfo := st.ErrorInfo()
		assert.NotNil(t, errorInfo)
		assert.Equal(t, "TEST_REASON", errorInfo.Reason)
		assert.Equal(t, "test.domain", errorInfo.Domain)
	})

	t.Run("from reason with nil error", func(t *testing.T) {
		reason := &testReason{
			reason: "TEST_REASON",
			domain: "test.domain",
			code:   code.Code_INVALID_ARGUMENT,
		}
		st := FromReason(nil, reason, nil)
		assert.Nil(t, st)
	})
}

// TestCoverError tests CoverError function
func TestCoverError(t *testing.T) {
	t.Run("cover nil error", func(t *testing.T) {
		st, ok := CoverError(nil)
		assert.Nil(t, st)
		assert.True(t, ok)
	})

	t.Run("cover standard error", func(t *testing.T) {
		err := errors.New("standard error")
		st, ok := CoverError(err)
		assert.NotNil(t, st)
		assert.False(t, ok)
		assert.Equal(t, code.Code_UNKNOWN, st.Code())
	})

	t.Run("cover status error", func(t *testing.T) {
		originalSt := WithCode(code.Code_NOT_FOUND, errors.New("not found"))
		wrappedErr := fmt.Errorf("wrapped: %w", originalSt)
		st, ok := CoverError(wrappedErr)
		assert.True(t, ok)
		assert.NotNil(t, st)
		assert.Equal(t, code.Code_NOT_FOUND, st.Code())
	})
}

// TestIsReason tests IsReason function
func TestIsReason(t *testing.T) {
	t.Run("is reason with matching reason", func(t *testing.T) {
		reason := &testReason{
			reason: "TEST_REASON",
			domain: "test.domain",
			code:   code.Code_INVALID_ARGUMENT,
		}
		err := errors.New("test error")
		st := FromReason(err, reason, nil)

		// Wrap status so coverError can extract it
		wrappedErr := fmt.Errorf("wrapped: %w", st)
		assert.True(t, IsReason(wrappedErr, reason))
	})

	t.Run("is reason with non-matching reason", func(t *testing.T) {
		reason1 := &testReason{
			reason: "REASON_1",
			domain: "test.domain",
			code:   code.Code_INVALID_ARGUMENT,
		}
		reason2 := &testReason{
			reason: "REASON_2",
			domain: "test.domain",
			code:   code.Code_INVALID_ARGUMENT,
		}
		err := errors.New("test error")
		st := FromReason(err, reason1, nil)

		wrappedErr := fmt.Errorf("wrapped: %w", st)
		assert.False(t, IsReason(wrappedErr, reason2))
	})

	t.Run("is reason with standard error", func(t *testing.T) {
		err := errors.New("standard error")
		reason := &testReason{
			reason: "TEST_REASON",
			domain: "test.domain",
			code:   code.Code_INVALID_ARGUMENT,
		}
		assert.False(t, IsReason(err, reason))
	})
}

// TestFromError tests creating status from error
func TestFromError(t *testing.T) {
	t.Run("from nil error", func(t *testing.T) {
		st := FromError(nil)
		assert.Nil(t, st)
	})

	t.Run("from standard error", func(t *testing.T) {
		err := errors.New("standard error")
		st := FromError(err)
		assert.NotNil(t, st)
		assert.Equal(t, code.Code_UNKNOWN, st.Code())
		assert.Equal(t, "standard error", st.Message())
	})

	t.Run("from status error", func(t *testing.T) {
		originalSt := WithCode(code.Code_NOT_FOUND, errors.New("not found"))
		wrappedErr := fmt.Errorf("wrapped: %w", originalSt)
		st := FromError(wrappedErr)
		assert.NotNil(t, st)
		assert.Equal(t, code.Code_NOT_FOUND, st.Code())
	})
}

// TestIsCode tests IsCode function
func TestIsCode(t *testing.T) {
	t.Run("is code with matching code", func(t *testing.T) {
		st := WithCode(code.Code_NOT_FOUND, errors.New("not found"))
		wrappedErr := fmt.Errorf("wrapped: %w", st)
		assert.True(t, IsCode(wrappedErr, code.Code_NOT_FOUND))
	})

	t.Run("is code with non-matching code", func(t *testing.T) {
		st := WithCode(code.Code_NOT_FOUND, errors.New("not found"))
		wrappedErr := fmt.Errorf("wrapped: %w", st)
		assert.False(t, IsCode(wrappedErr, code.Code_OK))
	})

	t.Run("is code with standard error", func(t *testing.T) {
		err := errors.New("standard error")
		assert.False(t, IsCode(err, code.Code_NOT_FOUND))
	})
}

// TestStatus_Conversions tests status conversions
func TestStatus_Conversions(t *testing.T) {
	t.Run("round trip from error to status", func(t *testing.T) {
		originalErr := errors.New("original error")
		st := FromError(originalErr)
		assert.NotNil(t, st)
		assert.Equal(t, code.Code_UNKNOWN, st.Code())
		assert.Equal(t, "original error", st.Message())
	})

	t.Run("context error conversion", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := ctx.Err()
		st := FromContextError(err)
		assert.NotNil(t, st)
		assert.Equal(t, code.Code_CANCELLED, st.Code())
	})
}

// TestStatus_RealWorldScenarios tests real-world scenarios
func TestStatus_RealWorldScenarios(t *testing.T) {
	t.Run("gRPC error scenario with reason", func(t *testing.T) {
		err := errors.New("validation failed")
		reason := &testReason{
			reason: "VALIDATION_ERROR",
			domain: "api.example.com",
			code:   code.Code_INVALID_ARGUMENT,
		}
		st := FromReason(err, reason, map[string]string{"field": "email"})
		assert.NotNil(t, st)

		errorInfo := st.ErrorInfo()
		assert.NotNil(t, errorInfo)
		assert.Equal(t, "VALIDATION_ERROR", errorInfo.Reason)
		assert.Equal(t, "api.example.com", errorInfo.Domain)
	})

	t.Run("context deadline scenario", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1)
		defer cancel()
		<-ctx.Done()

		st := FromContextError(ctx.Err())
		assert.Equal(t, code.Code_DEADLINE_EXCEEDED, st.Code())
		assert.Equal(t, int32(http.StatusGatewayTimeout), st.HTTPCode())
	})

	t.Run("wrapping status error", func(t *testing.T) {
		originalSt := WithCode(code.Code_NOT_FOUND, errors.New("not found"))
		wrappedErr := fmt.Errorf("operation failed: %w", originalSt)

		st := FromError(wrappedErr)
		assert.NotNil(t, st)
		assert.Equal(t, code.Code_NOT_FOUND, st.Code())
		assert.True(t, IsCode(wrappedErr, code.Code_NOT_FOUND))
	})

	t.Run("multiple details in status", func(t *testing.T) {
		err := errors.New("complex error")
		st := WithCode(code.Code_INTERNAL, err)

		// Add multiple details
		_ = st.WithDetails(
			&errdetails.ErrorInfo{Reason: "error1", Domain: "domain1"},
			&errdetails.DebugInfo{Detail: "debug info"},
			&errdetails.Help{Links: []*errdetails.Help_Link{{Url: "http://example.com"}}},
		)

		assert.Equal(t, 3, len(st.Status().Details))
	})
}

// TestFromErrorCode tests creating status from error with specific code
func TestFromErrorCode(t *testing.T) {
	t.Run("from nil error", func(t *testing.T) {
		st := FromErrorCode(nil, code.Code_INTERNAL)
		assert.Nil(t, st)
	})

	t.Run("from error with code", func(t *testing.T) {
		err := errors.New("test error")
		st := FromErrorCode(err, code.Code_PERMISSION_DENIED)
		assert.NotNil(t, st)
		assert.Equal(t, code.Code_PERMISSION_DENIED, st.Code())
	})

	t.Run("from status error preserves code", func(t *testing.T) {
		originalSt := WithCode(code.Code_NOT_FOUND, errors.New("not found"))
		wrappedErr := fmt.Errorf("wrapped: %w", originalSt)
		st := FromErrorCode(wrappedErr, code.Code_INTERNAL)
		assert.NotNil(t, st)
		assert.Equal(t, code.Code_NOT_FOUND, st.Code())
	})
}

// TestFromContextError tests creating status from context error
func TestFromContextError(t *testing.T) {
	t.Run("from nil error", func(t *testing.T) {
		st := FromContextError(nil)
		assert.Nil(t, st)
	})

	t.Run("from deadline exceeded", func(t *testing.T) {
		st := FromContextError(context.DeadlineExceeded)
		assert.NotNil(t, st)
		assert.Equal(t, code.Code_DEADLINE_EXCEEDED, st.Code())
	})

	t.Run("from canceled", func(t *testing.T) {
		st := FromContextError(context.Canceled)
		assert.NotNil(t, st)
		assert.Equal(t, code.Code_CANCELLED, st.Code())
	})

	t.Run("from other error", func(t *testing.T) {
		err := errors.New("other error")
		st := FromContextError(err)
		assert.NotNil(t, st)
		assert.Equal(t, code.Code_UNKNOWN, st.Code())
	})

	t.Run("from wrapped deadline exceeded", func(t *testing.T) {
		err := fmt.Errorf("wrapped: %w", context.DeadlineExceeded)
		st := FromContextError(err)
		assert.NotNil(t, st)
		assert.Equal(t, code.Code_DEADLINE_EXCEEDED, st.Code())
	})
}
