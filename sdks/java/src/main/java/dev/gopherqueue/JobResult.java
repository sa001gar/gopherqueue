package dev.gopherqueue;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.time.Instant;

/**
 * Result of a completed job.
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class JobResult {
    @JsonProperty("job_id")
    private String jobId;
    
    private boolean success;
    private byte[] output;
    private String error;
    
    @JsonProperty("error_category")
    private String errorCategory;
    
    private long duration;
    
    @JsonProperty("completed_at")
    private Instant completedAt;

    // Getters and setters
    public String getJobId() { return jobId; }
    public void setJobId(String jobId) { this.jobId = jobId; }
    
    public boolean isSuccess() { return success; }
    public void setSuccess(boolean success) { this.success = success; }
    
    public byte[] getOutput() { return output; }
    public void setOutput(byte[] output) { this.output = output; }
    
    public String getError() { return error; }
    public void setError(String error) { this.error = error; }
    
    public String getErrorCategory() { return errorCategory; }
    public void setErrorCategory(String errorCategory) { this.errorCategory = errorCategory; }
    
    public long getDuration() { return duration; }
    public void setDuration(long duration) { this.duration = duration; }
    
    public Instant getCompletedAt() { return completedAt; }
    public void setCompletedAt(Instant completedAt) { this.completedAt = completedAt; }

    @Override
    public String toString() {
        return "JobResult{jobId='" + jobId + "', success=" + success + "}";
    }
}
