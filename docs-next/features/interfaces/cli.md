# Command Line Interface

Master Dagu's powerful CLI for workflow management and automation.

## Overview

The Dagu CLI provides complete control over your workflows from the terminal. It's designed to be intuitive, scriptable, and integration-friendly.

## Basic Usage

```bash
dagu [global options] command [command options] [arguments...]
```

### Getting Help

```bash
# General help
dagu --help

# Command-specific help
dagu start --help

# Show version
dagu version
```

## Essential Commands

### Running Workflows

#### Start a Workflow
```bash
# Basic execution
dagu start my-workflow.yaml

# With named parameters (use -- separator)
dagu start etl.yaml -- DATE=2024-01-01 ENV=prod

# With positional parameters
dagu start my-workflow.yaml -- value1 value2 value3

# Queue for later
dagu enqueue my-workflow.yaml
```

#### Stop a Running Workflow
```bash
# Stop currently running workflow
dagu stop my-workflow

# Stop specific run
dagu stop --run-id=20240101_120000 my-workflow

# Can also use file path
dagu stop my-workflow.yaml
```

#### Restart a Workflow
```bash
# Restart latest run
dagu restart my-workflow

# Restart specific run
dagu restart --run-id=20240101_120000 my-workflow
```

#### Retry Failed Workflow
```bash
# Retry specific run (run-id is required)
dagu retry --run-id=20240101_120000 my-workflow

# Can also use file path
dagu retry --run-id=20240101_120000 my-workflow.yaml
```

### Monitoring Workflows

#### Check Status
```bash
# Check latest run status
dagu status my-workflow

# Check specific run status
dagu status --run-id=20240101_120000 my-workflow

# Can also use file path
dagu status my-workflow.yaml
```

Output:
```
Workflow: my-workflow
Status: running
Started: 2024-01-01 12:00:00
Duration: 5m 23s

Steps:
  ✓ download     [completed] (2m 10s)
  ⟳ process      [running] (3m 13s)
  ○ upload       [pending]
```

#### View Execution History
```bash
# Check detailed status and output
dagu status my-workflow.yaml

# Note: For detailed logs, use the web UI at http://localhost:8080
# or check log files in the configured log directory
```

### Development Commands

#### Test and Validate
```bash
# Dry run validates syntax and shows execution plan
dagu dry my-workflow.yaml

# With parameters (use -- separator)
dagu dry etl.yaml -- DATE=2024-01-01
```

### Server Commands

#### Start Everything
```bash
# Start scheduler and web UI (default: localhost:8080)
dagu start-all

# Custom host and port
dagu start-all --host=0.0.0.0 --port=9000

# Custom DAGs directory
dagu start-all --dags=/path/to/directory
```

#### Start Scheduler Only
```bash
# Run just the scheduler (no UI)
dagu scheduler

# Custom DAGs directory
dagu scheduler --dags=/opt/workflows
```

#### Start Web UI Only
```bash
# Run just the web server (no scheduler)
dagu server

# Custom host and port
dagu server --host=0.0.0.0 --port=9000

# Custom DAGs directory
dagu server --dags=/path/to/directory
```

## Advanced Usage

### Queue Management

```bash
# Add to queue
dagu enqueue my-workflow.yaml

# Add to queue with custom ID
dagu enqueue --run-id=custom-001 my-workflow.yaml

# Add to queue with parameters
dagu enqueue my-workflow.yaml -- KEY=value

# Remove from queue (requires DAG-name:run-id format)
dagu dequeue --dag-run=my-workflow:custom-001
```

### Working with Parameters

Parameters can be passed in multiple ways:

```bash
# Positional parameters (use -- separator)
dagu start my-workflow.yaml -- param1 param2 param3

# Named parameters (use -- separator)
dagu start my-workflow.yaml -- KEY1=value1 KEY2=value2

# Mixed (use -- separator)
dagu start my-workflow.yaml -- param1 KEY=value param2
```

### Using Configuration Files

```bash
# Use custom config
dagu --config /etc/dagu/prod.yaml start-all

# Override config with flags
dagu --config prod.yaml start-all --port 9000
```

### Environment Variables

All CLI options can be set via environment:

```bash
# Set defaults
export DAGU_HOST=0.0.0.0
export DAGU_PORT=8080
export DAGU_DAGS=/opt/workflows

# Start with environment config
dagu start-all
```

## Scripting with Dagu

### Automation Examples

#### Wait for Completion
```bash
#!/bin/bash
# Start workflow and wait for completion

dagu start my-workflow.yaml

while true; do
    status=$(dagu status my-workflow.yaml | grep "Status:" | awk '{print $2}')
    if [[ "$status" != "running" ]]; then
        echo "Workflow completed with status: $status"
        break
    fi
    sleep 5
done
```

