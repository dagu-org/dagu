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
```

### With Concurrency Control

```yaml
steps:
  - name: process-files
    run: file-processor
    parallel:
      items: ${FILE_LIST}
      maxConcurrent: 2  # Process max 2 files at a time
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
```

### Using the $ITEM Variable

Access the current item in the parent DAG's params field:

```yaml
steps:
  - name: process-files
    run: file-processor
    parallel:
      items: ["/path/file1.csv", "/path/file2.csv"]
    params:
      - FILE: ${ITEM}
      - OUTPUT_DIR: /processed
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
maxActiveSteps: 5  # Run up to 5 steps in parallel

steps:
  - name: task1
    command: ./task1.sh
  - name: task2
    command: ./task2.sh
  - name: task3
    command: ./task3.sh
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
    command: ./process.sh
```

### Cleanup Timeout

```yaml
maxCleanUpTimeSec: 300  # 5 minutes for cleanup

handlerOn:
  exit:
    command: ./cleanup.sh  # Must finish within 5 minutes
```

## Initial Delay

Delay workflow start:

```yaml
delaySec: 60  # Wait 60 seconds before starting

steps:
  - name: delayed-task
    command: ./task.sh
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
    command: ./setup.sh
  
  - name: task-a
    command: ./task-a.sh
    depends: setup
  
  - name: task-b
    command: ./task-b.sh
    depends: setup
  
  - name: finalize
    command: ./finalize.sh
    depends:
      - task-a
      - task-b
```

## See Also

- [Error Handling](/writing-workflows/error-handling) - Handle failures gracefully
- [Scheduling](/features/scheduling) - Schedule workflow execution
- [Queues](/features/queues) - Detailed queue management
