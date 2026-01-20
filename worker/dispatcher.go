// Package worker provides the dispatcher that bridges scheduler and worker pool.
package worker

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/sa001gar/gopherqueue/core"
	"github.com/sa001gar/gopherqueue/scheduler"
)

// Dispatcher bridges the scheduler and worker pool, managing job flow.
type Dispatcher interface {
	// Start begins the dispatcher's main loop.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the dispatcher.
	Stop(ctx context.Context) error

	// Stats returns dispatcher statistics.
	Stats() *DispatcherStats

	// IsHealthy returns whether the dispatcher is operating normally.
	IsHealthy() bool
}

// DispatcherStats contains dispatcher metrics.
type DispatcherStats struct {
	JobsDispatched     int64         `json:"jobs_dispatched"`
	JobsCompleted      int64         `json:"jobs_completed"`
	JobsFailed         int64         `json:"jobs_failed"`
	AvgDispatchLatency time.Duration `json:"avg_dispatch_latency"`
	Healthy            bool          `json:"healthy"`
}

// SimpleDispatcher is a basic dispatcher implementation.
type SimpleDispatcher struct {
	scheduler  scheduler.Scheduler
	pool       WorkerPool
	running    int32
	stopCh     chan struct{}
	dispatched int64
	completed  int64
	failed     int64
}

// NewSimpleDispatcher creates a new dispatcher.
func NewSimpleDispatcher(sched scheduler.Scheduler, pool WorkerPool) *SimpleDispatcher {
	return &SimpleDispatcher{
		scheduler: sched,
		pool:      pool,
		stopCh:    make(chan struct{}),
	}
}

// Start begins the dispatcher's main loop.
func (d *SimpleDispatcher) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&d.running, 0, 1) {
		return nil
	}

	go d.dispatchLoop(ctx)
	return nil
}

// Stop gracefully shuts down the dispatcher.
func (d *SimpleDispatcher) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&d.running, 1, 0) {
		return nil
	}
	close(d.stopCh)
	return nil
}

// dispatchLoop continuously fetches and dispatches jobs.
func (d *SimpleDispatcher) dispatchLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		default:
			job, err := d.scheduler.GetNextJob(ctx)
			if err != nil {
				if err != context.Canceled {
					time.Sleep(100 * time.Millisecond)
				}
				continue
			}

			if err := d.pool.Submit(job); err != nil {
				// Re-enqueue on failure
				d.scheduler.Enqueue(ctx, job)
				continue
			}

			atomic.AddInt64(&d.dispatched, 1)
		}
	}
}

// RecordCompletion records a successful job completion.
func (d *SimpleDispatcher) RecordCompletion(result *core.JobResult) {
	if result.Success {
		atomic.AddInt64(&d.completed, 1)
	} else {
		atomic.AddInt64(&d.failed, 1)
	}
}

// Stats returns dispatcher statistics.
func (d *SimpleDispatcher) Stats() *DispatcherStats {
	return &DispatcherStats{
		JobsDispatched: atomic.LoadInt64(&d.dispatched),
		JobsCompleted:  atomic.LoadInt64(&d.completed),
		JobsFailed:     atomic.LoadInt64(&d.failed),
		Healthy:        d.IsHealthy(),
	}
}

// IsHealthy returns whether the dispatcher is operating normally.
func (d *SimpleDispatcher) IsHealthy() bool {
	return atomic.LoadInt32(&d.running) == 1
}

// Ensure SimpleDispatcher implements Dispatcher.
var _ Dispatcher = (*SimpleDispatcher)(nil)
