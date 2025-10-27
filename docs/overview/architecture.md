# Architecture

Understanding how Dagu works under the hood.

## Design Philosophy

Dagu follows a simple philosophy: **do one thing well with minimal dependencies**.

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
│  Scheduler  │     Agent        │  Execution Scheduler       │
├─────────────┼──────────────────┼────────────────────────────┤
│ DAG Loader  │    Executors     │    Persistence Layer       │
└─────────────┴──────────────────┴────────────────────────────┘
                              │
┌─────────────────────────────┴───────────────────────────────┐
│                      Storage Layer                          │
├─────────────┬──────────────────┬────────────────────────────┤
│  DAG Files  │    Log Files     │    State Files             │
└─────────────┴──────────────────┴────────────────────────────┘
```

## Core Components

### 1. DAG Loader
- Loads YAML workflow definitions and builds DAG structure
- Validates DAG syntax and dependencies

### 2. Scheduler
- Monitors and triggers DAGs based on cron expressions
- Consumes queued DAG runs and executes them
- Supports high availability through directory-based locking
- Automatic failover when primary scheduler fails
- See [Scheduling](/features/scheduling) for details

### 3. Agent
- Manages complete lifecycle of a single DAG run
- Handles Unix socket communication for status updates
- Writes logs and updates run status

### 4. Executors
- Shell: Runs shell commands in subprocesses
- Docker: Executes in containers
- SSH: Remote command execution
- HTTP: Makes API requests
- Mail: Sends email notifications
- JQ: JSON data processing

### 5. Persistence Layer
- DAG Store: Manages DAG definitions
- DAG-run Store: Tracks execution history and attempts
- Proc Store: Process heartbeat tracking
- Queue Store: Dual-priority queue system
- By default, all state is stored in files under `~/.config/dagu/` and `~/.local/share/dagu/`
- You can set a custom directory structure using the `DAGU_HOME` environment variable or configuration options

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
│   │                           └── subdags/           # Sub DAG runs (nested workflows)
│   │                               └── sub_xyz789/
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
├── suspend/           # Workflow suspend flags
│   └── my-workflow.suspend
└── scheduler/         # Scheduler coordination
    └── locks/         # Directory-based locks for HA
        └── .dagu_lock.<hostname@pid>.<timestamp>/
```

## Distributed Execution Architecture

Dagu supports distributed execution through a coordinator-worker model. Workers require access to shared storage for DAG files and execution state.

### Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     Dagu Instance                           │
├──────────────┬────────────────┬─────────────────────────────┤
│  Scheduler   │   Web UI       │      Coordinator Service   │
│              │                │         (gRPC Server)       │
└──────────────┴────────────────┴─────────────────────────────┘
                                              │
                                              │ gRPC (Long Polling)
                                              │
                ┌─────────────────────────────┴────────────────┐
                │                                              │
         ┌──────▼───────┐                            ┌────────▼──────┐
         │   Worker 1   │                            │   Worker N    │
         │              │                            │               │
         │ Labels:      │                            │ Labels:       │
         │ - gpu=true   │                            │ - region=eu   │
         │ - memory=64G │                            │ - cpu=high    │
         └──────────────┘                            └───────────────┘
```

### Core Components

#### 1. Coordinator Service
- gRPC Server: Listens on configurable port (default: 50055)
- Task Distribution: Routes tasks to appropriate workers based on labels
- Long Polling: Workers poll for tasks using efficient long-polling mechanism
- Health Monitoring: Tracks worker heartbeats (10s intervals) and health status
- Automatic Failover: Redistributes tasks from unhealthy workers
- Authentication: Supports signing keys and mutual TLS

#### 2. Worker Service
- Auto-Registration: Workers automatically register in registry system
- Task Polling: Multiple concurrent pollers per worker
- Label-Based Routing: Workers advertise capabilities via labels
- Task Execution: Runs DAGs using the same execution engine
- Heartbeat: Regular health updates every 10 seconds
- Graceful Shutdown: Completes running tasks before terminating

#### 3. Task Routing

Tasks are routed to workers based on `workerSelector` in DAG definitions:

```yaml
workerSelector:
  gpu: "true"
  memory: "64G"
steps:
  - python train.py
```

### Communication Protocol

1. Worker Registration
   - Workers register in file-based registry system
   - Connect to coordinator via gRPC
   - Send regular heartbeats with status updates
   - Advertise labels for capability matching

2. Task Assignment
   - Scheduler creates tasks with worker requirements
   - Coordinator matches tasks to eligible workers
   - Workers poll and receive matching tasks

3. Status Updates
   - Workers track task execution status
   - Real-time updates visible in Web UI
   - Hierarchical DAG tracking (root/parent/child)

### Health Monitoring

Worker health is determined by heartbeat recency:
- `Healthy`: Last heartbeat < 5 seconds ago (green)
- `Warning`: Last heartbeat 5-15 seconds ago (yellow)
- `Unhealthy`: Last heartbeat > 15 seconds ago (red)
- `Offline`: No heartbeat for > 30 seconds (removed from registry)

### Service Registry

The file-based service registry system enables:
- Automatic Registration: Services register on startup
- Dynamic Discovery: Coordinators find workers automatically
- Heartbeat Tracking: Regular updates maintain service health
- Shared Storage: Registry files stored in configurable directory
- Graceful Cleanup: Stale entries removed automatically

### Security Features

1. TLS Support
   - Server certificates for encrypted communication
   - Client certificates for mutual TLS authentication
   - CA certificate validation

2. Authentication
   - Signing key for request validation

3. Network Security
   - Configurable bind addresses
   - Firewall-friendly single port

### Deployment Patterns

#### Single Coordinator, Multiple Workers
```bash
# Start coordinator on main server
dagu coordinator --coordinator.host=0.0.0.0

# Start workers on compute nodes
dagu worker --worker.labels gpu=true --worker.coordinator-host=coordinator.internal
dagu worker --worker.labels region=us-east-1 --worker.coordinator-host=coordinator.internal
```

### Requirements

- Workers need access to the same DAG files and data directories as the main Dagu instance
  - Exception: When DAG definitions are embedded in the task (e.g., local DAGs defined within a parent DAG file)
- Shared storage can be provided via NFS, cloud storage (EFS, GCS, Azure Files), or distributed filesystems
- Workers execute DAGs using the same file-based storage system described above
