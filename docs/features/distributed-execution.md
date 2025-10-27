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
2. **Coordinator Service**: gRPC server that distributes tasks to workers with automatic service registry
3. **Worker Nodes**: Poll the coordinator for tasks and execute them with heartbeat monitoring
4. **Service Registry**: File-based system for automatic worker registration and health tracking
5. **Shared Storage**: Required for DAG files and execution state

```
Main Instance (Scheduler + UI + Coordinator)
         │                    │
         │ Service Registry   │ gRPC + Heartbeat
         │ (File-based)       │
         │                    │
    ┌────┴──────────┬─────────┴─────────────┐
    │               │                       │
Worker 1        Worker 2                Worker 3
(gpu=true)   (region=eu-west)        (cpu-optimized)
    │               │                       │
    └───────────────┴───────────────────────┘
                    │
            Shared Storage
        (DAG files, logs, state)
```

### Service Registry & Health Monitoring

The distributed execution system features automatic service registry and health monitoring:

- **File-based Service Registry**: Workers automatically register themselves in a shared service registry directory
- **Heartbeat Mechanism**: Workers send regular heartbeats (every 10 seconds by default)
- **Automatic Failover**: Tasks are automatically redistributed if a worker becomes unhealthy
- **Dynamic Scaling**: Add or remove workers at runtime without coordinator restart

## Setting Up Distributed Execution

### Step 1: Start the Coordinator

The coordinator service can be started with `dagu start-all` (requires `--coordinator.host` set to a non-localhost address):

```bash
# Start all services including coordinator (distributed mode)
dagu start-all --coordinator.host=0.0.0.0 --port=8080

# Single instance mode (coordinator disabled, default)
dagu start-all

# Or start coordinator separately
dagu coordinator --coordinator.host=0.0.0.0 --coordinator.port=50055
```

**Note:** The coordinator is only started by `start-all` when `--coordinator.host` is set to a non-localhost address (not `127.0.0.1` or `localhost`). This allows running in single instance mode by default.

The coordinator automatically registers itself in the service registry system and begins accepting worker connections.

**For containerized environments (Docker, Kubernetes)**, you need to configure both the bind address and advertise address:

```bash
# Bind to all interfaces and advertise the service name
dagu coordinator \
  --coordinator.host=0.0.0.0 \
  --coordinator.advertise=dagu-server \
  --coordinator.port=50055
```

Or using environment variables:

```bash
DAGU_COORDINATOR_HOST=0.0.0.0 \
DAGU_COORDINATOR_ADVERTISE=dagu-server \
DAGU_COORDINATOR_PORT=50055 \
dagu coordinator
```

- `--coordinator.host`: Address to bind the gRPC server (use `0.0.0.0` for containers)
- `--coordinator.advertise`: Address workers use to connect (defaults to hostname if not set)

### Step 2: Deploy Workers

Start workers on your compute nodes with appropriate labels:

```bash
# GPU-enabled worker
dagu worker \
  --worker.labels gpu=true,cuda=11.8,memory=64G

# CPU-optimized worker
dagu worker \
  --worker.labels cpu-arch=amd64,cpu-cores=32,region=us-east-1

# Region-specific worker
dagu worker \
  --worker.labels region=eu-west-1,compliance=gdpr
```

### Step 3: Route Tasks to Workers

Use `workerSelector` in your DAG definitions to route tasks:

```yaml
steps:
  # This task requires GPU
  - call: train-model
    workerSelector:
      gpu: "true"

---
name: train-model
command: python train_model.py
workerSelector:
  gpu: "true"
steps:
  - python train_model.py
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

# Service registry configuration
paths:
  serviceRegistryDir: "~/.local/share/dagu/service-registry"  # Directory for service registry files

# TLS configuration for peer connections (both coordinator and worker)
peer:
  insecure: true   # Use h2c (HTTP/2 cleartext) instead of TLS (default: true)
  certFile: /path/to/cert.pem
  keyFile: /path/to/key.pem
  clientCaFile: /path/to/ca.pem  # For mutual TLS
  skipTlsVerify: false  # Skip TLS certificate verification
```

