# GopherQueue JavaScript/TypeScript SDK

A TypeScript/JavaScript client library for [GopherQueue](https://github.com/sa001gar/gopherqueue),
an enterprise-grade background job processing system.

## Installation

```bash
npm install gopherqueue
# or
yarn add gopherqueue
# or
pnpm add gopherqueue
```

## Quick Start

```typescript
import { GopherQueue } from "gopherqueue";

const client = new GopherQueue("http://localhost:8080");

// Submit a job
const job = await client.submit("email", {
  to: "user@example.com",
  subject: "Hello!",
});
console.log(`Job created: ${job.id}`);

// Wait for completion
const result = await client.wait(job.id, { timeout: 30000 });
console.log(`Job completed: ${result.success}`);
```

## Features

- **Full TypeScript support** - Complete type definitions included
- **Modern async/await API** - Promise-based interface
- **Zero dependencies** - Uses native fetch API
- **Batch submission** - Submit up to 1000 jobs atomically
- **Long-polling wait** - Efficiently wait for job completion
- **SSE events** - Real-time job status updates via async generators
- **Tree-shakeable** - ESM and CJS builds available

## API Reference

### Client Options

```typescript
import { GopherQueue } from "gopherqueue";

const client = new GopherQueue("http://localhost:8080", {
  apiKey: "your-api-key", // Optional API key
  timeout: 30000, // Request timeout in ms
});
```

### Submit Job

```typescript
const job = await client.submit(
  "email",
  { to: "user@example.com" },
  {
    priority: 1, // 0=Critical, 1=High, 2=Normal, 3=Low, 4=Bulk
    delay: "5m", // Delay before execution
    timeout: "30m", // Max execution time
    maxAttempts: 3, // Retry attempts
    idempotencyKey: "unique-key", // Deduplication
    tags: { env: "prod" }, // Metadata
  },
);
```

### Batch Submit

```typescript
const result = await client.submitBatch(
  [
    { type: "email", payload: { to: "user1@example.com" } },
    { type: "email", payload: { to: "user2@example.com" } },
  ],
  { atomic: true },
); // All-or-nothing

console.log(`Accepted: ${result.accepted}, Rejected: ${result.rejected}`);
```

### Wait for Job

```typescript
const result = await client.wait(job.id, { timeout: 60000 });
if (result.completed) {
  if (result.success) {
    console.log("Output:", result.result);
  } else {
    console.log("Error:", result.result?.error);
  }
}
```

### Get Job Status

```typescript
const job = await client.get(jobId);
console.log(`State: ${job.state}, Progress: ${job.progress}%`);
```

### List Jobs

```typescript
const jobs = await client.list({ state: "running", type: "email", limit: 100 });
for (const job of jobs) {
  console.log(`${job.id}: ${job.state}`);
}
```

### Cancel Job

```typescript
const job = await client.cancel(jobId, {
  reason: "No longer needed",
  force: true,
});
```

### Retry Failed Job

```typescript
const job = await client.retry(jobId, { resetAttempts: true });
```

### SSE Events

```typescript
// Subscribe to all job events
for await (const event of client.events("*")) {
  console.log(`Event: ${event.event}, Data:`, event.data);
}

// Subscribe to specific job
for await (const event of client.events(jobId)) {
  if (event.event === "job.completed") break;
}
```

### Queue Statistics

```typescript
const stats = await client.stats();
console.log(`Pending: ${stats.pending}, Running: ${stats.running}`);
```

## Error Handling

```typescript
import { GopherQueue, JobNotFoundError, ValidationError } from "gopherqueue";

try {
  const job = await client.get("non-existent-id");
} catch (error) {
  if (error instanceof JobNotFoundError) {
    console.log("Job not found:", error.jobId);
  } else if (error instanceof ValidationError) {
    console.log("Validation error:", error.message);
  }
}
```

## Browser Support

This SDK uses the native `fetch` API and is compatible with:

- Node.js 18+
- Modern browsers (Chrome, Firefox, Safari, Edge)
- Deno
- Bun

For Node.js 16-17, you may need to provide a polyfill:

```typescript
import fetch from "node-fetch";

const client = new GopherQueue("http://localhost:8080", {
  fetch: fetch as any,
});
```

## License

MIT License
