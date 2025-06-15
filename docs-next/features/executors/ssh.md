# SSH Executor

The SSH executor enables you to execute commands on remote servers over SSH, perfect for managing distributed systems and remote operations.

## Overview

The SSH executor allows you to:

- Execute commands on remote hosts securely
- Use key-based authentication
- Connect to custom SSH ports
- Run scripts and complex operations remotely
- Manage multiple servers from a single workflow
- Maintain persistent connections for efficiency

## Basic Usage

### Simple SSH Command

```yaml
steps:
  - name: remote-command
    executor:
      type: ssh
      config:
        user: ubuntu
        ip: 192.168.1.100
        key: /home/user/.ssh/id_rsa
    command: echo "Hello from remote server"
```

### With Custom Port

```yaml
steps:
  - name: custom-port
    executor:
      type: ssh
      config:
        user: admin
        ip: example.com
        port: 2222
        key: /home/user/.ssh/private_key
    command: uptime
```

## Authentication

### Key-Based Authentication

The SSH executor uses key-based authentication for security:

```yaml
steps:
  - name: key-auth
    executor:
      type: ssh
      config:
        user: deploy
        ip: production.example.com
        key: /opt/keys/deploy_key.pem
    command: systemctl status myapp
```

### Using Environment Variables

Store sensitive configuration in environment variables:

```yaml
env:
  - SSH_USER: ${REMOTE_USER}
  - SSH_HOST: ${REMOTE_HOST}
  - SSH_KEY_PATH: ${SSH_KEY_PATH}

steps:
  - name: env-based-ssh
    executor:
      type: ssh
      config:
        user: ${SSH_USER}
        ip: ${SSH_HOST}
        key: ${SSH_KEY_PATH}
    command: df -h
```

## Remote Command Execution

### Single Commands

```yaml
steps:
  - name: check-disk-space
    executor:
      type: ssh
      config:
        user: admin
        ip: server1.example.com
        key: ~/.ssh/admin_key
    command: df -h /data
```

### Multi-line Scripts

```yaml
steps:
  - name: complex-script
    executor:
      type: ssh
      config:
        user: deploy
        ip: app-server.example.com
        key: ~/.ssh/deploy_key
    command: |
      cd /opt/application
      git pull origin main
      npm install
      npm run build
      sudo systemctl restart app-service
```

### Script with Variables

```yaml
params:
  - VERSION: 1.2.3
  - ENVIRONMENT: production

steps:
  - name: deploy-version
    executor:
      type: ssh
      config:
        user: deploy
        ip: ${ENVIRONMENT}.example.com
        key: ~/.ssh/deploy_key
    command: |
      echo "Deploying version ${VERSION} to ${ENVIRONMENT}"
      cd /opt/app
      ./deploy.sh --version ${VERSION} --env ${ENVIRONMENT}
```

## Real-World Examples

### Server Health Checks

```yaml
name: health-check
schedule: "*/5 * * * *"  # Every 5 minutes

steps:
  - name: check-web-server
    executor:
      type: ssh
      config:
        user: monitor
        ip: web1.example.com
        key: ~/.ssh/monitor_key
    command: |
      # Check if nginx is running
      systemctl is-active nginx || exit 1
      
      # Check if port 80 is listening
      netstat -tlnp | grep :80 || exit 1
      
      # Check disk space
      df -h / | awk 'NR==2 {gsub("%",""); if($5 > 90) exit 1}'
      
      # Check memory usage
      free | awk '/^Mem/ {if($3/$2 > 0.9) exit 1}'
    output: WEB_STATUS

  - name: check-db-server
    executor:
      type: ssh
      config:
        user: monitor
        ip: db1.example.com
        key: ~/.ssh/monitor_key
    command: |
      # Check if MySQL is running
      systemctl is-active mysql || exit 1
      
      # Check if can connect to MySQL
      mysql -e "SELECT 1" || exit 1
      
      # Check replication status
      mysql -e "SHOW SLAVE STATUS\G" | grep -E "Slave_IO_Running|Slave_SQL_Running"
    output: DB_STATUS
```

### Multi-Server Deployment

