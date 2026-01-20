# GopherQueue Formal Job Model Specification

**Version:** 1.0  
**Status:** Normative Specification

---

## Overview

This document defines the formal model for a "Job" within GopherQueue. These definitions are language-agnostic and form the contract that all implementations must honor.

---

## 1. Job Identity

### Identity Definition

A Job is uniquely identified by a **Job ID**—an immutable, globally unique identifier assigned at submission time.

| Property     | Specification                                          |
| ------------ | ------------------------------------------------------ |
| Format       | UUID v4 (128-bit, RFC 4122 compliant)                  |
| Assignment   | System-assigned at submission; client cannot specify   |
| Immutability | Never changes for the lifetime of the job record       |
| Uniqueness   | Globally unique across all jobs, past and present      |
| Persistence  | Survives system restart, retained per retention policy |

### Idempotency Key (Optional)

Clients may provide an **Idempotency Key** to prevent duplicate job creation.

| Property   | Specification                                                                 |
| ---------- | ----------------------------------------------------------------------------- |
| Format     | Arbitrary string, max 256 characters                                          |
| Assignment | Client-provided at submission                                                 |
| Scope      | Unique per job type within a configurable time window                         |
| Behavior   | Duplicate key submission returns existing job ID                              |
| Expiration | Idempotency guarantee expires after configurable duration (default: 24 hours) |

---

## 2. Lifecycle States

A Job transitions through a defined set of states. Transitions are unidirectional; there is no backward movement through states.

```
                    ┌──────────────┐
                    │   PENDING    │
                    └──────┬───────┘
                           │
              ┌────────────┼────────────┐
              ▼            │            ▼
       ┌──────────┐        │     ┌──────────────┐
       │ SCHEDULED│        │     │   DELAYED    │
       └────┬─────┘        │     └──────┬───────┘
            │              │            │
            └──────────────┼────────────┘
                           ▼
                    ┌──────────────┐
                    │   RUNNING    │◀──────────┐
                    └──────┬───────┘           │
                           │                   │
         ┌─────────────────┼─────────────────┐ │
         ▼                 ▼                 ▼ │
  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
  │  COMPLETED   │  │    FAILED    │  │   RETRYING   │
  └──────────────┘  └──────┬───────┘  └──────────────┘
                           │
                           ▼
                    ┌──────────────┐
                    │ DEAD_LETTER  │
                    └──────────────┘
```

### State Definitions

| State       | Definition                                                          |
| ----------- | ------------------------------------------------------------------- |
| PENDING     | Job accepted and persisted; awaiting scheduling                     |
| SCHEDULED   | Job queued for immediate execution when worker available            |
| DELAYED     | Job accepted but execution deferred until specified time            |
| RUNNING     | Job dispatched to worker; execution in progress                     |
| COMPLETED   | Job executed successfully; result stored                            |
| FAILED      | Job execution failed; no retries remaining or permanent failure     |
| RETRYING    | Job execution failed; scheduled for retry after backoff             |
| DEAD_LETTER | Job exhausted retries without success; requires manual intervention |

### Transition Rules

| From      | To          | Trigger                                                 |
| --------- | ----------- | ------------------------------------------------------- |
| PENDING   | SCHEDULED   | Immediate execution requested, no delay                 |
| PENDING   | DELAYED     | Delay specified; waiting for trigger time               |
| DELAYED   | SCHEDULED   | Delay expired                                           |
| SCHEDULED | RUNNING     | Worker accepts job                                      |
| RUNNING   | COMPLETED   | Handler returns success                                 |
| RUNNING   | FAILED      | Handler returns permanent failure                       |
| RUNNING   | RETRYING    | Handler returns temporary failure, retries remaining    |
| RUNNING   | DEAD_LETTER | Handler returns temporary failure, no retries remaining |
| RETRYING  | SCHEDULED   | Backoff expired                                         |
| RETRYING  | DEAD_LETTER | Retry limit exhausted                                   |

---

## 3. Priority Semantics

### Priority Levels

Jobs are assigned a priority level that determines scheduling order.

| Level | Name     | Use Case                                           |
| ----- | -------- | -------------------------------------------------- |
| 0     | CRITICAL | System-level jobs that preempt all others          |
| 1     | HIGH     | User-facing operations requiring prompt execution  |
| 2     | NORMAL   | Standard background processing (default)           |
| 3     | LOW      | Deferred work that can wait indefinitely           |
| 4     | BULK     | Batch processing that should not impact other work |

