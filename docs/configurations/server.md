# Server Configuration

Configure Dagu server settings.

## Configuration Methods

Precedence order:
1. Command-line flags (highest)
2. Environment variables (`DAGU_` prefix)
3. Configuration file (lowest)

```bash
# CLI flag wins
dagu start-all --port=8000

# Even with env var
export DAGU_PORT=8080

# And config file
port: 7000
```

## Configuration File

Location: `~/.config/dagu/config.yaml`

```yaml
# Server Configuration
host: "127.0.0.1"         # Web UI binding host
port: 8080                # Web UI binding port
basePath: ""              # Base path for reverse proxy (e.g., "/dagu")
tz: "Asia/Tokyo"          # Server timezone
debug: false              # Debug mode
logFormat: "text"         # Log format: "text" or "json"
headless: false           # Run without Web UI

# Directory Paths
dagsDir: "~/.config/dagu/dags"                    # DAG definitions
workDir: ""                                       # Default working directory
logDir: "~/.local/share/dagu/logs"                # Log files
dataDir: "~/.local/share/dagu/data"               # Application data
suspendFlagsDir: "~/.local/share/dagu/suspend"    # Suspend flags
adminLogsDir: "~/.local/share/dagu/logs/admin"    # Admin logs
baseConfig: "~/.config/dagu/base.yaml"            # Base configuration

# Permissions
permissions:
  writeDAGs: true         # Allow creating/editing/deleting DAGs
  runDAGs: true           # Allow running/stopping/retrying DAGs

# Authentication
auth:
  basic:
    enabled: true
    username: "admin"
    password: "secret"
  token:
    enabled: true
    value: "your-secret-token"

# TLS/HTTPS Configuration
tls:
  certFile: "/path/to/cert.pem"
  keyFile: "/path/to/key.pem"

# UI Customization
ui:
  navbarColor: "#1976d2"        # Header color (hex or name)
  navbarTitle: "Dagu"           # Header title
  logEncodingCharset: "utf-8"   # Log file encoding
  maxDashboardPageLimit: 100    # Max items on dashboard

# Latest Status Configuration
latestStatusToday: true         # Show only today's latest status

# Queue System
queues:
  enabled: true                 # Enable queue system
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

All options support `DAGU_` prefix:

**Server:**
- `DAGU_HOST` - Host (default: `127.0.0.1`)
- `DAGU_PORT` - Port (default: `8080`)
- `DAGU_TZ` - Timezone
- `DAGU_DEBUG` - Debug mode
- `DAGU_LOG_FORMAT` - Log format (`text`/`json`)

**Paths:**
- `DAGU_HOME` - Set all paths
- `DAGU_DAGS_DIR` - DAGs directory
- `DAGU_LOG_DIR` - Logs
- `DAGU_DATA_DIR` - Data

**Auth:**
- `DAGU_AUTH_BASIC_ENABLED`
- `DAGU_AUTH_BASIC_USERNAME`
- `DAGU_AUTH_BASIC_PASSWORD`
- `DAGU_AUTH_TOKEN`

## Common Setups

### Development
```yaml
host: "127.0.0.1"
port: 8080
debug: true
```

### Production
```yaml
host: "0.0.0.0"
port: 443
permissions:
  writeDAGs: false
auth:
  basic:
    enabled: true
    username: "admin"
    password: "${ADMIN_PASSWORD}"
tls:
  certFile: "/etc/ssl/cert.pem"
  keyFile: "/etc/ssl/key.pem"
```

### Docker
```bash
docker run -d \
  -e DAGU_HOST=0.0.0.0 \
  -e DAGU_AUTH_BASIC_ENABLED=true \
  -e DAGU_AUTH_BASIC_USERNAME=admin \
  -e DAGU_AUTH_BASIC_PASSWORD=secret \
  -p 8080:8080 \
  ghcr.io/dagu-org/dagu:latest
```

## Authentication

### Basic Auth
```yaml
auth:
  basic:
    enabled: true
    username: "admin"
    password: "${ADMIN_PASSWORD}"
```

### API Token
```yaml
auth:
  token:
    enabled: true
    value: "${API_TOKEN}"
```

```bash
curl -H "Authorization: Bearer your-token" \
  http://localhost:8080/api/v2/dags
```


### TLS/HTTPS

**Let's Encrypt:**
```bash
certbot certonly --standalone -d dagu.example.com

export DAGU_CERT_FILE=/etc/letsencrypt/live/dagu.example.com/fullchain.pem
export DAGU_KEY_FILE=/etc/letsencrypt/live/dagu.example.com/privkey.pem
```

**Behind Nginx:**
```yaml
# config.yaml
basePath: "/dagu"
host: "127.0.0.1"
port: 8080
```

```nginx
location /dagu/ {
    proxy_pass http://127.0.0.1:8080/;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
}
```

## UI Customization

```yaml
ui:
  navbarColor: "#1976d2"
  navbarTitle: "Workflows"
  logEncodingCharset: "utf-8"
```

Color suggestions:
- Production: `#ff0000` (red)
- Staging: `#ff9800` (orange)
- Development: `#4caf50` (green)

## Remote Nodes

```yaml
remoteNodes:
  - name: "production"
    apiBaseURL: "https://prod.example.com/api/v2"
    isBasicAuth: true
    basicAuthUsername: "admin"
    basicAuthPassword: "${PROD_PASSWORD}"
    
  - name: "staging"
    apiBaseURL: "https://staging.example.com/api/v2"
    isAuthToken: true
    authToken: "${STAGING_TOKEN}"
```

## Queues

```yaml
queues:
  enabled: true
  config:
    - name: "critical"
      maxConcurrency: 5
    - name: "batch"
      maxConcurrency: 1
    - name: "default"
      maxConcurrency: 2
```

## Base Configuration

`~/.config/dagu/base.yaml` - Shared DAG settings:

```yaml
mailOn:
  failure: true

smtp:
  host: "smtp.company.com"
  port: "587"
  username: "${SMTP_USER}"
  password: "${SMTP_PASS}"

env:
  - ENVIRONMENT: production
```

## Troubleshooting

**Config not loading:**
```bash
ls -la ~/.config/dagu/config.yaml
yamllint ~/.config/dagu/config.yaml
env | grep DAGU_
```

**Port in use:**
```bash
lsof -i :8080
dagu start-all --port 9000
```

**Permissions:**
```bash
chmod 755 ~/.config/dagu
chown -R $USER:$USER ~/.config/dagu
```

## See Also

- [Set up authentication](#authentication) for secure access
- [Configure remote nodes](#remote-nodes) for multi-environment management
- [Customize the UI](#ui-customization) for your organization
- [Enable HTTPS](#tlshttps-configuration) for encrypted connections
