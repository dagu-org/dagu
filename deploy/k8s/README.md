# Boltbase Kubernetes Local Setup

This directory contains Kubernetes manifests for running Boltbase in distributed mode on your local machine for testing purposes.

## Architecture

This setup deploys Boltbase in **distributed execution mode**:

- **1 Server Pod** (`start-all`): Runs Web UI, Scheduler, and Coordinator (gRPC)
- **2 Worker Pods**: Poll the coordinator and execute DAG runs

**Note**: The coordinator is only started by `start-all` when `BOLTBASE_COORDINATOR_HOST` is set to a non-localhost address (not `127.0.0.1` or `localhost`). This setup explicitly configures it to `0.0.0.0` to enable distributed execution.

## Prerequisites

Choose one local Kubernetes solution:

### Option 1: Docker Desktop (Easiest)
```bash
# Enable Kubernetes in Docker Desktop
# Settings → Kubernetes → Enable Kubernetes
```

### Option 2: minikube
```bash
brew install minikube
minikube start
```

### Option 3: kind (Kubernetes in Docker)
```bash
brew install kind
kind create cluster --name boltbase-test
```

### Option 4: k3d (Fastest)
```bash
brew install k3d
k3d cluster create boltbase-test --agents 2
```

## Setup Instructions

### 1. Deploy to Kubernetes

The manifests use the official `ghcr.io/dagu-org/boltbase:latest` image by default, so you can deploy directly:

```bash
# Apply all manifests
kubectl apply -f deploy/k8s/

# Or apply individually in order:
kubectl apply -f deploy/k8s/namespace.yaml
kubectl apply -f deploy/k8s/configmap.yaml
kubectl apply -f deploy/k8s/pvc.yaml
kubectl apply -f deploy/k8s/server-deployment.yaml
kubectl apply -f deploy/k8s/worker-deployment.yaml
```

**Optional: Use Local Development Image**

If you're developing Boltbase and want to test local changes:

```bash
# Build local image
docker build -t boltbase:local .

# Load into cluster (for kind/k3d)
kind load docker-image boltbase:local --name boltbase-test
# OR
k3d image import boltbase:local --cluster boltbase-test

# Update deployments to use local image
sed -i '' 's|ghcr.io/dagu-org/boltbase:latest|boltbase:local|g' deploy/k8s/*-deployment.yaml
sed -i '' 's|imagePullPolicy: Always|imagePullPolicy: IfNotPresent|g' deploy/k8s/*-deployment.yaml
```

### 2. Verify Deployment

```bash
# Check all pods are running
kubectl get pods -n boltbase-dev

# Expected output:
# NAME                               READY   STATUS    RESTARTS   AGE
# boltbase-server-xxxxxxxxxx-xxxxx   1/1     Running   0          1m
# boltbase-worker-xxxxxxxxxx-xxxxx   1/1     Running   0          1m
# boltbase-worker-xxxxxxxxxx-xxxxx   1/1     Running   0          1m

# Check logs
kubectl logs -n boltbase-dev -l component=server
kubectl logs -n boltbase-dev -l component=worker
```

### 3. Access the Web UI

The server is exposed on NodePort 30080:

```bash
# For Docker Desktop / minikube
open http://localhost:30080

# For minikube specifically
minikube service boltbase-server -n boltbase-dev

# For kind, you may need port-forwarding
kubectl port-forward -n boltbase-dev service/boltbase-server 8080:8080
# Then access http://localhost:8080
```

## Testing Distributed Execution

### 1. Copy Example DAG to Server Pod

```bash
# Get the server pod name
SERVER_POD=$(kubectl get pod -n boltbase-dev -l component=server -o jsonpath='{.items[0].metadata.name}')

# Copy example DAG
kubectl cp deploy/k8s/example-dag.yaml boltbase-dev/$SERVER_POD:/var/lib/boltbase/dags/test-distributed.yaml

# Or create directly
kubectl exec -n boltbase-dev $SERVER_POD -- sh -c 'cat > /var/lib/boltbase/dags/test-distributed.yaml' < deploy/k8s/example-dag.yaml
```

### 2. Trigger DAG Execution via Web UI

1. Open http://localhost:30080
2. Navigate to DAGs
3. Find "test-distributed"
4. Click "Run"

### 3. Monitor Execution

```bash
# Watch worker logs to see which worker picks up the job
kubectl logs -n boltbase-dev -l component=worker -f

# Check server logs
kubectl logs -n boltbase-dev -l component=server -f

# View pod status
kubectl get pods -n boltbase-dev -w
```

### 4. Verify Distributed Execution

The example DAG outputs the hostname in each step. Check the logs to confirm different workers executed the DAG:

