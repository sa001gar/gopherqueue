# GopherQueue High-Level Architecture

**Version:** 1.0  
**Status:** Architectural Specification

---

## Overview

GopherQueue implements a layered architecture with strict separation of concerns. Each layer has defined responsibilities, explicit inputs and outputs, and clear boundaries regarding what it must NOT handle.

```
┌─────────────────────────────────────────────────────────────────┐
│                     SECURITY BOUNDARY                           │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐      │
│  │   INGESTION  │───▶│   SCHEDULER  │───▶│    WORKER    │      │
│  │    LAYER     │    │              │    │  EXECUTION   │      │
│  └──────────────┘    └──────────────┘    └──────────────┘      │
│         │                   │                   │               │
│         ▼                   ▼                   ▼               │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                   PERSISTENCE LAYER                      │   │
│  └─────────────────────────────────────────────────────────┘   │
│         │                   │                   │               │
│         ▼                   ▼                   ▼               │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐      │
│  │   RECOVERY   │    │ OBSERVABILITY│    │   SECURITY   │      │
│  │  SUBSYSTEM   │    │  SUBSYSTEM   │    │   BOUNDARY   │      │
│  └──────────────┘    └──────────────┘    └──────────────┘      │
└─────────────────────────────────────────────────────────────────┘
```

---

## Layer 1: Ingestion Layer

### Responsibilities

1. **Accept job submissions** from external clients via defined protocols (HTTP API, library calls)
2. **Validate job specifications** against the formal job model (required fields, type constraints, value ranges)
3. **Assign job identity** (unique identifier generation)
4. **Delegate persistence** to the persistence layer before acknowledging submission
5. **Return submission response** with job identifier and initial status

### Inputs

| Input                      | Source          | Format                             |
| -------------------------- | --------------- | ---------------------------------- |
| Job specification          | External client | JSON payload or Go struct          |
| Authentication credentials | External client | Token/key in request header        |
| Submission options         | External client | Priority, delay, timeout overrides |

### Outputs

| Output                    | Destination             | Format                   |
| ------------------------- | ----------------------- | ------------------------ |
| Job identifier            | External client         | UUID string              |
| Submission acknowledgment | External client         | Success/failure response |
| Persisted job             | Persistence layer       | Structured job record    |
| Submission metrics        | Observability subsystem | Counter increments       |

### Must NOT Handle

- **Job execution logic:** Ingestion validates shape, not semantics
- **Scheduling decisions:** When a job runs is not ingestion's concern
- **Retry policy enforcement:** Ingestion records the policy; scheduler enforces it
- **Worker selection:** Ingestion does not know about workers
- **Result storage:** Ingestion handles input, not output

---

## Layer 2: Scheduler

### Responsibilities

1. **Maintain job priority queue** ordered by priority class, then submission timestamp
2. **Track job execution timing** (delays, scheduled execution times)
3. **Dispatch jobs to workers** via the dispatcher interface
4. **Handle job state transitions** (pending → running, running → completed/failed)
5. **Enforce retry policies** when jobs fail with retriable errors
6. **Dead-letter jobs** that exhaust retry attempts

### Inputs

| Input                  | Source                 | Format             |
| ---------------------- | ---------------------- | ------------------ |
| Persisted jobs         | Persistence layer      | Job records        |
| Job completion signals | Worker execution layer | Result with status |
| Retry requests         | Recovery subsystem     | Job identifiers    |
| Current time           | System clock           | Timestamp          |

### Outputs

| Output             | Destination             | Format              |
| ------------------ | ----------------------- | ------------------- |
| Dispatch requests  | Worker execution layer  | Job with context    |
| State transitions  | Persistence layer       | Job state updates   |
| Dead-lettered jobs | Persistence layer       | Failed job records  |
| Scheduling metrics | Observability subsystem | Gauges and counters |

### Must NOT Handle

- **Job validation:** Jobs arrive pre-validated from ingestion
- **Actual job execution:** The scheduler dispatches; workers execute
- **Persistence mechanics:** The scheduler requests state changes; persistence commits them
- **Authentication:** The scheduler operates on authenticated jobs only
- **Result interpretation:** Success or failure is determined by workers, not the scheduler

