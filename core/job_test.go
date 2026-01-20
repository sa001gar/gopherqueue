// Package core provides unit tests for the core package.
package core

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewJob(t *testing.T) {
	payload := []byte(`{"key": "value"}`)
	job := NewJob("email", payload)

	if job.ID == uuid.Nil {
		t.Error("job ID should not be nil")
	}
	if job.Type != "email" {
		t.Errorf("expected type 'email', got '%s'", job.Type)
	}
	if string(job.Payload) != string(payload) {
		t.Error("payload mismatch")
	}
	if job.State != JobStatePending {
		t.Errorf("expected state pending, got %s", job.State)
	}
	if job.Priority != PriorityNormal {
		t.Errorf("expected normal priority, got %d", job.Priority)
	}
	if job.MaxAttempts != 3 {
		t.Errorf("expected 3 max attempts, got %d", job.MaxAttempts)
	}
}

func TestJobOptions(t *testing.T) {
	customID := uuid.New()
	now := time.Now().Add(1 * time.Hour)

	job := NewJob("test", nil,
		WithID(customID),
		WithPriority(PriorityCritical),
		WithIdempotencyKey("unique-123"),
		WithCorrelationID("trace-456"),
		WithMaxAttempts(5),
		WithTimeout(10*time.Minute),
		WithScheduledAt(now),
		WithTag("env", "prod"),
	)

	if job.ID != customID {
		t.Error("custom ID not set")
	}
	if job.Priority != PriorityCritical {
		t.Error("priority not set")
	}
	if job.IdempotencyKey != "unique-123" {
		t.Error("idempotency key not set")
	}
	if job.CorrelationID != "trace-456" {
		t.Error("correlation ID not set")
	}
	if job.MaxAttempts != 5 {
		t.Error("max attempts not set")
	}
	if job.Timeout != 10*time.Minute {
		t.Error("timeout not set")
	}
	if job.ScheduledAt != now {
		t.Error("scheduled at not set")
	}
	if job.Tags["env"] != "prod" {
		t.Error("tag not set")
	}
}

func TestJobWithDelay(t *testing.T) {
	before := time.Now()
	job := NewJob("test", nil, WithDelay(5*time.Minute))
	after := time.Now()

	expectedMin := before.Add(5 * time.Minute)
	expectedMax := after.Add(5 * time.Minute)

	if job.ScheduledAt.Before(expectedMin) || job.ScheduledAt.After(expectedMax) {
		t.Errorf("scheduled at should be ~5 minutes from now, got %v", job.ScheduledAt)
	}
	if job.Delay != 5*time.Minute {
		t.Errorf("delay should be 5m, got %v", job.Delay)
	}
}

func TestJobIsTerminal(t *testing.T) {
	tests := []struct {
		state    JobState
		terminal bool
	}{
		{JobStatePending, false},
		{JobStateScheduled, false},
		{JobStateRunning, false},
		{JobStateRetrying, false},
		{JobStateDelayed, false},
		{JobStateCompleted, true},
		{JobStateFailed, true},
		{JobStateDeadLetter, true},
		{JobStateCancelled, true},
	}

	for _, tt := range tests {
		job := &Job{State: tt.state}
		if job.IsTerminal() != tt.terminal {
			t.Errorf("state %s: expected terminal=%v, got %v", tt.state, tt.terminal, job.IsTerminal())
		}
	}
}

func TestJobCanRetry(t *testing.T) {
	// Can retry: not terminal and attempts < max
	job := &Job{
		State:       JobStateRunning,
		Attempt:     1,
		MaxAttempts: 3,
	}
	if !job.CanRetry() {
		t.Error("should be able to retry")
	}

	// Cannot retry: terminal state
	job.State = JobStateCompleted
	if job.CanRetry() {
		t.Error("should not retry completed job")
	}

	// Cannot retry: max attempts reached
	job.State = JobStateRunning
	job.Attempt = 3
	if job.CanRetry() {
		t.Error("should not retry after max attempts")
	}
}

func TestErrorClassification(t *testing.T) {
	tests := []struct {
		err      error
		expected ErrorCategory
	}{
		{nil, ""},
		{context.DeadlineExceeded, ErrorCategoryTimeout},
		{context.Canceled, ErrorCategorySystem},
		{&TemporaryError{Message: "temp"}, ErrorCategoryTemporary},
		{&PermanentError{Message: "perm"}, ErrorCategoryPermanent},
		{&TimeoutError{Message: "timeout"}, ErrorCategoryTimeout},
		{&RateLimitError{Message: "rate"}, ErrorCategoryRateLimit},
	}

	for _, tt := range tests {
		category := ClassifyError(tt.err)
		if category != tt.expected {
			t.Errorf("for %v: expected %s, got %s", tt.err, tt.expected, category)
		}
	}
}

func TestIsRetriable(t *testing.T) {
	tests := []struct {
		err       error
		retriable bool
	}{
		{&TemporaryError{Message: "temp"}, true},
		{&RateLimitError{Message: "rate"}, true},
		{&PermanentError{Message: "perm"}, false},
		{&TimeoutError{Message: "timeout"}, false},
	}

	for _, tt := range tests {
		if IsRetriable(tt.err) != tt.retriable {
			t.Errorf("for %v: expected retriable=%v", tt.err, tt.retriable)
		}
	}
}

func TestErrorTypes(t *testing.T) {
	// Test TemporaryError
	tempErr := NewTemporaryError("failed to connect", nil)
	if tempErr.Error() != "failed to connect" {
		t.Errorf("unexpected message: %s", tempErr.Error())
	}
	if !tempErr.Temporary() {
		t.Error("should be temporary")
	}

	// Test with underlying error
	underlying := &PermanentError{Message: "bad data"}
	tempErr2 := NewTemporaryError("wrapper", underlying)
	if tempErr2.Unwrap() != underlying {
		t.Error("unwrap should return underlying error")
	}

	// Test PermanentError
	permErr := NewPermanentError("invalid input", nil)
	if !permErr.Permanent() {
		t.Error("should be permanent")
	}

	// Test TimeoutError
	timeoutErr := NewTimeoutError("operation timed out")
	if !timeoutErr.Timeout() {
		t.Error("should be timeout")
	}

	// Test RateLimitError
	rlErr := &RateLimitError{Message: "slow down", RetryIn: 60}
	if !rlErr.RateLimited() {
		t.Error("should be rate limited")
	}

	// Test ValidationError
	valErr := &ValidationError{Field: "email", Message: "invalid format"}
	expected := "validation error on email: invalid format"
	if valErr.Error() != expected {
		t.Errorf("expected '%s', got '%s'", expected, valErr.Error())
	}
}

func TestDefaultServerConfig(t *testing.T) {
	config := DefaultServerConfig()

	if config.HTTPAddr != ":8080" {
		t.Errorf("expected :8080, got %s", config.HTTPAddr)
	}
	if config.Workers != 10 {
		t.Errorf("expected 10 workers, got %d", config.Workers)
	}
	if config.DataDir != "./data" {
		t.Errorf("expected ./data, got %s", config.DataDir)
	}
}
