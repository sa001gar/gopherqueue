// Package api provides HTTP handlers for GopherQueue.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sa001gar/gopherqueue/core"
	"github.com/sa001gar/gopherqueue/persistence"
)

// JobSubmitRequest is the request body for job submission.
type JobSubmitRequest struct {
	Type           string            `json:"type"`
	Payload        json.RawMessage   `json:"payload"`
	Priority       *int              `json:"priority,omitempty"`
	IdempotencyKey string            `json:"idempotency_key,omitempty"`
	CorrelationID  string            `json:"correlation_id,omitempty"`
	Delay          string            `json:"delay,omitempty"`
	ScheduledAt    *time.Time        `json:"scheduled_at,omitempty"`
	Timeout        string            `json:"timeout,omitempty"`
	MaxAttempts    *int              `json:"max_attempts,omitempty"`
	Tags           map[string]string `json:"tags,omitempty"`
}

// JobResponse is the response for job operations.
type JobResponse struct {
	ID              string            `json:"id"`
	Type            string            `json:"type"`
	State           string            `json:"state"`
	Priority        int               `json:"priority"`
	Attempt         int               `json:"attempt"`
	MaxAttempts     int               `json:"max_attempts"`
	Progress        float64           `json:"progress,omitempty"`
	ProgressMessage string            `json:"progress_message,omitempty"`
	Tags            map[string]string `json:"tags,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	ScheduledAt     *time.Time        `json:"scheduled_at,omitempty"`
	StartedAt       *time.Time        `json:"started_at,omitempty"`
	CompletedAt     *time.Time        `json:"completed_at,omitempty"`
	LastError       string            `json:"last_error,omitempty"`
}

// StatsResponse is the response for statistics.
type StatsResponse struct {
	Queue   *persistence.QueueStats `json:"queue"`
	Workers interface{}             `json:"workers"`
}

// BatchSubmitRequest is the request body for batch job submission.
type BatchSubmitRequest struct {
	Jobs   []JobSubmitRequest `json:"jobs"`
	Atomic bool               `json:"atomic"` // All-or-nothing semantics
}

// BatchSubmitResponse is the response for batch job submission.
type BatchSubmitResponse struct {
	Total    int                    `json:"total"`
	Accepted int                    `json:"accepted"`
	Rejected int                    `json:"rejected"`
	Results  []BatchSubmitJobResult `json:"results"`
}

// BatchSubmitJobResult is the result for a single job in batch submission.
type BatchSubmitJobResult struct {
	Index   int         `json:"index"`
	Success bool        `json:"success"`
	Job     JobResponse `json:"job,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// WaitRequest is the request body for waiting on a job.
type WaitRequest struct {
	Timeout string `json:"timeout"` // e.g., "30s"
}

// WaitResponse is the response for waiting on a job.
type WaitResponse struct {
	ID        string          `json:"id"`
	State     string          `json:"state"`
	Completed bool            `json:"completed"`
	Success   bool            `json:"success,omitempty"`
	Result    *core.JobResult `json:"result,omitempty"`
	TimedOut  bool            `json:"timed_out,omitempty"`
}

// SSEEvent represents a Server-Sent Event.
type SSEEvent struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
	ID    string      `json:"id,omitempty"`
}

// EventBus manages SSE subscriptions for real-time job updates.
type EventBus struct {
	subscribers map[string][]chan SSEEvent
	mu          sync.RWMutex
}

// NewEventBus creates a new event bus.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string][]chan SSEEvent),
	}
}

// Subscribe subscribes to events for a specific job or all jobs.
func (eb *EventBus) Subscribe(jobID string) chan SSEEvent {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	ch := make(chan SSEEvent, 100)
	eb.subscribers[jobID] = append(eb.subscribers[jobID], ch)
	return ch
}

// Unsubscribe removes a subscription.
func (eb *EventBus) Unsubscribe(jobID string, ch chan SSEEvent) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	subscribers := eb.subscribers[jobID]
	for i, sub := range subscribers {
		if sub == ch {
			eb.subscribers[jobID] = append(subscribers[:i], subscribers[i+1:]...)
			close(ch)
			break
		}
	}
}

// Publish sends an event to all subscribers.
func (eb *EventBus) Publish(jobID string, event SSEEvent) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	// Send to job-specific subscribers
	for _, ch := range eb.subscribers[jobID] {
		select {
		case ch <- event:
		default:
			// Channel full, skip
		}
	}

	// Send to wildcard subscribers (subscribe to all)
	for _, ch := range eb.subscribers["*"] {
		select {
		case ch <- event:
		default:
		}
	}
}