```yaml
name: rolling-deployment
params:
  - VERSION: latest
  - SERVERS: "app1.example.com app2.example.com app3.example.com"

steps:
  - name: deploy-to-app1
    executor:
      type: ssh
      config:
        user: deploy
        ip: app1.example.com
        key: ~/.ssh/deploy_key
    command: |
      # Remove from load balancer
      curl -X POST http://lb.example.com/api/remove/app1
      sleep 10
      
      # Deploy new version
      cd /opt/application
      docker pull myapp:${VERSION}
      docker stop myapp || true
      docker run -d --name myapp -p 80:80 myapp:${VERSION}
      
      # Health check
      timeout 60 bash -c 'until curl -f http://localhost/health; do sleep 1; done'
      
      # Add back to load balancer
      curl -X POST http://lb.example.com/api/add/app1

  - name: deploy-to-app2
    executor:
      type: ssh
      config:
        user: deploy
        ip: app2.example.com
        key: ~/.ssh/deploy_key
    command: |
      # Same deployment process for app2
      cd /opt/application
      docker pull myapp:${VERSION}
      docker stop myapp || true
      docker run -d --name myapp -p 80:80 myapp:${VERSION}
    depends: deploy-to-app1

  - name: deploy-to-app3
    executor:
      type: ssh
      config:
        user: deploy
        ip: app3.example.com
        key: ~/.ssh/deploy_key
    command: |
      # Same deployment process for app3
      cd /opt/application
      docker pull myapp:${VERSION}
      docker stop myapp || true
      docker run -d --name myapp -p 80:80 myapp:${VERSION}
    depends: deploy-to-app2
```

### Remote Backup Operations

```yaml
name: database-backup
schedule: "0 2 * * *"  # 2 AM daily

steps:
  - name: create-backup
    executor:
      type: ssh
      config:
        user: backup
        ip: db-primary.example.com
        key: ~/.ssh/backup_key
    command: |
      BACKUP_FILE="/backups/db_$(date +%Y%m%d_%H%M%S).sql.gz"
      
      # Create backup
      mysqldump --all-databases --single-transaction --quick | gzip > $BACKUP_FILE
      
      # Verify backup
      if [ ! -s $BACKUP_FILE ]; then
        echo "Backup failed - file is empty"
        exit 1
      fi
      
      echo $BACKUP_FILE
    output: BACKUP_PATH

  - name: transfer-to-storage
    executor:
      type: ssh
      config:
        user: backup
        ip: storage.example.com
        key: ~/.ssh/backup_key
    command: |
      # Pull backup from database server
      scp -i ~/.ssh/db_key backup@db-primary.example.com:${BACKUP_PATH} /storage/backups/
      
      # Verify transfer
      LOCAL_FILE="/storage/backups/$(basename ${BACKUP_PATH})"
      if [ ! -s $LOCAL_FILE ]; then
        echo "Transfer failed"
        exit 1
      fi
      
      # Calculate checksum
      sha256sum $LOCAL_FILE
    depends: create-backup

  - name: cleanup-old-backups
    executor:
      type: ssh
      config:
        user: backup
        ip: storage.example.com
        key: ~/.ssh/backup_key
    command: |
      # Keep only last 30 days of backups
      find /storage/backups -name "db_*.sql.gz" -mtime +30 -delete
      
      # Report storage usage
      df -h /storage
    depends: transfer-to-storage
```

### Log Collection

```yaml
name: collect-logs
params:
  - DATE: "`date +%Y%m%d`"

steps:
  - name: collect-web-logs
    executor:
      type: ssh
      config:
        user: loguser
        ip: web-server.example.com
        key: ~/.ssh/log_key
    command: |
      # Compress today's logs
      cd /var/log/nginx
      tar -czf /tmp/web-logs-${DATE}.tar.gz access.log error.log
      
      # Copy to central location
      scp /tmp/web-logs-${DATE}.tar.gz logserver:/logs/web/
      
      # Cleanup
      rm /tmp/web-logs-${DATE}.tar.gz

  - name: collect-app-logs
    executor:
      type: ssh
      config:
        user: loguser
        ip: app-server.example.com
        key: ~/.ssh/log_key
    command: |
      # Collect application logs
      cd /opt/app/logs
      tar -czf /tmp/app-logs-${DATE}.tar.gz *.log
      
      # Copy to central location
      scp /tmp/app-logs-${DATE}.tar.gz logserver:/logs/app/
      
      # Cleanup
      rm /tmp/app-logs-${DATE}.tar.gz

  - name: analyze-logs
    executor:
      type: ssh
      config:
        user: analyst
        ip: logserver.example.com
        key: ~/.ssh/analyst_key
    command: |
      cd /logs
      
      # Extract and analyze
      tar -xzf web/web-logs-${DATE}.tar.gz -C /tmp/
      tar -xzf app/app-logs-${DATE}.tar.gz -C /tmp/
      
      # Run analysis
      python3 /opt/log-analyzer/analyze.py --date ${DATE}
    depends:
      - collect-web-logs
      - collect-app-logs
```

