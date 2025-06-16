# Server Configuration

Configure Dagu server settings for development and production environments.

## Configuration Methods

Dagu supports three configuration methods, in order of precedence:

1. **Command-line flags** (highest priority)
2. **Environment variables** (prefix: `DAGU_`)
3. **Configuration file** (lowest priority)

```bash
# Command-line flag (highest precedence)
dagu start-all --host=0.0.0.0 --port=8000

# Environment variable
export DAGU_PORT=8080

# Configuration file (lowest precedence)
# ~/.config/dagu/config.yaml
port: 7000
```

## Configuration File

The default configuration file location is `~/.config/dagu/config.yaml`. Here's a complete example with all available options:

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
    apiBaseURL: "https://staging.example.com/api/v1"
    isBasicAuth: true
    basicAuthUsername: "admin"
    basicAuthPassword: "password"
  - name: "production"
    apiBaseURL: "https://prod.example.com/api/v1"
    isAuthToken: true
    authToken: "prod-token"
    skipTLSVerify: false
```

## Environment Variables

All configuration options can be set via environment variables with the `DAGU_` prefix:

### Server Settings
- `DAGU_HOST` (default: `127.0.0.1`) - Server binding host
- `DAGU_PORT` (default: `8080`) - Server binding port
- `DAGU_BASE_PATH` (default: `""`) - Base path for reverse proxy
- `DAGU_TZ` (default: system timezone) - Server timezone
- `DAGU_DEBUG` (default: `false`) - Enable debug mode
- `DAGU_LOG_FORMAT` (default: `text`) - Log format
- `DAGU_HEADLESS` (default: `false`) - Run without Web UI

### Directory Paths
- `DAGU_HOME` - Set all directories to a custom location (e.g., `~/.dagu`)
- `DAGU_DAGS_DIR` - DAG definitions directory
- `DAGU_WORK_DIR` - Default working directory
- `DAGU_LOG_DIR` - Log files directory
- `DAGU_DATA_DIR` - Application data directory
- `DAGU_SUSPEND_FLAGS_DIR` - Suspend flags directory
- `DAGU_ADMIN_LOG_DIR` - Admin logs directory
- `DAGU_BASE_CONFIG` - Base configuration file path

### Authentication
- `DAGU_AUTH_BASIC_ENABLED` - Enable basic auth
- `DAGU_AUTH_BASIC_USERNAME` - Basic auth username
- `DAGU_AUTH_BASIC_PASSWORD` - Basic auth password
- `DAGU_AUTH_TOKEN_ENABLED` - Enable API token auth
- `DAGU_AUTH_TOKEN` - API token value

### TLS/HTTPS
- `DAGU_CERT_FILE` - SSL certificate file path
- `DAGU_KEY_FILE` - SSL key file path

### UI Customization
- `DAGU_UI_NAVBAR_COLOR` - Navigation bar color
- `DAGU_UI_NAVBAR_TITLE` - Navigation bar title
- `DAGU_UI_LOG_ENCODING_CHARSET` - Log encoding charset

### Queue System
- `DAGU_QUEUE_ENABLED` (default: `true`) - Enable/disable queue system

## Common Configurations

### Development Setup

```yaml
# ~/.config/dagu/config.yaml
host: "127.0.0.1"
port: 8080
debug: true
ui:
  navbarColor: "#00ff00"
  navbarTitle: "Dagu - DEV"
```

### Production Setup

```yaml
# /etc/dagu/config.yaml
host: "0.0.0.0"
port: 443
headless: false
permissions:
  writeDAGs: false    # Read-only in production
  runDAGs: true
auth:
  basic:
    enabled: true
    username: "admin"
    password: "${DAGU_ADMIN_PASSWORD}"  # From environment
tls:
  certFile: "/etc/dagu/cert.pem"
  keyFile: "/etc/dagu/key.pem"
ui:
  navbarColor: "#ff0000"
  navbarTitle: "Dagu - PRODUCTION"
```

### Docker Setup

```bash
docker run -d \
  -e DAGU_HOST=0.0.0.0 \
  -e DAGU_PORT=8080 \
  -e DAGU_AUTH_BASIC_ENABLED=true \
  -e DAGU_AUTH_BASIC_USERNAME=admin \
  -e DAGU_AUTH_BASIC_PASSWORD=secret \
  -e DAGU_UI_NAVBAR_COLOR="#ff0000" \
  -e DAGU_UI_NAVBAR_TITLE="Dagu - Docker" \
  -p 8080:8080 \
  -v ~/.config/dagu:/config \
  ghcr.io/dagu-org/dagu:latest start-all
```

## Authentication

### Basic Authentication

Enable username/password authentication:

```yaml
# config.yaml
auth:
  basic:
    enabled: true
    username: "admin"
    password: "${ADMIN_PASSWORD}"  # Use environment variable
```

Or via environment variables:

```bash
export DAGU_AUTH_BASIC_ENABLED=true
export DAGU_AUTH_BASIC_USERNAME=admin
export DAGU_AUTH_BASIC_PASSWORD=secret
dagu start-all
```

### API Token Authentication

Enable token-based authentication for API access:

```yaml
# config.yaml
auth:
  token:
    enabled: true
    value: "${API_TOKEN}"  # Use environment variable
