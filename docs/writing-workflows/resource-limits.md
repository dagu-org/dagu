# Resource Limits

Manage workflow execution with concurrency limits.

## Concurrency Control

```yaml
maxActiveRuns: 1        # Only one instance of this DAG
maxActiveSteps: 1       # Max 1 steps running concurrently

steps:
  - sh -c "echo Starting heavy computation; sleep 3; echo Completed"
  - echo "Processing large dataset"
  - parallel:
      items: ${FILE_LIST}
      maxConcurrent: 3  # Limit parallel I/O
    command: echo "Processing file ${ITEM}"
```

## Limit by Queue

```yaml
queue: heavy-jobs       # Assign to specific queue
maxActiveRuns: 2        # Queue allows 2 concurrent

steps:
  - sh -c "echo Starting intensive task; sleep 10; echo Done"
  - echo "Quick task"
```

You can define queues in the global configuration to set concurrency limits.

```yaml
queues:
  enable: true
  heavy-jobs:
    maxConcurrency: 2
  light-jobs:
    maxConcurrency: 10 
```

See [Queue System](/features/queues) for details.
