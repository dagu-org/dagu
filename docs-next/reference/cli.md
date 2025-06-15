# CLI Reference

Complete reference for all Dagu command-line interface commands.

## Global Options

These options can be used with any command:

```bash
dagu [global options] command [command options] [arguments...]
```

| Option | Description | Default |
|--------|-------------|---------|
| `--config FILE` | Load configuration from FILE | `~/.config/dagu/config.yaml` |
| `--log-level LEVEL` | Set log level (debug, info, warn, error) | `info` |
| `--log-format FORMAT` | Set log format (text, json) | `text` |
| `--help, -h` | Show help | |
| `--version, -v` | Print version | |

## Commands

### `start`

Run a DAG workflow.

```bash
dagu start [options] DAG_FILE [PARAMS...]
```

**Options:**
- `--params` - Specify parameters as a JSON string
- `--run-id ID` - Specify a custom run ID
- `--parent` - Parent DAG run ID
- `--root` - Root DAG run ID
- `--no-queue` - Bypass the queue

**Examples:**
```bash
# Run a workflow
dagu start my-workflow.yaml

# Run with named parameters (note the -- separator)
dagu start etl.yaml -- DATE=2024-01-01 ENV=prod

# Run with positional parameters
dagu start my-workflow.yaml -- value1 value2 value3

# Run with custom ID
dagu start --run-id custom-001 my-workflow.yaml

# Run with JSON parameters
dagu start --params '{"date":"2024-01-01","env":"prod"}' etl.yaml
```

### `stop`

Stop a running DAG.

```bash
dagu stop DAG_FILE
```

**Examples:**
```bash
dagu stop my-workflow.yaml
```

### `restart`

Restart a running DAG.

```bash
dagu restart DAG_FILE
```

**Options:**
- `--params KEY=VALUE` - Override parameters for restart

**Examples:**
```bash
dagu restart my-workflow.yaml
dagu restart etl.yaml DATE=2024-01-02
```

### `retry`

Retry a failed DAG execution.

```bash
dagu retry [options] DAG_FILE
```

**Options:**
- `--request-id ID` - Retry specific request ID
- `--params KEY=VALUE` - Override parameters

**Examples:**
```bash
# Retry last failed run
dagu retry my-workflow.yaml

# Retry specific run
dagu retry --request-id=20240101_120000 my-workflow.yaml
```

### `status`

Display current status of a DAG.

```bash
dagu status DAG_FILE
```

**Examples:**
```bash
dagu status my-workflow.yaml
```

**Output:**
```
Status: running
Started: 2024-01-01 12:00:00
Steps:
  ✓ download     [completed]
  ⟳ process      [running]
  ○ upload       [pending]
```


### `server`

Start the web UI server.

```bash
dagu server [options]
```

**Options:**
- `--host HOST` - Host to bind (default: 127.0.0.1)
- `--port PORT` - Port to bind (default: 8080)
- `--dags DIR` - Directory containing DAG files

**Examples:**
```bash
# Start server with default settings
dagu server

# Custom host and port
dagu server --host=0.0.0.0 --port=9000

# Use custom DAGs directory
dagu server --dags=/path/to/directory
```

### `scheduler`

Start the DAG scheduler daemon.

```bash
dagu scheduler [options]
```

**Options:**
- `--dags DIR` - Directory containing DAG files

**Examples:**
```bash
# Start scheduler with default settings
dagu scheduler

# Use custom DAGs directory
dagu scheduler --dags=/path/to/directory
```

### `start-all`

Start both scheduler and web UI.

```bash
dagu start-all [options]
```

**Options:**
- `--host HOST` - Host to bind (default: 127.0.0.1)
- `--port PORT` - Port to bind (default: 8080)
- `--dags DIR` - Directory containing DAG files

**Examples:**
```bash
# Start with defaults
dagu start-all

# Custom host and port
dagu start-all --host 0.0.0.0 --port 9000

# Custom DAGs directory
dagu start-all --dags=/path/to/directory
```

### `dry`

Validate a DAG without executing it.

```bash
dagu dry DAG_FILE [-- PARAMS...]
```

**Examples:**
```bash
dagu dry my-workflow.yaml
# With parameters (note the -- separator)
dagu dry complex-etl.yaml -- DATE=2024-01-01
```

### `enqueue`

Add a DAG to the execution queue.

```bash
dagu enqueue [options] DAG_FILE [-- PARAMS...]
```

**Options:**
- `--run-id ID` - Specify a custom run ID

**Examples:**
```bash
# Add to queue
dagu enqueue my-workflow.yaml

# Add with custom ID
dagu enqueue --run-id=custom-001 my-workflow.yaml

# Add with parameters
dagu enqueue my-workflow.yaml -- KEY=value
```

### `dequeue`

Remove a DAG from the execution queue.

```bash
dagu dequeue DAG_FILE
```

**Options:**
- `--dag-run` - Specify DAG name and run ID as `<dag-name>:<run-id>`

**Examples:**
```bash
# Remove specific run from queue
dagu dequeue --dag-run=my-workflow:custom-001
```

### `version`

Display version information.

```bash
dagu version
```

**Output:**
```
Dagu version: 1.14.0
Go version: go1.21.5
Git commit: abc123def
Built: 2024-01-01T00:00:00Z
```

## Configuration

Dagu can be configured via:

1. **Configuration file** (`~/.config/dagu/config.yaml`)
2. **Environment variables** (prefix: `DAGU_`)
3. **Command-line flags**

Priority: CLI flags > Environment variables > Config file

### Configuration File Example

```yaml
# ~/.config/dagu/config.yaml
host: 127.0.0.1
port: 8080
logDir: ~/.local/share/dagu/logs
dataDir: ~/.local/share/dagu/data
dags: ~/workflows

# Notification settings
smtp:
  host: smtp.gmail.com
  port: 587
  username: notifications@example.com

# UI settings
ui:
  navbarColor: "#00D9FF"
  navbarTitle: "My Workflows"
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DAGU_HOST` | Server host | `127.0.0.1` |
| `DAGU_PORT` | Server port | `8080` |
| `DAGU_DAGS` | DAGs directory | `~/.config/dagu/dags` |
| `DAGU_LOG_DIR` | Log directory | `~/.local/share/dagu/logs` |
| `DAGU_DATA_DIR` | Data directory | `~/.local/share/dagu/data` |
| `DAGU_BASE_CONFIG` | Base config file | `~/.config/dagu/base.yaml` |
| `DAGU_LOG_LEVEL` | Log level | `info` |
| `DAGU_LOG_FORMAT` | Log format | `text` |

## Exit Codes

| Code | Description |
|------|-------------|
| 0 | Success |
| 1 | General error |
| 2 | Invalid arguments |
| 3 | DAG not found |
| 4 | DAG validation failed |
| 5 | Execution failed |
| 130 | Interrupted (SIGINT) |
| 143 | Terminated (SIGTERM) |

## Common Patterns

### Development Workflow
```bash
# Edit DAG
vim my-workflow.yaml

# Dry run (validate without executing)
dagu dry my-workflow.yaml

# Run
dagu start my-workflow.yaml

# Check status
dagu status my-workflow.yaml
```

### Production Deployment
```bash
# Start services
dagu start-all --host=0.0.0.0 --port=8080

# Monitor via systemd
systemctl status dagu

# View logs
journalctl -u dagu -f
```

### Debugging Failed Workflows
```bash
# Check status
dagu status failed-workflow.yaml

# Retry
dagu retry failed-workflow.yaml
```