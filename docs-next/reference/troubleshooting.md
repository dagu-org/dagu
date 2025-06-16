# Troubleshooting Guide

Solutions to common issues and debugging techniques for Dagu.

## Installation Issues

### Command Not Found

After installation, if `dagu` command is not found:

```bash
# Check if binary exists
ls -la /usr/local/bin/dagu

# Add to PATH if needed
echo 'export PATH="/usr/local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc

# Or create symlink
sudo ln -s /usr/local/bin/dagu /usr/bin/dagu
```

### Permission Denied

```bash
# Fix binary permissions
chmod +x /usr/local/bin/dagu

# Fix directory permissions
mkdir -p ~/.config/dagu ~/.local/share/dagu
chmod 755 ~/.config/dagu
chmod 755 ~/.local/share/dagu
```

### Installation Script Fails

```bash
# Manual installation
VERSION=$(curl -s https://api.github.com/repos/dagu-org/dagu/releases/latest | grep tag_name | cut -d '"' -f 4)
curl -L -o dagu.tar.gz "https://github.com/dagu-org/dagu/releases/download/${VERSION}/dagu_${VERSION}_linux_amd64.tar.gz"
tar -xzf dagu.tar.gz
sudo mv dagu /usr/local/bin/
```

## Server Issues

### Port Already in Use

```bash
# Find what's using the port
lsof -i :8080

# Kill the process
kill -9 <PID>

# Or use different port
dagu start-all --port 9000
```

### Server Won't Start

1. Check logs:
   ```bash
   dagu start-all --debug
   ```

2. Verify configuration:
   ```bash
   # Check config syntax
   cat ~/.config/dagu/config.yaml | yamllint -
   ```

3. Check permissions:
   ```bash
   ls -la ~/.config/dagu/
   ls -la ~/.local/share/dagu/
   ```

### Can't Access Web UI

1. Check server is running:
   ```bash
   ps aux | grep dagu
   curl http://localhost:8080/api/v1/dags
   ```

2. Check firewall:
   ```bash
   # Ubuntu/Debian
   sudo ufw status
   sudo ufw allow 8080

   # RHEL/CentOS
   sudo firewall-cmd --list-ports
   sudo firewall-cmd --add-port=8080/tcp --permanent
   ```

3. Check binding address:
   ```bash
   # For remote access, bind to all interfaces
   dagu start-all --host 0.0.0.0
   ```

## Workflow Execution Issues

### Workflow Not Found

```bash
# Check DAG exists
ls ~/.config/dagu/dags/

# Check file extension
# Must be .yaml or .yml
mv workflow.dag workflow.yaml

# Run with full path
dagu start /path/to/workflow.yaml
```

### Workflow Fails Immediately

1. Validate syntax:
   ```bash
   dagu dry workflow.yaml
   ```

2. Check for syntax errors:
   ```yaml
   # Common mistakes:
   
   # Wrong - missing dash
   steps:
     name: step1
   
   # Correct
   steps:
     - name: step1
   
   # Wrong - bad indentation
   steps:
   - name: step1
   command: echo hello
   
   # Correct
   steps:
     - name: step1
       command: echo hello
   ```

3. Check dependencies:
   ```yaml
   # Wrong - dependency doesn't exist
   steps:
     - name: step1
       command: echo hello
     - name: step2
       depends: step-one  # Typo!
   
   # Correct
   steps:
     - name: step1
       command: echo hello
     - name: step2
       depends: step1
   ```

### Command Not Found in Workflow

```yaml
# Use absolute paths
steps:
  - name: run-script
    command: /opt/scripts/process.sh
    
# Or set PATH
steps:
  - name: run-script
    command: process.sh
    env:
      - PATH: /opt/scripts:${PATH}
```

### Variables Not Working

1. Check syntax:
   ```yaml
   # Wrong
   env:
     - VAR: $VALUE     # Missing braces
   
   # Correct
   env:
     - VAR: ${VALUE}
   ```

2. Check variable is defined:
   ```yaml
   steps:
     - name: debug
       command: |
         echo "VAR=${VAR}"
         env | grep VAR
   ```

3. Use command substitution correctly:
   ```yaml
   # Wrong - regular quotes
   env:
     - DATE: "date +%Y-%m-%d"
   
   # Correct - backticks
   env:
     - DATE: "`date +%Y-%m-%d`"
   ```

## Scheduler Issues

### Scheduled Workflows Not Running

1. Check scheduler is running:
   ```bash
   ps aux | grep "dagu scheduler"
   ```

2. Verify schedule syntax:
   ```yaml
   # Correct cron syntax
   schedule: "0 2 * * *"    # 2 AM daily
   
   # With timezone
   schedule: "CRON_TZ=America/New_York 0 2 * * *"
   ```

3. Check logs:
   ```bash
   tail -f ~/.local/share/dagu/logs/admin/scheduler.log
   ```

### Workflow Runs Multiple Times

```yaml
# Add this to prevent redundant runs
skipIfSuccessful: true
```

### Wrong Timezone

```yaml
# Set timezone in schedule
schedule: "CRON_TZ=Asia/Tokyo 0 9 * * *"

# Or globally
env:
  - TZ: Asia/Tokyo
```

## Performance Issues

### Slow Workflow Execution

1. Check system resources:
   ```bash
   top
   df -h
   free -m
   ```

2. Limit parallelism:
   ```yaml
   maxActiveSteps: 5    # Limit concurrent steps
   ```

