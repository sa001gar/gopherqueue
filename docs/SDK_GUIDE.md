# GopherQueue SDK Guide

GopherQueue provides official SDKs for **Python** and **JavaScript/TypeScript**. All SDKs communicate with the GopherQueue HTTP API and provide idiomatic interfaces for each language.

## Quick Reference

| Feature         | Python             | JavaScript/TS      |
| --------------- | ------------------ | ------------------ |
| Async Support   | ✅ asyncio         | ✅ async/await     |
| Sync Support    | ✅ httpx           | ❌ async only      |
| Type Hints      | ✅ Full            | ✅ TypeScript      |
| SSE Events      | ✅ async generator | ✅ async generator |
| Package Manager | pip                | npm                |

---

## Python SDK

### Installation

```bash
pip install gopherqueue
```

### Basic Usage

```python
from gopherqueue import GopherQueue

async def main():
    async with GopherQueue("http://localhost:8080") as client:
        # Submit a job
        job = await client.submit("email", {"to": "user@example.com"})
        print(f"Created job: {job.id}")

        # Wait for completion
        result = await client.wait(job.id, timeout=30)
        if result.success:
            print("Job completed successfully!")

# Run
import asyncio
asyncio.run(main())
```

### Synchronous Client

```python
from gopherqueue import GopherQueueSync

client = GopherQueueSync("http://localhost:8080")
job = client.submit("email", {"to": "user@example.com"})
result = client.wait(job.id, timeout=30)
```

### SSE Real-Time Events

```python
async for event in client.events(job_types=["email"]):
    print(f"{event.event_type}: {event.data}")
```

---

## JavaScript/TypeScript SDK

### Installation

```bash
npm install gopherqueue
# or
yarn add gopherqueue
```

### Basic Usage

```typescript
import { GopherQueue } from "gopherqueue";

const client = new GopherQueue("http://localhost:8080");

// Submit a job
const job = await client.submit("email", { to: "user@example.com" });
console.log(`Created job: ${job.id}`);

// Wait for completion
const result = await client.wait(job.id, { timeout: 30000 });
if (result.success) {
  console.log("Job completed successfully!");
}
```

### With API Key

```typescript
const client = new GopherQueue("http://localhost:8080", {
  apiKey: "your-api-key",
  timeout: 60000,
});
```

### SSE Real-Time Events

```typescript
for await (const event of client.events({ jobTypes: ["email"] })) {
  console.log(`${event.event}: ${event.data.id}`);
}
```

---

## Common Operations

### Batch Job Submission

Submit multiple jobs atomically (all succeed or all fail):

**Python:**

```python
jobs = [
    {"type": "email", "payload": {"to": "user1@example.com"}},
    {"type": "email", "payload": {"to": "user2@example.com"}}
]
result = await client.submit_batch(jobs, atomic=True)
```

**JavaScript:**

```typescript
const result = await client.submitBatch(
  [
    { type: "email", payload: { to: "user1@example.com" } },
    { type: "email", payload: { to: "user2@example.com" } },
  ],
  { atomic: true },
);
```

### Error Handling

All SDKs provide specific exception types:

| Error      | Python                | JavaScript            |
| ---------- | --------------------- | --------------------- |
| Not Found  | `JobNotFoundError`    | `JobNotFoundError`    |
| Validation | `ValidationError`     | `ValidationError`     |
| Auth       | `AuthenticationError` | `AuthenticationError` |

### Queue Statistics

```python
stats = await client.stats()
print(f"Pending: {stats.pending}, Running: {stats.running}")
```

```typescript
const stats = await client.stats();
console.log(`Pending: ${stats.pending}, Running: ${stats.running}`);
```

---

## API Compatibility

All SDKs support these API endpoints:

| Endpoint                   | Method | Description       |
| -------------------------- | ------ | ----------------- |
| `/api/v1/jobs`             | POST   | Submit single job |
| `/api/v1/jobs/batch`       | POST   | Submit batch      |
| `/api/v1/jobs/{id}`        | GET    | Get job status    |
| `/api/v1/jobs/{id}/wait`   | POST   | Long-poll wait    |
| `/api/v1/jobs/{id}/cancel` | POST   | Cancel job        |
| `/api/v1/jobs/{id}/retry`  | POST   | Retry failed job  |
| `/api/v1/jobs/{id}/result` | GET    | Get result        |
| `/api/v1/events`           | GET    | SSE stream        |
| `/api/v1/stats`            | GET    | Queue stats       |

## Framework Integrations

GopherQueue is designed to be framework-agnostic, but here are standard patterns for integrating with popular frameworks.

### Next.js (Node.js)

In Next.js, use GopherQueue in your API routes or Server Actions.

**Installation:**

```bash
npm install gopherqueue
```

**API Route (`pages/api/queue.ts` or `app/api/queue/route.ts`):**

