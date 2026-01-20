// Package persistence provides an in-memory implementation of JobStore.
package persistence

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sa001gar/gopherqueue/core"
)

// MemoryStore is an in-memory implementation of JobStore.
// This is intended for testing and development; use BoltStore for production.
type MemoryStore struct {
	jobs        map[uuid.UUID]*core.Job
	results     map[uuid.UUID]*core.JobResult
	checkpoints map[uuid.UUID][]byte
	idempotency map[string]uuid.UUID
	mu          sync.RWMutex
}

// NewMemoryStore creates a new in-memory job store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		jobs:        make(map[uuid.UUID]*core.Job),
		results:     make(map[uuid.UUID]*core.JobResult),
		checkpoints: make(map[uuid.UUID][]byte),
		idempotency: make(map[string]uuid.UUID),
	}
}

// Create persists a new job.
func (m *MemoryStore) Create(ctx context.Context, job *core.Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check idempotency key
	if job.IdempotencyKey != "" {
		if _, exists := m.idempotency[job.IdempotencyKey]; exists {
			return core.ErrJobAlreadyExists
		}
		m.idempotency[job.IdempotencyKey] = job.ID
	}

	// Check for duplicate ID
	if _, exists := m.jobs[job.ID]; exists {
		return core.ErrJobAlreadyExists
	}

	// Deep copy the job
	jobCopy := *job
	m.jobs[job.ID] = &jobCopy
	return nil
}

// Get retrieves a job by ID.
func (m *MemoryStore) Get(ctx context.Context, id uuid.UUID) (*core.Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, exists := m.jobs[id]
	if !exists {
		return nil, core.ErrJobNotFound
	}

	// Return a copy
	jobCopy := *job
	return &jobCopy, nil
}

// GetByIdempotencyKey retrieves a job by its idempotency key.
func (m *MemoryStore) GetByIdempotencyKey(ctx context.Context, key string) (*core.Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id, exists := m.idempotency[key]
	if !exists {
		return nil, core.ErrJobNotFound
	}

	job, exists := m.jobs[id]
	if !exists {
		return nil, core.ErrJobNotFound
	}

	jobCopy := *job
	return &jobCopy, nil
}

// Update atomically updates a job.
func (m *MemoryStore) Update(ctx context.Context, job *core.Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.jobs[job.ID]; !exists {
		return core.ErrJobNotFound
	}

	job.UpdatedAt = time.Now().UTC()
	jobCopy := *job
	m.jobs[job.ID] = &jobCopy
	return nil
}

// UpdateState atomically updates only the job state.
func (m *MemoryStore) UpdateState(ctx context.Context, id uuid.UUID, state core.JobState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, exists := m.jobs[id]
	if !exists {
		return core.ErrJobNotFound
	}

	job.State = state
	job.UpdatedAt = time.Now().UTC()
	return nil
}

// Delete removes a job.
func (m *MemoryStore) Delete(ctx context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, exists := m.jobs[id]
	if !exists {
		return core.ErrJobNotFound
	}

	// Remove idempotency key mapping
	if job.IdempotencyKey != "" {
		delete(m.idempotency, job.IdempotencyKey)
	}

	delete(m.jobs, id)
	delete(m.results, id)
	delete(m.checkpoints, id)
	return nil
}

// List returns jobs matching the given filter.
func (m *MemoryStore) List(ctx context.Context, filter JobFilter) ([]*core.Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*core.Job
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}

	for _, job := range m.jobs {
		if matchesFilter(job, filter) {
			jobCopy := *job
			result = append(result, &jobCopy)
		}
		if len(result) >= limit {
			break
		}
	}

	return result, nil
}

// Count returns the number of jobs matching the filter.
func (m *MemoryStore) Count(ctx context.Context, filter JobFilter) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var count int64
	for _, job := range m.jobs {
		if matchesFilter(job, filter) {
			count++
		}
	}

	return count, nil
}

// GetPending returns jobs ready for scheduling.
func (m *MemoryStore) GetPending(ctx context.Context, limit int) ([]*core.Job, error) {
	return m.List(ctx, JobFilter{
		States: []core.JobState{core.JobStatePending, core.JobStateScheduled},
		Limit:  limit,
		Order:  OrderPriorityAsc,
	})
}

