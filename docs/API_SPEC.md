# GopherQueue External Developer API Specification

**Version:** 1.0  
**Status:** Interface Specification

---

## Overview

This document defines the external interface that application developers use to interact with GopherQueue. It describes the **shape of data**, not implementation details.

---

## 1. Job Submission Model

### Submit Single Job

**Operation:** Submit a single job for processing

**Input Shape:**

```
JobSubmission {
    // Required
    type:           string          // Job type identifier (handler mapping)
    payload:        object | bytes  // Job-specific data

    // Optional - Scheduling
    priority:       enum            // CRITICAL | HIGH | NORMAL | LOW | BULK
    delay:          duration        // Delay before execution starts
    scheduled_at:   timestamp       // Absolute time to start execution

    // Optional - Execution
    timeout:        duration        // Max execution time
    max_attempts:   integer         // Max execution attempts (1-100)
    backoff:        BackoffConfig   // Retry delay configuration

    // Optional - Correlation
    idempotency_key: string         // Deduplication key
    correlation_id:  string         // Tracing correlation
    tags:            map<string>    // User-defined labels

    // Optional - Dependencies
    depends_on:      []JobID        // Jobs that must complete first
    dependency_type: enum           // AFTER | AFTER_ANY | CHAIN

    // Optional - Notification
    callback_url:    string         // Webhook on completion
}

BackoffConfig {
    strategy:       enum            // CONSTANT | LINEAR | EXPONENTIAL | CUSTOM
    initial_delay:  duration        // First retry delay
    max_delay:      duration        // Maximum retry delay
    multiplier:     float           // For exponential backoff
}
```

**Output Shape:**

```
SubmissionResult {
    job_id:         JobID           // Assigned job identifier
    status:         enum            // ACCEPTED | DUPLICATE | REJECTED
    state:          enum            // Current job state
    created_at:     timestamp       // When job was created
    scheduled_for:  timestamp       // When job will be eligible for execution

    // Present if duplicate
    existing_job_id: JobID          // ID of existing job (for DUPLICATE status)
}
```

**Error Shapes:**

```
ValidationError {
    field:          string          // Field that failed validation
    message:        string          // Human-readable error message
    code:           string          // Machine-readable error code
}

QuotaError {
    limit:          string          // Which limit was exceeded
    current:        integer         // Current usage
    maximum:        integer         // Maximum allowed
    retry_after:    duration        // When to retry
}
```

---

### Submit Job Batch

**Operation:** Submit multiple jobs atomically

**Input Shape:**

```
BatchSubmission {
    jobs:           []JobSubmission // 1-1000 jobs
    atomic:         boolean         // All-or-nothing semantics
}
```

**Output Shape:**

```
BatchResult {
    total:          integer         // Jobs submitted
    accepted:       integer         // Jobs accepted
    rejected:       integer         // Jobs rejected
    results:        []SubmissionResult // Per-job results
}
```

---

## 2. Job Status Query Model

### Get Single Job Status

**Operation:** Retrieve current status of a specific job

**Input Shape:**

```
JobQuery {
    job_id:         JobID           // Required: Job identifier
    include_result: boolean         // Include execution result if complete
    include_history: boolean        // Include state transition history
}
```

**Output Shape:**

```
JobStatus {
    // Identity
    job_id:         JobID
    type:           string

    // Current State
    state:          enum            // Current lifecycle state
    attempt:        integer         // Current attempt number

    // Timing
    created_at:     timestamp
    updated_at:     timestamp
    scheduled_for:  timestamp       // Null if not scheduled
    started_at:     timestamp       // Null if never started
    completed_at:   timestamp       // Null if not complete

    // Progress (if supported by handler)
    progress:       ProgressInfo    // Optional progress details

    // Result (if include_result and completed)
    result:         JobResult       // Optional result data

    // History (if include_history)
    history:        []StateTransition // Optional state history

    // Metadata
    priority:       enum
    tags:           map<string>
    correlation_id: string
}

ProgressInfo {
    percent:        integer         // 0-100
    message:        string          // Current activity
    updated_at:     timestamp       // When progress was reported
}

JobResult {
    status:         enum            // SUCCESS | FAILURE
    output:         object | bytes  // Handler return value
    error:          ErrorInfo       // Present if FAILURE
    duration_ms:    integer         // Execution time
    attempt:        integer         // Which attempt produced result
}

ErrorInfo {
    category:       enum            // TEMPORARY | PERMANENT | TIMEOUT | SYSTEM
    code:           string          // Error code
    message:        string          // Error message
    stack_trace:    string          // Optional stack trace
}

StateTransition {
    from_state:     enum
    to_state:       enum
    timestamp:      timestamp
    reason:         string          // Why transition occurred
}
```

