// Package worker provides a production-ready worker pool implementation.
package worker

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/sa001gar/gopherqueue/core"
	"github.com/sa001gar/gopherqueue/persistence"
	"github.com/sa001gar/gopherqueue/scheduler"
)

// SimplePool is a production-ready worker pool implementation.
type SimplePool struct {
	config    *WorkerConfig
	store     persistence.JobStore
	scheduler scheduler.Scheduler

	handlers  map[string]Handler
	handlerMu sync.RWMutex

	// Job channel
	jobCh chan *core.Job

	// State
	running   int32
	stopCh    chan struct{}
	wg        sync.WaitGroup
	startTime time.Time

	// Stats
	processed     int64
	succeeded     int64
	failed        int64
	activeCount   int32
	processingIDs sync.Map
}

// NewSimplePool creates a new worker pool.
func NewSimplePool(store persistence.JobStore, sched scheduler.Scheduler, config *WorkerConfig) *SimplePool {
	if config == nil {
		config = DefaultWorkerConfig()
	}
	return &SimplePool{
		config:    config,
		store:     store,
		scheduler: sched,
		handlers:  make(map[string]Handler),
		jobCh:     make(chan *core.Job, config.Concurrency*2),
		stopCh:    make(chan struct{}),
	}
}

// Start initializes the worker pool and begins processing.
func (p *SimplePool) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&p.running, 0, 1) {
		return nil
	}

	p.startTime = time.Now()

	// Start worker goroutines
	for i := 0; i < p.config.Concurrency; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i)
	}

	// Start job fetcher
	p.wg.Add(1)
	go p.jobFetcher(ctx)

	log.Printf("[WorkerPool] Started with %d workers", p.config.Concurrency)
	return nil
}

// Stop gracefully shuts down all workers.
func (p *SimplePool) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&p.running, 1, 0) {
		return nil
	}

	close(p.stopCh)

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("[WorkerPool] Stopped gracefully")
	case <-time.After(p.config.ShutdownGracePeriod):
		log.Println("[WorkerPool] Shutdown timed out")
	case <-ctx.Done():
		log.Println("[WorkerPool] Shutdown cancelled")
	}

	return nil
}

// RegisterHandler registers a handler for a job type.
func (p *SimplePool) RegisterHandler(jobType string, handler Handler) {
	p.handlerMu.Lock()
	defer p.handlerMu.Unlock()
	p.handlers[jobType] = handler
	log.Printf("[WorkerPool] Registered handler for job type: %s", jobType)
}

// Submit adds a job to the pool for execution.
func (p *SimplePool) Submit(job *core.Job) error {
	if atomic.LoadInt32(&p.running) == 0 {
		return fmt.Errorf("worker pool not running")
	}

	select {
	case p.jobCh <- job:
		return nil
	default:
		return fmt.Errorf("job queue full")
	}
}

// WaitForJob blocks until a job is available.
func (p *SimplePool) WaitForJob(ctx context.Context) (*core.Job, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case job := <-p.jobCh:
		return job, nil
	}
}

// CompleteJob is called when a job finishes successfully.
func (p *SimplePool) CompleteJob(ctx context.Context, result *core.JobResult) error {
	return p.scheduler.Complete(ctx, result.JobID, result)
}

// FailJob is called when a job fails.
func (p *SimplePool) FailJob(ctx context.Context, result *core.JobResult) error {
	return p.scheduler.Fail(ctx, result.JobID, result)
}

// jobFetcher fetches jobs from the scheduler.
func (p *SimplePool) jobFetcher(ctx context.Context) {
	defer p.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		default:
			job, err := p.scheduler.GetNextJob(ctx)
			if err != nil {
				if err != context.Canceled {
					time.Sleep(100 * time.Millisecond)
				}
				continue
			}

			select {
			case p.jobCh <- job:
			case <-p.stopCh:
				return
			}
		}
	}
}

// worker is the main worker loop.
func (p *SimplePool) worker(ctx context.Context, id int) {
	defer p.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case job := <-p.jobCh:
			p.processJob(ctx, job, id)
		}
	}
}