## Advanced Patterns

### Dynamic Server Lists

```yaml
steps:
  - name: get-server-list
    command: |
      # Get list of servers from inventory
      curl -s http://inventory.example.com/api/servers?env=production | \
        jq -r '.[] | select(.status=="active") | .hostname'
    output: SERVERS

  - name: run-on-all-servers
    command: |
      # Loop through servers
      for server in ${SERVERS}; do
        echo "Processing $server"
        
        ssh -i ~/.ssh/admin_key -o StrictHostKeyChecking=no admin@$server \
          "uname -a; uptime; df -h"
      done
    depends: get-server-list
```

### Conditional Remote Execution

```yaml
steps:
  - name: check-service
    executor:
      type: ssh
      config:
        user: admin
        ip: app-server.example.com
        key: ~/.ssh/admin_key
    command: systemctl is-active myapp
    continueOn:
      exitCode: [3]  # Service not found
    output: SERVICE_STATUS

  - name: install-if-missing
    executor:
      type: ssh
      config:
        user: admin
        ip: app-server.example.com
        key: ~/.ssh/admin_key
    command: |
      echo "Service not found, installing..."
      wget https://releases.example.com/myapp-latest.tar.gz
      tar -xzf myapp-latest.tar.gz
      sudo ./install.sh
    preconditions:
      - condition: "${SERVICE_STATUS}"
        expected: ""
    depends: check-service
```

### Remote File Operations

```yaml
steps:
  - name: remote-file-check
    executor:
      type: ssh
      config:
        user: data
        ip: fileserver.example.com
        key: ~/.ssh/data_key
    command: |
      # Check if file exists and is not empty
      FILE="/data/input/daily_feed.csv"
      
      if [ ! -f "$FILE" ]; then
        echo "ERROR: File not found"
        exit 1
      fi
      
      if [ ! -s "$FILE" ]; then
        echo "ERROR: File is empty"
        exit 1
      fi
      
      # Return file info
      ls -la "$FILE"
      wc -l "$FILE"
    output: FILE_INFO

  - name: process-remote-file
    executor:
      type: ssh
      config:
        user: data
        ip: fileserver.example.com
        key: ~/.ssh/data_key
    command: |
      cd /data/processing
      python3 process_daily_feed.py --input /data/input/daily_feed.csv
    depends: remote-file-check
```

## Error Handling

### Connection Retry

```yaml
steps:
  - name: resilient-connection
    executor:
      type: ssh
      config:
        user: admin
        ip: unstable-server.example.com
        key: ~/.ssh/admin_key
    command: systemctl status myapp
    retryPolicy:
      limit: 3
      intervalSec: 10
```

### Timeout Handling

```yaml
steps:
  - name: long-running-command
    executor:
      type: ssh
      config:
        user: admin
        ip: remote-server.example.com
        key: ~/.ssh/admin_key
    command: |
      # Use timeout to prevent hanging
      timeout 300 /opt/scripts/long_process.sh
    timeout: 600  # DAG step timeout
```

### Error Recovery

```yaml
steps:
  - name: safe-deployment
    executor:
      type: ssh
      config:
        user: deploy
        ip: production.example.com
        key: ~/.ssh/deploy_key
    command: |
      set -e
      
      # Backup current version
      cp -r /opt/app /opt/app.backup
      
      # Deploy new version
      cd /opt/app
      git pull || {
        echo "Git pull failed, restoring backup"
        rm -rf /opt/app
        mv /opt/app.backup /opt/app
        exit 1
      }
      
      # Test deployment
      ./test.sh || {
        echo "Tests failed, restoring backup"
        rm -rf /opt/app
        mv /opt/app.backup /opt/app
        exit 1
      }
      
      # Success - remove backup
      rm -rf /opt/app.backup
```

