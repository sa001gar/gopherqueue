# GopherQueue Failure Playbook

**Version:** 1.0  
**Status:** Operations Runbook

---

## Overview

This playbook documents realistic failure scenarios and the expected system responses. Use this as a reference for understanding system behavior and for operational troubleshooting.

---

## Failure Scenario 1: Worker Crash Mid-Job

### Scenario

A worker process crashes (segfault, OOM kill, panic) while executing a job.

### Observable Symptoms

- `gq_workers_active` drops suddenly
- Job remains in RUNNING state past visibility timeout
- `gq_stuck_jobs_detected` increments
- Logs: Worker heartbeat stops

### System Response

1. **Detection:** Recovery subsystem detects job exceeded visibility timeout (default: 5 minutes)
2. **Classification:** Job marked as potentially orphaned
3. **Recovery:** Job state reset to SCHEDULED, attempt counter incremented
4. **Rescheduling:** Scheduler dispatches job to healthy worker
5. **Logging:** Recovery event logged with original worker ID

### Operator Action Required

- **Immediate:** None - system self-heals
- **Follow-up:** Investigate crash root cause via worker logs/coredump
- **Prevention:** Add memory limits, improve handler error handling

### Data Integrity Impact

- **Job payload:** Safe (persisted before dispatch)
- **Partial progress:** Lost unless handler checkpointed
- **In-memory state:** Lost

---

## Failure Scenario 2: Power Loss / Unclean Shutdown

### Scenario

The entire server loses power or the process receives SIGKILL without graceful shutdown.

### Observable Symptoms

- Upon restart: `gq_recovery_jobs_reclaimed` shows elevated count
- Logs: "Recovery completed, N jobs reclaimed"
- Metrics gap during outage

### System Response

1. **On restart:** Recovery subsystem scans for incomplete jobs
2. **Identification:** Jobs in RUNNING state from previous instance identified
3. **Reset:** All interrupted jobs reset to SCHEDULED
4. **Verification:** Persistence layer verifies data integrity
5. **Resume:** Normal operation continues

### Operator Action Required

- **Immediate:** Verify system restarted successfully
- **Follow-up:** Review recovered job count for anomalies
- **Prevention:** Configure UPS, enable graceful shutdown signals

### Data Integrity Impact

- **Committed jobs:** Safe (fsync before acknowledgment)
- **In-flight writes:** May be lost if not fsynced
- **Currently running jobs:** Will be retried

---

## Failure Scenario 3: Corrupt Data / Storage Failure

### Scenario

Persistence layer returns corrupted data or reports storage errors.

### Observable Symptoms

- `gq_persistence_errors_total` spikes
- Jobs fail with SYSTEM error category
- Readiness probe returns not ready
- Logs: Persistence error messages

### System Response

1. **Detection:** Persistence layer reports errors
2. **Circuit breaker:** After N consecutive failures, stop accepting submissions
3. **Alerting:** Critical alert fired
4. **Degradation:** Queue operations paused
5. **Health:** Readiness probe fails

### Operator Action Required

- **Immediate:** Investigate storage subsystem
- **Diagnosis:** Check disk health, filesystem, database state
- **Recovery:** Restore from backup if needed
- **Restart:** Once storage healthy, restart services

### Data Integrity Impact

- **Corrupted records:** May be unrecoverable
- **Recent submissions:** Acknowledged jobs are durable
- **Historical data:** Depends on corruption extent

---

## Failure Scenario 4: Stuck Job (Handler Hangs)

### Scenario

A job handler enters an infinite loop, deadlock, or waits indefinitely on external resource.

### Observable Symptoms

- Single job in RUNNING state for extended duration
- `gq_execution_seconds` shows outlier
- Other jobs process normally
- Worker count decreases (if max concurrency reached)

### System Response

1. **Detection:** Job exceeds execution timeout (default: 30 minutes)
2. **Timeout:** Handler context cancelled
3. **Classification:** Job marked as TIMEOUT failure
4. **Retry decision:** If retries remaining and not permanent, retry with backoff
5. **Worker recovery:** Worker slot freed for other jobs

### Operator Action Required

- **Immediate:** None for system health
- **Investigation:** Identify which job type hangs
- **Fix:** Add timeouts to external calls, fix handler bugs
- **Prevention:** Set appropriate per-handler timeouts

### Data Integrity Impact

- **Job state:** Safe
- **External systems:** May have partial state changes
- **Retry safety:** Handler should be idempotent

---

## Failure Scenario 5: Job Overload (Backpressure)

### Scenario

Job submission rate exceeds processing capacity; queue grows faster than it drains.

### Observable Symptoms

- `gq_queue_pending` grows continuously
- `gq_queue_wait_seconds` increases
- `gq_worker_utilization` at 100%
- Submission still succeeds

### System Response

