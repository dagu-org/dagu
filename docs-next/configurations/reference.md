# Configuration Reference

Complete reference for all Dagu configuration options.

## Configuration File

Default location: `~/.config/dagu/config.yaml`

```yaml
# Complete configuration example with all options
# Server Configuration
host: "127.0.0.1"              # Server binding host
port: 8080                     # Server binding port  
basePath: ""                   # Base path for reverse proxy (e.g., "/dagu")
tz: "America/New_York"         # Server timezone
debug: false                   # Enable debug mode
logFormat: "text"              # Log format: "text" or "json"
headless: false                # Run without Web UI

# Directory Configuration  
dagsDir: "~/.config/dagu/dags"                    # DAG definitions directory
workDir: ""                                       # Default working directory for DAGs
logDir: "~/.local/share/dagu/logs"               # Log files directory
dataDir: "~/.local/share/dagu/data"              # Application data directory
suspendFlagsDir: "~/.local/share/dagu/suspend"   # DAG suspend flags directory
adminLogsDir: "~/.local/share/dagu/logs/admin"   # Admin logs directory
baseConfig: "~/.config/dagu/base.yaml"            # Base configuration file path

# Permissions
permissions:
  writeDAGs: true              # Allow creating/editing/deleting DAGs via UI/API
  runDAGs: true                # Allow running/stopping/retrying DAGs via UI/API

# Authentication
auth:
  basic:
    enabled: true              # Enable basic authentication
    username: "admin"          # Basic auth username
    password: "secret"         # Basic auth password
  token:
    enabled: true              # Enable API token authentication
    value: "your-token"        # API token value

# TLS/HTTPS Configuration
tls:
  certFile: "/path/to/cert.pem"    # SSL certificate file path
  keyFile: "/path/to/key.pem"      # SSL private key file path

# UI Configuration
ui:
  navbarColor: "#1976d2"           # Navigation bar color (hex or CSS color name)
  navbarTitle: "Dagu"              # Navigation bar title text
  logEncodingCharset: "utf-8"      # Character encoding for log files
  maxDashboardPageLimit: 100       # Maximum items to display on dashboard page

# Latest Status Configuration
latestStatusToday: true            # Show only today's latest status on dashboard

# Queue System Configuration
queues:
  enabled: true                    # Enable/disable the queue system
  config:                          # Named queue configurations
    - name: "critical"             # Queue name
      maxConcurrency: 5            # Maximum concurrent DAGs in this queue
      
    - name: "batch"
      maxConcurrency: 1
      
    - name: "default"
      maxConcurrency: 2

# Remote Nodes Configuration
remoteNodes:
  - name: "staging"                                # Display name for the remote node
    apiBaseURL: "https://staging.example.com/api/v1"  # API endpoint (must end with /api/v1)
    isBasicAuth: true                              # Use basic authentication
    basicAuthUsername: "admin"                     # Basic auth username
    basicAuthPassword: "password"                  # Basic auth password
    isAuthToken: false                             # Use token authentication
    authToken: ""                                  # API token
    skipTLSVerify: false                          # Skip TLS certificate verification
```

## Environment Variables

All configuration options can be set via environment variables with the `DAGU_` prefix.

### Server Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `DAGU_HOST` | `127.0.0.1` | Server binding host |
| `DAGU_PORT` | `8080` | Server binding port |
| `DAGU_BASE_PATH` | `""` | Base path for reverse proxy |
| `DAGU_TZ` | System timezone | Server timezone (e.g., `Asia/Tokyo`) |
| `DAGU_DEBUG` | `false` | Enable debug mode |
| `DAGU_LOG_FORMAT` | `text` | Log format (`text` or `json`) |
| `DAGU_HEADLESS` | `false` | Run without Web UI (1=enabled) |

### Directory Paths

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `DAGU_HOME` | - | Set all directories to this base path |
| `DAGU_DAGS_DIR` | `~/.config/dagu/dags` | DAG definitions directory |
| `DAGU_WORK_DIR` | DAG location | Default working directory |
| `DAGU_LOG_DIR` | `~/.local/share/dagu/logs` | Log files directory |
| `DAGU_DATA_DIR` | `~/.local/share/dagu/data` | Application data directory |
| `DAGU_SUSPEND_FLAGS_DIR` | `~/.local/share/dagu/suspend` | Suspend flags directory |
| `DAGU_ADMIN_LOG_DIR` | `~/.local/share/dagu/logs/admin` | Admin logs directory |
| `DAGU_BASE_CONFIG` | `~/.config/dagu/base.yaml` | Base configuration file |