```typescript
import { NextResponse } from "next/server";
import { GopherQueue } from "gopherqueue";

// Initialize client (singleton pattern recommended)
const gq = new GopherQueue(
  process.env.GOPHERQUEUE_URL || "http://localhost:8080",
);

export async function POST(request: Request) {
  const body = await request.json();

  // Submit job
  const job = await gq.submit("email_workflow", body, {
    priority: 1, // High
    idempotencyKey: body.requestId, // prevent duplicates
  });

  return NextResponse.json({
    success: true,
    jobId: job.id,
    status: "queued",
  });
}
```

### React (Frontend)

Since GopherQueue is a backend service, your React frontend should not connect to it directly. Instead, have your React app call your backend API (like the Next.js example above), which then enququeues the job.

**Component Example:**

```tsx
"use client";
import { useState } from "react";

export default function JobTrigger() {
  const [status, setStatus] = useState("idle");

  const triggerJob = async () => {
    setStatus("submitting");
    try {
      const res = await fetch("/api/queue", {
        method: "POST",
        body: JSON.stringify({ action: "generate_report", userId: "123" }),
      });
      const data = await res.json();
      setStatus(`Job ${data.jobId} submitted!`);
    } catch (err) {
      setStatus("Failed");
    }
  };

  return (
    <button onClick={triggerJob} disabled={status === "submitting"}>
      {status === "submitting" ? "Processing..." : "Start Background Job"}
    </button>
  );
}
```

### Django (Python) - Complete Integration Guide

GopherQueue integrates seamlessly with Django for background job processing.

#### Installation

```bash
pip install gopherqueue
```

#### Configuration

**`settings.py`:**

```python
# GopherQueue Configuration
GOPHERQUEUE_URL = "http://localhost:8080"
GOPHERQUEUE_API_KEY = env("GOPHERQUEUE_API_KEY", default=None)
GOPHERQUEUE_TIMEOUT = 30  # seconds
```

**`queue.py` (Create a reusable client module):**

```python
from django.conf import settings
from gopherqueue import GopherQueue, GopherQueueSync

# Synchronous client for regular Django views
sync_client = GopherQueueSync(
    url=settings.GOPHERQUEUE_URL,
    api_key=settings.GOPHERQUEUE_API_KEY,
    timeout=settings.GOPHERQUEUE_TIMEOUT
)

# Async client for async views/management commands
async_client = GopherQueue(
    url=settings.GOPHERQUEUE_URL,
    api_key=settings.GOPHERQUEUE_API_KEY,
)
```

#### Synchronous Views (Standard Django)

**`views.py`:**

```python
from django.http import JsonResponse
from django.views.decorators.http import require_POST
from django.contrib.auth.decorators import login_required
from .queue import sync_client

@login_required
@require_POST
def send_welcome_email(request):
    """Trigger welcome email job."""
    job = sync_client.submit(
        job_type="send_email",
        payload={
            "template": "welcome",
            "user_id": request.user.id,
            "email": request.user.email,
        },
        priority=1,  # High priority
        idempotency_key=f"welcome-{request.user.id}"  # Prevent duplicates
    )
    return JsonResponse({
        "status": "queued",
        "job_id": job.id,
        "message": "Email will be sent shortly"
    })

@login_required
@require_POST
def generate_report(request):
    """Generate PDF report in background."""
    import json
    data = json.loads(request.body)

    job = sync_client.submit(
        job_type="generate_report",
        payload={
            "report_type": data.get("type", "monthly"),
            "user_id": request.user.id,
            "filters": data.get("filters", {}),
        },
        priority=3,  # Low priority (batch work)
        timeout="10m",  # Allow up to 10 minutes
    )
    return JsonResponse({"job_id": job.id})
```

#### Async Views (Django 4.1+)

```python
from django.http import JsonResponse
from .queue import async_client

async def async_batch_process(request):
    """Submit batch jobs asynchronously."""
    jobs = []
    for item_id in request.POST.getlist("item_ids"):
        job = await async_client.submit(
            "process_item",
            {"item_id": item_id}
        )
        jobs.append(job.id)

    return JsonResponse({"queued_jobs": jobs})
```

#### Django Signals Integration

Automatically queue jobs when models change:

**`signals.py`:**

```python
from django.db.models.signals import post_save
from django.dispatch import receiver
from .models import Order
from .queue import sync_client

@receiver(post_save, sender=Order)
def order_created_handler(sender, instance, created, **kwargs):
    if created:
        # Queue multiple background tasks for new orders
        sync_client.submit("send_order_confirmation", {
            "order_id": instance.id,
            "email": instance.customer_email
        })
        sync_client.submit("notify_warehouse", {
            "order_id": instance.id,
            "items": list(instance.items.values("sku", "quantity"))
        })
        sync_client.submit("update_analytics", {
            "event": "order_created",
            "order_id": instance.id,
            "total": str(instance.total)
        }, priority=4)  # Bulk priority
```

#### Management Command for Job Status

**`management/commands/job_status.py`:**