1. **Detection:** Queue depth exceeds warning threshold
2. **Alert:** Warning alert for queue backlog
3. **Continued acceptance:** Jobs still accepted (until hard limit)
4. **Priority enforcement:** Higher priority jobs processed first
5. **Metrics:** Latency metrics accurately reflect backlog

### Operator Action Required

- **Immediate:** Assess if temporary spike or sustained
- **Scaling:** Increase worker count if possible
- **Throttling:** Consider rate limiting at client
- **Priority review:** Ensure priority levels appropriate
- **Capacity planning:** Plan for sustained load

### Data Integrity Impact

- **Job data:** Safe - all accepted jobs persist
- **SLAs:** May be impacted by queue latency
- **Ordering:** Maintained within priority levels

---

## Failure Scenario 6: Dependency Deadlock

### Scenario

Circular or blocked job dependencies create a situation where jobs cannot progress.

### Observable Symptoms

- Group of jobs stuck in PENDING state
- Jobs have dependencies on each other or on blocked jobs
- `gq_queue_pending` elevated for specific types
- Logs: "Dependency not satisfiable" warnings

### System Response

1. **Prevention:** Cycle detection at submission time rejects circular dependencies
2. **Blocked detection:** Monitor for dependencies that cannot resolve
3. **Timeout:** Total lifetime timeout eventually moves blocked jobs to DEAD_LETTER
4. **Logging:** Dependency chains logged for diagnosis

### Operator Action Required

- **Immediate:** Identify blocking dependency chain
- **Resolution:** Cancel or complete blocking jobs
- **Root cause:** Fix submission logic creating problematic dependencies
- **Prevention:** Review dependency usage patterns

### Data Integrity Impact

- **Job data:** Safe
- **Business logic:** May be impacted by unexecuted jobs
- **Downstream systems:** May have inconsistent state

---

## Failure Scenario 7: Restart During Execution

### Scenario

System receives SIGTERM while jobs are actively executing.

### Observable Symptoms

- Brief increase then decrease in `gq_workers_active`
- Logs: "Graceful shutdown initiated"
- Logs: "Waiting for N jobs to complete"
- Restart logs show minimal recovery needed

### System Response

1. **Signal handling:** Catch SIGTERM, begin graceful shutdown
2. **Drain:** Stop accepting new jobs
3. **Wait:** Wait for running jobs to complete (up to shutdown timeout)
4. **Interrupt:** After timeout, cancel remaining handlers
5. **State preservation:** Interrupted jobs remain in RUNNING state
6. **Restart recovery:** On restart, interrupted jobs are detected and rescheduled

### Operator Action Required

- **Immediate:** None - graceful shutdown is automatic
- **Verification:** Check restart completes successfully
- **Tuning:** Adjust shutdown timeout if handlers need more time

### Data Integrity Impact

- **Completing jobs:** Finish successfully
- **Long-running jobs:** May be interrupted
- **State transitions:** Atomic - no partial updates

---

## Failure Scenario 8: Resource Exhaustion

### Scenario

System runs out of memory, file descriptors, disk space, or other resources.

### Observable Symptoms

- Varies by resource type
- Memory: OOM kills, increased GC pressure
- Disk: Persistence errors, job submission failures
- FDs: Connection failures, accept errors

### System Response

**Memory Exhaustion:**

1. Large job payloads rejected before persistence
2. Background compaction/cleanup triggered
3. If severe: Process OOM killed, restart recovery

**Disk Exhaustion:**

1. New job submissions rejected
2. Alert critical
3. Automatic cleanup of expired data (if configured)

**FD Exhaustion:**

1. New connections rejected
2. Existing connections maintained
3. Alert critical

### Operator Action Required

- **Immediate:** Free resources
- **Disk:** Purge old data, increase capacity
- **Memory:** Restart process, reduce concurrency
- **FDs:** Check for leaks, increase limits
- **Prevention:** Set up proactive alerting before exhaustion

### Data Integrity Impact

- **Persisted data:** Safe
- **In-flight data:** May be lost if OOM killed
- **Submissions:** Failed submissions not persisted

---

## Failure Scenario 9: External Service Failure

### Scenario

A job handler depends on an external service (database, API) that becomes unavailable.

### Observable Symptoms

- Increased `gq_jobs_failed_total` with category=temporary
- Increased `gq_jobs_retried_total`
- Handler-specific error messages in logs
- Pattern correlates with specific job types

### System Response

1. **Handler returns error:** Classified as TEMPORARY
2. **Retry scheduled:** With exponential backoff
3. **Backpressure:** Retrying jobs don't block new submissions
4. **Dead letter:** After max retries exhausted
5. **Metrics:** Failure category accurately reflects type

### Operator Action Required