### Authentication

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `DAGU_AUTH_BASIC_ENABLED` | `false` | Enable basic authentication |
| `DAGU_AUTH_BASIC_USERNAME` | `""` | Basic auth username |
| `DAGU_AUTH_BASIC_PASSWORD` | `""` | Basic auth password |
| `DAGU_AUTH_TOKEN_ENABLED` | `false` | Enable API token authentication |
| `DAGU_AUTH_TOKEN` | `""` | API token value |

### TLS/HTTPS

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `DAGU_CERT_FILE` | `""` | SSL certificate file path |
| `DAGU_KEY_FILE` | `""` | SSL private key file path |

### UI Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `DAGU_UI_NAVBAR_COLOR` | `""` | Navigation bar color |
| `DAGU_UI_NAVBAR_TITLE` | `Dagu` | Navigation bar title |
| `DAGU_UI_LOG_ENCODING_CHARSET` | `utf-8` | Log file encoding |
| `DAGU_UI_MAX_DASHBOARD_PAGE_LIMIT` | `100` | Dashboard page limit |

### Queue System

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `DAGU_QUEUE_ENABLED` | `true` | Enable/disable queue system |

### Permissions

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `DAGU_PERMISSIONS_WRITE_DAGS` | `true` | Allow DAG modifications |
| `DAGU_PERMISSIONS_RUN_DAGS` | `true` | Allow DAG execution |

## Base Configuration

The base configuration file (`~/.config/dagu/base.yaml`) provides default settings for all DAGs.

```yaml
# base.yaml - Shared configuration for all DAGs

# Environment variables available to all DAGs
env:
  - ENVIRONMENT: production
  - LOG_LEVEL: info
  - COMPANY: "ACME Corp"

# Default parameters
params:
  - DEFAULT_TIMEOUT: 3600
  - RETRY_LIMIT: 3

# Queue assignment
queue: "default"              # Default queue for all DAGs
maxActiveRuns: 1              # Maximum concurrent runs per DAG

# Working directory
workDir: "/opt/dagu/work"     # Default working directory

# Log configuration
logDir: "/var/log/dagu/dags"  # Custom log directory for DAG output

# History retention
histRetentionDays: 30         # Keep execution history for 30 days

# Email notifications
mailOn:
  failure: true               # Send email on failure
  success: false              # Don't send on success

# SMTP configuration
smtp:
  host: "smtp.gmail.com"
  port: "587"
  username: "notifications@company.com"
  password: "${SMTP_PASSWORD}"  # Use environment variable

# Error mail settings
errorMail:
  from: "dagu@company.com"
  to: "ops-team@company.com"
  prefix: "[DAGU ERROR]"
  attachLogs: true            # Attach execution logs

# Info mail settings
infoMail:
  from: "dagu@company.com"
  to: "ops-team@company.com"
  prefix: "[DAGU INFO]"
```

## Command-Line Flags

### Server Command

```bash
dagu server [flags]
```

| Flag | Environment Variable | Description |
|------|---------------------|-------------|
| `--host` | `DAGU_HOST` | Server host |
| `--port` | `DAGU_PORT` | Server port |
| `--dags` | `DAGU_DAGS_DIR` | DAGs directory |
| `--config` | - | Config file path |
| `--debug` | `DAGU_DEBUG` | Debug mode |

### Scheduler Command

```bash
dagu scheduler [flags]
```

| Flag | Environment Variable | Description |
|------|---------------------|-------------|
| `--dags` | `DAGU_DAGS_DIR` | DAGs directory |
| `--config` | - | Config file path |
| `--debug` | `DAGU_DEBUG` | Debug mode |

### Start-All Command

```bash
dagu start-all [flags]
```

Combines server and scheduler with all flags from both commands.

## Configuration Precedence

Configuration values are applied in the following order (highest to lowest precedence):

1. **Command-line flags**
2. **Environment variables**
3. **Configuration file** (`config.yaml`)
4. **Base configuration** (`base.yaml`)
5. **Default values**

Example:
```bash
# This will use port 9000, overriding all other settings
DAGU_PORT=8080 dagu start-all --port 9000
```

## Special Environment Variables

These environment variables are automatically set by Dagu and available in all DAG executions:

