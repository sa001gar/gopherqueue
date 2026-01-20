// Package persistence defines the storage interface for GopherQueue.
package persistence

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/sa001gar/gopherqueue/core"
)

// JobStore defines the persistence interface for job storage.
// Implementations must provide durable storage with ACID guarantees.
type JobStore interface {
	// Create persists a new job. Returns ErrJobAlreadyExists if
	// a job with the same idempotency key exists.
	Create(ctx context.Context, job *core.Job) error

	// Get retrieves a job by ID. Returns ErrJobNotFound if not found.
	Get(ctx context.Context, id uuid.UUID) (*core.Job, error)

	// GetByIdempotencyKey retrieves a job by its idempotency key.
	// Returns ErrJobNotFound if not found.
	GetByIdempotencyKey(ctx context.Context, key string) (*core.Job, error)

	// Update atomically updates a job. The job must already exist.
	Update(ctx context.Context, job *core.Job) error

	// UpdateState atomically updates only the job state and updated_at.
	UpdateState(ctx context.Context, id uuid.UUID, state core.JobState) error

	// Delete removes a job. This is typically used for cleanup.
	Delete(ctx context.Context, id uuid.UUID) error

	// List returns jobs matching the given filter.
	List(ctx context.Context, filter JobFilter) ([]*core.Job, error)

	// Count returns the number of jobs matching the filter.
	Count(ctx context.Context, filter JobFilter) (int64, error)

	// GetPending returns jobs ready for scheduling, ordered by
	// priority (ascending) then created_at (ascending).
	GetPending(ctx context.Context, limit int) ([]*core.Job, error)

	// GetDelayed returns jobs whose scheduled time has passed.
	GetDelayed(ctx context.Context, now time.Time, limit int) ([]*core.Job, error)

	// GetStuck returns jobs that have been running longer than
	// the visibility timeout and should be reclaimed.
	GetStuck(ctx context.Context, visibilityTimeout time.Duration) ([]*core.Job, error)

	// ClaimJob atomically transitions a job from scheduled to running
	// and assigns it to the specified worker. Returns false if the
	// job is no longer claimable.
	ClaimJob(ctx context.Context, id uuid.UUID, workerID string) (bool, error)

	// SaveResult stores the result of a job execution.
	SaveResult(ctx context.Context, result *core.JobResult) error

	// GetResult retrieves the result for a completed job.
	GetResult(ctx context.Context, jobID uuid.UUID) (*core.JobResult, error)

	// SaveCheckpoint stores a checkpoint for a running job.
	SaveCheckpoint(ctx context.Context, jobID uuid.UUID, data []byte) error

	// GetCheckpoint retrieves the last checkpoint for a job.
	GetCheckpoint(ctx context.Context, jobID uuid.UUID) ([]byte, error)

	// Stats returns queue statistics.
	Stats(ctx context.Context) (*QueueStats, error)

	// Cleanup removes completed/failed jobs older than the retention period.
	Cleanup(ctx context.Context, retention time.Duration) (int64, error)

	// Close releases any resources held by the store.
	Close() error
}

// JobFilter specifies criteria for listing jobs.
type JobFilter struct {
	// Filter by job types
	Types []string

	// Filter by states
	States []core.JobState

	// Filter by priorities
	Priorities []core.Priority

	// Filter by tags (all must match)
	Tags map[string]string

	// Filter by time range
	CreatedAfter  time.Time
	CreatedBefore time.Time
	UpdatedAfter  time.Time

	// Filter by correlation
	CorrelationID string

	// Pagination
	Cursor string
	Limit  int
	Order  ListOrder
}

// ListOrder specifies the ordering for job lists.
type ListOrder string

const (
	OrderCreatedAsc  ListOrder = "created_asc"
	OrderCreatedDesc ListOrder = "created_desc"
	OrderUpdatedDesc ListOrder = "updated_desc"
	OrderPriorityAsc ListOrder = "priority_asc"
)

// QueueStats contains aggregate queue statistics.
type QueueStats struct {
	// Current state counts
	Pending    int64 `json:"pending"`
	Scheduled  int64 `json:"scheduled"`
	Running    int64 `json:"running"`
	Delayed    int64 `json:"delayed"`
	Completed  int64 `json:"completed"`
	Failed     int64 `json:"failed"`
	DeadLetter int64 `json:"dead_letter"`
	Cancelled  int64 `json:"cancelled"`

	// Total jobs
	TotalJobs int64 `json:"total_jobs"`

	// Storage info
	StorageBytes int64 `json:"storage_bytes"`

	// Timestamp
	Timestamp time.Time `json:"timestamp"`
}

// matchesFilter checks if a job matches the filter criteria.
func matchesFilter(job *core.Job, filter JobFilter) bool {
	// Check types
	if len(filter.Types) > 0 {
		found := false
		for _, t := range filter.Types {
			if job.Type == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check states
	if len(filter.States) > 0 {
		found := false
		for _, s := range filter.States {
			if job.State == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check priorities
	if len(filter.Priorities) > 0 {
		found := false
		for _, p := range filter.Priorities {
			if job.Priority == p {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check tags
	for k, v := range filter.Tags {
		if job.Tags[k] != v {
			return false
		}
	}

	// Check time filters
	if !filter.CreatedAfter.IsZero() && job.CreatedAt.Before(filter.CreatedAfter) {
		return false
	}
	if !filter.CreatedBefore.IsZero() && job.CreatedAt.After(filter.CreatedBefore) {
		return false
	}
	if !filter.UpdatedAfter.IsZero() && job.UpdatedAt.Before(filter.UpdatedAfter) {
		return false
	}

	// Check correlation
	if filter.CorrelationID != "" && job.CorrelationID != filter.CorrelationID {
		return false
	}

	return true
}
