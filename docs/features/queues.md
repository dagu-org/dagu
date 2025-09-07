# Queue Management

Control concurrent execution and resource usage with queues.

## Overview

Dagu's queue system helps you:
- Limit concurrent workflow executions
- Manage resource usage
- Prioritize critical workflows
- Prevent system overload

**Additional Note: Queue-level `maxConcurrency` vs DAG-level `maxActiveRuns`:**

At the queue level, `maxConcurrency` is enforced by the scheduler process.  
- If a queue has a defined `maxConcurrency`, any DAG assigned to that queue can run in parallel up to the queue’s limit, regardless of its own DAG-level `maxActiveRuns`.  
- If a DAG is not assigned to a queue (i.e., no `queue: <string>` is set), it follows its own `maxActiveRuns` setting (default: `1`).  

When starting a DAG run (via API or CLI) that belongs to a queue and has `maxActiveRuns > 0`, the system checks the sum of:  
1. The DAG’s queued runs within that queue, plus  
2. Its currently running instances.  

If the new run would push this total beyond the DAG-level `maxActiveRuns` and the DAG is assigned to a queue, the request is rejected with an error. This enforces DAG-level `maxActiveRuns` will not be exceeded effectively.

## Basic Queue Configuration

### Assign to Queue

```yaml
queue: "batch"              # Assign to batch queue
maxActiveRuns: 2            # Allow 2 concurrent runs
schedule: "*/10 * * * *"    # Every 10 minutes

steps:
  - echo "Processing batch"
```

### Disable Queueing

For critical workflows that should always run:

```yaml
# critical-alert
maxActiveRuns: -1           # Never queue - always run

steps:
  - echo "Checking alerts"
```

## Global Queue Configuration

Configure queues in server config:

```yaml
# ~/.config/dagu/config.yaml
queues:
  enabled: true
  config:
    - name: "critical"
      maxConcurrency: 5      # 5 critical jobs concurrently
    - name: "batch"
      maxConcurrency: 1      # One batch job at a time
    - name: "reporting"
      maxConcurrency: 3      # 3 reports concurrently
```

## Default Queue via Base Config

Set default queue for all workflows:

```yaml
# ~/.config/dagu/base.yaml
queue: "default"
maxActiveRuns: 2

# All DAGs inherit these settings
```

## Manual Queue Management

### Enqueue Workflows

```bash
# Basic enqueue
dagu enqueue workflow.yaml

# With custom run ID
dagu enqueue workflow.yaml --run-id=batch-2024-01-15

# With parameters
dagu enqueue process.yaml -- DATE=2024-01-15 TYPE=daily

# Override the queue at enqueue-time
dagu enqueue --queue=high-priority workflow.yaml
```

### Remove from Queue

```bash
# Remove by DAG name
dagu dequeue workflow.yaml

# Remove specific run
dagu dequeue --dag-run=workflow:batch-2024-01-15
```