```python
from django.core.management.base import BaseCommand
from myapp.queue import sync_client

class Command(BaseCommand):
    help = 'Check GopherQueue job status'

    def add_arguments(self, parser):
        parser.add_argument('job_id', type=str)

    def handle(self, *args, **options):
        job = sync_client.get(options['job_id'])
        self.stdout.write(f"Job: {job.id}")
        self.stdout.write(f"State: {job.state}")
        self.stdout.write(f"Progress: {job.progress}%")
        if job.result:
            self.stdout.write(f"Result: {job.result}")
```

#### Checking Job Status from Frontend

**`views.py`:**

```python
def job_status(request, job_id):
    """API endpoint for frontend to poll job status."""
    try:
        job = sync_client.get(job_id)
        return JsonResponse({
            "id": job.id,
            "state": job.state,
            "progress": job.progress,
            "result": job.result if job.state == "completed" else None,
            "error": job.result.get("error") if job.state == "failed" else None
        })
    except Exception as e:
        return JsonResponse({"error": str(e)}, status=404)
```

#### Migration from Celery

If you're migrating from Celery:

| Celery            | GopherQueue          |
| ----------------- | -------------------- |
| `@task` decorator | HTTP API call        |
| `delay()`         | `client.submit()`    |
| `AsyncResult`     | `client.get(job_id)` |
| Redis/RabbitMQ    | Built-in BoltDB      |
| `celery worker`   | `gq serve`           |

**Before (Celery):**

```python
@app.task
def send_email(user_id, template):
    # task code
    pass

send_email.delay(user_id=123, template="welcome")
```

**After (GopherQueue):**

```python
from .queue import sync_client

sync_client.submit("send_email", {
    "user_id": 123,
    "template": "welcome"
})
```

---

### Flask & FastAPI

#### Flask Integration

```python
from flask import Flask, request, jsonify
from gopherqueue import GopherQueueSync

app = Flask(__name__)
queue = GopherQueueSync("http://localhost:8080")

@app.route('/api/jobs', methods=['POST'])
def create_job():
    data = request.json
    job = queue.submit(data['type'], data['payload'])
    return jsonify({"job_id": job.id})

@app.route('/api/jobs/<job_id>')
def get_job(job_id):
    job = queue.get(job_id)
    return jsonify({"state": job.state, "progress": job.progress})
```

#### FastAPI Integration (Async Native)

```python
from fastapi import FastAPI, BackgroundTasks
from gopherqueue import GopherQueue

app = FastAPI()
queue = GopherQueue("http://localhost:8080")

@app.on_event("startup")
async def startup():
    # Connection is lazy, but you can warm it up
    pass

@app.post("/api/jobs")
async def create_job(job_type: str, payload: dict):
    job = await queue.submit(job_type, payload)
    return {"job_id": job.id}

@app.get("/api/jobs/{job_id}")
async def get_job(job_id: str):
    job = await queue.get(job_id)
    return {"state": job.state, "progress": job.progress}

@app.get("/api/jobs/{job_id}/wait")
async def wait_for_job(job_id: str, timeout: int = 30):
    result = await queue.wait(job_id, timeout=timeout)
    return {"completed": result.completed, "success": result.success}
```

---

## Deployment & Operations

### Running GopherQueue

GopherQueue is a single binary application. You can run it directly or via Docker.

**Basic Start Command:**

```bash
gq serve
```

**Production Flags:**

| Flag            | Description             | Recommended Prod Value   |
| --------------- | ----------------------- | ------------------------ |
| `--http`        | Bind address            | `:8080` (or internal IP) |
| `--workers`     | Concurrent worker count | `20-50` (depends on CPU) |
| `--data-dir`    | Storage location        | `/var/lib/gopherqueue`   |
| `--auth-secret` | API Key Secret          | `env:GQ_SECRET`          |

### Docker Deployment

**`docker-compose.yml`:**

```yaml
version: "3.8"
services:
  gopherqueue:
    image: sa001gar/gopherqueue:latest
    container_name: gopherqueue
    restart: always
    ports:
      - "8080:8080"
    volumes:
      - gq_data:/data
    command: ["serve", "--http=:8080", "--data-dir=/data", "--workers=20"]
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/live"]
      interval: 30s
      timeout: 10s
      retries: 3

volumes:
  gq_data:
```

### Deployment Diagrams

**Simple Deployment:**

```text
[ Client App ]  --->  [ GopherQueue Server ]  <--- [ Worker 1..N ]
     (API)               (BoltDB Storage)           (Embedded)
```

**High Availability (with Load Balancer):**
_Note: Since GopherQueue uses embedded storage (BoltDB), it is designed for vertical scaling or partitioned queues. For multi-node setup, you would typically run separate instances for different queues or use a shared storage backend (future feature)._

```text
[ Load Balancer ]
      |
      +---> [ GopherQueue (Instance A) ]
      |
      +---> [ GopherQueue (Instance B) ]
```
