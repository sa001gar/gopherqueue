# GopherQueue

<div align="center">

[![Go Reference](https://pkg.go.dev/badge/github.com/sa001gar/gopherqueue.svg)](https://pkg.go.dev/github.com/sa001gar/gopherqueue)
[![Go Report Card](https://goreportcard.com/badge/github.com/sa001gar/gopherqueue)](https://goreportcard.com/report/github.com/sa001gar/gopherqueue)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![npm](https://img.shields.io/npm/v/gopherqueue)](https://www.npmjs.com/package/gopherqueue)

**🚀 Enterprise-grade, local-first background job engine for Go**

_Zero external dependencies • BoltDB persistence • Python & TypeScript SDKs_

[Quick Start](#-quick-start) •
[SDKs](#-multi-language-sdks) •
[Documentation](#-documentation) •
[Contributing](#-contributing)

</div>

---

## ✨ Features

| Feature                 | Description                                                   |
| ----------------------- | ------------------------------------------------------------- |
| 💾 **Durable Storage**  | BoltDB-backed persistence — jobs survive crashes and restarts |
| ⚡ **Priority Queues**  | Critical, High, Normal, Low, and Bulk priority levels         |
| 🔄 **Smart Retries**    | Exponential, linear, or constant backoff strategies           |
| 📊 **Observability**    | Prometheus metrics, structured logging, health checks         |
| 🛡️ **Fault Tolerant**   | Panic recovery, checkpointing, graceful shutdown              |
| 🔐 **Security Ready**   | API key auth, role-based authorization                        |
| 🔗 **Job Dependencies** | Chain jobs with wait conditions                               |
| 🆔 **Idempotency**      | Built-in deduplication via idempotency keys                   |

---

## 🚀 Quick Start

### Install

```bash
go install github.com/sa001gar/gopherqueue/cmd/gq@latest
```

### Start Server

```bash
gq serve                                              # Default: 10 workers, port 8080
gq serve --http :8080 --workers 20 --data-dir ./data  # Custom config
```

### 🐳 Docker

```bash
docker run -d --name gopherqueue -p 8080:8080 -v gq_data:/data sa001gar/gopherqueue:latest
```

### Submit a Job

```bash
# CLI
gq submit --type email --payload '{"to": "user@example.com"}'

# HTTP API
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{"type": "email", "payload": {"to": "user@example.com"}, "priority": 1}'
```

---

## 📦 Multi-Language SDKs

Use GopherQueue from any language with our official SDKs.

<table>
<tr>
<td width="50%">

### 🐍 Python

```bash
pip install gopherqueue
```

```python
from gopherqueue import GopherQueueSync

queue = GopherQueueSync("http://localhost:8080")
job = queue.submit("email", {"to": "user@example.com"})
print(f"Job {job.id} queued!")
```

</td>
<td width="50%">

### 📜 TypeScript / JavaScript

```bash
npm install gopherqueue
```

```typescript
import { GopherQueue } from "gopherqueue";

const queue = new GopherQueue("http://localhost:8080");
const job = await queue.submit("email", { to: "user@example.com" });
console.log(`Job ${job.id} queued!`);
```

</td>
</tr>
</table>

---

## 🔄 Job Lifecycle

```
                                    ┌─────────────┐
                                    │  completed  │
                                    └─────────────┘
                                          ▲
                                          │ success
┌─────────┐    ┌───────────┐    ┌─────────┴───┐
│ pending │───▶│ scheduled │───▶│   running   │
└─────────┘    └───────────┘    └──────┬──────┘
                                       │ failure
                    ┌──────────────────┴──────────────────┐
                    ▼                                     ▼
             ┌────────────┐                         ┌──────────┐
             │  retrying  │                         │  failed  │
             └────────────┘                         └─────┬────┘
                                                          │
                                                          ▼
                                                    ┌─────────────┐
                                                    │ dead_letter │
                                                    └─────────────┘
```

| State         | Description                                   |
| ------------- | --------------------------------------------- |
| `pending`     | Created, waiting to be scheduled              |
| `scheduled`   | In priority queue, ready for pickup           |
| `running`     | Worker actively processing                    |
| `completed`   | Finished successfully                         |
| `retrying`    | Failed, waiting for retry                     |
| `failed`      | Exceeded max attempts                         |
| `dead_letter` | Permanently failed, needs manual intervention |

---

## ⚙️ Configuration

| Flag                 | Default  | Description               |
| -------------------- | -------- | ------------------------- |
| `--http`             | `:8080`  | HTTP server address       |
| `--workers`          | `10`     | Concurrent worker count   |
| `--data-dir`         | `./data` | BoltDB storage directory  |
| `--shutdown-timeout` | `30s`    | Graceful shutdown timeout |

### Priority Levels

| Priority | Value | Use Case                 |
| -------- | ----- | ------------------------ |
| Critical | 0     | System alerts, payments  |
| High     | 1     | User-initiated actions   |
| Normal   | 2     | Standard background work |
| Low      | 3     | Batch processing         |
| Bulk     | 4     | Data migrations          |

---

## 📚 Documentation

| Guide                                     | Description                                    |
| ----------------------------------------- | ---------------------------------------------- |
| 📖 [SDK Guide](docs/SDK_GUIDE.md)         | Complete SDK reference with framework examples |
| 🚀 [Deployment](docs/DEPLOYMENT.md)       | Self-hosting, Docker, Kubernetes               |
| 🔌 [API Spec](docs/API_SPEC.md)           | REST API documentation                         |
| 🏗️ [Architecture](docs/ARCHITECTURE.md)   | System design & internals                      |
| 🔐 [Security](docs/SECURITY.md)           | Auth, authorization, best practices            |
| 📊 [Observability](docs/OBSERVABILITY.md) | Metrics, logging, monitoring                   |

### Framework Guides

| Framework          | Link                                                                                       |
| ------------------ | ------------------------------------------------------------------------------------------ |
| 🐍 Django          | [Complete Integration Guide](docs/SDK_GUIDE.md#django-python---complete-integration-guide) |
| ⚛️ Next.js         | [API Routes Example](docs/SDK_GUIDE.md#nextjs-nodejs)                                      |
| 🌶️ Flask / FastAPI | [Python Web Frameworks](docs/SDK_GUIDE.md#flask--fastapi)                                  |

---

## 🏗️ Project Structure

```
gopherqueue/
├── api/           # HTTP API handlers
├── cli/           # Command-line interface
├── cmd/gq/        # Main entry point
├── core/          # Core types & options
├── docs/          # Documentation
├── observability/ # Metrics & health
├── persistence/   # Storage (BoltDB)
├── scheduler/     # Priority queue
├── sdks/          # Python & TypeScript SDKs
├── security/      # Auth & authorization
└── worker/        # Job execution
```

---

## 🤝 Contributing

Contributions welcome! Please read our [Contributing Guide](CONTRIBUTING.md).

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

MIT License — see [LICENSE](LICENSE).

---

<div align="center">

**Built with ❤️ for developers who value simplicity**

[⬆ Back to top](#gopherqueue)

</div>
