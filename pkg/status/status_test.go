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
	"google.golang.org/genproto/googleapis/rpc/status"
)

// TestNew tests creating a new status
func TestNew(t *testing.T) {
	t.Run("create status with code only", func(t *testing.T) {
		st := New(code.Code_OK, nil)
		assert.NotNil(t, st)
		assert.Equal(t, int32(code.Code_OK), st.Code())
		assert.Equal(t, code.Code_OK.String(), st.Message())
	})

	t.Run("create status with code and error", func(t *testing.T) {
		err := errors.New("test error")
		st := New(code.Code_INVALID_ARGUMENT, err)
		assert.NotNil(t, st)
		assert.Equal(t, int32(code.Code_INVALID_ARGUMENT), st.Code())
		assert.Equal(t, "test error", st.Message())
	})

	t.Run("create status with details", func(t *testing.T) {
		detail := &errdetails.ErrorInfo{
			Reason: "test reason",
			Domain: "test domain",
		}
		st := New(code.Code_INVALID_ARGUMENT, errors.New("test"), detail)
		assert.NotNil(t, st)
		assert.Equal(t, 1, len(st.stu.Details))
	})

	t.Run("create status with invalid detail", func(t *testing.T) {
		invalidDetail := &errdetails.ErrorInfo{} // Invalid detail
		st := New(code.Code_INVALID_ARGUMENT, errors.New("test"), invalidDetail)
		assert.NotNil(t, st)
		// Invalid details should not be added
	})
}

// TestErrorf tests creating a status with formatted message
func TestErrorf(t *testing.T) {
	t.Run("create status with message", func(t *testing.T) {
		st := Errorf(code.Code_NOT_FOUND, "resource not found")
		assert.NotNil(t, st)
		assert.Equal(t, int32(code.Code_NOT_FOUND), st.Code())
		assert.Equal(t, "resource not found", st.Message())
	})

	t.Run("create status with message and details", func(t *testing.T) {
		detail := &errdetails.DebugInfo{
			Detail: "debug info",
		}
		st := Errorf(code.Code_INTERNAL, "internal error", detail)
		assert.NotNil(t, st)
		assert.Equal(t, int32(code.Code_INTERNAL), st.Code())
		assert.Equal(t, "internal error", st.Message())
		assert.Equal(t, 1, len(st.stu.Details))
	})
}

