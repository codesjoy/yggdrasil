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

// Package status provides a way to handle error details.
package status

import (
	"context"
	"errors"

	istatus "github.com/codesjoy/yggdrasil/v2/internal/status"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/genproto/googleapis/rpc/status"
)

// Status is the status of the error.
type Status = istatus.Status

// HTTPCodeToStuCode converts HTTP status code to RPC status code.
func HTTPCodeToStuCode(httpCode int32) code.Code {
	return istatus.HTTPCodeToStuCode(httpCode)
}

// New creates a new status with error message.
func New(code code.Code, msg string) *Status {
	return istatus.New(code, msg)
}

// WithCode creates a new status from code and error.
func WithCode(code code.Code, err error) *Status {
	return istatus.WithCode(code, err)
}

// FromReason creates a new status with error message and reason.
func FromReason(err error, reason Reason, metadata map[string]string) *Status {
	e := FromErrorCode(err, reason.Code())
	if e == nil {
		return nil
	}
	_ = e.WithDetails(NewReason(reason, metadata))
	return e
}

// FromError creates a new status with error message.
func FromError(err error) *Status {
	return FromErrorCode(err, code.Code_UNKNOWN)
}

// CoverError converts a non-nil error into a Status.
func CoverError(err error) (*Status, bool) {
	st, ok := istatus.CoverError(err)
	if ok {
		return st, ok
	}
	return WithCode(code.Code_UNKNOWN, err), false
}

// FromErrorCode creates a new status with error message and code.
func FromErrorCode(err error, code2 code.Code) *Status {
	st, ok := istatus.CoverError(err)
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
func FromProto(stu *status.Status) *Status {
	return istatus.FromProto(stu)
}

// IsReason returns true if the error is a reason.
func IsReason(err error, targets ...Reason) bool {
	e, ok := istatus.CoverError(err)
	if !ok {
		return false
	}
	src := e.ErrorInfo()
	if src == nil {
		return false
	}
	for _, target := range targets {
		if src.GetReason() == target.Reason() && src.GetDomain() == target.Domain() {
			return true
		}
	}

	return false
}

// IsCode returns true if the error is a code.
func IsCode(err error, code2 code.Code) bool {
	e, ok := istatus.CoverError(err)
	if !ok {
		return false
	}
	return e.Code() == code2
}
