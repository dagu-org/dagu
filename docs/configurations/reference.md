# Configuration Reference

All Dagu configuration options.

## Configuration File

Default: `~/.config/dagu/config.yaml`

```yaml
# Server
host: "127.0.0.1"
port: 8080
basePath: ""              # For reverse proxy (e.g., "/dagu")
apiBasePath: "/api/v2"    # API endpoint base path
tz: "America/New_York"
debug: false
logFormat: "text"         # "text" or "json"
headless: false
skipExamples: false       # Skip creating example DAGs

# Directories (must be under "paths" key)
paths:
  dagsDir: "~/.config/dagu/dags"
  logDir: "~/.local/share/dagu/logs"
  dataDir: "~/.local/share/dagu/data"
  suspendFlagsDir: "~/.local/share/dagu/suspend"
  adminLogsDir: "~/.local/share/dagu/logs/admin"
  baseConfig: "~/.config/dagu/base.yaml"
  dagRunsDir: ""            # Auto: {dataDir}/dag-runs
  queueDir: ""              # Auto: {dataDir}/queue
  procDir: ""               # Auto: {dataDir}/proc
  serviceRegistryDir: ""    # Auto: {dataDir}/service-registry
  executable: ""            # Auto: current executable path

# Permissions
permissions:
  writeDAGs: true         # Create/edit/delete DAGs
  runDAGs: true           # Run/stop/retry DAGs

# Authentication (enabled when credentials are set)
auth:
  basic:
    username: "admin"
    password: "secret"
  token:
    value: "your-token"

# TLS/HTTPS
tls:
  certFile: "/path/to/cert.pem"
  keyFile: "/path/to/key.pem"

# UI
ui:
  navbarColor: "#1976d2"     # Hex or CSS color
  navbarTitle: "Dagu"
  logEncodingCharset: "utf-8"
  maxDashboardPageLimit: 100
  dags:
    sortField: "name"        # Default sort field (name/status/lastRun/schedule/suspended)
    sortOrder: "asc"         # Default sort order (asc/desc)

latestStatusToday: true      # Show only today's status

# Queues
queues:
  enabled: true          # Default: true
  config:
    - name: "critical"
      maxConcurrency: 5   # Maximum concurrent DAG runs
    - name: "batch"
      maxConcurrency: 1
    - name: "default"
      maxConcurrency: 2

# Remote Nodes
remoteNodes:
  - name: "staging"
    apiBaseURL: "https://staging.example.com/api/v2"
    isBasicAuth: true
    basicAuthUsername: "admin"
    basicAuthPassword: "password"
  - name: "production"
    apiBaseURL: "https://prod.example.com/api/v2"
    isAuthToken: true
    authToken: "prod-token"
    skipTLSVerify: false

# Coordinator (for distributed execution)
coordinator:
  host: "127.0.0.1"       # Bind address
  port: 50055             # gRPC port

# Worker (for distributed execution)
worker:
  id: ""                  # Worker ID (default: hostname@PID)
  maxActiveRuns: 100      # Max parallel task executions
  labels:                 # Worker capabilities
    gpu: "false"
    memory: "16G"
    region: "us-east-1"

# Peer TLS configuration (for distributed execution)
peer:
  insecure: true          # Use h2c instead of TLS (default: true)
  certFile: ""            # TLS certificate for peer connections
  keyFile: ""             # TLS key for peer connections
  clientCaFile: ""        # CA certificate for peer verification (mTLS)
  skipTlsVerify: false    # Skip TLS certificate verification

# Scheduler
scheduler:
  port: 8090              # Health check port (0 to disable)
  lockStaleThreshold: "30s"  # Time after which a scheduler lock is considered stale
  lockRetryInterval: "5s"   # Interval between lock acquisition attempts
  zombieDetectionInterval: "45s"  # Interval for detecting zombie DAG runs (0 to disable)
```

## Environment Variables

All options support `DAGU_` prefix.

