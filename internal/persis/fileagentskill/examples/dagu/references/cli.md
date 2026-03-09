# Dagu CLI Reference

Global flags on all commands: `--config/-c`, `--dagu-home`, `--quiet/-q`, `--cpu-profile`

## Commands

### dagu start

Execute a DAG.

```
dagu start [flags] <dag> [-- params...]
```

Flags:
- `--params/-p` — Parameters (key=value or positional)
- `--name/-N` — Override DAG name
- `--run-id/-r` — Custom run ID
- `--from-run-id` — Use a previous run as template for a new run
- `--tags` — Additional tags (comma-separated key=value or key-only)
- `--default-working-dir` — Default working directory for DAGs without explicit workingDir
- `--worker-id` — Worker ID for distributed execution (defaults to `local`)
- `--trigger-type` — How this run was initiated: `manual` (default), `scheduler`, `webhook`, `subdag`, `retry`, `catchup`
- `--parent` — Parent dag-run reference (sub dag-runs only)
- `--root` — Root dag-run reference (sub dag-runs only)

### dagu exec

Run a one-off command as a DAG run without a YAML file.

```
dagu exec [flags] -- <command> [args...]
```

Flags:
- `--name/-N` — Name for the run
- `--run-id/-r` — Custom run ID
- `--workdir` — Working directory (default: current directory)
- `--shell` — Override shell binary
- `--base` — Path to a base DAG YAML whose defaults are applied
- `--env/-E` — Environment variable (KEY=VALUE), repeatable
- `--dotenv` — Path to a dotenv file, repeatable
- `--worker-label` — Worker label selector (key=value) for distributed execution, repeatable

### dagu enqueue

Enqueue a DAG run for later execution.

```
dagu enqueue [flags] <dag> [-- params...]
```

Flags:
- `--params/-p` — Parameters (key=value or positional)
- `--name/-N` — Override DAG name
- `--queue/-u` — Override the DAG-level queue definition
- `--run-id/-r` — Custom run ID
- `--tags` — Additional tags (comma-separated)
- `--default-working-dir` — Default working directory for DAGs without explicit workingDir
- `--trigger-type` — Trigger type (default: `manual`)

### dagu dequeue

Dequeue a DAG run from the specified queue. Marks the dequeued run as aborted.

```
dagu dequeue <queue-name>
```

Flags:
- `--dag-run/-d` — DAG run identifier (format: `<dag-name>:<run-id>`). If omitted, dequeues the first item in the queue.
- `--params/-p` — Parameters

### dagu stop

Stop an active DAG run.

```
dagu stop <dag-name>
```

Flags:
- `--run-id/-r` — Run ID to stop (if omitted, stops the currently active run)

### dagu restart

Stop and restart a DAG run with a new run ID.

```
dagu restart <dag-name>
```

Flags:
- `--run-id/-r` — Run ID to restart

### dagu retry

Retry a previous DAG run using the same run ID.

```
dagu retry [flags] <dag>
```

Flags:
- `--run-id/-r` — Run ID to retry (required)
- `--step` — Retry only the specified step
- `--worker-id` — Worker ID (defaults to `local`)

### dagu dry

Dry-run a DAG without executing commands.

