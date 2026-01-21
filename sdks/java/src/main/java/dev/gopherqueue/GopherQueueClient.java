package dev.gopherqueue;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import okhttp3.*;

import java.io.IOException;
import java.time.Duration;
import java.util.*;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.TimeUnit;

/**
 * GopherQueue Java client.
 * 
 * <p>Example usage:
 * <pre>{@code
 * GopherQueueClient client = new GopherQueueClient("http://localhost:8080");
 * 
 * // Submit a job
 * Job job = client.submit("email", Map.of("to", "user@example.com")).get();
 * 
 * // Wait for completion
 * WaitResult result = client.wait(job.getId(), Duration.ofSeconds(30)).get();
 * System.out.println("Success: " + result.isSuccess());
 * }</pre>
 */
public class GopherQueueClient implements AutoCloseable {
    private final String baseUrl;
    private final String apiKey;
    private final OkHttpClient httpClient;
    private final ObjectMapper objectMapper;
    
    /**
     * Create a new GopherQueue client.
     * 
     * @param url Base URL of the GopherQueue server (e.g., "http://localhost:8080")
     */
    public GopherQueueClient(String url) {
        this(url, null);
    }
    
    /**
     * Create a new GopherQueue client with API key authentication.
     * 
     * @param url Base URL of the GopherQueue server
     * @param apiKey Optional API key for authentication
     */
    public GopherQueueClient(String url, String apiKey) {
        this(url, apiKey, Duration.ofSeconds(30));
    }
    
    /**
     * Create a new GopherQueue client with custom timeout.
     * 
     * @param url Base URL of the GopherQueue server
     * @param apiKey Optional API key for authentication
     * @param timeout Request timeout
     */
    public GopherQueueClient(String url, String apiKey, Duration timeout) {
        this.baseUrl = url.replaceAll("/$", "");
        this.apiKey = apiKey;
        
        this.httpClient = new OkHttpClient.Builder()
            .connectTimeout(timeout.toMillis(), TimeUnit.MILLISECONDS)
            .readTimeout(timeout.toMillis(), TimeUnit.MILLISECONDS)
            .writeTimeout(timeout.toMillis(), TimeUnit.MILLISECONDS)
            .build();
        
        this.objectMapper = new ObjectMapper()
            .registerModule(new JavaTimeModule())
            .configure(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES, false);
    }
    
    /**
     * Submit a new job.
     * 
     * @param type The job type (must match a registered handler)
     * @param payload The job payload
     * @return CompletableFuture containing the created job
     */
    public CompletableFuture<Job> submit(String type, Object payload) {
        return submit(type, payload, null);
    }
    
    /**
     * Submit a new job with options.
     * 
     * @param type The job type
     * @param payload The job payload
     * @param options Submission options
     * @return CompletableFuture containing the created job
     */
    public CompletableFuture<Job> submit(String type, Object payload, SubmitOptions options) {
        Map<String, Object> body = new HashMap<>();
        body.put("type", type);
        body.put("payload", payload);
        
        if (options != null) {
            if (options.getPriority() != null) {
                body.put("priority", options.getPriority());
            }
            if (options.getDelay() != null) {
                body.put("delay", formatDuration(options.getDelay()));
            }
            if (options.getTimeout() != null) {
                body.put("timeout", formatDuration(options.getTimeout()));
            }
            if (options.getMaxAttempts() != null) {
                body.put("max_attempts", options.getMaxAttempts());
            }
            if (options.getIdempotencyKey() != null) {
                body.put("idempotency_key", options.getIdempotencyKey());
            }
            if (options.getCorrelationId() != null) {
                body.put("correlation_id", options.getCorrelationId());
            }
            if (options.getTags() != null) {
                body.put("tags", options.getTags());
            }
        }
        
        return post("/api/v1/jobs", body, Job.class);
    }
    
    /**
     * Submit multiple jobs in a batch.
     * 
     * @param jobs List of job specifications
     * @param atomic If true, all jobs must succeed or all fail
     * @return CompletableFuture containing the batch result
     */
    public CompletableFuture<BatchResult> submitBatch(List<Map<String, Object>> jobs, boolean atomic) {
        Map<String, Object> body = new HashMap<>();
        body.put("jobs", jobs);
        body.put("atomic", atomic);
        
        return post("/api/v1/jobs/batch", body, BatchResult.class);
    }
    
    /**
     * Get a job by ID.
     * 
     * @param jobId The job ID
     * @return CompletableFuture containing the job
     */
    public CompletableFuture<Job> get(String jobId) {
        return get("/api/v1/jobs/" + jobId, Job.class);
    }
    
