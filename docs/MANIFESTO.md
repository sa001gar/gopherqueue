# GopherQueue System Manifesto

**Version:** 1.0  
**Status:** Foundational Document  
**Classification:** Engineering Documentation

---

## I. The Problem We Solve

Modern applications require background job processing. Whether it's sending emails, processing payments, generating reports, or synchronizing data, developers need reliable mechanisms to execute work outside the request-response cycle.

The existing landscape presents developers with a painful choice:

**Distributed Message Brokers** (RabbitMQ, Kafka, Redis Streams) provide durability and scalability but demand operational expertise, infrastructure overhead, and introduce network partitioning as a failure mode. A developer who needs to process 1,000 jobs per hour inherits the complexity designed for 1,000,000 jobs per second.

**Cloud Queue Services** (AWS SQS, Google Cloud Tasks, Azure Queue) offer managed infrastructure but create vendor lock-in, introduce latency through network round-trips, incur per-operation costs, and place your job data on infrastructure you do not control.

**In-Process Solutions** (goroutine pools, simple channels) provide simplicity but sacrifice durability. A process restart means lost jobs. A crash means lost jobs. There is no recovery, no visibility, no operational tooling.

**Language-Specific Frameworks** (Celery, Sidekiq, BullMQ) solve well for their ecosystems but bring heavy dependencies, assume specific brokers, and embed opinions that may conflict with application architecture.

GopherQueue exists for a specific class of systems: **applications that need reliable background job processing without distributed infrastructure complexity**. These are applications where:

- Jobs number in the thousands per hour, not millions per second
- A single server (or small cluster) handles the workload
- Durability to local disk is sufficient; geographic replication is unnecessary
- Operational simplicity outweighs theoretical scalability
- The development team cannot (or should not) operate message broker infrastructure

---

## II. Philosophical Principles

### Simplicity Over Features

Every feature added is complexity inherited by every user. We prefer a small, understandable core over a comprehensive feature set. A developer should be able to read and understand the entire codebase in a day.

What this means in practice:

- No plugin architectures; extend through composition
- No configuration files with hundreds of options; sensible defaults
- No abstraction layers that hide what the system actually does
- No magic; behavior is explicit and traceable

### Reliability Over Performance

We optimize for correctness first. A system that loses jobs quickly is worthless; a system that processes jobs slowly but never loses them is useful.

What this means in practice:

- We fsync before acknowledging persistence
- We checkpoint state before advancing
- We favor pessimistic locking over optimistic concurrency
- We accept performance costs for stronger guarantees

### Determinism Over Flexibility

Given the same inputs and state, the system behaves identically. Randomness, time-based jitter, and non-deterministic scheduling are explicit opt-ins, never defaults.

What this means in practice:

- Job ordering within a priority class is deterministic
- Retry timing follows explicit, predictable schedules
- Recovery produces identical results regardless of when it runs
- Testing can rely on reproducible behavior

### Fault Tolerance Through Honesty

We do not prevent failures; we assume they are inevitable and design for graceful handling. The system acknowledges its limitations rather than promising impossible guarantees.

What this means in practice:

- We document what can fail and how
- We provide clear signals when the system is degraded
- We favor explicit failure modes over silent corruption
- We make recovery a first-class operation, not an afterthought

---

## III. System Guarantees

### Delivery Guarantees

**At-Least-Once Delivery:** Every job that is successfully persisted will be delivered to a worker at least once. In failure scenarios (worker crash, timeout), the job may be delivered multiple times.

**No Exactly-Once Delivery:** We do not provide exactly-once semantics. Exactly-once requires either application-level idempotency or distributed transaction coordination. We provide the primitives for the former; we refuse the complexity of the latter.

**Ordering Guarantee:** Jobs within the same priority class are delivered in submission order. Jobs across priority classes are delivered in priority order. Priority preemption does not violate per-class ordering.

### Durability Guarantees

**Persistence Before Acknowledgment:** A job submission call returns only after the job is durably persisted. Process termination immediately after return will not lose the job.

**Crash Recovery:** The system recovers all persisted-but-incomplete jobs after any form of crash (process, OS, hardware). Recovery is automatic and requires no operator intervention beyond restarting the process.

**State Consistency:** Job state transitions are atomic. A job is never observed in an intermediate or corrupted state, even after crash recovery.

