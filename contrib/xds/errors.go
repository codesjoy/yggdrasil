package xds

import (
	"fmt"
)

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

func NewXDSError(code, message string, cause error) *XDSError {
	return &XDSError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

const (
	ErrCodeConnectionFailed    = "CONNECTION_FAILED"
	ErrCodeSubscriptionFailed  = "SUBSCRIPTION_FAILED"
	ErrCodeResourceNotFound    = "RESOURCE_NOT_FOUND"
	ErrCodeUnmarshalFailed     = "UNMARSHAL_FAILED"
	ErrCodeClientClosed        = "CLIENT_CLOSED"
	ErrCodeRateLimitExceeded   = "RATE_LIMIT_EXCEEDED"
	ErrCodeCircuitBreakerOpen  = "CIRCUIT_BREAKER_OPEN"
	ErrCodeNoAvailableInstance = "NO_AVAILABLE_INSTANCE"
	ErrCodeInvalidConfig       = "INVALID_CONFIG"
)

func ErrConnectionFailed(cause error) error {
	return NewXDSError(ErrCodeConnectionFailed, "failed to connect to xDS server", cause)
}

func ErrSubscriptionFailed(resourceType, resourceNames string, cause error) error {
	return NewXDSError(ErrCodeSubscriptionFailed,
		fmt.Sprintf("failed to subscribe to %s: %s", resourceType, resourceNames), cause)
}

func ErrResourceNotFound(resourceType, resourceName string) error {
	return NewXDSError(ErrCodeResourceNotFound,
		fmt.Sprintf("resource not found: %s/%s", resourceType, resourceName), nil)
}

func ErrUnmarshalFailed(resourceType string, cause error) error {
	return NewXDSError(ErrCodeUnmarshalFailed,
		fmt.Sprintf("failed to unmarshal %s resource", resourceType), cause)
}

func ErrClientClosed() error {
	return NewXDSError(ErrCodeClientClosed, "xDS client is closed", nil)
}

func ErrRateLimitExceeded() error {
	return NewXDSError(ErrCodeRateLimitExceeded, "rate limit exceeded", nil)
}

func ErrCircuitBreakerOpen() error {
	return NewXDSError(ErrCodeCircuitBreakerOpen, "circuit breaker is open", nil)
}

func ErrInvalidConfig(message string) error {
	return NewXDSError(ErrCodeInvalidConfig, message, nil)
}
