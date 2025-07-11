# Worker Labels

Worker labels allow you to tag workers with specific capabilities and route tasks to appropriate workers based on their requirements.

## Configuring Worker Labels

### Command Line

Specify labels when starting a worker using the `--worker-labels` flag:

```bash
# GPU-enabled worker
dagu worker --worker-labels gpu=true,memory=64G,region=us-east-1

# CPU-optimized worker  
dagu worker --worker-labels cpu-arch=amd64,cpu-cores=16,instance-type=m5.large

# Region-specific worker
dagu worker --worker-labels region=eu-west-1,compliance=gdpr
```

### Configuration File

Set labels in the configuration file:

```yaml
# config.yaml
worker:
  labels:
    gpu: "true"
    memory: "64G"
    region: "us-east-1"
```

### Environment Variable

Set labels via environment variable:

```bash
export DAGU_WORKER_LABELS="gpu=true,memory=64G"
dagu worker
```

## Using Worker Selectors in DAGs

Specify `workerSelector` on any step to route it to workers with matching labels:

```yaml
steps:
  # This task will only run on workers with gpu=true label
  - name: train-model
    command: python train.py
    workerSelector:
      gpu: "true"
      memory: "64G"
  
  # This task requires a specific region
  - name: process-eu-data
    command: ./process_data.sh
    workerSelector:
      region: "eu-west-1"
      compliance: "gdpr"
  
  # This task can run on any worker (no selector)
  - name: send-notification
    command: notify.sh
```

## Label Matching Rules

1. **All labels must match**: A worker must have ALL labels specified in the `workerSelector` to be eligible
2. **Empty selector**: Tasks without `workerSelector` can run on any worker
3. **Exact match**: Label values must match exactly (case-sensitive)

## Example Use Cases

### GPU/CPU Task Routing
```yaml
# GPU worker
dagu worker --worker-labels gpu=true,cuda=11.8

# CPU worker  
dagu worker --worker-labels cpu-only=true,avx2=true

# DAG
steps:
  - name: gpu-task
    workerSelector:
      gpu: "true"
  - name: cpu-task
    workerSelector:
      cpu-only: "true"
```

### Multi-Region Deployment
```yaml
# US worker
dagu worker --worker-labels region=us-east-1,az=us-east-1a

# EU worker
dagu worker --worker-labels region=eu-west-1,az=eu-west-1a

# DAG with region requirements
steps:
  - name: process-us-data
    workerSelector:
      region: "us-east-1"
  - name: process-eu-data
    workerSelector:
      region: "eu-west-1"
```

### Resource-Based Routing
```yaml
# High-memory worker
dagu worker --worker-labels memory=128G,instance-type=r5.4xlarge

# Standard worker
dagu worker --worker-labels memory=16G,instance-type=m5.large

# Memory-intensive task
steps:
  - name: large-dataset-analysis
    workerSelector:
      memory: "128G"
```