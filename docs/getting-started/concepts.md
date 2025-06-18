# Core Concepts

Essential concepts for working with Dagu.

## DAG (Directed Acyclic Graph)

A DAG defines your workflow as a graph of dependencies:

- **Directed**: Steps execute in a specific order
- **Acyclic**: No circular dependencies allowed
- **Graph**: Steps connected by dependency relationships

```mermaid
graph LR
    A[Extract] --> B[Transform]
    B --> C[Load]
    B --> D[Report]
    C --> E[Notify]
    D --> E
```

## Workflow Components

### Steps

The basic unit of execution. Each step runs a command:

```yaml
steps:
  - name: download
    command: curl -O https://example.com/data.csv
    
  - name: process
    command: python analyze.py data.csv
```

### Dependencies

By default, steps run sequentially. Use `depends` for parallel execution:

```yaml
steps:
  - name: A
    command: echo "First"
    
  - name: B
    command: echo "Second (after A)"
    
  - name: C
    command: echo "Parallel with B"
    depends: A  # Only depends on A, runs parallel to B
    
  - name: D
    command: echo "After both B and C"
    depends: [B, C]
```

### Parameters

Make workflows reusable with parameters:

```yaml
params:
  - ENV: dev
  - REGION: us-east-1

steps:
  - name: deploy
    command: ./deploy.sh ${ENV} ${REGION}
```

Override at runtime:
```bash
dagu start workflow.yaml -- ENV=prod REGION=eu-west-1
```

### Variables

Pass data between steps using `output`:

```yaml
steps:
  - name: get-date
    command: date +%Y%m%d
    output: TODAY
    
  - name: backup
    command: tar -czf backup_${TODAY}.tar.gz /data
```

## Execution Model

### Parallel Execution

Steps with the same dependencies run in parallel:

```yaml
steps:
  - name: start
    command: echo "Begin"
    
  - name: task1
    command: ./heavy-task-1.sh
    depends: start
    
  - name: task2
    command: ./heavy-task-2.sh
    depends: start
    
  - name: finish
    command: echo "All done"
    depends: [task1, task2]
```

Control parallelism with `maxActiveSteps`:

```yaml
maxActiveSteps: 2  # Only 2 steps run concurrently
```

### Conditional Execution

Run steps based on conditions:

```yaml
steps:
  - name: process
    command: python process.py
    preconditions:
      - condition: "test -f /data/input.csv"
```

The `process` step only runs if the precondition matches.

### Error Handling

Built-in retry mechanism:

```yaml
steps:
  - name: fetch
    command: curl -f https://flaky-api.com/data
    retryPolicy:
      limit: 3
      intervalSec: 30
```

Continue on failure:

```yaml
steps:
  - name: optional-task
    command: ./might-fail.sh
    continueOn:
      failure: true
```

The workflow continues even if `optional-task` fails. The overall status will be `partial success` if any step fails but does not block the execution of subsequent steps due to `continueOn`.

## Executors

### Shell (Default)

Runs commands in the system shell:

```yaml
steps:
  - name: example
    command: echo "Hello"
    shell: bash  # or sh, zsh
```

See [Shell Executor](/features/executors/shell/) for more details.

### Docker

Execute in containers:

```yaml
steps:
  - name: python-task
    executor:
      type: docker
      config:
        image: python:3.11
        volumes:
          - /data:/data
    command: python script.py
```

See [Docker Executor](/features/executors/docker/) for more details.

### SSH

Run on remote machines:

```yaml
steps:
  - name: remote-task
    executor:
      type: ssh
      config:
        user: ubuntu
        host: server.example.com
        key: ~/.ssh/id_rsa
    command: ./remote-script.sh
```

See [SSH Executor](/features/executors/ssh/) for more details.

### HTTP

Make API calls:

```yaml
steps:
  - name: webhook
    executor:
      type: http
      config:
        method: POST
        url: https://api.example.com/trigger
        headers:
          Authorization: Bearer ${API_TOKEN}
```

See [HTTP Executor](/features/executors/http/) for more details.

## Scheduling

Cron-based scheduling:

```yaml
schedule: "0 2 * * *"  # Daily at 2 AM
```

Multiple schedules:

```yaml
schedule:
  - "0 9 * * MON-FRI"  # Weekdays at 9 AM
  - "0 14 * * SAT,SUN" # Weekends at 2 PM
```

Start/stop schedules:

```yaml
schedule:
  start: "0 8 * * *"   # Start at 8 AM
  stop: "0 18 * * *"   # Stop at 6 PM
```

See [Scheduling](/features/scheduling/) for more details.

## Lifecycle Handlers

Execute commands on workflow events:

```yaml
handlerOn:
  success:
    command: echo "Workflow succeeded"
    
  failure:
    command: |
      echo "Workflow failed" | mail -s "Alert" admin@example.com
      
  cancel:
    command: ./cleanup.sh
    
  exit:
    command: echo "Always runs"
```

## What's Next?

- [Writing Workflows](/writing-workflows/) - Create your own workflows
- [Examples](/writing-workflows/examples/) - Ready-to-use patterns
- [CLI Reference](/reference/cli) - Command-line usage
