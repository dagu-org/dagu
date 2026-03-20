---
name: Server, Worker & Scheduler Configuration
description: Configuration reference for Dagu server, worker, scheduler, coordinator, and queue settings
version: 1.0.0
author: Dagu
tags:
  - server
  - worker
  - scheduler
  - configuration
---
# Server, Worker & Scheduler Configuration

## Config File Location

Dagu looks for its config file in this order:

1. Path specified by `--config` flag
2. `$DAGU_HOME/config.yaml` (unified layout when `DAGU_HOME` is set)
3. `~/.config/dagu/config.yaml` (XDG default)

Set `DAGU_HOME=/var/lib/dagu` to keep all Dagu files under a single directory.

## Environment Variables

All config keys map to `DAGU_<UPPER_SNAKE_CASE>` env vars:

```bash
DAGU_HOST=0.0.0.0
DAGU_PORT=8080
DAGU_AUTH_MODE=builtin
DAGU_AUTH_TOKEN_SECRET=change-me
DAGU_DAGS_DIR=/var/lib/dagu/dags
```

Nested keys use underscores: `auth.builtin.token.secret` → `DAGU_AUTH_TOKEN_SECRET`

## Server Configuration

```yaml
host: 127.0.0.1        # Bind address (default: 127.0.0.1)
port: 8080              # Web UI port (default: 8080)
base_path: /dagu        # URL prefix for reverse proxy subpath hosting (optional)
headless: false         # Disable web UI, API only
debug: false
log_format: text        # "text" or "json"
tz: UTC                 # Timezone (e.g., America/New_York)
```

### TLS

```yaml
tls:
  cert_file: /etc/dagu/tls/server.crt
  key_file:  /etc/dagu/tls/server.key
```

### Authentication

**No auth** (development only):
```yaml
auth:
  mode: none
```

**Builtin** (RBAC with JWT — recommended for production):
```yaml
auth:
  mode: builtin
  builtin:
    admin:
      username: admin
      password: ""           # Leave blank; auto-generated on first run
    token:
      secret: change-me-to-a-secure-random-string
      ttl: 24h
```

Env-var equivalents:
```bash
DAGU_AUTH_MODE=builtin
DAGU_AUTH_TOKEN_SECRET=change-me
DAGU_AUTH_ADMIN_USERNAME=admin
DAGU_AUTH_ADMIN_PASSWORD=mypassword   # optional, auto-generated if omitted
DAGU_AUTH_TOKEN_TTL=24h
```

**OIDC** (SSO via Google, Okta, etc.):
```yaml
auth:
  mode: oidc
  oidc:
    client_id: my-app-client-id
    client_secret: my-app-client-secret
    issuer: https://accounts.google.com
    client_url: https://dagu.example.com
    scopes: [openid, profile, email]
```

### UI Settings

```yaml
ui:
  navbar_color: "#2563eb"          # Hex color for the top navbar
  navbar_title: "My Dagu"          # Custom title in the navbar
  max_dashboard_page_limit: 100    # Max rows on the dashboard (1-1000)
  dags:
    sort_field: name               # Default DAG sort field
    sort_order: asc                # "asc" or "desc"
```

### Permissions

```yaml
permissions:
  write_dags: true    # Allow creating/editing DAGs via web UI (default: true)
  run_dags: true      # Allow triggering DAG runs via web UI (default: true)
```

### Additional Server Features

```yaml
terminal:
  enabled: false       # Enable web-based terminal (requires auth.mode=builtin)

audit:
  enabled: true        # Record audit log of user actions (default: true)
  retention_days: 7    # Days to keep audit entries (0 = keep forever)

metrics: private       # Expose /metrics endpoint: "private" (auth-gated) or "public"

cache: normal          # Cache preset: "low", "normal", "high"
```

## Scheduler Configuration

The scheduler triggers DAG runs on cron schedules and processes the run queue.

```yaml
scheduler:
  port: 8090                        # Health check HTTP port (default: 8090; 0 to disable)
  lock_stale_threshold: 30s         # HA: takeover lock after this idle period
  lock_retry_interval: 5s           # HA: standby retry interval
  zombie_detection_interval: 45s    # Detect zombie runs (0 to disable)
```

Start the scheduler:
```bash
dagu scheduler --dags=/path/to/dags
```

## Coordinator Configuration

The coordinator is a gRPC server that distributes tasks to workers.

```yaml
coordinator:
  enabled: true          # Default: true; set false to disable in start-all
  host: 127.0.0.1        # gRPC bind address
  port: 50055            # gRPC port (default: 50055)
  advertise: ""          # Address workers use to connect (auto-detected if empty)
```

