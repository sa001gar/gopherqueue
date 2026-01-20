# GopherQueue

[![Go Reference](https://pkg.go.dev/badge/github.com/sa001gar/gopherqueue.svg)](https://pkg.go.dev/github.com/sa001gar/gopherqueue)
[![Go Report Card](https://goreportcard.com/badge/github.com/sa001gar/gopherqueue)](https://goreportcard.com/report/github.com/sa001gar/gopherqueue)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Enterprise-grade, local-first background job engine for Go.**

GopherQueue is a production-ready background job processing system that prioritizes reliability, simplicity, and operational ease. Unlike distributed systems that require external message brokers, GopherQueue runs entirely locally with persistent storage, making it perfect for single-server deployments, edge computing, and scenarios where operational simplicity matters.

## Features

- **Durable by Default**: BoltDB-backed persistence ensures jobs survive crashes and restarts
- **Priority Queues**: Critical, High, Normal, Low, and Bulk priority levels
- **Automatic Retries**: Configurable retry strategies with exponential backoff
- **Full Observability**: Prometheus metrics, structured logging, health checks
- **Panic Recovery**: Handlers that panic are caught and the job is properly failed
- **Checkpointing**: Long-running jobs can save progress for crash recovery
- **Job Dependencies**: Jobs can wait for other jobs to complete
- **Idempotency**: Built-in deduplication via idempotency keys
- **Security Ready**: API key authentication, role-based authorization

## Quick Start

### Installation

```bash
go install github.com/sa001gar/gopherqueue/cmd/gq@latest
```

### Start the Server

```bash
# Start with default settings (10 workers, BoltDB storage)
gq serve

# Or with custom configuration
gq serve --http :8080 --workers 20 --data-dir ./my-data
```

### Submit Jobs

```bash
# Submit via CLI
gq submit --type echo --payload '{"message": "Hello World!"}'

# Check status
gq status --stats

# List jobs
gq status --state running
```

### Submit via HTTP API

```bash
# Submit a job
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "payload": {"to": "user@example.com", "subject": "Welcome!"},
    "priority": 1
  }'

# Check job status
curl http://localhost:8080/api/v1/jobs/{job-id}

# Get server statistics
curl http://localhost:8080/api/v1/stats
```

## Use as a Library

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/sa001gar/gopherqueue/core"
	"github.com/sa001gar/gopherqueue/persistence"
	"github.com/sa001gar/gopherqueue/scheduler"
	"github.com/sa001gar/gopherqueue/worker"
)

func main() {
	ctx := context.Background()

	// Initialize BoltDB store
	store, err := persistence.NewBoltStore("./data")
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	// Create scheduler
	sched := scheduler.NewPriorityScheduler(store, nil)
	sched.Start(ctx)

	// Create worker pool
	pool := worker.NewSimplePool(store, sched, &worker.WorkerConfig{
		Concurrency: 10,
	})

	// Register job handlers
	pool.RegisterHandler("email", func(ctx context.Context, jctx worker.JobContext) error {
		job := jctx.Job()
		log.Printf("Sending email: %s", string(job.Payload))
		// Do work...
		return nil
	})

	pool.Start(ctx)

	// Submit a job
	job := core.NewJob("email", []byte(`{"to": "user@example.com"}`),
		core.WithPriority(core.PriorityHigh),
		core.WithMaxAttempts(3),
	)
	store.Create(ctx, job)
	sched.Enqueue(ctx, job)

	// Keep running...
	select {}
}
```

## Core Concepts

### Job States

```
pending -> scheduled -> running -> completed
                    \          \
                     retrying -> failed -> dead_letter
                         |
                      delayed
```

### Priority Levels

| Priority | Value | Use Case                             |
| -------- | ----- | ------------------------------------ |
| Critical | 0     | System alerts, payment confirmations |
| High     | 1     | User-initiated actions               |
| Normal   | 2     | Standard background work (default)   |
| Low      | 3     | Batch processing, reports            |
| Bulk     | 4     | Data migrations, cleanup             |

### Retry Strategies

- **Exponential**: Doubles delay each attempt (default)
- **Linear**: Fixed increment each attempt
- **Constant**: Fixed delay between attempts

## Configuration

### Server Configuration

| Flag                 | Default  | Description                      |
| -------------------- | -------- | -------------------------------- |
| `--http`             | `:8080`  | HTTP server address              |
| `--workers`          | `10`     | Number of worker goroutines      |
| `--data-dir`         | `./data` | BoltDB data directory            |
| `--shutdown-timeout` | `30s`    | Graceful shutdown timeout        |
| `--bolt`             | `true`   | Use BoltDB (false for in-memory) |

### Job Options

```go
job := core.NewJob("type", payload,
    core.WithPriority(core.PriorityCritical),
    core.WithDelay(5*time.Minute),
    core.WithTimeout(30*time.Minute),
    core.WithMaxAttempts(5),
    core.WithIdempotencyKey("unique-key"),
    core.WithTags(map[string]string{"env": "prod"}),
    core.WithBackoff(core.BackoffExponential, time.Second, time.Hour, 2.0),
)
```

## Observability

### Health Endpoints

- `GET /health` - Full health status
- `GET /live` - Liveness probe (always returns 200 if server is up)
- `GET /ready` - Readiness probe (checks all components)

### Metrics

- `GET /metrics` - JSON metrics endpoint
- Jobs enqueued/completed/failed/retried
- Processing time statistics
- Queue depth
- Worker utilization

## Project Structure

```
gopherqueue/
├── api/          # HTTP API server and handlers
├── cli/          # Command-line interface
├── cmd/gq/       # Main entry point
├── core/         # Core types (Job, errors, options)
├── docs/         # Documentation
├── observability/ # Metrics and health checks
├── persistence/  # Storage (BoltDB, memory)
├── recovery/     # Stuck job detection and recovery
├── scheduler/    # Job scheduling and priority queue
├── security/     # Authentication and authorization
├── tests/        # Test documentation
└── worker/       # Job execution
```

## Contributing

Contributions are welcome! Please read our [Contributing Guide](CONTRIBUTING.md) for details.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

- [BoltDB](https://github.com/etcd-io/bbolt) - The embedded key/value database
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- Inspired by Sidekiq, Machinery, and Asynq