### Retry Guarantees

**Bounded Retries:** Every job has a maximum retry count. We do not retry indefinitely; poison jobs eventually move to a dead-letter state.

**Backoff Semantics:** Retry delays follow configurable backoff strategies (linear, exponential, custom). Backoff configuration is per-job, not system-wide.

**Failure Classification:** Temporary failures trigger retries; permanent failures do not. The handler determines classification; we do not guess.

### Recovery Guarantees

**Stuck Job Detection:** Jobs that exceed their timeout without completion are detected and either retried or failed based on configuration.

**Orphan Job Recovery:** Jobs claimed by workers that subsequently crashed are reclaimed after a configurable visibility timeout.

**Idempotent Recovery:** Running recovery multiple times produces the same result as running it once.

---

## IV. What Failure Means

In GopherQueue, **failure is not an exception; it is an expected system state**. We design for failure at every layer:

### Expected Failures

- Workers crash mid-execution
- The process receives SIGKILL without cleanup opportunity
- The filesystem becomes temporarily unavailable
- Jobs take longer than expected
- Jobs fail due to transient external conditions
- Jobs fail due to permanent external conditions
- The system runs out of memory under load
- The disk fills during operation
- Jobs are submitted faster than they can be processed

### Our Contract With Failure

1. **We detect it.** Silent failures are forbidden. If something fails, the system knows and reports it.

2. **We record it.** Failure history is preserved for operational diagnosis. We do not discard evidence.

3. **We classify it.** Temporary failures get retries; permanent failures get dead-lettering. The distinction is explicit.

4. **We recover from it.** Automatic recovery handles transient failures. Operator intervention handles permanent failures. The distinction is clear.

5. **We degrade gracefully.** When the system cannot function fully, it continues functioning partially rather than failing atomically. Job submission may fail while job processing continues.

---

## V. Why Not The Alternatives?

### Why Not Celery?

Celery is a mature, powerful system. It is also Python-specific, requires a broker (Redis/RabbitMQ), introduces complex configuration, and brings substantial operational overhead. For Go applications, it means running a Python process as a sidecar or a broker as infrastructure.

GopherQueue is pure Go, requires no external infrastructure, and compiles to a single binary.

### Why Not BullMQ?

BullMQ requires Redis. For applications that don't otherwise need Redis, this means operating a distributed system to process background jobs. BullMQ is Node-specific; Go applications would need interprocess communication.

GopherQueue uses local persistence. No additional infrastructure required.

### Why Not RabbitMQ?

RabbitMQ is a messaging system that can be used for job queues. It is not a job queue. The abstraction gap means implementing retry logic, delay semantics, job state tracking, and failure handling in application code. Operating RabbitMQ requires expertise; misconfiguration leads to message loss.

GopherQueue is purpose-built for background jobs with these concerns handled by the system.

### Why Not Cloud Queues?

Cloud queues (SQS, Cloud Tasks) are excellent for cloud-native applications accepting vendor lock-in. They add latency (network round-trips for every operation), cost (per-operation pricing), and place job data on third-party infrastructure.

GopherQueue is local-first. Your data stays on your servers. Latency is disk I/O, not network I/O.

### Why Not In-Process Queues?

In-process queues (channel-based workers, thread pools) provide no durability. Process restart loses all pending work. There is no visibility, no management, no recovery.

GopherQueue provides durability, visibility, and recovery while remaining operationally simple.

---

## VI. Closing Statement

GopherQueue is not the right choice for every application. If you need:

- Millions of jobs per second: Use Kafka or a distributed job system
- Geographic replication: Use a distributed message broker
- Managed infrastructure: Use cloud queue services
- Complex workflow orchestration: Use Temporal or Cadence

GopherQueue is the right choice when you need **reliable background job processing without operational complexity**. It is for developers who want to:

- Add background jobs to an application in an afternoon
- Deploy a single binary, not a distributed system
- Sleep soundly knowing jobs will not be lost
- Debug problems with standard tools, not specialized expertise

We build GopherQueue because we believe reliable software should be simple software. Complexity is not a feature; it is a cost. We pay that cost only when we must.

---

_This document represents the foundational philosophy of the GopherQueue project. Implementation decisions should be evaluated against these principles. When trade-offs arise, refer here for guidance._
