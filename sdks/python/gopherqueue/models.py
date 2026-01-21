"""
Data models for the GopherQueue Python SDK.
"""

from dataclasses import dataclass, field
from datetime import datetime
from enum import Enum
from typing import Any, Dict, List, Optional


class JobState(str, Enum):
    """Job lifecycle states."""
    PENDING = "pending"
    SCHEDULED = "scheduled"
    RUNNING = "running"
    RETRYING = "retrying"
    DELAYED = "delayed"
    COMPLETED = "completed"
    FAILED = "failed"
    DEAD_LETTER = "dead_letter"
    CANCELLED = "cancelled"


class Priority(int, Enum):
    """Job priority levels."""
    CRITICAL = 0
    HIGH = 1
    NORMAL = 2
    LOW = 3
    BULK = 4


@dataclass
class Job:
    """Represents a job in the queue."""
    id: str
    type: str
    state: JobState
    priority: int
    attempt: int
    max_attempts: int
    created_at: datetime
    updated_at: datetime
    progress: float = 0.0
    progress_message: str = ""
    tags: Dict[str, str] = field(default_factory=dict)
    scheduled_at: Optional[datetime] = None
    started_at: Optional[datetime] = None
    completed_at: Optional[datetime] = None
    last_error: Optional[str] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Job":
        """Create a Job from an API response dictionary."""
        return cls(
            id=data["id"],
            type=data["type"],
            state=JobState(data["state"]),
            priority=data["priority"],
            attempt=data["attempt"],
            max_attempts=data["max_attempts"],
            created_at=datetime.fromisoformat(data["created_at"].replace("Z", "+00:00")),
            updated_at=datetime.fromisoformat(data["updated_at"].replace("Z", "+00:00")),
            progress=data.get("progress", 0.0),
            progress_message=data.get("progress_message", ""),
            tags=data.get("tags", {}),
            scheduled_at=datetime.fromisoformat(data["scheduled_at"].replace("Z", "+00:00")) if data.get("scheduled_at") else None,
            started_at=datetime.fromisoformat(data["started_at"].replace("Z", "+00:00")) if data.get("started_at") else None,
            completed_at=datetime.fromisoformat(data["completed_at"].replace("Z", "+00:00")) if data.get("completed_at") else None,
            last_error=data.get("last_error"),
        )


@dataclass
class JobResult:
    """Result of a completed job."""
    job_id: str
    success: bool
    output: Optional[bytes] = None
    error: Optional[str] = None
    error_category: Optional[str] = None
    duration_ns: int = 0
    completed_at: Optional[datetime] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "JobResult":
        """Create a JobResult from an API response dictionary."""
        return cls(
            job_id=data["job_id"],
            success=data["success"],
            output=data.get("output"),
            error=data.get("error"),
            error_category=data.get("error_category"),
            duration_ns=data.get("duration", 0),
            completed_at=datetime.fromisoformat(data["completed_at"].replace("Z", "+00:00")) if data.get("completed_at") else None,
        )


@dataclass
class BatchResult:
    """Result of a batch submission."""
    total: int
    accepted: int
    rejected: int
    results: List[Dict[str, Any]] = field(default_factory=list)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "BatchResult":
        """Create a BatchResult from an API response dictionary."""
        return cls(
            total=data["total"],
            accepted=data["accepted"],
            rejected=data["rejected"],
            results=data.get("results", []),
        )


@dataclass
class WaitResult:
    """Result of waiting for a job."""
    id: str
    state: JobState
    completed: bool
    success: bool = False
    result: Optional[JobResult] = None
    timed_out: bool = False

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "WaitResult":
        """Create a WaitResult from an API response dictionary."""
        result = None
        if data.get("result"):
            result = JobResult.from_dict(data["result"])
        
        return cls(
            id=data["id"],
            state=JobState(data["state"]),
            completed=data["completed"],
            success=data.get("success", False),
            result=result,
            timed_out=data.get("timed_out", False),
        )


@dataclass
class SSEEvent:
    """Server-Sent Event."""
    event: str
    data: Any
    id: Optional[str] = None


@dataclass
class QueueStats:
    """Queue statistics."""
    pending: int = 0
    scheduled: int = 0
    running: int = 0
    delayed: int = 0
    completed: int = 0
    failed: int = 0
    dead_letter: int = 0
    cancelled: int = 0
    total_jobs: int = 0
    storage_bytes: int = 0
    timestamp: Optional[datetime] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "QueueStats":
        """Create QueueStats from an API response dictionary."""
        return cls(
            pending=data.get("pending", 0),
            scheduled=data.get("scheduled", 0),
            running=data.get("running", 0),
            delayed=data.get("delayed", 0),
            completed=data.get("completed", 0),
            failed=data.get("failed", 0),
            dead_letter=data.get("dead_letter", 0),
            cancelled=data.get("cancelled", 0),
            total_jobs=data.get("total_jobs", 0),
            storage_bytes=data.get("storage_bytes", 0),
            timestamp=datetime.fromisoformat(data["timestamp"].replace("Z", "+00:00")) if data.get("timestamp") else None,
        )


class GopherQueueError(Exception):
    """Base exception for GopherQueue errors."""
    def __init__(self, message: str, code: str = "unknown"):
        self.message = message
        self.code = code
        super().__init__(message)


class JobNotFoundError(GopherQueueError):
    """Job not found."""
    def __init__(self, job_id: str):
        super().__init__(f"Job not found: {job_id}", "not_found")
        self.job_id = job_id


class ValidationError(GopherQueueError):
    """Validation error."""
    def __init__(self, message: str):
        super().__init__(message, "validation_error")


class AuthenticationError(GopherQueueError):
    """Authentication failed."""
    def __init__(self, message: str = "Authentication failed"):
        super().__init__(message, "unauthorized")