    /**
     * Wait for a job to complete using long-polling.
     * 
     * @param jobId The job ID
     * @param timeout Maximum wait time
     * @return CompletableFuture containing the wait result
     */
    public CompletableFuture<WaitResult> wait(String jobId, Duration timeout) {
        Map<String, Object> body = new HashMap<>();
        body.put("timeout", formatDuration(timeout));
        
        // Use a longer timeout for the HTTP request
        return postWithTimeout("/api/v1/jobs/" + jobId + "/wait", body, WaitResult.class, 
            timeout.plusSeconds(5));
    }
    
    /**
     * List jobs with optional filters.
     * 
     * @param state Filter by state (optional)
     * @param type Filter by type (optional)
     * @param limit Maximum results (default 100)
     * @return CompletableFuture containing list of jobs
     */
    public CompletableFuture<List<Job>> list(String state, String type, Integer limit) {
        StringBuilder path = new StringBuilder("/api/v1/jobs?");
        List<String> params = new ArrayList<>();
        
        if (state != null) params.add("state=" + state);
        if (type != null) params.add("type=" + type);
        if (limit != null) params.add("limit=" + limit);
        
        path.append(String.join("&", params));
        
        return get(path.toString(), new TypeReference<Map<String, Object>>() {})
            .thenApply(response -> {
                @SuppressWarnings("unchecked")
                List<Map<String, Object>> jobMaps = (List<Map<String, Object>>) response.get("jobs");
                return objectMapper.convertValue(jobMaps, new TypeReference<List<Job>>() {});
            });
    }
    
    /**
     * Cancel a job.
     * 
     * @param jobId The job ID
     * @param reason Cancellation reason
     * @param force Force cancel running jobs
     * @return CompletableFuture containing the updated job
     */
    public CompletableFuture<Job> cancel(String jobId, String reason, boolean force) {
        Map<String, Object> body = new HashMap<>();
        body.put("reason", reason != null ? reason : "");
        body.put("force", force);
        
        return post("/api/v1/jobs/" + jobId + "/cancel", body, Job.class);
    }
    
    /**
     * Retry a failed job.
     * 
     * @param jobId The job ID
     * @param resetAttempts Reset the attempt counter
     * @return CompletableFuture containing the updated job
     */
    public CompletableFuture<Job> retry(String jobId, boolean resetAttempts) {
        Map<String, Object> body = new HashMap<>();
        body.put("reset_attempts", resetAttempts);
        
        return post("/api/v1/jobs/" + jobId + "/retry", body, Job.class);
    }
    
    /**
     * Delete a job.
     * 
     * @param jobId The job ID
     * @return CompletableFuture that completes when deleted
     */
    public CompletableFuture<Void> delete(String jobId) {
        return CompletableFuture.supplyAsync(() -> {
            Request request = new Request.Builder()
                .url(baseUrl + "/api/v1/jobs/" + jobId)
                .delete()
                .headers(getHeaders())
                .build();
            
            try (Response response = httpClient.newCall(request).execute()) {
                if (response.code() == 404) {
                    throw new JobNotFoundException(jobId);
                }
                if (!response.isSuccessful()) {
                    throw parseError(response);
                }
                return null;
            } catch (IOException e) {
                throw new GopherQueueException("Network error", "network_error", e);
            }
        });
    }
    
    /**
     * Get the result of a completed job.
     * 
     * @param jobId The job ID
     * @return CompletableFuture containing the job result
     */
    public CompletableFuture<JobResult> getResult(String jobId) {
        return get("/api/v1/jobs/" + jobId + "/result", JobResult.class);
    }
    
    /**
     * Get queue statistics.
     * 
     * @return CompletableFuture containing queue stats
     */
    public CompletableFuture<QueueStats> stats() {
        return get("/api/v1/stats", new TypeReference<Map<String, Object>>() {})
            .thenApply(response -> {
                @SuppressWarnings("unchecked")
                Map<String, Object> queue = (Map<String, Object>) response.get("queue");
                return objectMapper.convertValue(queue, QueueStats.class);
            });
    }
    
    @Override
    public void close() {
        httpClient.dispatcher().executorService().shutdown();
        httpClient.connectionPool().evictAll();
    }
    
    // Helper methods
    
    private Headers getHeaders() {
        Headers.Builder builder = new Headers.Builder()
            .add("Content-Type", "application/json");
        
        if (apiKey != null) {
            builder.add("X-API-Key", apiKey);
        }
        
        return builder.build();
    }
    