- **Immediate:** Investigate external service
- **Triage:** Determine if external issue or handler bug
- **Mitigation:** Consider pausing affected job type
- **Recovery:** Once external service healthy, jobs will retry
- **Retry dead letters:** After service recovery, retry DEAD_LETTER jobs

### Data Integrity Impact

- **Queue data:** Safe
- **External system:** State depends on handler idempotency
- **Retries:** Will continue until exhausted

---

## Failure Scenario 10: Malformed Job Payload

### Scenario

A job is submitted with a payload that the handler cannot process.

### Observable Symptoms

- Job fails immediately with PERMANENT category
- `gq_jobs_failed_total` with category=permanent
- No retries for this job
- Error logs show validation/parsing failures

### System Response

1. **Ingestion validation:** Catches schema violations before persistence
2. **Handler failure:** Catches semantic issues during execution
3. **Classification:** Handler returns PERMANENT error
4. **No retry:** Job moved directly to FAILED state
5. **Result stored:** Error details preserved for inspection

### Operator Action Required

- **Immediate:** None - system behaves correctly
- **Investigation:** Check what submitted malformed job
- **Fix:** Correct client submission logic
- **Retry:** After fixing payload, submit new job (old job not retriable)

### Data Integrity Impact

- **Job record:** Preserved with error
- **No side effects:** Handler should fail fast before partial processing
- **Client:** Should handle rejection and fix input

---

## Failure Scenario 11: Handler Panic

### Scenario

A job handler panics due to nil pointer, index out of bounds, or explicit panic.

### Observable Symptoms

- Job fails with SYSTEM error category
- Worker remains healthy (panic recovered)
- Stack trace in error logs
- `gq_jobs_failed_total{category="system"}` increments

### System Response

1. **Panic recovery:** Worker pool recovers panic, preventing process crash
2. **Classification:** Panic treated as SYSTEM failure
3. **Retry decision:** SYSTEM failures retry by default (configurable)
4. **Logging:** Full stack trace captured
5. **Worker continues:** Worker processes next job normally

### Operator Action Required

- **Immediate:** None - system self-heals
- **Investigation:** Review stack trace to identify bug
- **Fix:** Deploy handler fix
- **Testing:** Add test case for panic scenario

### Data Integrity Impact

- **Job data:** Safe
- **Handler state:** Any mutations before panic may persist
- **Recommendation:** Handlers should use transactions for external mutations

---

## Failure Scenario 12: Clock Skew / Time Jump

### Scenario

System clock changes significantly (NTP correction, daylight saving, manual adjustment).

### Observable Symptoms

- Jobs with scheduled times behave unexpectedly
- Delay calculations incorrect
- Timeout enforcement irregular
- Metrics with timestamp gaps or overlaps

### System Response

1. **Monotonic clock:** Use monotonic clock for durations (timeouts, delays)
2. **Wall clock:** Use wall clock only for scheduled times
3. **Tolerance:** Small skews handled gracefully
4. **Large jumps:** May trigger unexpected timeouts or scheduling

### Operator Action Required

- **Prevention:** Use NTP with gradual slew, not step
- **Monitoring:** Alert on clock sync issues
- **Recovery:** Restart services after large time jumps
- **Review:** Check jobs scheduled during clock anomaly

### Data Integrity Impact

- **Job data:** Safe
- **Timing:** May be inaccurate during skew
- **Ordering:** Within priority maintained by sequence numbers, not timestamps

---

## Response Severity Guidelines

| Severity | Examples                                    | Response Time | Escalation        |
| -------- | ------------------------------------------- | ------------- | ----------------- |
| Critical | Database down, queue not draining           | 5 minutes     | Immediate page    |
| High     | High failure rate, resource near exhaustion | 30 minutes    | Page if persists  |
| Medium   | Elevated latency, unusual patterns          | 4 hours       | Next business day |
| Low      | Single job failures, expected spikes        | 24 hours      | Weekly review     |

---

## Recovery Procedures Summary

| Scenario                 | Self-Healing       | Manual Required     |
| ------------------------ | ------------------ | ------------------- |
| Worker crash             | ✓ (via timeout)    | Investigation only  |
| Power loss               | ✓ (on restart)     | Verification only   |
| Corrupt data             | ✗                  | Restore required    |
| Stuck job                | ✓ (via timeout)    | Handler fix         |
| Overload                 | Partial (priority) | Scaling decision    |
| Dependency deadlock      | Limited            | Intervention        |
| Restart during execution | ✓                  | None                |
| Resource exhaustion      | ✗                  | Resource allocation |
| External service down    | ✓ (retry)          | External fix        |
| Malformed payload        | N/A                | Client fix          |
| Handler panic            | ✓                  | Handler fix         |
| Clock skew               | Partial            | Depends on severity |

---

_This playbook is a living document. Update it as new failure modes are discovered and response procedures are refined._
