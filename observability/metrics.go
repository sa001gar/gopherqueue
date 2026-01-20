// Package observability provides metrics, logging, and health checks for GopherQueue.
package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sa001gar/gopherqueue/persistence"
	"github.com/sa001gar/gopherqueue/scheduler"
	"github.com/sa001gar/gopherqueue/worker"
)

// MetricsCollector collects and exposes system metrics.
type MetricsCollector interface {
	// RecordJobEnqueued records a job being enqueued.
	RecordJobEnqueued(jobType string, priority int)

	// RecordJobCompleted records a job completion.
	RecordJobCompleted(jobType string, duration time.Duration, success bool)

	// RecordJobRetry records a job retry.
	RecordJobRetry(jobType string, attempt int)

	// GetMetrics returns current metrics.
	GetMetrics() *Metrics

	// ServeHTTP handles metrics requests.
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

// Metrics contains all system metrics.
type Metrics struct {
	// Job metrics
	JobsEnqueued  int64            `json:"jobs_enqueued"`
	JobsCompleted int64            `json:"jobs_completed"`
	JobsFailed    int64            `json:"jobs_failed"`
	JobsRetried   int64            `json:"jobs_retried"`
	JobsByType    map[string]int64 `json:"jobs_by_type"`

	// Latency metrics
	AvgProcessingTime time.Duration `json:"avg_processing_time"`
	MaxProcessingTime time.Duration `json:"max_processing_time"`

	// Queue metrics
	QueueDepth int64 `json:"queue_depth"`

	// System metrics
	Uptime    time.Duration `json:"uptime"`
	Timestamp time.Time     `json:"timestamp"`
}

// SimpleMetrics is a basic metrics collector implementation.
type SimpleMetrics struct {
	startTime time.Time

	enqueued  int64
	completed int64
	failed    int64
	retried   int64

	totalDuration int64
	maxDuration   int64
	jobCount      int64

	byType   map[string]int64
	byTypeMu sync.RWMutex
}

// NewSimpleMetrics creates a new metrics collector.
func NewSimpleMetrics() *SimpleMetrics {
	return &SimpleMetrics{
		startTime: time.Now(),
		byType:    make(map[string]int64),
	}
}

// RecordJobEnqueued records a job being enqueued.
func (m *SimpleMetrics) RecordJobEnqueued(jobType string, priority int) {
	atomic.AddInt64(&m.enqueued, 1)
	m.byTypeMu.Lock()
	m.byType[jobType]++
	m.byTypeMu.Unlock()
}

// RecordJobCompleted records a job completion.
func (m *SimpleMetrics) RecordJobCompleted(jobType string, duration time.Duration, success bool) {
	if success {
		atomic.AddInt64(&m.completed, 1)
	} else {
		atomic.AddInt64(&m.failed, 1)
	}

	atomic.AddInt64(&m.jobCount, 1)
	atomic.AddInt64(&m.totalDuration, int64(duration))

	for {
		current := atomic.LoadInt64(&m.maxDuration)
		if int64(duration) <= current {
			break
		}
		if atomic.CompareAndSwapInt64(&m.maxDuration, current, int64(duration)) {
			break
		}
	}
}

// RecordJobRetry records a job retry.
func (m *SimpleMetrics) RecordJobRetry(jobType string, attempt int) {
	atomic.AddInt64(&m.retried, 1)
}

// GetMetrics returns current metrics.
func (m *SimpleMetrics) GetMetrics() *Metrics {
	m.byTypeMu.RLock()
	byType := make(map[string]int64)
	for k, v := range m.byType {
		byType[k] = v
	}
	m.byTypeMu.RUnlock()

	jobCount := atomic.LoadInt64(&m.jobCount)
	var avgDuration time.Duration
	if jobCount > 0 {
		avgDuration = time.Duration(atomic.LoadInt64(&m.totalDuration) / jobCount)
	}

	return &Metrics{
		JobsEnqueued:      atomic.LoadInt64(&m.enqueued),
		JobsCompleted:     atomic.LoadInt64(&m.completed),
		JobsFailed:        atomic.LoadInt64(&m.failed),
		JobsRetried:       atomic.LoadInt64(&m.retried),
		JobsByType:        byType,
		AvgProcessingTime: avgDuration,
		MaxProcessingTime: time.Duration(atomic.LoadInt64(&m.maxDuration)),
		Uptime:            time.Since(m.startTime),
		Timestamp:         time.Now(),
	}
}

// ServeHTTP handles metrics requests.
func (m *SimpleMetrics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	metrics := m.GetMetrics()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(metrics)
}

// HealthChecker provides health check endpoints.
type HealthChecker interface {
	// CheckHealth performs a health check.
	CheckHealth(ctx context.Context) *HealthStatus

	// ServeHTTP handles health check requests.
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

// HealthStatus contains health check results.
type HealthStatus struct {
	Healthy    bool                        `json:"healthy"`
	Components map[string]*ComponentHealth `json:"components"`
	Timestamp  time.Time                   `json:"timestamp"`
}

// ComponentHealth represents health of a single component.
type ComponentHealth struct {
	Healthy bool          `json:"healthy"`
	Message string        `json:"message,omitempty"`
	Latency time.Duration `json:"latency,omitempty"`
}

// SimpleHealthChecker is a basic health checker implementation.
type SimpleHealthChecker struct {
	store     persistence.JobStore
	scheduler scheduler.Scheduler
	pool      worker.WorkerPool
}

// NewSimpleHealthChecker creates a new health checker.
func NewSimpleHealthChecker(store persistence.JobStore, sched scheduler.Scheduler, pool worker.WorkerPool) *SimpleHealthChecker {
	return &SimpleHealthChecker{
		store:     store,
		scheduler: sched,
		pool:      pool,
	}
}

// CheckHealth performs a health check.
func (h *SimpleHealthChecker) CheckHealth(ctx context.Context) *HealthStatus {
	status := &HealthStatus{
		Healthy:    true,
		Components: make(map[string]*ComponentHealth),
		Timestamp:  time.Now(),
	}

	// Check store
	start := time.Now()
	if _, err := h.store.Stats(ctx); err != nil {
		status.Healthy = false
		status.Components["store"] = &ComponentHealth{
			Healthy: false,
			Message: fmt.Sprintf("failed to get stats: %v", err),
			Latency: time.Since(start),
		}
	} else {
		status.Components["store"] = &ComponentHealth{
			Healthy: true,
			Latency: time.Since(start),
		}
	}

	// Check scheduler
	if h.scheduler != nil && h.scheduler.IsHealthy() {
		status.Components["scheduler"] = &ComponentHealth{
			Healthy: true,
		}
	} else {
		status.Healthy = false
		status.Components["scheduler"] = &ComponentHealth{
			Healthy: false,
			Message: "scheduler not running",
		}
	}

	// Check worker pool
	if h.pool != nil && h.pool.IsHealthy() {
		poolStats := h.pool.Stats()
		status.Components["workers"] = &ComponentHealth{
			Healthy: true,
			Message: fmt.Sprintf("%d/%d active", poolStats.ActiveWorkers, poolStats.TotalWorkers),
		}
	} else {
		status.Healthy = false
		status.Components["workers"] = &ComponentHealth{
			Healthy: false,
			Message: "worker pool not running",
		}
	}

	return status
}

// ServeHTTP handles health check requests.
func (h *SimpleHealthChecker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	status := h.CheckHealth(ctx)

	w.Header().Set("Content-Type", "application/json")
	if !status.Healthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(w).Encode(status)
}

// Ensure implementations match interfaces.
var _ MetricsCollector = (*SimpleMetrics)(nil)
var _ HealthChecker = (*SimpleHealthChecker)(nil)