// handleJobs handles /api/v1/jobs endpoints.
func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.submitJob(w, r)
	case http.MethodGet:
		s.listJobs(w, r)
	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

// handleJobByID handles /api/v1/jobs/{id} endpoints.
func (s *Server) handleJobByID(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/jobs/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "job ID required")
		return
	}

	idStr := parts[0]
	id, err := uuid.Parse(idStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "invalid job ID format")
		return
	}

	// Check for sub-resource
	if len(parts) > 1 {
		switch parts[1] {
		case "cancel":
			s.cancelJob(w, r, id)
		case "retry":
			s.retryJob(w, r, id)
		case "result":
			s.getJobResult(w, r, id)
		case "wait":
			s.waitForJob(w, r, id)
		default:
			s.writeError(w, http.StatusNotFound, "not_found", "resource not found")
		}
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getJob(w, r, id)
	case http.MethodDelete:
		s.deleteJob(w, r, id)
	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

// handleBatchSubmit handles /api/v1/jobs/batch endpoint.
func (s *Server) handleBatchSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req BatchSubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	if len(req.Jobs) == 0 {
		s.writeError(w, http.StatusBadRequest, "validation_error", "at least one job is required")
		return
	}

	if len(req.Jobs) > 1000 {
		s.writeError(w, http.StatusBadRequest, "validation_error", "maximum 1000 jobs per batch")
		return
	}

	response := BatchSubmitResponse{
		Total:   len(req.Jobs),
		Results: make([]BatchSubmitJobResult, len(req.Jobs)),
	}

	// Process jobs
	var createdJobs []*core.Job
	for i, jobReq := range req.Jobs {
		result := BatchSubmitJobResult{Index: i}

		if jobReq.Type == "" {
			result.Success = false
			result.Error = "job type is required"
			response.Rejected++
			response.Results[i] = result
			if req.Atomic {
				s.writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("job %d: type is required", i))
				return
			}
			continue
		}

		job := s.createJobFromRequest(jobReq)
		createdJobs = append(createdJobs, job)
		result.Success = true
		result.Job = jobToResponse(job)
		response.Accepted++
		response.Results[i] = result
	}

	// Persist and enqueue jobs
	for i, job := range createdJobs {
		if err := s.store.Create(r.Context(), job); err != nil {
			if req.Atomic {
				s.writeError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("failed to create job %d: %v", i, err))
				return
			}
			response.Results[i].Success = false
			response.Results[i].Error = err.Error()
			response.Accepted--
			response.Rejected++
			continue
		}

		if err := s.scheduler.Enqueue(r.Context(), job); err != nil {
			if req.Atomic {
				s.writeError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("failed to enqueue job %d: %v", i, err))
				return
			}
		}

		s.metrics.RecordJobEnqueued(job.Type, int(job.Priority))

		// Publish SSE event
		if s.eventBus != nil {
			s.eventBus.Publish(job.ID.String(), SSEEvent{
				Event: "job.created",
				Data:  jobToResponse(job),
				ID:    job.ID.String(),
			})
		}
	}

	s.writeJSON(w, http.StatusCreated, response)
}

// waitForJob handles long-polling wait for job completion.
func (s *Server) waitForJob(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	// Parse timeout
	timeout := 30 * time.Second
	if r.Method == http.MethodPost {
		var req WaitRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.Timeout != "" {
			if d, err := time.ParseDuration(req.Timeout); err == nil {
				timeout = d
			}
		}
	} else {
		if t := r.URL.Query().Get("timeout"); t != "" {
			if d, err := time.ParseDuration(t); err == nil {
				timeout = d
			}
		}
	}

	// Cap timeout at 5 minutes
	if timeout > 5*time.Minute {
		timeout = 5 * time.Minute
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	// Poll for job completion
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Timeout - return current state
			job, err := s.store.Get(r.Context(), id)
			if err != nil {
				s.writeError(w, http.StatusNotFound, "not_found", "job not found")
				return
			}
			s.writeJSON(w, http.StatusOK, WaitResponse{
				ID:        job.ID.String(),
				State:     string(job.State),
				Completed: job.IsTerminal(),
				TimedOut:  true,
			})
			return

		case <-ticker.C:
			job, err := s.store.Get(r.Context(), id)
			if err != nil {
				s.writeError(w, http.StatusNotFound, "not_found", "job not found")
				return
			}

			if job.IsTerminal() {
				response := WaitResponse{
					ID:        job.ID.String(),
					State:     string(job.State),
					Completed: true,
					Success:   job.State == core.JobStateCompleted,
				}

				// Get result if available
				if result, err := s.store.GetResult(r.Context(), id); err == nil {
					response.Result = result
				}

				s.writeJSON(w, http.StatusOK, response)
				return
			}
		}
	}
}

