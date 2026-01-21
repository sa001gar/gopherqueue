/**
 * GopherQueue client implementation
 */

import {
  Job,
  JobResult,
  BatchJobSpec,
  BatchResult,
  WaitResult,
  QueueStats,
  SSEEvent,
  ClientOptions,
  SubmitOptions,
  ListOptions,
  ApiError,
} from './types';

import {
  GopherQueueError,
  JobNotFoundError,
  AuthenticationError,
} from './errors';

/**
 * GopherQueue client for TypeScript/JavaScript
 * 
 * @example
 * ```typescript
 * const client = new GopherQueue('http://localhost:8080');
 * 
 * // Submit a job
 * const job = await client.submit('email', { to: 'user@example.com' });
 * 
 * // Wait for completion
 * const result = await client.wait(job.id, { timeout: 30000 });
 * console.log(result.success);
 * ```
 */
export class GopherQueue {
  private readonly baseUrl: string;
  private readonly apiKey?: string;
  private readonly timeout: number;
  private readonly fetchFn: typeof fetch;

  /**
   * Create a new GopherQueue client
   * 
   * @param url - Base URL of the GopherQueue server (e.g., "http://localhost:8080")
   * @param options - Client configuration options
   */
  constructor(url: string, options: ClientOptions = {}) {
    this.baseUrl = url.replace(/\/$/, '');
    this.apiKey = options.apiKey;
    this.timeout = options.timeout ?? 30000;
    this.fetchFn = options.fetch ?? fetch;
  }

