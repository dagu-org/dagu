# Configurations

Everything about deploying, configuring, and operating Dagu in production.

## Overview

Dagu is designed to be simple to configure. You can run it with zero configuration for development, or customize every aspect for production deployments.

## Configuration Methods

Dagu supports three configuration methods, in order of precedence:

1. **Command-line flags** (highest priority)
2. **Environment variables** (prefix: `DAGU_`)  
3. **Configuration file** (lowest priority)

```bash
# CLI flag takes precedence
dagu start-all --port 9000

# Even if environment variable is set
export DAGU_PORT=8080

# And config file specifies
port: 7000
```

## Quick Start Configuration

### Development

Zero configuration needed:

```bash
# Just run - uses all defaults
dagu start-all
```

### Basic Production

Create `~/.config/dagu/config.yaml`:

```yaml
# Server settings
host: 0.0.0.0  # Listen on all interfaces
port: 8080

# Authentication
auth:
  basic:
    enabled: true
    username: admin
    password: ${DAGU_ADMIN_PASSWORD}  # From environment

# Directories
dagsDir: /opt/dagu/workflows
logDir: /var/log/dagu
dataDir: /var/lib/dagu
```

### Docker

Use environment variables:

```bash
docker run -d \
  -e DAGU_HOST=0.0.0.0 \
  -e DAGU_PORT=8080 \
  -e DAGU_AUTH_BASIC_USERNAME=admin \
  -e DAGU_AUTH_BASIC_PASSWORD=secret \
  -p 8080:8080 \
  ghcr.io/dagu-org/dagu:latest
```

## Configuration Topics

### 📦 [Installation & Setup](/getting-started/installation)
- Installation methods
- System requirements  
- Initial setup
- Directory structure

### ⚙️ [Server Configuration](/configurations/server)
- Host and port settings
- Authentication setup
- TLS/HTTPS configuration
- UI customization

### 🚀 [Operations](/configurations/operations)
- Running as a service
- Monitoring and metrics
- Logging configuration
- Backup and recovery
- Performance tuning

### 🔧 [Advanced Setup](/configurations/advanced)
- Remote nodes configuration
- Queue management
- CI/CD integration

### 📖 [Configuration Reference](/configurations/reference)
- All configuration options
- Environment variables
- Default values
- Example configurations

## Common Configurations

### Secure Production Setup

```yaml
# ~/.config/dagu/config.yaml
host: 127.0.0.1  # Only localhost
port: 8080

# Enable HTTPS
tls:
  certFile: /etc/dagu/cert.pem
  keyFile: /etc/dagu/key.pem

# Authentication
auth:
  basic:
    enabled: true
    username: admin
    password: ${DAGU_ADMIN_PASSWORD}

# Permissions
permissions:
  writeDAGs: false  # Read-only UI
  runDAGs: true     # Can execute

# Paths
dagsDir: /opt/dagu/workflows
logDir: /var/log/dagu
dataDir: /var/lib/dagu

# UI
ui:
  navbarColor: "#FF0000"
  navbarTitle: "Production Workflows"
```

### High-Performance Setup

```yaml
# Server settings for high-throughput
debug: false

# Paths
logDir: /var/log/dagu
dataDir: /var/lib/dagu
  
# Note: maxActiveRuns, maxCleanUpTimeSec, and histRetentionDays 
# are DAG-level configurations, not server configurations
```

### Development Setup

```yaml
# For local development
host: 127.0.0.1
port: 8080
debug: true

# Hot reload friendly
dagsDir: ./workflows
workDir: ./

# No authentication
auth:
  basic:
    enabled: false
```

## Environment Variables

All configuration options can be set via environment variables:

```bash
# Server settings
export DAGU_HOST=0.0.0.0
export DAGU_PORT=8080

# Paths
export DAGU_DAGS_DIR=/opt/workflows
export DAGU_LOG_DIR=/var/log/dagu
export DAGU_DATA_DIR=/var/lib/dagu

# Authentication
export DAGU_AUTH_BASIC_USERNAME=admin
export DAGU_AUTH_BASIC_PASSWORD=secret

# Start with environment config
dagu start-all
```

## Configuration Precedence

Understanding precedence is crucial for troubleshooting:

```yaml
# config.yaml
port: 8080  # Lowest priority
```

```bash
export DAGU_PORT=9000  # Medium priority
dagu start-all --port 8000  # Highest priority - wins
```

## Migration Guide

### From Cron

1. Export existing cron jobs
2. Convert to Dagu YAML
3. Test in dry-run mode
4. Deploy with same schedule

### From Airflow

1. Map DAGs to Dagu workflows
2. Convert operators to executors
3. Migrate connections to env vars
4. Test parallel execution

### From Other Tools

1. Identify workflow patterns
2. Map to Dagu concepts
3. Test incrementally
4. Run in parallel during transition

## Troubleshooting

### Configuration Not Loading

Check precedence:
```bash
# See what config is loaded
dagu start-all --debug

# Check environment
env | grep DAGU_
```

### Permission Issues

```bash
# Check file permissions
ls -la ~/.config/dagu/
ls -la ~/.local/share/dagu/

# Fix ownership
sudo chown -R $USER:$USER ~/.config/dagu
sudo chown -R $USER:$USER ~/.local/share/dagu
```

### Port Conflicts

```bash
# Find what's using the port
lsof -i :8080

# Use different port
dagu start-all --port 9000
```

## See Also

- [Set up authentication](/configurations/server#authentication) for production
- [Configure monitoring](/configurations/operations#monitoring) for visibility
- [Integrate with CI/CD](/configurations/advanced#cicd-integration) for automation
- [Review all options](/configurations/reference) for fine-tuning
