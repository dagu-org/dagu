# SSH Executor

Execute commands on remote servers via SSH.

## Basic Usage

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

## Configuration

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `user` | Yes | - | SSH username |
| `ip` | Yes | - | Hostname or IP address |
| `port` | No | "22" | SSH port |
| `key` | No | - | Private key path |
| `password` | No | - | Password (not recommended) |

## Examples

### Custom Port

```yaml
steps:
  - name: custom-port
    executor:
      type: ssh
      config:
        user: admin
        ip: example.com
        port: "2222"
        key: ~/.ssh/id_rsa
    command: uptime
```

### Multi-Server Deployment

```yaml
params:
  - VERSION: ${VERSION}

steps:
  - name: deploy-web1
    executor:
      type: ssh
      config:
        user: deploy
        ip: web1.example.com
        key: ~/.ssh/deploy_key
    command: |
      cd /opt/app
      git pull
      ./deploy.sh ${VERSION}

  - name: deploy-web2
    executor:
      type: ssh
      config:
        user: deploy
        ip: web2.example.com
        key: ~/.ssh/deploy_key
    command: |
      cd /opt/app
      git pull
      ./deploy.sh ${VERSION}
    depends: deploy-web1
```

### Health Check with Retry

```yaml
steps:
  - name: check-service
    executor:
      type: ssh
      config:
        user: monitor
        ip: app.example.com
        key: ~/.ssh/monitor_key
    command: systemctl is-active nginx
    retryPolicy:
      limit: 3
      intervalSec: 30
```

### Capture Remote Output

```yaml
steps:
  - name: get-version
    executor:
      type: ssh
      config:
        user: admin
        ip: server.example.com
        key: ~/.ssh/admin_key
    command: cat /opt/app/version.txt
    output: REMOTE_VERSION

  - name: log-version
    command: echo "Remote version: ${REMOTE_VERSION}"
```

## Common Patterns

### Database Backup

```yaml
steps:
  - name: backup-db
    executor:
      type: ssh
      config:
        user: backup
        ip: db.example.com
        key: ~/.ssh/backup_key
    command: |
      BACKUP="/backups/db_$(date +%Y%m%d_%H%M%S).sql.gz"
      mysqldump mydb | gzip > $BACKUP
      echo $BACKUP
    output: BACKUP_FILE
```

### Rolling Restart

```yaml
steps:
  - name: restart-app1
    executor:
      type: ssh
      config:
        user: admin
        ip: app1.example.com
        key: ~/.ssh/admin_key
    command: |
      systemctl restart myapp
      sleep 10
      systemctl is-active myapp

  - name: restart-app2
    executor:
      type: ssh
      config:
        user: admin
        ip: app2.example.com
        key: ~/.ssh/admin_key
    command: |
      systemctl restart myapp
      sleep 10
      systemctl is-active myapp
```

## See Also

- [Docker Executor](/features/executors/docker) - Container execution
- [Shell Executor](/features/executors/shell) - Local commands