#### Batch Processing
```bash
#!/bin/bash
# Process multiple dates

for date in 2024-01-{01..31}; do
    echo "Processing $date"
    dagu start etl.yaml -- DATE=$date
    
    # Wait between runs
    sleep 60
done
```

#### Error Handling
```bash
#!/bin/bash
# Run with retry on failure

if ! dagu start critical-workflow.yaml; then
    echo "First attempt failed, retrying..."
    sleep 30
    
    # Get the latest run ID to retry (this is just an example - you'd need actual logic)
    RUN_ID=$(dagu status critical-workflow.yaml | grep "Run ID:" | awk '{print $3}')
    
    if ! dagu retry --run-id="$RUN_ID" critical-workflow; then
        echo "Retry failed, sending alert"
        # Send notification
        exit 1
    fi
fi
```

### Output Parsing

#### JSON Output
```bash
# Get status as JSON (using jq)
dagu status my-workflow.yaml --json | jq '.status'

# Get specific step status
dagu status my-workflow.yaml --json | jq '.steps[] | select(.name=="process")'
```

#### Exit Codes
```bash
# Check execution success
if dagu start my-workflow.yaml; then
    echo "Success"
else
    echo "Failed with code: $?"
fi
```

## Integration Patterns

### CI/CD Integration

#### GitHub Actions
```yaml
- name: Run Dagu Workflow
  run: |
    dagu start deployment.yaml -- \
      BRANCH=${{ github.ref }} \
      COMMIT=${{ github.sha }}
```

#### Jenkins
```groovy
stage('Run ETL') {
    sh 'dagu start etl.yaml -- DATE=${BUILD_ID}'
}
```

#### GitLab CI
```yaml
deploy:
  script:
    - dagu start deploy.yaml -- ENV=production
```

### Cron Replacement

```bash
# Instead of complex cron
# 0 2 * * * cd /app && ./etl.sh >> /var/log/etl.log 2>&1

# Use Dagu
dagu scheduler --dags /app/workflows
```

### Monitoring Integration

```bash
# Prometheus metrics
curl http://localhost:8080/api/v2/metrics

# Health check for monitoring tools
curl -f http://localhost:8080/api/v1/health || exit 1
```

## Best Practices

### 1. **Use Dry Run for Testing**
```bash
# Always test complex workflows
dagu dry complex-workflow.yaml
```

### 2. **Version Control Your Workflows**
```bash
# Keep workflows in git
git add workflows/
git commit -m "Add new ETL workflow"
```

### 3. **Use Meaningful Run IDs**
```bash
# For tracking
dagu enqueue --run-id="deploy-$(date +%Y%m%d-%H%M%S)" deploy.yaml
```

### 4. **Monitor Execution**
```bash
# Capture output and monitor status
dagu start my-workflow.yaml
dagu status my-workflow.yaml
```

### 5. **Handle Signals Properly**
```bash
# Graceful shutdown
trap 'dagu stop my-workflow.yaml' SIGINT SIGTERM
dagu start my-workflow.yaml
```

## Troubleshooting

### Common Issues

#### Workflow Not Found
```bash
# Check path with dry run
dagu dry ./workflows/my-workflow.yaml

# Use absolute path
dagu start /opt/dagu/workflows/my-workflow.yaml

# Or use DAG name if already loaded
dagu start my-workflow
```

#### Permission Denied
```bash
# Check file permissions
ls -la my-workflow.yaml

# Check execution permissions
chmod +x my-workflow.yaml
```

#### Port Already in Use
```bash
# Find what's using the port
lsof -i :8080

# Use different port
dagu start-all --port 9000
```

### Debug Mode

```bash
# Enable debug logging
dagu --log-level debug start my-workflow.yaml

# Verbose output
DAGU_LOG_LEVEL=debug dagu start my-workflow.yaml
```

## CLI Configuration

### Global Options

| Option | Description | Default |
|--------|-------------|---------|
| `--config` | Config file path | `~/.config/dagu/config.yaml` |
| `--log-level` | Log verbosity | `info` |
| `--log-format` | Output format | `text` |
| `--quiet` | Suppress output | `false` |

### Command Aliases

Create shell aliases for common operations:

```bash
# ~/.bashrc or ~/.zshrc
alias ds='dagu start'
alias dst='dagu status'
alias dr='dagu retry'
alias dd='dagu dry'
```

## Next Steps

- [Explore the REST API](/features/interfaces/api) for programmatic access
- [Set up the Web UI](/features/interfaces/web-ui) for visual monitoring
- [Learn workflow syntax](/writing-workflows/) to build complex DAGs