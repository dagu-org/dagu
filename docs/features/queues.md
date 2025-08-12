# Queue Management

Control concurrent execution and resource usage with queues.

## Overview

Dagu's queue system helps you:
- Limit concurrent workflow executions
- Manage resource usage
- Prioritize critical workflows
- Prevent system overload

## Basic Queue Configuration

### Assign to Queue

```yaml
name: batch-processor
queue: "batch"              # Assign to batch queue
maxActiveRuns: 2            # Allow 2 concurrent runs
schedule: "*/10 * * * *"    # Every 10 minutes

steps:
  - name: process
    command: echo "Processing batch"
```

### Disable Queueing

For critical workflows that should always run:

```yaml
name: critical-alert
maxActiveRuns: -1           # Never queue - always run

steps:
  - name: check
    command: echo "Checking alerts"
```

## Global Queue Configuration

Configure queues in server config:

```yaml
# ~/.config/dagu/config.yaml
queues:
  enabled: true
  config:
    - name: "critical"
      maxActiveRuns: 5      # 5 critical jobs concurrently
    - name: "batch"
      maxActiveRuns: 1      # One batch job at a time
    - name: "reporting"
      maxActiveRuns: 3      # 3 reports concurrently
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
```

### Remove from Queue

```bash
# Remove by DAG name
dagu dequeue workflow.yaml

# Remove specific run
dagu dequeue --dag-run=workflow:batch-2024-01-15
```

## Queue Behavior

### FIFO Processing

Queues process in First-In-First-Out order:

```yaml
# First workflow submitted
name: report-1
queue: "reporting"
maxActiveRuns: 1

# Second workflow submitted
name: report-2
queue: "reporting"
maxActiveRuns: 1

# report-1 runs first, report-2 waits
```

### Priority Handling

Use different queues for priority:

```yaml
# High priority
name: critical-job
queue: "high-priority"
maxActiveRuns: 5

# Low priority
name: maintenance
queue: "low-priority"
maxActiveRuns: 1
```

## See Also

- [Execution Control](/features/execution-control) - Control workflow execution
- [Scheduling](/features/scheduling) - Schedule workflows
- [Configuration Reference](/configurations/reference) - Complete configuration guide
