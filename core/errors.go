// Package core provides error types for GopherQueue.
package core

import (
	"context"
	"errors"
	"fmt"
)

// Common errors.
var (
	ErrJobNotFound       = errors.New("job not found")
	ErrJobAlreadyExists  = errors.New("job already exists")
	ErrJobNotCancellable = errors.New("job cannot be cancelled in current state")
	ErrQueueFull         = errors.New("job queue is full")
	ErrShuttingDown      = errors.New("system is shutting down")
	ErrRateLimited       = errors.New("rate limit exceeded")
)

// ErrorCategory classifies errors for retry decisions.
type ErrorCategory string

const (
	ErrorCategoryTemporary ErrorCategory = "temporary"
	ErrorCategoryPermanent ErrorCategory = "permanent"
	ErrorCategoryTimeout   ErrorCategory = "timeout"
	ErrorCategorySystem    ErrorCategory = "system"
	ErrorCategoryRateLimit ErrorCategory = "rate_limit"
)

// TemporaryError indicates a retriable failure.
type TemporaryError struct {
	Message    string
	Underlying error
}

func (e *TemporaryError) Error() string {
	if e.Underlying != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Underlying)
	}
	return e.Message
}

func (e *TemporaryError) Unwrap() error {
	return e.Underlying
}

func (e *TemporaryError) Temporary() bool {
	return true
}

// PermanentError indicates a non-retriable failure.
type PermanentError struct {
	Message    string
	Underlying error
}

func (e *PermanentError) Error() string {
	if e.Underlying != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Underlying)
	}
	return e.Message
}

func (e *PermanentError) Unwrap() error {
	return e.Underlying
}

func (e *PermanentError) Permanent() bool {
	return true
}

// TimeoutError indicates a timeout failure.
type TimeoutError struct {
	Message string
}

func (e *TimeoutError) Error() string {
	return e.Message
}

func (e *TimeoutError) Timeout() bool {
	return true
}

// RateLimitError indicates a rate limit failure.
type RateLimitError struct {
	Message string
	RetryIn int64 // seconds
}

func (e *RateLimitError) Error() string {
	return e.Message
}

func (e *RateLimitError) RateLimited() bool {
	return true
}

// ValidationError indicates invalid input.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}

// ClassifyError determines the category of an error.
func ClassifyError(err error) ErrorCategory {
	if err == nil {
		return ""
	}

	// Check for context errors
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrorCategoryTimeout
	}
	if errors.Is(err, context.Canceled) {
		return ErrorCategorySystem
	}

	// Check for typed errors
	var tempErr *TemporaryError
	if errors.As(err, &tempErr) {
		return ErrorCategoryTemporary
	}

	var permErr *PermanentError
	if errors.As(err, &permErr) {
		return ErrorCategoryPermanent
	}

	var timeoutErr *TimeoutError
	if errors.As(err, &timeoutErr) {
		return ErrorCategoryTimeout
	}

	var rateLimitErr *RateLimitError
	if errors.As(err, &rateLimitErr) {
		return ErrorCategoryRateLimit
	}

	// Default to temporary (retriable)
	return ErrorCategoryTemporary
}

// IsRetriable returns whether an error should be retried.
func IsRetriable(err error) bool {
	category := ClassifyError(err)
	return category == ErrorCategoryTemporary || category == ErrorCategoryRateLimit
}

// NewTemporaryError creates a new temporary error.
func NewTemporaryError(message string, underlying error) *TemporaryError {
	return &TemporaryError{Message: message, Underlying: underlying}
}

// NewPermanentError creates a new permanent error.
func NewPermanentError(message string, underlying error) *PermanentError {
	return &PermanentError{Message: message, Underlying: underlying}
}

// NewTimeoutError creates a new timeout error.
func NewTimeoutError(message string) *TimeoutError {
	return &TimeoutError{Message: message}
}
