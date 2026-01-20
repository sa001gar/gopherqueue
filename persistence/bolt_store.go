// Package persistence provides a BoltDB implementation of JobStore.
package persistence

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/sa001gar/gopherqueue/core"
	bolt "go.etcd.io/bbolt"
)

// Bucket names
var (
	jobsBucket        = []byte("jobs")
	resultsBucket     = []byte("results")
	checkpointBucket  = []byte("checkpoints")
	idempotencyBucket = []byte("idempotency")
	indexBucket       = []byte("indexes")
)

// BoltStore is a BoltDB-backed implementation of JobStore.
type BoltStore struct {
	db   *bolt.DB
	path string
}

// NewBoltStore creates a new BoltDB job store.
func NewBoltStore(dataDir string) (*BoltStore, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "gopherqueue.db")
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{
		Timeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create buckets
	err = db.Update(func(tx *bolt.Tx) error {
		for _, bucket := range [][]byte{jobsBucket, resultsBucket, checkpointBucket, idempotencyBucket, indexBucket} {
			if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create buckets: %w", err)
	}

	return &BoltStore{
		db:   db,
		path: dbPath,
	}, nil
}

// Create persists a new job.
func (s *BoltStore) Create(ctx context.Context, job *core.Job) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		jobs := tx.Bucket(jobsBucket)
		idem := tx.Bucket(idempotencyBucket)

		// Check idempotency key
		if job.IdempotencyKey != "" {
			if existing := idem.Get([]byte(job.IdempotencyKey)); existing != nil {
				return core.ErrJobAlreadyExists
			}
		}

		// Check if job exists
		key := []byte(job.ID.String())
		if jobs.Get(key) != nil {
			return core.ErrJobAlreadyExists
		}

		// Serialize job
		data, err := encodeJob(job)
		if err != nil {
			return err
		}

		// Store job
		if err := jobs.Put(key, data); err != nil {
			return err
		}

		// Store idempotency mapping
		if job.IdempotencyKey != "" {
			if err := idem.Put([]byte(job.IdempotencyKey), key); err != nil {
				return err
			}
		}

		return nil
	})
}

// Get retrieves a job by ID.
func (s *BoltStore) Get(ctx context.Context, id uuid.UUID) (*core.Job, error) {
	var job *core.Job
	err := s.db.View(func(tx *bolt.Tx) error {
		data := tx.Bucket(jobsBucket).Get([]byte(id.String()))
		if data == nil {
			return core.ErrJobNotFound
		}
		var err error
		job, err = decodeJob(data)
		return err
	})
	return job, err
}

// GetByIdempotencyKey retrieves a job by its idempotency key.
func (s *BoltStore) GetByIdempotencyKey(ctx context.Context, key string) (*core.Job, error) {
	var job *core.Job
	err := s.db.View(func(tx *bolt.Tx) error {
		idem := tx.Bucket(idempotencyBucket)
		jobID := idem.Get([]byte(key))
		if jobID == nil {
			return core.ErrJobNotFound
		}

		data := tx.Bucket(jobsBucket).Get(jobID)
		if data == nil {
			return core.ErrJobNotFound
		}
		var err error
		job, err = decodeJob(data)
		return err
	})
	return job, err
}

// Update atomically updates a job.
func (s *BoltStore) Update(ctx context.Context, job *core.Job) error {
	job.UpdatedAt = time.Now().UTC()
	return s.db.Update(func(tx *bolt.Tx) error {
		jobs := tx.Bucket(jobsBucket)
		key := []byte(job.ID.String())

		if jobs.Get(key) == nil {
			return core.ErrJobNotFound
		}

		data, err := encodeJob(job)
		if err != nil {
			return err
		}
		return jobs.Put(key, data)
	})
}

// UpdateState atomically updates only the job state.
func (s *BoltStore) UpdateState(ctx context.Context, id uuid.UUID, state core.JobState) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		jobs := tx.Bucket(jobsBucket)
		key := []byte(id.String())

		data := jobs.Get(key)
		if data == nil {
			return core.ErrJobNotFound
		}

		job, err := decodeJob(data)
		if err != nil {
			return err
		}

		job.State = state
		job.UpdatedAt = time.Now().UTC()

		newData, err := encodeJob(job)
		if err != nil {
			return err
		}
		return jobs.Put(key, newData)
	})
}

