# Operations

Production deployment and monitoring.

## Running as a Service

### systemd

Create `/etc/systemd/system/dagu.service`:

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

Create `/etc/dagu/environment`:
```bash
DAGU_HOST=0.0.0.0
DAGU_PORT=8080
DAGU_TZ=America/New_York
DAGU_LOG_FORMAT=json
```

Setup:
```bash
# Create user and directories
sudo useradd -r -s /bin/false dagu
sudo mkdir -p /opt/dagu/{dags,data,logs}
sudo chown -R dagu:dagu /opt/dagu

# Enable and start
sudo systemctl enable dagu
sudo systemctl start dagu

# Check status
sudo systemctl status dagu
sudo journalctl -u dagu -f
```

### Docker Compose

`compose.yml`:

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
      - dagu:/var/lib/dagu
      
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

```bash
# Start
docker compose up -d

# Logs
docker compose logs -f

# Stop
docker compose down
```

With authentication (`.env` file):
```bash
DAGU_AUTH_BASIC_USERNAME=admin
DAGU_AUTH_BASIC_PASSWORD=secure-password
```

### Prometheus Metrics

Metrics available at `/api/v2/metrics`:

**System:**
- `dagu_info` - Build information
- `dagu_uptime_seconds` - Uptime
- `dagu_scheduler_running` - Scheduler status

**DAGs:**
- `dagu_dags_total` - Total DAGs
- `dagu_dag_runs_currently_running` - Running DAGs
- `dagu_dag_runs_queued_total` - Queued DAGs
- `dagu_dag_runs_total` - DAG runs by status (24h)

**Standard:**
- Go runtime metrics
- Process metrics

### Logging

```yaml
# config.yaml
logFormat: json    # text or json
debug: true       # Debug mode
paths:
  logDir: /var/log/dagu
```

```bash
# Or via environment
export DAGU_LOG_FORMAT=json
export DAGU_DEBUG=true
export DAGU_LOG_DIR=/var/log/dagu
```

JSON log example:
```json
{
  "time": "2024-03-15T12:00:00Z",
  "level": "INFO",
  "msg": "DAG execution started",
  "dag": "data-pipeline",
  "run_id": "20240315_120000_abc123"
}
```

#### Log Cleanup

Automatic cleanup based on `histRetentionDays`:

```yaml
# Per-DAG
histRetentionDays: 7  # Keep 7 days

# Or global in base.yaml
histRetentionDays: 30  # Default
```

Special values:
- `0` - Delete after each run
- `-1` - Keep forever

Deletes:
- Execution logs
- Step output (.out, .err)
- Status files (.jsonl)
- Sub DAG logs

### Alerting

#### Email

```yaml
# base.yaml
smtp:
  host: "smtp.gmail.com"
  port: "587"
  username: "notifications@company.com"
  password: "${SMTP_PASSWORD}"

errorMail:
  from: "dagu@company.com"
  to: "ops-team@company.com"
  prefix: "[ERROR]"
  attachLogs: true

mailOn:
  failure: true
  success: false
```

Per-step notification:
```yaml
steps:
  - name: critical-task
    command: echo "Processing"
    mailOnError: true
```

#### Webhooks

**Slack:**
```yaml
handlerOn:
  failure:
    executor:
      type: http
      config:
        url: "${SLACK_WEBHOOK_URL}"
        method: POST
        body: |
          {
            "text": "Workflow Failed: ${DAG_NAME}",
            "blocks": [{
              "type": "section",
              "text": {
                "type": "mrkdwn",
                "text": "*Run ID:* ${DAG_RUN_ID}"
              }
            }]
          }
```

**PagerDuty:**
```yaml
handlerOn:
  failure:
    executor:
      type: http
      config:
        url: https://events.pagerduty.com/v2/enqueue
        body: |
          {
            "routing_key": "${PAGERDUTY_KEY}",
            "event_action": "trigger",
            "payload": {
              "summary": "Failed: ${DAG_NAME}",
              "severity": "error"
            }
          }
```

## Security

### Environment Variable Filtering

Dagu implements environment variable filtering to prevent accidental exposure of sensitive data to step processes and sub DAGs.

**How It Works:**

System environment variables are available for variable expansion (`${VAR}`) when parsing the DAG configuration, but only filtered variables are passed to the actual step execution environment and sub DAG processes.

**Filtered Variables (passed to step processes):**

Only these system environment variables are automatically passed to step processes and sub DAGs:

- **Whitelisted:** `PATH`, `HOME`, `LANG`, `TZ`, `SHELL`
- **Allowed Prefixes:** `DAGU_*`, `LC_*`, `DAG_*`

**Note:** Dagu automatically sets special variables with the `DAG_*` prefix for every step execution:
- `DAG_NAME`, `DAG_RUN_ID`, `DAG_RUN_STEP_NAME`
- `DAG_RUN_LOG_FILE`, `DAG_RUN_STEP_STDOUT_FILE`, `DAG_RUN_STEP_STDERR_FILE`

You can still use `${SYSTEM_VAR}` in your DAG YAML for variable expansion during configuration parsing, but the variable itself won't be in the step process environment unless it's whitelisted or explicitly defined in the `env` section.

**Using Sensitive Variables:**

The recommended approach is to use `.env` files to provide sensitive credentials to workflows:

```yaml
# workflow.yaml
dotenv: .env.secrets  # Load from .env file (not tracked in git)

steps:
  - name: deploy
    command: aws s3 sync ./build s3://my-bucket
```

```bash
# .env.secrets (add to .gitignore)
AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
DATABASE_PASSWORD=secure-password
```

Alternatively, if you need to pass through system environment variables, you must explicitly reference them:

```yaml
# workflow.yaml
env:
  - AWS_ACCESS_KEY_ID: ${AWS_ACCESS_KEY_ID}      # Explicit reference
  - AWS_SECRET_ACCESS_KEY: ${AWS_SECRET_ACCESS_KEY}

steps:
  - name: deploy
    command: aws s3 sync ./build s3://my-bucket
```

### Process Isolation

```bash
# systemd security features
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/dagu/data /opt/dagu/logs
```

## See Also

- [Server Configuration](/configurations/server) - Configure server settings
- [Deployment](/configurations/deployment) - Installation and deployment guides
- [Reference](/configurations/reference) - Complete configuration reference
- [Variables Reference](/reference/variables) - Environment variable usage
