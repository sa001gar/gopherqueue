// Package core provides the fundamental types for GopherQueue.
package core

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Job represents a unit of work to be processed.
type Job struct {
	// Identity
	ID             uuid.UUID         `json:"id"`
	IdempotencyKey string            `json:"idempotency_key,omitempty"`
	CorrelationID  string            `json:"correlation_id,omitempty"`
	Type           string            `json:"type"`
	Payload        []byte            `json:"payload"`
	Tags           map[string]string `json:"tags,omitempty"`

	// Priority and Scheduling
	Priority    Priority      `json:"priority"`
	Delay       time.Duration `json:"delay,omitempty"`
	ScheduledAt time.Time     `json:"scheduled_at,omitempty"`

	// Execution Control
	Timeout           time.Duration   `json:"timeout,omitempty"`
	MaxAttempts       int             `json:"max_attempts"`
	Attempt           int             `json:"attempt"`
	BackoffStrategy   BackoffStrategy `json:"backoff_strategy"`
	BackoffInitial    time.Duration   `json:"backoff_initial"`
	BackoffMax        time.Duration   `json:"backoff_max"`
	BackoffMultiplier float64         `json:"backoff_multiplier"`

	// Dependencies
	DependsOn []uuid.UUID `json:"depends_on,omitempty"`
	BlockedBy []uuid.UUID `json:"blocked_by,omitempty"`

	// State
	State           JobState `json:"state"`
	WorkerID        string   `json:"worker_id,omitempty"`
	Progress        float64  `json:"progress"`
	ProgressMessage string   `json:"progress_message,omitempty"`

	// Error information
	LastError     string `json:"last_error,omitempty"`
	ErrorCategory string `json:"error_category,omitempty"`

	// Timestamps
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	CompletedAt time.Time `json:"completed_at,omitempty"`

	// Metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// JobState represents the current state of a job.
type JobState string

const (
	JobStatePending    JobState = "pending"
	JobStateScheduled  JobState = "scheduled"
	JobStateRunning    JobState = "running"
	JobStateRetrying   JobState = "retrying"
	JobStateDelayed    JobState = "delayed"
	JobStateCompleted  JobState = "completed"
	JobStateFailed     JobState = "failed"
	JobStateDeadLetter JobState = "dead_letter"
	JobStateCancelled  JobState = "cancelled"
)

// Priority represents job priority levels.
type Priority int

const (
	PriorityCritical Priority = 0
	PriorityHigh     Priority = 1
	PriorityNormal   Priority = 2
	PriorityLow      Priority = 3
	PriorityBulk     Priority = 4
)

// BackoffStrategy defines retry backoff algorithms.
type BackoffStrategy string

const (
	BackoffConstant    BackoffStrategy = "constant"
	BackoffLinear      BackoffStrategy = "linear"
	BackoffExponential BackoffStrategy = "exponential"
)

// JobResult represents the outcome of a job execution.
type JobResult struct {
	JobID         uuid.UUID     `json:"job_id"`
	Success       bool          `json:"success"`
	Output        []byte        `json:"output,omitempty"`
	Error         string        `json:"error,omitempty"`
	ErrorCategory ErrorCategory `json:"error_category,omitempty"`
	Duration      time.Duration `json:"duration"`
	CompletedAt   time.Time     `json:"completed_at"`
	RetryAfter    time.Duration `json:"retry_after,omitempty"`
}

// JobContext provides the execution context for a job handler.
type JobContext interface {
	// Job returns the current job being executed.
	Job() *Job

	// Context returns the Go context for cancellation.
	Context() context.Context

	// Checkpoint saves progress for crash recovery.
	Checkpoint(data []byte) error

	// LastCheckpoint retrieves the last checkpoint data.
	LastCheckpoint() ([]byte, error)

	// SetOutput sets the job output.
	SetOutput(data []byte)

	// Progress reports execution progress.
	Progress(percent float64, message string)

	// Logger returns the job's logger.
	Logger() interface{}
}

// NewJob creates a new job with the given type and payload.
func NewJob(jobType string, payload []byte, opts ...JobOption) *Job {
	job := &Job{
		ID:                uuid.New(),
		Type:              jobType,
		Payload:           payload,
		Priority:          PriorityNormal,
		MaxAttempts:       3,
		BackoffStrategy:   BackoffExponential,
		BackoffInitial:    1 * time.Second,
		BackoffMax:        1 * time.Hour,
		BackoffMultiplier: 2.0,
		State:             JobStatePending,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
		Tags:              make(map[string]string),
	}

	for _, opt := range opts {
		opt(job)
	}

	return job
}

// IsTerminal returns whether the job is in a terminal state.
func (j *Job) IsTerminal() bool {
	return j.State == JobStateCompleted ||
		j.State == JobStateFailed ||
		j.State == JobStateDeadLetter ||
		j.State == JobStateCancelled
}

// CanRetry returns whether the job can be retried.
func (j *Job) CanRetry() bool {
	return j.Attempt < j.MaxAttempts && !j.IsTerminal()
}
