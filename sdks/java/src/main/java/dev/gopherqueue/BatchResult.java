package dev.gopherqueue;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.util.List;

/**
 * Result of a batch submission.
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class BatchResult {
    private int total;
    private int accepted;
    private int rejected;
    private List<BatchJobResult> results;

    public int getTotal() { return total; }
    public void setTotal(int total) { this.total = total; }
    
    public int getAccepted() { return accepted; }
    public void setAccepted(int accepted) { this.accepted = accepted; }
    
    public int getRejected() { return rejected; }
    public void setRejected(int rejected) { this.rejected = rejected; }
    
    public List<BatchJobResult> getResults() { return results; }
    public void setResults(List<BatchJobResult> results) { this.results = results; }

    @Override
    public String toString() {
        return "BatchResult{total=" + total + ", accepted=" + accepted + ", rejected=" + rejected + "}";
    }

    @JsonIgnoreProperties(ignoreUnknown = true)
    public static class BatchJobResult {
        private int index;
        private boolean success;
        private Job job;
        private String error;

        public int getIndex() { return index; }
        public void setIndex(int index) { this.index = index; }
        
        public boolean isSuccess() { return success; }
        public void setSuccess(boolean success) { this.success = success; }
        
        public Job getJob() { return job; }
        public void setJob(Job job) { this.job = job; }
        
        public String getError() { return error; }
        public void setError(String error) { this.error = error; }
    }
}
