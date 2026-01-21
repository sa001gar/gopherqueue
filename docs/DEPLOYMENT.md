# GopherQueue Deployment Guide

Complete guide for deploying GopherQueue in production environments.

---

## Quick Start

### Binary Installation

```bash
# Install via Go
go install github.com/sa001gar/gopherqueue/cmd/gq@latest

# Or download pre-built binary from GitHub Releases
curl -L https://github.com/sa001gar/gopherqueue/releases/latest/download/gq-linux-amd64 -o gq
chmod +x gq && sudo mv gq /usr/local/bin/
```

### Start the Server

```bash
# Development
gq serve

# Production
gq serve --http :8080 --workers 20 --data-dir /var/lib/gopherqueue
```

---

## Self-Hosting Options

### Option 1: Direct Binary (Simplest)

Best for: Single-server deployments, edge computing, development.

```bash
# Linux/macOS
./gq serve --http :8080 --data-dir /var/lib/gopherqueue --workers 20

# Windows
gq.exe serve --http :8080 --data-dir C:\gopherqueue\data --workers 20
```

**Systemd Service (`/etc/systemd/system/gopherqueue.service`):**

```ini
[Unit]
Description=GopherQueue Background Job Engine
After=network.target

[Service]
Type=simple
User=gopherqueue
ExecStart=/usr/local/bin/gq serve --http :8080 --data-dir /var/lib/gopherqueue --workers 20
Restart=always
RestartSec=5
Environment=GQ_API_KEY=your-secret-key

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable gopherqueue
sudo systemctl start gopherqueue
```

### Option 2: Docker (Recommended for Production)

**Single Container:**

```bash
docker run -d \
  --name gopherqueue \
  -p 8080:8080 \
  -v gq_data:/data \
  -e GQ_API_KEY=your-secret-key \
  sa001gar/gopherqueue:latest \
  serve --http=:8080 --data-dir=/data --workers=20
```

**Docker Compose:**

```yaml
version: "3.8"
services:
  gopherqueue:
    image: sa001gar/gopherqueue:latest
    container_name: gopherqueue
    restart: always
    ports:
      - "8080:8080"
      - "9090:9090" # Metrics
    volumes:
      - gq_data:/data
    environment:
      - GQ_API_KEY=${GQ_API_KEY}
    command: >
      serve 
      --http=:8080 
      --data-dir=/data 
      --workers=20
      --metrics-addr=:9090
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/live"]
      interval: 30s
      timeout: 10s
      retries: 3

volumes:
  gq_data:
```

### Option 3: Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gopherqueue
spec:
  replicas: 1 # Single replica due to embedded storage
  selector:
    matchLabels:
      app: gopherqueue
  template:
    metadata:
      labels:
        app: gopherqueue
    spec:
      containers:
        - name: gopherqueue
          image: sa001gar/gopherqueue:latest
          ports:
            - containerPort: 8080
            - containerPort: 9090
          volumeMounts:
            - name: data
              mountPath: /data
          env:
            - name: GQ_API_KEY
              valueFrom:
                secretKeyRef:
                  name: gopherqueue-secret
                  key: api-key
          args: ["serve", "--http=:8080", "--data-dir=/data", "--workers=20"]
          livenessProbe:
            httpGet:
              path: /live
              port: 8080
            initialDelaySeconds: 5
          readinessProbe:
            httpGet:
              path: /ready
              port: 8080
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: gopherqueue-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: gopherqueue
spec:
  selector:
    app: gopherqueue
  ports:
    - name: http
      port: 8080
    - name: metrics
      port: 9090
```

---

## Microservice Architecture Integration

### GopherQueue as a Dedicated Service

Deploy GopherQueue as a standalone microservice that other services interact with via HTTP API.

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  API Gateway │     │ User Service │     │Order Service │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       └────────────────────┼────────────────────┘
                            ▼
                   ┌─────────────────┐
                   │   GopherQueue   │
                   │  (Job Service)  │
                   └────────┬────────┘
                            │
              ┌─────────────┼─────────────┐
              ▼             ▼             ▼
      ┌─────────────┐ ┌───────────┐ ┌───────────┐
      │Email Worker │ │PDF Worker │ │ETL Worker │
      └─────────────┘ └───────────┘ └───────────┘
```

### Event-Driven Architecture