```bash
# Get execution logs from the UI or via CLI
kubectl exec -n boltbase-dev $SERVER_POD -- boltbase status test-distributed
```

## Configuration

### Scaling Workers

```bash
# Scale to 5 workers
kubectl scale deployment boltbase-worker -n boltbase-dev --replicas=5

# Verify
kubectl get pods -n boltbase-dev -l component=worker
```

### Worker Labels

Workers are configured with labels for capability-based task matching. Edit `worker-deployment.yaml` to customize:

```yaml
command:
  - boltbase
  - worker
  - --worker.labels
  - environment=dev,region=local,cluster=k8s,gpu=false
  - --worker.max-active-runs
  - "50"
```

DAGs can specify required worker labels in their YAML:

```yaml
queue:
  enabled: true
  labels:
    - environment=dev
    - region=local
```

### Environment Variables

Modify `configmap.yaml` to adjust configuration:

- `BOLTBASE_COORDINATOR_HOST`: Bind address for coordinator gRPC server (set to `0.0.0.0` for distributed mode)
- `BOLTBASE_COORDINATOR_ADVERTISE`: Address workers use to connect (set to service name `boltbase-server` for K8s)
- `BOLTBASE_COORDINATOR_PORT`: Coordinator gRPC port (default: 50055)
- `BOLTBASE_PEER_INSECURE`: Use insecure gRPC (true for local)
- `BOLTBASE_TZ`: Timezone

**Important**: For distributed execution to work:
- `BOLTBASE_COORDINATOR_HOST` must be set to `0.0.0.0` (not `127.0.0.1`)
- `BOLTBASE_COORDINATOR_ADVERTISE` should be set to the service DNS name for K8s

After modifying:
```bash
kubectl apply -f deploy/k8s/configmap.yaml
kubectl rollout restart deployment -n boltbase-dev
```

### Using Specific Version

To pin to a specific version instead of `latest`:

```bash
# Update both deployments
kubectl set image deployment/boltbase-server -n boltbase-dev boltbase=ghcr.io/dagu-org/boltbase:v1.14.0
kubectl set image deployment/boltbase-worker -n boltbase-dev boltbase=ghcr.io/dagu-org/boltbase:v1.14.0
```

## Troubleshooting

### Pods Not Starting

```bash
# Check pod events
kubectl describe pod -n boltbase-dev -l app=boltbase

# Check if pods are pulling image
kubectl get pods -n boltbase-dev -w

# If using local image, ensure it's loaded into cluster
kind load docker-image boltbase:local --name boltbase-test
# OR
k3d image import boltbase:local --cluster boltbase-test
```

### Workers Not Connecting

```bash
# Check worker logs for connection errors
kubectl logs -n boltbase-dev -l component=worker

# Verify service DNS resolution
kubectl exec -n boltbase-dev $SERVER_POD -- nslookup boltbase-server

# Check coordinator is listening
kubectl exec -n boltbase-dev $SERVER_POD -- netstat -tlnp | grep 50055
```

### PVC Not Binding

```bash
# Check PVC status
kubectl get pvc -n boltbase-dev

# Check available storage classes
kubectl get storageclass

# For minikube, enable default storage provisioner
minikube addons enable default-storageclass
minikube addons enable storage-provisioner
```

### Cannot Access Web UI

```bash
# For Docker Desktop
curl http://localhost:30080/health

# Port forward as fallback
kubectl port-forward -n boltbase-dev service/boltbase-server 8080:8080

# Check service
kubectl get svc -n boltbase-dev
```

## Cleanup

```bash
# Delete all resources
kubectl delete -f deploy/k8s/

# Or delete namespace (removes everything)
kubectl delete namespace boltbase-dev

# For kind/k3d, delete the entire cluster
kind delete cluster --name boltbase-test
# OR
k3d cluster delete boltbase-test
```

## Advanced: Using with TLS

For production-like testing with TLS:

1. Generate certificates (see Boltbase docs)
2. Create TLS secret:
   ```bash
   kubectl create secret tls boltbase-tls -n boltbase-dev \
     --cert=path/to/cert.pem \
     --key=path/to/key.pem
   ```
3. Update configmap:
   ```yaml
   BOLTBASE_PEER_INSECURE: "false"
   BOLTBASE_PEER_TLS_CERT_FILE: "/etc/tls/tls.crt"
   BOLTBASE_PEER_TLS_KEY_FILE: "/etc/tls/tls.key"
   ```
4. Mount secret in deployments

## Next Steps

- Add more worker replicas for load testing
- Configure worker labels for selective execution
- Set up persistent storage for production
- Enable OpenTelemetry tracing (see `deploy/docker/compose.prod.yaml` for reference)
- Configure Prometheus metrics collection
