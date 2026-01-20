// Package recovery provides job recovery and orphan detection for GopherQueue.
package recovery

import (
	"context"
	"log"
	"sync/atomic"
	"time"

	"github.com/sa001gar/gopherqueue/core"
	"github.com/sa001gar/gopherqueue/persistence"
)

// RecoveryManager handles detection and recovery of stuck jobs.
type RecoveryManager interface {
	// Start begins the recovery manager.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the recovery manager.
	Stop(ctx context.Context) error

	// RecoverStuckJobs scans for and recovers stuck jobs.
	RecoverStuckJobs(ctx context.Context) (int, error)

	// RecoverOrphanedJobs scans for and recovers orphaned jobs.
	RecoverOrphanedJobs(ctx context.Context) (int, error)

	// Stats returns recovery statistics.
	Stats() *RecoveryStats

	// IsHealthy returns whether the recovery manager is operating normally.
	IsHealthy() bool
}

// RecoveryStats contains recovery metrics.
type RecoveryStats struct {
	StuckJobsRecovered    int64         `json:"stuck_jobs_recovered"`
	OrphanedJobsRecovered int64         `json:"orphaned_jobs_recovered"`
	LastRecoveryTime      time.Time     `json:"last_recovery_time"`
	LastRecoveryDuration  time.Duration `json:"last_recovery_duration"`
	Healthy               bool          `json:"healthy"`
}

// RecoveryConfig configures recovery behavior.
type RecoveryConfig struct {
	// How often to scan for stuck jobs
	ScanInterval time.Duration

	// How long a job can be running before considered stuck
	VisibilityTimeout time.Duration

	// Maximum jobs to recover per scan
	BatchSize int
}

// DefaultRecoveryConfig returns sensible defaults.
func DefaultRecoveryConfig() *RecoveryConfig {
	return &RecoveryConfig{
		ScanInterval:      1 * time.Minute,
		VisibilityTimeout: 5 * time.Minute,
		BatchSize:         100,
	}
}

// SimpleRecoveryManager is a basic recovery manager implementation.
type SimpleRecoveryManager struct {
	store   persistence.JobStore
	config  *RecoveryConfig
	running int32
	stopCh  chan struct{}
	stats   *RecoveryStats
}

// NewSimpleRecoveryManager creates a new recovery manager.
func NewSimpleRecoveryManager(store persistence.JobStore, config *RecoveryConfig) *SimpleRecoveryManager {
	if config == nil {
		config = DefaultRecoveryConfig()
	}
	return &SimpleRecoveryManager{
		store:  store,
		config: config,
		stats:  &RecoveryStats{Healthy: true},
		stopCh: make(chan struct{}),
	}
}

// Start begins the recovery manager.
func (r *SimpleRecoveryManager) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&r.running, 0, 1) {
		return nil
	}

	go r.runRecoveryLoop(ctx)
	log.Println("[RecoveryManager] Started")
	return nil
}

// Stop gracefully shuts down the recovery manager.
func (r *SimpleRecoveryManager) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&r.running, 1, 0) {
		return nil
	}

	close(r.stopCh)
	log.Println("[RecoveryManager] Stopped")
	return nil
}

// runRecoveryLoop periodically scans for stuck jobs.
func (r *SimpleRecoveryManager) runRecoveryLoop(ctx context.Context) {
	ticker := time.NewTicker(r.config.ScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case <-ticker.C:
			_, _ = r.RecoverStuckJobs(ctx)
		}
	}
}

// RecoverStuckJobs scans for and recovers stuck jobs.
func (r *SimpleRecoveryManager) RecoverStuckJobs(ctx context.Context) (int, error) {
	start := time.Now()

	jobs, err := r.store.GetStuck(ctx, r.config.VisibilityTimeout)
	if err != nil {
		return 0, err
	}

	recovered := 0
	for _, job := range jobs {
		// Check if should retry
		if job.Attempt < job.MaxAttempts {
			job.State = core.JobStateRetrying
			job.WorkerID = ""
			job.ScheduledAt = time.Now().Add(time.Duration(job.Attempt) * time.Second * 10)
		} else {
			job.State = core.JobStateDeadLetter
			job.CompletedAt = time.Now()
			job.LastError = "job exceeded visibility timeout"
		}
		job.UpdatedAt = time.Now()

		if err := r.store.Update(ctx, job); err != nil {
			log.Printf("[RecoveryManager] Failed to recover job %s: %v", job.ID, err)
			continue
		}

		recovered++
		atomic.AddInt64(&r.stats.StuckJobsRecovered, 1)
		log.Printf("[RecoveryManager] Recovered stuck job %s, attempt %d/%d", job.ID, job.Attempt, job.MaxAttempts)
	}

	r.stats.LastRecoveryTime = start
	r.stats.LastRecoveryDuration = time.Since(start)

	return recovered, nil
}

// RecoverOrphanedJobs scans for and recovers orphaned jobs.
func (r *SimpleRecoveryManager) RecoverOrphanedJobs(ctx context.Context) (int, error) {
	// In a distributed system, this would check for jobs assigned to dead workers
	// For now, this is similar to stuck job recovery
	return r.RecoverStuckJobs(ctx)
}

// Stats returns recovery statistics.
func (r *SimpleRecoveryManager) Stats() *RecoveryStats {
	return &RecoveryStats{
		StuckJobsRecovered:    atomic.LoadInt64(&r.stats.StuckJobsRecovered),
		OrphanedJobsRecovered: atomic.LoadInt64(&r.stats.OrphanedJobsRecovered),
		LastRecoveryTime:      r.stats.LastRecoveryTime,
		LastRecoveryDuration:  r.stats.LastRecoveryDuration,
		Healthy:               r.IsHealthy(),
	}
}

// IsHealthy returns whether the recovery manager is operating normally.
func (r *SimpleRecoveryManager) IsHealthy() bool {
	return atomic.LoadInt32(&r.running) == 1
}

// Ensure SimpleRecoveryManager implements RecoveryManager.
var _ RecoveryManager = (*SimpleRecoveryManager)(nil)
