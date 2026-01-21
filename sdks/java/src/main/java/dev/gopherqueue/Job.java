package dev.gopherqueue;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.time.Instant;
import java.util.Map;

/**
 * Represents a job in the GopherQueue system.
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class Job {
    private String id;
    private String type;
    private JobState state;
    private int priority;
    private int attempt;
    
    @JsonProperty("max_attempts")
    private int maxAttempts;
    
    private double progress;
    
    @JsonProperty("progress_message")
    private String progressMessage;
    
    private Map<String, String> tags;
    
    @JsonProperty("created_at")
    private Instant createdAt;
    
    @JsonProperty("updated_at")
    private Instant updatedAt;
    
    @JsonProperty("scheduled_at")
    private Instant scheduledAt;
    
    @JsonProperty("started_at")
    private Instant startedAt;
    
    @JsonProperty("completed_at")
    private Instant completedAt;
    
    @JsonProperty("last_error")
    private String lastError;

    // Getters and setters
    public String getId() { return id; }
    public void setId(String id) { this.id = id; }
    
    public String getType() { return type; }
    public void setType(String type) { this.type = type; }
    
    public JobState getState() { return state; }
    public void setState(JobState state) { this.state = state; }
    
    public int getPriority() { return priority; }
    public void setPriority(int priority) { this.priority = priority; }
    
    public int getAttempt() { return attempt; }
    public void setAttempt(int attempt) { this.attempt = attempt; }
    
    public int getMaxAttempts() { return maxAttempts; }
    public void setMaxAttempts(int maxAttempts) { this.maxAttempts = maxAttempts; }
    
    public double getProgress() { return progress; }
    public void setProgress(double progress) { this.progress = progress; }
    
    public String getProgressMessage() { return progressMessage; }
    public void setProgressMessage(String progressMessage) { this.progressMessage = progressMessage; }
    
    public Map<String, String> getTags() { return tags; }
    public void setTags(Map<String, String> tags) { this.tags = tags; }
    
    public Instant getCreatedAt() { return createdAt; }
    public void setCreatedAt(Instant createdAt) { this.createdAt = createdAt; }
    
    public Instant getUpdatedAt() { return updatedAt; }
    public void setUpdatedAt(Instant updatedAt) { this.updatedAt = updatedAt; }
    
    public Instant getScheduledAt() { return scheduledAt; }
    public void setScheduledAt(Instant scheduledAt) { this.scheduledAt = scheduledAt; }
    
    public Instant getStartedAt() { return startedAt; }
    public void setStartedAt(Instant startedAt) { this.startedAt = startedAt; }
    
    public Instant getCompletedAt() { return completedAt; }
    public void setCompletedAt(Instant completedAt) { this.completedAt = completedAt; }
    
    public String getLastError() { return lastError; }
    public void setLastError(String lastError) { this.lastError = lastError; }
    
    /**
     * Check if the job is in a terminal state.
     */
    public boolean isTerminal() {
        return state == JobState.COMPLETED || 
               state == JobState.FAILED || 
               state == JobState.DEAD_LETTER || 
               state == JobState.CANCELLED;
    }

    @Override
    public String toString() {
        return "Job{id='" + id + "', type='" + type + "', state=" + state + "}";
    }
}
