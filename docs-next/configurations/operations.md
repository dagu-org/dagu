# Operations

Production deployment, monitoring, and maintenance guide for Dagu.

## Running as a Service

### systemd (Linux)

Create `/etc/systemd/system/dagu.service`:

```ini
[Unit]
Description=Dagu Workflow Engine
Documentation=https://dagu.readthedocs.io/
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
ReadWritePaths=/opt/dagu
ReadOnlyPaths=/etc/dagu

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

# Environment
EnvironmentFile=/etc/dagu/environment
Environment="DAGU_CONFIG=/etc/dagu/config.yaml"

[Install]
WantedBy=multi-user.target
```

Environment file `/etc/dagu/environment`:
```bash
# Server settings
DAGU_HOST=0.0.0.0
DAGU_PORT=8080

# Paths
DAGU_HOME=/opt/dagu

# Authentication
DAGU_AUTH_BASIC_USERNAME=admin
DAGU_AUTH_BASIC_PASSWORD=change-this-password

# Performance
GOMAXPROCS=4
```

Manage the service:
```bash
# Enable auto-start
sudo systemctl enable dagu

# Start service
sudo systemctl start dagu

# Check status
sudo systemctl status dagu

# View logs
sudo journalctl -u dagu -f

# Restart
sudo systemctl restart dagu

# Stop
sudo systemctl stop dagu
```

### Docker Compose (Production)

```yaml
# docker-compose.yml
version: '3.8'

services:
  dagu:
    image: ghcr.io/dagu-org/dagu:latest
    container_name: dagu
    restart: unless-stopped
    
    # Resource limits
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '0.5'
          memory: 512M
    
    # Health check
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:8080/api/v1/dags"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    
    # Networking
    ports:
      - "8080:8080"
    networks:
      - dagu-network
    
    # Environment
    environment:
      - DAGU_TZ=America/New_York
      - DAGU_AUTH_BASIC_USERNAME=admin
      - DAGU_AUTH_BASIC_PASSWORD_FILE=/run/secrets/admin_password
      - DAGU_LOG_LEVEL=info
    
    # Volumes
    volumes:
      - ./dags:/home/dagu/.config/dagu/dags:ro
      - dagu-logs:/home/dagu/.local/share/dagu/logs
      - dagu-data:/home/dagu/.local/share/dagu/data
      - ./config/config.yaml:/home/dagu/.config/dagu/config.yaml:ro
    
    # Secrets
    secrets:
      - admin_password
    
    # Logging
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "5"
    
    command: dagu start-all

networks:
  dagu-network:
    driver: bridge

volumes:
  dagu-logs:
  dagu-data:

secrets:
  admin_password:
    file: ./secrets/admin_password.txt
```

### Kubernetes

Deployment manifest:

```yaml
# dagu-deployment.yaml
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
      serviceAccountName: dagu
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
        env:
        - name: DAGU_HOST
          value: "0.0.0.0"
        - name: DAGU_PORT
          value: "8080"
        - name: DAGU_AUTH_BASIC_USERNAME
          valueFrom:
            secretKeyRef:
              name: dagu-auth
              key: username
        - name: DAGU_AUTH_BASIC_PASSWORD
          valueFrom:
            secretKeyRef:
              name: dagu-auth
              key: password
        volumeMounts:
        - name: dags
          mountPath: /home/dagu/.config/dagu/dags
        - name: data
          mountPath: /home/dagu/.local/share/dagu
        - name: config
          mountPath: /home/dagu/.config/dagu/config.yaml
          subPath: config.yaml
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "2000m"
        livenessProbe:
          httpGet:
            path: /api/v1/dags
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /api/v1/dags
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: dags
        configMap:
          name: dagu-workflows
      - name: data
        persistentVolumeClaim:
          claimName: dagu-data
      - name: config
        configMap:
          name: dagu-config
```

## Monitoring

### Health Checks

Built-in health endpoint:
```bash
# Check if service is healthy
curl http://localhost:8080/api/v1/dags

# Simple health check script
#!/bin/bash
if curl -f -s http://localhost:8080/api/v1/dags > /dev/null; then
    echo "Dagu is healthy"
    exit 0
else
    echo "Dagu is unhealthy"
    exit 1
fi
```

### Prometheus Metrics

While Dagu doesn't have built-in Prometheus metrics, you can monitor:

1. **Process metrics** using node_exporter
2. **Log metrics** using promtail/loki
3. **API metrics** using nginx/prometheus exporter

Example prometheus config:
```yaml
scrape_configs:
  - job_name: 'dagu-logs'
    static_configs:
      - targets: ['loki:3100']
    
  - job_name: 'dagu-process'
    static_configs:
      - targets: ['node-exporter:9100']
```

### Logging

#### Log Configuration

Configure log output format:
```yaml
# config.yaml
logFormat: "json"  # or "text"
debug: false       # Enable debug logging
```

#### Log Aggregation