### Scheduling Rules

1. **Priority ordering:** Higher priority jobs (lower number) are scheduled before lower priority jobs
2. **FIFO within priority:** Jobs with equal priority are scheduled in submission order
3. **No starvation guarantee:** Low-priority jobs will execute when higher-priority queues are empty
4. **No preemption:** A running job is never interrupted for a higher-priority job
5. **Priority inheritance:** Dependent jobs inherit the maximum priority of their dependencies

---

## 4. Timeout Rules

### Timeout Types

| Timeout            | Definition                                          | Default    |
| ------------------ | --------------------------------------------------- | ---------- |
| Execution Timeout  | Maximum wall-clock time for handler execution       | 30 minutes |
| Visibility Timeout | Time job is invisible after dispatch before reclaim | 5 minutes  |
| Submission Timeout | Maximum time for submission acknowledgment          | 30 seconds |
| Total Lifetime     | Maximum time from submission to terminal state      | 7 days     |

### Timeout Behavior

1. **Execution timeout exceeded:** Job is marked as failed with timeout error; triggers retry if configured
2. **Visibility timeout exceeded:** Job is reclaimed and rescheduled (orphan recovery)
3. **Total lifetime exceeded:** Job is forcibly moved to DEAD_LETTER regardless of state
4. **Handler-specified timeout:** Handler may request shorter timeout than system default; cannot exceed it

### Timeout Configuration

```
TimeoutConfig {
    execution:   duration    // Handler execution limit
    visibility:  duration    // Visibility window before reclaim
    lifetime:    duration    // Total job lifespan
}
```

---

## 5. Retry Rules

### Retry Configuration

| Parameter        | Definition                                   | Default     |
| ---------------- | -------------------------------------------- | ----------- |
| Max Attempts     | Maximum execution attempts including initial | 3           |
| Backoff Strategy | Algorithm for computing retry delay          | Exponential |
| Initial Delay    | Delay before first retry                     | 1 second    |
| Max Delay        | Maximum delay between retries                | 1 hour      |
| Multiplier       | Factor for exponential backoff               | 2.0         |

### Backoff Strategies

**Constant:** Every retry waits the same duration

```
delay(n) = initial_delay
```

**Linear:** Delay increases linearly with attempt number

```
delay(n) = initial_delay * n
capped at max_delay
```

**Exponential:** Delay doubles (or multiplies) with each attempt

```
delay(n) = initial_delay * (multiplier ^ (n - 1))
capped at max_delay
```

**Custom:** Application-provided function computes delay

```
delay(n) = custom_function(n, initial_delay, max_delay)
```

### Retry Behavior

1. **Attempt counter:** Incremented on each execution start, not completion
2. **Retry on timeout:** Execution timeout is treated as temporary failure
3. **Partial progress:** Application may checkpoint progress; restart from checkpoint on retry
4. **Backoff jitter:** Optional random jitter (±10%) prevents thundering herd

---

## 6. Dependency Rules

### Dependency Types

| Type      | Definition                                                       |
| --------- | ---------------------------------------------------------------- |
| AFTER     | Job executes only after all dependencies complete successfully   |
| AFTER_ANY | Job executes after any dependency completes (success or failure) |
| CHAIN     | Sequential execution; failure stops the chain                    |
| FAN_OUT   | Single job spawns multiple parallel jobs                         |
| FAN_IN    | Job waits for all spawned jobs to complete                       |

### Dependency Constraints

1. **Acyclic:** Dependency graph must be acyclic; cycles are rejected at submission
2. **Bounded depth:** Maximum dependency chain depth: 100 levels
3. **Bounded breadth:** Maximum dependencies per job: 1000
4. **Resolved references:** All dependency job IDs must exist at submission time
5. **Cross-priority:** Dependencies may cross priority levels; child inherits max priority

### Dependency State Propagation

| Dependency State | Effect on Dependent                             |
| ---------------- | ----------------------------------------------- |
| COMPLETED        | Dependent may proceed (for AFTER type)          |
| FAILED           | Dependent moves to FAILED (for AFTER type)      |
| DEAD_LETTER      | Dependent moves to DEAD_LETTER (for AFTER type) |
| CANCELLED        | Dependent moves to CANCELLED (for AFTER type)   |