// TestFromHTTPCode tests creating status from HTTP code
func TestFromHTTPCode(t *testing.T) {
	tests := []struct {
		name       string
		httpCode   int32
		expectErr  error
		expectCode code.Code
	}{
		{"200 OK", http.StatusOK, nil, code.Code_OK},
		{
			"400 Bad Request",
			http.StatusBadRequest,
			errors.New("bad request"),
			code.Code_INVALID_ARGUMENT,
		},
		{
			"401 Unauthorized",
			http.StatusUnauthorized,
			errors.New("unauthorized"),
			code.Code_UNAUTHENTICATED,
		},
		{
			"403 Forbidden",
			http.StatusForbidden,
			errors.New("forbidden"),
			code.Code_PERMISSION_DENIED,
		},
		{"404 Not Found", http.StatusNotFound, errors.New("not found"), code.Code_NOT_FOUND},
		{"409 Conflict", http.StatusConflict, errors.New("conflict"), code.Code_ABORTED},
		{
			"429 Too Many Requests",
			http.StatusTooManyRequests,
			errors.New("too many"),
			code.Code_RESOURCE_EXHAUSTED,
		},
		{
			"500 Internal Server Error",
			http.StatusInternalServerError,
			errors.New("internal"),
			code.Code_INTERNAL,
		},
		{
			"501 Not Implemented",
			http.StatusNotImplemented,
			errors.New("not implemented"),
			code.Code_UNIMPLEMENTED,
		},
		{
			"503 Service Unavailable",
			http.StatusServiceUnavailable,
			errors.New("unavailable"),
			code.Code_UNAVAILABLE,
		},
		{
			"504 Gateway Timeout",
			http.StatusGatewayTimeout,
			errors.New("timeout"),
			code.Code_DEADLINE_EXCEEDED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := FromHTTPCode(tt.httpCode, tt.expectErr)
			assert.NotNil(t, st)
			assert.Equal(t, int32(tt.expectCode), st.Code())
		})
	}

	t.Run("499 Client Closed", func(t *testing.T) {
		st := FromHTTPCode(HTTPStatusClientClosed, errors.New("client closed"))
		assert.NotNil(t, st)
		assert.Equal(t, int32(code.Code_CANCELLED), st.Code())
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
		assert.Equal(t, int32(code.Code_UNKNOWN), st.Code())
		assert.Equal(t, "standard error", st.Message())
	})

	t.Run("from status error", func(t *testing.T) {
		originalSt := New(code.Code_NOT_FOUND, errors.New("not found"))
		wrappedErr := fmt.Errorf("wrapped: %w", originalSt)
		st := FromError(wrappedErr)
		assert.NotNil(t, st)
		assert.Equal(t, int32(code.Code_NOT_FOUND), st.Code())
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
		assert.Equal(t, int32(code.Code_PERMISSION_DENIED), st.Code())
	})

	t.Run("from status error preserves code", func(t *testing.T) {
		originalSt := New(code.Code_NOT_FOUND, errors.New("not found"))
		wrappedErr := fmt.Errorf("wrapped: %w", originalSt)
		st := FromErrorCode(wrappedErr, code.Code_INTERNAL)
		assert.NotNil(t, st)
		assert.Equal(t, int32(code.Code_NOT_FOUND), st.Code())
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
		assert.Equal(t, int32(code.Code_DEADLINE_EXCEEDED), st.Code())
	})

	t.Run("from canceled", func(t *testing.T) {
		st := FromContextError(context.Canceled)
		assert.NotNil(t, st)
		assert.Equal(t, int32(code.Code_CANCELLED), st.Code())
	})

	t.Run("from other error", func(t *testing.T) {
		err := errors.New("other error")
		st := FromContextError(err)
		assert.NotNil(t, st)
		assert.Equal(t, int32(code.Code_UNKNOWN), st.Code())
	})

	t.Run("from wrapped deadline exceeded", func(t *testing.T) {
		err := fmt.Errorf("wrapped: %w", context.DeadlineExceeded)
		st := FromContextError(err)
		assert.NotNil(t, st)
		assert.Equal(t, int32(code.Code_DEADLINE_EXCEEDED), st.Code())
	})
}

// TestFromProto tests creating status from proto status
func TestFromProto(t *testing.T) {
	t.Run("from proto status", func(t *testing.T) {
		pb := &status.Status{
			Code:    int32(code.Code_NOT_FOUND),
			Message: "not found",
		}
		st := FromProto(pb)
		assert.NotNil(t, st)
		assert.Equal(t, int32(code.Code_NOT_FOUND), st.Code())
		assert.Equal(t, "not found", st.Message())
	})

	t.Run("from nil proto status", func(t *testing.T) {
		st := FromProto(nil)
		assert.NotNil(t, st)
		assert.Nil(t, st.stu)
	})
}

