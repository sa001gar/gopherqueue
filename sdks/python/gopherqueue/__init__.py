"""
GopherQueue Python SDK

A Python client library for GopherQueue, an enterprise-grade background job processing system.
"""

from gopherqueue.client import GopherQueue, GopherQueueSync
from gopherqueue.models import (
    Job,
    JobResult,
    JobState,
    Priority,
    BatchResult,
    WaitResult,
    SSEEvent,
    QueueStats,
)

__version__ = "1.0.0"
__all__ = [
    "GopherQueue",
    "GopherQueueSync",
    "Job",
    "JobResult",
    "JobState",
    "Priority",
    "BatchResult",
    "WaitResult",
    "SSEEvent",
    "QueueStats",
]