Start standalone:
```bash
dagu coordinator --coordinator.host=0.0.0.0 --coordinator.port=50055
```

## Worker Configuration

Workers poll the coordinator for tasks and execute them.

```yaml
worker:
  id: ""                 # Worker ID (default: hostname@PID)
  max_active_runs: 100   # Concurrent runs per worker (default: 100)

  # Capability labels for targeted routing (DAG-level worker_selector matches these)
  labels:
    region: us-east-1
    gpu: "true"
    tier: production

  # Static coordinator discovery (shared-nothing mode only)
  coordinators:
    - coordinator-1:50055
    - coordinator-2:50055

  # Postgres pool (shared-nothing mode only)
  postgres_pool:
    max_open_conns: 25
    max_idle_conns: 5
    conn_max_lifetime: 300   # seconds
    conn_max_idle_time: 60   # seconds
```

Start a worker:
```bash
dagu worker
dagu worker --worker.max-active-runs=50
dagu worker --worker.labels=gpu=true,region=us-east-1
```

## Queue Configuration

Queues limit concurrent DAG runs across the cluster.

```yaml
queues:
  enabled: true
  config:
    - name: etl
      max_concurrency: 5
    - name: reports
      max_concurrency: 2
```

Reference a queue from a DAG definition:
```yaml
queue: etl
```

## Paths Configuration

**XDG layout** (default when `DAGU_HOME` is not set):
```text
~/.config/dagu/         # config.yaml, base.yaml, dags/
~/.local/share/dagu/    # data/, logs/
```

**Unified layout** (when `DAGU_HOME` is set):
```text
$DAGU_HOME/config.yaml
$DAGU_HOME/dags/
$DAGU_HOME/data/
$DAGU_HOME/logs/
```

Override individual paths:
```yaml
paths:
  dags_dir: /var/lib/dagu/dags
  data_dir: /var/lib/dagu/data
  log_dir:  /var/log/dagu
  base_config: /etc/dagu/base.yaml   # Shared defaults applied to all DAGs
```

## Execution Mode

```yaml
default_execution_mode: local    # "local" (default) or "distributed"
```

- `local`: DAG runs execute on the same host as the scheduler
- `distributed`: DAG runs are dispatched to workers via the coordinator

## Peer TLS (Distributed Mode)

Secures gRPC traffic between coordinator and workers.

```yaml
peer:
  insecure: true           # Default: true (h2c/plaintext); set false for TLS
  cert_file: /etc/dagu/peer/client.crt
  key_file:  /etc/dagu/peer/client.key
  client_ca_file: /etc/dagu/peer/ca.crt   # For mutual TLS (mTLS)
  skip_tls_verify: false
```

## CLI Commands

| Command | Description | Key Flags |
|---------|-------------|-----------|
| `dagu server` | Web UI + API server | `--host`, `--port`, `--dags` |
| `dagu scheduler` | Cron scheduler + queue consumer | `--dags` |
| `dagu worker` | Task worker (polls coordinator) | `--worker.max-active-runs`, `--worker.labels`, `--worker.coordinators` |
| `dagu coordinator` | gRPC coordinator server | `--coordinator.host`, `--coordinator.port`, `--coordinator.advertise` |
| `dagu start-all` | Server + scheduler + coordinator in one process | `--host`, `--port`, `--coordinator.host`, `--coordinator.port` |

All commands accept `--config` to specify the config file path.

## Common Deployment Patterns

### Single-Node (Development)

```bash
dagu start-all
```

### Production: Auth + TLS

```yaml
host: 0.0.0.0
port: 8080

tls:
  cert_file: /etc/dagu/tls/server.crt
  key_file:  /etc/dagu/tls/server.key

auth:
  mode: builtin
  builtin:
    admin:
      username: admin
    token:
      secret: ${AUTH_TOKEN_SECRET}
      ttl: 8h

audit:
  enabled: true
  retention_days: 30
```

### Distributed: Coordinator + Workers

On the control plane host:
```yaml
coordinator:
  host: 0.0.0.0
  port: 50055
  advertise: dagu-coordinator.internal

default_execution_mode: distributed
```

```bash
dagu start-all --coordinator.host=0.0.0.0 --coordinator.advertise=dagu-coordinator.internal
```

On each worker host:
```yaml
worker:
  max_active_runs: 50
  labels:
    region: us-east-1
```

```bash
dagu worker
```
