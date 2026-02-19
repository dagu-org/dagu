# Dagu Helm Chart

A Helm chart for deploying Dagu on Kubernetes.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- **A storage class that supports `ReadWriteMany` access mode** (required)

Dagu uses a shared filesystem for state persistence. You must have a storage class that supports `ReadWriteMany`:
- NFS (via nfs-client-provisioner)
- AWS EFS
- CephFS
- Azure Files (Premium)
- GlusterFS

## Installation

```bash
# Install with default values
helm install dagu ./charts/dagu

# Install with custom storage class
helm install dagu ./charts/dagu --set persistence.storageClass=nfs-client

# Install with custom image tag
helm install dagu ./charts/dagu --set image.tag=v1.12.0
```

## Architecture

The chart deploys four components:

- **Coordinator**: gRPC server for distributed task execution (port 50055)
- **Scheduler**: Manages DAG execution schedules (port 8090 for health)
- **Worker**: Executes DAG steps (configurable pools with independent replicas)
- **UI**: Web interface for managing DAGs (port 8080)

All components share a single PersistentVolumeClaim with `ReadWriteMany` access mode.

## Configuration

### Required Values

```yaml
persistence:
  storageClass: "nfs-client"  # REQUIRED: Must support RWX
```

### Local Testing (Kind, Docker Desktop)

For local single-node clusters that don't support RWX:

```bash
helm install dagu charts/dagu \
  --set persistence.accessMode=ReadWriteOnce \
  --set persistence.skipValidation=true \
  --set workerPools.general.replicas=1
```

### Worker Pools

Workers are organized into pools. Each pool creates a separate Kubernetes Deployment with its own replicas, labels, resources, and scheduling constraints. DAGs select workers via `workerSelector` labels that match a pool's labels.

```yaml
workerPools:
  general:
    replicas: 2
    labels: {}
    resources:
      requests:
        memory: "128Mi"
        cpu: "100m"
      limits:
        memory: "256Mi"
        cpu: "200m"
    nodeSelector: {}
    tolerations: []
    affinity: {}

  gpu:
    replicas: 1
    labels:
      gpu: "true"
    resources:
      requests:
        memory: "512Mi"
        cpu: "500m"
        nvidia.com/gpu: "1"
      limits:
        memory: "1Gi"
        cpu: "1000m"
        nvidia.com/gpu: "1"
    nodeSelector:
      nvidia.com/gpu.present: "true"
    tolerations:
      - key: nvidia.com/gpu
        operator: Exists
        effect: NoSchedule
    affinity: {}
```

A pool with `labels: {}` (like `general` above) matches any DAG that has no `workerSelector`. To route a DAG to a specific pool, set `workerSelector` in the DAG definition to match the pool's labels:

```yaml
# In your DAG file
workerSelector:
  gpu: "true"
```

### Authentication

By default, the chart uses builtin authentication. **Change these values in production!**

```yaml
auth:
  mode: "builtin"  # Options: "none", "basic", "builtin" (default)
  builtin:
    admin:
      username: "admin"        # optional: skips setup page
      password: ""             # optional: auto-generated if empty
    token:
      secret: ""               # optional: auto-generated at {data_dir}/auth/token_secret
      ttl: "24h"
```

To disable authentication:
```bash
helm install dagu ./charts/dagu --set auth.mode=none
```

### Component Resources

```yaml
image:
  repository: ghcr.io/dagu-org/dagu
  tag: latest

coordinator:
  replicas: 1
  resources:
    requests:
      memory: "128Mi"
      cpu: "100m"

scheduler:
  replicas: 1
  resources:
    requests:
      memory: "256Mi"
      cpu: "250m"

workerPools:
  general:
    replicas: 2
    labels: {}
    resources:
      requests:
        memory: "128Mi"
        cpu: "100m"

ui:
  replicas: 1
  resources:
    requests:
      memory: "256Mi"
      cpu: "250m"
```

## Accessing the UI

```bash
# Port forward to access UI
kubectl port-forward svc/dagu-ui 8080:8080

# Then visit http://localhost:8080
# If no admin password was set, check server logs for the auto-generated password
```

## Current Constraints

This chart reflects Dagu's current architecture:

- **Shared filesystem required**: All components must share the same RWX volume
- **File-based state**: State is stored in files on the shared volume
- **No database**: Dagu does not use a database for state management

## Uninstall

```bash
helm uninstall dagu
```

**Warning**: This will delete the PersistentVolumeClaim and all data. Backup your DAGs and logs first!