    private <T> CompletableFuture<T> get(String path, Class<T> responseType) {
        return CompletableFuture.supplyAsync(() -> {
            Request request = new Request.Builder()
                .url(baseUrl + path)
                .get()
                .headers(getHeaders())
                .build();
            
            try (Response response = httpClient.newCall(request).execute()) {
                return handleResponse(response, responseType);
            } catch (IOException e) {
                throw new GopherQueueException("Network error", "network_error", e);
            }
        });
    }
    
    private <T> CompletableFuture<T> get(String path, TypeReference<T> typeRef) {
        return CompletableFuture.supplyAsync(() -> {
            Request request = new Request.Builder()
                .url(baseUrl + path)
                .get()
                .headers(getHeaders())
                .build();
            
            try (Response response = httpClient.newCall(request).execute()) {
                return handleResponse(response, typeRef);
            } catch (IOException e) {
                throw new GopherQueueException("Network error", "network_error", e);
            }
        });
    }
    
    private <T> CompletableFuture<T> post(String path, Object body, Class<T> responseType) {
        return CompletableFuture.supplyAsync(() -> {
            try {
                String json = objectMapper.writeValueAsString(body);
                RequestBody requestBody = RequestBody.create(json, MediaType.get("application/json"));
                
                Request request = new Request.Builder()
                    .url(baseUrl + path)
                    .post(requestBody)
                    .headers(getHeaders())
                    .build();
                
                try (Response response = httpClient.newCall(request).execute()) {
                    return handleResponse(response, responseType);
                }
            } catch (IOException e) {
                throw new GopherQueueException("Network error", "network_error", e);
            }
        });
    }
    
    private <T> CompletableFuture<T> postWithTimeout(String path, Object body, Class<T> responseType, Duration timeout) {
        return CompletableFuture.supplyAsync(() -> {
            try {
                String json = objectMapper.writeValueAsString(body);
                RequestBody requestBody = RequestBody.create(json, MediaType.get("application/json"));
                
                OkHttpClient client = httpClient.newBuilder()
                    .readTimeout(timeout.toMillis(), TimeUnit.MILLISECONDS)
                    .build();
                
                Request request = new Request.Builder()
                    .url(baseUrl + path)
                    .post(requestBody)
                    .headers(getHeaders())
                    .build();
                
                try (Response response = client.newCall(request).execute()) {
                    return handleResponse(response, responseType);
                }
            } catch (IOException e) {
                throw new GopherQueueException("Network error", "network_error", e);
            }
        });
    }
    
    private <T> T handleResponse(Response response, Class<T> responseType) throws IOException {
        if (response.code() == 404) {
            throw new JobNotFoundException("unknown");
        }
        
        ResponseBody responseBody = response.body();
        if (responseBody == null) {
            throw new GopherQueueException("Empty response", "empty_response");
        }
        
        String bodyStr = responseBody.string();
        
        if (!response.isSuccessful()) {
            throw parseError(bodyStr);
        }
        
        return objectMapper.readValue(bodyStr, responseType);
    }
    
    private <T> T handleResponse(Response response, TypeReference<T> typeRef) throws IOException {
        if (response.code() == 404) {
            throw new JobNotFoundException("unknown");
        }
        
        ResponseBody responseBody = response.body();
        if (responseBody == null) {
            throw new GopherQueueException("Empty response", "empty_response");
        }
        
        String bodyStr = responseBody.string();
        
        if (!response.isSuccessful()) {
            throw parseError(bodyStr);
        }
        
        return objectMapper.readValue(bodyStr, typeRef);
    }
    
    private GopherQueueException parseError(Response response) {
        try {
            ResponseBody body = response.body();
            if (body != null) {
                return parseError(body.string());
            }
        } catch (IOException ignored) {
        }
        return new GopherQueueException("Unknown error", "unknown");
    }
    
    private GopherQueueException parseError(String body) {
        try {
            Map<String, Object> errorResponse = objectMapper.readValue(body, new TypeReference<>() {});
            @SuppressWarnings("unchecked")
            Map<String, String> error = (Map<String, String>) errorResponse.get("error");
            if (error != null) {
                return new GopherQueueException(
                    error.getOrDefault("message", "Unknown error"),
                    error.getOrDefault("code", "unknown")
                );
            }
        } catch (IOException ignored) {
        }
        return new GopherQueueException("Unknown error", "unknown");
    }
    
    private String formatDuration(Duration duration) {
        long seconds = duration.getSeconds();
        if (seconds >= 3600 && seconds % 3600 == 0) {
            return (seconds / 3600) + "h";
        }
        if (seconds >= 60 && seconds % 60 == 0) {
            return (seconds / 60) + "m";
        }
        return seconds + "s";
    }
}
