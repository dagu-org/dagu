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
apiBasePath: "/api/v2"    # API endpoint base path
tz: "Asia/Tokyo"          # Server timezone
debug: false              # Debug mode
logFormat: "text"         # Log format: "text" or "json"
headless: false           # Run without Web UI

# Directory Paths (must be under "paths" key)
paths:
  dagsDir: "~/.config/dagu/dags"                    # DAG definitions
  logDir: "~/.local/share/dagu/logs"                # Log files
  dataDir: "~/.local/share/dagu/data"               # Application data
  suspendFlagsDir: "~/.local/share/dagu/suspend"    # Suspend flags
  adminLogsDir: "~/.local/share/dagu/logs/admin"    # Admin logs
  baseConfig: "~/.config/dagu/base.yaml"            # Base configuration
  dagRunsDir: ""                                    # Auto: {dataDir}/dag-runs
  queueDir: ""                                      # Auto: {dataDir}/queue
  procDir: ""                                       # Auto: {dataDir}/proc
  executable: ""                                    # Auto: current executable

# Permissions
permissions:
  writeDAGs: true         # Allow creating/editing/deleting DAGs
  runDAGs: true           # Allow running/stopping/retrying DAGs

# Authentication (enabled when credentials are set)
auth:
  basic:
    username: "admin"
    password: "secret"
  token:
    value: "your-secret-token"
  oidc:
    clientId: "your-client-id"
    clientSecret: "your-client-secret"
    clientUrl: "http://localhost:8080"
    issuer: "https://accounts.google.com"
    scopes: ["openid", "profile", "email"]
    whitelist: ["admin@example.com"]

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
  dags:
    sortField: "name"           # Default sort field (name/status/lastRun/schedule/suspended)
    sortOrder: "asc"            # Default sort order (asc/desc)

# Latest Status Configuration
latestStatusToday: true         # Show only today's latest status

# Queue System
queues:
  enabled: true                 # Enable queue system (default: true)
  config:
    - name: "critical"
      maxConcurrency: 5          # Maximum concurrent DAG runs
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
- `DAGU_AUTH_BASIC_USERNAME` - Basic auth username
- `DAGU_AUTH_BASIC_PASSWORD` - Basic auth password
- `DAGU_AUTH_TOKEN` - API token
- `DAGU_AUTH_OIDC_CLIENT_ID` - OIDC client ID
- `DAGU_AUTH_OIDC_CLIENT_SECRET` - OIDC client secret
- `DAGU_AUTH_OIDC_CLIENT_URL` - OIDC client URL
- `DAGU_AUTH_OIDC_ISSUER` - OIDC issuer URL
- `DAGU_AUTH_OIDC_SCOPES` - OIDC scopes (comma-separated)
- `DAGU_AUTH_OIDC_WHITELIST` - OIDC email whitelist (comma-separated)

**UI:**
- `DAGU_UI_DAGS_SORT_FIELD` - Default DAGs page sort field
- `DAGU_UI_DAGS_SORT_ORDER` - Default DAGs page sort order

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
    username: "admin"
    password: "${ADMIN_PASSWORD}"
```

### API Token
```yaml
auth:
  token:
    value: "${API_TOKEN}"
```

```bash
curl -H "Authorization: Bearer your-token" \
  http://localhost:8080/api/v2/dags
```

### OIDC Authentication
```yaml
auth:
  oidc:
    clientId: "${OIDC_CLIENT_ID}"
    clientSecret: "${OIDC_CLIENT_SECRET}"
    clientUrl: "https://dagu.example.com"
    issuer: "https://accounts.google.com"
    scopes: 
      - "email"
    whitelist:
      - "admin@dagu.example.com" # Optional: restrict to specific emails
```

See [OIDC Configuration](authentication/oidc) for detailed setup.

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

## See Also

- [Set up authentication](#authentication) for secure access
- [Configure remote nodes](#remote-nodes) for multi-environment management
- [Customize the UI](#ui-customization) for your organization
- [Enable HTTPS](#tlshttps-configuration) for encrypted connections