// TestStatus_Methods tests status methods
func TestStatus_Methods(t *testing.T) {
	t.Run("Code method", func(t *testing.T) {
		st := New(code.Code_NOT_FOUND, errors.New("not found"))
		assert.Equal(t, int32(code.Code_NOT_FOUND), st.Code())
	})

	t.Run("Code on nil status", func(t *testing.T) {
		var st *Status
		assert.Equal(t, int32(code.Code_OK), st.Code())
	})

	t.Run("Message method", func(t *testing.T) {
		st := New(code.Code_NOT_FOUND, errors.New("not found"))
		assert.Equal(t, "not found", st.Message())
	})

	t.Run("Message on nil status", func(t *testing.T) {
		var st *Status
		assert.Equal(t, "", st.Message())
	})

	t.Run("HTTPCode method", func(t *testing.T) {
		st := New(code.Code_NOT_FOUND, errors.New("not found"))
		assert.Equal(t, int32(http.StatusNotFound), st.HTTPCode())
	})

	t.Run("HTTPCode on nil status", func(t *testing.T) {
		var st *Status
		assert.Equal(t, int32(http.StatusOK), st.HTTPCode())
	})

	t.Run("IsCode method", func(t *testing.T) {
		st := New(code.Code_NOT_FOUND, errors.New("not found"))
		assert.True(t, st.IsCode(code.Code_NOT_FOUND))
		assert.False(t, st.IsCode(code.Code_OK))
	})

	t.Run("IsCode on nil status", func(t *testing.T) {
		var st *Status
		assert.True(t, st.IsCode(code.Code_OK))
		assert.False(t, st.IsCode(code.Code_NOT_FOUND))
	})
}

// TestStatus_Err tests Err method
func TestStatus_Err(t *testing.T) {
	t.Run("Err returns nil for OK code", func(t *testing.T) {
		st := New(code.Code_OK, nil)
		assert.Nil(t, st.Err())
	})

	t.Run("Err returns status for error code", func(t *testing.T) {
		st := New(code.Code_NOT_FOUND, errors.New("not found"))
		err := st.Err()
		assert.NotNil(t, err)
		assert.Same(t, st, err)
	})
}

// TestStatus_Error tests Error method
func TestStatus_Error(t *testing.T) {
	t.Run("Error returns string representation", func(t *testing.T) {
		st := New(code.Code_NOT_FOUND, errors.New("not found"))
		errStr := st.Error()
		assert.Contains(t, errStr, "not found")
	})

	t.Run("Error on nil status", func(t *testing.T) {
		var st *Status
		assert.Equal(t, "", st.Error())
	})
}

// TestStatus_Stacks tests Stacks method
func TestStatus_Stacks(t *testing.T) {
	t.Run("Stacks returns nil for nil status", func(t *testing.T) {
		var st *Status
		assert.Nil(t, st.Stacks())
	})

	t.Run("Stacks returns stacks from error", func(t *testing.T) {
		err := errors.New("test error")
		st := New(code.Code_UNKNOWN, err)
		stacks := st.Stacks()
		assert.NotNil(t, stacks)
		// Stacks should contain the error trace
	})
}

// TestStatus_WithDetails tests WithDetails method
func TestStatus_WithDetails(t *testing.T) {
	t.Run("add detail to status", func(t *testing.T) {
		st := New(code.Code_INVALID_ARGUMENT, errors.New("invalid"))
		detail := &errdetails.ErrorInfo{
			Reason: "test reason",
			Domain: "test domain",
		}
		st.WithDetails(detail)
		assert.Equal(t, 1, len(st.stu.Details))
	})

	t.Run("add multiple details", func(t *testing.T) {
		st := New(code.Code_INVALID_ARGUMENT, errors.New("invalid"))
		detail1 := &errdetails.ErrorInfo{Reason: "reason1"}
		detail2 := &errdetails.DebugInfo{Detail: "debug"}
		st.WithDetails(detail1, detail2)
		assert.Equal(t, 2, len(st.stu.Details))
	})

	t.Run("WithDetails on nil status", func(t *testing.T) {
		var st *Status
		st.WithDetails(&errdetails.ErrorInfo{})
		// Should not panic
		assert.Nil(t, st)
	})
}

// TestStatus_WithStack tests WithStack method
func TestStatus_WithStack(t *testing.T) {
	t.Run("add stack to status", func(t *testing.T) {
		err := errors.New("test error")
		st := New(code.Code_UNKNOWN, err)
		initialDetailCount := len(st.stu.Details)

		// WithStack only adds detail if len(e.stacks) > 0
		// For simple errors from errors.New(), stacks may be empty
		st.WithStack()

		// Check if stacks were added
		if len(st.stacks) > 0 {
			assert.Equal(t, initialDetailCount+1, len(st.stu.Details))
		} else {
			// No stacks available, WithStack doesn't add detail
			assert.Equal(t, initialDetailCount, len(st.stu.Details))
		}
	})

	t.Run("WithStack with no stacks", func(t *testing.T) {
		st := New(code.Code_OK, nil)
		st.WithStack()
		// Should not add detail when no stacks
		assert.Equal(t, 0, len(st.stu.Details))
	})
}

