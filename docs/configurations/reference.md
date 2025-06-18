# Configuration Reference

All Dagu configuration options.

## Configuration File

Default: `~/.config/dagu/config.yaml`

```yaml
# Server
host: "127.0.0.1"
port: 8080
basePath: ""              # For reverse proxy (e.g., "/dagu")
tz: "America/New_York"
debug: false
logFormat: "text"         # "text" or "json"
headless: false

# Directories
dagsDir: "~/.config/dagu/dags"
workDir: ""               # Default working directory
logDir: "~/.local/share/dagu/logs"
dataDir: "~/.local/share/dagu/data"
suspendFlagsDir: "~/.local/share/dagu/suspend"
adminLogsDir: "~/.local/share/dagu/logs/admin"
baseConfig: "~/.config/dagu/base.yaml"

# Permissions
permissions:
  writeDAGs: true         # Create/edit/delete DAGs
  runDAGs: true           # Run/stop/retry DAGs

# Authentication
auth:
  basic:
    enabled: true
    username: "admin"
    password: "secret"
  token:
    enabled: true
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

latestStatusToday: true      # Show only today's status

# Queues
queues:
  enabled: true
  config:
    - name: "critical"
      maxConcurrency: 5
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
```

## Environment Variables

All options support `DAGU_` prefix.

### Server
- `DAGU_HOST` - Server host (default: `127.0.0.1`)
- `DAGU_PORT` - Server port (default: `8080`)
- `DAGU_BASE_PATH` - Base path for reverse proxy
- `DAGU_TZ` - Server timezone
- `DAGU_DEBUG` - Enable debug mode
- `DAGU_LOG_FORMAT` - Log format (`text`/`json`)
- `DAGU_HEADLESS` - Run without UI

### Directories
- `DAGU_HOME` - Set all directories to this path
- `DAGU_DAGS_DIR` - DAG definitions
- `DAGU_WORK_DIR` - Default working directory
- `DAGU_LOG_DIR` - Log files
- `DAGU_DATA_DIR` - Application data
- `DAGU_SUSPEND_FLAGS_DIR` - Suspend flags
- `DAGU_ADMIN_LOG_DIR` - Admin logs
- `DAGU_BASE_CONFIG` - Base configuration

### Authentication
- `DAGU_AUTH_BASIC_ENABLED` - Enable basic auth
- `DAGU_AUTH_BASIC_USERNAME` - Username
- `DAGU_AUTH_BASIC_PASSWORD` - Password
- `DAGU_AUTH_TOKEN_ENABLED` - Enable token auth
- `DAGU_AUTH_TOKEN` - API token

### TLS/HTTPS
- `DAGU_CERT_FILE` - SSL certificate
- `DAGU_KEY_FILE` - SSL key

### UI
- `DAGU_UI_NAVBAR_COLOR` - Nav bar color
- `DAGU_UI_NAVBAR_TITLE` - Nav bar title
- `DAGU_UI_LOG_ENCODING_CHARSET` - Log encoding
- `DAGU_UI_MAX_DASHBOARD_PAGE_LIMIT` - Dashboard limit

### Queue
- `DAGU_QUEUE_ENABLED` - Enable queue system

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

### Server/Start-All
- `--host` - Server host
- `--port` - Server port
- `--dags` - DAGs directory
- `--config` - Config file
- `--debug` - Debug mode

### Scheduler
- `--dags` - DAGs directory
- `--config` - Config file
- `--debug` - Debug mode

## Configuration Precedence

1. Command-line flags (highest)
2. Environment variables
3. Configuration file
4. Base configuration
5. Default values (lowest)

```bash
# Port 9000 wins (CLI flag beats env var)
DAGU_PORT=8080 dagu start-all --port 9000
```

## Special Environment Variables

Automatically set during DAG execution:

- `DAG_NAME` - Current DAG name
- `DAG_RUN_ID` - Unique execution ID
- `DAG_RUN_LOG_FILE` - Main log file path
- `DAG_RUN_STEP_NAME` - Current step name
- `DAG_RUN_STEP_STDOUT_FILE` - Step stdout log
- `DAG_RUN_STEP_STDERR_FILE` - Step stderr log

## Directory Structure

### Default (XDG)
```
~/.config/dagu/
├── dags/
├── config.yaml
└── base.yaml

~/.local/share/dagu/
├── logs/
│   ├── admin/
│   └── dags/
├── data/
└── suspend/
```

### With DAGU_HOME
```
$DAGU_HOME/
├── dags/
├── logs/
├── data/
├── suspend/
├── config.yaml
└── base.yaml
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
    enabled: true
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

## Validation

```bash
# Check config syntax
dagu validate --config ~/.config/dagu/config.yaml

# Test with dry run
dagu start-all --config ~/.config/dagu/config.yaml --dry-run
```

## Migration Notes

### v1.13 → v1.14+

Update authentication format:
```yaml
# Old
auth:
  enabled: true
  username: admin
  password: secret

# New
auth:
  basic:
    enabled: true
    username: admin
    password: secret
```

## Troubleshooting

**Configuration not loading:**
- Check file exists: `ls -la ~/.config/dagu/config.yaml`
- Validate YAML syntax
- Enable debug: `--debug`

**Environment variables not working:**
- Use `DAGU_` prefix
- Export properly: `export DAGU_PORT=8080`
- Check precedence (CLI > env > file)

**Permission errors:**
- Fix ownership: `chown -R $USER ~/.config/dagu`
- Check permissions: `chmod 755 ~/.config/dagu`

## See Also

- [Server Configuration](/configurations/server)
- [Production Setup](/configurations/operations)
- [Advanced Patterns](/configurations/advanced)