**Note:** For security, Dagu filters which system environment variables are passed to step processes and sub DAGs. System variables are available for expansion (`${VAR}`) in DAG configuration, but only whitelisted variables (`PATH`, `HOME`, `LANG`, `TZ`, `SHELL`) and variables with allowed prefixes (`DAGU_*`, `LC_*`, `DAG_*`) are passed to the step execution environment. See [Operations - Security](/configurations/operations#security) for details.

### Server
- `DAGU_HOST` - Server host (default: `127.0.0.1`)
- `DAGU_PORT` - Server port (default: `8080`)
- `DAGU_BASE_PATH` - Base path for reverse proxy
- `DAGU_API_BASE_URL` - **DEPRECATED** - Use `apiBasePath` config instead
- `DAGU_TZ` - Server timezone
- `DAGU_DEBUG` - Enable debug mode
- `DAGU_LOG_FORMAT` - Log format (`text`/`json`)
- `DAGU_HEADLESS` - Run without UI
- `DAGU_LATEST_STATUS_TODAY` - Show only today's status
- `DAGU_SKIP_EXAMPLES` - Skip automatic creation of example DAGs (default: `false`)

### Directories
- `DAGU_HOME` - Set all directories to this path (can be overridden by `--dagu-home` flag)
- `DAGU_DAGS_DIR` - DAG definitions
- `DAGU_DAGS` - Alternative to `DAGU_DAGS_DIR`
- `DAGU_WORK_DIR` - Default working directory
- `DAGU_LOG_DIR` - Log files
- `DAGU_DATA_DIR` - Application data
- `DAGU_SUSPEND_FLAGS_DIR` - Suspend flags
- `DAGU_ADMIN_LOG_DIR` - Admin logs
- `DAGU_BASE_CONFIG` - Base configuration
- `DAGU_DAG_RUNS_DIR` - DAG run data directory
- `DAGU_QUEUE_DIR` - Queue data directory
- `DAGU_PROC_DIR` - Process data directory
- `DAGU_SERVICE_REGISTRY_DIR` - Service registry data directory
- `DAGU_EXECUTABLE` - Path to Dagu executable

**Note:** The `--dagu-home` CLI flag takes precedence over the `DAGU_HOME` environment variable.

### Authentication
- `DAGU_AUTH_BASIC_USERNAME` - Basic auth username
- `DAGU_AUTH_BASIC_PASSWORD` - Basic auth password
- `DAGU_AUTH_TOKEN` - API token for token authentication

### TLS/HTTPS
- `DAGU_CERT_FILE` - SSL certificate
- `DAGU_KEY_FILE` - SSL key

### UI
- `DAGU_UI_NAVBAR_COLOR` - Nav bar color
- `DAGU_UI_NAVBAR_TITLE` - Nav bar title
- `DAGU_UI_LOG_ENCODING_CHARSET` - Log encoding
- `DAGU_UI_MAX_DASHBOARD_PAGE_LIMIT` - Dashboard limit
- `DAGU_UI_DAGS_SORT_FIELD` - Default DAGs page sort field
- `DAGU_UI_DAGS_SORT_ORDER` - Default DAGs page sort order

### Queue
- `DAGU_QUEUE_ENABLED` - Enable queue system (default: true)

### Coordinator
- `DAGU_COORDINATOR_HOST` - Coordinator bind address (default: `127.0.0.1`)
- `DAGU_COORDINATOR_ADVERTISE` - Address to advertise in service registry (default: auto-detected hostname)
- `DAGU_COORDINATOR_PORT` - Coordinator gRPC port (default: `50055`)

### Worker
- `DAGU_WORKER_ID` - Worker instance ID (default: `hostname@PID`)
- `DAGU_WORKER_MAX_ACTIVE_RUNS` - Max concurrent task executions (default: `100`)
- `DAGU_WORKER_LABELS` - Worker labels (format: `key1=value1,key2=value2`)

### Peer (for distributed TLS)
- `DAGU_PEER_INSECURE` - Use insecure connection (default: `true`)
- `DAGU_PEER_CERT_FILE` - TLS certificate for peer connections
- `DAGU_PEER_KEY_FILE` - TLS key for peer connections
- `DAGU_PEER_CLIENT_CA_FILE` - CA certificate for peer verification (mTLS)
- `DAGU_PEER_SKIP_TLS_VERIFY` - Skip TLS certificate verification

### Scheduler
- `DAGU_SCHEDULER_PORT` - Health check server port (default: `8090`)
- `DAGU_SCHEDULER_LOCK_STALE_THRESHOLD` - Time after which a scheduler lock is considered stale (default: `30s`)
- `DAGU_SCHEDULER_LOCK_RETRY_INTERVAL` - Interval between lock acquisition attempts (default: `5s`)
- `DAGU_SCHEDULER_ZOMBIE_DETECTION_INTERVAL` - Interval for detecting zombie DAG runs (default: `45s`, `0` to disable)

### Legacy Environment Variables (Deprecated)
These variables are maintained for backward compatibility but should not be used in new deployments:
- `DAGU__ADMIN_NAVBAR_COLOR` - Use `DAGU_UI_NAVBAR_COLOR`
- `DAGU__ADMIN_NAVBAR_TITLE` - Use `DAGU_UI_NAVBAR_TITLE`
- `DAGU__ADMIN_PORT` - Use `DAGU_PORT`
- `DAGU__ADMIN_HOST` - Use `DAGU_HOST`
- `DAGU__DATA` - Use `DAGU_DATA_DIR`
- `DAGU__SUSPEND_FLAGS_DIR` - Use `DAGU_SUSPEND_FLAGS_DIR`
- `DAGU__ADMIN_LOGS_DIR` - Use `DAGU_ADMIN_LOG_DIR`

## Base Configuration

Shared defaults for all DAGs: `~/.config/dagu/base.yaml`

```yaml
# Environment
env:
  - ENVIRONMENT: production
  - LOG_LEVEL: info

# Defaults
params:
  - DEFAULT_TIMEOUT: 3600
  - RETRY_LIMIT: 3

queue: "default"
maxActiveRuns: 1
histRetentionDays: 30

# Email
mailOn:
  failure: true
  success: false

smtp:
  host: "smtp.gmail.com"
  port: "587"
  username: "notifications@company.com"
  password: "${SMTP_PASSWORD}"

errorMail:
  from: "dagu@company.com"
  to: "ops-team@company.com"
  prefix: "[ERROR]"
  attachLogs: true
```

## Command-Line Flags

### Global Flags (All Commands)
- `--config, -c` - Config file (default: `~/.config/dagu/config.yaml`)
- `--dagu-home` - Override DAGU_HOME for this command invocation
- `--quiet, -q` - Suppress output
- `--cpu-profile` - Enable CPU profiling

The `--dagu-home` flag sets a custom application home directory for the current command, overriding the `DAGU_HOME` environment variable. When set, all paths use a unified structure under the specified directory.

**Example:**
```bash
# Use a custom home directory
dagu --dagu-home=/tmp/dagu-test start my-workflow.yaml

# Run server with isolated data
dagu --dagu-home=/opt/dagu-prod start-all
```

### Server/Start-All
- `--host, -s` - Server host
- `--port, -p` - Server port
- `--dags, -d` - DAGs directory

### Scheduler
- `--dags, -d` - DAGs directory

## Configuration Precedence

1. Command-line flags (highest)
2. Environment variables
3. Configuration file
4. Base configuration
5. Default values (lowest)

```bash
# Port 9000 wins (CLI flag beats env var)
DAGU_PORT=8080 dagu start-all --port 9000

# --dagu-home flag overrides DAGU_HOME environment variable
DAGU_HOME=/opt/dagu dagu --dagu-home=/tmp/dagu-test start my-workflow.yaml
```

## Special Environment Variables

Dagu sets metadata like `DAG_RUN_ID`, `DAG_RUN_LOG_FILE`, and the active `DAG_RUN_STEP_NAME` while each workflow runs. Consult [Special Environment Variables](/reference/special-environment-variables) for the full list and examples of how to use them in automations.

## Directory Structure

### Default (XDG)
```
~/.config/dagu/
├── dags/              # DAG definitions
├── config.yaml        # Main configuration
└── base.yaml          # Shared DAG defaults

~/.local/share/dagu/
├── logs/              # All log files
│   ├── admin/         # Admin/server logs
│   └── dags/          # DAG execution logs
├── data/              # Application data
│   ├── dag-runs/      # DAG run history
│   ├── queue/         # Queue data
│   ├── proc/          # Process data
│   └── service-registry/  # Service registry data
└── suspend/           # DAG suspend flags
```

### With DAGU_HOME
```
$DAGU_HOME/
├── dags/              # DAG definitions
├── logs/              # All log files
│   └── admin/         # Admin/server logs
├── data/              # Application data
│   ├── dag-runs/      # DAG run history
│   ├── queue/         # Queue data
│   ├── proc/          # Process data
│   └── service-registry/  # Service registry data
├── suspend/           # DAG suspend flags
├── config.yaml        # Main configuration
└── base.yaml          # Shared DAG defaults
```

## Configuration Examples

### Minimal
```yaml
port: 8080
```

### Production
```yaml
host: 0.0.0.0
port: 443

auth:
  basic:
    username: admin
    password: ${ADMIN_PASSWORD}

tls:
  certFile: /etc/ssl/certs/dagu.crt
  keyFile: /etc/ssl/private/dagu.key

permissions:
  writeDAGs: false
  runDAGs: true

ui:
  navbarColor: "#ff0000"
  navbarTitle: "Production"
```

### Multi-Environment
```yaml
remoteNodes:
  - name: staging
    apiBaseURL: https://staging.example.com/api/v2
    isAuthToken: true
    authToken: ${STAGING_TOKEN}
    
  - name: production
    apiBaseURL: https://prod.example.com/api/v2
    isAuthToken: true
    authToken: ${PROD_TOKEN}
```

### Distributed Execution
```yaml
# Main instance with coordinator
host: 0.0.0.0
port: 8080

coordinator:
  host: 0.0.0.0
  port: 50055

# Worker configuration
worker:
  id: gpu-worker-01
  maxActiveRuns: 10
  labels:
    gpu: "true"
    cuda: "11.8"
    memory: "64G"
    region: "us-east-1"

# TLS configuration for peer connections
peer:
  insecure: false  # Enable TLS
  certFile: /etc/dagu/tls/cert.pem
  keyFile: /etc/dagu/tls/key.pem
  clientCaFile: /etc/dagu/tls/ca.pem
```

## Default Values

### Key Defaults
- `apiBasePath`: `/api/v2`
- `queues.enabled`: `true`
- `permissions.writeDAGs`: `true`
- `permissions.runDAGs`: `true`
- `ui.maxDashboardPageLimit`: `100`
- `ui.logEncodingCharset`: `utf-8`
- `ui.dags.sortField`: `name`
- `ui.dags.sortOrder`: `asc`
- `logFormat`: `text`
- `host`: `127.0.0.1`
- `port`: `8080`

### Auto-generated Paths
When not specified, these paths are automatically set based on `paths.dataDir`:
- `paths.dagRunsDir`: `{paths.dataDir}/dag-runs` - Stores DAG run history
- `paths.queueDir`: `{paths.dataDir}/queue` - Stores queue data
- `paths.procDir`: `{paths.dataDir}/proc` - Stores process data
- `paths.executable`: Current executable path - Auto-detected from running process

## See Also

- [Server Configuration](/configurations/server)
- [Production Setup](/configurations/operations)
- [Deployment Guides](/configurations/deployment)
