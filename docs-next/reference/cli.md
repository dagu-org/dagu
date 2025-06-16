# CLI Reference

Complete reference for all Dagu command-line interface commands.

## Important: DAG Names vs File Paths

Commands accept either **DAG names** or **DAG file paths**:

- **DAG Name**: The `name` field from the YAML file (e.g., `my-workflow`)
- **DAG File Path**: Path to the YAML file (e.g., `./workflows/my-workflow.yaml`)

**Commands that accept both:**
- `start`, `stop`, `status`, `retry` - Can use either DAG name or file path

**Commands that require file paths:**
- `dry`, `enqueue` - Must provide the actual YAML file

**Commands that require DAG names:**
- `restart` - Works with existing DAG runs, use DAG name only

## Global Options

These options can be used with any command:

```bash
dagu [global options] command [command options] [arguments...]
```

| Option | Description | Default |
|--------|-------------|---------|
| `--config, -c` | Load configuration from FILE | `~/.config/dagu/config.yaml` |
| `--quiet, -q` | Suppress output during execution | `false` |
| `--cpu-profile` | Enable CPU profiling (saves to cpu.pprof) | `false` |
| `--help, -h` | Show help | |
| `--version, -v` | Print version | |

## Commands

### `start`

Run a DAG workflow.

```bash
dagu start [options] DAG_NAME_OR_FILE [-- PARAMS...]
```

**Options:**
- `--params, -p` - Specify parameters (supports key=value pairs)
- `--run-id, -r` - Specify a custom run ID
- `--no-queue, -n` - Do not queue the DAG run, execute immediately

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
dagu stop [options] DAG_NAME_OR_FILE
```

**Options:**
- `--run-id, -r` - Unique identifier of the DAG run to stop (optional)

**Examples:**
```bash
# Stop currently running DAG
dagu stop my-workflow

# Stop specific DAG run
dagu stop --run-id=20240101_120000 my-workflow

# Can also use file path
dagu stop my-workflow.yaml
```

### `restart`

Restart a DAG run with a new ID.

```bash
dagu restart [options] DAG_NAME
```

**Options:**
- `--run-id, -r` - Unique identifier of the DAG run to restart (optional)

**Examples:**
```bash
# Restart latest DAG run
dagu restart my-workflow

# Restart specific DAG run
dagu restart --run-id=20240101_120000 my-workflow
```

### `retry`

Retry a failed DAG execution.

```bash
dagu retry [options] DAG_NAME_OR_FILE
```

**Options:**
- `--run-id, -r` - Unique identifier of the DAG run to retry (required)

**Examples:**
```bash
# Retry specific run using DAG name
dagu retry --run-id=20240101_120000 my-workflow

# Retry specific run using file path
dagu retry --run-id=20240101_120000 my-workflow.yaml
```

### `status`

Display current status of a DAG.

```bash
dagu status [options] DAG_NAME_OR_FILE
```

**Options:**
- `--run-id, -r` - Unique identifier of the DAG run to check (optional)

**Examples:**
```bash
# Show status of latest DAG run
dagu status my-workflow

# Show status of specific DAG run
dagu status --run-id=20240101_120000 my-workflow

# Can also use file path
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
- `--host, -s` - Server hostname or IP address (default: localhost)
- `--port, -p` - Server port number (default: 8080)
- `--dags, -d` - Directory containing DAG files (default: ~/.config/dagu/dags)

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
- `--dags, -d` - Directory containing DAG files (default: ~/.config/dagu/dags)

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
- `--host, -s` - Server hostname or IP address (default: localhost)
- `--port, -p` - Server port number (default: 8080)
- `--dags, -d` - Directory containing DAG files (default: ~/.config/dagu/dags)

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
dagu dry [options] DAG_FILE [-- PARAMS...]
```

**Options:**
- `--params, -p` - Specify parameters for validation

**Examples:**
```bash
dagu dry my-workflow.yaml
# With parameters (note the -- separator)
dagu dry complex-etl.yaml -- DATE=2024-01-01
# With params flag
dagu dry --params "DATE=2024-01-01" complex-etl.yaml
```

### `enqueue`

Add a DAG to the execution queue.

```bash
dagu enqueue [options] DAG_FILE [-- PARAMS...]
```

**Options:**
- `--run-id, -r` - Specify a custom run ID
- `--params, -p` - Specify parameters for the DAG run

**Examples:**
```bash
# Add to queue
dagu enqueue my-workflow.yaml

# Add with custom ID
dagu enqueue --run-id=custom-001 my-workflow.yaml

# Add with parameters
dagu enqueue my-workflow.yaml -- KEY=value

# Add with params flag
dagu enqueue --params "KEY=value" my-workflow.yaml
```

### `dequeue`

Remove a DAG from the execution queue.

```bash
dagu dequeue [options]
```

**Options:**
- `--dag-run, -d` - Specify DAG name and run ID as `<dag-name>:<run-id>` (required)
- `--params, -p` - Parameters (not typically used)

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

### `migrate`

Migrate legacy data to new format.

```bash
dagu migrate history
```

**Subcommands:**
- `history` - Migrate DAG run history from v1.16 format to v1.17+ format

**Examples:**
```bash
# Migrate historical data after upgrade
dagu migrate history
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