## Best Practices

### 1. Use Dedicated SSH Keys

```yaml
# Good - specific keys for different purposes
steps:
  - name: deploy-key
    executor:
      type: ssh
      config:
        user: deploy
        ip: server.example.com
        key: ~/.ssh/deploy_key  # Limited permissions

  - name: admin-key
    executor:
      type: ssh
      config:
        user: admin
        ip: server.example.com
        key: ~/.ssh/admin_key  # Admin permissions
```

### 2. Set Proper Timeouts

```yaml
steps:
  - name: quick-check
    executor:
      type: ssh
      config:
        user: monitor
        ip: server.example.com
        key: ~/.ssh/monitor_key
    command: uptime
    timeout: 30  # Quick commands

  - name: slow-operation
    executor:
      type: ssh
      config:
        user: admin
        ip: server.example.com
        key: ~/.ssh/admin_key
    command: ./backup_database.sh
    timeout: 3600  # Long operations
```

### 3. Use SSH Config Files

Configure SSH options in `~/.ssh/config`:

```
Host production
    HostName production.example.com
    User deploy
    Port 22
    IdentityFile ~/.ssh/deploy_key
    StrictHostKeyChecking yes
    ServerAliveInterval 60
```

Then use in Dagu:

```yaml
steps:
  - name: use-ssh-config
    command: ssh production "systemctl status myapp"
```

### 4. Handle SSH Agent

When running Dagu as a service, ensure SSH keys are accessible:

```yaml
steps:
  - name: with-ssh-agent
    command: |
      # Start SSH agent if needed
      eval $(ssh-agent -s)
      ssh-add ~/.ssh/deploy_key
      
      # Now use SSH
      ssh deploy@server.example.com "deploy.sh"
```

## Security Considerations

### Restrict SSH Key Permissions

```yaml
steps:
  - name: check-key-permissions
    command: |
      # Ensure proper permissions
      chmod 600 ~/.ssh/deploy_key
      chmod 700 ~/.ssh
    
  - name: secure-ssh
    executor:
      type: ssh
      config:
        user: deploy
        ip: server.example.com
        key: ~/.ssh/deploy_key
    command: ./deploy.sh
    depends: check-key-permissions
```

### Use Jump Hosts

For accessing servers through bastion hosts:

```yaml
steps:
  - name: via-bastion
    command: |
      # Use ProxyJump
      ssh -J bastion.example.com deploy@internal-server.local \
        "systemctl status myapp"
```

### Audit Commands

```yaml
steps:
  - name: audit-command
    executor:
      type: ssh
      config:
        user: admin
        ip: server.example.com
        key: ~/.ssh/admin_key
    command: |
      # Log the command
      logger -t dagu-ssh "Executing maintenance script"
      
      # Run the actual command
      /opt/scripts/maintenance.sh
```

## Troubleshooting

### Debug SSH Connections

```yaml
steps:
  - name: debug-ssh
    command: |
      # Verbose SSH for debugging
      ssh -vvv -i ~/.ssh/test_key user@server.example.com "echo test"
```

### Test Connectivity

```yaml
steps:
  - name: test-ssh
    executor:
      type: ssh
      config:
        user: test
        ip: server.example.com
        port: 22
        key: ~/.ssh/test_key
    command: echo "SSH connection successful"
    continueOn:
      failure: true
    
  - name: report-status
    command: |
      if [ $? -eq 0 ]; then
        echo "SSH is working"
      else
        echo "SSH connection failed"
        # Additional diagnostics
        ping -c 3 server.example.com
        nc -zv server.example.com 22
      fi
    depends: test-ssh
```

## Next Steps

- Learn about [Mail Executor](/features/executors/mail) for notifications
- Explore [JQ Executor](/features/executors/jq) for JSON processing
- Check out [Execution Control](/features/execution-control) for advanced patterns