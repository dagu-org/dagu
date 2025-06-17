# Operations

Production deployment and monitoring guide for Dagu.

## Running as a Service

### systemd (Linux)

1. **Create service file** `/etc/systemd/system/dagu.service`:

```ini
[Unit]
Description=Dagu Workflow Engine
Documentation=https://dagu.cloud/
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=dagu
Group=dagu
WorkingDirectory=/opt/dagu

# Main process
ExecStart=/usr/local/bin/dagu start-all

# Graceful shutdown
ExecStop=/bin/kill -TERM $MAINPID
TimeoutStopSec=30
KillMode=mixed
KillSignal=SIGTERM

# Restart policy
Restart=always
RestartSec=10
StartLimitInterval=60
StartLimitBurst=3

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/dagu/data /opt/dagu/logs

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

# Environment
EnvironmentFile=-/etc/dagu/environment
Environment="DAGU_HOME=/opt/dagu"

[Install]
WantedBy=multi-user.target
```

2. **Create environment file** `/etc/dagu/environment`:
```bash
# Server settings
DAGU_HOST=0.0.0.0
DAGU_PORT=8080

# Timezone
DAGU_TZ=America/New_York

# Logging
DAGU_LOG_FORMAT=json

# Authentication (optional)
# DAGU_AUTH_BASIC_USERNAME=admin
# DAGU_AUTH_BASIC_PASSWORD=secure-password
# DAGU_AUTH_TOKEN=your-api-token
```

3. **Set up directories and permissions**:
```bash
# Create user and directories
sudo useradd -r -s /bin/false dagu
sudo mkdir -p /opt/dagu/{dags,data,logs}
sudo chown -R dagu:dagu /opt/dagu

# Copy DAG files
sudo cp your-dags/*.yaml /opt/dagu/dags/
sudo chown -R dagu:dagu /opt/dagu/dags/
```

4. **Enable and start service**:
```bash
# Enable auto-start
sudo systemctl enable dagu

# Start service
sudo systemctl start dagu

# Check status
sudo systemctl status dagu

# View logs
sudo journalctl -u dagu -f
```

### Docker Compose

1. **Create `docker-compose.yml`**:

```yaml
version: '3.8'

services:
  dagu:
    image: ghcr.io/dagu-org/dagu:latest
    container_name: dagu
    restart: unless-stopped
    
    # Health check
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/api/v2/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    
    # Port mapping
    ports:
      - "8080:8080"
    
    # Environment variables
    environment:
      # Server configuration
      - DAGU_PORT=8080
      - DAGU_HOST=0.0.0.0
      - DAGU_TZ=America/New_York
      
      # Logging
      - DAGU_LOG_FORMAT=json
      
      # Authentication (optional)
      # - DAGU_AUTH_BASIC_USERNAME=admin
      # - DAGU_AUTH_BASIC_PASSWORD=your-secure-password
      
      # User/Group IDs (optional)
      # - PUID=1000
      # - PGID=1000
      
      # Docker-in-Docker support (optional)
      # - DOCKER_GID=999
    
    # Volume mounts
    volumes:
      # DAG definitions (read-only recommended)
      - ./dags:/config/dags:ro
      
      # Persistent data
      - dagu-data:/config/data
      - dagu-logs:/config/logs
      
      # Configuration files (optional)
      # - ./config.yaml:/config/config.yaml:ro
      # - ./base.yaml:/config/base.yaml:ro
      
      # Docker socket for Docker executor (optional)
      # - /var/run/docker.sock:/var/run/docker.sock
    
    # Logging configuration
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "5"

volumes:
  dagu-data:
  dagu-logs:
```

2. **Start the service**:
```bash
# Start in background
docker-compose up -d

# View logs
docker-compose logs -f

# Stop service
docker-compose down
```

3. **With authentication** (create `.env` file):
```bash
DAGU_AUTH_BASIC_USERNAME=admin
DAGU_AUTH_BASIC_PASSWORD=your-secure-password
```

### Kubernetes

1. **Create namespace and secrets**:
```bash
kubectl create namespace dagu

# Create authentication secret (optional)
kubectl create secret generic dagu-auth \
  --from-literal=username=admin \
  --from-literal=password=your-secure-password \
  -n dagu
```

2. **Deploy Dagu** (`dagu-deployment.yaml`):
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dagu
  namespace: dagu
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
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        fsGroup: 1000
      containers:
      - name: dagu
        image: ghcr.io/dagu-org/dagu:latest
        imagePullPolicy: Always
        ports:
        - containerPort: 8080
          name: http
        env:
        - name: DAGU_HOST
          value: "0.0.0.0"
        - name: DAGU_PORT
          value: "8080"
        - name: DAGU_TZ
          value: "UTC"
        - name: DAGU_LOG_FORMAT
          value: "json"
        # Authentication (optional)
        # - name: DAGU_AUTH_BASIC_USERNAME
        #   valueFrom:
        #     secretKeyRef:
        #       name: dagu-auth
        #       key: username
        # - name: DAGU_AUTH_BASIC_PASSWORD
        #   valueFrom:
        #     secretKeyRef:
        #       name: dagu-auth
        #       key: password
        volumeMounts:
        - name: dags
          mountPath: /config/dags
        - name: data
          mountPath: /config/data
        - name: logs
          mountPath: /config/logs
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
        livenessProbe:
          httpGet:
            path: /api/v2/health
            port: http
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /api/v2/health
            port: http
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: dags
        configMap:
          name: dagu-workflows
      - name: data
        persistentVolumeClaim:
          claimName: dagu-data
      - name: logs
        emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: dagu
  namespace: dagu
