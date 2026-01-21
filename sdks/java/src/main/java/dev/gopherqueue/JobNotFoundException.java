package dev.gopherqueue;

/**
 * Exception thrown when a job is not found.
 */
public class JobNotFoundException extends GopherQueueException {
    private final String jobId;

    public JobNotFoundException(String jobId) {
        super("Job not found: " + jobId, "not_found");
        this.jobId = jobId;
    }

    public String getJobId() {
        return jobId;
    }
}