// handleEvents handles SSE endpoint for real-time job updates.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get job filter (optional)
	jobID := r.URL.Query().Get("job_id")
	if jobID == "" {
		jobID = "*" // Subscribe to all
	}

	// Subscribe to events
	ch := s.eventBus.Subscribe(jobID)
	defer s.eventBus.Unsubscribe(jobID, ch)

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, http.StatusInternalServerError, "internal_error", "streaming not supported")
		return
	}

	// Send initial connection event
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\"}\n\n")
	flusher.Flush()

	// Keep-alive ticker
	keepAlive := time.NewTicker(30 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepAlive.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case event := <-ch:
			data, _ := json.Marshal(event.Data)
			fmt.Fprintf(w, "event: %s\ndata: %s\n", event.Event, data)
			if event.ID != "" {
				fmt.Fprintf(w, "id: %s\n", event.ID)
			}
			fmt.Fprintf(w, "\n")
			flusher.Flush()
		}
	}
}

// createJobFromRequest creates a job from a submit request.
func (s *Server) createJobFromRequest(req JobSubmitRequest) *core.Job {
	var opts []core.JobOption

	if req.IdempotencyKey != "" {
		opts = append(opts, core.WithIdempotencyKey(req.IdempotencyKey))
	}
	if req.CorrelationID != "" {
		opts = append(opts, core.WithCorrelationID(req.CorrelationID))
	}
	if req.Priority != nil {
		opts = append(opts, core.WithPriority(core.Priority(*req.Priority)))
	}
	if req.Delay != "" {
		if d, err := time.ParseDuration(req.Delay); err == nil {
			opts = append(opts, core.WithDelay(d))
		}
	}
	if req.ScheduledAt != nil {
		opts = append(opts, core.WithScheduledAt(*req.ScheduledAt))
	}
	if req.Timeout != "" {
		if d, err := time.ParseDuration(req.Timeout); err == nil {
			opts = append(opts, core.WithTimeout(d))
		}
	}
	if req.MaxAttempts != nil {
		opts = append(opts, core.WithMaxAttempts(*req.MaxAttempts))
	}
	if len(req.Tags) > 0 {
		opts = append(opts, core.WithTags(req.Tags))
	}

	return core.NewJob(req.Type, req.Payload, opts...)
}