// processJob executes a single job with panic recovery.
func (p *SimplePool) processJob(ctx context.Context, job *core.Job, workerID int) {
	atomic.AddInt32(&p.activeCount, 1)
	p.processingIDs.Store(job.ID, true)
	defer func() {
		atomic.AddInt32(&p.activeCount, -1)
		p.processingIDs.Delete(job.ID)
	}()

	start := time.Now()
	atomic.AddInt64(&p.processed, 1)

	// Claim the job
	claimed, err := p.store.ClaimJob(ctx, job.ID, p.config.ID)
	if err != nil || !claimed {
		log.Printf("[Worker %d] Failed to claim job %s: %v", workerID, job.ID, err)
		return
	}

	// Get handler
	p.handlerMu.RLock()
	handler, exists := p.handlers[job.Type]
	p.handlerMu.RUnlock()

	if !exists {
		result := &core.JobResult{
			JobID:         job.ID,
			Error:         fmt.Sprintf("no handler registered for job type: %s", job.Type),
			ErrorCategory: core.ErrorCategoryPermanent,
			CompletedAt:   time.Now(),
			Duration:      time.Since(start),
		}
		p.FailJob(ctx, result)
		atomic.AddInt64(&p.failed, 1)
		return
	}

	// Create job context
	jobCtx := p.createJobContext(ctx, job)

	// Execute with timeout
	timeout := job.Timeout
	if timeout == 0 {
		timeout = p.config.JobTimeout
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute with panic recovery
	err = p.executeWithRecovery(execCtx, jobCtx, handler)

	duration := time.Since(start)

	if err != nil {
		atomic.AddInt64(&p.failed, 1)
		result := &core.JobResult{
			JobID:         job.ID,
			Error:         err.Error(),
			ErrorCategory: core.ClassifyError(err),
			CompletedAt:   time.Now(),
			Duration:      duration,
		}
		p.FailJob(ctx, result)
		log.Printf("[Worker %d] Job %s failed after %v: %v", workerID, job.ID, duration, err)
	} else {
		atomic.AddInt64(&p.succeeded, 1)
		result := &core.JobResult{
			JobID:       job.ID,
			Success:     true,
			Output:      jobCtx.(*jobContext).output,
			CompletedAt: time.Now(),
			Duration:    duration,
		}
		p.CompleteJob(ctx, result)
		log.Printf("[Worker %d] Job %s completed in %v", workerID, job.ID, duration)
	}
}

// executeWithRecovery executes the handler with panic recovery.
func (p *SimplePool) executeWithRecovery(ctx context.Context, jctx core.JobContext, handler Handler) (err error) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			err = &core.PermanentError{
				Message: fmt.Sprintf("panic: %v\n%s", r, stack),
			}
		}
	}()
	return handler(ctx, jctx)
}

// createJobContext creates a job context for execution.
func (p *SimplePool) createJobContext(ctx context.Context, job *core.Job) core.JobContext {
	return &jobContext{
		ctx:   ctx,
		job:   job,
		store: p.store,
	}
}

// Stats returns current worker pool statistics.
func (p *SimplePool) Stats() *PoolStats {
	var processingIDs []uuid.UUID
	p.processingIDs.Range(func(key, value interface{}) bool {
		processingIDs = append(processingIDs, key.(uuid.UUID))
		return true
	})

	active := int(atomic.LoadInt32(&p.activeCount))
	return &PoolStats{
		TotalWorkers:  p.config.Concurrency,
		ActiveWorkers: active,
		IdleWorkers:   p.config.Concurrency - active,
		QueuedJobs:    int64(len(p.jobCh)),
		ProcessedJobs: atomic.LoadInt64(&p.processed),
		SucceededJobs: atomic.LoadInt64(&p.succeeded),
		FailedJobs:    atomic.LoadInt64(&p.failed),
		ProcessingIDs: processingIDs,
		Uptime:        time.Since(p.startTime),
		Healthy:       atomic.LoadInt32(&p.running) == 1,
	}
}

// IsHealthy returns whether the worker pool is operating normally.
func (p *SimplePool) IsHealthy() bool {
	return atomic.LoadInt32(&p.running) == 1
}

// jobContext implements core.JobContext.
type jobContext struct {
	ctx    context.Context
	job    *core.Job
	store  persistence.JobStore
	output []byte
}

func (jc *jobContext) Job() *core.Job {
	return jc.job
}

func (jc *jobContext) Context() context.Context {
	return jc.ctx
}

func (jc *jobContext) Checkpoint(data []byte) error {
	return jc.store.SaveCheckpoint(jc.ctx, jc.job.ID, data)
}

func (jc *jobContext) LastCheckpoint() ([]byte, error) {
	return jc.store.GetCheckpoint(jc.ctx, jc.job.ID)
}

func (jc *jobContext) SetOutput(data []byte) {
	jc.output = data
}

func (jc *jobContext) Progress(percent float64, message string) {
	jc.job.Progress = percent
	jc.job.ProgressMessage = message
}

func (jc *jobContext) Logger() interface{} {
	return log.Default()
}

// Ensure SimplePool implements WorkerPool.
var _ WorkerPool = (*SimplePool)(nil)
