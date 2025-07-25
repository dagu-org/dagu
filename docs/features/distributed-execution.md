# Distributed Execution

Run workflow tasks across multiple worker nodes.

## Overview

Distributed execution allows you to:

- Route tasks to workers based on labels (GPU, memory, region, etc.)
- Run tasks in specific regions for compliance or performance
- Add workers to handle increased load
- Route tasks to workers with specific hardware capabilities

## Architecture

The distributed execution system consists of:

1. **Main Dagu Instance**: Runs the scheduler, web UI, and coordinator service
2. **Coordinator Service**: gRPC server that distributes tasks to workers
3. **Worker Nodes**: Poll the coordinator for tasks and execute them
4. **Shared Storage**: Required for DAG files and execution state

```
Main Instance (Scheduler + UI + Coordinator)
                    │
                    │ gRPC
                    │
    ┌───────────────┴───────────────┐
    │                               │
Worker 1                        Worker 2
(gpu=true)                   (region=eu-west)
    │                               │
    └───────────────┬───────────────┘
                    │
            Shared Storage
        (DAG files, logs, state)
```

## Setting Up Distributed Execution

### Step 1: Start the Coordinator

The coordinator service is automatically started when you use `dagu start-all`:

```bash
# Start all services including coordinator
dagu start-all --host=0.0.0.0 --port=8080

# Or start coordinator separately
dagu coordinator --coordinator.host=0.0.0.0 --coordinator.port=50055
```

### Step 2: Deploy Workers

Start workers on your compute nodes with appropriate labels:

```bash
# GPU-enabled worker
dagu worker \
  --worker.labels gpu=true,cuda=11.8,memory=64G \
  --worker.coordinator-host=coordinator.example.com

# CPU-optimized worker
dagu worker \
  --worker.labels cpu-arch=amd64,cpu-cores=32,region=us-east-1 \
  --worker.coordinator-host=coordinator.example.com

# Region-specific worker
dagu worker \
  --worker.labels region=eu-west-1,compliance=gdpr \
  --worker.coordinator-host=coordinator.example.com
```

### Step 3: Route Tasks to Workers

Use `workerSelector` in your DAG definitions to route tasks:

```yaml
name: distributed-pipeline
steps:
  # This task requires GPU
  - name: train-model
    run: train-model
    workerSelector:
      gpu: "true"

---
name: train-model
command: python train_model.py
workerSelector:
  gpu: "true"
steps:
  - name: train-model
    command: python train_model.py
```

## Worker Labels

Worker labels are key-value pairs that describe worker capabilities:

### Common Label Patterns

```bash
# Hardware capabilities
--worker.labels gpu=true,gpu-model=a100,vram=40G

# Geographic location
--worker.labels region=us-east-1,zone=us-east-1a,datacenter=dc1

# Resource specifications
--worker.labels memory=128G,cpu-cores=64,storage=fast-nvme

# Compliance and security
--worker.labels compliance=hipaa,security-clearance=high
```

### Label Matching Rules

- **All specified labels must match**: A worker must have ALL labels in the `workerSelector`
- **Exact value matching**: Label values must match exactly (case-sensitive)
- **No selector = any worker**: Tasks without `workerSelector` can run on any worker

## Configuration

### Coordinator Configuration

```yaml
# config.yaml
coordinator:
  host: 0.0.0.0
  port: 50055
  signingKey: "your-secret-key"
  tls:
    certFile: /path/to/server.crt
    keyFile: /path/to/server.key
    caFile: /path/to/ca.crt  # For mutual TLS
```

### Worker Configuration

```yaml
# config.yaml
worker:
  id: "worker-gpu-01"  # Defaults to hostname@PID
  maxActiveRuns: 10
  coordinatorHost: coordinator.example.com
  coordinatorPort: 50055
  labels:
    gpu: "true"
    memory: "64G"
    region: "us-east-1"
  insecure: false  # Use TLS
  tls:
    certFile: /path/to/client.crt
    keyFile: /path/to/client.key
    caFile: /path/to/ca.crt
```

### Environment Variables

```bash
# Coordinator
export DAGU_COORDINATOR_HOST=0.0.0.0
export DAGU_COORDINATOR_PORT=50055
export DAGU_COORDINATOR_SIGNING_KEY=secret

# Worker
export DAGU_WORKER_ID=worker-01
export DAGU_WORKER_COORDINATOR_HOST=coordinator.example.com
export DAGU_WORKER_LABELS="gpu=true,region=us-east-1"
```

## Monitoring Workers

### Web UI Workers Page

Access the Workers page in the Web UI to monitor:
- Connected workers and their labels
- Worker health status (Healthy/Warning/Unhealthy)
- Currently running tasks on each worker
- Task hierarchy (root/parent/child DAGs)

