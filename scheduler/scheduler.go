// Package scheduler defines the scheduling interface and provides implementations for GopherQueue.
package scheduler

import (
	"context"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/sa001gar/gopherqueue/core"
	"github.com/sa001gar/gopherqueue/persistence"
)

// Scheduler coordinates job scheduling and dispatching.
// It maintains the priority queue and manages job lifecycle transitions.
type Scheduler interface {
	// Start begins the scheduler's main loop.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the scheduler.
	Stop(ctx context.Context) error

	// Enqueue adds a new job to the scheduler.
	Enqueue(ctx context.Context, job *core.Job) error

	// Schedule moves a delayed job to scheduled state.
	Schedule(ctx context.Context, id uuid.UUID) error

	// Complete marks a job as successfully completed.
	Complete(ctx context.Context, id uuid.UUID, result *core.JobResult) error

	// Fail marks a job as failed and determines retry behavior.
	Fail(ctx context.Context, id uuid.UUID, result *core.JobResult) error

	// Cancel requests cancellation of a job.
	Cancel(ctx context.Context, id uuid.UUID, reason string, force bool) error

	// Retry reschedules a failed or dead-lettered job.
	Retry(ctx context.Context, id uuid.UUID, resetAttempts bool) error

	// GetNextJob returns the next job to execute, blocking until available.
	GetNextJob(ctx context.Context) (*core.Job, error)

	// Stats returns current scheduler statistics.
	Stats() *SchedulerStats

	// IsHealthy returns whether the scheduler is operating normally.
	IsHealthy() bool
}

// SchedulerStats contains scheduler metrics.
type SchedulerStats struct {
	PendingCount   int64 `json:"pending_count"`
	ScheduledCount int64 `json:"scheduled_count"`
	RunningCount   int64 `json:"running_count"`
	DelayedCount   int64 `json:"delayed_count"`

	EnqueueRate  float64 `json:"enqueue_rate"`
	CompleteRate float64 `json:"complete_rate"`
	FailRate     float64 `json:"fail_rate"`

	ScheduleLatencyP50 time.Duration `json:"schedule_latency_p50"`
	ScheduleLatencyP99 time.Duration `json:"schedule_latency_p99"`

	LastCycleTime time.Time     `json:"last_cycle_time"`
	CycleDuration time.Duration `json:"cycle_duration"`
	Healthy       bool          `json:"healthy"`
}

// SchedulerConfig configures scheduler behavior.
type SchedulerConfig struct {
	TickInterval      time.Duration
	BatchSize         int
	VisibilityTimeout time.Duration
	MaxPendingJobs    int64
}

// DefaultSchedulerConfig returns sensible defaults.
func DefaultSchedulerConfig() *SchedulerConfig {
	return &SchedulerConfig{
		TickInterval:      100 * time.Millisecond,
		BatchSize:         100,
		VisibilityTimeout: 5 * time.Minute,
		MaxPendingJobs:    1000000,
	}
}

// PriorityScheduler is a full-featured scheduler implementation with priority queue.
type PriorityScheduler struct {
	store  persistence.JobStore
	config *SchedulerConfig

	// Job channel for workers
	jobCh chan *core.Job

	// State
	running int32
	stopCh  chan struct{}

	// Stats
	stats     *SchedulerStats
	statsMu   sync.RWMutex
	enqueued  int64
	completed int64
	failed    int64

	mu sync.RWMutex
}

// NewPriorityScheduler creates a new priority-based scheduler.
func NewPriorityScheduler(store persistence.JobStore, config *SchedulerConfig) *PriorityScheduler {
	if config == nil {
		config = DefaultSchedulerConfig()
	}
	return &PriorityScheduler{
		store:  store,
		config: config,
		jobCh:  make(chan *core.Job, config.BatchSize),
		stats:  &SchedulerStats{Healthy: true},
		stopCh: make(chan struct{}),
	}
}