spec:
  selector:
    app: dagu
  ports:
  - port: 8080
    targetPort: http
    name: http
  type: ClusterIP
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: dagu-data
  namespace: dagu
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
```

3. **Deploy DAG ConfigMap** (`dagu-workflows.yaml`):
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: dagu-workflows
  namespace: dagu
data:
  hello.yaml: |
    steps:
      - name: hello
        command: echo "Hello from Kubernetes!"
```

4. **Apply manifests**:
```bash
kubectl apply -f dagu-deployment.yaml
kubectl apply -f dagu-workflows.yaml

# Check status
kubectl get pods -n dagu
kubectl logs -f deployment/dagu -n dagu
```

## Monitoring

### Health Check Endpoints

Dagu provides a health check endpoint:

```bash
curl http://localhost:8080/api/v2/health
```

Response:
```json
{
  "status": "healthy",
  "version": "x.y.z",
  "uptime": 3600,
  "now": "2024-03-15T12:00:00Z"
}
```

**Health check script**:
```bash
#!/bin/bash
HEALTH_URL="http://localhost:8080/api/v2/health"

if curl -f -s "$HEALTH_URL" | grep -q '"status":"healthy"'; then
    echo "Dagu is healthy"
    exit 0
else
    echo "Dagu is unhealthy"
    exit 1
fi
```

### Prometheus Metrics

Dagu exposes Prometheus-compatible metrics at `/api/v2/metrics`:

**Available Metrics**:

1. **System Metrics**:
   - `dagu_info` - Build information (version, build date, Go version)
   - `dagu_uptime_seconds` - Time since server start
   - `dagu_scheduler_running` - Whether scheduler is running (0 or 1)

2. **DAG Metrics**:
   - `dagu_dags_total` - Total number of DAGs

3. **DAG Run Metrics**:
   - `dagu_dag_runs_currently_running` - Number of currently running DAG runs
   - `dagu_dag_runs_queued_total` - Total number of DAG runs in queue
   - `dagu_dag_runs_total` - Total DAG runs by status (last 24 hours)
     - Labels: `status` (success, error, partial_success, cancelled, running, queued, none)

4. **Standard Go Metrics**:
   - Go runtime metrics (memory, GC, goroutines)
   - Process metrics (CPU, memory, file descriptors)

**Prometheus Configuration**:
```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'dagu'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/api/v2/metrics'
    scrape_interval: 30s
```

**Example Queries**:
```promql
# Running DAGs
dagu_dag_runs_currently_running

# Success rate (last 24h)
rate(dagu_dag_runs_total{status="success"}[5m])

# Queue depth
dagu_dag_runs_queued_total

# Uptime
dagu_uptime_seconds
```

**Grafana Dashboard Example**:
```json
{
  "panels": [
    {
      "title": "DAG Runs by Status",
      "targets": [{
        "expr": "dagu_dag_runs_total"
      }]
    },
    {
      "title": "Currently Running",
      "targets": [{
        "expr": "dagu_dag_runs_currently_running"
      }]
    }
  ]
}
```

### Logging

#### Log Configuration

Dagu uses structured logging with support for text and JSON formats:

**Configuration options**:
```yaml
# config.yaml
logFormat: json    # Options: text, json
debug: true       # Enable debug logging with source locations
logDir: /var/log/dagu  # Custom log directory
```

**Environment variables**:
```bash
export DAGU_LOG_FORMAT=json
export DAGU_DEBUG=true
export DAGU_LOG_DIR=/var/log/dagu
```

**Log structure**:
- DAG execution logs: `{logDir}/{dag-name}/dag-run_{timestamp}_{run-id}/`
  - Main log file: `dag-run_{timestamp}.{run-id-prefix}.log`
  - Step outputs: `{step-name}.out` and `{step-name}.err`
- Server/scheduler logs: Output to stdout/stderr (not file by default)

#### JSON Log Format

Example JSON log entry:
```json
{
  "time": "2024-03-15T12:00:00Z",
  "level": "INFO",
  "msg": "DAG execution started",
  "dag": "data-pipeline",
  "run_id": "20240315_120000_abc123",
  "step": "extract-data"
}
```

#### Log Aggregation

**Using Filebeat (Elastic):**
```yaml
# filebeat.yml
filebeat.inputs:
- type: log
  enabled: true
  paths:
    - /opt/dagu/logs/**/*.log
  json.keys_under_root: true
  json.add_error_key: true
  fields:
    service: dagu
  fields_under_root: true

output.elasticsearch:
  hosts: ["elasticsearch:9200"]
  index: "dagu-%{+yyyy.MM.dd}"
```

