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
    command: ./batch-process.sh
```

### Disable Queueing

For critical workflows that should always run:

```yaml
name: critical-alert
maxActiveRuns: -1           # Never queue - always run

steps:
  - name: check
    command: ./check-alerts.sh
```

## Global Queue Configuration

Configure queues in server config:

```yaml
# ~/.config/dagu/config.yaml
queues:
  enabled: true
  config:
    - name: "critical"
      maxConcurrency: 5     # 5 critical jobs concurrently
    - name: "batch"
      maxConcurrency: 1     # One batch job at a time
    - name: "reporting"
      maxConcurrency: 3     # 3 reports concurrently
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

### View Queue Status

Check queue status in the web UI or via API:

```bash
# List queued items
curl http://localhost:8080/api/v1/queues
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

### Resource Isolation

Separate queues for different resources:

```yaml
# CPU intensive
name: data-processing
queue: "cpu-intensive"
maxActiveRuns: 2

# Memory intensive
name: analytics
queue: "memory-intensive"
maxActiveRuns: 1

# I/O intensive
name: file-sync
queue: "io-intensive"
maxActiveRuns: 5
```

## Common Patterns

### Batch Processing Queue

```yaml
name: nightly-batch
queue: "batch"
maxActiveRuns: 1
schedule: "0 2 * * *"

steps:
  - name: process-all
    command: ./batch-process.sh
```

### Multi-Environment Deployment

```yaml
# Staging deployments
name: deploy-staging
queue: "staging-deploy"
maxActiveRuns: 2

# Production deployments
name: deploy-prod
queue: "prod-deploy"
maxActiveRuns: 1  # One at a time for safety
```

### Report Generation

```yaml
name: generate-report
queue: "reporting"
maxActiveRuns: 3
params:
  - REPORT_TYPE: daily

steps:
  - name: generate
    command: ./generate-report.sh ${REPORT_TYPE}
```

## Queue Monitoring

### Web UI

- View queued workflows in the dashboard
- See queue depths and wait times
- Monitor queue processing rates

### Metrics

Track queue performance:
- Queue depth over time
- Average wait time
- Processing throughput
- Queue utilization

## Best Practices

1. **Use Queues for Resource Management**
   - Prevent system overload
   - Ensure fair resource allocation

2. **Separate Critical from Batch**
   - Critical workflows: `maxActiveRuns: -1`
   - Batch workflows: Use queues

3. **Monitor Queue Depths**
   - Long queues indicate resource constraints
   - Adjust concurrency limits as needed

4. **Use Meaningful Queue Names**
   - `batch`, `reporting`, `deployment`
   - Not `queue1`, `queue2`

5. **Test Queue Behavior**
   - Verify workflows queue correctly
   - Test with expected load

## Troubleshooting

### Workflows Not Starting

Check:
- Queue configuration exists
- `maxActiveRuns` is not 0
- No other workflows blocking the queue

### Queue Growing Too Large

Solutions:
- Increase `maxConcurrency` for the queue
- Add more processing resources
- Optimize workflow execution time

### Workflows Skipping Queue

Verify:
- `maxActiveRuns` is not -1
- Workflow is assigned to a queue
- Queue is enabled in config

## Next Steps

- [Execution Control](/features/execution-control) - Control workflow execution
- [Scheduling](/features/scheduling) - Schedule workflows
- [Configuration Reference](/configurations/reference) - Complete configuration guide