Use GopherQueue's SSE (Server-Sent Events) for real-time job status updates.

**Publisher (Your Service):**

```python
# Django example
from gopherqueue import GopherQueueSync

queue = GopherQueueSync("http://gopherqueue:8080")

def handle_order_created(order):
    # Enqueue background tasks
    queue.submit("send_confirmation_email", {"order_id": order.id})
    queue.submit("generate_invoice_pdf", {"order_id": order.id})
    queue.submit("sync_to_warehouse", {"order_id": order.id})
```

**Subscriber (Status Updates):**

```python
async for event in queue.events(job_types=["*"]):
    if event.event_type == "job.completed":
        # Notify user, trigger next workflow step, etc.
        await notify_user(event.data.id, "completed")
```

---

## Production Configuration

### Environment Variables

| Variable              | Description            | Default  |
| --------------------- | ---------------------- | -------- |
| `GQ_HTTP_ADDR`        | HTTP bind address      | `:8080`  |
| `GQ_DATA_DIR`         | Data storage path      | `./data` |
| `GQ_WORKERS`          | Worker concurrency     | `10`     |
| `GQ_API_KEY`          | API authentication key | (none)   |
| `GQ_METRICS_ADDR`     | Prometheus metrics     | `:9090`  |
| `GQ_SHUTDOWN_TIMEOUT` | Graceful shutdown      | `30s`    |

### Security Configuration

```bash
# Enable API key authentication
gq serve --api-key-enabled --api-key="your-secret-key"

# Or via environment
export GQ_API_KEY="your-secret-key"
gq serve
```

**Client Authentication:**

```bash
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "X-API-Key: your-secret-key" \
  -H "Content-Type: application/json" \
  -d '{"type": "email", "payload": {...}}'
```

### Fault Tolerance

GopherQueue provides built-in fault tolerance:

| Feature                | Description                                 |
| ---------------------- | ------------------------------------------- |
| **Durable Storage**    | BoltDB persistence survives crashes         |
| **Automatic Retries**  | Configurable retry with exponential backoff |
| **Dead Letter Queue**  | Failed jobs preserved for analysis          |
| **Graceful Shutdown**  | In-flight jobs complete before exit         |
| **Health Checks**      | `/live` and `/ready` endpoints              |
| **Visibility Timeout** | Stuck jobs automatically re-queued          |

### Monitoring & Observability

**Prometheus Metrics:**

```yaml
# prometheus.yml
scrape_configs:
  - job_name: "gopherqueue"
    static_configs:
      - targets: ["gopherqueue:9090"]
```

**Key Metrics:**

- `gopherqueue_jobs_submitted_total`
- `gopherqueue_jobs_completed_total`
- `gopherqueue_jobs_failed_total`
- `gopherqueue_queue_depth`
- `gopherqueue_processing_duration_seconds`

---

## High Availability Patterns

### Pattern 1: Queue Partitioning

Run multiple GopherQueue instances, each handling different job types:

```
Instance A: --job-types=email,sms,push
Instance B: --job-types=pdf,report,export
Instance C: --job-types=etl,sync,cleanup
```

### Pattern 2: Active-Passive Failover

Use external health monitoring to failover to standby instance:

```
┌─────────────────────────────────────────┐
│           Load Balancer                 │
│       (Health-check based)              │
└─────────────────┬───────────────────────┘
                  │
       ┌──────────┴──────────┐
       ▼                     ▼
┌─────────────┐       ┌─────────────┐
│  Primary    │       │  Standby    │
│ (Active)    │       │ (Passive)   │
└─────────────┘       └─────────────┘
       │                     │
       ▼                     ▼
  [Volume A]            [Volume B]
  (Replicated)          (Replicated)
```

---

## Roadmap Features

### gRPC Support (Planned)

Future releases will include gRPC interface for:

- Lower latency communication
- Bi-directional streaming
- Better language support via protobuf

### Embedded SDK Mode (Planned)

Package GopherQueue binary within language SDKs:

- Zero-configuration local development
- SDK auto-starts embedded server
- Seamless transition to production (external server)

```python
# Future API
from gopherqueue import GopherQueue

# Embedded mode - SDK starts local server automatically
queue = GopherQueue.embedded(data_dir="./jobs")

# Production mode - connect to external server
queue = GopherQueue("http://gopherqueue.internal:8080")
```
