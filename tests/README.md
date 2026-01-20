# GopherQueue Testing Strategy

## Overview

GopherQueue employs a three-layer testing strategy to ensure reliability at every level: unit tests for individual components, integration tests for component interactions, and chaos tests for failure resilience.

---

## 1. Unit Tests

Unit tests validate individual functions and types in isolation. They are fast, deterministic, and run on every commit.

### Coverage Targets

| Package     | Target Coverage | Focus Areas                                            |
| ----------- | --------------- | ------------------------------------------------------ |
| core        | 90%             | Job creation, option application, error classification |
| persistence | 85%             | CRUD operations, filtering, statistics                 |
| scheduler   | 80%             | State transitions, retry logic, priority ordering      |
| worker      | 80%             | Job execution, panic recovery, timeout enforcement     |
| security    | 90%             | Authentication, authorization decisions                |

### Example Test Cases

**core/job_test.go:**

- `TestNewJob_DefaultValues` - Verify default field values
- `TestNewJob_WithOptions` - Verify option application
- `TestJob_StateTransitions` - Verify valid state transitions
- `TestClassifyError_TemporaryError` - Verify temporary error classification
- `TestClassifyError_PermanentError` - Verify permanent error classification

**persistence/memory_store_test.go:**

- `TestMemoryStore_Create` - Job creation succeeds
- `TestMemoryStore_Create_Duplicate` - Idempotency key deduplication
- `TestMemoryStore_Get_NotFound` - Returns proper error for missing job
- `TestMemoryStore_UpdateState` - Atomic state update
- `TestMemoryStore_GetPending_Priority` - Priority-ordered retrieval
- `TestMemoryStore_Cleanup` - Expired job removal

**scheduler/scheduler_test.go:**

- `TestScheduler_Enqueue` - Job added to queue
- `TestScheduler_Complete` - State transition to completed
- `TestScheduler_Fail_WithRetry` - Retry scheduling with backoff
- `TestScheduler_Fail_ExhaustedRetries` - Dead-letter on max attempts
- `TestScheduler_Cancel_Pending` - Cancel pending job

**worker/pool_test.go:**

- `TestPool_Start_Stop` - Graceful lifecycle
- `TestPool_Submit` - Job accepted when running
- `TestPool_Submit_Shutdown` - Job rejected when stopped
- `TestPool_RegisterHandler` - Handler registration
- `TestPool_ProcessJob_Success` - Successful execution
- `TestPool_ProcessJob_Panic` - Panic recovery

---

## 2. Integration Tests

Integration tests validate component interactions and end-to-end flows. They use real (in-memory) implementations and verify the system behaves correctly as a whole.

### Test Scenarios

**Job Lifecycle:**

```go
func TestJobLifecycle_SuccessfulExecution(t *testing.T) {
    // Setup: Create store, scheduler, and pool
    // Register handler that succeeds
    // Submit job
    // Verify: Job transitions pending → running → completed
}

func TestJobLifecycle_FailureAndRetry(t *testing.T) {
    // Setup: Create store, scheduler, and pool
    // Register handler that fails twice then succeeds
    // Submit job with max_attempts=3
    // Verify: Job retries twice then completes
}

func TestJobLifecycle_ExhaustedRetries(t *testing.T) {
    // Setup: Create components
    // Register handler that always fails
    // Submit job with max_attempts=3
    // Verify: Job transitions to dead_letter after 3 attempts
}
```

**API Integration:**

```go
func TestAPI_SubmitAndQuery(t *testing.T) {
    // Setup: Create server with all components
    // POST /api/v1/jobs with valid payload
    // GET /api/v1/jobs/{id}
    // Verify: Response matches submitted job
}

func TestAPI_Idempotency(t *testing.T) {
    // Submit same job twice with idempotency key
    // Verify: Second request returns existing job ID
}

func TestAPI_CancelRunningJob(t *testing.T) {
    // Submit long-running job
    // DELETE /api/v1/jobs/{id}?force=true
    // Verify: Job transitions to cancelled
}
```

