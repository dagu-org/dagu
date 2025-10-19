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
  - curl -O https://example.com/data.csv  # Download data
  - python analyze.py data.csv           # Process data
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

```yaml
params:
  - ENV: dev
  - REGION: us-east-1

steps:
  - echo "Deploying to ${ENV} in ${REGION}"
```

Override at runtime:
```bash
dagu start workflow.yaml -- ENV=prod REGION=eu-west-1
```

### Variables

Pass data between steps using `output`:

```yaml
steps:
  - command: date +%Y%m%d
    output: TODAY
  - tar -czf backup_${TODAY}.tar.gz /data
```

## Status Management

### Execution States

- `not_started`: DAG has been defined but execution has not begun
- `queued`: DAG is waiting to be executed
- `running`: DAG is currently executing
- `succeeded`: All steps completed successfully
- `partially_succeeded`: Some steps failed but execution continued (via `continueOn`)
- `failed`: DAG execution failed
- `canceled`: DAG was manually canceled

### Status Transitions

```mermaid
graph LR
    N[not_started] --> Q[queued]
    Q --> R[running]
    R --> S[succeeded]
    R --> PS[partially_succeeded]
    R --> F[failed]
    R --> C[canceled]
```

### Step Status

- `not_started`: Step is waiting for dependencies
- `running`: Step is executing
- `succeeded`: Step completed successfully
- `partially_succeeded`: Step completed with warnings or continue-on logic
- `failed`: Step execution failed
- `canceled`: Step was canceled
- `skipped`: Step was skipped (precondition not met)

### Status Hooks

```yaml
handlerOn:
  success:
    command: notify-team.sh "Workflow succeeded"
  failure:
    command: alert-oncall.sh "Workflow failed"
  partial_success:
    command: log-partial.sh "Some steps partially succeeded"
```

## Executors

### Shell (Default)

Runs commands in the system shell:

```yaml
steps:
  - command: echo "Hello"
    shell: bash  # or sh, zsh
```

See [Shell Executor](/features/executors/shell) for more details.

### Docker

Execute in containers:

```yaml
container:
  image: python:3.11
  workingDir: /app
  volumes:
    - /app/data:/data
steps:
  - python script.py
```

See [Docker Executor](/features/executors/docker) for more details.

### SSH

Run on remote machines:

```yaml
ssh:
  user: ubuntu
  host: server.example.com
  key: ~/.ssh/id_rsa

steps:
  - echo "Running remote script"
```

See [SSH Executor](/features/executors/ssh) for more details.

### HTTP

Make API calls:

```yaml
steps:
  - executor:
      type: http
      config:
        method: POST
        url: https://api.example.com/trigger
        headers:
          Authorization: Bearer ${API_TOKEN}
```

See [HTTP Executor](/features/executors/http) for more details.

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

See [Scheduling](/features/scheduling) for more details.

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
    command: echo "Cleaning up resources"
    
  exit:
    command: echo "Always runs"
```

## See Also

- [Writing Workflows](/writing-workflows/) - Create your own workflows
- [Examples](/writing-workflows/examples) - Ready-to-use patterns
- [CLI Reference](/reference/cli) - Command-line usage
