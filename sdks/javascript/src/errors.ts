/**
 * GopherQueue error classes
 */

/**
 * Base error class for GopherQueue errors
 */
export class GopherQueueError extends Error {
  public readonly code: string;

  constructor(message: string, code: string = 'unknown') {
    super(message);
    this.name = 'GopherQueueError';
    this.code = code;
  }
}

/**
 * Job not found error
 */
export class JobNotFoundError extends GopherQueueError {
  public readonly jobId: string;

  constructor(jobId: string) {
    super(`Job not found: ${jobId}`, 'not_found');
    this.name = 'JobNotFoundError';
    this.jobId = jobId;
  }
}

/**
 * Validation error
 */
export class ValidationError extends GopherQueueError {
  constructor(message: string) {
    super(message, 'validation_error');
    this.name = 'ValidationError';
  }
}

/**
 * Authentication error
 */
export class AuthenticationError extends GopherQueueError {
  constructor(message: string = 'Authentication failed') {
    super(message, 'unauthorized');
    this.name = 'AuthenticationError';
  }
}
