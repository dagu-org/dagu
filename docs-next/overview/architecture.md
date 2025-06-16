# Architecture

Understanding how Dagu works under the hood.

## Design Philosophy

Dagu follows a simple philosophy: **do one thing well with minimal dependencies**. Unlike traditional workflow orchestrators that require complex distributed systems, Dagu runs as a single process that manages everything locally.

## System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         User Interfaces                     │
├─────────────┬──────────────────┬────────────────────────────┤
│     CLI     │     Web UI       │         REST API           │
└─────────────┴──────────────────┴────────────────────────────┘
                              │
┌─────────────────────────────┴───────────────────────────────┐
│                        Core Engine                          │
├─────────────┬──────────────────┬────────────────────────────┤
│  Scheduler  │  Executor        │    Queue Manager           │
├─────────────┼──────────────────┼────────────────────────────┤
│  DAG Parser │  Process Manager │    State Manager           │
└─────────────┴──────────────────┴────────────────────────────┘
                              │
┌─────────────────────────────┴───────────────────────────────┐
│                      Storage Layer                          │
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
- Tracks running processes via Unix sockets and heartbeat files
- Stores process heartbeats in `data/proc/` directory
- Handles signals (SIGTERM, SIGINT, etc.) for graceful shutdown
- Manages process groups for proper cleanup of orphaned processes
- Coordinates between scheduler and executor processes

### 5. Queue Manager
- File-based queue implementation using timestamped JSON files
- Priority-based execution (high/low priority queues)
- Atomic file operations prevent race conditions
- Per-DAG queue directories for organization
- Chronological processing based on file timestamps

### 6. State Manager
- Tracks workflow and step states using JSON Lines format
- Manages hierarchical execution history (year/month/day structure)
- Handles state transitions and attempt-based retries
- Supports child DAG state tracking for nested workflows
- Provides efficient status queries through organized file structure

## Storage Architecture

Dagu follows the XDG Base Directory specification for file organization:

```
~/.config/dagu/
├── dags/              # Workflow definitions
│   ├── my-workflow.yaml
│   └── another-workflow.yaml
├── config.yaml        # Main configuration
└── base.yaml          # Shared base configuration

~/.local/share/dagu/
├── data/              # Main data directory
│   ├── dag-runs/      # Workflow execution history & state (hierarchical)
│   │   └── my-workflow/
│   │       └── dag-runs/
│   │           └── 2024/           # Year
│   │               └── 03/         # Month
│   │                   └── 15/     # Day
│   │                       └── dag-run_20240315_120000Z_abc123/
│   │                           ├── attempt_20240315_120001_123Z_def456/
│   │                           │   ├── status.jsonl     # Status updates (JSON Lines)
│   │                           │   ├── step1.stdout.log # Step stdout
│   │                           │   ├── step1.stderr.log # Step stderr
│   │                           │   └── step2.stdout.log
│   │                           └── children/           # Child DAG runs (nested workflows)
│   │                               └── child_xyz789/
│   │                                   └── attempt_20240315_120002_456Z_ghi012/
│   │                                       └── status.jsonl
│   ├── queue/         # File-based execution queue
│   │   └── my-workflow/
│   │       ├── item_high_20240315_120000_123Z_priority1.json  # High priority
│   │       └── item_low_20240315_120030_456Z_batch1.json      # Low priority
│   └── proc/          # Process tracking files (heartbeats)
│       └── my-workflow/
│           └── abc123_1710504000  # Process heartbeat (binary)
├── logs/              # Human-readable execution logs
│   ├── admin/         # Admin/scheduler logs
│   │   ├── scheduler.log
│   │   └── server.log
│   └── dags/          # DAG-specific logs (for web UI)
│       └── my-workflow/
│           └── 20240315_120000_abc123/
│               ├── step1.stdout.log
│               ├── step1.stderr.log
│               └── status.yaml
└── suspend/           # Workflow suspend flags
    └── my-workflow.suspend
```

### Storage Components Explained

#### 1. DAG Runs Storage (`data/dag-runs/`)
- **Hierarchical organization**: Year/Month/Day structure for efficient access
- **Attempt-based**: Each execution attempt has its own directory
- **Status tracking**: JSON Lines format for real-time status updates
- **Child DAG support**: Nested workflows store results in `children/` subdirectories
- **DAG name sanitization**: Unsafe characters replaced, hash appended if modified

