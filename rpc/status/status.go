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

// Package status provides a way to handle RPC status details.
package status

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	// HTTPStatusClientClosed represents the http status code for client closed.
	HTTPStatusClientClosed = 499
)

// Status represents a status.
type Status struct {
	stu *statuspb.Status
}

// New creates a new status from code and message.
func New(code code.Code, msg string) *Status {
	return &Status{stu: &statuspb.Status{
		Code:    int32(code),
		Message: msg,
	}}
}

// WithCode creates a new status from code and error.
func WithCode(code code.Code, err error) *Status {
	selfErr := &Status{stu: &statuspb.Status{
		Code: int32(code),
	}}
	if err == nil {
		selfErr.stu.Message = code.String()
	} else {
		selfErr.stu.Message = err.Error()
	}
	return selfErr
}

// WithDetails adds details to the status.
func (e *Status) WithDetails(details ...proto.Message) *Status {
	if e == nil || e.stu == nil {
		return e
	}
	for _, detail := range details {
		detail, _ := anypb.New(detail)
		e.stu.Details = append(e.stu.Details, detail)
	}
	return e
}

// HTTPCode returns the http status code of the status.
func (e *Status) HTTPCode() int32 {
	if e == nil || e.stu == nil {
		return http.StatusOK
	}
	return statusCodeToHTTPCode(code.Code(e.stu.Code))
}

// Code returns the code of the status.
func (e *Status) Code() code.Code {
	if e == nil || e.stu == nil {
		return code.Code_OK
	}
	return code.Code(e.stu.Code)
}

// IsCode returns true if the status code is equal to the given code.
func (e *Status) IsCode(c code.Code) bool {
	if e == nil || e.stu == nil {
		return code.Code_OK == c
	}
	return e.stu.Code == int32(c)
}

// Err returns the error of the status.
func (e *Status) Err() error {
	if e.Code() == code.Code_OK {
		return nil
	}
	return e
}

// Error returns the error message of the status.
func (e *Status) Error() string {
	if e == nil || e.stu == nil {
		return ""
	}
	return e.stu.String()
}

// Message returns the message of the status.
func (e *Status) Message() string {
	if e == nil || e.stu == nil {
		return ""
	}
	return e.stu.Message
}

// Status returns the protobuf status.
func (e *Status) Status() *statuspb.Status {
	if e == nil || e.stu == nil {
		return nil
	}
	return proto.Clone(e.stu).(*statuspb.Status)
}

// ErrorInfo returns the reason of the status.
func (e *Status) ErrorInfo() *errdetails.ErrorInfo {
	if e != nil {
		reason := &errdetails.ErrorInfo{}
		for _, detail := range e.stu.Details {
			if detail.MessageIs(reason) {
				_ = detail.UnmarshalTo(reason)
				return reason
			}
		}
	}
	return nil
}

// Format formats the status.
func (e *Status) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			_, _ = io.WriteString(s, e.Message())
			return
		}
		fallthrough
	case 's':
		_, _ = io.WriteString(s, e.Message())
	case 'q':
		_, _ = fmt.Fprintf(s, "%q", e.Error())
	}
}

// HTTPCodeToStuCode converts HTTP status code to RPC status code.
func HTTPCodeToStuCode(httpCode int32) code.Code {
	switch httpCode {
	case http.StatusOK:
		return code.Code_OK
	case HTTPStatusClientClosed:
		return code.Code_CANCELLED
	case http.StatusBadRequest:
		return code.Code_INVALID_ARGUMENT
	case http.StatusGatewayTimeout:
		return code.Code_DEADLINE_EXCEEDED
	case http.StatusNotFound:
		return code.Code_NOT_FOUND
	case http.StatusForbidden:
		return code.Code_PERMISSION_DENIED
	case http.StatusUnauthorized:
		return code.Code_UNAUTHENTICATED
	case http.StatusTooManyRequests:
		return code.Code_RESOURCE_EXHAUSTED
	case http.StatusConflict:
		return code.Code_ABORTED
	case http.StatusNotImplemented:
		return code.Code_UNIMPLEMENTED
	case http.StatusInternalServerError:
		return code.Code_INTERNAL
	case http.StatusServiceUnavailable:
		return code.Code_UNAVAILABLE
	}
	return code.Code_INTERNAL
}

// FromError creates a new status with error message.
func FromError(err error) *Status {
	return FromErrorCode(err, code.Code_UNKNOWN)
}

// CoverError converts a non-nil error into a Status.
func CoverError(err error) (*Status, bool) {
	if err == nil {
		return nil, true
	}

	var s *Status
	ok := errors.As(err, &s)
	if ok {
		return s, true
	}

	s, ok = fromXError(err)
	if ok {
		return s, true
	}

	return WithCode(code.Code_UNKNOWN, err), false
}

// FromErrorCode creates a new status with error message and code.
func FromErrorCode(err error, code2 code.Code) *Status {
	st, ok := CoverError(err)
	if ok {
		return st
	}
	return WithCode(code2, err)
}

// FromContextError converts a context reason or wrapped context reason into a
// Status.  It returns a Status with codes.OK if err is nil, or a Status with
// codes.Unknown if err is non-nil and not a context reason.
func FromContextError(err error) *Status {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return WithCode(code.Code_DEADLINE_EXCEEDED, err)
	}
	if errors.Is(err, context.Canceled) {
		return WithCode(code.Code_CANCELLED, err)
	}
	return WithCode(code.Code_UNKNOWN, err)
}

// FromProto creates a new status from a protobuf status.
func FromProto(stu *statuspb.Status) *Status {
	return &Status{stu: stu}
}

func statusCodeToHTTPCode(stuCode code.Code) int32 {
	switch stuCode {
	case code.Code_OK:
		return http.StatusOK
	case code.Code_CANCELLED:
		return HTTPStatusClientClosed
	case code.Code_UNKNOWN:
		return http.StatusInternalServerError
	case code.Code_INVALID_ARGUMENT:
		return http.StatusBadRequest
	case code.Code_DEADLINE_EXCEEDED:
		return http.StatusGatewayTimeout
	case code.Code_NOT_FOUND:
		return http.StatusNotFound
	case code.Code_ALREADY_EXISTS:
		return http.StatusConflict
	case code.Code_PERMISSION_DENIED:
		return http.StatusForbidden
	case code.Code_UNAUTHENTICATED:
		return http.StatusUnauthorized
	case code.Code_RESOURCE_EXHAUSTED:
		return http.StatusTooManyRequests
	case code.Code_FAILED_PRECONDITION:
		return http.StatusBadRequest
	case code.Code_ABORTED:
		return http.StatusConflict
	case code.Code_OUT_OF_RANGE:
		return http.StatusBadRequest
	case code.Code_UNIMPLEMENTED:
		return http.StatusNotImplemented
	case code.Code_INTERNAL:
		return http.StatusInternalServerError
	case code.Code_UNAVAILABLE:
		return http.StatusServiceUnavailable
	case code.Code_DATA_LOSS:
		return http.StatusInternalServerError
	}
	return http.StatusInternalServerError
}