```

Use the token in API requests:

```bash
curl -H "Authorization: Bearer your-secret-token" \
  http://localhost:8080/api/v1/dags
```

### Permissions

Control what users can do:

```yaml
permissions:
  writeDAGs: true    # Create, edit, delete DAGs
  runDAGs: true      # Start, stop, retry DAGs
```

## TLS/HTTPS Configuration

### Using Certificate Files

```yaml
# config.yaml
tls:
  certFile: "/etc/dagu/cert.pem"
  keyFile: "/etc/dagu/key.pem"
```

### Using Let's Encrypt

```bash
# Generate certificates
certbot certonly --standalone -d dagu.example.com

# Configure Dagu
export DAGU_CERT_FILE=/etc/letsencrypt/live/dagu.example.com/fullchain.pem
export DAGU_KEY_FILE=/etc/letsencrypt/live/dagu.example.com/privkey.pem
dagu start-all
```

### Behind a Reverse Proxy

When running behind nginx or another reverse proxy:

```yaml
# config.yaml
basePath: "/dagu"    # If serving from subdirectory
host: "127.0.0.1"    # Only listen locally
port: 8080
```

Nginx configuration:

```nginx
location /dagu/ {
    proxy_pass http://127.0.0.1:8080/;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

## UI Customization

### Branding

Customize the UI appearance:

```yaml
ui:
  navbarColor: "#1976d2"           # Material blue
  navbarTitle: "Workflow Engine"   # Custom title
```

Color examples:
- Production: `"#ff0000"` (red)
- Staging: `"#ff9800"` (orange)
- Development: `"#4caf50"` (green)
- Custom hex: `"#7b1fa2"` (purple)

### Log Encoding

Set character encoding for log files:

```yaml
ui:
  logEncodingCharset: "utf-8"    # Default
  # Options: utf-8, shift-jis, euc-jp, etc.
```

## Remote Nodes

Connect to other Dagu instances:

```yaml
remoteNodes:
  - name: "production"
    apiBaseURL: "https://prod.example.com/api/v1"
    isBasicAuth: true
    basicAuthUsername: "admin"
    basicAuthPassword: "${PROD_PASSWORD}"
    
  - name: "staging"
    apiBaseURL: "https://staging.example.com/api/v1"
    isAuthToken: true
    authToken: "${STAGING_TOKEN}"
    skipTLSVerify: false  # For self-signed certificates
```

## Queue Configuration

Configure named queues with concurrency limits:

```yaml
queues:
  enabled: true
  config:
    - name: "critical"
      maxConcurrency: 5      # Up to 5 critical DAGs
      
    - name: "batch"
      maxConcurrency: 1      # Only 1 batch job at a time
      
    - name: "default"
      maxConcurrency: 2      # Default queue
```

## Base Configuration

Share common settings across all DAGs:

```yaml
# ~/.config/dagu/base.yaml
mailOn:
  failure: true
  success: false

smtp:
  host: "smtp.company.com"
  port: "587"
  username: "${SMTP_USER}"
  password: "${SMTP_PASS}"

env:
  - COMPANY: "ACME Corp"
  - ENVIRONMENT: "production"
```

## Troubleshooting

### Configuration Not Loading

1. Check file location:
   ```bash
   ls -la ~/.config/dagu/config.yaml
   ```

2. Verify syntax:
   ```bash
   yamllint ~/.config/dagu/config.yaml
   ```

3. Check environment variables:
   ```bash
   env | grep DAGU_
   ```

### Port Already in Use

```bash
# Find process using port
lsof -i :8080

# Kill process or use different port
dagu start-all --port 9000
```

### Permission Denied

```bash
# Fix directory permissions
chmod 755 ~/.config/dagu
chmod 644 ~/.config/dagu/config.yaml

# Fix ownership
chown -R $USER:$USER ~/.config/dagu
```

### TLS Certificate Issues

```bash
# Verify certificate
openssl x509 -in cert.pem -text -noout

# Check certificate chain
openssl verify -CAfile ca.pem cert.pem
```

## Best Practices

1. **Use Environment Variables for Secrets**
   ```yaml
   # Good
   password: "${ADMIN_PASSWORD}"
   
   # Bad
   password: "hardcoded-password"
   ```

2. **Separate Environments**
   ```bash
   # Development
   dagu start-all --config ~/.config/dagu/dev.yaml
   
   # Production
   dagu start-all --config /etc/dagu/prod.yaml
   ```

3. **Lock Down Production**
   ```yaml
   permissions:
     writeDAGs: false    # No editing in production
     runDAGs: true       # Can still execute
   ```

4. **Monitor Configuration**
   ```bash
   # Log startup configuration
   dagu start-all --debug 2>&1 | tee /var/log/dagu-startup.log
   ```

5. **Version Control Templates**
   ```
   configs/
   ├── base.yaml
   ├── development.yaml
   ├── staging.yaml
   └── production.yaml.template  # Without secrets
   ```

## Next Steps

- [Set up authentication](#authentication) for secure access
- [Configure remote nodes](#remote-nodes) for multi-environment management
- [Customize the UI](#ui-customization) for your organization
- [Enable HTTPS](#tlshttps-configuration) for encrypted connections