3. Check for bottlenecks:
   ```yaml
   steps:
     - name: slow-step
       command: time ./slow-command.sh
   ```

### High Memory Usage

1. Limit output size:
   ```yaml
   maxOutputSize: 1048576  # 1MB limit
   ```

2. Redirect large outputs:
   ```yaml
   steps:
     - name: large-output
       command: ./generate-data.sh
       stdout: /tmp/output.txt
   ```

3. Clean up old logs:
   ```yaml
   histRetentionDays: 7  # Keep only 7 days
   ```

### Disk Space Issues

```bash
# Check disk usage
du -sh ~/.local/share/dagu/

# Clean old logs
find ~/.local/share/dagu/logs -mtime +30 -delete

# Set retention
```

## Docker Executor Issues

### Cannot Connect to Docker

```bash
# Check Docker is running
docker ps

# Check permissions
sudo usermod -aG docker $USER
newgrp docker

# If using container, mount socket
docker run -v /var/run/docker.sock:/var/run/docker.sock ...
```

### Image Pull Errors

```yaml
steps:
  - name: docker-step
    executor:
      type: docker
      config:
        image: myimage:latest
        pull: never  # For local images
```

### Container Permission Issues

```yaml
steps:
  - name: docker-step
    executor:
      type: docker
      config:
        image: alpine
        container:
          user: "1000:1000"  # Match host user
```

## Authentication Issues

### Basic Auth Not Working

1. Check configuration:
   ```bash
   # Environment variables
   export DAGU_AUTH_BASIC_USERNAME=admin
   export DAGU_AUTH_BASIC_PASSWORD=secret
   
   # Or in config.yaml
   auth:
     basic:
       enabled: true
       username: admin
       password: secret
   ```

2. Clear browser cache

3. Try incognito/private mode

### API Token Rejected

```bash
# Correct header format
curl -H "Authorization: Bearer your-token" http://localhost:8080/api/v1/dags

# Not "Token" or "Bearer Token"
```

## Common Error Messages

### "No such file or directory"

- Check file paths are absolute or relative to working directory
- Verify file exists: `ls -la /path/to/file`
- Check permissions: `ls -la`

### "Permission denied"

- Check file permissions: `chmod +x script.sh`
- Check directory permissions: `chmod 755 directory/`
- Run with correct user

### "Exit status 127"

- Command not found
- Check PATH environment variable
- Use absolute paths

### "Exit status 126"

- Command found but not executable
- Run: `chmod +x command`

### "Signal: killed"

- Process terminated by system (often OOM)
- Check system resources
- Reduce memory usage

## Debugging Techniques

### Enable Debug Mode

```bash
# Start with debug logging
dagu start-all --debug

# Or set in config
debug: true
```

### Check Process Status

```bash
# See running DAGs
ps aux | grep dagu

# Check specific DAG
dagu status my-workflow.yaml
```

### Examine Logs

```bash
# Scheduler logs
tail -f ~/.local/share/dagu/logs/admin/scheduler.log

# DAG logs
ls ~/.local/share/dagu/logs/dags/

# Step output
cat ~/.local/share/dagu/logs/dags/my-workflow/*/step1.stdout.log
```

### Test in Isolation

```yaml
# Test individual steps
steps:
  - name: test-step
    command: |
      set -x  # Enable debug output
      echo "Testing..."
      # Your command here
```

### Dry Run

```bash
# Test without executing
dagu dry my-workflow.yaml

# With parameters
dagu dry my-workflow.yaml -- PARAM=value
```

## Getting Help

### Collect Information

When reporting issues, include:

1. Dagu version:
   ```bash
   dagu version
   ```

2. System information:
   ```bash
   uname -a
   ```

3. Configuration (sanitized):
   ```bash
   cat ~/.config/dagu/config.yaml | grep -v password
   ```

4. Workflow file (minimal example)

5. Error messages and logs

### Community Support

- 💬 [Discord Community](https://discord.gg/gpahPpqyAP) - Get help from the community
- 🐛 [GitHub Issues](https://github.com/dagu-org/dagu/issues) - Report bugs
- 📖 [Documentation](/writing-workflows/) - Detailed guides

## FAQ

### How Long is History Data Stored?

By default, 30 days. Configure with:
```yaml
histRetentionDays: 7  # Keep only 7 days
```

### How to Retry from a Specific Step?

1. Open workflow in Web UI
2. Click on the failed step
3. Set status to "failed"
4. Rerun the workflow

### How Does Dagu Track Processes Without a Database?

Dagu uses Unix sockets to communicate with running processes. Process information is stored in local files under `~/.local/share/dagu/`.

### Can I Run Multiple Dagu Instances?

Yes, but:
- Use different ports
- Use different data directories
- Don't share DAG directories without coordination

### How to Migrate from Cron?

1. Convert cron schedule:
   ```yaml
   # Cron: 0 2 * * *
   schedule: "0 2 * * *"
   ```

2. Replace command:
   ```yaml
   steps:
     - name: old-cron-job
       command: /path/to/script.sh
   ```

3. Add error handling:
   ```yaml
   mailOn:
     failure: true
   ```

## Next Steps

- Review [best practices](/writing-workflows/advanced#best-practices)
- Explore [examples](https://github.com/dagu-org/dagu/tree/main/examples)
- Join [Discord](https://discord.gg/gpahPpqyAP) for help