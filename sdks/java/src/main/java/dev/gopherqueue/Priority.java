package dev.gopherqueue;

/**
 * Job priority levels.
 */
public enum Priority {
    CRITICAL(0),
    HIGH(1),
    NORMAL(2),
    LOW(3),
    BULK(4);

    private final int value;

    Priority(int value) {
        this.value = value;
    }

    public int getValue() {
        return value;
    }
}
