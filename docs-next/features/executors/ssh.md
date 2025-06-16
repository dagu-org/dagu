# SSH Executor

Execute commands on remote servers over SSH.

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
| `port` | No | "22" | SSH port number (string) |
| `key` | No | - | Path to SSH private key file |
| `password` | No | - | Password for authentication |

**Authentication:**
- **Key-based** (recommended): Provide `key` field
- **Password-based**: Provide `password` field

## Examples

### With Custom Port

```yaml
steps:
  - name: custom-port
    executor:
      type: ssh
      config:
        user: admin
        ip: example.com
        port: "2222"
        key: ~/.ssh/private_key
    command: uptime
```

### Password Authentication

```yaml
steps:
  - name: password-auth
    executor:
      type: ssh
      config:
        user: deploy
        ip: server.example.com
        password: "${SSH_PASSWORD}"
    command: systemctl status myapp
```

### Multi-line Script

```yaml
steps:
  - name: deploy
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

### With Variables

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

## Common Patterns

### Health Check

```yaml
steps:
  - name: health-check
    executor:
      type: ssh
      config:
        user: monitor
        ip: web1.example.com
        key: ~/.ssh/monitor_key
    command: |
      systemctl is-active nginx || exit 1
      netstat -tlnp | grep :80 || exit 1
    retryPolicy:
      limit: 3
      intervalSec: 30
```

### Sequential Deployment

```yaml
steps:
  - name: deploy-app1
    executor:
      type: ssh
      config:
        user: deploy
        ip: app1.example.com
        key: ~/.ssh/deploy_key
    command: ./deploy.sh

  - name: deploy-app2
    executor:
      type: ssh
      config:
        user: deploy
        ip: app2.example.com
        key: ~/.ssh/deploy_key
    command: ./deploy.sh
    depends: deploy-app1
```

### Backup with Output

```yaml
steps:
  - name: create-backup
    executor:
      type: ssh
      config:
        user: backup
        ip: db-server.example.com
        key: ~/.ssh/backup_key
    command: |
      BACKUP_FILE="/backups/db_$(date +%Y%m%d_%H%M%S).sql.gz"
      mysqldump --all-databases | gzip > $BACKUP_FILE
      echo $BACKUP_FILE
    output: BACKUP_PATH
    
  - name: verify-backup
    executor:
      type: ssh
      config:
        user: backup
        ip: db-server.example.com
        key: ~/.ssh/backup_key
    command: |
      if [ ! -s "${BACKUP_PATH}" ]; then
        echo "Backup failed - file is empty"
        exit 1
      fi
      echo "Backup successful: ${BACKUP_PATH}"
    depends: create-backup
```

## Error Handling

### Connection Retry

```yaml
steps:
  - name: resilient-ssh
    executor:
      type: ssh
      config:
        user: admin
        ip: server.example.com
        key: ~/.ssh/admin_key
    command: systemctl status myapp
    retryPolicy:
      limit: 3
      intervalSec: 10
```

### Continue on Failure

```yaml
steps:
  - name: optional-cleanup
    executor:
      type: ssh
      config:
        user: admin
        ip: server.example.com
        key: ~/.ssh/admin_key
    command: /opt/scripts/cleanup.sh
    continueOn:
      failure: true
```

## See Also

- [Docker Executor](/features/executors/docker) - Run commands in containers
- [HTTP Executor](/features/executors/http) - Make API calls
- [Error Handling](/writing-workflows/error-handling) - Handle failures gracefully
