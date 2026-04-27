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

import "fmt"

// ErrorCode is one stable planner/app error code.
type ErrorCode string

// Error codes for assembly and install failures.
const (
	ErrInvalidMode                      ErrorCode = "InvalidMode"
	ErrInvalidAutoRule                  ErrorCode = "InvalidAutoRule"
	ErrUnknownTemplate                  ErrorCode = "UnknownTemplate"
	ErrTemplateVersionNotFound          ErrorCode = "TemplateVersionNotFound"
	ErrAmbiguousDefault                 ErrorCode = "AmbiguousDefault"
	ErrConflictingOverride              ErrorCode = "ConflictingOverride"
	ErrUnknownExplicitBinding           ErrorCode = "UnknownExplicitBinding"
	ErrComposeFailed                    ErrorCode = "ComposeFailed"
	ErrRuntimeNotReady                  ErrorCode = "RuntimeNotReady"
	ErrRuntimeSurfaceUnavailable        ErrorCode = "RuntimeSurfaceUnavailable"
	ErrPreparedRuntimeRollbackFailed    ErrorCode = "PreparedRuntimeRollbackFailed"
	ErrPartialInstallRollbackFailed     ErrorCode = "PartialInstallRollbackFailed"
	ErrComposeLocalResourceLeaked       ErrorCode = "ComposeLocalResourceLeaked"
	ErrInstallValidationFailed          ErrorCode = "InstallValidationFailed"
	ErrInstallRegistrationConflict      ErrorCode = "InstallRegistrationConflict"
	ErrPlanDiffInconsistent             ErrorCode = "PlanDiffInconsistent"
	ErrReloadRequiresRestart            ErrorCode = "ReloadRequiresRestart"
	ErrRuntimeReconcileFailed           ErrorCode = "RuntimeReconcileFailed"
	ErrIsolationRequiresProcessDefaults ErrorCode = "IsolationRequiresProcessDefaults"
)

// Error is one structured assembly/app error.
type Error struct {
	Code    ErrorCode         `json:"code"              yaml:"code"`
	Stage   string            `json:"stage"             yaml:"stage"`
	Message string            `json:"message"           yaml:"message"`
	Context map[string]string `json:"context,omitempty" yaml:"context,omitempty"`
	Cause   error             `json:"-"                 yaml:"-"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Code == "" {
		return e.Message
	}
	if e.Message == "" {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the wrapped cause.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// NewError builds one structured assembly/app error.
func NewError(code ErrorCode, stage, message string, cause error, ctx map[string]string) *Error {
	return &Error{
		Code:    code,
		Stage:   stage,
		Message: message,
		Context: ctx,
		Cause:   cause,
	}
}

func newError(code ErrorCode, stage, message string, cause error, ctx map[string]string) error {
	return NewError(code, stage, message, cause, ctx)
}
