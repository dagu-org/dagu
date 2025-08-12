# Execution Control

Advanced control over how your workflows execute.

## Parallel Execution

Execute the same workflow with different parameters in parallel.

### Basic Usage

```yaml
steps:
  - name: process-files
    run: file-processor
    parallel:
      items:
        - "file1.csv"
        - "file2.csv"
        - "file3.csv"
    params: "FILE=${ITEM}"

---
name: file-processor
params:
  - FILE: ""
steps:
  - name: process
    command: python process.py --file ${FILE}
```

### With Concurrency Control

```yaml
steps:
  - name: process-files
    run: file-processor
    parallel:
      items: ${FILE_LIST}
      maxConcurrent: 2  # Process max 2 files at a time
    params: "FILE=${ITEM}"
```

### Dynamic Items

```yaml
steps:
  - name: find-files
    command: find /data -name "*.csv" -type f
    output: CSV_FILES
  
  - name: process-files
    run: file-processor
    parallel: ${CSV_FILES}
    params: "FILE=${ITEM}"
```

### Capturing Output

```yaml
steps:
  - name: parallel-tasks
    run: task-processor
    parallel:
      items: [1, 2, 3]
    output: RESULTS
  
  - name: summary
    command: |
      echo "Total: ${RESULTS.summary.total}"
      echo "Succeeded: ${RESULTS.summary.succeeded}"
      echo "Failed: ${RESULTS.summary.failed}"
```

Output structure:
```json
{
  "summary": {
    "total": 3,
    "succeeded": 3,
    "failed": 0
  },
  "outputs": [
    {"RESULT": "output1"},
    {"RESULT": "output2"},
    {"RESULT": "output3"}
  ]
}
```

## Maximum Active Steps

Control how many steps run concurrently:

```yaml
maxActiveSteps: 2  # Run up to 2 steps in parallel

steps:
  - name: task1
    command: echo "Running task 1"
    depends: [] # Explicitly declare no dependency
  - name: task2
    command: echo "Running task 2"
    depends: []
  - name: task3
    command: echo "Running task 3"
    depends: []
  - name: task4
    command: echo "Running task 4"
    depends: []
  # All start in parallel, limited by maxActiveSteps
```

## Maximum Active Runs

Control concurrent workflow instances:

```yaml
maxActiveRuns: 1  # Only one instance at a time

schedule: "*/5 * * * *"  # Every 5 minutes
```

Options:
- `1`: Only one instance (default)
- `N`: Allow N concurrent instances
- `-1`: Unlimited instances

## Queue Management

Assign workflows to queues:

```yaml
queue: "batch"
maxActiveRuns: 2
```

Manual queue control:
```bash
# Enqueue with custom ID
dagu enqueue workflow.yaml --run-id=custom-id

# Remove from queue
dagu dequeue --dag-run=workflow:custom-id
```

## Timeout Control

Set execution time limits:

### Workflow Timeout

```yaml
timeoutSec: 3600  # 1 hour timeout

steps:
  - name: long-task
    command: echo "Processing"
```

### Cleanup Timeout

```yaml
maxCleanUpTimeSec: 300  # 5 minutes for cleanup

handlerOn:
  exit:
    command: echo "Cleaning up"  # Must finish within 5 minutes
```

## Initial Delay

Delay workflow start:

```yaml
delaySec: 60  # Wait 60 seconds before starting

steps:
  - name: delayed-task
    command: echo "Running task"
```

## Execution Order

### Sequential Execution

```yaml
steps:
  - name: first
    command: echo "1"
  - name: second
    command: echo "2"
    depends: first
  - name: third
    command: echo "3"
    depends: second
```

### Parallel with Dependencies

```yaml
steps:
  - name: setup
    command: echo "Setting up"
  
  - name: task-a
    command: echo "Running task A"
    depends: setup
  
  - name: task-b
    command: echo "Running task B"
    depends: setup
  
  - name: finalize
    command: echo "Finalizing"
    depends:
      - task-a
      - task-b
```

## Retry and Repeat Control

### Exponential Backoff

Control retry and repeat intervals with exponential backoff to avoid overwhelming systems:

#### Retry with Backoff

```yaml
steps:
  # API call with exponential backoff
  - name: resilient-api-call
    command: curl https://api.example.com/data
    retryPolicy:
      limit: 6
      intervalSec: 1
      backoff: 2.0         # Double interval each time
      maxIntervalSec: 60   # Cap at 60 seconds
      exitCode: [429, 503] # Rate limit or service unavailable
      # Intervals: 1s, 2s, 4s, 8s, 16s, 32s → 60s
```

#### Repeat with Backoff

```yaml
steps:
  # Service health check with backoff
  - name: wait-for-healthy
    command: echo "Health check OK"
    output: STATUS
    repeatPolicy:
      repeat: until
      condition: "${STATUS}"
      expected: "healthy"
      intervalSec: 2
      backoff: 1.5         # Gentler backoff (1.5x)
      maxIntervalSec: 120  # Cap at 2 minutes
      limit: 50
      # Intervals: 2s, 3s, 4.5s, 6.75s, 10.125s...
```

#### Practical Examples

**Database Connection Retry**:
```yaml
steps:
  - name: connect-db
    command: psql -h db.example.com -c "SELECT 1"
    retryPolicy:
      limit: 10
      intervalSec: 0.5
      backoff: true        # true = 2.0 multiplier
      maxIntervalSec: 30
      # Quick initial retries, backing off to 30s max
```

**Service Startup Monitoring**:
```yaml
steps:
  - name: start-service
    command: systemctl start myservice
    
  - name: wait-for-ready
    command: systemctl is-active myservice
    repeatPolicy:
      repeat: until
      exitCode: [0]        # Exit 0 means service is active
      intervalSec: 1
      backoff: 2.0
      maxIntervalSec: 60
      limit: 30
      # Check frequently at first, then less often
```

**API Polling with Rate Limit Awareness**:
```yaml
steps:
  - name: poll-job-status
    command: |
      response=$(curl -s https://api.example.com/job/123)
      echo "$response" | jq -r '.status'
    output: JOB_STATUS
    repeatPolicy:
      repeat: until
      condition: "${JOB_STATUS}"
      expected: "re:completed|failed"
      intervalSec: 5
      backoff: 1.5
      maxIntervalSec: 300  # Max 5 minutes between checks
      limit: 100
```

### Backoff Benefits

1. **Resource Efficiency**: Reduces load on failing services
2. **Cost Optimization**: Fewer API calls means lower costs
3. **Better Recovery**: Gives services time to recover
4. **Rate Limit Compliance**: Naturally backs off when hitting limits
5. **Network Stability**: Reduces network congestion during outages

### Configuration Tips

- **Start Small**: Begin with short intervals for quick recovery
- **Choose Multipliers**: 
  - `2.0`: Standard exponential (1, 2, 4, 8...)
  - `1.5`: Gentler increase (1, 1.5, 2.25...)
  - `3.0`: Aggressive backoff (1, 3, 9, 27...)
- **Set Caps**: Always use `maxIntervalSec` to prevent excessive waits
- **Consider Limits**: Set reasonable retry/repeat limits

## See Also

- [Error Handling](/writing-workflows/error-handling) - Handle failures gracefully
- [Scheduling](/features/scheduling) - Schedule workflow execution
- [Queues](/features/queues) - Detailed queue management
