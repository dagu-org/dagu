# Execution Control

Advanced control over how your workflows execute.

## Parallel Execution

Execute the same workflow with different parameters in parallel.

### Basic Usage

```yaml
steps:
  - call: file-processor
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
  - python process.py --file ${FILE}
```

### With Concurrency Control

```yaml
steps:
  - call: file-processor
    parallel:
      items: ${FILE_LIST}
      maxConcurrent: 2  # Process max 2 files at a time
    params: "FILE=${ITEM}"
```

### Dynamic Items

```yaml
steps:
  - command: find /data -name "*.csv" -type f
    output: CSV_FILES
  
  - call: file-processor
    parallel: ${CSV_FILES}
    params: "FILE=${ITEM}"
```

### Capturing Output

```yaml
steps:
  - call: task-processor
    parallel:
      items: [1, 2, 3]
    output: RESULTS
  
  - |
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
  - command: echo "Running task 1"
    depends: [] # Explicitly declare no dependency
  - command: echo "Running task 2"
    depends: []
  - command: echo "Running task 3"
    depends: []
  - command: echo "Running task 4"
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
  - echo "Processing"
```

### Cleanup Timeout

By default DAGs wait 5 seconds for cleanup before forcefully terminating steps. Increase `maxCleanUpTimeSec` if your handlers need longer.

```yaml
maxCleanUpTimeSec: 300  # allow 5 minutes for cleanup (default timeout: 5s)

handlerOn:
  exit:
    command: echo "Cleaning up"  # Must finish within 5 minutes
```

## Initial Delay

Delay workflow start:

```yaml
delaySec: 60  # Wait 60 seconds before starting

steps:
  - echo "Running task"
```

## Execution Order

### Sequential Execution

```yaml
steps:
  - echo "1"
  - echo "2"  # Runs after step 1
  - echo "3"  # Runs after step 2
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
  
  - command: echo "Finalizing"
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
  - command: curl https://api.example.com/data
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
  - command: echo "Health check OK"
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