  /**
   * Get request headers
   */
  private getHeaders(): Record<string, string> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };
    if (this.apiKey) {
      headers['X-API-Key'] = this.apiKey;
    }
    return headers;
  }

  /**
   * Make an HTTP request
   */
  private async request<T>(
    method: string,
    path: string,
    body?: unknown,
    options: { timeout?: number } = {}
  ): Promise<T> {
    const url = `${this.baseUrl}${path}`;
    const timeout = options.timeout ?? this.timeout;

    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), timeout);

    try {
      const response = await this.fetchFn(url, {
        method,
        headers: this.getHeaders(),
        body: body ? JSON.stringify(body) : undefined,
        signal: controller.signal,
      });

      clearTimeout(timeoutId);

      const data = await response.json();

      if (!response.ok) {
        const apiError = data as ApiError;
        const code = apiError.error?.code ?? 'unknown';
        const message = apiError.error?.message ?? 'Unknown error';

        if (code === 'not_found') {
          throw new JobNotFoundError(path.split('/').pop() ?? '');
        }
        if (code === 'unauthorized') {
          throw new AuthenticationError(message);
        }
        throw new GopherQueueError(message, code);
      }

      return data as T;
    } catch (error) {
      clearTimeout(timeoutId);
      if (error instanceof GopherQueueError) {
        throw error;
      }
      if (error instanceof Error && error.name === 'AbortError') {
        throw new GopherQueueError('Request timeout', 'timeout');
      }
      throw new GopherQueueError(String(error), 'network_error');
    }
  }

  /**
   * Submit a new job
   * 
   * @param type - The type of job (must match a registered handler)
   * @param payload - Job payload
   * @param options - Submission options
   * @returns The created job
   */
  async submit(type: string, payload: unknown, options: SubmitOptions = {}): Promise<Job> {
    const body: Record<string, unknown> = { type, payload };

    if (options.priority !== undefined) body.priority = options.priority;
    if (options.delay) body.delay = options.delay;
    if (options.timeout) body.timeout = options.timeout;
    if (options.maxAttempts !== undefined) body.max_attempts = options.maxAttempts;
    if (options.idempotencyKey) body.idempotency_key = options.idempotencyKey;
    if (options.correlationId) body.correlation_id = options.correlationId;
    if (options.tags) body.tags = options.tags;

    return this.request<Job>('POST', '/api/v1/jobs', body);
  }

  /**
   * Submit multiple jobs in a batch
   * 
   * @param jobs - Array of job specifications
   * @param options - Batch options
   * @returns Batch submission result
   */
  async submitBatch(
    jobs: BatchJobSpec[],
    options: { atomic?: boolean } = {}
  ): Promise<BatchResult> {
    return this.request<BatchResult>('POST', '/api/v1/jobs/batch', {
      jobs,
      atomic: options.atomic ?? false,
    });
  }

  /**
   * Get a job by ID
   * 
   * @param jobId - The job ID
   * @returns The job
   */
  async get(jobId: string): Promise<Job> {
    return this.request<Job>('GET', `/api/v1/jobs/${jobId}`);
  }

  /**
   * Wait for a job to complete using long-polling
   * 
   * @param jobId - The job ID
   * @param options - Wait options
   * @returns Wait result with completion status
   */
  async wait(
    jobId: string,
    options: { timeout?: number } = {}
  ): Promise<WaitResult> {
    const timeout = options.timeout ?? 30000;
    const timeoutStr = `${Math.floor(timeout / 1000)}s`;

    return this.request<WaitResult>(
      'POST',
      `/api/v1/jobs/${jobId}/wait`,
      { timeout: timeoutStr },
      { timeout: timeout + 5000 }
    );
  }

  /**
   * List jobs with optional filters
   * 
   * @param options - Filter options
   * @returns Array of jobs
   */
  async list(options: ListOptions = {}): Promise<Job[]> {
    const params = new URLSearchParams();
    if (options.state) params.append('state', options.state);
    if (options.type) params.append('type', options.type);
    if (options.limit) params.append('limit', String(options.limit));

    const query = params.toString();
    const path = query ? `/api/v1/jobs?${query}` : '/api/v1/jobs';
    const response = await this.request<{ jobs: Job[] }>('GET', path);
    return response.jobs;
  }

  /**
   * Cancel a job
   * 
   * @param jobId - The job ID
   * @param options - Cancel options
   * @returns The updated job
   */
  async cancel(
    jobId: string,
    options: { reason?: string; force?: boolean } = {}
  ): Promise<Job> {
    return this.request<Job>('POST', `/api/v1/jobs/${jobId}/cancel`, {
      reason: options.reason ?? '',
      force: options.force ?? false,
    });
  }

  /**
   * Retry a failed job
   * 
   * @param jobId - The job ID
   * @param options - Retry options
   * @returns The updated job
   */
  async retry(
    jobId: string,
    options: { resetAttempts?: boolean } = {}
  ): Promise<Job> {
    return this.request<Job>('POST', `/api/v1/jobs/${jobId}/retry`, {
      reset_attempts: options.resetAttempts ?? false,
    });
  }

  /**
   * Delete a job
   * 
   * @param jobId - The job ID
   */
  async delete(jobId: string): Promise<void> {
    const url = `${this.baseUrl}/api/v1/jobs/${jobId}`;
    const response = await this.fetchFn(url, {
      method: 'DELETE',
      headers: this.getHeaders(),
    });

    if (response.status === 404) {
      throw new JobNotFoundError(jobId);
    }
    if (!response.ok) {
      const data = await response.json() as ApiError;
      throw new GopherQueueError(data.error?.message ?? 'Unknown error');
    }
  }

  /**
   * Get the result of a completed job
   * 
   * @param jobId - The job ID
   * @returns The job result
   */
  async getResult(jobId: string): Promise<JobResult> {
    return this.request<JobResult>('GET', `/api/v1/jobs/${jobId}/result`);
  }

  /**
   * Get queue statistics
   * 
   * @returns Queue statistics
   */
  async stats(): Promise<QueueStats> {
    const response = await this.request<{ queue: QueueStats }>('GET', '/api/v1/stats');
    return response.queue;
  }

  /**
   * Subscribe to real-time job events via SSE
   * 
   * @param jobId - Filter events by job ID, or "*" for all jobs
   * @returns AsyncGenerator of SSE events
   */
  async *events(jobId: string = '*'): AsyncGenerator<SSEEvent, void, undefined> {
    const params = jobId !== '*' ? `?job_id=${jobId}` : '';
    const url = `${this.baseUrl}/api/v1/events${params}`;

    const response = await this.fetchFn(url, {
      headers: this.getHeaders(),
    });

    if (!response.ok) {
      throw new GopherQueueError('Failed to connect to events stream');
    }

    const reader = response.body?.getReader();
    if (!reader) {
      throw new GopherQueueError('Streaming not supported');
    }

    const decoder = new TextDecoder();
    let buffer = '';
    let currentEvent = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() ?? '';

      for (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed || trimmed.startsWith(':')) continue;

        if (trimmed.startsWith('event:')) {
          currentEvent = trimmed.slice(6).trim();
        } else if (trimmed.startsWith('data:')) {
          const data = JSON.parse(trimmed.slice(5).trim());
          yield { event: currentEvent, data };
        }
      }
    }
  }
}
