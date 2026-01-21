# GopherQueue Python SDK

A Python client library for [GopherQueue](https://github.com/sa001gar/gopherqueue),
an enterprise-grade background job processing system.

## Installation

```bash
pip install gopherqueue
```

Or install from source:

```bash
pip install -e .
```

## Quick Start

```python
import asyncio
from gopherqueue import GopherQueue

async def main():
    # Create client
    client = GopherQueue("http://localhost:8080")

    # Submit a job
    job = await client.submit("email", {"to": "user@example.com", "subject": "Hello!"})
    print(f"Job created: {job.id}")

    # Wait for completion
    result = await client.wait(job.id, timeout=30)
    print(f"Job completed: {result.success}")

asyncio.run(main())
```

## Features

- **Async/await support** - Built on `aiohttp` for high performance
- **Sync API available** - Use `GopherQueueSync` for synchronous code
- **Batch submission** - Submit up to 1000 jobs atomically
- **Long-polling wait** - Efficiently wait for job completion
- **SSE events** - Real-time job status updates
- **Type hints** - Full type annotation support
- **API key auth** - Secure API access

## API Reference

### Client Options

```python
from gopherqueue import GopherQueue

client = GopherQueue(
    url="http://localhost:8080",
    api_key="your-api-key",       # Optional API key
    timeout=30,                     # Request timeout in seconds
    max_connections=100,            # Connection pool size
)
```

### Submit Job

```python
job = await client.submit(
    job_type="email",
    payload={"to": "user@example.com"},
    priority=1,                    # 0=Critical, 1=High, 2=Normal, 3=Low, 4=Bulk
    delay="5m",                    # Delay before execution
    timeout="30m",                 # Max execution time
    max_attempts=3,                # Retry attempts
    idempotency_key="unique-key",  # Deduplication
    tags={"env": "prod"},          # Metadata
)
```

### Batch Submit

```python
results = await client.submit_batch([
    {"type": "email", "payload": {"to": "user1@example.com"}},
    {"type": "email", "payload": {"to": "user2@example.com"}},
], atomic=True)  # All-or-nothing

print(f"Accepted: {results.accepted}, Rejected: {results.rejected}")
```

### Wait for Job

```python
result = await client.wait(job.id, timeout=60)
if result.completed:
    if result.success:
        print(f"Output: {result.result}")
    else:
        print(f"Error: {result.result.error}")
```

### Get Job Status

```python
job = await client.get(job_id)
print(f"State: {job.state}, Progress: {job.progress}%")
```

### List Jobs

```python
jobs = await client.list(state="running", type="email", limit=100)
for job in jobs:
    print(f"{job.id}: {job.state}")
```

### Cancel Job

```python
job = await client.cancel(job_id, reason="No longer needed", force=True)
```

### Retry Failed Job

```python
job = await client.retry(job_id, reset_attempts=True)
```

### SSE Events

```python
async for event in client.events(job_id="*"):  # Subscribe to all jobs
    print(f"Event: {event.type}, Job: {event.data.id}")
```

## Synchronous API

```python
from gopherqueue import GopherQueueSync

client = GopherQueueSync("http://localhost:8080")

job = client.submit("email", {"to": "user@example.com"})
result = client.wait(job.id, timeout=30)
```

## License

MIT License