**Using Promtail (Loki):**
```yaml
# promtail.yml
clients:
  - url: http://loki:3100/loki/api/v1/push

scrape_configs:
  - job_name: dagu
    static_configs:
      - targets:
          - localhost
        labels:
          job: dagu
          __path__: /opt/dagu/logs/**/*.log
    pipeline_stages:
      - json:
          expressions:
            level: level
            dag: dag
            run_id: run_id
      - labels:
          level:
          dag:
```

#### Log Rotation

Configure logrotate (`/etc/logrotate.d/dagu`):
```
/opt/dagu/logs/**/*.log {
    daily
    rotate 14
    compress
    delaycompress
    missingok
    notifempty
    create 0644 dagu dagu
    size 100M
    copytruncate
}
```

**Note**: Dagu doesn't require USR1 signal for log rotation when using `copytruncate`.

#### Automatic Log Cleanup

Dagu automatically removes old execution logs based on the `histRetentionDays` setting:

**How it works**:
- Cleanup runs automatically before each DAG execution
- Removes both execution data and log files older than retention days
- Default retention: 30 days

**Configuration**:
```yaml
# Per-DAG configuration
name: my-workflow
histRetentionDays: 7  # Keep only 7 days of logs

# Or in base.yaml for all DAGs
histRetentionDays: 14  # Global 14-day retention
```

**Special values**:
- `0`: Delete all historical data after each run
- `-1`: Keep logs forever (no cleanup)
- Default: `30` days

**What gets deleted**:
- Main DAG execution logs
- Step output files (.out and .err)
- Status files (.jsonl)
- Child DAG logs (for nested workflows)
- Empty parent directories

::: warning Important
The cleanup process deletes **both data files and log files**. If you need to preserve logs for compliance, either:
- Set a longer retention period
- Use external log aggregation (Filebeat, Promtail)
- Archive logs before retention expires
:::

### Alerting

#### Email Notifications

Configure SMTP settings in `base.yaml` (applies to all DAGs) or per-DAG:

```yaml
# base.yaml - Global email configuration
smtp:
  host: "smtp.gmail.com"
  port: "587"
  username: "notifications@company.com"
  password: "${SMTP_PASSWORD}"  # Use environment variable

errorMail:
  from: "dagu@company.com"
  to: "ops-team@company.com"
  prefix: "[DAGU ERROR]"
  attachLogs: true  # Attach step logs to email

# Enable notifications
mailOn:
  failure: true
  success: false
```

**Per-DAG configuration**:
```yaml
name: critical-workflow
mailOn:
  failure: true
  success: true
steps:
  - name: important-task
    command: ./process.sh
    mailOnError: true  # Step-specific notification
```

**Email timeout**: 30 seconds (hardcoded)

#### Webhook Notifications

Use HTTP executor in lifecycle handlers:

**Slack notification**:
```yaml
handlerOn:
  failure:
    executor:
      type: http
      config:
        url: "${SLACK_WEBHOOK_URL}"
        method: POST
        headers:
          Content-Type: application/json
        body: |
          {
            "text": "🚨 Workflow Failed",
            "blocks": [{
              "type": "section",
              "text": {
                "type": "mrkdwn",
                "text": "*Workflow:* ${DAG_NAME}\n*Run ID:* ${DAG_RUN_ID}\n*Time:* `date`"
              }
            }]
          }
```

**PagerDuty integration**:
```yaml
handlerOn:
  failure:
    executor:
      type: http
      config:
        url: https://events.pagerduty.com/v2/enqueue
        method: POST
        headers:
          Content-Type: application/json
        body: |
          {
            "routing_key": "${PAGERDUTY_ROUTING_KEY}",
            "event_action": "trigger",
            "payload": {
              "summary": "Dagu workflow failed: ${DAG_NAME}",
              "severity": "error",
              "source": "dagu",
              "custom_details": {
                "run_id": "${DAG_RUN_ID}",
                "log_file": "${DAG_RUN_LOG_FILE}"
              }
            }
          }
```

#### Prometheus Alerting

Example alert rules:
```yaml
# prometheus-alerts.yml
groups:
  - name: dagu
    rules:
      - alert: DaguHighFailureRate
        expr: |
          rate(dagu_dag_runs_total{status="error"}[5m]) > 0.1
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "High DAG failure rate"
          description: "Failure rate is {{ $value }} per second"
      
      - alert: DaguQueueBacklog
        expr: dagu_dag_runs_queued_total > 50
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Large queue backlog"
          description: "{{ $value }} DAGs queued"
      
      - alert: DaguDown
        expr: up{job="dagu"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Dagu is down"
```

## See Also

- [Server Configuration](/configurations/server) - Configure server settings
- [Advanced Setup](/configurations/advanced) - High availability and scaling
- [Reference](/configurations/reference) - Complete configuration reference
