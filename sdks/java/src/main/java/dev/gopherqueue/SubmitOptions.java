package dev.gopherqueue;

import java.time.Duration;
import java.util.Map;

/**
 * Options for submitting a job.
 */
public class SubmitOptions {
    private Integer priority;
    private Duration delay;
    private Duration timeout;
    private Integer maxAttempts;
    private String idempotencyKey;
    private String correlationId;
    private Map<String, String> tags;

    private SubmitOptions() {}

    public static Builder builder() {
        return new Builder();
    }

    public Integer getPriority() { return priority; }
    public Duration getDelay() { return delay; }
    public Duration getTimeout() { return timeout; }
    public Integer getMaxAttempts() { return maxAttempts; }
    public String getIdempotencyKey() { return idempotencyKey; }
    public String getCorrelationId() { return correlationId; }
    public Map<String, String> getTags() { return tags; }

    public static class Builder {
        private final SubmitOptions options = new SubmitOptions();

        /**
         * Set the job priority (0=Critical, 1=High, 2=Normal, 3=Low, 4=Bulk).
         */
        public Builder priority(int priority) {
            options.priority = priority;
            return this;
        }

        /**
         * Set the job priority.
         */
        public Builder priority(Priority priority) {
            options.priority = priority.getValue();
            return this;
        }

        /**
         * Set a delay before the job can be executed.
         */
        public Builder delay(Duration delay) {
            options.delay = delay;
            return this;
        }

        /**
         * Set the maximum execution timeout.
         */
        public Builder timeout(Duration timeout) {
            options.timeout = timeout;
            return this;
        }

        /**
         * Set the maximum number of retry attempts.
         */
        public Builder maxAttempts(int maxAttempts) {
            options.maxAttempts = maxAttempts;
            return this;
        }

        /**
         * Set an idempotency key for deduplication.
         */
        public Builder idempotencyKey(String key) {
            options.idempotencyKey = key;
            return this;
        }

        /**
         * Set a correlation ID for tracing.
         */
        public Builder correlationId(String id) {
            options.correlationId = id;
            return this;
        }

        /**
         * Set metadata tags.
         */
        public Builder tags(Map<String, String> tags) {
            options.tags = tags;
            return this;
        }

        public SubmitOptions build() {
            return options;
        }
    }
}
