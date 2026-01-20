# GopherQueue Observability Model

**Version:** 1.0  
**Status:** Operations Specification

---

## Overview

This document defines what operators must see in production to effectively monitor, troubleshoot, and maintain GopherQueue. The observability model covers metrics, logs, alerts, and health indicators.

---

## 1. Core Metrics

### Job Lifecycle Metrics

| Metric                        | Type    | Labels                   | Description                 |
| ----------------------------- | ------- | ------------------------ | --------------------------- |
| `gq_jobs_submitted_total`     | Counter | type, priority           | Total jobs submitted        |
| `gq_jobs_completed_total`     | Counter | type, priority           | Successfully completed jobs |
| `gq_jobs_failed_total`        | Counter | type, priority, category | Failed jobs by failure type |
| `gq_jobs_retried_total`       | Counter | type                     | Jobs that triggered retry   |
| `gq_jobs_dead_lettered_total` | Counter | type                     | Jobs moved to dead letter   |
| `gq_jobs_cancelled_total`     | Counter | type                     | Jobs cancelled              |

### Queue State Metrics

| Metric               | Type  | Labels                | Description                  |
| -------------------- | ----- | --------------------- | ---------------------------- |
| `gq_queue_depth`     | Gauge | state, type, priority | Current job count by state   |
| `gq_queue_pending`   | Gauge | type, priority        | Jobs waiting to be scheduled |
| `gq_queue_scheduled` | Gauge | type                  | Jobs ready for workers       |
| `gq_queue_running`   | Gauge | type                  | Jobs currently executing     |
| `gq_queue_delayed`   | Gauge | type                  | Jobs waiting for delay       |

### Latency Metrics

| Metric                          | Type      | Labels         | Description                |
| ------------------------------- | --------- | -------------- | -------------------------- |
| `gq_queue_wait_seconds`         | Histogram | type, priority | Time from submit to start  |
| `gq_execution_seconds`          | Histogram | type           | Handler execution duration |
| `gq_total_latency_seconds`      | Histogram | type           | Submit to completion time  |
| `gq_scheduling_latency_seconds` | Histogram | -              | Scheduler cycle duration   |

### Worker Metrics

| Metric                  | Type  | Labels | Description                  |
| ----------------------- | ----- | ------ | ---------------------------- |
| `gq_workers_active`     | Gauge | -      | Workers currently processing |
| `gq_workers_idle`       | Gauge | -      | Workers waiting for jobs     |
| `gq_workers_total`      | Gauge | -      | Total worker pool size       |
| `gq_worker_utilization` | Gauge | -      | Percentage of workers busy   |

### Persistence Metrics

| Metric                            | Type      | Labels           | Description           |
| --------------------------------- | --------- | ---------------- | --------------------- |
| `gq_persistence_operations_total` | Counter   | operation        | DB operations count   |
| `gq_persistence_latency_seconds`  | Histogram | operation        | DB operation duration |
| `gq_persistence_errors_total`     | Counter   | operation, error | DB errors by type     |
| `gq_storage_bytes`                | Gauge     | -                | Total storage used    |

### Recovery Metrics

| Metric                        | Type      | Labels | Description               |
| ----------------------------- | --------- | ------ | ------------------------- |
| `gq_recovery_runs_total`      | Counter   | -      | Recovery cycle executions |
| `gq_recovery_jobs_reclaimed`  | Counter   | reason | Jobs recovered            |
| `gq_recovery_latency_seconds` | Histogram | -      | Recovery cycle duration   |
| `gq_stuck_jobs_detected`      | Gauge     | -      | Currently stuck jobs      |

---

## 2. Metric Collection Strategy

### Collection Intervals

| Metric Category | Push Interval | Retention                      |
| --------------- | ------------- | ------------------------------ |
| Counters        | On event      | Aggregated                     |
| Gauges          | 10 seconds    | 15 days                        |
| Histograms      | On event      | 7 days raw, 90 days aggregated |

### Histogram Buckets

**Queue Wait Time:**

```
buckets: [0.1, 0.5, 1, 5, 10, 30, 60, 120, 300, 600, 1800, 3600]
unit: seconds
```

**Execution Time:**

```
buckets: [0.01, 0.05, 0.1, 0.5, 1, 5, 10, 30, 60, 300, 900, 1800]
unit: seconds
```

**Persistence Latency:**

```
buckets: [0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1]
unit: seconds
```

### Label Cardinality

To prevent metric explosion:

- `type`: Limited to registered job types
- `priority`: Enum with 5 values
- `state`: Enum with 8 values
- `category`: Enum with 5 values
- `operation`: Enum with defined operations

---

## 3. Structured Logging

### Log Format