---

## Layer 3: Worker Execution Layer

### Responsibilities

1. **Maintain worker pool** with configurable concurrency
2. **Execute job handlers** with timeout enforcement
3. **Provide job context** to handlers (cancellation, deadline, metadata access)
4. **Capture job results** (success value, error, execution duration)
5. **Report completion** back to the scheduler
6. **Enforce resource limits** (memory, CPU time where possible)

### Inputs

| Input                | Source                  | Format                |
| -------------------- | ----------------------- | --------------------- |
| Dispatch requests    | Scheduler               | Job with context      |
| Handler registry     | Application code        | Handler function map  |
| Worker configuration | System configuration    | Concurrency, timeouts |
| Cancellation signals | Scheduler (via context) | Context cancellation  |

### Outputs

| Output                | Destination             | Format                    |
| --------------------- | ----------------------- | ------------------------- |
| Job results           | Scheduler               | Success/failure with data |
| Execution metrics     | Observability subsystem | Histograms, counters      |
| Worker health signals | Observability subsystem | Health status             |
| Resource usage data   | Observability subsystem | Resource metrics          |

### Must NOT Handle

- **Job persistence:** Workers process in-memory; persistence layer handles durability
- **Retry decisions:** Workers report failure; scheduler decides on retry
- **Scheduling:** Workers execute what they receive; they do not choose
- **Authentication:** Workers trust that dispatched jobs are authorized
- **Job validation:** Workers trust that dispatched jobs are valid

---

## Layer 4: Persistence Layer

### Responsibilities

1. **Durably store job records** with ACID guarantees (or documented weaker guarantees)
2. **Transactionally update job state** as jobs progress through lifecycle
3. **Provide job queries** by ID, state, type, time range
4. **Store job results** for completed jobs
5. **Maintain job history** for debugging and compliance
6. **Support recovery queries** (stuck jobs, orphaned jobs, failed jobs)

### Inputs

| Input             | Source               | Format                |
| ----------------- | -------------------- | --------------------- |
| Job records       | Ingestion layer      | Structured job data   |
| State transitions | Scheduler            | State change requests |
| Query requests    | All layers           | Query specifications  |
| Retention policy  | System configuration | Duration, counts      |

### Outputs

| Output                     | Destination             | Format              |
| -------------------------- | ----------------------- | ------------------- |
| Query results              | Requesting layer        | Job records         |
| Persistence acknowledgment | Requesting layer        | Success/failure     |
| Storage metrics            | Observability subsystem | Size, counts        |
| Recovery data              | Recovery subsystem      | Stuck/orphaned jobs |

### Must NOT Handle

- **Job execution:** Persistence stores; it does not compute
- **Scheduling logic:** Persistence provides data; scheduler makes decisions
- **Business validation:** Persistence enforces schema; application enforces semantics
- **Network protocols:** Persistence is local; ingestion handles external communication
- **Authentication:** Persistence trusts that callers are authorized

---

## Layer 5: Recovery Subsystem

### Responsibilities

1. **Detect stuck jobs** that exceed their execution timeout
2. **Reclaim orphaned jobs** from workers that crashed or disconnected
3. **Reset job state** for jobs requiring re-execution
4. **Trigger re-scheduling** for recovered jobs
5. **Log recovery actions** for operational visibility
6. **Prevent recovery loops** for permanently failing jobs

### Inputs

| Input                  | Source                 | Format             |
| ---------------------- | ---------------------- | ------------------ |
| Current job states     | Persistence layer      | Active job records |
| Timeout configurations | System configuration   | Duration values    |
| Current time           | System clock           | Timestamp          |
| Worker health          | Worker execution layer | Liveness status    |

### Outputs

| Output           | Destination             | Format                |
| ---------------- | ----------------------- | --------------------- |
| Recovery actions | Scheduler               | Jobs to reschedule    |
| State resets     | Persistence layer       | State change requests |
| Recovery events  | Observability subsystem | Event records         |
| Recovery metrics | Observability subsystem | Counters              |

### Must NOT Handle