// Start begins the scheduler's main loop.
func (s *PriorityScheduler) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		return nil
	}

	go s.runScheduleLoop(ctx)
	go s.runDelayedLoop(ctx)

	return nil
}

// Stop gracefully shuts down the scheduler.
func (s *PriorityScheduler) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&s.running, 1, 0) {
		return nil
	}

	close(s.stopCh)
	return nil
}

// runScheduleLoop processes pending jobs and dispatches them.
func (s *PriorityScheduler) runScheduleLoop(ctx context.Context) {
	ticker := time.NewTicker(s.config.TickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.scheduleJobs(ctx)
		}
	}
}

// runDelayedLoop processes delayed jobs that are now ready.
func (s *PriorityScheduler) runDelayedLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.processDelayedJobs(ctx)
		}
	}
}

// scheduleJobs fetches pending jobs and sends them to workers.
func (s *PriorityScheduler) scheduleJobs(ctx context.Context) {
	start := time.Now()

	// Get pending jobs
	jobs, err := s.store.GetPending(ctx, s.config.BatchSize)
	if err != nil {
		return
	}

	// Sort by priority, then by created time
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].Priority != jobs[j].Priority {
			return jobs[i].Priority < jobs[j].Priority
		}
		return jobs[i].CreatedAt.Before(jobs[j].CreatedAt)
	})

	// Send to workers
	for _, job := range jobs {
		// Update state to scheduled
		if job.State == core.JobStatePending {
			s.store.UpdateState(ctx, job.ID, core.JobStateScheduled)
			job.State = core.JobStateScheduled
		}

		select {
		case s.jobCh <- job:
			// Job sent successfully
		default:
			// Channel full, stop for now
			break
		}
	}

	// Update stats
	s.statsMu.Lock()
	s.stats.LastCycleTime = start
	s.stats.CycleDuration = time.Since(start)
	s.statsMu.Unlock()
}

// processDelayedJobs moves delayed jobs that are ready to scheduled state.
func (s *PriorityScheduler) processDelayedJobs(ctx context.Context) {
	jobs, err := s.store.GetDelayed(ctx, time.Now(), s.config.BatchSize)
	if err != nil {
		return
	}

	for _, job := range jobs {
		s.store.UpdateState(ctx, job.ID, core.JobStateScheduled)
	}
}

// Enqueue adds a new job to the scheduler.
func (s *PriorityScheduler) Enqueue(ctx context.Context, job *core.Job) error {
	atomic.AddInt64(&s.enqueued, 1)

	// If job has delay, set as delayed
	if job.Delay > 0 {
		job.State = core.JobStateDelayed
		job.ScheduledAt = time.Now().Add(job.Delay)
	} else if !job.ScheduledAt.IsZero() && job.ScheduledAt.After(time.Now()) {
		job.State = core.JobStateDelayed
	} else {
		job.State = core.JobStatePending
	}

	return nil
}

// Schedule moves a delayed job to scheduled state.
func (s *PriorityScheduler) Schedule(ctx context.Context, id uuid.UUID) error {
	return s.store.UpdateState(ctx, id, core.JobStateScheduled)
}

// GetNextJob returns the next job to execute.
func (s *PriorityScheduler) GetNextJob(ctx context.Context) (*core.Job, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case job := <-s.jobCh:
		return job, nil
	}
}

// Complete marks a job as successfully completed.
func (s *PriorityScheduler) Complete(ctx context.Context, id uuid.UUID, result *core.JobResult) error {
	atomic.AddInt64(&s.completed, 1)

	job, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}

	job.State = core.JobStateCompleted
	job.CompletedAt = time.Now().UTC()
	job.UpdatedAt = job.CompletedAt

	if err := s.store.Update(ctx, job); err != nil {
		return err
	}

	return s.store.SaveResult(ctx, result)
}