### Worker Configuration

```yaml
# config.yaml
worker:
  id: "worker-gpu-01"  # Defaults to hostname@PID
  maxActiveRuns: 10
  labels:
    gpu: "true"
    memory: "64G"
    region: "us-east-1"

# TLS configuration shared with coordinator
peer:
  insecure: false  # Enable TLS
  certFile: /path/to/cert.pem
  keyFile: /path/to/key.pem
  clientCaFile: /path/to/ca.pem
```

### Environment Variables

```bash
# Coordinator
export DAGU_COORDINATOR_HOST=0.0.0.0
export DAGU_COORDINATOR_PORT=50055

# Worker
export DAGU_WORKER_ID=worker-01
export DAGU_WORKER_LABELS="gpu=true,region=us-east-1"

# Service Registry
export DAGU_PATHS_SERVICE_REGISTRY_DIR=/shared/dagu/service-registry

# Peer TLS configuration (for both coordinator and worker)
export DAGU_PEER_INSECURE=true  # Default: true (use h2c)
export DAGU_PEER_CERT_FILE=/path/to/cert.pem
export DAGU_PEER_KEY_FILE=/path/to/key.pem
export DAGU_PEER_CLIENT_CA_FILE=/path/to/ca.pem
export DAGU_PEER_SKIP_TLS_VERIFY=false
```

## Monitoring Workers

### Web UI Workers Page

Access the Workers page in the Web UI to monitor:
- Connected workers and their labels
- Worker health status (Healthy/Warning/Unhealthy)
- Currently running tasks on each worker
- Task hierarchy (root/parent/sub DAGs)

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

## Service Registry Deep Dive

### How It Works

The file-based service registry system provides automatic worker registration and health monitoring:

1. **Worker Registration**: When a worker starts, it creates a registry file containing:
   - Worker ID
   - Host and port information
   - Process ID (PID)
   - Timestamp of registration

2. **Heartbeat Updates**: Workers update their registry files every 10 seconds with:
   - Current timestamp
   - Health status
   - Active task count

3. **Coordinator Monitoring**: The coordinator continuously monitors registry files to:
   - Track available workers
   - Detect unhealthy workers
   - Remove stale entries (no heartbeat for 30+ seconds)

4. **Automatic Cleanup**: Registry files are automatically removed when:
   - Worker shuts down gracefully
   - Worker process terminates unexpectedly
   - Heartbeat timeout exceeds threshold

### Registry Directory Structure

```
~/.local/share/dagu/service-registry/
├── coordinator/
│   └── coordinator-primary-host1-50055.json
└── worker/
    ├── worker-gpu-01-host2-1234.json
    ├── worker-cpu-02-host3-5678.json
    └── worker-cpu-03-host4-9012.json
```

### Configuring Service Registry

```yaml
# Shared registry directory (must be accessible by all nodes)
paths:
  serviceRegistryDir: "/nfs/shared/dagu/service-registry"  # NFS mount example

# Or use environment variable
export DAGU_PATHS_SERVICE_REGISTRY_DIR=/nfs/shared/dagu/service-registry
```

## Security

### TLS Configuration

#### Server-side TLS (Coordinator)
```bash
dagu coordinator \
  --peer.insecure=false \
  --peer.cert-file=server.pem \
  --peer.key-file=server-key.pem
```

#### Mutual TLS (mTLS)
```bash
# Coordinator with client verification
dagu coordinator \
  --peer.insecure=false \
  --peer.cert-file=server.pem \
  --peer.key-file=server-key.pem \
  --peer.client-ca-file=ca.pem

# Worker with client certificate
dagu worker \
  --peer.insecure=false \
  --peer.cert-file=client.pem \
  --peer.key-file=client-key.pem \
  --peer.client-ca-file=ca.pem
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
    volumes:
      - ./data:/var/lib/dagu  # Shared storage for service registry
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
    volumes:
      - ./data:/var/lib/dagu  # Shared storage for service registry
    deploy:
      replicas: 5
```
