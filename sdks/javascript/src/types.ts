/**
 * Type definitions for GopherQueue SDK
 */

/**
 * Job lifecycle states
 */
export type JobState =
  | 'pending'
  | 'scheduled'
  | 'running'
  | 'retrying'
  | 'delayed'
  | 'completed'
  | 'failed'
  | 'dead_letter'
  | 'cancelled';

/**
 * Job priority levels
 */
export enum Priority {
  Critical = 0,
  High = 1,
  Normal = 2,
  Low = 3,
  Bulk = 4,
}

/**
 * Job representation
 */
export interface Job {
  id: string;
  type: string;
  state: JobState;
  priority: number;
  attempt: number;
  max_attempts: number;
  progress: number;
  progress_message?: string;
  tags?: Record<string, string>;
  created_at: string;
  updated_at: string;
  scheduled_at?: string;
  started_at?: string;
  completed_at?: string;
  last_error?: string;
}

/**
 * Job result
 */
export interface JobResult {
  job_id: string;
  success: boolean;
  output?: unknown;
  error?: string;
  error_category?: string;
  duration: number;
  completed_at?: string;
}

/**
 * Job submission options
 */
export interface SubmitOptions {
  /** Priority level (0=Critical, 1=High, 2=Normal, 3=Low, 4=Bulk) */
  priority?: number;
  /** Delay before execution (e.g., "5m", "1h") */
  delay?: string;
  /** Maximum execution time (e.g., "30m") */
  timeout?: string;
  /** Maximum retry attempts */
  maxAttempts?: number;
  /** Key for deduplication */
  idempotencyKey?: string;
  /** Tracing correlation ID */
  correlationId?: string;
  /** Metadata tags */
  tags?: Record<string, string>;
}

/**
 * Batch job specification
 */
export interface BatchJobSpec {
  type: string;
  payload: unknown;
  priority?: number;
  delay?: string;
  timeout?: string;
  max_attempts?: number;
  idempotency_key?: string;
  correlation_id?: string;
  tags?: Record<string, string>;
}

/**
 * Batch submission result
 */
export interface BatchResult {
  total: number;
  accepted: number;
  rejected: number;
  results: BatchJobResult[];
}

/**
 * Individual job result in batch
 */
export interface BatchJobResult {
  index: number;
  success: boolean;
  job?: Job;
  error?: string;
}

/**
 * Wait result
 */
export interface WaitResult {
  id: string;
  state: JobState;
  completed: boolean;
  success?: boolean;
  result?: JobResult;
  timed_out?: boolean;
}

/**
 * Queue statistics
 */
export interface QueueStats {
  pending: number;
  scheduled: number;
  running: number;
  delayed: number;
  completed: number;
  failed: number;
  dead_letter: number;
  cancelled: number;
  total_jobs: number;
  storage_bytes: number;
  timestamp: string;
}

/**
 * Server-Sent Event
 */
export interface SSEEvent {
  event: string;
  data: unknown;
  id?: string;
}

/**
 * Client configuration options
 */
export interface ClientOptions {
  /** API key for authentication */
  apiKey?: string;
  /** Request timeout in milliseconds */
  timeout?: number;
  /** Custom fetch implementation */
  fetch?: typeof fetch;
}

/**
 * List options
 */
export interface ListOptions {
  /** Filter by job state */
  state?: JobState;
  /** Filter by job type */
  type?: string;
  /** Maximum number of jobs to return */
  limit?: number;
}

/**
 * API error response
 */
export interface ApiError {
  error: {
    code: string;
    message: string;
  };
}
