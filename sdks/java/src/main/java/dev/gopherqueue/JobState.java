package dev.gopherqueue;

import com.fasterxml.jackson.annotation.JsonValue;

/**
 * Job lifecycle states.
 */
public enum JobState {
    PENDING("pending"),
    SCHEDULED("scheduled"),
    RUNNING("running"),
    RETRYING("retrying"),
    DELAYED("delayed"),
    COMPLETED("completed"),
    FAILED("failed"),
    DEAD_LETTER("dead_letter"),
    CANCELLED("cancelled");

    private final String value;

    JobState(String value) {
        this.value = value;
    }

    @JsonValue
    public String getValue() {
        return value;
    }

    public static JobState fromValue(String value) {
        for (JobState state : values()) {
            if (state.value.equals(value)) {
                return state;
            }
        }
        throw new IllegalArgumentException("Unknown job state: " + value);
    }
}