| Variable | Description | Example |
|----------|-------------|---------|
| `DAG_NAME` | Name of the current DAG | `my-workflow` |
| `DAG_RUN_ID` | Unique ID for this execution | `20240115_140000_abc123` |
| `DAG_RUN_LOG_FILE` | Path to the main log file | `/logs/my-workflow/20240115_140000.log` |
| `DAG_RUN_STEP_NAME` | Name of the current step | `process-data` |
| `DAG_RUN_STEP_STDOUT_FILE` | Path to step's stdout log | `/logs/my-workflow/process-data.stdout.log` |
| `DAG_RUN_STEP_STDERR_FILE` | Path to step's stderr log | `/logs/my-workflow/process-data.stderr.log` |

## Directory Structure

### Default Structure (XDG)

When no custom paths are configured:

```
~/.config/dagu/
├── dags/              # DAG definitions
├── config.yaml        # Main configuration
└── base.yaml          # Base configuration

~/.local/share/dagu/
├── logs/
│   ├── admin/         # Scheduler/admin logs
│   └── dags/          # DAG execution logs
├── data/              # DAG state and history
└── suspend/           # Suspend flag files
```

### Custom Structure (DAGU_HOME)

When `DAGU_HOME` is set:

```
$DAGU_HOME/
├── dags/              # DAG definitions
├── logs/              # All logs
├── data/              # State and history
├── suspend/           # Suspend flags
├── config.yaml        # Main configuration
└── base.yaml          # Base configuration
```

## Configuration Examples

### Minimal Configuration

```yaml
# Minimal config for local development
port: 8080
```

### Production Configuration

```yaml
# Production-ready configuration
host: 0.0.0.0
port: 443

# Security
auth:
  basic:
    enabled: true
    username: admin
    password: ${ADMIN_PASSWORD}
tls:
  certFile: /etc/ssl/certs/dagu.crt
  keyFile: /etc/ssl/private/dagu.key

# Permissions
permissions:
  writeDAGs: false    # Read-only in production
  runDAGs: true

# Paths
dagsDir: /opt/dagu/workflows
logDir: /var/log/dagu
dataDir: /var/lib/dagu

# UI
ui:
  navbarColor: "#ff0000"
  navbarTitle: "Production Workflows"

# Queues
queues:
  enabled: true
  config:
    - name: critical
      maxConcurrency: 10
    - name: default
      maxConcurrency: 5
```

### Multi-Environment Configuration

```yaml
# Configuration with remote nodes
remoteNodes:
  - name: development
    apiBaseURL: http://dev.internal:8080/api/v1
    isBasicAuth: true
    basicAuthUsername: dev
    basicAuthPassword: ${DEV_PASSWORD}
    
  - name: staging
    apiBaseURL: https://staging.company.com/api/v1
    isAuthToken: true
    authToken: ${STAGING_TOKEN}
    
  - name: production
    apiBaseURL: https://prod.company.com/api/v1
    isAuthToken: true
    authToken: ${PROD_TOKEN}
```

## Validation

Validate your configuration:

```bash
# Check config syntax
dagu validate --config ~/.config/dagu/config.yaml

# Test with dry run
dagu start-all --config ~/.config/dagu/config.yaml --dry-run
```

## Migration from Older Versions

### From v1.13 or earlier

1. Update authentication configuration:
   ```yaml
   # Old format
   auth:
     enabled: true
     username: admin
     password: secret
   
   # New format
   auth:
     basic:
       enabled: true
       username: admin
       password: secret
   ```

2. Update directory paths to use XDG structure
3. Migrate from `DAGU_DATA` to `DAGU_DATA_DIR`

## Troubleshooting

### Configuration Not Loading

1. Check file location and permissions
2. Validate YAML syntax
3. Check environment variable conflicts
4. Enable debug mode: `--debug`

### Environment Variables Not Working

1. Ensure proper prefix: `DAGU_`
2. Check for typos in variable names
3. Verify shell exports: `export DAGU_PORT=8080`
4. Check precedence order

### Permission Errors

1. Verify file ownership
2. Check directory permissions
3. Ensure user has read/write access
4. Review SELinux/AppArmor policies

## See Also

- [Configure server settings](/configurations/server) for your environment
- [Set up for production](/configurations/operations) with monitoring
- [Explore advanced patterns](/configurations/advanced) for complex deployments
- [Review security best practices](/configurations/operations#security-hardening)
