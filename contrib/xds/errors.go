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

package xds

import (
	"fmt"
)

// XDSError represents an xDS-specific error
//
//nolint:revive // XDSError name stutter is acceptable for domain-specific error type
type XDSError struct {
	Code    string
	Message string
	Cause   error
}

func (e *XDSError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("xds[%s]: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("xds[%s]: %s", e.Code, e.Message)
}

func (e *XDSError) Unwrap() error {
	return e.Cause
}

// NewXDSError creates a new xDS error with the given code, message, and cause
func NewXDSError(code, message string, cause error) *XDSError {
	return &XDSError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

const (
	// ErrCodeConnectionFailed indicates connection to xDS server failed
	ErrCodeConnectionFailed = "CONNECTION_FAILED"
	// ErrCodeSubscriptionFailed indicates resource subscription failed
	ErrCodeSubscriptionFailed = "SUBSCRIPTION_FAILED"
	// ErrCodeResourceNotFound indicates requested resource was not found
	ErrCodeResourceNotFound = "RESOURCE_NOT_FOUND"
	// ErrCodeUnmarshalFailed indicates resource unmarshaling failed
	ErrCodeUnmarshalFailed = "UNMARSHAL_FAILED"
	// ErrCodeClientClosed indicates xDS client is closed
	ErrCodeClientClosed = "CLIENT_CLOSED"
	// ErrCodeRateLimitExceeded indicates rate limit was exceeded
	ErrCodeRateLimitExceeded = "RATE_LIMIT_EXCEEDED"
	// ErrCodeCircuitBreakerOpen indicates circuit breaker is open
	ErrCodeCircuitBreakerOpen = "CIRCUIT_BREAKER_OPEN"
	// ErrCodeNoAvailableInstance indicates no available instances
	ErrCodeNoAvailableInstance = "NO_AVAILABLE_INSTANCE"
	// ErrCodeInvalidConfig indicates invalid configuration
	ErrCodeInvalidConfig = "INVALID_CONFIG"
)

// ErrConnectionFailed creates an error for connection failures
func ErrConnectionFailed(cause error) error {
	return NewXDSError(ErrCodeConnectionFailed, "failed to connect to xDS server", cause)
}

// ErrSubscriptionFailed creates an error for subscription failures
func ErrSubscriptionFailed(resourceType, resourceNames string, cause error) error {
	return NewXDSError(ErrCodeSubscriptionFailed,
		fmt.Sprintf("failed to subscribe to %s: %s", resourceType, resourceNames), cause)
}

// ErrResourceNotFound creates an error for missing resources
func ErrResourceNotFound(resourceType, resourceName string) error {
	return NewXDSError(ErrCodeResourceNotFound,
		fmt.Sprintf("resource not found: %s/%s", resourceType, resourceName), nil)
}

// ErrUnmarshalFailed creates an error for unmarshaling failures
func ErrUnmarshalFailed(resourceType string, cause error) error {
	return NewXDSError(ErrCodeUnmarshalFailed,
		fmt.Sprintf("failed to unmarshal %s resource", resourceType), cause)
}

// ErrClientClosed creates an error for closed client
func ErrClientClosed() error {
	return NewXDSError(ErrCodeClientClosed, "xDS client is closed", nil)
}

// ErrRateLimitExceeded creates an error for rate limit exceeded
func ErrRateLimitExceeded() error {
	return NewXDSError(ErrCodeRateLimitExceeded, "rate limit exceeded", nil)
}

// ErrCircuitBreakerOpen creates an error for open circuit breaker
func ErrCircuitBreakerOpen() error {
	return NewXDSError(ErrCodeCircuitBreakerOpen, "circuit breaker is open", nil)
}

// ErrInvalidConfig creates an error for invalid configuration
func ErrInvalidConfig(message string) error {
	return NewXDSError(ErrCodeInvalidConfig, message, nil)
}
