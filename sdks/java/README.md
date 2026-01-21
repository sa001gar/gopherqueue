# GopherQueue Java SDK

A Java client library for [GopherQueue](https://github.com/sa001gar/gopherqueue),
an enterprise-grade background job processing system.

## Installation

### Maven

```xml
<dependency>
    <groupId>dev.gopherqueue</groupId>
    <artifactId>gopherqueue-sdk</artifactId>
    <version>1.0.0</version>
</dependency>
```

### Gradle

```groovy
implementation 'dev.gopherqueue:gopherqueue-sdk:1.0.0'
```

## Quick Start

```java
import dev.gopherqueue.*;
import java.time.Duration;
import java.util.Map;

public class Example {
    public static void main(String[] args) throws Exception {
        try (GopherQueueClient client = new GopherQueueClient("http://localhost:8080")) {
            // Submit a job
            Job job = client.submit("email", Map.of(
                "to", "user@example.com",
                "subject", "Hello!"
            )).get();

            System.out.println("Job created: " + job.getId());

            // Wait for completion
            WaitResult result = client.wait(job.getId(), Duration.ofSeconds(30)).get();
            System.out.println("Job completed: " + result.isSuccess());
        }
    }
}
```

## Features

- **CompletableFuture async API** - Non-blocking operations
- **Java 11+ support** - Modern Java compatibility
- **Batch submission** - Submit up to 1000 jobs atomically
- **Long-polling wait** - Efficiently wait for job completion
- **Builder pattern** - Fluent API for job options
- **AutoCloseable** - Resource management via try-with-resources

## API Reference

### Client Options

```java
// Basic client
GopherQueueClient client = new GopherQueueClient("http://localhost:8080");

// With API key
GopherQueueClient client = new GopherQueueClient("http://localhost:8080", "your-api-key");

// With custom timeout
GopherQueueClient client = new GopherQueueClient(
    "http://localhost:8080",
    "your-api-key",
    Duration.ofSeconds(60)
);
```

### Submit Job

```java
// Simple submission
Job job = client.submit("email", Map.of("to", "user@example.com")).get();

// With options
SubmitOptions options = SubmitOptions.builder()
    .priority(Priority.HIGH)
    .delay(Duration.ofMinutes(5))
    .timeout(Duration.ofMinutes(30))
    .maxAttempts(3)
    .idempotencyKey("unique-key")
    .tags(Map.of("env", "prod"))
    .build();

Job job = client.submit("email", payload, options).get();
```

### Batch Submit

```java
List<Map<String, Object>> jobs = List.of(
    Map.of("type", "email", "payload", Map.of("to", "user1@example.com")),
    Map.of("type", "email", "payload", Map.of("to", "user2@example.com"))
);

BatchResult result = client.submitBatch(jobs, true).get();  // atomic=true
System.out.println("Accepted: " + result.getAccepted());
```

### Wait for Job

```java
WaitResult result = client.wait(job.getId(), Duration.ofSeconds(60)).get();
if (result.isCompleted()) {
    if (result.isSuccess()) {
        System.out.println("Output: " + result.getResult());
    } else {
        System.out.println("Error: " + result.getResult().getError());
    }
}
```

### Get Job Status

```java
Job job = client.get(jobId).get();
System.out.println("State: " + job.getState() + ", Progress: " + job.getProgress() + "%");
```

### List Jobs

```java
List<Job> jobs = client.list("running", "email", 100).get();
for (Job job : jobs) {
    System.out.println(job.getId() + ": " + job.getState());
}
```

### Cancel Job

```java
Job job = client.cancel(jobId, "No longer needed", true).get();
```

### Retry Failed Job

```java
Job job = client.retry(jobId, true).get();  // resetAttempts=true
```

### Queue Statistics

```java
QueueStats stats = client.stats().get();
System.out.println("Pending: " + stats.getPending() + ", Running: " + stats.getRunning());
```

## Error Handling

```java
try {
    Job job = client.get("non-existent-id").get();
} catch (ExecutionException e) {
    if (e.getCause() instanceof JobNotFoundException) {
        JobNotFoundException notFound = (JobNotFoundException) e.getCause();
        System.out.println("Job not found: " + notFound.getJobId());
    } else if (e.getCause() instanceof GopherQueueException) {
        GopherQueueException gqe = (GopherQueueException) e.getCause();
        System.out.println("Error [" + gqe.getCode() + "]: " + gqe.getMessage());
    }
}
```

## Async vs Sync Usage

All methods return `CompletableFuture`. For synchronous usage:

```java
// Blocking (synchronous)
Job job = client.submit("email", payload).get();

// Non-blocking (asynchronous)
client.submit("email", payload)
    .thenAccept(job -> System.out.println("Created: " + job.getId()))
    .exceptionally(e -> {
        System.err.println("Error: " + e.getMessage());
        return null;
    });
```

## License

MIT License