---

## 7. Idempotency Expectations

### System vs. Application Idempotency

GopherQueue provides **at-least-once delivery**, not exactly-once. This means:

- The system guarantees delivery but may deliver more than once
- Applications requiring exactly-once semantics must implement idempotency

### Idempotency Primitives

The system provides these primitives to support application-level idempotency:

| Primitive           | Purpose                                          |
| ------------------- | ------------------------------------------------ |
| Job ID              | Stable identifier for deduplication lookups      |
| Idempotency Key     | Client-provided key for submission deduplication |
| Attempt Number      | Distinguishes first run from retries             |
| Previous Attempt ID | Links retry attempts for correlation             |

### Idempotency Patterns

**Database Unique Constraint:**

```
// Application ensures single processing via database
INSERT INTO processed_jobs (job_id, result) VALUES (?, ?)
ON CONFLICT DO NOTHING
```

**Distributed Lock:**

```
// Application acquires lock before processing
lock = acquire_lock(job.idempotency_key)
if (lock.acquired) process(job)
```

**Version Check:**

```
// Application compares expected vs. actual state
if (entity.version == job.expected_version) {
    process(job)
    entity.version++
}
```

---

## 8. Failure Categories

### Failure Classification

All job failures are classified into exactly one category.

| Category  | Retry        | Definition                                       |
| --------- | ------------ | ------------------------------------------------ |
| TEMPORARY | Yes          | Transient error; retry may succeed               |
| PERMANENT | No           | Deterministic error; retry will fail identically |
| TIMEOUT   | Configurable | Execution exceeded time limit                    |
| CANCELLED | No           | Job was explicitly cancelled                     |
| SYSTEM    | Configurable | System failure (resource exhaustion, crash)      |

### Temporary Failures (Retriable)

- Network timeout connecting to external service
- External service returns 503 (Service Unavailable)
- Rate limit exceeded
- Temporary resource unavailability
- Deadlock detected

### Permanent Failures (Not Retriable)

- Invalid job payload
- Referenced entity does not exist
- Permission denied (will not change on retry)
- Business rule violation
- External service returns 400 (Bad Request)
- Validation failure

### Failure Response Contract

Handlers must return a typed error that the system can classify:

```
TemporaryError {
    message: string
    cause: error (optional)
    retry_after: duration (optional)
}

PermanentError {
    message: string
    cause: error (optional)
    code: string
}
```

---

## 9. Job Payload

### Payload Constraints

| Constraint    | Specification                                  |
| ------------- | ---------------------------------------------- |
| Format        | JSON-serializable (recommended) or binary      |
| Max size      | 1 MB (default), configurable to 16 MB          |
| Character set | UTF-8 for string payloads                      |
| Schema        | Application-defined; system does not interpret |

### Payload Immutability

The job payload is immutable after submission. To modify payload, submit a new job and cancel the original.

### Sensitive Data Handling

Payloads may contain sensitive data. Applications should:

1. Encrypt sensitive fields before submission
2. Reference sensitive data by ID rather than inline
3. Consider payload content when configuring log verbosity

---

## 10. Job Metadata

### System Metadata (Read-Only)

| Field        | Type      | Description                        |
| ------------ | --------- | ---------------------------------- |
| id           | UUID      | Job identifier                     |
| created_at   | timestamp | Submission time                    |
| updated_at   | timestamp | Last state change time             |
| state        | enum      | Current lifecycle state            |
| attempt      | integer   | Current attempt number (1-indexed) |
| scheduled_at | timestamp | When job became scheduled          |
| started_at   | timestamp | When current attempt started       |
| completed_at | timestamp | When job reached terminal state    |

### User Metadata (Read-Write)

| Field          | Type   | Description                         |
| -------------- | ------ | ----------------------------------- |
| type           | string | Job type identifier (required)      |
| priority       | enum   | Priority level                      |
| tags           | map    | User-defined key-value labels       |
| correlation_id | string | Trace correlation identifier        |
| callback_url   | string | Webhook for completion notification |

---

_This specification defines the formal model. Implementations must conform to these definitions. Extensions are permitted only where explicitly allowed._
