"""
GopherQueue Python client implementation.
"""

import asyncio
import json
from typing import Any, AsyncGenerator, Dict, List, Optional, Union

import aiohttp
import httpx

from gopherqueue.models import (
    BatchResult,
    GopherQueueError,
    Job,
    JobNotFoundError,
    JobResult,
    QueueStats,
    SSEEvent,
    WaitResult,
)


class GopherQueue:
    """
    Async client for GopherQueue.
    
    Example:
        async with GopherQueue("http://localhost:8080") as client:
            job = await client.submit("email", {"to": "user@example.com"})
            result = await client.wait(job.id, timeout=30)
    """

    def __init__(
        self,
        url: str,
        api_key: Optional[str] = None,
        timeout: float = 30.0,
        max_connections: int = 100,
    ):
        """
        Initialize the GopherQueue client.
        
        Args:
            url: Base URL of the GopherQueue server (e.g., "http://localhost:8080")
            api_key: Optional API key for authentication
            timeout: Request timeout in seconds
            max_connections: Maximum number of concurrent connections
        """
        self.url = url.rstrip("/")
        self.api_key = api_key
        self.timeout = timeout
        self._session: Optional[aiohttp.ClientSession] = None
        self._max_connections = max_connections

    async def _get_session(self) -> aiohttp.ClientSession:
        """Get or create the aiohttp session."""
        if self._session is None or self._session.closed:
            connector = aiohttp.TCPConnector(limit=self._max_connections)
            timeout = aiohttp.ClientTimeout(total=self.timeout)
            self._session = aiohttp.ClientSession(
                connector=connector,
                timeout=timeout,
                headers=self._get_headers(),
            )
        return self._session

    def _get_headers(self) -> Dict[str, str]:
        """Get request headers."""
        headers = {"Content-Type": "application/json"}
        if self.api_key:
            headers["X-API-Key"] = self.api_key
        return headers

    async def close(self) -> None:
        """Close the client session."""
        if self._session and not self._session.closed:
            await self._session.close()

    async def __aenter__(self) -> "GopherQueue":
        return self

    async def __aexit__(self, *args: Any) -> None:
        await self.close()

    async def _request(
        self,
        method: str,
        path: str,
        data: Optional[Dict[str, Any]] = None,
        params: Optional[Dict[str, str]] = None,
    ) -> Dict[str, Any]:
        """Make an HTTP request to the API."""
        session = await self._get_session()
        url = f"{self.url}{path}"
        
        async with session.request(
            method,
            url,
            json=data,
            params=params,
        ) as response:
            body = await response.json()
            
            if response.status >= 400:
                error = body.get("error", {})
                code = error.get("code", "unknown")
                message = error.get("message", "Unknown error")
                
                if code == "not_found":
                    raise JobNotFoundError(path.split("/")[-1])
                raise GopherQueueError(message, code)
            
            return body

    async def submit(
        self,
        job_type: str,
        payload: Any,
        *,
        priority: Optional[int] = None,
        delay: Optional[str] = None,
        timeout: Optional[str] = None,
        max_attempts: Optional[int] = None,
        idempotency_key: Optional[str] = None,
        correlation_id: Optional[str] = None,
        tags: Optional[Dict[str, str]] = None,
    ) -> Job:
        """
        Submit a new job.
        
        Args:
            job_type: The type of job (must match a registered handler)
            payload: Job payload (will be JSON serialized)
            priority: Priority level (0=Critical, 1=High, 2=Normal, 3=Low, 4=Bulk)
            delay: Delay before execution (e.g., "5m", "1h")
            timeout: Maximum execution time (e.g., "30m")
            max_attempts: Maximum retry attempts
            idempotency_key: Key for deduplication
            correlation_id: Tracing correlation ID
            tags: Metadata tags
            
        Returns:
            The created Job
        """
        data: Dict[str, Any] = {
            "type": job_type,
            "payload": payload,
        }
        
        if priority is not None:
            data["priority"] = priority
        if delay:
            data["delay"] = delay
        if timeout:
            data["timeout"] = timeout
        if max_attempts is not None:
            data["max_attempts"] = max_attempts
        if idempotency_key:
            data["idempotency_key"] = idempotency_key
        if correlation_id:
            data["correlation_id"] = correlation_id
        if tags:
            data["tags"] = tags

        response = await self._request("POST", "/api/v1/jobs", data=data)
        return Job.from_dict(response)

    async def submit_batch(
        self,
        jobs: List[Dict[str, Any]],
        atomic: bool = False,
    ) -> BatchResult:
        """
        Submit multiple jobs in a batch.
        
        Args:
            jobs: List of job specifications (each with 'type' and 'payload')
            atomic: If True, all jobs must succeed or all fail
            
        Returns:
            BatchResult with submission results
        """
        data = {"jobs": jobs, "atomic": atomic}
        response = await self._request("POST", "/api/v1/jobs/batch", data=data)
        return BatchResult.from_dict(response)

    async def get(self, job_id: str) -> Job:
        """
        Get a job by ID.
        
        Args:
            job_id: The job ID
            
        Returns:
            The Job
            
        Raises:
            JobNotFoundError: If the job doesn't exist
        """
        response = await self._request("GET", f"/api/v1/jobs/{job_id}")
        return Job.from_dict(response)

    async def wait(
        self,
        job_id: str,
        timeout: int = 30,
    ) -> WaitResult:
        """
        Wait for a job to complete using long-polling.
        
        Args:
            job_id: The job ID
            timeout: Maximum wait time in seconds
            
        Returns:
            WaitResult with completion status
        """
        # Use a longer timeout for this request
        session = await self._get_session()
        url = f"{self.url}/api/v1/jobs/{job_id}/wait"
        
        async with session.post(
            url,
            json={"timeout": f"{timeout}s"},
            timeout=aiohttp.ClientTimeout(total=timeout + 5),
        ) as response:
            body = await response.json()
            if response.status >= 400:
                error = body.get("error", {})
                raise GopherQueueError(error.get("message", "Unknown error"))
            return WaitResult.from_dict(body)

    async def list(
        self,
        *,
        state: Optional[str] = None,
        type: Optional[str] = None,
        limit: int = 100,
    ) -> List[Job]:
        """
        List jobs with optional filters.
        
        Args:
            state: Filter by job state
            type: Filter by job type
            limit: Maximum number of jobs to return
            
        Returns:
            List of Jobs
        """
        params: Dict[str, str] = {"limit": str(limit)}
        if state:
            params["state"] = state
        if type:
            params["type"] = type

        response = await self._request("GET", "/api/v1/jobs", params=params)
        return [Job.from_dict(j) for j in response.get("jobs", [])]

    async def cancel(
        self,
        job_id: str,
        reason: str = "",
        force: bool = False,
    ) -> Job:
        """
        Cancel a job.
        
        Args:
            job_id: The job ID
            reason: Cancellation reason
            force: Force cancel running jobs
            
        Returns:
            The updated Job
        """
        data = {"reason": reason, "force": force}
        response = await self._request("POST", f"/api/v1/jobs/{job_id}/cancel", data=data)
        return Job.from_dict(response)

    async def retry(
        self,
        job_id: str,
        reset_attempts: bool = False,
    ) -> Job:
        """
        Retry a failed job.
        
        Args:
            job_id: The job ID
            reset_attempts: Reset the attempt counter
            
        Returns:
            The updated Job
        """
        data = {"reset_attempts": reset_attempts}
        response = await self._request("POST", f"/api/v1/jobs/{job_id}/retry", data=data)
        return Job.from_dict(response)

    async def delete(self, job_id: str) -> None:
        """
        Delete a job.
        
        Args:
            job_id: The job ID
        """
        session = await self._get_session()
        url = f"{self.url}/api/v1/jobs/{job_id}"
        
        async with session.delete(url) as response:
            if response.status == 404:
                raise JobNotFoundError(job_id)
            if response.status >= 400:
                body = await response.json()
                error = body.get("error", {})
                raise GopherQueueError(error.get("message", "Unknown error"))

    async def get_result(self, job_id: str) -> JobResult:
        """
        Get the result of a completed job.
        
        Args:
            job_id: The job ID
            
        Returns:
            The JobResult
        """
        response = await self._request("GET", f"/api/v1/jobs/{job_id}/result")
        return JobResult.from_dict(response)

    async def stats(self) -> QueueStats:
        """
        Get queue statistics.
        
        Returns:
            QueueStats with current queue state
        """
        response = await self._request("GET", "/api/v1/stats")
        return QueueStats.from_dict(response.get("queue", {}))

    async def events(
        self,
        job_id: str = "*",
    ) -> AsyncGenerator[SSEEvent, None]:
        """
        Subscribe to real-time job events via SSE.
        
        Args:
            job_id: Filter events by job ID, or "*" for all jobs
            
        Yields:
            SSEEvent for each job update
        """
        session = await self._get_session()
        params = {"job_id": job_id} if job_id != "*" else {}
        url = f"{self.url}/api/v1/events"
        
        async with session.get(url, params=params) as response:
            async for line in response.content:
                line = line.decode("utf-8").strip()
                if not line or line.startswith(":"):
                    continue
                
                if line.startswith("event:"):
                    event_type = line[6:].strip()
                elif line.startswith("data:"):
                    data = json.loads(line[5:].strip())
                    yield SSEEvent(event=event_type, data=data)


