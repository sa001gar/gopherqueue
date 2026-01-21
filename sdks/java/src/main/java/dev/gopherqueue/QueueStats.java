package dev.gopherqueue;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.time.Instant;

/**
 * Queue statistics.
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class QueueStats {
    private long pending;
    private long scheduled;
    private long running;
    private long delayed;
    private long completed;
    private long failed;
    
    @JsonProperty("dead_letter")
    private long deadLetter;
    
    private long cancelled;
    
    @JsonProperty("total_jobs")
    private long totalJobs;
    
    @JsonProperty("storage_bytes")
    private long storageBytes;
    
    private Instant timestamp;

    // Getters and setters
    public long getPending() { return pending; }
    public void setPending(long pending) { this.pending = pending; }
    
    public long getScheduled() { return scheduled; }
    public void setScheduled(long scheduled) { this.scheduled = scheduled; }
    
    public long getRunning() { return running; }
    public void setRunning(long running) { this.running = running; }
    
    public long getDelayed() { return delayed; }
    public void setDelayed(long delayed) { this.delayed = delayed; }
    
    public long getCompleted() { return completed; }
    public void setCompleted(long completed) { this.completed = completed; }
    
    public long getFailed() { return failed; }
    public void setFailed(long failed) { this.failed = failed; }
    
    public long getDeadLetter() { return deadLetter; }
    public void setDeadLetter(long deadLetter) { this.deadLetter = deadLetter; }
    
    public long getCancelled() { return cancelled; }
    public void setCancelled(long cancelled) { this.cancelled = cancelled; }
    
    public long getTotalJobs() { return totalJobs; }
    public void setTotalJobs(long totalJobs) { this.totalJobs = totalJobs; }
    
    public long getStorageBytes() { return storageBytes; }
    public void setStorageBytes(long storageBytes) { this.storageBytes = storageBytes; }
    
    public Instant getTimestamp() { return timestamp; }
    public void setTimestamp(Instant timestamp) { this.timestamp = timestamp; }

    @Override
    public String toString() {
        return "QueueStats{pending=" + pending + ", running=" + running + ", completed=" + completed + "}";
    }
}