#### 2. Queue Storage (`data/queue/`)
- **File-based queuing**: Each queued DAG run becomes a JSON file
- **Priority support**: `item_high_*` and `item_low_*` file prefixes
- **Timestamp ordering**: Files processed in chronological order
- **Atomic operations**: File creation ensures queue consistency

#### 3. Process Tracking (`data/proc/`)
- **Heartbeat files**: Binary files track running processes
- **Process groups**: Enables proper cleanup of orphaned processes
- **Unix socket communication**: Coordinates between scheduler and executors
- **Automatic cleanup**: Files removed when processes terminate

#### 4. Logs vs Data Distinction
- **`logs/`**: Human-readable logs for debugging and UI display
- **`data/`**: Machine-readable state for system operation
- **Duplication**: Some output stored in both locations for different purposes

### Legacy Mode
If `~/.dagu` directory exists or `DAGU_HOME` environment variable is set, all files are stored under that single directory instead of following XDG specification:

```
~/.dagu/  (or $DAGU_HOME)
├── dags/
├── data/
│   ├── dag-runs/
│   ├── queue/
│   └── proc/
├── logs/
└── suspend/
```

### Why File-Based Storage?

1. **Zero Dependencies**: No database to install or maintain
2. **Portability**: Easy to backup, move, or version control
3. **Transparency**: Logs and state are human-readable (where appropriate)
4. **Reliability**: Leverages filesystem guarantees
5. **Performance**: No network overhead for local operations
6. **Scalability**: Hierarchical structure handles thousands of executions
7. **Distributed Workflow Capability**: Can create distributed workflows by mounting shared storage across multiple machines, allowing DAG processes to run on separate nodes while sharing the same file-based state

### File Formats and Naming Conventions

#### DAG Run Timestamps
- **DAG runs**: `dag-run_YYYYMMDD_HHMMSSZ_{run-id}/`
- **Attempts**: `attempt_YYYYMMDD_HHMMSS_sssZ_{attempt-id}/` (includes milliseconds)
- **Timezone**: All timestamps in UTC (Z suffix)

#### Queue Files
- **High priority**: `item_high_YYYYMMDD_HHMMSS_{sss}Z_{run-id}.json`
- **Low priority**: `item_low_YYYYMMDD_HHMMSS_{sss}Z_{run-id}.json`
- **Content**: JSON with DAG run metadata and parameters

#### Status Files
- **Format**: JSON Lines (`.jsonl`) for append-only status updates
- **Real-time**: Status changes written immediately
- **History**: Complete execution timeline preserved

#### DAG Name Sanitization
- **Safe characters**: Alphanumeric, hyphens, underscores only
- **Unsafe replacement**: Other characters become underscores
- **Hash suffix**: 4-character hash added if name modified for uniqueness

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

## Distributed and High Availability Patterns

### Distributed Workflow Execution

Dagu's file-based architecture enables distributed execution patterns:

1. **Shared Storage Distribution**
   ```
   Node 1 (Scheduler) → Shared Storage ← Node 2 (Executor)
                              ↕
                         Node 3 (Executor)
   ```

2. **Implementation Approaches**
   - **NFS/CIFS**: Mount shared filesystem across nodes
   - **Distributed filesystems**: GlusterFS, CephFS for high performance
   - **Cloud storage**: EFS (AWS), FileStore (GCP), Files (Azure)
   - **Container orchestration**: Shared volumes in Kubernetes

3. **Coordination Benefits**
   - Queue files enable work distribution
   - Process heartbeats prevent conflicts
   - Status files provide real-time coordination
   - No complex message brokers required

### High Availability Setup

1. **Active-Passive Configuration**
   ```
   Primary Dagu → Shared Storage ← Standby Dagu
   ```
   - Shared storage ensures state consistency
   - Process heartbeats prevent split-brain scenarios
   - Automatic failover via external monitoring

2. **Load Distribution**
   ```
   DAG Scheduler → Shared Queue → Multiple Executor Nodes
   ```
   - Scheduler writes to queue files
   - Executors process from shared queue
   - Natural load balancing via file timestamps

3. **Monitoring and Health Checks**
   - Health check endpoints
   - Prometheus metrics
   - Process heartbeat monitoring
   - File system health validation

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