**Concurrency:**

```go
func TestConcurrency_ParallelSubmissions(t *testing.T) {
    // Submit 1000 jobs in parallel
    // Verify: All jobs created without errors
}

func TestConcurrency_ParallelExecution(t *testing.T) {
    // Submit 100 jobs
    // Run with 10 workers
    // Verify: All jobs complete, worker utilization balanced
}
```

---

## 3. Chaos/Failure Tests

Chaos tests validate system resilience under failure conditions. They simulate real-world failures and verify the system recovers correctly.

### Failure Injection

**Worker Failures:**

```go
func TestChaos_WorkerPanic(t *testing.T) {
    // Register handler that panics
    // Submit job
    // Verify: Worker recovers, job marked as failed
}

func TestChaos_WorkerTimeout(t *testing.T) {
    // Register handler that sleeps forever
    // Submit job with 1s timeout
    // Verify: Job times out, worker continues processing
}

func TestChaos_WorkerCrashMidJob(t *testing.T) {
    // Register handler that simulates crash (cancel context)
    // Submit job
    // Wait for visibility timeout
    // Verify: Job recovered and rescheduled
}
```

**Scheduler Failures:**

```go
func TestChaos_SchedulerRestart(t *testing.T) {
    // Submit jobs
    // Stop scheduler
    // Restart scheduler
    // Verify: Pending jobs continue processing
}
```

**Persistence Failures:**

```go
func TestChaos_PersistenceLatency(t *testing.T) {
    // Wrap store with artificial latency
    // Submit and process jobs
    // Verify: System handles slow persistence gracefully
}

func TestChaos_PersistenceErrors(t *testing.T) {
    // Wrap store with intermittent errors
    // Verify: Operations retry appropriately
}
```

**Resource Exhaustion:**

```go
func TestChaos_QueueFull(t *testing.T) {
    // Submit jobs until queue is full
    // Verify: New submissions rejected with proper error
}

func TestChaos_HighLoad(t *testing.T) {
    // Submit jobs at 10x normal rate
    // Verify: System remains stable, no goroutine leaks
}
```

---

## Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test ./core/...

# Run with race detector
go test -race ./...

# Run integration tests (tagged)
go test -tags=integration ./tests/...

# Run chaos tests (tagged)
go test -tags=chaos ./tests/...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

---

## Test Fixtures

### Example Job Payloads

```go
var testPayloads = map[string][]byte{
    "email.send":       []byte(`{"to":"test@example.com","subject":"Test"}`),
    "payment.process":  []byte(`{"amount":100,"currency":"USD"}`),
    "report.generate":  []byte(`{"type":"monthly","format":"pdf"}`),
}
```

### Example Handlers

```go
func successHandler(ctx core.JobContext, job *core.Job) (*core.JobResult, error) {
    return &core.JobResult{Success: true}, nil
}

func failingHandler(ctx core.JobContext, job *core.Job) (*core.JobResult, error) {
    return nil, core.NewTemporaryError("transient failure", nil)
}

func slowHandler(ctx core.JobContext, job *core.Job) (*core.JobResult, error) {
    time.Sleep(5 * time.Second)
    return &core.JobResult{Success: true}, nil
}

func panicHandler(ctx core.JobContext, job *core.Job) (*core.JobResult, error) {
    panic("simulated panic")
}
```

---

## Continuous Integration

Tests run automatically on:

- Every pull request
- Every push to main
- Nightly for extended chaos tests

### CI Pipeline

```yaml
test:
  steps:
    - name: Unit Tests
      run: go test -race -cover ./...

    - name: Integration Tests
      run: go test -tags=integration -race ./tests/...

    - name: Chaos Tests (nightly only)
      run: go test -tags=chaos -timeout=30m ./tests/...
```
