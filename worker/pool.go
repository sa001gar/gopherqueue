// Package worker provides job execution capabilities for GopherQueue.
package worker

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/sa001gar/gopherqueue/core"
)

// Handler is a function that processes a job.
type Handler func(ctx context.Context, jctx core.JobContext) error

// WorkerPool manages a set of workers that execute jobs.
type WorkerPool interface {
	// Start initializes the worker pool and begins processing.
	Start(ctx context.Context) error

	// Stop gracefully shuts down all workers.
	Stop(ctx context.Context) error

	// RegisterHandler registers a handler for a job type.
	RegisterHandler(jobType string, handler Handler)

	// Submit adds a job to the pool for execution.
	Submit(job *core.Job) error

	// Stats returns current worker pool statistics.
	Stats() *PoolStats

	// IsHealthy returns whether the worker pool is operating normally.
	IsHealthy() bool

	// WaitForJob blocks until a job is available or context is cancelled.
	WaitForJob(ctx context.Context) (*core.Job, error)

	// CompleteJob is called when a job finishes execution.
	CompleteJob(ctx context.Context, result *core.JobResult) error

	// FailJob is called when a job fails with an error.
	FailJob(ctx context.Context, result *core.JobResult) error
}

// PoolStats contains worker pool metrics.
type PoolStats struct {
	// Worker counts
	TotalWorkers  int `json:"total_workers"`
	ActiveWorkers int `json:"active_workers"`
	IdleWorkers   int `json:"idle_workers"`

	// Job counts
	QueuedJobs    int64 `json:"queued_jobs"`
	ProcessedJobs int64 `json:"processed_jobs"`
	SucceededJobs int64 `json:"succeeded_jobs"`
	FailedJobs    int64 `json:"failed_jobs"`
	RetriedJobs   int64 `json:"retried_jobs"`

	// Latencies
	AvgProcessingTime time.Duration `json:"avg_processing_time"`
	MaxProcessingTime time.Duration `json:"max_processing_time"`
	P99ProcessingTime time.Duration `json:"p99_processing_time"`

	// Current state
	ProcessingIDs []uuid.UUID   `json:"processing_ids"`
	Uptime        time.Duration `json:"uptime"`
	Healthy       bool          `json:"healthy"`
}

// WorkerConfig configures worker behavior.
type WorkerConfig struct {
	// ID is the unique identifier for this worker instance.
	ID string

	// Concurrency is the number of concurrent workers.
	Concurrency int

	// HeartbeatInterval is how often workers report heartbeat.
	HeartbeatInterval time.Duration

	// JobTimeout is the maximum time a job can run before being killed.
	JobTimeout time.Duration

	// ShutdownGracePeriod is the time to wait for jobs to complete on shutdown.
	ShutdownGracePeriod time.Duration

	// CheckpointInterval is how often to checkpoint long-running jobs.
	CheckpointInterval time.Duration

	// DataDir is the directory for local storage
	DataDir string
}

// DefaultWorkerConfig returns sensible defaults.
func DefaultWorkerConfig() *WorkerConfig {
	return &WorkerConfig{
		ID:                  uuid.New().String(),
		Concurrency:         10,
		HeartbeatInterval:   30 * time.Second,
		JobTimeout:          30 * time.Minute,
		ShutdownGracePeriod: 30 * time.Second,
		CheckpointInterval:  1 * time.Minute,
		DataDir:             "./data",
	}
}

// JobContext is an alias to core.JobContext for external use.
type JobContext = core.JobContext