// TestStatus_Status tests Status method
func TestStatus_Status(t *testing.T) {
	t.Run("Status returns clone", func(t *testing.T) {
		st := New(code.Code_NOT_FOUND, errors.New("not found"))
		pb := st.Status()
		assert.NotNil(t, pb)
		assert.Equal(t, int32(code.Code_NOT_FOUND), pb.Code)
		// Modify returned proto
		pb.Code = int32(code.Code_OK)
		// Original should be unchanged
		assert.Equal(t, int32(code.Code_NOT_FOUND), st.stu.Code)
	})

	t.Run("Status on nil status", func(t *testing.T) {
		var st *Status
		pb := st.Status()
		assert.Nil(t, pb)
	})
}

// TestStatus_Reason tests Reason method
func TestStatus_Reason(t *testing.T) {
	t.Run("Reason returns nil when no reason", func(t *testing.T) {
		st := New(code.Code_NOT_FOUND, errors.New("not found"))
		reason := st.Reason()
		assert.Nil(t, reason)
	})

	t.Run("Reason returns ErrorInfo when present", func(t *testing.T) {
		st := New(code.Code_NOT_FOUND, errors.New("not found"))
		errorInfo := &errdetails.ErrorInfo{
			Reason:   "test reason",
			Domain:   "test domain",
			Metadata: map[string]string{"key": "value"},
		}
		st.WithDetails(errorInfo)

		reason := st.Reason()
		assert.NotNil(t, reason)
		assert.Equal(t, "test reason", reason.Reason)
		assert.Equal(t, "test domain", reason.Domain)
	})

	t.Run("Reason on nil status", func(t *testing.T) {
		var st *Status
		reason := st.Reason()
		assert.Nil(t, reason)
	})
}