// Delete removes a job.
func (s *BoltStore) Delete(ctx context.Context, id uuid.UUID) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		jobs := tx.Bucket(jobsBucket)
		key := []byte(id.String())

		data := jobs.Get(key)
		if data == nil {
			return core.ErrJobNotFound
		}

		job, err := decodeJob(data)
		if err != nil {
			return err
		}

		// Remove idempotency key
		if job.IdempotencyKey != "" {
			tx.Bucket(idempotencyBucket).Delete([]byte(job.IdempotencyKey))
		}

		// Remove checkpoint
		tx.Bucket(checkpointBucket).Delete(key)

		// Remove result
		tx.Bucket(resultsBucket).Delete(key)

		// Remove job
		return jobs.Delete(key)
	})
}

// List returns jobs matching the given filter.
func (s *BoltStore) List(ctx context.Context, filter JobFilter) ([]*core.Job, error) {
	var result []*core.Job
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}

	err := s.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(jobsBucket).Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			job, err := decodeJob(v)
			if err != nil {
				continue
			}

			if matchesFilter(job, filter) {
				result = append(result, job)
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort based on order
	sortJobs(result, filter.Order)

	// Apply limit
	if len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

// Count returns the number of jobs matching the filter.
func (s *BoltStore) Count(ctx context.Context, filter JobFilter) (int64, error) {
	var count int64
	err := s.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(jobsBucket).Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			job, err := decodeJob(v)
			if err != nil {
				continue
			}

			if matchesFilter(job, filter) {
				count++
			}
		}
		return nil
	})
	return count, err
}

// GetPending returns jobs ready for scheduling.
func (s *BoltStore) GetPending(ctx context.Context, limit int) ([]*core.Job, error) {
	jobs, err := s.List(ctx, JobFilter{
		States: []core.JobState{core.JobStatePending, core.JobStateScheduled},
		Limit:  limit,
		Order:  OrderPriorityAsc,
	})
	if err != nil {
		return nil, err
	}

	// Sort by priority then created time
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].Priority != jobs[j].Priority {
			return jobs[i].Priority < jobs[j].Priority
		}
		return jobs[i].CreatedAt.Before(jobs[j].CreatedAt)
	})

	return jobs, nil
}

// GetDelayed returns jobs whose scheduled time has passed.
func (s *BoltStore) GetDelayed(ctx context.Context, now time.Time, limit int) ([]*core.Job, error) {
	var result []*core.Job

	err := s.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(jobsBucket).Cursor()

		for k, v := c.First(); k != nil && len(result) < limit; k, v = c.Next() {
			job, err := decodeJob(v)
			if err != nil {
				continue
			}

			if job.State == core.JobStateDelayed && !job.ScheduledAt.After(now) {
				result = append(result, job)
			}
		}
		return nil
	})

	return result, err
}

// GetStuck returns stuck jobs.
func (s *BoltStore) GetStuck(ctx context.Context, visibilityTimeout time.Duration) ([]*core.Job, error) {
	cutoff := time.Now().Add(-visibilityTimeout)
	var result []*core.Job

	err := s.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(jobsBucket).Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			job, err := decodeJob(v)
			if err != nil {
				continue
			}

			if job.State == core.JobStateRunning && !job.StartedAt.IsZero() && job.StartedAt.Before(cutoff) {
				result = append(result, job)
			}
		}
		return nil
	})

	return result, err
}

// ClaimJob atomically claims a job for a worker.
func (s *BoltStore) ClaimJob(ctx context.Context, id uuid.UUID, workerID string) (bool, error) {
	var claimed bool
	err := s.db.Update(func(tx *bolt.Tx) error {
		jobs := tx.Bucket(jobsBucket)
		key := []byte(id.String())

		data := jobs.Get(key)
		if data == nil {
			return core.ErrJobNotFound
		}

		job, err := decodeJob(data)
		if err != nil {
			return err
		}

		if job.State != core.JobStateScheduled && job.State != core.JobStatePending {
			claimed = false
			return nil
		}

		job.State = core.JobStateRunning
		job.WorkerID = workerID
		job.StartedAt = time.Now().UTC()
		job.UpdatedAt = job.StartedAt
		job.Attempt++

		newData, err := encodeJob(job)
		if err != nil {
			return err
		}

		claimed = true
		return jobs.Put(key, newData)
	})

	return claimed, err
}

// SaveResult stores the result of a job execution.
func (s *BoltStore) SaveResult(ctx context.Context, result *core.JobResult) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		data, err := encodeResult(result)
		if err != nil {
			return err
		}
		return tx.Bucket(resultsBucket).Put([]byte(result.JobID.String()), data)
	})
}

