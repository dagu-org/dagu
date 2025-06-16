# Authentication

Dagu provides multiple authentication methods to secure your workflow orchestration system.

## Basic Authentication

Enable username/password authentication:

```yaml
# ~/.config/dagu/config.yaml
auth:
  basic:
    enabled: true
    username: admin
    password: secure-password-123
```

### Using Basic Auth

```bash
# CLI with basic auth
export DAGU_USERNAME=admin
export DAGU_PASSWORD=secure-password-123
dagu status my-dag.yaml

# Or use curl
curl -u admin:secure-password-123 http://localhost:8080/api/v1/dags
```

## Token Authentication

Use API tokens for programmatic access:

```yaml
# ~/.config/dagu/config.yaml
auth:
  token:
    enabled: true
    value: your-secret-api-token-here
```

### Using API Tokens

```bash
# CLI with token
export DAGU_API_TOKEN=your-secret-api-token-here
dagu status my-dag.yaml

# HTTP header
curl -H "Authorization: Bearer your-secret-api-token-here" \
     http://localhost:8080/api/v1/dags

# Or use X-API-Token header
curl -H "X-API-Token: your-secret-api-token-here" \
     http://localhost:8080/api/v1/dags
```

## Multiple Authentication Methods

Enable both basic and token authentication:

```yaml
auth:
  basic:
    enabled: true
    username: admin
    password: admin-password
  token:
    enabled: true
    value: api-token-for-scripts
```

## TLS/HTTPS

Secure connections with TLS:

```yaml
# ~/.config/dagu/config.yaml
tls:
  certFile: /path/to/server.crt
  keyFile: /path/to/server.key
```

### Generate Self-Signed Certificate

```bash
# Generate private key
openssl genrsa -out server.key 2048

# Generate certificate
openssl req -new -x509 -sha256 -key server.key \
  -out server.crt -days 365 \
  -subj "/C=US/ST=State/L=City/O=Org/CN=localhost"
```

### Start with HTTPS

```bash
dagu start-all --cert=/path/to/server.crt --key=/path/to/server.key
```

## Permissions

Control what authenticated users can do:

```yaml
permissions:
  writeDAGs: true   # Create, edit, delete DAGs
  runDAGs: true     # Execute DAGs
```

### Permission Levels

- **Read-Only**: Set both to `false`
  - View DAGs and history
  - Cannot modify or execute

- **Operator**: `writeDAGs: false, runDAGs: true`
  - Execute existing DAGs
  - Cannot create or modify DAGs

- **Developer**: `writeDAGs: true, runDAGs: true`
  - Full access to all features

## Environment Variables

Configure authentication via environment:

```bash
# Basic auth
export DAGU_AUTH_BASIC_ENABLED=true
export DAGU_AUTH_BASIC_USERNAME=admin
export DAGU_AUTH_BASIC_PASSWORD=secret

# Token auth
export DAGU_AUTH_TOKEN_ENABLED=true
export DAGU_AUTH_TOKEN_VALUE=my-api-token

# Permissions
export DAGU_PERMISSIONS_WRITE_DAGS=true
export DAGU_PERMISSIONS_RUN_DAGS=true

# TLS
export DAGU_TLS_CERT_FILE=/path/to/cert
export DAGU_TLS_KEY_FILE=/path/to/key

dagu start-all
```

## Remote Node Authentication

Configure authentication for remote nodes:

```yaml
remoteNodes:
  - name: production
    apiBaseURL: https://prod.example.com/api/v1
    isBasicAuth: true
    basicAuthUsername: admin
    basicAuthPassword: prod-password
    
  - name: staging
    apiBaseURL: https://staging.example.com/api/v1
    isAuthToken: true
    authToken: staging-api-token
    skipTLSVerify: true  # For self-signed certs
```

## Security Best Practices

### 1. Strong Passwords

Use strong, unique passwords:

```yaml
auth:
  basic:
    username: admin
    password: "$(openssl rand -base64 32)"  # Generate random
```

### 2. Rotate Tokens

Regularly rotate API tokens:

```bash
# Generate new token
NEW_TOKEN=$(openssl rand -hex 32)
echo "New token: $NEW_TOKEN"

# Update config
sed -i "s/value: .*/value: $NEW_TOKEN/" ~/.config/dagu/config.yaml

# Restart Dagu
dagu restart
```

### 3. Use HTTPS in Production

Always use TLS for production:

```yaml
# Production config
host: 0.0.0.0
port: 443
tls:
  certFile: /etc/ssl/certs/dagu.crt
  keyFile: /etc/ssl/private/dagu.key
```

### 4. Limit Permissions

Follow principle of least privilege:

```yaml
# Read-only for monitoring
monitoring:
  auth:
    token:
      value: monitoring-token
  permissions:
    writeDAGs: false
    runDAGs: false

# Operator for scheduled runs
operator:
  auth:
    token:
      value: operator-token
  permissions:
    writeDAGs: false
    runDAGs: true
```

### 5. Network Security

Restrict network access:

```yaml
# Bind to localhost only
host: 127.0.0.1
port: 8080

# Use reverse proxy for external access
```

## Integration Examples

### Nginx Reverse Proxy

```nginx
server {
    listen 443 ssl;
    server_name dagu.example.com;
    
    ssl_certificate /etc/ssl/certs/dagu.crt;
    ssl_certificate_key /etc/ssl/private/dagu.key;
    
    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Pass auth headers
        proxy_set_header Authorization $http_authorization;
        proxy_pass_header Authorization;
    }
}
```

### Systemd Service

```ini
[Unit]
Description=Dagu Workflow Engine
After=network.target

[Service]
Type=simple
User=dagu
Group=dagu
Environment="DAGU_AUTH_BASIC_ENABLED=true"
Environment="DAGU_AUTH_BASIC_USERNAME=admin"
EnvironmentFile=/etc/dagu/auth.env
ExecStart=/usr/local/bin/dagu start-all
Restart=always

[Install]
WantedBy=multi-user.target
```

### Docker Compose

```yaml
version: '3.8'
services:
  dagu:
    image: ghcr.io/dagu-org/dagu:latest
    ports:
      - "8443:8443"
    environment:
      - DAGU_HOST=0.0.0.0
      - DAGU_PORT=8443
      - DAGU_AUTH_BASIC_ENABLED=true
      - DAGU_AUTH_BASIC_USERNAME=admin
      - DAGU_AUTH_BASIC_PASSWORD=${DAGU_PASSWORD}
      - DAGU_AUTH_TOKEN_ENABLED=true
      - DAGU_AUTH_TOKEN_VALUE=${DAGU_API_TOKEN}
    volumes:
      - ./dags:/app/dags
      - ./certs:/app/certs:ro
    command: >
      dagu start-all
      --cert=/app/certs/server.crt
      --key=/app/certs/server.key
```