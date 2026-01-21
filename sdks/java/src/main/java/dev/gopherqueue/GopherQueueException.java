package dev.gopherqueue;

/**
 * Base exception for GopherQueue errors.
 */
public class GopherQueueException extends RuntimeException {
    private final String code;

    public GopherQueueException(String message, String code) {
        super(message);
        this.code = code;
    }

    public GopherQueueException(String message, String code, Throwable cause) {
        super(message, cause);
        this.code = code;
    }

    public String getCode() {
        return code;
    }
}