// GetResult retrieves the result for a completed job.
func (s *BoltStore) GetResult(ctx context.Context, jobID uuid.UUID) (*core.JobResult, error) {
	var result *core.JobResult
	err := s.db.View(func(tx *bolt.Tx) error {
		data := tx.Bucket(resultsBucket).Get([]byte(jobID.String()))
		if data == nil {
			return core.ErrJobNotFound
		}
		var err error
		result, err = decodeResult(data)
		return err
	})
	return result, err
}

// SaveCheckpoint stores a checkpoint for a running job.
func (s *BoltStore) SaveCheckpoint(ctx context.Context, jobID uuid.UUID, data []byte) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(checkpointBucket).Put([]byte(jobID.String()), data)
	})
}

// GetCheckpoint retrieves the last checkpoint for a job.
func (s *BoltStore) GetCheckpoint(ctx context.Context, jobID uuid.UUID) ([]byte, error) {
	var data []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		result := tx.Bucket(checkpointBucket).Get([]byte(jobID.String()))
		if result != nil {
			data = make([]byte, len(result))
			copy(data, result)
		}
		return nil
	})
	return data, err
}

// Stats returns queue statistics.
func (s *BoltStore) Stats(ctx context.Context) (*QueueStats, error) {
	stats := &QueueStats{
		Timestamp: time.Now().UTC(),
	}

	err := s.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(jobsBucket).Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			job, err := decodeJob(v)
			if err != nil {
				continue
			}

			stats.TotalJobs++
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
		return nil
	})

	// Get storage size
	if info, err := os.Stat(s.path); err == nil {
		stats.StorageBytes = info.Size()
	}

	return stats, err
}

// Cleanup removes old completed/failed jobs.
func (s *BoltStore) Cleanup(ctx context.Context, retention time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retention)
	var removed int64

	err := s.db.Update(func(tx *bolt.Tx) error {
		jobs := tx.Bucket(jobsBucket)
		idem := tx.Bucket(idempotencyBucket)
		results := tx.Bucket(resultsBucket)
		checkpoints := tx.Bucket(checkpointBucket)
		c := jobs.Cursor()

		var toDelete [][]byte
		for k, v := c.First(); k != nil; k, v = c.Next() {
			job, err := decodeJob(v)
			if err != nil {
				continue
			}

			if (job.State == core.JobStateCompleted || job.State == core.JobStateFailed || job.State == core.JobStateCancelled) &&
				!job.CompletedAt.IsZero() && job.CompletedAt.Before(cutoff) {
				toDelete = append(toDelete, append([]byte{}, k...))
				if job.IdempotencyKey != "" {
					idem.Delete([]byte(job.IdempotencyKey))
				}
			}
		}

		for _, k := range toDelete {
			jobs.Delete(k)
			results.Delete(k)
			checkpoints.Delete(k)
			removed++
		}

		return nil
	})

	return removed, err
}

// Close releases resources.
func (s *BoltStore) Close() error {
	return s.db.Close()
}

// Encoding/decoding helpers
func encodeJob(job *core.Job) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(job); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeJob(data []byte) (*core.Job, error) {
	var job core.Job
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&job); err != nil {
		return nil, err
	}
	return &job, nil
}

func encodeResult(result *core.JobResult) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(result); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeResult(data []byte) (*core.JobResult, error) {
	var result core.JobResult
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// sortJobs sorts jobs based on the specified order.
func sortJobs(jobs []*core.Job, order ListOrder) {
	switch order {
	case OrderCreatedAsc:
		sort.Slice(jobs, func(i, j int) bool {
			return jobs[i].CreatedAt.Before(jobs[j].CreatedAt)
		})
	case OrderCreatedDesc:
		sort.Slice(jobs, func(i, j int) bool {
			return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
		})
	case OrderUpdatedDesc:
		sort.Slice(jobs, func(i, j int) bool {
			return jobs[i].UpdatedAt.After(jobs[j].UpdatedAt)
		})
	case OrderPriorityAsc:
		sort.Slice(jobs, func(i, j int) bool {
			if jobs[i].Priority != jobs[j].Priority {
				return jobs[i].Priority < jobs[j].Priority
			}
			return jobs[i].CreatedAt.Before(jobs[j].CreatedAt)
		})
	}
}

// Ensure BoltStore implements JobStore.
var _ JobStore = (*BoltStore)(nil)
