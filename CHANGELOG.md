# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.1] - 2026-01-21

### Fixed

- Fixed various linter warnings and unchecked errors
- Fixed "ineffective break" in scheduler loop
- Improved code quality in worker and persistence layers
- Fixed .gitignore to correctly include cmd/gq directory

## [0.1.0] - 2026-01-21

### Added

- Initial release of GopherQueue
- Core job model with 9 states (pending, scheduled, running, retrying, delayed, completed, failed, dead_letter, cancelled)
- Priority levels (Critical, High, Normal, Low, Bulk)
- BoltDB persistence for durable job storage
- In-memory store for testing
- Priority scheduler with exponential, linear, and constant backoff strategies
- Worker pool with configurable concurrency
- Panic recovery in job handlers
- Checkpointing support for long-running jobs
- Job dependencies
- Idempotency keys for deduplication
- Recovery manager for stuck job detection
- HTTP API with endpoints for job management, health checks, and metrics
- CLI tool (`gq`) with serve, submit, and status commands
- API key authentication
- Role-based authorization (admin, operator, viewer)
- Health endpoints (liveness, readiness)
- JSON metrics endpoint

### Documentation

- Project manifesto and philosophy
- Architecture documentation
- Job model specification
- API specification
- Security model
- Observability guide
- Failure playbook
- Testing strategy

[Unreleased]: https://github.com/sa001gar/gopherqueue/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/sa001gar/gopherqueue/releases/tag/v0.1.0
