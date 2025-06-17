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
Documentation=https://dagu.readthedocs.io

[Service]
Type=simple
User=dagu
Group=dagu
WorkingDirectory=/opt/dagu
ExecStart=/usr/local/bin/dagu start-all --config /etc/dagu/config.yaml
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
  -v $HOME/.config/dagu:/config \
  -e DAGU_HOST=0.0.0.0 \
  -e DAGU_PORT=8080 \
  --restart always \
  ghcr.io/dagu-org/dagu:latest
```

### Docker Compose

Basic deployment using the official Docker image:

```yaml
# docker-compose.yml
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
      - dagu_config:/config
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/api/v1/status"]
      interval: 30s
      timeout: 10s
      retries: 3
volumes:
  dagu_config: {}
```

For custom configuration with separate volumes:

```yaml
# docker-compose.yml with custom paths
services:
  dagu:
    image: "ghcr.io/dagu-org/dagu:latest"
    container_name: dagu
    restart: always
    ports:
      - "8080:8080"
    volumes:
      - ./dags:/config/dags
      - ./logs:/config/logs
      - ./data:/config/data
      - ./config.yaml:/config/config.yaml:ro
    environment:
      - DAGU_HOST=0.0.0.0
      - DAGU_PORT=8080
      - DAGU_CONFIG=/config/config.yaml
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/api/v1/status"]
      interval: 30s
      timeout: 10s
      retries: 3
```

For Docker-in-Docker support (to run Docker executors):

```yaml
# docker-compose.yml with Docker-in-Docker
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
      - dagu_config:/config
      - /var/run/docker.sock:/var/run/docker.sock
    user: "0:0"
    entrypoint: []
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/api/v1/status"]
      interval: 30s
      timeout: 10s
      retries: 3
volumes:
  dagu_config: {}
```

### Kubernetes

Deploy on Kubernetes:

```yaml
# dagu-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dagu
  labels:
    app: dagu
spec:
  replicas: 1
  selector:
    matchLabels:
      app: dagu
  template:
    metadata:
      labels:
        app: dagu
    spec:
      containers:
      - name: dagu
        image: ghcr.io/dagu-org/dagu:latest
        ports:
        - containerPort: 8080
        env:
        - name: DAGU_HOST
          value: "0.0.0.0"
        - name: DAGU_PORT
          value: "8080"
        volumeMounts:
        - name: dags
          mountPath: /app/dags
        - name: logs
          mountPath: /app/logs
        - name: config
          mountPath: /app/config.yaml
          subPath: config.yaml
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
        livenessProbe:
          httpGet:
            path: /api/v1/status
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 30
      volumes:
      - name: dags
        persistentVolumeClaim:
          claimName: dagu-dags-pvc
      - name: logs
        persistentVolumeClaim:
          claimName: dagu-logs-pvc
      - name: config
        configMap:
          name: dagu-config
---
apiVersion: v1
kind: Service
metadata:
  name: dagu
spec:
  selector:
    app: dagu
  ports:
  - port: 8080
    targetPort: 8080
  type: LoadBalancer
```

## Reverse Proxy

### Nginx

```nginx
# /etc/nginx/sites-available/dagu
server {
    listen 80;
    server_name dagu.example.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name dagu.example.com;
    
    ssl_certificate /etc/letsencrypt/live/dagu.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/dagu.example.com/privkey.pem;
    
    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        
        # Timeouts
        proxy_read_timeout 86400;
    }
}
```

### Apache

```apache
# /etc/apache2/sites-available/dagu.conf
<VirtualHost *:80>
    ServerName dagu.example.com
    Redirect permanent / https://dagu.example.com/
</VirtualHost>

<VirtualHost *:443>
    ServerName dagu.example.com
    
    SSLEngine on
    SSLCertificateFile /etc/letsencrypt/live/dagu.example.com/fullchain.pem
    SSLCertificateKeyFile /etc/letsencrypt/live/dagu.example.com/privkey.pem
    
    ProxyPreserveHost On
    ProxyPass / http://localhost:8080/
    ProxyPassReverse / http://localhost:8080/
    
    # WebSocket
    RewriteEngine on
    RewriteCond %{HTTP:Upgrade} websocket [NC]
    RewriteCond %{HTTP:Connection} upgrade [NC]
    RewriteRule ^/?(.*) "ws://localhost:8080/$1" [P,L]
</VirtualHost>
```

### Traefik

```yaml
# docker-compose.yml with Traefik
version: '3.8'

services:
  traefik:
    image: traefik:v2.10
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./traefik.yml:/traefik.yml:ro
      - ./acme.json:/acme.json
    labels:
      - "traefik.enable=true"

  dagu:
    image: ghcr.io/dagu-org/dagu:latest
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.dagu.rule=Host(`dagu.example.com`)"
      - "traefik.http.routers.dagu.entrypoints=websecure"
      - "traefik.http.routers.dagu.tls.certresolver=letsencrypt"
      - "traefik.http.services.dagu.loadbalancer.server.port=8080"