// submitJob handles job submission.
func (s *Server) submitJob(w http.ResponseWriter, r *http.Request) {
	var req JobSubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	if req.Type == "" {
		s.writeError(w, http.StatusBadRequest, "validation_error", "job type is required")
		return
	}

	// Create job
	job := s.createJobFromRequest(req)

	// Save to store
	if err := s.store.Create(r.Context(), job); err != nil {
		if err == core.ErrJobAlreadyExists {
			// Return existing job for idempotent request
			if req.IdempotencyKey != "" {
				existing, _ := s.store.GetByIdempotencyKey(r.Context(), req.IdempotencyKey)
				if existing != nil {
					s.writeJSON(w, http.StatusOK, jobToResponse(existing))
					return
				}
			}
			s.writeError(w, http.StatusConflict, "conflict", "job already exists")
			return
		}
		s.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	// Enqueue for scheduling
	if err := s.scheduler.Enqueue(r.Context(), job); err != nil {
		s.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	// Record metric
	s.metrics.RecordJobEnqueued(job.Type, int(job.Priority))

	// Publish SSE event
	if s.eventBus != nil {
		s.eventBus.Publish(job.ID.String(), SSEEvent{
			Event: "job.created",
			Data:  jobToResponse(job),
			ID:    job.ID.String(),
		})
	}

	s.writeJSON(w, http.StatusCreated, jobToResponse(job))
}

// getJob handles getting a single job.
func (s *Server) getJob(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	job, err := s.store.Get(r.Context(), id)
	if err != nil {
		if err == core.ErrJobNotFound {
			s.writeError(w, http.StatusNotFound, "not_found", "job not found")
			return
		}
		s.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, jobToResponse(job))
}

// listJobs handles listing jobs with filters.
func (s *Server) listJobs(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	filter := persistence.JobFilter{
		Limit: 100,
		Order: persistence.OrderCreatedDesc,
	}

	// Parse state filter
	if state := query.Get("state"); state != "" {
		filter.States = []core.JobState{core.JobState(state)}
	}

	// Parse type filter
	if jobType := query.Get("type"); jobType != "" {
		filter.Types = []string{jobType}
	}

	jobs, err := s.store.List(r.Context(), filter)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	response := make([]JobResponse, len(jobs))
	for i, job := range jobs {
		response[i] = jobToResponse(job)
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"jobs":  response,
		"count": len(response),
	})
}

// cancelJob handles job cancellation.
func (s *Server) cancelJob(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req struct {
		Reason string `json:"reason"`
		Force  bool   `json:"force"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := s.scheduler.Cancel(r.Context(), id, req.Reason, req.Force); err != nil {
		if err == core.ErrJobNotFound {
			s.writeError(w, http.StatusNotFound, "not_found", "job not found")
			return
		}
		if err == core.ErrJobNotCancellable {
			s.writeError(w, http.StatusConflict, "conflict", "job cannot be cancelled")
			return
		}
		s.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	job, _ := s.store.Get(r.Context(), id)

	// Publish SSE event
	if s.eventBus != nil && job != nil {
		s.eventBus.Publish(job.ID.String(), SSEEvent{
			Event: "job.cancelled",
			Data:  jobToResponse(job),
			ID:    job.ID.String(),
		})
	}

	s.writeJSON(w, http.StatusOK, jobToResponse(job))
}

// retryJob handles job retry.
func (s *Server) retryJob(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req struct {
		ResetAttempts bool `json:"reset_attempts"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := s.scheduler.Retry(r.Context(), id, req.ResetAttempts); err != nil {
		if err == core.ErrJobNotFound {
			s.writeError(w, http.StatusNotFound, "not_found", "job not found")
			return
		}
		s.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	job, _ := s.store.Get(r.Context(), id)

	// Publish SSE event
	if s.eventBus != nil && job != nil {
		s.eventBus.Publish(job.ID.String(), SSEEvent{
			Event: "job.retried",
			Data:  jobToResponse(job),
			ID:    job.ID.String(),
		})
	}

	s.writeJSON(w, http.StatusOK, jobToResponse(job))
}

// getJobResult handles getting job result.
func (s *Server) getJobResult(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	result, err := s.store.GetResult(r.Context(), id)
	if err != nil {
		if err == core.ErrJobNotFound {
			s.writeError(w, http.StatusNotFound, "not_found", "result not found")
			return
		}
		s.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, result)
}

// deleteJob handles job deletion.
func (s *Server) deleteJob(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	if err := s.store.Delete(r.Context(), id); err != nil {
		if err == core.ErrJobNotFound {
			s.writeError(w, http.StatusNotFound, "not_found", "job not found")
			return
		}
		s.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	// Publish SSE event
	if s.eventBus != nil {
		s.eventBus.Publish(id.String(), SSEEvent{
			Event: "job.deleted",
			Data:  map[string]string{"id": id.String()},
			ID:    id.String(),
		})
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleStats handles /api/v1/stats endpoint.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	queueStats, err := s.store.Stats(r.Context())
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	var workerStats interface{}
	if s.pool != nil {
		workerStats = s.pool.Stats()
	}

	s.writeJSON(w, http.StatusOK, StatsResponse{
		Queue:   queueStats,
		Workers: workerStats,
	})
}

// jobToResponse converts a job to API response.
func jobToResponse(job *core.Job) JobResponse {
	resp := JobResponse{
		ID:              job.ID.String(),
		Type:            job.Type,
		State:           string(job.State),
		Priority:        int(job.Priority),
		Attempt:         job.Attempt,
		MaxAttempts:     job.MaxAttempts,
		Progress:        job.Progress,
		ProgressMessage: job.ProgressMessage,
		Tags:            job.Tags,
		CreatedAt:       job.CreatedAt,
		UpdatedAt:       job.UpdatedAt,
		LastError:       job.LastError,
	}

	if !job.ScheduledAt.IsZero() {
		resp.ScheduledAt = &job.ScheduledAt
	}
	if !job.StartedAt.IsZero() {
		resp.StartedAt = &job.StartedAt
	}
	if !job.CompletedAt.IsZero() {
		resp.CompletedAt = &job.CompletedAt
	}

	return resp
}
