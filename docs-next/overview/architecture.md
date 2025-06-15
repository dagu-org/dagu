# Architecture

Understanding how Dagu works under the hood.

## Design Philosophy

Dagu follows a simple philosophy: **do one thing well with minimal dependencies**. Unlike traditional workflow orchestrators that require complex distributed systems, Dagu runs as a single process that manages everything locally.

## System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         User Interfaces                      │
├─────────────┬──────────────────┬────────────────────────────┤
│     CLI     │     Web UI       │         REST API           │
└─────────────┴──────────────────┴────────────────────────────┘
                              │
┌─────────────────────────────┴───────────────────────────────┐
│                        Core Engine                           │
├─────────────┬──────────────────┬────────────────────────────┤
│  Scheduler  │  Executor        │    Queue Manager           │
├─────────────┼──────────────────┼────────────────────────────┤
│  DAG Parser │  Process Manager │    State Manager           │
└─────────────┴──────────────────┴────────────────────────────┘
                              │
┌─────────────────────────────┴───────────────────────────────┐
│                      Storage Layer                           │
├─────────────┬──────────────────┬────────────────────────────┤
│  DAG Files  │    Log Files     │    State Files             │
└─────────────┴──────────────────┴────────────────────────────┘
```

## Core Components

### 1. DAG Parser
- Reads and validates YAML workflow definitions
- Builds execution graph from dependencies
- Handles parameter substitution and environment variables

### 2. Scheduler
- Manages cron-based scheduling
- Queues workflows for execution
- Handles schedule conflicts and overlaps

### 3. Executor
- Spawns and manages system processes
- Handles different executor types (shell, docker, ssh, etc.)
- Manages process groups for proper cleanup

### 4. Process Manager
- Tracks running processes via Unix sockets
- Handles signals (SIGTERM, SIGINT, etc.)
- Ensures graceful shutdown
- Manages process output and logging

### 5. Queue Manager
- File-based queue implementation
- Priority-based execution
- Prevents duplicate executions
- Handles concurrency limits

### 6. State Manager
- Tracks workflow and step states
- Manages execution history
- Handles state transitions
- Provides status queries

## Storage Architecture

Dagu uses a file-based storage system organized as follows:

```
~/.local/share/dagu/
├── logs/
│   └── my-workflow/
│       └── 20240315_120000_abc123/
│           ├── step1.stdout.log
│           ├── step1.stderr.log
│           └── workflow.log
├── data/
│   └── my-workflow/
│       └── status.json
├── history/
│   └── my-workflow/
│       └── 20240315.json
└── suspend/
    └── my-workflow.suspend
```

### Why File-Based Storage?

1. **Zero Dependencies**: No database to install or maintain
2. **Portability**: Easy to backup, move, or version control
3. **Transparency**: Logs and state are human-readable
4. **Reliability**: Leverages filesystem guarantees
5. **Performance**: No network overhead

## Process Communication

Dagu uses Unix domain sockets for inter-process communication:

```
┌──────────────┐     Unix Socket      ┌──────────────┐
│   Scheduler  │◄────────────────────►│   Executor   │
└──────────────┘                      └──────────────┘
                                             │
                                             ▼
                                      ┌──────────────┐
                                      │ Child Process│
                                      └──────────────┘
```

Benefits:
- Fast local communication
- Secure by default (filesystem permissions)
- No port conflicts
- Automatic cleanup on process exit

## Execution Model

### 1. DAG Execution Flow

```
Parse DAG → Build Graph → Topological Sort → Execute Steps → Update State
```

### 2. Step Execution

Each step goes through these states:

```
Pending → Running → Success/Failed/Cancelled
```

### 3. Parallel Execution

Dagu uses goroutines for concurrent step execution:

```go
// Simplified execution logic
for _, step := range readySteps {
    go executeStep(step)
}
```

## Security Model

### Process Isolation
- Each workflow runs with the permissions of the Dagu process
- No privilege escalation
- Process groups ensure cleanup

### File Permissions
- DAG files: Read access required
- Log files: Write access to log directory
- State files: Write access to data directory

### Network Security
- Web UI binds to localhost by default
- TLS support for production deployments
- Token-based API authentication

## Performance Characteristics

### Scalability
- **Workflows**: Thousands of DAG files
- **Steps**: Hundreds per workflow
- **Parallel**: Limited by system resources
- **History**: Configurable retention

### Resource Usage
- **Memory**: ~50MB base + workflow overhead
- **CPU**: Minimal when idle
- **Disk**: Proportional to log retention
- **Network**: Local only (unless using SSH/HTTP executors)

### Limitations
- Single machine execution
- File system performance dependent
- Process spawn overhead for each step

## High Availability Considerations

While Dagu is designed for single-machine operation, you can achieve high availability through:

1. **Active-Passive Setup**
   ```
   Primary Server → Shared Storage ← Standby Server
   ```

2. **Shared Storage**
   - NFS for DAG files
   - Replicated logs
   - Synchronized state

3. **Monitoring**
   - Health check endpoints
   - Prometheus metrics
   - External monitoring

## Comparison with Other Architectures

### Dagu vs Airflow

| Aspect | Dagu | Airflow |
|--------|------|---------|
| Architecture | Single Process | Distributed |
| Storage | File System | Database |
| Communication | Unix Sockets | Message Broker |
| Dependencies | None | PostgreSQL, Redis/RabbitMQ |
| Complexity | Simple | Complex |

### Dagu vs Kubernetes Jobs

| Aspect | Dagu | K8s Jobs |
|--------|------|----------|
| Scope | Workflow Engine | Container Orchestrator |
| Resources | Single Machine | Cluster |
| Overhead | Minimal | Significant |
| Use Case | General Workflows | Container Workloads |

## Future Architecture Considerations

While maintaining simplicity, potential enhancements include:

1. **Plugin System**: Extensible executors and hooks
2. **Remote Execution**: Distributed mode (optional)
3. **State Backends**: Pluggable storage (keeping file as default)
4. **Event Streaming**: Real-time event bus

## Summary

Dagu's architecture prioritizes:
- **Simplicity**: Single binary, no dependencies
- **Reliability**: Process management, proper cleanup
- **Transparency**: File-based, debuggable
- **Performance**: Low overhead, fast startup
- **Flexibility**: Multiple executors, extensible

This design makes Dagu ideal for teams that need powerful workflow orchestration without operational complexity.