### Health Status Indicators

- 🟢 **Healthy**: Last heartbeat < 5 seconds ago
- 🟡 **Warning**: Last heartbeat 5-15 seconds ago
- 🔴 **Unhealthy**: Last heartbeat > 15 seconds ago
- ⚫ **Offline**: No heartbeat for > 30 seconds

### API Endpoint

```bash
# Get worker status via API
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v2/workers

# Response
{
  "workers": [
    {
      "id": "worker-gpu-01",
      "labels": {"gpu": "true", "memory": "64G"},
      "health_status": "HEALTHY",
      "last_heartbeat": "2024-02-11T12:00:00Z",
      "running_tasks": [
        {
          "dag_name": "ml-pipeline",
          "dag_run_id": "20240211_120000",
          "root_dag_run_name": "ml-pipeline",
          "started_at": "2024-02-11T12:00:00Z"
        }
      ]
    }
  ]
}
```

## Security

### TLS Configuration

#### Server-side TLS (Coordinator)
```bash
dagu coordinator \
  --coordinator.tls-cert=server.crt \
  --coordinator.tls-key=server.key
```

#### Mutual TLS (mTLS)
```bash
# Coordinator with client verification
dagu coordinator \
  --coordinator.tls-cert=server.crt \
  --coordinator.tls-key=server.key \
  --coordinator.tls-ca=ca.crt

# Worker with client certificate
dagu worker \
  --worker.insecure=false \
  --worker.tls-cert=client.crt \
  --worker.tls-key=client.key \
  --worker.tls-ca=ca.crt
```

### Authentication

Use signing keys for additional security:

```bash
# Coordinator
dagu coordinator --coordinator.signing-key=your-secret-key

# Worker (key configured in coordinator)
dagu worker --worker.coordinator-host=secure-coordinator.example.com
```

## Deployment Examples

### Docker Compose

```yaml
version: '3.8'

services:
  dagu-main:
    image: dagu:latest
    command: start-all --host=0.0.0.0
    ports:
      - "8080:8080"
      - "50055:50055"
    volumes:
      - ./dags:/etc/dagu/dags
      - ./data:/var/lib/dagu
  
  worker-gpu:
    image: dagu:latest
    command: >
      worker
      --worker.labels=gpu=true,cuda=11.8
      --worker.coordinator-host=dagu-main
      --worker.coordinator-port=50055
    deploy:
      replicas: 2
      resources:
        reservations:
          devices:
            - capabilities: [gpu]
  
  worker-cpu:
    image: dagu:latest
    command: >
      worker
      --worker.labels=cpu-only=true,region=us-east-1
      --worker.coordinator-host=dagu-main
      --worker.coordinator-port=50055
    deploy:
      replicas: 5
```

### Kubernetes

```yaml
# Coordinator Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dagu-coordinator
spec:
  replicas: 1
  selector:
    matchLabels:
      app: dagu-coordinator
  template:
    metadata:
      labels:
        app: dagu-coordinator
    spec:
      containers:
      - name: dagu
        image: dagu:latest
        command: ["dagu", "start-all"]
        args: ["--host=0.0.0.0"]
        ports:
        - containerPort: 8080
          name: web
        - containerPort: 50055
          name: grpc

---
# GPU Worker Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dagu-worker-gpu
spec:
  replicas: 3
  selector:
    matchLabels:
      app: dagu-worker-gpu
  template:
    metadata:
      labels:
        app: dagu-worker-gpu
    spec:
      containers:
      - name: worker
        image: dagu:latest
        command: ["dagu", "worker"]
        args:
        - --worker.labels=gpu=true,node-type=gpu-node
        - --worker.coordinator-host=dagu-coordinator
        - --worker.coordinator-port=50055
        resources:
          limits:
            nvidia.com/gpu: 1

---
# CPU Worker Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dagu-worker-cpu
spec:
  replicas: 10
  selector:
    matchLabels:
      app: dagu-worker-cpu
  template:
    metadata:
      labels:
        app: dagu-worker-cpu
    spec:
      containers:
      - name: worker
        image: dagu:latest
        command: ["dagu", "worker"]
        args:
        - --worker.labels=cpu-optimized=true,region=us-east-1
        - --worker.coordinator-host=dagu-coordinator
        - --worker.coordinator-port=50055
```

## See Also

- [Worker Labels](/features/worker-labels) - Detailed guide on worker label configuration
- [Architecture](/overview/architecture#distributed-execution-architecture) - Technical architecture details
- [CLI Reference](/reference/cli) - Complete command reference
- [Configuration Reference](/configurations/reference) - All configuration options
