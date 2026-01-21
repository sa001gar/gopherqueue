package dev.gopherqueue;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;

/**
 * Result of waiting for a job to complete.
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class WaitResult {
    private String id;
    private String state;
    private boolean completed;
    private boolean success;
    private JobResult result;
    
    @JsonProperty("timed_out")
    private boolean timedOut;

    // Getters and setters
    public String getId() { return id; }
    public void setId(String id) { this.id = id; }
    
    public String getState() { return state; }
    public void setState(String state) { this.state = state; }
    
    public boolean isCompleted() { return completed; }
    public void setCompleted(boolean completed) { this.completed = completed; }
    
    public boolean isSuccess() { return success; }
    public void setSuccess(boolean success) { this.success = success; }
    
    public JobResult getResult() { return result; }
    public void setResult(JobResult result) { this.result = result; }
    
    public boolean isTimedOut() { return timedOut; }
    public void setTimedOut(boolean timedOut) { this.timedOut = timedOut; }

    @Override
    public String toString() {
        return "WaitResult{id='" + id + "', completed=" + completed + ", success=" + success + "}";
    }
}