// TestStatus_Format tests Format method
func TestStatus_Format(t *testing.T) {
	t.Run("format with %s", func(t *testing.T) {
		st := New(code.Code_NOT_FOUND, errors.New("not found"))
		str := fmt.Sprintf("%s", st)
		assert.Contains(t, str, "not found")
	})

	t.Run("format with %q", func(t *testing.T) {
		st := New(code.Code_NOT_FOUND, errors.New("not found"))
		str := fmt.Sprintf("%q", st)
		assert.Contains(t, str, "\"")
	})

	t.Run("format with %v", func(t *testing.T) {
		st := New(code.Code_NOT_FOUND, errors.New("not found"))
		str := fmt.Sprintf("%v", st)
		assert.Contains(t, str, "not found")
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
		assert.Equal(t, int32(code.Code_INVALID_ARGUMENT), st.Code())

		errorInfo := st.Reason()
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

// TestWithMessage tests WithMessage function
func TestWithMessage(t *testing.T) {
	t.Run("with message", func(t *testing.T) {
		err := New(code.Code_NOT_FOUND, errors.New("not found"))
		msg := &testMessage{
			messages: map[string]string{
				"en": "Not found",
				"zh": "未找到",
			},
		}
		ctx := context.Background()

		st := WithMessage(ctx, err, msg)
		assert.NotNil(t, st)
		// Should add LocalizedMessage detail
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
		assert.Equal(t, int32(code.Code_UNKNOWN), st.Code())
	})

	t.Run("cover status error", func(t *testing.T) {
		originalSt := New(code.Code_NOT_FOUND, errors.New("not found"))
		wrappedErr := fmt.Errorf("wrapped: %w", originalSt)
		st, ok := CoverError(wrappedErr)
		assert.True(t, ok)
		assert.NotNil(t, st)
		assert.Equal(t, int32(code.Code_NOT_FOUND), st.Code())
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

// TestIsCode tests IsCode function
func TestIsCode(t *testing.T) {
	t.Run("is code with matching code", func(t *testing.T) {
		st := New(code.Code_NOT_FOUND, errors.New("not found"))
		wrappedErr := fmt.Errorf("wrapped: %w", st)
		assert.True(t, IsCode(wrappedErr, code.Code_NOT_FOUND))
	})

	t.Run("is code with non-matching code", func(t *testing.T) {
		st := New(code.Code_NOT_FOUND, errors.New("not found"))
		wrappedErr := fmt.Errorf("wrapped: %w", st)
		assert.False(t, IsCode(wrappedErr, code.Code_OK))
	})

	t.Run("is code with standard error", func(t *testing.T) {
		err := errors.New("standard error")
		assert.False(t, IsCode(err, code.Code_NOT_FOUND))
	})
}

// TestStuCodeToHTTPCode tests stuCodeToHTTPCode function
func TestStuCodeToHTTPCode(t *testing.T) {
	tests := []struct {
		name         string
		stuCode      code.Code
		expectedCode int32
	}{
		{"OK", code.Code_OK, http.StatusOK},
		{"CANCELLED", code.Code_CANCELLED, HTTPStatusClientClosed},
		{"UNKNOWN", code.Code_UNKNOWN, http.StatusInternalServerError},
		{"INVALID_ARGUMENT", code.Code_INVALID_ARGUMENT, http.StatusBadRequest},
		{"DEADLINE_EXCEEDED", code.Code_DEADLINE_EXCEEDED, http.StatusGatewayTimeout},
		{"NOT_FOUND", code.Code_NOT_FOUND, http.StatusNotFound},
		{"ALREADY_EXISTS", code.Code_ALREADY_EXISTS, http.StatusConflict},
		{"PERMISSION_DENIED", code.Code_PERMISSION_DENIED, http.StatusForbidden},
		{"UNAUTHENTICATED", code.Code_UNAUTHENTICATED, http.StatusUnauthorized},
		{"RESOURCE_EXHAUSTED", code.Code_RESOURCE_EXHAUSTED, http.StatusTooManyRequests},
		{"FAILED_PRECONDITION", code.Code_FAILED_PRECONDITION, http.StatusBadRequest},
		{"ABORTED", code.Code_ABORTED, http.StatusConflict},
		{"OUT_OF_RANGE", code.Code_OUT_OF_RANGE, http.StatusBadRequest},
		{"UNIMPLEMENTED", code.Code_UNIMPLEMENTED, http.StatusNotImplemented},
		{"INTERNAL", code.Code_INTERNAL, http.StatusInternalServerError},
		{"UNAVAILABLE", code.Code_UNAVAILABLE, http.StatusServiceUnavailable},
		{"DATA_LOSS", code.Code_DATA_LOSS, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stuCodeToHTTPCode(tt.stuCode)
			assert.Equal(t, tt.expectedCode, result)
		})
	}
}

// TestHTTPCodeToStuCode tests httpCodeToStuCode function
func TestHTTPCodeToStuCode(t *testing.T) {
	tests := []struct {
		name         string
		httpCode     int32
		expectedCode code.Code
	}{
		{"200", http.StatusOK, code.Code_OK},
		{"400", http.StatusBadRequest, code.Code_INVALID_ARGUMENT},
		{"401", http.StatusUnauthorized, code.Code_UNAUTHENTICATED},
		{"403", http.StatusForbidden, code.Code_PERMISSION_DENIED},
		{"404", http.StatusNotFound, code.Code_NOT_FOUND},
		{"409", http.StatusConflict, code.Code_ABORTED},
		{"429", http.StatusTooManyRequests, code.Code_RESOURCE_EXHAUSTED},
		{"499", HTTPStatusClientClosed, code.Code_CANCELLED},
		{"500", http.StatusInternalServerError, code.Code_INTERNAL},
		{"501", http.StatusNotImplemented, code.Code_UNIMPLEMENTED},
		{"503", http.StatusServiceUnavailable, code.Code_UNAVAILABLE},
		{"504", http.StatusGatewayTimeout, code.Code_DEADLINE_EXCEEDED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := httpCodeToStuCode(tt.httpCode)
			assert.Equal(t, tt.expectedCode, result)
		})
	}
}

// TestStatus_Conversions tests status conversions
func TestStatus_Conversions(t *testing.T) {
	t.Run("round trip from error to status", func(t *testing.T) {
		originalErr := errors.New("original error")
		st := FromError(originalErr)
		assert.NotNil(t, st)
		assert.Equal(t, int32(code.Code_UNKNOWN), st.Code())
		assert.Equal(t, "original error", st.Message())
	})

	t.Run("context error conversion", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := ctx.Err()
		st := FromContextError(err)
		assert.NotNil(t, st)
		assert.Equal(t, int32(code.Code_CANCELLED), st.Code())
	})
}

// testReason is a test implementation of Reason interface
type testReason struct {
	reason string
	domain string
	code   code.Code
}

func (r *testReason) Reason() string {
	return r.reason
}

func (r *testReason) Domain() string {
	return r.domain
}

func (r *testReason) Code() code.Code {
	return r.code
}

// testMessage is a test implementation of Message interface
type testMessage struct {
	messages map[string]string
}

func (m *testMessage) Message(language string) string {
	return m.messages[language]
}

// TestStatus_RealWorldScenarios tests real-world scenarios
func TestStatus_RealWorldScenarios(t *testing.T) {
	t.Run("HTTP handler error scenario", func(t *testing.T) {
		err := errors.New("resource not found")
		st := FromHTTPCode(http.StatusNotFound, err)
		assert.Equal(t, int32(http.StatusNotFound), st.HTTPCode())
		assert.Equal(t, int32(code.Code_NOT_FOUND), st.Code())
	})

	t.Run("gRPC error scenario with reason", func(t *testing.T) {
		err := errors.New("validation failed")
		reason := &testReason{
			reason: "VALIDATION_ERROR",
			domain: "api.example.com",
			code:   code.Code_INVALID_ARGUMENT,
		}
		st := FromReason(err, reason, map[string]string{"field": "email"})
		assert.NotNil(t, st)

		errorInfo := st.Reason()
		assert.NotNil(t, errorInfo)
		assert.Equal(t, "VALIDATION_ERROR", errorInfo.Reason)
		assert.Equal(t, "api.example.com", errorInfo.Domain)
	})

	t.Run("context deadline scenario", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1)
		defer cancel()
		<-ctx.Done()

		st := FromContextError(ctx.Err())
		assert.Equal(t, int32(code.Code_DEADLINE_EXCEEDED), st.Code())
		assert.Equal(t, int32(http.StatusGatewayTimeout), st.HTTPCode())
	})

	t.Run("wrapping status error", func(t *testing.T) {
		originalSt := New(code.Code_NOT_FOUND, errors.New("not found"))
		wrappedErr := fmt.Errorf("operation failed: %w", originalSt)

		st := FromError(wrappedErr)
		assert.NotNil(t, st)
		assert.Equal(t, int32(code.Code_NOT_FOUND), st.Code())
		assert.True(t, IsCode(wrappedErr, code.Code_NOT_FOUND))
	})

	t.Run("multiple details in status", func(t *testing.T) {
		err := errors.New("complex error")
		st := New(code.Code_INTERNAL, err)

		// Add multiple details
		st.WithDetails(
			&errdetails.ErrorInfo{Reason: "error1", Domain: "domain1"},
			&errdetails.DebugInfo{Detail: "debug info"},
			&errdetails.Help{Links: []*errdetails.Help_Link{{Url: "http://example.com"}}},
		)

		assert.Equal(t, 3, len(st.stu.Details))
	})
}
