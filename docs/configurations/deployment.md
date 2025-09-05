# Deployment

This guide covers deploying Dagu in production environments.

## Deployment Methods

### Systemd Service

Create a systemd service for automatic startup:

```ini
# /etc/systemd/system/dagu.service
[Unit]
Description=Dagu Workflow Engine
After=network.target
Documentation=https://docs.dagu.cloud

[Service]
Type=simple
User=dagu
Group=dagu
WorkingDirectory=/opt/dagu
ExecStart=/usr/local/bin/dagu start-all
Restart=always
RestartSec=10

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/dagu /var/log/dagu

# Environment
Environment="DAGU_HOST=0.0.0.0"
Environment="DAGU_PORT=8080"
EnvironmentFile=-/etc/dagu/env

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=dagu

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable dagu
sudo systemctl start dagu
sudo systemctl status dagu
```

### Docker

Run with Docker:

```bash
docker run -d \
  --name dagu \
  -p 8080:8080 \
  -v $HOME/.dagu:/var/lib/dagu \
  -e DAGU_HOST=0.0.0.0 \
  -e DAGU_PORT=8080 \
  --restart always \
  ghcr.io/dagu-org/dagu:latest
```

### Docker Compose

Basic deployment using the official Docker image:

```yaml
# compose.yml
services:
  dagu:
    image: "ghcr.io/dagu-org/dagu:latest"
    container_name: dagu
    hostname: dagu
    restart: always
    ports:
      - "8080:8080"
    environment:
      - DAGU_PORT=8080
      - DAGU_TZ=Asia/Tokyo  # Set your timezone
      - DAGU_BASE_PATH=/    # Change if using reverse proxy
      - PUID=1000           # User ID for file permissions
      - PGID=1000           # Group ID for file permissions
    volumes:
      - dagu:/var/lib/dagu
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/api/v2/health"]
      interval: 30s
      timeout: 10s
      retries: 3
volumes:
  dagu: {}
```

For custom configuration with separate volumes:

```yaml
# compose.yml with custom paths
services:
  dagu:
    image: "ghcr.io/dagu-org/dagu:latest"
    container_name: dagu
    restart: always
    ports:
      - "8080:8080"
    volumes:
      - ./dags:/var/lib/dagu/dags
      - ./logs:/var/lib/dagu/logs
      - ./data:/var/lib/dagu/data
      - ./config.yaml:/var/lib/dagu/config.yaml:ro
    environment:
      - DAGU_HOST=0.0.0.0
      - DAGU_PORT=8080
      - DAGU_CONFIG=/dagu/config.yaml
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/api/v2/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

For Docker-in-Docker support (to run Docker executors):

```yaml
# compose.yml with Docker-in-Docker
services:
  dagu:
    image: "ghcr.io/dagu-org/dagu:latest"
    container_name: dagu
    hostname: dagu
    restart: always
    ports:
      - "8080:8080"
    environment:
      - DAGU_PORT=8080
      - DAGU_TZ=Asia/Tokyo
      - DAGU_BASE_PATH=/
    volumes:
      - dagu:/var/lib/dagu
      - /var/run/docker.sock:/var/run/docker.sock
    entrypoint: [] # Override default entrypoint
    user: "0:0"    # Run as root for Docker access
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/api/v2/health"]
      interval: 30s
      timeout: 10s
      retries: 3
volumes:
  dagu: {}
```

## Monitoring

### Health Checks

```yaml
# Kubernetes readiness probe
readinessProbe:
  httpGet:
    path: /api/v2/health
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 5

# Docker healthcheck
healthcheck:
  test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:8080/api/v2/health"]
  interval: 30s
  timeout: 10s
  retries: 3
```

### Prometheus Metrics

You can use Dagu's built-in metrics endpoint to monitor performance and health. Enable metrics in your configuration: `http://localhost:8080/api/v2/metrics`.
