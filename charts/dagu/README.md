# Dagu Helm Chart

A simple Helm chart for deploying Dagu on Kubernetes.

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

## Configuration

The chart deploys three components:

- **Scheduler**: Manages DAG execution schedules (1 replica)
- **Worker**: Executes DAG steps (2 replicas by default)
- **UI**: Web interface for managing DAGs (1 replica)

All components share a single PersistentVolumeClaim with `ReadWriteMany` access mode.

### Required Values

```yaml
persistence:
  storageClass: "nfs-client"  # REQUIRED: Must support RWX
```

### Optional Values

```yaml
image:
  repository: ghcr.io/dagu-org/dagu
  tag: latest

scheduler:
  replicas: 1
  resources:
    requests:
      memory: "256Mi"
      cpu: "250m"

worker:
  replicas: 2
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