// GetDelayed returns jobs whose scheduled time has passed.
func (m *MemoryStore) GetDelayed(ctx context.Context, now time.Time, limit int) ([]*core.Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*core.Job
	for _, job := range m.jobs {
		if job.State == core.JobStateDelayed && !job.ScheduledAt.After(now) {
			jobCopy := *job
			result = append(result, &jobCopy)
		}
		if len(result) >= limit {
			break
		}
	}

	return result, nil
}

// GetStuck returns stuck jobs.
func (m *MemoryStore) GetStuck(ctx context.Context, visibilityTimeout time.Duration) ([]*core.Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cutoff := time.Now().Add(-visibilityTimeout)
	var result []*core.Job

	for _, job := range m.jobs {
		if job.State == core.JobStateRunning && job.StartedAt.Before(cutoff) {
			jobCopy := *job
			result = append(result, &jobCopy)
		}
	}

	return result, nil
}

// ClaimJob atomically claims a job for a worker.
func (m *MemoryStore) ClaimJob(ctx context.Context, id uuid.UUID, workerID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, exists := m.jobs[id]
	if !exists {
		return false, core.ErrJobNotFound
	}

	if job.State != core.JobStateScheduled && job.State != core.JobStatePending {
		return false, nil
	}

	job.State = core.JobStateRunning
	job.WorkerID = workerID
	job.StartedAt = time.Now().UTC()
	job.UpdatedAt = job.StartedAt
	job.Attempt++

	return true, nil
}

// SaveResult stores the result of a job execution.
func (m *MemoryStore) SaveResult(ctx context.Context, result *core.JobResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	resultCopy := *result
	m.results[result.JobID] = &resultCopy
	return nil
}

// GetResult retrieves the result for a completed job.
func (m *MemoryStore) GetResult(ctx context.Context, jobID uuid.UUID) (*core.JobResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result, exists := m.results[jobID]
	if !exists {
		return nil, core.ErrJobNotFound
	}

	resultCopy := *result
	return &resultCopy, nil
}

// SaveCheckpoint stores a checkpoint for a running job.
func (m *MemoryStore) SaveCheckpoint(ctx context.Context, jobID uuid.UUID, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	m.checkpoints[jobID] = dataCopy
	return nil
}

// GetCheckpoint retrieves the last checkpoint for a job.
func (m *MemoryStore) GetCheckpoint(ctx context.Context, jobID uuid.UUID) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, exists := m.checkpoints[jobID]
	if !exists {
		return nil, nil
	}

	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	return dataCopy, nil
}

// Stats returns queue statistics.
func (m *MemoryStore) Stats(ctx context.Context) (*QueueStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &QueueStats{
		Timestamp: time.Now().UTC(),
		TotalJobs: int64(len(m.jobs)),
	}

	for _, job := range m.jobs {
		switch job.State {
		case core.JobStatePending:
			stats.Pending++
		case core.JobStateScheduled:
			stats.Scheduled++
		case core.JobStateRunning:
			stats.Running++
		case core.JobStateDelayed:
			stats.Delayed++
		case core.JobStateCompleted:
			stats.Completed++
		case core.JobStateFailed:
			stats.Failed++
		case core.JobStateDeadLetter:
			stats.DeadLetter++
		case core.JobStateCancelled:
			stats.Cancelled++
		}
	}

	return stats, nil
}

// Cleanup removes old completed/failed jobs.
func (m *MemoryStore) Cleanup(ctx context.Context, retention time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-retention)
	var removed int64

	for id, job := range m.jobs {
		if (job.State == core.JobStateCompleted || job.State == core.JobStateFailed) &&
			job.CompletedAt.Before(cutoff) {
			if job.IdempotencyKey != "" {
				delete(m.idempotency, job.IdempotencyKey)
			}
			delete(m.jobs, id)
			delete(m.results, id)
			delete(m.checkpoints, id)
			removed++
		}
	}

	return removed, nil
}

// Close releases resources.
func (m *MemoryStore) Close() error {
	return nil
}

// Ensure MemoryStore implements JobStore.
var _ JobStore = (*MemoryStore)(nil)
