# CLI Reference

Commands accept either DAG names (from YAML `name` field) or file paths.

- **Both formats**: `start`, `stop`, `status`, `retry`
- **File path only**: `dry`, `enqueue`
- **DAG name only**: `restart`

## Global Options

```bash
dagu [global options] command [command options] [arguments...]
```

- `--config, -c` - Config file (default: `~/.config/dagu/config.yaml`)
- `--quiet, -q` - Suppress output
- `--cpu-profile` - Enable CPU profiling
- `--help, -h` - Show help
- `--version, -v` - Print version

## Commands

### `start`

Run a DAG workflow.

```bash
dagu start [options] DAG_NAME_OR_FILE [-- PARAMS...]
```

**Options:**
- `--params, -p` - Parameters as JSON
- `--run-id, -r` - Custom run ID
- `--no-queue, -n` - Execute immediately

```bash
# Basic run
dagu start my-workflow.yaml

# With parameters (note the -- separator)
dagu start etl.yaml -- DATE=2024-01-01 ENV=prod

# Custom run ID
dagu start --run-id batch-001 etl.yaml
```

### `stop`

Stop a running DAG.

```bash
dagu stop [options] DAG_NAME_OR_FILE
```

**Options:**
- `--run-id, -r` - Specific run ID (optional)

```bash
dagu stop my-workflow                     # Stop current run
dagu stop --run-id=20240101_120000 etl   # Stop specific run
```

### `restart`

Restart a DAG run with a new ID.

```bash
dagu restart [options] DAG_NAME
```

**Options:**
- `--run-id, -r` - Run to restart (optional)

```bash
dagu restart my-workflow                  # Restart latest
dagu restart --run-id=20240101_120000 etl # Restart specific
```

### `retry`

Retry a failed DAG execution.

```bash
dagu retry [options] DAG_NAME_OR_FILE
```

**Options:**
- `--run-id, -r` - Run to retry (required)

```bash
dagu retry --run-id=20240101_120000 my-workflow
```

### `status`

Display current status of a DAG.

```bash
dagu status [options] DAG_NAME_OR_FILE
```

**Options:**
- `--run-id, -r` - Check specific run (optional)

```bash
dagu status my-workflow  # Latest run status
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
- `--host, -s` - Host (default: localhost)
- `--port, -p` - Port (default: 8080)
- `--dags, -d` - DAGs directory

```bash
dagu server                               # Default settings
dagu server --host=0.0.0.0 --port=9000  # Custom host/port
```

### `scheduler`

Start the DAG scheduler daemon.

```bash
dagu scheduler [options]
```

**Options:**
- `--dags, -d` - DAGs directory

```bash
dagu scheduler                  # Default settings
dagu scheduler --dags=/opt/dags # Custom directory
```

### `start-all`

Start both scheduler and web UI.

```bash
dagu start-all [options]
```

**Options:**
- `--host, -s` - Host (default: localhost)
- `--port, -p` - Port (default: 8080)
- `--dags, -d` - DAGs directory

```bash
dagu start-all                           # Default settings
dagu start-all --host=0.0.0.0 --port=9000 # Production mode
```

### `dry`

Validate a DAG without executing it.

```bash
dagu dry [options] DAG_FILE [-- PARAMS...]
```

```bash
dagu dry my-workflow.yaml
dagu dry etl.yaml -- DATE=2024-01-01  # With parameters
```

### `enqueue`

Add a DAG to the execution queue.

```bash
dagu enqueue [options] DAG_FILE [-- PARAMS...]
```

**Options:**
- `--run-id, -r` - Custom run ID
- `--params, -p` - Parameters as JSON

```bash
dagu enqueue my-workflow.yaml
dagu enqueue --run-id=batch-001 etl.yaml -- TYPE=daily
```

### `dequeue`

Remove a DAG from the execution queue.

```bash
dagu dequeue --dag-run=<dag-name>:<run-id>
```

```bash
dagu dequeue --dag-run=my-workflow:batch-001
```

### `version`

Display version information.

```bash
dagu version
```

### `migrate`

Migrate legacy data to new format.

```bash
dagu migrate history  # Migrate v1.16 -> v1.17+ format
```

## Configuration

Priority: CLI flags > Environment variables > Config file

### Key Environment Variables

- `DAGU_HOST` - Server host (default: `127.0.0.1`)
- `DAGU_PORT` - Server port (default: `8080`)
- `DAGU_DAGS_DIR` - DAGs directory
- `DAGU_LOG_DIR` - Log directory
- `DAGU_DATA_DIR` - Data directory
- `DAGU_AUTH_BASIC_USERNAME` - Basic auth username
- `DAGU_AUTH_BASIC_PASSWORD` - Basic auth password

## Exit Codes

- `0` - Success
- `1` - General error
- `2` - Invalid arguments
- `3` - DAG not found
- `4` - Validation failed
- `5` - Execution failed
- `130` - Interrupted (SIGINT)
- `143` - Terminated (SIGTERM)

## Common Usage

```bash
# Development
dagu dry my-workflow.yaml        # Validate
dagu start my-workflow.yaml      # Run
dagu status my-workflow          # Check

# Production
dagu start-all --host=0.0.0.0   # Start services
systemctl status dagu            # Monitor
journalctl -u dagu -f           # View logs

# Debugging
dagu status failed-workflow      # Check failure
dagu retry --run-id=xyz failed  # Retry run
```