```
dagu dry [flags] <dag> [-- params...]
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
- `--sub-run-id/-s` — Sub-run ID (requires `--run-id`)

### dagu history

Show DAG run history.

```
dagu history [dag-name]
```

Flags:
- `--from` — Start date/time in UTC (format: `2006-01-02` or `2006-01-02T15:04:05Z`)
- `--to` — End date/time in UTC (same formats as `--from`)
- `--last` — Relative time period (e.g. `7d`, `24h`, `1w`). Cannot combine with `--from`/`--to`
- `--status` — Filter by status: `running`, `succeeded`, `failed`, `aborted`, `queued`, `waiting`, `rejected`, `not_started`, `partially_succeeded`
- `--run-id` — Filter by run ID (partial match supported)
- `--tags` — Filter by tags (comma-separated, AND logic)
- `--format/-f` — Output format: `table` (default), `json`, `csv`
- `--limit/-l` — Max results (default 100, max 1000)

Default: shows runs from the last 30 days, newest first.

### dagu cleanup

Remove old DAG run history. Active runs are never deleted.

```
dagu cleanup <dag-name>
```

Flags:
- `--retention-days` — Days to retain (default 0 = delete all except active)
- `--dry-run` — Preview what would be deleted
- `--yes/-y` — Skip confirmation

### dagu server

Start web UI + REST API.

```
dagu server [flags]
```

Flags:
- `--host/-s` — Host (default `localhost`)
- `--port/-p` — Port (default `8080`)
- `--dags/-d` — DAGs directory
- `--tunnel/-t` — Enable tunnel mode for remote access
- `--tunnel-token` — Tailscale auth key for headless authentication
- `--tunnel-funnel` — Enable Tailscale Funnel for public internet access
- `--tunnel-https` — Use HTTPS for Tailscale

### dagu scheduler

Start cron scheduler. Monitors DAG definitions and triggers runs based on schedule expressions. Also processes queued runs.

```
dagu scheduler [flags]
```

Flags:
- `--dags/-d` — DAGs directory

### dagu coordinator

Start gRPC coordinator for distributed execution.

```
dagu coordinator [flags]
```

Flags:
- `--coordinator.host/-H` — Host (default `127.0.0.1`)
- `--coordinator.port/-P` — Port (default `50055`)
- `--coordinator.advertise/-A` — Advertise address for service registry (default: auto-detected hostname)
- `--peer.insecure` — Use insecure connection (h2c) instead of TLS
- `--peer.cert-file` — Path to TLS certificate file for mutual TLS
- `--peer.key-file` — Path to TLS key file for mutual TLS
- `--peer.client-ca-file` — Path to CA certificate file for client verification (mTLS)
- `--peer.skip-tls-verify` — Skip TLS certificate verification

### dagu worker

Start distributed worker. Connects to coordinator and polls for tasks.

```
dagu worker [flags]
```

Flags:
- `--worker.id/-w` — Worker instance ID (default: `hostname@PID`)
- `--worker.max-active-runs/-m` — Max concurrent runs (default 100)
- `--worker.labels/-l` — Worker labels for capability matching (format: `key1=value1,key2=value2`)
- `--worker.coordinators` — Coordinator addresses for static discovery (format: `host1:port1,host2:port2`). When set, uses shared-nothing mode (no shared filesystem needed).
- `--peer.insecure` — Use insecure connection (h2c) instead of TLS
- `--peer.cert-file` — Path to TLS certificate file for mutual TLS
- `--peer.key-file` — Path to TLS key file for mutual TLS
- `--peer.client-ca-file` — Path to CA certificate file for server verification
- `--peer.skip-tls-verify` — Skip TLS certificate verification

### dagu start-all

Start server + scheduler + optionally coordinator in one process.

```
dagu start-all [flags]
```

The coordinator is enabled by default. Disable with `coordinator.enabled=false` in config or `DAGU_COORDINATOR_ENABLED=false`.

Flags:
- `--host/-s` — Web server host (default `localhost`)
- `--port/-p` — Web server port (default `8080`)
- `--dags/-d` — DAGs directory
- `--coordinator.host/-H` — Coordinator gRPC host (default `127.0.0.1`)
- `--coordinator.port/-P` — Coordinator gRPC port (default `50055`)
- `--coordinator.advertise/-A` — Advertise address for service registry
- `--peer.insecure` — Use insecure connection (h2c) instead of TLS
- `--peer.cert-file` — Path to TLS certificate file
- `--peer.key-file` — Path to TLS key file
- `--peer.client-ca-file` — Path to CA certificate file for client verification (mTLS)
- `--peer.skip-tls-verify` — Skip TLS certificate verification

### dagu ai

AI coding tool integrations.

#### dagu ai install

Install the Dagu DAG authoring skill into detected AI coding tools.

```
dagu ai install [--yes/-y]
```

Supported tools: Claude Code, Codex, OpenCode, Gemini CLI, Copilot CLI.

Flags:
- `--yes/-y` — Install to all detected tools without prompting

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

Self-update binary. Cannot be used if installed via Homebrew, Snap, `go install`, or Docker.

```
dagu upgrade [flags]
```

Flags:
- `--check` — Only check for updates
- `--version/-v` — Target version (e.g. `v1.30.0`)
- `--dry-run` — Show what would happen
- `--backup` — Backup current binary before upgrade
- `--yes/-y` — Skip confirmation
- `--force/-f` — Allow downgrade
- `--pre-release` — Include pre-release versions

### dagu license

Manage license.

```
dagu license <activate|deactivate|check>
```

### dagu sync

Git sync operations for DAG definitions.

```
dagu sync <subcommand>
```

#### dagu sync status

Show Git sync status (repository, branch, per-DAG status).

```
dagu sync status
```

#### dagu sync pull

Pull changes from remote repository.

```
dagu sync pull
```

#### dagu sync publish

Publish local changes to remote repository.

```
dagu sync publish [dag-name]
```

Flags:
- `--message/-m` — Commit message
- `--all` — Publish all modified DAGs (cannot combine with dag-name argument)
- `--force/-f` — Force publish even with conflicts

#### dagu sync discard

Discard local changes for a DAG and restore the remote version.

```
dagu sync discard <dag-name>
```

Flags:
- `--yes/-y` — Skip confirmation

#### dagu sync forget

Remove state entries for missing/untracked/conflict sync items without deleting from remote.

```
dagu sync forget <item-id> [item-id...]
```

Flags:
- `--yes/-y` — Skip confirmation

#### dagu sync cleanup

Remove all missing entries from sync state.

```
dagu sync cleanup
```

Flags:
- `--yes/-y` — Skip confirmation
- `--dry-run` — Show what would be cleaned up

#### dagu sync delete

Delete a sync item from remote repository, local disk, and sync state.

```
dagu sync delete <item-id>
```

Flags:
- `--message/-m` — Commit message
- `--force` — Force delete even with local modifications
- `--all-missing` — Delete all missing items (cannot combine with item-id argument)
- `--dry-run` — Show what would be deleted
- `--yes/-y` — Skip confirmation

#### dagu sync mv

Atomically rename a sync item across local filesystem, remote repository, and sync state.

```
dagu sync mv <old-id> <new-id>
```

Flags:
- `--message/-m` — Commit message
- `--force` — Force move even with conflicts
- `--dry-run` — Show what would be moved
- `--yes/-y` — Skip confirmation
