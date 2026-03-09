# Dagu CLI Reference

Global flags on all commands: `--config/-c`, `--dagu-home`, `--quiet/-q`, `--cpu-profile`

## Commands

### dagu start

Execute a DAG.

```
dagu start <dag> [-- params...]
```

Flags:
- `--params/-p` — Parameters (key=value)
- `--name/-N` — Override DAG name
- `--run-id/-r` — Custom run ID
- `--from-run-id` — Resume from previous run
- `--tags` — Tags for the run
- `--worker-id` — Worker ID for distributed execution
- `--trigger-type` — Trigger type identifier

### dagu exec

Run a one-off command as a DAG run without a YAML file.

```
dagu exec -- <command> [args...]
```

Flags:
- `--name/-N` — Name for the run
- `--run-id/-r` — Custom run ID
- `--workdir` — Working directory
- `--shell` — Shell to use
- `--base` — Base DAG file for defaults
- `--env/-E` — Environment variables
- `--dotenv` — Path to .env file
- `--worker-label` — Worker label

### dagu enqueue

Enqueue a DAG run for later execution.

```
dagu enqueue <dag> [-- params...]
```

Flags:
- `--params/-p` — Parameters
- `--queue/-u` — Queue name
- `--run-id/-r` — Custom run ID
- `--tags` — Tags
- `--trigger-type` — Trigger type

### dagu dequeue

Dequeue a DAG run.

```
dagu dequeue <queue-name>
```

Flags:
- `--dag-run/-d` — DAG run identifier (format: `<dag-name>:<run-id>`)

### dagu stop

Stop an active DAG run.

```
dagu stop <dag-name>
```

Flags:
- `--run-id/-r` — Run ID to stop

### dagu restart

Stop and restart a DAG run.

```
dagu restart <dag-name>
```

Flags:
- `--run-id/-r` — Run ID to restart

### dagu retry

Retry a previous DAG run.

```
dagu retry <dag>
```

Flags:
- `--run-id/-r` — Run ID to retry (required)
- `--step` — Specific step to retry
- `--worker-id` — Worker ID

### dagu dry

Dry-run a DAG without executing commands.

```
dagu dry <dag> [-- params...]
```

Flags:
- `--params/-p` — Parameters
- `--name/-N` — Override name

### dagu validate

Validate DAG YAML without executing.

```
dagu validate <dag>
```

### dagu status

Show DAG run status.

```
dagu status <dag-name>
```

Flags:
- `--run-id/-r` — Specific run ID
- `--sub-run-id/-s` — Sub-run ID

### dagu history

Show DAG run history.

```
dagu history [dag-name]
```

Flags:
- `--from` — Start date
- `--to` — End date
- `--last` — Duration (e.g. `7d`)
- `--status` — Filter by status
- `--run-id` — Filter by run ID
- `--tags` — Filter by tags
- `--format/-f` — Output format: `table`, `json`, `csv`
- `--limit/-l` — Max results (default 100, max 1000)

### dagu cleanup

Remove old DAG run history.

```
dagu cleanup <dag-name>
```

Flags:
- `--retention-days` — Days to retain (default 0)
- `--dry-run` — Show what would be removed
- `--yes/-y` — Skip confirmation

### dagu server

Start web UI + REST API.

```
dagu server
```

Flags:
- `--host/-s` — Host (default `localhost`)
- `--port/-p` — Port (default `8080`)
- `--dags/-d` — DAGs directory
- `--tunnel/-t` — Enable tunnel
- `--tunnel-token` — Tunnel auth token
- `--tunnel-funnel` — Funnel mode
- `--tunnel-https` — HTTPS tunnel

### dagu scheduler

Start cron scheduler.

```
dagu scheduler
```

Flags:
- `--dags/-d` — DAGs directory

### dagu coordinator

Start gRPC coordinator for distributed execution.

```
dagu coordinator
```

Flags:
- `--coordinator.host/-H` — Host (default `127.0.0.1`)
- `--coordinator.port/-P` — Port (default `50055`)
- `--coordinator.advertise/-A` — Advertise address
- Peer TLS flags for mTLS

### dagu worker

Start distributed worker.

```
dagu worker
```

Flags:
- `--worker.id/-w` — Worker ID
- `--worker.max-active-runs/-m` — Max concurrent runs (default 100)
- `--worker.labels/-l` — Worker labels
- `--worker.coordinators` — Coordinator addresses
- Peer TLS flags for mTLS

### dagu start-all

Start server + scheduler + coordinator in one process.

```
dagu start-all
```

Combines server and coordinator flags.

### dagu schema

Show JSON schema documentation.

```
dagu schema <dag|config> [path]
```

Examples:
- `dagu schema dag` — Show all DAG root-level fields
- `dagu schema dag steps` — Show step properties
- `dagu schema config server` — Show server config

### dagu example

Show example DAGs.

```
dagu example [id]
```

12 built-in examples: `parallel-steps`, `output-passing`, `schedule-params-env`, `defaults-and-retry`, `preconditions`, `lifecycle-hooks`, `http-requests`, `docker-container`, `sub-dag`, `conditional-routing`, `approval-gate`, `agent-step`

### dagu config

Show resolved configuration paths.

```
dagu config
```

### dagu version

Show version.

```
dagu version
```

### dagu migrate

Migrate data formats.

```
dagu migrate history
```

Migrates history from v1.16 to v1.17+ format.

### dagu upgrade

Self-update binary.

```
dagu upgrade
```

Flags:
- `--check` — Only check for updates
- `--version/-v` — Target version (e.g. `v1.30.0`)
- `--dry-run` — Show what would happen
- `--backup` — Backup current binary
- `--yes/-y` — Skip confirmation
- `--force/-f` — Allow downgrade
- `--pre-release` — Include pre-releases

### dagu license

Manage license.

```
dagu license <activate|deactivate|check>
```

### dagu sync

Git sync operations.

```
dagu sync <subcommand>
```

Subcommands: `status`, `pull`, `publish`, `discard`, `forget`, `cleanup`, `delete`, `mv`
