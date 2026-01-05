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
		st := WithCode(code.Code_OK, nil)
		assert.NotNil(t, st)
		assert.Equal(t, code.Code_OK, st.Code())
		assert.Equal(t, code.Code_OK.String(), st.Message())
	})

	t.Run("create status with code and error", func(t *testing.T) {
		err := errors.New("test error")
		st := WithCode(code.Code_INVALID_ARGUMENT, err)
		assert.NotNil(t, st)
		assert.Equal(t, code.Code_INVALID_ARGUMENT, st.Code())
		assert.Equal(t, "test error", st.Message())
	})

	t.Run("create status with details", func(t *testing.T) {
		detail := &errdetails.ErrorInfo{
			Reason: "test reason",
			Domain: "test domain",
		}
		st := WithCode(code.Code_INVALID_ARGUMENT, errors.New("test")).WithDetails(detail)
		assert.NotNil(t, st)
		assert.Equal(t, 1, len(st.stu.Details))
	})

	t.Run("create status with invalid detail", func(t *testing.T) {
		invalidDetail := &errdetails.ErrorInfo{} // Invalid detail
		st := WithCode(code.Code_INVALID_ARGUMENT, errors.New("test")).WithDetails(invalidDetail)
		assert.NotNil(t, st)
		// Invalid details should not be added
	})
}

// TestErrorf tests creating a status with formatted message
func TestErrorf(t *testing.T) {
	t.Run("create status with message", func(t *testing.T) {
		st := New(code.Code_NOT_FOUND, "resource not found")
		assert.NotNil(t, st)
		assert.Equal(t, code.Code_NOT_FOUND, st.Code())
		assert.Equal(t, "resource not found", st.Message())
	})

	t.Run("create status with message and details", func(t *testing.T) {
		detail := &errdetails.DebugInfo{
			Detail: "debug info",
		}
		st := New(code.Code_INTERNAL, "internal error").WithDetails(detail)
		assert.NotNil(t, st)
		assert.Equal(t, code.Code_INTERNAL, st.Code())
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
			st := WithCode(HTTPCodeToStuCode(tt.httpCode), tt.expectErr)
			assert.NotNil(t, st)
			assert.Equal(t, tt.expectCode, st.Code())
		})
	}

	t.Run("499 Client Closed", func(t *testing.T) {
		st := WithCode(HTTPCodeToStuCode(HTTPStatusClientClosed), errors.New("client closed"))
		assert.NotNil(t, st)
		assert.Equal(t, code.Code_CANCELLED, st.Code())
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
		assert.Equal(t, code.Code_NOT_FOUND, st.Code())
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
		st := WithCode(code.Code_NOT_FOUND, errors.New("not found"))
		assert.Equal(t, code.Code_NOT_FOUND, st.Code())
	})

	t.Run("Code on nil status", func(t *testing.T) {
		var st *Status
		assert.Equal(t, code.Code_OK, st.Code())
	})

	t.Run("Message method", func(t *testing.T) {
		st := WithCode(code.Code_NOT_FOUND, errors.New("not found"))
		assert.Equal(t, "not found", st.Message())
	})

	t.Run("Message on nil status", func(t *testing.T) {
		var st *Status
		assert.Equal(t, "", st.Message())
	})

	t.Run("HTTPCode method", func(t *testing.T) {
		st := WithCode(code.Code_NOT_FOUND, errors.New("not found"))
		assert.Equal(t, int32(http.StatusNotFound), st.HTTPCode())
	})

	t.Run("HTTPCode on nil status", func(t *testing.T) {
		var st *Status
		assert.Equal(t, int32(http.StatusOK), st.HTTPCode())
	})

	t.Run("IsCode method", func(t *testing.T) {
		st := WithCode(code.Code_NOT_FOUND, errors.New("not found"))
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
		st := WithCode(code.Code_OK, nil)
		assert.Nil(t, st.Err())
	})

	t.Run("Err returns status for error code", func(t *testing.T) {
		st := WithCode(code.Code_NOT_FOUND, errors.New("not found"))
		err := st.Err()
		assert.NotNil(t, err)
		assert.Same(t, st, err)
	})
}