// Fail marks a job as failed and determines retry behavior.
func (s *PriorityScheduler) Fail(ctx context.Context, id uuid.UUID, result *core.JobResult) error {
	atomic.AddInt64(&s.failed, 1)

	job, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}

	// Save the result
	if err := s.store.SaveResult(ctx, result); err != nil {
		return err
	}

	// Classify the error
	category := result.ErrorCategory
	if category == "" {
		category = core.ErrorCategorySystem
	}

	// Check if we should retry
	shouldRetry := category == core.ErrorCategoryTemporary && job.Attempt < job.MaxAttempts

	if shouldRetry {
		// Calculate backoff delay
		delay := CalculateBackoff(job.Attempt, job.BackoffStrategy, job.BackoffInitial, job.BackoffMax, job.BackoffMultiplier)

		job.State = core.JobStateRetrying
		job.ScheduledAt = time.Now().Add(delay)
		job.LastError = result.Error
		job.ErrorCategory = string(category)
	} else if job.Attempt >= job.MaxAttempts {
		// Dead letter
		job.State = core.JobStateDeadLetter
		job.CompletedAt = time.Now().UTC()
		job.LastError = result.Error
		job.ErrorCategory = string(category)
	} else {
		// Permanent failure
		job.State = core.JobStateFailed
		job.CompletedAt = time.Now().UTC()
		job.LastError = result.Error
		job.ErrorCategory = string(category)
	}

	job.UpdatedAt = time.Now().UTC()
	return s.store.Update(ctx, job)
}

// CalculateBackoff calculates the retry delay based on the backoff strategy.
func CalculateBackoff(attempt int, strategy core.BackoffStrategy, initial, max time.Duration, multiplier float64) time.Duration {
	var delay time.Duration

	switch strategy {
	case core.BackoffConstant:
		delay = initial
	case core.BackoffLinear:
		delay = time.Duration(attempt) * initial
	case core.BackoffExponential:
		delay = time.Duration(float64(initial) * math.Pow(multiplier, float64(attempt-1)))
	default:
		delay = time.Duration(float64(initial) * math.Pow(2, float64(attempt-1)))
	}

	if delay > max {
		delay = max
	}

	return delay
}

// Cancel requests cancellation of a job.
func (s *PriorityScheduler) Cancel(ctx context.Context, id uuid.UUID, reason string, force bool) error {
	job, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}

	switch job.State {
	case core.JobStateCompleted, core.JobStateFailed, core.JobStateDeadLetter, core.JobStateCancelled:
		return core.ErrJobNotCancellable
	case core.JobStateRunning:
		if !force {
			// TODO: Mark for cancellation on next checkpoint
			return nil
		}
	}

	job.State = core.JobStateCancelled
	job.CompletedAt = time.Now().UTC()
	job.UpdatedAt = job.CompletedAt
	job.LastError = reason

	return s.store.Update(ctx, job)
}

// Retry reschedules a failed or dead-lettered job.
func (s *PriorityScheduler) Retry(ctx context.Context, id uuid.UUID, resetAttempts bool) error {
	job, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}

	if job.State != core.JobStateFailed && job.State != core.JobStateDeadLetter {
		return core.ErrJobNotCancellable
	}

	if resetAttempts {
		job.Attempt = 0
	}

	job.State = core.JobStatePending
	job.UpdatedAt = time.Now().UTC()
	job.CompletedAt = time.Time{}
	job.LastError = ""
	job.ErrorCategory = ""

	return s.store.Update(ctx, job)
}

// Stats returns current scheduler statistics.
func (s *PriorityScheduler) Stats() *SchedulerStats {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()

	stats := *s.stats
	stats.Healthy = atomic.LoadInt32(&s.running) == 1
	return &stats
}

// IsHealthy returns whether the scheduler is operating normally.
func (s *PriorityScheduler) IsHealthy() bool {
	return atomic.LoadInt32(&s.running) == 1
}

// Ensure PriorityScheduler implements Scheduler.
var _ Scheduler = (*PriorityScheduler)(nil)
