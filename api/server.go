// Package api provides the HTTP API server for GopherQueue.
package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/sa001gar/gopherqueue/observability"
	"github.com/sa001gar/gopherqueue/persistence"
	"github.com/sa001gar/gopherqueue/scheduler"
	"github.com/sa001gar/gopherqueue/security"
	"github.com/sa001gar/gopherqueue/worker"
)

// Server is the HTTP API server.
type Server struct {
	httpServer *http.Server
	store      persistence.JobStore
	scheduler  scheduler.Scheduler
	pool       worker.WorkerPool
	metrics    observability.MetricsCollector
	health     observability.HealthChecker
	auth       security.Authenticator
	authz      security.Authorizer
	eventBus   *EventBus

	config *ServerConfig
}

// ServerConfig configures the API server.
type ServerConfig struct {
	Addr           string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	MaxRequestSize int64
	AuthEnabled    bool
	CORSEnabled    bool
	CORSOrigins    []string
}

// DefaultServerConfig returns sensible defaults.
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Addr:           ":8080",
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		MaxRequestSize: 10 * 1024 * 1024, // 10MB
		AuthEnabled:    false,
		CORSEnabled:    true,
		CORSOrigins:    []string{"*"},
	}
}

// NewServer creates a new API server.
func NewServer(
	store persistence.JobStore,
	sched scheduler.Scheduler,
	pool worker.WorkerPool,
	config *ServerConfig,
) *Server {
	if config == nil {
		config = DefaultServerConfig()
	}

	s := &Server{
		store:     store,
		scheduler: sched,
		pool:      pool,
		metrics:   observability.NewSimpleMetrics(),
		health:    observability.NewSimpleHealthChecker(store, sched, pool),
		authz:     security.NewSimpleAuthorizer(),
		eventBus:  NewEventBus(),
		config:    config,
	}

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.httpServer = &http.Server{
		Addr:         config.Addr,
		Handler:      s.middleware(mux),
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	}

	return s
}

// SetAuthenticator sets the authenticator.
func (s *Server) SetAuthenticator(auth security.Authenticator) {
	s.auth = auth
	s.config.AuthEnabled = true
}

// GetEventBus returns the event bus for publishing events.
func (s *Server) GetEventBus() *EventBus {
	return s.eventBus
}

// registerRoutes registers all API routes.
func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Jobs API
	mux.HandleFunc("/api/v1/jobs", s.handleJobs)
	mux.HandleFunc("/api/v1/jobs/", s.handleJobByID)
	mux.HandleFunc("/api/v1/jobs/batch", s.handleBatchSubmit)

	// Stats API
	mux.HandleFunc("/api/v1/stats", s.handleStats)

	// SSE Events API
	mux.HandleFunc("/api/v1/events", s.handleEvents)

	// Health endpoints
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)
	mux.HandleFunc("/live", s.handleLive)

	// Metrics endpoint
	mux.HandleFunc("/metrics", s.handleMetrics)

	// API documentation endpoint
	mux.HandleFunc("/api/v1", s.handleAPIInfo)
	mux.HandleFunc("/api/v1/", s.handleAPIInfo)
}

// handleAPIInfo returns API information and available endpoints.
func (s *Server) handleAPIInfo(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/v1" && r.URL.Path != "/api/v1/" {
		s.writeError(w, http.StatusNotFound, "not_found", "endpoint not found")
		return
	}

	info := map[string]interface{}{
		"name":    "GopherQueue API",
		"version": "1.0.0",
		"endpoints": map[string]string{
			"POST /api/v1/jobs":             "Submit a new job",
			"GET /api/v1/jobs":              "List jobs with filters",
			"POST /api/v1/jobs/batch":       "Submit multiple jobs",
			"GET /api/v1/jobs/{id}":         "Get job by ID",
			"DELETE /api/v1/jobs/{id}":      "Delete a job",
			"POST /api/v1/jobs/{id}/wait":   "Wait for job completion (long-polling)",
			"POST /api/v1/jobs/{id}/cancel": "Cancel a job",
			"POST /api/v1/jobs/{id}/retry":  "Retry a failed job",
			"GET /api/v1/jobs/{id}/result":  "Get job result",
			"GET /api/v1/stats":             "Get queue statistics",
			"GET /api/v1/events":            "SSE stream for real-time updates",
			"GET /health":                   "Full health status",
			"GET /live":                     "Liveness probe",
			"GET /ready":                    "Readiness probe",
			"GET /metrics":                  "Metrics endpoint",
		},
		"features": []string{
			"Priority queues (Critical, High, Normal, Low, Bulk)",
			"Batch job submission (up to 1000 jobs)",
			"Long-polling job wait",
			"Server-Sent Events (SSE) for real-time updates",
			"Automatic retries with exponential backoff",
			"Job checkpointing for crash recovery",
			"API key authentication",
			"Role-based authorization",
		},
	}

	s.writeJSON(w, http.StatusOK, info)
}

// middleware wraps handlers with common middleware.
func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// CORS headers
		if s.config.CORSEnabled {
			origin := r.Header.Get("Origin")
			if origin != "" {
				allowed := false
				for _, o := range s.config.CORSOrigins {
					if o == "*" || o == origin {
						allowed = true
						break
					}
				}
				if allowed {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, X-Request-ID")
					w.Header().Set("Access-Control-Allow-Credentials", "true")
					w.Header().Set("Access-Control-Max-Age", "86400")
				}
			}

			// Handle preflight
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}

		// Request ID
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = time.Now().Format("20060102150405.000000")
		}
		w.Header().Set("X-Request-ID", requestID)

		// Authentication
		if s.config.AuthEnabled && s.auth != nil {
			// Skip auth for health endpoints
			if r.URL.Path != "/health" && r.URL.Path != "/live" && r.URL.Path != "/ready" {
				principal, err := s.auth.AuthenticateRequest(r)
				if err != nil {
					s.writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
					return
				}
				r = r.WithContext(security.WithPrincipal(r.Context(), principal))
			}
		}

		// Process request
		next.ServeHTTP(w, r)

		// Log request
		log.Printf("[API] %s %s %s %v", r.Method, r.URL.Path, requestID, time.Since(start))
	})
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	log.Printf("[API] Starting server on %s", s.config.Addr)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[API] Server error: %v", err)
		}
	}()
	return nil
}

// Stop gracefully shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	log.Println("[API] Shutting down server...")
	return s.httpServer.Shutdown(ctx)
}

// Health check handlers
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.health.ServeHTTP(w, r)
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	status := s.health.CheckHealth(r.Context())
	w.Header().Set("Content-Type", "application/json")
	if !status.Healthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	json.NewEncoder(w).Encode(map[string]bool{"ready": status.Healthy}) //nolint:errcheck
}

func (s *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"alive": true}) //nolint:errcheck
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	s.metrics.ServeHTTP(w, r)
}

// writeJSON writes a JSON response.
func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// writeError writes an error response.
func (s *Server) writeError(w http.ResponseWriter, status int, code, message string) {
	s.writeJSON(w, status, map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
