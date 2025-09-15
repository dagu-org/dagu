# Configurations

Deploy, configure, and operate Dagu.

## Configuration Methods

Precedence order:
1. Command-line flags (highest)
2. Environment variables (`DAGU_` prefix)
3. Configuration file (lowest)

```bash
# Port 9000 wins
dagu start-all --port 9000

# Despite env var
export DAGU_PORT=8080

# And config file
port: 7000
```

## Quick Start

### Development
```bash
# Zero config
dagu start-all
```

### Production
```yaml
# ~/.config/dagu/config.yaml
host: 0.0.0.0
port: 8080

auth:
  basic:
    enabled: true
    username: admin
    password: ${ADMIN_PASSWORD}

paths:
  dagsDir: /opt/dagu/workflows
  logDir: /var/log/dagu
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

## Topics

**[Server Configuration](/configurations/server)**
- Host, port, authentication
- TLS/HTTPS setup
- UI customization

**[Operations](/configurations/operations)**
- Running as a service
- Monitoring and metrics
- Logging and alerting

**[Remote Nodes](/configurations/remote-nodes)**
- Configure remote instances
- Multi-node setup

**[Distributed Execution](/features/distributed-execution)**
- Coordinator and worker setup
- Service registry configuration
- Worker labels and routing

**[Configuration Reference](/configurations/reference)**
- All options
- Environment variables
- Examples

## Common Configurations

### Production
```yaml
host: 127.0.0.1
port: 8080

tls:
  certFile: /etc/ssl/cert.pem
  keyFile: /etc/ssl/key.pem

auth:
  basic:
    enabled: true
    username: admin
    password: ${ADMIN_PASSWORD}

permissions:
  writeDAGs: false  # Read-only
  runDAGs: true

ui:
  navbarColor: "#FF0000"
  navbarTitle: "Production"
```

### Development
```yaml
host: 127.0.0.1
port: 8080
debug: true

auth:
  basic:
    enabled: false
```

## Environment Variables

```bash
# Server
export DAGU_HOST=0.0.0.0
export DAGU_PORT=8080

# Paths
export DAGU_DAGS_DIR=/opt/workflows
export DAGU_LOG_DIR=/var/log/dagu

# Auth
export DAGU_AUTH_BASIC_USERNAME=admin
export DAGU_AUTH_BASIC_PASSWORD=secret

dagu start-all
```

## See Also

- [Set up authentication](/configurations/server#authentication) for production
- [Configure monitoring](/configurations/operations#monitoring) for visibility
- [Set up distributed execution](/features/distributed-execution) for scaling
- [Review all options](/configurations/reference) for fine-tuning
