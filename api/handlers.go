// Package api provides HTTP handlers for GopherQueue.
package api

import (
	"encoding/json"
	"net/http"
	"strings"
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

	// Build job options
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

	// Create job
	job := core.NewJob(req.Type, req.Payload, opts...)

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
