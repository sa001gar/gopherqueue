// Package core provides configuration options for GopherQueue.
package core

import (
	"time"

	"github.com/google/uuid"
)

// JobOption is a functional option for configuring a job.
type JobOption func(*Job)

// WithID sets a specific job ID.
func WithID(id uuid.UUID) JobOption {
	return func(j *Job) {
		j.ID = id
	}
}

// WithIdempotencyKey sets the idempotency key.
func WithIdempotencyKey(key string) JobOption {
	return func(j *Job) {
		j.IdempotencyKey = key
	}
}

// WithCorrelationID sets the correlation ID for tracing.
func WithCorrelationID(id string) JobOption {
	return func(j *Job) {
		j.CorrelationID = id
	}
}

// WithPriority sets the job priority.
func WithPriority(priority Priority) JobOption {
	return func(j *Job) {
		j.Priority = priority
	}
}

// WithDelay sets the initial delay before processing.
func WithDelay(delay time.Duration) JobOption {
	return func(j *Job) {
		j.Delay = delay
		j.ScheduledAt = time.Now().Add(delay)
	}
}

// WithScheduledAt sets the specific time to run.
func WithScheduledAt(t time.Time) JobOption {
	return func(j *Job) {
		j.ScheduledAt = t
	}
}

// WithTimeout sets the maximum execution time.
func WithTimeout(timeout time.Duration) JobOption {
	return func(j *Job) {
		j.Timeout = timeout
	}
}

// WithMaxAttempts sets the maximum retry attempts.
func WithMaxAttempts(attempts int) JobOption {
	return func(j *Job) {
		j.MaxAttempts = attempts
	}
}

// WithBackoff configures the retry backoff strategy.
func WithBackoff(strategy BackoffStrategy, initial, max time.Duration, multiplier float64) JobOption {
	return func(j *Job) {
		j.BackoffStrategy = strategy
		j.BackoffInitial = initial
		j.BackoffMax = max
		j.BackoffMultiplier = multiplier
	}
}

// WithTags sets the job tags.
func WithTags(tags map[string]string) JobOption {
	return func(j *Job) {
		j.Tags = tags
	}
}

// WithTag adds a single tag.
func WithTag(key, value string) JobOption {
	return func(j *Job) {
		if j.Tags == nil {
			j.Tags = make(map[string]string)
		}
		j.Tags[key] = value
	}
}

// WithDependencies sets job dependencies.
func WithDependencies(deps ...uuid.UUID) JobOption {
	return func(j *Job) {
		j.DependsOn = deps
	}
}

// WithMetadata sets job metadata.
func WithMetadata(metadata map[string]interface{}) JobOption {
	return func(j *Job) {
		j.Metadata = metadata
	}
}

// ServerConfig holds the main server configuration.
type ServerConfig struct {
	// HTTP server configuration
	HTTPAddr string `json:"http_addr"`

	// Data directory for persistence
	DataDir string `json:"data_dir"`

	// Worker configuration
	Workers           int           `json:"workers"`
	ShutdownTimeout   time.Duration `json:"shutdown_timeout"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`

	// Scheduler configuration
	SchedulerTickInterval time.Duration `json:"scheduler_tick_interval"`
	VisibilityTimeout     time.Duration `json:"visibility_timeout"`

	// Cleanup configuration
	RetentionPeriod time.Duration `json:"retention_period"`
	CleanupInterval time.Duration `json:"cleanup_interval"`

	// Observability
	MetricsEnabled bool   `json:"metrics_enabled"`
	MetricsAddr    string `json:"metrics_addr"`

	// Security
	APIKeyEnabled bool   `json:"api_key_enabled"`
	APIKey        string `json:"api_key"`
}

// DefaultServerConfig returns sensible defaults.
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		HTTPAddr:              ":8080",
		DataDir:               "./data",
		Workers:               10,
		ShutdownTimeout:       30 * time.Second,
		HeartbeatInterval:     30 * time.Second,
		SchedulerTickInterval: 100 * time.Millisecond,
		VisibilityTimeout:     5 * time.Minute,
		RetentionPeriod:       7 * 24 * time.Hour,
		CleanupInterval:       1 * time.Hour,
		MetricsEnabled:        true,
		MetricsAddr:           ":9090",
	}
}