All logs are emitted as structured JSON:

```json
{
  "timestamp": "2024-01-15T10:30:00.123Z",
  "level": "INFO",
  "message": "Job completed successfully",
  "service": "gopherqueue",
  "component": "worker",
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "job_type": "email.send",
  "correlation_id": "req-12345",
  "duration_ms": 150,
  "attempt": 1
}
```

### Log Levels

| Level | Usage                                                          |
| ----- | -------------------------------------------------------------- |
| DEBUG | Detailed execution traces, development only                    |
| INFO  | Normal operations (job start, complete, state changes)         |
| WARN  | Recoverable issues (retry triggered, timeout, degraded)        |
| ERROR | Failures requiring attention (permanent failure, system error) |
| FATAL | Unrecoverable errors causing shutdown                          |

### Standard Log Fields

All log entries include:

| Field      | Description                                    |
| ---------- | ---------------------------------------------- |
| timestamp  | ISO 8601 with milliseconds                     |
| level      | Log severity                                   |
| message    | Human-readable description                     |
| service    | Always "gopherqueue"                           |
| component  | Subsystem (ingestion, scheduler, worker, etc.) |
| request_id | Request correlation (if applicable)            |

### Context Fields

Job-related logs include:

| Field          | Description        |
| -------------- | ------------------ |
| job_id         | Job identifier     |
| job_type       | Job type           |
| correlation_id | Client correlation |
| attempt        | Attempt number     |
| priority       | Job priority       |

---

## 4. Health Indicators

### Liveness Probe

**Endpoint:** `GET /health/live`

**Purpose:** Is the process running and responsive?

**Response:**

