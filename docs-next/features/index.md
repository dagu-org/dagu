# Features

Deep dive into all Dagu capabilities.

## Overview

Dagu provides a comprehensive set of features for building and managing workflows. This section covers everything you need to know about each feature in detail.

## Feature Categories

### 🖥️ [Interfaces](/features/interfaces/cli)

How to interact with Dagu:

- **[Command Line Interface](/features/interfaces/cli)** - Run and manage workflows from the terminal
- **[Web UI](/features/interfaces/web-ui)** - Monitor and control workflows visually  
- **[REST API](/features/interfaces/api)** - Integrate Dagu programmatically

### 🔧 [Executors](/features/executors/shell)

Different ways to run your commands:

- **[Shell](/features/executors/shell)** - Run any command or script (default)
- **[Docker](/features/executors/docker)** - Execute in containers for isolation
- **[SSH](/features/executors/ssh)** - Run commands on remote servers
- **[HTTP](/features/executors/http)** - Make REST API calls and webhook requests
- **[Mail](/features/executors/mail)** - Send email notifications and reports
- **[JQ](/features/executors/jq)** - Process and transform JSON data
- **[DAG](/features/executors/dag)** - Execute nested workflows for composition

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

## Feature Highlights

### Zero Dependencies

Unlike other workflow engines, Dagu requires:
- ❌ No database
- ❌ No message broker  
- ❌ No runtime dependencies
- ✅ Just a single binary

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
    command: ./script.sh
```

### Hierarchical Workflows

Build complex systems from simple parts:

```yaml
steps:
  - name: data-pipeline
    run: workflows/etl.yaml
    params: "DATE=today"
    
  - name: ml-training
    run: workflows/train.yaml
    depends: data-pipeline
    
  - name: deployment
    run: workflows/deploy.yaml
    depends: ml-training
```

### Production Ready

Built for reliability:

- **Process Management**: Proper signal handling and graceful shutdown
- **Error Recovery**: Configurable retry policies and failure handlers
- **Logging**: Comprehensive logs with stdout/stderr separation
- **Monitoring**: Built-in metrics and health checks

## Common Use Cases

### Data Engineering
- ETL pipelines with dependency management
- Parallel data processing
- Scheduled batch jobs
- Data quality checks

### DevOps Automation
- CI/CD pipelines
- Infrastructure provisioning
- Backup and restore workflows
- System maintenance tasks

### Business Process Automation
- Report generation
- Data synchronization
- Customer onboarding
- Invoice processing

## Performance Characteristics

### Scalability
- Handle thousands of concurrent workflows
- Efficient file-based storage
- Minimal memory footprint
- Fast startup times

### Limitations
- Single-machine execution (no distributed mode)
- 1MB default output limit per step
- 1000 item limit for parallel execution
- File system dependent

## Getting Started with Features

1. **Start with the basics**: Learn about [Interfaces](/features/interfaces/cli) to interact with Dagu
2. **Choose your executor**: Pick the right [Executor](/features/executors/shell) for your tasks
3. **Add scheduling**: Set up [automatic execution](/features/scheduling)
4. **Handle errors**: Implement proper [retry and error handling](/features/execution-control)
5. **Scale up**: Use [queues](/features/queues) for complex orchestration

## Feature Comparison

| Feature | Dagu | Airflow | GitHub Actions | Cron |
|---------|------|---------|----------------|------|
| Local Development | ✅ | ❌ | ❌ | ✅ |
| Web UI | ✅ | ✅ | ✅ | ❌ |
| Dependencies | ✅ | ✅ | ✅ | ❌ |
| Retries | ✅ | ✅ | ✅ | ❌ |
| Parallel Execution | ✅ | ✅ | ✅ | ❌ |
| No External Services | ✅ | ❌ | ❌ | ✅ |
| Language Agnostic | ✅ | ❌ | ✅ | ✅ |

## Next Steps

Explore specific features:

- [Command Line Interface](/features/interfaces/cli) - Master the CLI
- [Shell Executor](/features/executors/shell) - Run commands effectively
- [Scheduling](/features/scheduling) - Automate workflow execution
- [Execution Control](/features/execution-control) - Advanced patterns