Using fluentd:
```yaml
# fluent.conf
<source>
  @type tail
  path /opt/dagu/logs/**/*.log
  pos_file /var/log/td-agent/dagu.pos
  tag dagu.*
  <parse>
    @type json
  </parse>
</source>

<match dagu.**>
  @type elasticsearch
  host elasticsearch
  port 9200
  index_name dagu
  type_name _doc
</match>
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
    sharedscripts
    postrotate
        # Signal Dagu to reopen log files if needed
        pkill -USR1 dagu || true
    endscript
}
```

### Alerting

#### Email Alerts

Configure email notifications for failures:
```yaml
# base.yaml - Applied to all DAGs
mailOn:
  failure: true
  success: false

smtp:
  host: smtp.gmail.com
  port: "587"
  username: alerts@company.com
  password: ${SMTP_PASSWORD}

errorMail:
  from: dagu@company.com
  to: ops-team@company.com
  prefix: "[DAGU ALERT]"
  attachLogs: true
```

#### Webhook Alerts

Use lifecycle handlers for custom alerts:
```yaml
handlerOn:
  failure:
    executor:
      type: http
      config:
        url: https://hooks.slack.com/services/YOUR/WEBHOOK
        method: POST
        headers:
          Content-Type: application/json
        body: |
          {
            "text": "Workflow failed: ${DAG_NAME}",
            "attachments": [{
              "color": "danger",
              "fields": [
                {"title": "Workflow", "value": "${DAG_NAME}", "short": true},
                {"title": "Run ID", "value": "${DAG_RUN_ID}", "short": true},
                {"title": "Time", "value": "`date`", "short": false}
              ]
            }]
          }
```

## Backup and Recovery

### Backup Strategy

What to backup:
1. **DAG definitions** (`~/.config/dagu/dags/`)
2. **Configuration** (`~/.config/dagu/*.yaml`)
3. **Historical data** (`~/.local/share/dagu/data/`)
4. **Recent logs** (`~/.local/share/dagu/logs/`)

Backup script:
```bash
#!/bin/bash
# /opt/dagu/scripts/backup.sh

set -euo pipefail

BACKUP_ROOT="/backup/dagu"
BACKUP_DIR="${BACKUP_ROOT}/$(date +%Y%m%d_%H%M%S)"
RETENTION_DAYS=30

# Create backup directory
mkdir -p "$BACKUP_DIR"

# Backup configurations
echo "Backing up configurations..."
cp -r /opt/dagu/dags "$BACKUP_DIR/"
cp -r /etc/dagu "$BACKUP_DIR/"

# Backup data
echo "Backing up data..."
cp -r /opt/dagu/data "$BACKUP_DIR/"

# Backup recent logs (last 7 days)
echo "Backing up recent logs..."
find /opt/dagu/logs -type f -mtime -7 -exec cp --parents {} "$BACKUP_DIR/" \;

# Create archive
echo "Creating archive..."
tar -czf "${BACKUP_DIR}.tar.gz" -C "$BACKUP_ROOT" "$(basename "$BACKUP_DIR")"
rm -rf "$BACKUP_DIR"

# Upload to S3 (optional)
if command -v aws &> /dev/null; then
    echo "Uploading to S3..."
    aws s3 cp "${BACKUP_DIR}.tar.gz" s3://backup-bucket/dagu/
fi

# Clean old backups
echo "Cleaning old backups..."
find "$BACKUP_ROOT" -name "*.tar.gz" -mtime +$RETENTION_DAYS -delete

echo "Backup completed: ${BACKUP_DIR}.tar.gz"
```

Schedule backup:
```bash
# Add to crontab
0 2 * * * /opt/dagu/scripts/backup.sh >> /var/log/dagu-backup.log 2>&1
```

### Recovery Procedure

1. **Stop Dagu service**
   ```bash
   sudo systemctl stop dagu
   ```

2. **Restore from backup**
   ```bash
   # Extract backup
   tar -xzf /backup/dagu/20240115_020000.tar.gz -C /tmp/
   
   # Restore DAGs
   cp -r /tmp/20240115_020000/dags/* /opt/dagu/dags/
   
   # Restore data
   cp -r /tmp/20240115_020000/data/* /opt/dagu/data/
   
   # Restore config
   cp /tmp/20240115_020000/etc/dagu/config.yaml /etc/dagu/
   ```

3. **Verify permissions**
   ```bash
   chown -R dagu:dagu /opt/dagu
   ```

4. **Start service**
   ```bash
   sudo systemctl start dagu
   ```

### Disaster Recovery

For critical workflows:

1. **Multi-region backup**
   ```bash
   # Sync to multiple S3 regions
   aws s3 sync s3://backup-bucket/dagu/ s3://dr-bucket/dagu/ --source-region us-east-1 --region us-west-2
   ```

2. **Database replication** (if using external state store)
3. **Configuration management** (Ansible, Terraform)
4. **Automated recovery testing**

## Performance Tuning

### System Resources

#### CPU Optimization

```bash
# Set CPU affinity for Dagu process
taskset -c 0-3 dagu start-all

# Or use systemd
# In dagu.service:
[Service]
CPUAffinity=0-3
```

#### Memory Management