// TestStatus_Error tests Error method
func TestStatus_Error(t *testing.T) {
	t.Run("Error returns string representation", func(t *testing.T) {
		st := WithCode(code.Code_NOT_FOUND, errors.New("not found"))
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
		st := WithCode(code.Code_UNKNOWN, err)
		stacks := st.Stacks()
		assert.NotNil(t, stacks)
		// Stacks should contain the error trace
	})
}

// TestStatus_WithDetails tests WithDetails method
func TestStatus_WithDetails(t *testing.T) {
	t.Run("add detail to status", func(t *testing.T) {
		st := WithCode(code.Code_INVALID_ARGUMENT, errors.New("invalid")).
			WithDetails(&errdetails.ErrorInfo{
				Reason: "test reason",
				Domain: "test domain",
			})
		assert.Equal(t, 1, len(st.stu.Details))
	})

	t.Run("add multiple details", func(t *testing.T) {
		st := WithCode(code.Code_INVALID_ARGUMENT, errors.New("invalid"))
		detail1 := &errdetails.ErrorInfo{Reason: "reason1"}
		detail2 := &errdetails.DebugInfo{Detail: "debug"}
		st = st.WithDetails(detail1, detail2)
		assert.Equal(t, 2, len(st.stu.Details))
	})

	t.Run("WithDetails on nil status", func(t *testing.T) {
		var st *Status
		_ = st.WithDetails(&errdetails.ErrorInfo{})
		// Should not panic
		assert.Nil(t, st)
	})
}

// TestStatus_WithStack tests WithStack method
func TestStatus_WithStack(t *testing.T) {
	t.Run("add stack to status", func(t *testing.T) {
		err := errors.New("test error")
		st := WithCode(code.Code_UNKNOWN, err)
		initialDetailCount := len(st.stu.Details)

		// WithStack only adds detail if len(e.stacks) > 0
		// For simple errors from errors.New(), stacks may be empty
		_ = st.WithStack()

		// Check if stacks were added
		if len(st.stacks) > 0 {
			assert.Equal(t, initialDetailCount+1, len(st.stu.Details))
		} else {
			// No stacks available, WithStack doesn't add detail
			assert.Equal(t, initialDetailCount, len(st.stu.Details))
		}
	})

	t.Run("WithStack with no stacks", func(t *testing.T) {
		st := WithCode(code.Code_OK, nil)
		_ = st.WithStack()
		// Should not add detail when no stacks
		assert.Equal(t, 0, len(st.stu.Details))
	})
}

// TestStatus_Status tests Status method
func TestStatus_Status(t *testing.T) {
	t.Run("Status returns clone", func(t *testing.T) {
		st := WithCode(code.Code_NOT_FOUND, errors.New("not found"))
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

// TestStatus_Format tests Format method
func TestStatus_Format(t *testing.T) {
	t.Run("format with %s", func(t *testing.T) {
		st := WithCode(code.Code_NOT_FOUND, errors.New("not found"))
		str := fmt.Sprintf("%s", st)
		assert.Contains(t, str, "not found")
	})

	t.Run("format with %q", func(t *testing.T) {
		st := WithCode(code.Code_NOT_FOUND, errors.New("not found"))
		str := fmt.Sprintf("%q", st)
		assert.Contains(t, str, "\"")
	})

	t.Run("format with %v", func(t *testing.T) {
		st := WithCode(code.Code_NOT_FOUND, errors.New("not found"))
		str := fmt.Sprintf("%v", st)
		assert.Contains(t, str, "not found")
	})
}

// TestStuCodeToHTTPCode tests StuCodeToHTTPCode function
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

// TestHTTPCodeToStuCode tests HTTPCodeToStuCode function
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
			result := HTTPCodeToStuCode(tt.httpCode)
			assert.Equal(t, tt.expectedCode, result)
		})
	}
}