class GopherQueueSync:
    """
    Synchronous client for GopherQueue.
    
    This is a convenience wrapper around the async client for use in
    synchronous code.
    
    Example:
        client = GopherQueueSync("http://localhost:8080")
        job = client.submit("email", {"to": "user@example.com"})
        result = client.wait(job.id, timeout=30)
    """

    def __init__(
        self,
        url: str,
        api_key: Optional[str] = None,
        timeout: float = 30.0,
    ):
        """
        Initialize the synchronous GopherQueue client.
        
        Args:
            url: Base URL of the GopherQueue server
            api_key: Optional API key for authentication
            timeout: Request timeout in seconds
        """
        self.url = url.rstrip("/")
        self.api_key = api_key
        self.timeout = timeout
        self._client = httpx.Client(
            base_url=self.url,
            headers=self._get_headers(),
            timeout=timeout,
        )

    def _get_headers(self) -> Dict[str, str]:
        """Get request headers."""
        headers = {"Content-Type": "application/json"}
        if self.api_key:
            headers["X-API-Key"] = self.api_key
        return headers

    def close(self) -> None:
        """Close the client."""
        self._client.close()

    def __enter__(self) -> "GopherQueueSync":
        return self

    def __exit__(self, *args: Any) -> None:
        self.close()

    def _request(
        self,
        method: str,
        path: str,
        data: Optional[Dict[str, Any]] = None,
        params: Optional[Dict[str, str]] = None,
        timeout: Optional[float] = None,
    ) -> Dict[str, Any]:
        """Make an HTTP request to the API."""
        response = self._client.request(
            method,
            path,
            json=data,
            params=params,
            timeout=timeout or self.timeout,
        )
        
        body = response.json()
        
        if response.status_code >= 400:
            error = body.get("error", {})
            code = error.get("code", "unknown")
            message = error.get("message", "Unknown error")
            
            if code == "not_found":
                raise JobNotFoundError(path.split("/")[-1])
            raise GopherQueueError(message, code)
        
        return body

    def submit(
        self,
        job_type: str,
        payload: Any,
        **kwargs: Any,
    ) -> Job:
        """Submit a new job. See GopherQueue.submit for full documentation."""
        data: Dict[str, Any] = {"type": job_type, "payload": payload}
        data.update({k: v for k, v in kwargs.items() if v is not None})
        response = self._request("POST", "/api/v1/jobs", data=data)
        return Job.from_dict(response)

    def submit_batch(
        self,
        jobs: List[Dict[str, Any]],
        atomic: bool = False,
    ) -> BatchResult:
        """Submit multiple jobs. See GopherQueue.submit_batch for full documentation."""
        data = {"jobs": jobs, "atomic": atomic}
        response = self._request("POST", "/api/v1/jobs/batch", data=data)
        return BatchResult.from_dict(response)

    def get(self, job_id: str) -> Job:
        """Get a job by ID. See GopherQueue.get for full documentation."""
        response = self._request("GET", f"/api/v1/jobs/{job_id}")
        return Job.from_dict(response)

    def wait(self, job_id: str, timeout: int = 30) -> WaitResult:
        """Wait for job completion. See GopherQueue.wait for full documentation."""
        response = self._request(
            "POST",
            f"/api/v1/jobs/{job_id}/wait",
            data={"timeout": f"{timeout}s"},
            timeout=float(timeout + 5),
        )
        return WaitResult.from_dict(response)

    def list(self, **kwargs: Any) -> List[Job]:
        """List jobs. See GopherQueue.list for full documentation."""
        params = {k: str(v) for k, v in kwargs.items() if v is not None}
        response = self._request("GET", "/api/v1/jobs", params=params)
        return [Job.from_dict(j) for j in response.get("jobs", [])]

    def cancel(self, job_id: str, reason: str = "", force: bool = False) -> Job:
        """Cancel a job. See GopherQueue.cancel for full documentation."""
        response = self._request(
            "POST", f"/api/v1/jobs/{job_id}/cancel", data={"reason": reason, "force": force}
        )
        return Job.from_dict(response)

    def retry(self, job_id: str, reset_attempts: bool = False) -> Job:
        """Retry a job. See GopherQueue.retry for full documentation."""
        response = self._request(
            "POST", f"/api/v1/jobs/{job_id}/retry", data={"reset_attempts": reset_attempts}
        )
        return Job.from_dict(response)

    def delete(self, job_id: str) -> None:
        """Delete a job. See GopherQueue.delete for full documentation."""
        response = self._client.delete(f"/api/v1/jobs/{job_id}")
        if response.status_code == 404:
            raise JobNotFoundError(job_id)
        if response.status_code >= 400:
            body = response.json()
            error = body.get("error", {})
            raise GopherQueueError(error.get("message", "Unknown error"))

    def get_result(self, job_id: str) -> JobResult:
        """Get job result. See GopherQueue.get_result for full documentation."""
        response = self._request("GET", f"/api/v1/jobs/{job_id}/result")
        return JobResult.from_dict(response)

    def stats(self) -> QueueStats:
        """Get queue stats. See GopherQueue.stats for full documentation."""
        response = self._request("GET", "/api/v1/stats")
        return QueueStats.from_dict(response.get("queue", {}))