```yaml
# Limit concurrent executions
maxActiveRuns: 10      # Global limit
maxActiveSteps: 50     # Per-DAG limit

# Configure in base.yaml
maxCleanUpTimeSec: 300  # Cleanup timeout
histRetentionDays: 7    # Reduce history retention
```

#### Disk I/O

```bash
# Use separate disk for logs
mount /dev/sdb1 /opt/dagu/logs

# Enable noatime for better performance
mount -o remount,noatime /opt/dagu

# Configure log rotation to prevent disk full
```

### Database Optimization

If using external database for state:

```sql
-- Create indexes
CREATE INDEX idx_dag_runs_status ON dag_runs(status);
CREATE INDEX idx_dag_runs_created ON dag_runs(created_at);

-- Partition tables by date
CREATE TABLE dag_runs_2024_01 PARTITION OF dag_runs
FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');
```

### Network Optimization

```yaml
# nginx reverse proxy with caching
location /api/v1/dags {
    proxy_pass http://localhost:8080;
    proxy_cache dagu_cache;
    proxy_cache_valid 200 1m;
    proxy_cache_use_stale error timeout;
}
```

## Security Hardening

### File Permissions

```bash
# Secure directory structure
chmod 750 /opt/dagu
chmod 750 /opt/dagu/dags
chmod 740 /opt/dagu/data
chmod 740 /opt/dagu/logs
chmod 600 /etc/dagu/config.yaml

# SELinux context (if enabled)
semanage fcontext -a -t user_home_t "/opt/dagu(/.*)?"
restorecon -Rv /opt/dagu
```

### Network Security

```bash
# Firewall rules (UFW)
ufw allow from 10.0.0.0/8 to any port 8080
ufw deny 8080

# iptables
iptables -A INPUT -p tcp --dport 8080 -s 10.0.0.0/8 -j ACCEPT
iptables -A INPUT -p tcp --dport 8080 -j DROP
```

### Audit Logging

Enable audit logging for compliance:

```bash
# auditd rules
cat >> /etc/audit/rules.d/dagu.rules << EOF
# Monitor DAG modifications
-w /opt/dagu/dags/ -p wa -k dagu_dag_change

# Monitor config changes
-w /etc/dagu/ -p wa -k dagu_config_change

# Monitor service actions
-w /usr/local/bin/dagu -p x -k dagu_execution
EOF

# Reload rules
auditctl -R /etc/audit/rules.d/dagu.rules
```

## Maintenance

### Regular Tasks

Daily:
- Check service health
- Monitor disk usage
- Review error logs

Weekly:
- Verify backups
- Check for updates
- Review performance metrics

Monthly:
- Clean old logs
- Update documentation
- Security audit
- Test disaster recovery

### Maintenance Scripts

```bash
#!/bin/bash
# /opt/dagu/scripts/maintenance.sh

# Clean old logs
find /opt/dagu/logs -type f -mtime +30 -delete

# Vacuum data files
find /opt/dagu/data -name "*.tmp" -delete

# Check disk usage
df -h /opt/dagu | awk 'NR==2 {if($5+0 > 80) print "WARNING: Disk usage above 80%"}'

# Verify permissions
find /opt/dagu -type f -not -user dagu -exec echo "Wrong owner: {}" \;

# Test backup
/opt/dagu/scripts/backup.sh --test
```

### Upgrade Procedure

1. **Plan the upgrade**
   - Review release notes
   - Test in staging
   - Schedule maintenance window

2. **Backup current state**
   ```bash
   /opt/dagu/scripts/backup.sh
   ```

3. **Stop service**
   ```bash
   sudo systemctl stop dagu
   ```

4. **Upgrade binary**
   ```bash
   curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | sudo bash
   ```

5. **Verify configuration compatibility**
   ```bash
   dagu validate --config /etc/dagu/config.yaml
   ```

6. **Start service**
   ```bash
   sudo systemctl start dagu
   ```

7. **Verify operation**
   ```bash
   curl http://localhost:8080/api/v1/dags
   systemctl status dagu
   ```

8. **Monitor for issues**
   ```bash
   journalctl -u dagu -f
   ```

## Troubleshooting

### Common Issues

#### Service Won't Start

```bash
# Check logs
journalctl -u dagu -n 100

# Verify configuration
dagu validate --config /etc/dagu/config.yaml

# Check permissions
ls -la /opt/dagu/

# Test manually
sudo -u dagu /usr/local/bin/dagu start-all --debug
```

#### High Memory Usage

```bash
# Check process memory
ps aux | grep dagu

# Analyze memory profile
# Enable profiling in config
debug: true

# Check for memory leaks
pprof -http=:6060 http://localhost:8080/debug/pprof/heap
```

#### Slow Performance

```bash
# Check disk I/O
iostat -x 1

# Check CPU usage
top -p $(pgrep dagu)

# Review slow queries
grep -i "slow" /opt/dagu/logs/admin/*.log

# Analyze workflow execution times
```

## Next Steps

- [Configure monitoring](#monitoring) for your environment
- [Set up backups](#backup-and-recovery) for disaster recovery
- [Tune performance](#performance-tuning) for your workload
- [Review security](#security-hardening) best practices