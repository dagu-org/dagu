# Features

Dagu provides a comprehensive set of features for building and managing workflows. This section covers everything you need to know about each feature in detail.

## Feature Categories

### 🖥️ [Interfaces](/overview/cli)

How to interact with Dagu:

- **[Command Line Interface](/overview/cli)** - Run and manage workflows from the terminal
- **[Web UI](/overview/web-ui)** - Monitor and control workflows visually  
- **[REST API](/overview/api)** - Integrate Dagu programmatically

### 🔧 [Executors](/features/executors/shell)

Different ways to run your commands:

- **[Shell](/features/executors/shell)** - Run any command or script (default)
- **[Docker](/features/executors/docker)** - Execute in containers for isolation
- **[SSH](/features/executors/ssh)** - Run commands on remote servers
- **[HTTP](/features/executors/http)** - Make REST API calls and webhook requests
- **[Mail](/features/executors/mail)** - Send email notifications and reports
- **[JQ](/features/executors/jq)** - Process and transform JSON data

### ⏰ [Scheduling](/features/scheduling)

Control when workflows run:

- Cron expressions with timezone support
- Multiple schedules per workflow
- Start/stop/restart patterns
- Skip redundant executions

### 🚀 [Execution Control](/features/execution-control)

Advanced execution patterns:

- Parallel execution with concurrency limits
- Conditional execution (preconditions)
- Continue on failure patterns
- Retry and repeat policies
- Output size management
- Signal handling

### 📊 [Data Flow](/features/data-flow)

Managing data in workflows:

- Parameters and runtime values
- Output variables between steps
- Environment variable management
- JSON path references
- Template rendering
- Special system variables

### 📋 [Queue System](/features/queues)

Workflow orchestration at scale:

- Built-in queue management
- Per-DAG queue assignment
- Priority-based execution
- Manual queue operations
- Concurrency control

### 📧 [Notifications](/features/notifications)

Stay informed about workflow status:

- Email alerts on success/failure
- Custom notification handlers
- Log attachments
- Flexible SMTP configuration

### 🔍 [OpenTelemetry Tracing](/features/opentelemetry)

Distributed tracing for observability:

- End-to-end workflow visibility
- Performance bottleneck identification
- Nested DAG correlation
- Integration with Jaeger, Tempo, etc.

### [Distributed Execution](/features/distributed-execution)

Scale workflows across multiple machines:

- Coordinator-worker architecture
- Label-based task routing
- Real-time worker monitoring
- Requires shared storage for DAG files and state
- Horizontal scaling

### [Worker Labels](/features/worker-labels)

Task routing for distributed execution:

- Capability-based worker tagging
- Flexible label matching
- Resource optimization
- Geographic distribution
- Compliance requirements

## Feature Highlights

### Zero Dependencies

Unlike other workflow engines, Dagu requires:
- No database
- No message broker  
- No runtime dependencies
- Just a single binary

### Language Agnostic

Run anything that works on your system:

```yaml
steps:
  - name: python
    command: python script.py

  - name: node
    command: npm run task

  - name: go
    command: go run main.go

  - name: bash
    command: echo "Running script"
```

### Hierarchical Workflows

Build complex systems from simple parts:

```yaml
steps:
  - name: data-pipeline
    run: etl
    params: "DATE=today"
    
  - name: ml-training
    run: train
    parallel: "image text"
    params: "MODEL=latest TYPE=${ITEM}"
    
  - name: deployment
    run: deploy
    parallel: "staging production"
    params: "ENV=${ITEM}"

---
name: etl
params:
  - DATE: today
steps:
  - name: etl
    command: python etl.py

---
name: train
params:
  - MODEL: latest
  - TYPE: ""
steps:
  - name: train
    command: python train.py --model ${MODEL} --type ${TYPE}

---
name: deploy
params:
  - ENV: ""
steps:
  - name: deploy
    command: kubectl apply -f deployment.yaml --env ${ENV}
```

## See Also

Explore specific features:

- [Command Line Interface](/overview/cli) - Master the CLI
- [Shell Executor](/features/executors/shell) - Run commands effectively
- [Scheduling](/features/scheduling) - Automate workflow execution
- [Execution Control](/features/execution-control) - Advanced patterns
