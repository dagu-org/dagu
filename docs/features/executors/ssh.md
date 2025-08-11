# SSH Executor

Execute commands on remote servers via SSH.

## DAG-Level Configuration

You can configure SSH settings at the DAG level to avoid repetition:

```yaml
# DAG-level SSH configuration
ssh:
  user: deploy
  host: production.example.com
  port: 22
  key: ~/.ssh/deploy_key
  strictHostKey: true  # Default: true for security
  knownHostFile: ~/.ssh/known_hosts  # Default: ~/.ssh/known_hosts

steps:
  # All SSH steps inherit DAG-level configuration
  - name: check-health
    command: curl -f http://localhost:8080/health

  - name: restart-service
    command: systemctl restart myapp
```

## Step-Level Configuration

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

### DAG-Level Fields

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `user` | Yes | - | SSH username |
| `host` | Yes | - | Hostname or IP address |
| `port` | No | "22" | SSH port |
| `key` | No | Auto-detect | Private key path (see below) |
| `strictHostKey` | No | `true` | Enable host key verification |
| `knownHostFile` | No | `~/.ssh/known_hosts` | Known hosts file path |

### Step-Level Fields

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `user` | Yes | - | SSH username |
| `host` | Yes | - | Hostname or IP address |
| `port` | No | "22" | SSH port |
| `key` | No | Auto-detect | Private key path (see below) |
| `password` | No | - | Password (not recommended) |
| `strictHostKey` | No | `true` | Enable host key verification |
| `knownHostFile` | No | `~/.ssh/known_hosts` | Known hosts file path |

**Note:** Password authentication is not available at the DAG level for security reasons.

### SSH Key Auto-Detection

If no key is specified, Dagu automatically tries these default SSH keys in order:
1. `~/.ssh/id_rsa`
2. `~/.ssh/id_ecdsa`
3. `~/.ssh/id_ed25519`
4. `~/.ssh/id_dsa`

## Security Best Practices

1. **Host Key Verification**: Always enabled by default (`strictHostKey: true`)
   - Prevents man-in-the-middle attacks
   - Uses `~/.ssh/known_hosts` by default
   - Only disable for testing environments

2. **Key-Based Authentication**: Strongly recommended
   - DAG-level SSH only supports key-based authentication
   - Use dedicated deployment keys with limited permissions
   - Rotate keys regularly

3. **Known Hosts Management**:
   ```bash
   # Add host to known_hosts before running DAGs
   ssh-keyscan -H production.example.com >> ~/.ssh/known_hosts
   ```

## See Also

- [Docker Executor](/features/executors/docker) - Container execution
- [Shell Executor](/features/executors/shell) - Local commands