- **Normal job flow:** Recovery handles exceptions, not the happy path
- **Job execution:** Recovery resets state; workers execute
- **Business logic:** Recovery is mechanical; it does not interpret job content
- **Authentication:** Recovery operates on already-authenticated jobs
- **Result determination:** Recovery does not decide success or failure

---

## Layer 6: Observability Subsystem

### Responsibilities

1. **Collect metrics** from all system layers (counters, gauges, histograms)
2. **Expose metrics endpoints** for external monitoring systems
3. **Generate structured logs** for operational debugging
4. **Track health indicators** for each subsystem
5. **Provide administrative endpoints** for introspection
6. **Maintain audit trail** for compliance-sensitive operations

### Inputs

| Input          | Source               | Format                          |
| -------------- | -------------------- | ------------------------------- |
| Metric updates | All layers           | Counter/gauge/histogram updates |
| Log events     | All layers           | Structured log entries          |
| Health signals | All layers           | Health/ready status             |
| Configuration  | System configuration | Metric retention, log levels    |

### Outputs

| Output           | Destination              | Format                |
| ---------------- | ------------------------ | --------------------- |
| Metrics endpoint | External monitoring      | Prometheus format     |
| Log stream       | External log aggregation | Structured JSON       |
| Health endpoints | External probes          | HTTP health responses |
| Admin API        | Operators                | JSON responses        |

### Must NOT Handle

- **Metric interpretation:** Observability exposes; operators interpret
- **Alerting logic:** Observability provides data; external systems alert
- **Job processing:** Observability watches; it does not participate
- **State modification:** Observability reads; it does not write (except its own state)
- **Authentication decisions:** Observability reports; security decides

---

## Layer 7: Security Boundary

### Responsibilities

1. **Authenticate clients** attempting to submit or query jobs
2. **Authorize operations** based on client identity and operation type
3. **Protect sensitive job data** through encryption or access controls
4. **Rate-limit requests** to prevent abuse
5. **Audit security-relevant events** for compliance
6. **Enforce resource quotas** per client or tenant

### Inputs

| Input                      | Source               | Format                     |
| -------------------------- | -------------------- | -------------------------- |
| Authentication credentials | External clients     | Tokens, keys, certificates |
| Authorization requests     | Ingestion layer      | Operation descriptors      |
| Request patterns           | Ingestion layer      | Request metadata           |
| Security configuration     | System configuration | Policies, limits           |

### Outputs

| Output                   | Destination             | Format          |
| ------------------------ | ----------------------- | --------------- |
| Authentication decisions | Ingestion layer         | Allow/deny      |
| Authorization decisions  | All layers              | Allow/deny      |
| Rate-limit signals       | Ingestion layer         | Throttle/reject |
| Security events          | Observability subsystem | Audit records   |

### Must NOT Handle

- **Job processing:** Security gates; it does not execute
- **Persistence:** Security authorizes; persistence stores
- **Scheduling:** Security protects boundaries; scheduler allocates resources
- **Business logic:** Security enforces policy; applications define semantics
- **Metrics collection:** Security generates events; observability collects them

---

## Inter-Layer Communication

### Synchronous Calls

Layers communicate synchronously when the caller requires an immediate response:

- Ingestion → Persistence (job storage)
- Scheduler → Persistence (state updates)
- Query endpoints → Persistence (job lookups)

### Asynchronous Signals

Layers communicate asynchronously when immediate response is unnecessary:

- Recovery → Scheduler (jobs to reschedule)
- All layers → Observability (metrics and logs)
- Workers → Scheduler (completion signals via channels)

### Dependency Rules

1. Upper layers depend on lower layers, never the reverse
2. The persistence layer has no outbound dependencies
3. The observability subsystem receives from all layers but influences none
4. The security boundary wraps external interfaces; internal layers trust each other

---

## Deployment Topology

### Single-Process Mode (Default)

All layers run within a single OS process. Communication is via function calls and Go channels. This is the recommended deployment for most use cases.

### Worker-Separation Mode (Future)

Worker execution layer runs as separate processes, communicating with the core via gRPC or similar. This enables language-agnostic workers and process isolation.

---

_This architecture document defines layer responsibilities without prescribing implementation. Implementations must honor the layer boundaries and responsibility allocations defined here._