---

### Query Multiple Jobs

**Operation:** Search and filter jobs

**Input Shape:**

```
JobSearch {
    // Filters (all optional, combined with AND)
    types:          []string        // Job types to include
    states:         []enum          // States to include
    priorities:     []enum          // Priorities to include
    tags:           map<string>     // Tags to match (exact)

    // Time filters
    created_after:  timestamp
    created_before: timestamp
    updated_after:  timestamp

    // Pagination
    cursor:         string          // Pagination cursor
    limit:          integer         // Max results (1-1000)
    order:          enum            // CREATED_ASC | CREATED_DESC | UPDATED_DESC
}
```

**Output Shape:**

```
JobSearchResult {
    jobs:           []JobStatus     // Matching jobs
    total_count:    integer         // Total matching (may be estimate)
    next_cursor:    string          // Cursor for next page (null if last)
    has_more:       boolean
}
```

---

### Get Queue Statistics

**Operation:** Retrieve queue health and metrics

**Input Shape:**

```
StatsQuery {
    breakdown_by:   []enum          // TYPE | PRIORITY | STATE
    time_range:     duration        // History to include
}
```

**Output Shape:**

```
QueueStats {
    // Current counts
    pending:        integer         // Jobs waiting to run
    scheduled:      integer         // Jobs ready for workers
    running:        integer         // Jobs currently executing
    delayed:        integer         // Jobs waiting for delay

    // Terminal state counts
    completed:      integer         // In time range
    failed:         integer         // In time range
    dead_letter:    integer         // In time range

    // Rates
    submission_rate: float          // Jobs/second
    completion_rate: float          // Jobs/second
    failure_rate:    float          // Jobs/second

    // Latency (percentiles)
    queue_latency:   LatencyStats   // Time in queue
    execution_time:  LatencyStats   // Handler duration

    // Breakdowns (if requested)
    by_type:        map<string, Counts>
    by_priority:    map<enum, Counts>
    by_state:       map<enum, Counts>
}

LatencyStats {
    p50:            duration
    p90:            duration
    p99:            duration
    max:            duration
}

Counts {
    pending:        integer
    running:        integer
    completed:      integer
    failed:         integer
}
```

---

## 3. Job Cancellation Model

### Cancel Single Job

**Operation:** Request cancellation of a pending or running job

**Input Shape:**

```
CancellationRequest {
    job_id:         JobID           // Job to cancel
    reason:         string          // Why cancellation requested
    force:          boolean         // Interrupt running job
}
```

**Output Shape:**

```
CancellationResult {
    job_id:         JobID
    status:         enum            // CANCELLED | ALREADY_COMPLETE | NOT_FOUND | CANCEL_REQUESTED
    previous_state: enum            // State before cancellation
    message:        string          // Additional context
}
```

**Cancellation Semantics:**

- PENDING/SCHEDULED/DELAYED: Job transitions to CANCELLED immediately
- RUNNING (force=false): Cancellation noted; checked on next checkpoint
- RUNNING (force=true): Handler context cancelled; may take time to abort
- COMPLETED/FAILED/DEAD_LETTER: Cancellation fails (already terminal)

---

### Cancel Job Batch

**Operation:** Cancel multiple jobs matching criteria

**Input Shape:**

```
BatchCancellation {
    // Cancel by IDs
    job_ids:        []JobID         // Specific jobs

    // Or cancel by criteria
    filter:         JobSearch       // Jobs matching filter

    reason:         string
    force:          boolean
    dry_run:        boolean         // Preview without cancelling
}
```

**Output Shape:**

```
BatchCancellationResult {
    requested:      integer         // Jobs targeted
    cancelled:      integer         // Successfully cancelled
    skipped:        integer         // Already terminal
    failed:         integer         // Cancellation failed
    errors:         []CancellationError
}
```

---

## 4. Failure Inspection Model

### Get Job Failure Details

**Operation:** Retrieve detailed failure information for a failed job

**Input Shape:**

```
FailureQuery {
    job_id:         JobID
    include_all_attempts: boolean   // Include all failure attempts
}
```

**Output Shape:**

