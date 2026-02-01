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
- **Worker**: Executes DAG steps (2 replicas by default)
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
  --set worker.replicas=1
```

### Authentication

By default, the chart uses builtin authentication. **Change these values in production!**

```yaml
auth:
  mode: "builtin"  # Options: "none", "builtin", "oidc"
  builtin:
    admin:
      username: "admin"
      password: "adminpass"  # CHANGEME
    token:
      secret: "dagu-secret-key"  # CHANGEME
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
# Default credentials: admin / adminpass
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
