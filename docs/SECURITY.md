# GopherQueue Security Model

**Version:** 1.0  
**Status:** Security Specification

---

## Overview

This document defines the security principles and mechanisms for GopherQueue. Security is implemented at the Security Boundary layer and enforced across all external interfaces.

---

## 1. Principals and Roles

### Principal Types

| Principal   | Description                                       |
| ----------- | ------------------------------------------------- |
| Application | Service that submits jobs via library integration |
| API Client  | External client accessing HTTP/gRPC API           |
| Operator    | Human or service managing the queue system        |
| Worker      | Internal component executing jobs                 |

### Role Definitions

| Role      | Description         | Default Permissions          |
| --------- | ------------------- | ---------------------------- |
| admin     | Full system access  | All operations               |
| submitter | Job submission only | Submit, view own jobs        |
| observer  | Read-only access    | View all jobs, metrics       |
| operator  | Operations access   | View, cancel, retry, metrics |

### Permission Matrix

| Action            | admin | submitter | observer | operator |
| ----------------- | ----- | --------- | -------- | -------- |
| Submit job        | ✓     | ✓         | ✗        | ✗        |
| View own jobs     | ✓     | ✓         | ✓        | ✓        |
| View all jobs     | ✓     | ✗         | ✓        | ✓        |
| Cancel own jobs   | ✓     | ✓         | ✗        | ✓        |
| Cancel any job    | ✓     | ✗         | ✗        | ✓        |
| Retry failed jobs | ✓     | ✗         | ✗        | ✓        |
| View metrics      | ✓     | ✗         | ✓        | ✓        |
| Manage workers    | ✓     | ✗         | ✗        | ✓        |
| Configure system  | ✓     | ✗         | ✗        | ✗        |

---

## 2. Authentication

### Authentication Methods

#### API Key Authentication

For service-to-service communication and programmatic access.

```
Header: Authorization: Bearer <api-key>

API Key Format: gq_<environment>_<random-32-bytes-base64>
Example: gq_prod_a3JpZGdlLXNlY3VyaXR5LWtleQ==
```

**Properties:**

- Keys are environment-scoped (prod, staging, dev)
- Keys are associated with a principal and role
- Keys can be revoked without affecting other keys
- Keys have optional expiration dates

#### mTLS (Mutual TLS)

For high-security deployments requiring bidirectional authentication.

**Properties:**

- Client presents certificate signed by trusted CA
- Certificate CN or SAN maps to principal identity
- Certificate roles encoded in custom extensions
- Supports certificate rotation without downtime

#### Library Integration

For in-process usage, authentication is implicit.

**Properties:**

- Library runs in same process; trust is inherited
- No network boundary; no credential transmission
- Application responsible for authorization before calling library

### Authentication Flow

```
┌────────┐         ┌──────────────┐         ┌────────────┐
│ Client │─────────│ Auth Handler │─────────│ Auth Store │
└────────┘         └──────────────┘         └────────────┘
     │                    │                       │
     │ Request + Creds    │                       │
     ├───────────────────▶│                       │
     │                    │ Validate Credentials  │
     │                    ├──────────────────────▶│
     │                    │                       │
     │                    │ Principal + Roles     │
     │                    │◀──────────────────────┤
     │                    │                       │
     │ Authenticated Ctx  │                       │
     │◀───────────────────┤                       │
```

---

## 3. Authorization

### Authorization Model

GopherQueue uses Role-Based Access Control (RBAC) with optional resource-level restrictions.

**Hierarchy:**

1. System-level roles grant baseline permissions
2. Resource-level rules can further restrict access
3. Deny always overrides allow

### Resource-Level Authorization

Jobs can be restricted to specific principals:

```
JobSubmission {
    ...
    security: {
        owner: "service-foo"        // Principal ID
        visibility: "owner"         // Only owner can view
        cancellable_by: ["owner", "operator"]
    }
}
```

### Authorization Decision Flow

```
1. Extract principal from authenticated context
2. Extract required permission from operation
3. Check system role permissions
4. If resource-specific, check resource-level rules
5. Apply deny rules
6. Return allow/deny decision
```

### Audit Logging

All authorization decisions are logged:

```
AuthorizationEvent {
    timestamp:      timestamp
    principal:      string
    action:         string
    resource:       string
    decision:       enum       // ALLOW | DENY
    reason:         string     // Why decision was made
    request_id:     string
}
```

---

## 4. Job Data Protection

### Data Classification

| Classification | Description                | Handling                                 |
| -------------- | -------------------------- | ---------------------------------------- |
| Public         | Non-sensitive job metadata | Normal logging, normal storage           |
| Internal       | Business-sensitive data    | Masked in logs, encrypted at rest        |
| Confidential   | PII, credentials, secrets  | Never logged, encrypted, short retention |

### Payload Encryption

For jobs containing sensitive payloads:

**At-Rest Encryption:**

- Job payloads encrypted before persistence
- AES-256-GCM with per-job encryption keys
- Key encryption keys rotated on configurable schedule

**In-Transit Encryption:**

- TLS 1.3 required for all external communication
- mTLS available for enhanced security

**Application-Level Encryption:**

- Applications may encrypt fields before submission
- GopherQueue provides encryption helpers but does not require their use

### Result Sanitization

Job results follow the same classification as payloads:

- Confidential results are encrypted
- Results are purged according to retention policy
- Error messages are sanitized to prevent secret leakage

### Log Sanitization

```
Sanitization Rules:
1. Payloads larger than 1KB are not logged
2. Fields matching /password|secret|token|key/i are masked
3. Confidential job types have all payload fields masked
4. Stack traces are retained; local variables are not
```

---

## 5. Abuse Prevention

### Rate Limiting

**Submission Rate Limits:**

| Scope          | Default       | Configurable |
| -------------- | ------------- | ------------ |
| Per API key    | 100/second    | ✓            |
| Per IP address | 50/second     | ✓            |
| Global         | 10,000/second | ✓            |

**Rate Limit Response:**

```
HTTP 429 Too Many Requests
Retry-After: <seconds>

{
    "error": "QUOTA_EXCEEDED",
    "message": "Rate limit exceeded",
    "limit": "100/second",
    "retry_after": 1
}
```

### Queue Depth Limits

Prevent queue flooding:

| Limit                 | Default   | Action                    |
| --------------------- | --------- | ------------------------- |
| Total pending jobs    | 1,000,000 | Reject new submissions    |
| Pending per job type  | 100,000   | Reject for that type      |
| Pending per principal | 10,000    | Reject for that principal |

### Payload Size Limits

| Limit               | Default | Maximum |
| ------------------- | ------- | ------- |
| Single job payload  | 1 MB    | 16 MB   |
| Batch total payload | 10 MB   | 100 MB  |
| Result payload      | 1 MB    | 16 MB   |

### Resource Quotas

Per-principal resource quotas:

```
Quota {
    max_pending_jobs:       integer     // Max jobs in non-terminal state
    max_running_jobs:       integer     // Max concurrent executions
    max_job_priority:       enum        // Highest allowed priority
    max_execution_time:     duration    // Longest allowed timeout
    max_retention_days:     integer     // Longest result retention
}
```

### Abuse Detection

Automated detection of abuse patterns:

| Pattern          | Detection              | Response         |
| ---------------- | ---------------------- | ---------------- |
| Submission spike | Rate > 10x baseline    | Throttle + alert |
| Payload flood    | Large payloads in bulk | Reject + alert   |
| Priority abuse   | All jobs CRITICAL      | Demote + alert   |
| Retry storm      | High retry rate        | Backoff + alert  |

---

## 6. Network Security

### Network Boundaries

```
┌─────────────────────────────────────────────┐
│                 EXTERNAL ZONE               │
│   (Untrusted: API clients, webhooks)        │
└─────────────────────┬───────────────────────┘
                      │ TLS Required
        ┌─────────────▼─────────────┐
        │     SECURITY BOUNDARY     │
        │  (Authentication, Authz)  │
        └─────────────┬─────────────┘
                      │
┌─────────────────────▼───────────────────────┐
│                INTERNAL ZONE                │
│   (Trusted: workers, persistence, etc.)     │
└─────────────────────────────────────────────┘
```

### TLS Configuration

**Minimum Requirements:**

- TLS 1.2 (TLS 1.3 preferred)
- Strong cipher suites only
- Valid certificates (self-signed only in development)

**Recommended Cipher Suites:**

```
TLS_AES_256_GCM_SHA384
TLS_CHACHA20_POLY1305_SHA256
TLS_AES_128_GCM_SHA256
```

### Network ACLs

For multi-zone deployments:

- API servers accept traffic from load balancer only
- Workers accept traffic from scheduler only
- Persistence accepts traffic from internal zone only
- Metrics endpoint can be isolated from main API

---

## 7. Operational Security

### Secret Management

| Secret Type          | Storage                | Rotation      |
| -------------------- | ---------------------- | ------------- |
| API keys             | Hashed in database     | On demand     |
| Encryption keys      | HSM or secrets manager | Quarterly     |
| TLS certificates     | Secrets manager        | Before expiry |
| Database credentials | Secrets manager        | Monthly       |

### Minimal Privilege

Workers run with minimal permissions:

- No network access (unless job requires it)
- Read-only filesystem (except temp)
- No elevated privileges
- Resource limits enforced

### Security Events

Events requiring investigation:

| Event                   | Severity | Response                   |
| ----------------------- | -------- | -------------------------- |
| Failed authentication   | INFO     | Log, increment counter     |
| Failed authorization    | WARN     | Log, alert if repeated     |
| Invalid API key         | WARN     | Log origin, potential scan |
| Rate limit exceeded     | INFO     | Log, automatic throttling  |
| Suspicious payload      | WARN     | Log, potential malformed   |
| Certificate expiry soon | WARN     | Alert for rotation         |

---

## 8. Compliance Considerations

### Data Retention

- Job records retained according to configurable policy
- Default: 30 days for completed, 90 days for failed
- Confidential jobs: 7 days or less
- Audit logs: 1 year minimum

### Right to Deletion

Support for data deletion requests:

- Jobs can be purged by correlation ID
- Supports bulk deletion by principal or tag
- Deletion is permanent and logged

### Audit Trail

All security-relevant actions produce audit records:

- Authentication attempts (success and failure)
- Authorization decisions
- Configuration changes
- Data access for sensitive jobs
- Administrative actions

---

_This security model provides defense in depth. Implementations should layer these controls and adapt to specific deployment contexts._