```
FailureDetails {
    job_id:         JobID
    type:           string
    final_state:    enum            // FAILED | DEAD_LETTER
    total_attempts: integer

    // Current/Final failure
    failure:        FailureInfo

    // All attempts (if requested)
    attempts:       []AttemptInfo
}

FailureInfo {
    attempt:        integer
    timestamp:      timestamp
    duration_ms:    integer

    category:       enum            // TEMPORARY | PERMANENT | TIMEOUT | SYSTEM
    code:           string
    message:        string

    // Detailed diagnostics
    stack_trace:    string
    context:        map<string>     // Additional context data

    // System info
    worker_id:      string          // Which worker ran this
    host:           string          // Which host
}

AttemptInfo {
    attempt:        integer
    started_at:     timestamp
    finished_at:    timestamp
    result:         enum            // SUCCESS | FAILURE | TIMEOUT | CANCELLED
    failure:        FailureInfo     // Present if result != SUCCESS
}
```

---

### List Failed Jobs

**Operation:** Query jobs in failure states for investigation

**Input Shape:**

```
FailureSearch {
    states:         []enum          // FAILED | DEAD_LETTER | RETRYING
    failure_category: []enum        // TEMPORARY | PERMANENT | TIMEOUT | SYSTEM
    types:          []string        // Job types
    error_code:     string          // Specific error code

    time_range:     TimeRange
    pagination:     Pagination
}
```

**Output Shape:**

```
FailureSearchResult {
    failures:       []FailureDetails
    total_count:    integer
    next_cursor:    string

    // Aggregations
    by_category:    map<enum, integer>
    by_code:        map<string, integer>
    by_type:        map<string, integer>
}
```

---

### Retry Failed Job

**Operation:** Manually retry a failed or dead-lettered job

**Input Shape:**

```
RetryRequest {
    job_id:         JobID
    reset_attempts: boolean         // Reset attempt counter
    priority:       enum            // Override priority for retry
    delay:          duration        // Delay before retry
}
```

**Output Shape:**

```
RetryResult {
    job_id:         JobID
    status:         enum            // QUEUED | NOT_RETRIABLE | NOT_FOUND
    new_state:      enum            // State after retry request
    scheduled_for:  timestamp
}
```

---

## 5. Handler Registration Model

### Register Job Handler

**Operation:** Register a handler function for a job type (library API)

**Input Shape:**

```
HandlerRegistration {
    type:           string          // Job type identifier
    handler:        function        // Processing function

    // Optional configurations
    timeout:        duration        // Default timeout for this type
    max_attempts:   integer         // Default retries for this type
    concurrency:    integer         // Max parallel executions
}

// Handler function signature
handler(context: JobContext, payload: T) -> Result<R, Error>
```

**Output Shape:**

```
RegistrationResult {
    type:           string
    status:         enum            // REGISTERED | REPLACED | INVALID
    previous:       boolean         // Had previous handler
}
```

---

### Handler Context Interface

The context provided to handlers during execution:

```
JobContext {
    // Identity
    job_id():       JobID
    type():         string
    attempt():      integer

    // Metadata
    created_at():   timestamp
    correlation_id(): string
    tags():         map<string>

    // Control
    deadline():     timestamp       // When execution must complete
    is_cancelled(): boolean         // Check cancellation status

    // Progress reporting
    report_progress(percent: int, message: string)

    // Checkpointing (for long-running jobs)
    checkpoint(state: bytes)
    last_checkpoint(): bytes        // State from previous attempt

    // Child job creation
    spawn(submission: JobSubmission): JobID

    // Logging
    log(level: enum, message: string, fields: map)
}
```

---

## 6. Common Data Shapes

### Time Representations

```
timestamp:      ISO 8601 string (e.g., "2024-01-15T10:30:00Z")
duration:       ISO 8601 duration or seconds integer (e.g., "PT5M" or 300)
```

### Pagination

```
Pagination {
    cursor:         string          // Opaque pagination cursor
    limit:          integer         // 1-1000, default 100
}
```

### Error Response

```
APIError {
    code:           string          // Machine-readable code
    message:        string          // Human-readable message
    details:        []ErrorDetail   // Additional context
    request_id:     string          // For support correlation
}

ErrorDetail {
    field:          string          // Related field
    code:           string          // Specific error code
    message:        string          // Detailed message
}
```

### Standard Error Codes

| Code            | HTTP Status | Meaning                        |
| --------------- | ----------- | ------------------------------ |
| NOT_FOUND       | 404         | Job does not exist             |
| INVALID_REQUEST | 400         | Request failed validation      |
| DUPLICATE       | 409         | Idempotency key exists         |
| QUOTA_EXCEEDED  | 429         | Rate or resource limit hit     |
| UNAUTHORIZED    | 401         | Missing or invalid credentials |
| FORBIDDEN       | 403         | Insufficient permissions       |
| INTERNAL_ERROR  | 500         | System error                   |

---

_This specification defines the API contract. Wire format (JSON), transport (HTTP/gRPC), and language bindings are implementation details not covered here._