```json
{
  "status": "ok",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

**Failure Conditions:**

- Process unresponsive
- Critical goroutine deadlock
- Memory exhaustion

---

### Readiness Probe

**Endpoint:** `GET /health/ready`

**Purpose:** Can the service accept traffic?

**Response (Healthy):**

```json
{
  "status": "ready",
  "timestamp": "2024-01-15T10:30:00Z",
  "checks": {
    "persistence": "ok",
    "scheduler": "ok",
    "workers": "ok"
  }
}
```

**Response (Not Ready):**

```json
{
  "status": "not_ready",
  "timestamp": "2024-01-15T10:30:00Z",
  "checks": {
    "persistence": "ok",
    "scheduler": "ok",
    "workers": "not_ready"
  },
  "reason": "Worker pool not initialized"
}
```

**Failure Conditions:**

- Persistence layer unreachable
- Scheduler not running
- Worker pool not initialized

---

### Detailed Health Check

**Endpoint:** `GET /health/detailed`

**Purpose:** Comprehensive system status for operators

**Response:**

```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "version": "1.2.3",
  "uptime_seconds": 86400,
  "subsystems": {
    "ingestion": {
      "status": "healthy",
      "requests_per_second": 45.2,
      "error_rate": 0.001
    },
    "scheduler": {
      "status": "healthy",
      "cycle_latency_ms": 12,
      "pending_jobs": 1523
    },
    "workers": {
      "status": "healthy",
      "active": 8,
      "idle": 2,
      "total": 10,
      "utilization": 0.8
    },
    "persistence": {
      "status": "healthy",
      "latency_p99_ms": 5,
      "storage_used_bytes": 1073741824
    },
    "recovery": {
      "status": "healthy",
      "last_run": "2024-01-15T10:29:00Z",
      "stuck_jobs": 0
    }
  }
}
```

---

## 5. Alert Definitions

### Critical Alerts (Page Immediately)

| Alert            | Condition                                                                  | Duration | Action                   |
| ---------------- | -------------------------------------------------------------------------- | -------- | ------------------------ |
| QueueNotDraining | `rate(gq_jobs_completed_total[5m]) == 0 AND gq_queue_pending > 0`          | 5m       | Check workers, scheduler |
| PersistenceDown  | `gq_persistence_errors_total rate > 0.5/s`                                 | 1m       | Check database           |
| NoWorkersActive  | `gq_workers_active == 0 AND gq_queue_scheduled > 0`                        | 2m       | Check worker health      |
| HighFailureRate  | `rate(gq_jobs_failed_total[5m]) / rate(gq_jobs_completed_total[5m]) > 0.5` | 5m       | Investigate failures     |
| DeadLetterSpike  | `rate(gq_jobs_dead_lettered_total[10m]) > normal_rate * 10`                | 10m      | Check handlers           |

### Warning Alerts (Investigate Soon)

| Alert                    | Condition                                                                     | Duration | Action           |
| ------------------------ | ----------------------------------------------------------------------------- | -------- | ---------------- |
| QueueBacklogGrowing      | `delta(gq_queue_pending[30m]) > threshold`                                    | 30m      | Scale workers    |
| HighQueueLatency         | `histogram_quantile(0.99, gq_queue_wait_seconds) > 60`                        | 10m      | Check capacity   |
| WorkerUtilizationHigh    | `gq_worker_utilization > 0.9`                                                 | 15m      | Consider scaling |
| RecoveryFindingStuckJobs | `gq_stuck_jobs_detected > 0`                                                  | 5m       | Check for issues |
| HighRetryRate            | `rate(gq_jobs_retried_total[10m]) / rate(gq_jobs_submitted_total[10m]) > 0.2` | 10m      | Check handlers   |

### Informational Alerts (Review Daily)

| Alert                | Condition                                 | Duration | Action              |
| -------------------- | ----------------------------------------- | -------- | ------------------- |
| StorageGrowth        | `delta(gq_storage_bytes[24h]) > expected` | 24h      | Plan capacity       |
| UnusualJobTypes      | New job type detected                     | -        | Verify expected     |
| LowWorkerUtilization | `gq_worker_utilization < 0.1`             | 1h       | Consider downsizing |

---

## 6. Failure Categories

### Job Failure Categories

| Category  | Description                     | Metric Label | Typical Response    |
| --------- | ------------------------------- | ------------ | ------------------- |
| TEMPORARY | Transient error, worth retrying | `temporary`  | Automatic retry     |
| PERMANENT | Deterministic failure           | `permanent`  | Dead letter         |
| TIMEOUT   | Execution exceeded limit        | `timeout`    | Investigate handler |
| CANCELLED | Explicit cancellation           | `cancelled`  | Normal              |
| SYSTEM    | Infrastructure failure          | `system`     | Investigate infra   |

### Failure Analysis Queries

**Top failing job types:**

```promql
topk(10, sum by (type) (rate(gq_jobs_failed_total[1h])))
```

**Failure rate by category:**

```promql
sum by (category) (rate(gq_jobs_failed_total[1h]))
```

**Types with highest retry rate:**

```promql
topk(10,
    sum by (type) (rate(gq_jobs_retried_total[1h]))
    /
    sum by (type) (rate(gq_jobs_submitted_total[1h]))
)
```

---

## 7. Dashboards

### Executive Dashboard

High-level system health at a glance:

| Panel              | Visualization | Metric                                          |
| ------------------ | ------------- | ----------------------------------------------- |
| Jobs Processed     | Big Number    | `sum(rate(gq_jobs_completed_total[1h])) * 3600` |
| Success Rate       | Gauge         | `1 - (rate(failed) / rate(completed))`          |
| Queue Depth        | Graph         | `gq_queue_depth` by state                       |
| Worker Utilization | Gauge         | `gq_worker_utilization`                         |
| P99 Latency        | Graph         | `histogram_quantile(0.99, ...)`                 |

### Operations Dashboard

Detailed operational metrics:

| Panel                        | Visualization                     |
| ---------------------------- | --------------------------------- |
| Jobs by State                | Stacked area graph                |
| Submission Rate              | Line graph with anomaly detection |
| Failure Rate by Type         | Heatmap                           |
| Queue Wait Time Distribution | Histogram                         |
| Worker Pool Status           | Gauge per worker                  |
| Recovery Activity            | Event log                         |

### Debugging Dashboard

For troubleshooting specific issues:

| Panel                | Purpose                           |
| -------------------- | --------------------------------- |
| Individual Job Trace | Track job through states          |
| Slow Jobs            | Jobs exceeding P99 execution time |
| Error Patterns       | Common error messages             |
| Worker Assignment    | Which workers ran which jobs      |

---

## 8. Metrics Endpoint

### Prometheus Exposition

**Endpoint:** `GET /metrics`

**Format:** Prometheus text exposition format

**Example Output:**

```prometheus
# HELP gq_jobs_submitted_total Total number of jobs submitted
# TYPE gq_jobs_submitted_total counter
gq_jobs_submitted_total{type="email.send",priority="normal"} 15234
gq_jobs_submitted_total{type="report.generate",priority="low"} 892

# HELP gq_queue_depth Current number of jobs in queue
# TYPE gq_queue_depth gauge
gq_queue_depth{state="pending",type="email.send",priority="normal"} 45
gq_queue_depth{state="running",type="email.send",priority="normal"} 10

# HELP gq_execution_seconds Job execution duration
# TYPE gq_execution_seconds histogram
gq_execution_seconds_bucket{type="email.send",le="0.1"} 5000
gq_execution_seconds_bucket{type="email.send",le="0.5"} 12000
...
```

---

_This observability model provides the visibility operators need. Implementations should expose all defined metrics and support the specified endpoints._