```

## High Availability

### Multi-Node Setup

Configure multiple Dagu instances:

```yaml
# Primary node
host: 0.0.0.0
port: 8080
paths:
  dagsDir: /shared/dags  # Shared storage
  dataDir: /shared/data
  logDir: /shared/logs

# Secondary node (read-only)
host: 0.0.0.0
port: 8081
permissions:
  writeDAGs: false
  runDAGs: false
paths:
  dagsDir: /shared/dags
  dataDir: /shared/data
  logDir: /shared/logs
```

### Load Balancer

HAProxy configuration:

```
# /etc/haproxy/haproxy.cfg
global
    daemon

defaults
    mode http
    timeout connect 5000ms
    timeout client 50000ms
    timeout server 50000ms

frontend dagu_frontend
    bind *:80
    default_backend dagu_backend

backend dagu_backend
    balance roundrobin
    option httpchk GET /api/v1/status
    server dagu1 192.168.1.10:8080 check
    server dagu2 192.168.1.11:8080 check backup
```

## Monitoring

### Health Checks

```yaml
# Kubernetes readiness probe
readinessProbe:
  httpGet:
    path: /api/v1/status
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 5

# Docker healthcheck
healthcheck:
  test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:8080/api/v1/status"]
  interval: 30s
  timeout: 10s
  retries: 3
```

### Prometheus Metrics

Export metrics via API:

```yaml
steps:
  - name: export metrics
    command: |
      curl -s http://localhost:8080/api/v1/dags | \
      jq -r '.[] | "\(.name) \(.status)"' | \
      awk '{print "dagu_dag_status{name=\""$1"\",status=\""$2"\"} 1"}' > \
      /var/lib/prometheus/node_exporter/dagu.prom
    schedule: "* * * * *"
```

### Logging

Configure centralized logging:

```yaml
# Fluentd configuration
<source>
  @type tail
  path /var/log/dagu/*.log
  pos_file /var/log/fluentd/dagu.pos
  tag dagu.*
  <parse>
    @type json
  </parse>
</source>

<match dagu.**>
  @type elasticsearch
  host elasticsearch
  port 9200
  logstash_format true
  logstash_prefix dagu
</match>
```

## Security Hardening

### Network Security

```yaml
# Bind to localhost only
host: 127.0.0.1
port: 8080

# Use reverse proxy for external access
```

### File Permissions

```bash
# Create dedicated user
sudo useradd -r -s /bin/false dagu

# Set ownership
sudo chown -R dagu:dagu /opt/dagu
sudo chmod 750 /opt/dagu

# Secure configuration
sudo chmod 600 /etc/dagu/config.yaml
sudo chown dagu:dagu /etc/dagu/config.yaml
```

### Resource Limits

```yaml
# systemd resource limits
[Service]
# Memory
MemoryMax=2G
MemoryHigh=1G

# CPU
CPUQuota=100%

# File descriptors
LimitNOFILE=65536

# Process limits
LimitNPROC=4096
```

## Backup & Recovery

### Backup Script

```bash
#!/bin/bash
# /opt/dagu/backup.sh

BACKUP_DIR="/backup/dagu"
DATE=$(date +%Y%m%d_%H%M%S)

# Create backup
tar -czf "$BACKUP_DIR/dagu_backup_$DATE.tar.gz" \
  /opt/dagu/dags \
  /var/lib/dagu/data \
  /var/log/dagu \
  /etc/dagu/config.yaml

# Keep last 30 days
find "$BACKUP_DIR" -name "dagu_backup_*.tar.gz" -mtime +30 -delete
```

### Restore Procedure

```bash
# Stop Dagu
sudo systemctl stop dagu

# Restore from backup
tar -xzf /backup/dagu/dagu_backup_20240115_120000.tar.gz -C /

# Start Dagu
sudo systemctl start dagu
```

## Performance Tuning

### System Limits

```bash
# /etc/security/limits.d/dagu.conf
dagu soft nofile 65536
dagu hard nofile 65536
dagu soft nproc 32768
dagu hard nproc 32768
```

### Kernel Parameters

```bash
# /etc/sysctl.d/dagu.conf
# Increase file watchers
fs.inotify.max_user_watches=524288

# Network tuning
net.core.somaxconn=65535
net.ipv4.tcp_max_syn_backlog=65535
```

## Troubleshooting

### Common Issues

1. **Port Already in Use**
   ```bash
   # Find process using port
   sudo lsof -i :8080
   
   # Kill process
   sudo kill -9 <PID>
   ```

2. **Permission Denied**
   ```bash
   # Fix permissions
   sudo chown -R dagu:dagu /opt/dagu
   sudo chmod -R 755 /opt/dagu/dags
   ```

3. **Memory Issues**
   ```bash
   # Check memory usage
   ps aux | grep dagu
   
   # Increase limits
   sudo systemctl edit dagu
   # Add: MemoryMax=4G
   ```

### Debug Mode

Enable debug logging:

```yaml
# config.yaml
debug: true
logFormat: json
```

Or via command line:

```bash
dagu start-all --debug --log-format=json
```