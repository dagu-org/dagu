# Authentication

Secure your Dagu instance with authentication and access control.

## Basic Authentication

Enable username/password authentication:

```yaml
# ~/.config/dagu/config.yaml
auth:
  basic:
    username: admin
    password: secure-password
```

Access with credentials:
```bash
# CLI
export DAGU_USERNAME=admin
export DAGU_PASSWORD=secure-password
dagu status

# API
curl -u admin:secure-password http://localhost:8080/api/v2/dags
```

## Token Authentication

For programmatic access:

```yaml
auth:
  token:
    value: your-api-token
```

Use the token:
```bash
# CLI
export DAGU_API_TOKEN=your-api-token
dagu status

# API
curl -H "Authorization: Bearer your-api-token" \
     http://localhost:8080/api/v2/dags
```

## TLS/HTTPS

Enable encrypted connections:

```yaml
tls:
  certFile: /path/to/server.crt
  keyFile: /path/to/server.key
```

Generate self-signed certificate:
```bash
openssl req -x509 -newkey rsa:4096 -keyout server.key -out server.crt \
  -days 365 -nodes -subj "/CN=localhost"
```

## Permissions

Control user capabilities:

```yaml
permissions:
  writeDAGs: true   # Create/edit workflows
  runDAGs: true     # Execute workflows
```

Permission levels:
- **Read-only**: `writeDAGs: false, runDAGs: false`
- **Operator**: `writeDAGs: false, runDAGs: true`
- **Developer**: `writeDAGs: true, runDAGs: true`

## Environment Configuration

Set authentication via environment:

```bash
# Basic authentication (enabled when both username and password are set)
export DAGU_AUTH_BASIC_USERNAME=admin
export DAGU_AUTH_BASIC_PASSWORD=secret

# Token authentication (enabled when token is set)
export DAGU_AUTH_TOKEN=api-token

dagu start-all
```

## Remote Nodes

Configure authentication for remote Dagu instances:

```yaml
remoteNodes:
  - name: production
    apiBaseURL: https://prod.example.com/api/v2
    isBasicAuth: true
    basicAuthUsername: admin
    basicAuthPassword: prod-pass
    
  - name: staging
    apiBaseURL: https://staging.example.com/api/v2
    isAuthToken: true
    authToken: staging-token
```

## Common Patterns

### CI/CD Integration

```bash
# GitHub Actions
- name: Deploy workflow
  env:
    DAGU_API_TOKEN: ${{ secrets.DAGU_TOKEN }}
  run: |
    dagu push workflow.yaml
```

### Monitoring Access

Create read-only tokens:
```yaml
# Monitoring user - can view but not modify
permissions:
  writeDAGs: false
  runDAGs: false
```

### Multi-Environment Setup

```bash
# Development (no auth)
# Simply don't set auth environment variables

# Production (with auth and TLS)
export DAGU_AUTH_BASIC_USERNAME=admin
export DAGU_AUTH_BASIC_PASSWORD=secure-password
export DAGU_CERT_FILE=/etc/ssl/dagu.crt
export DAGU_KEY_FILE=/etc/ssl/dagu.key
```

## See Also

- [Server Configuration](/configurations/server) - Full server options
- [API Reference](/reference/api) - API authentication details
- [Remote Nodes](/configurations/advanced#remote-nodes) - Multi-instance setup
