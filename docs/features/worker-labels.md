# Worker Labels

Worker labels allow you to tag workers with specific capabilities and route tasks to appropriate workers based on their requirements.

::: tip
Worker labels are a key component of Dagu's [Distributed Execution](/features/distributed-execution) feature. Make sure to understand the distributed execution architecture before implementing worker labels.
:::

## Configuring Worker Labels

### Command Line

Specify labels when starting a worker using the `--worker-labels` flag:

```bash
# GPU-enabled worker
dagu worker --worker.labels gpu=true,memory=64G,region=us-east-1

# CPU-optimized worker  
dagu worker --worker.labels cpu-arch=amd64,cpu-cores=16,instance-type=m5.large

# Region-specific worker
dagu worker --worker.labels region=eu-west-1,compliance=gdpr
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
  - call: train

---
name: train
workerSelector:
  gpu: "true"
steps:
  - python train.py

```

## Label Matching Rules

1. **All labels must match**: A worker must have ALL labels specified in the `workerSelector` to be eligible
2. **Empty selector**: Tasks without `workerSelector` can run on any worker
3. **Exact match**: Label values must match exactly (case-sensitive)

## Example Use Cases

### GPU/CPU Task Routing
```yaml
# GPU worker
dagu worker --worker.labels gpu=true

# CPU worker  
dagu worker --worker.labels cpu=true

# DAG
steps:
  - call: gpu-task
  - call: cpu-task

---
# Run on a worker with gpu
name: gpu-task
workerSelector:
  gpu-task: "true"
steps:
  - python gpu-task.py

---
# Run on a worker with faster cpu
name: cpu-task
workerSelector:
  cpu-task: "true"
steps:
  - python cpu-task.py
```
