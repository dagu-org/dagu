# Environment Variables

## Execution Variables

Set automatically during DAG execution. Defined in `internal/core/exec/env.go`.

### Always Available (set for every step)

| Variable | Description |
|----------|-------------|
| `DAG_NAME` | Name of the executing DAG |
| `DAG_RUN_ID` | Unique run identifier |
| `DAG_RUN_LOG_FILE` | Path to the main log file for the DAG run |
| `DAG_RUN_STEP_NAME` | Name of the currently executing step |
| `DAG_RUN_STEP_STDOUT_FILE` | Path to the step's stdout log file |
| `DAG_RUN_STEP_STDERR_FILE` | Path to the step's stderr log file |

### Conditionally Set

| Variable | Condition | Description |
|----------|-----------|-------------|
| `DAG_RUN_WORK_DIR` | Only if a per-run working directory is configured | Path to the per-DAG-run working directory |
| `DAG_DOCS_DIR` | Only if `paths.docs_dir` is configured | Per-DAG docs directory (`{docs_dir}/{dag_name}`) |
| `DAGU_PARAMS_JSON` | Only if the DAG has parameters | Resolved parameters encoded as JSON |
| `DAG_PARAMS_JSON` | Only if the DAG has parameters | Same as `DAGU_PARAMS_JSON` (backward compatibility alias) |

### Handler-Only Variables

These are only available inside lifecycle handler steps, not during normal step execution.

| Variable | Handler Scope | Description |
|----------|---------------|-------------|
| `DAG_RUN_STATUS` | `onSuccess`, `onFailure`, `onCancel`, `onExit`, `onWait` | Current DAG run status (e.g., `success`, `failed`) |
| `DAG_WAITING_STEPS` | `onWait` only | Comma-separated list of step names that are waiting for approval |

## Configuration Variables

All configuration environment variables use the `DAGU_` prefix. They map to config keys via viper bindings in `internal/cmn/config/loader.go`.

### Paths

| Variable | Default | Description |
|----------|---------|-------------|
| `DAGU_HOME` | `~/.dagu` (legacy) or XDG dirs | Base directory for all Dagu data. When set, all paths use unified structure under this directory |
| `DAGU_DAGS_DIR` | `$DAGU_HOME/dags` | DAG YAML files directory |
| `DAGU_DAGS` | — | Legacy alias for `DAGU_DAGS_DIR` |
| `DAGU_LOG_DIR` | `$DAGU_HOME/logs` | Log files directory |
| `DAGU_DATA_DIR` | `$DAGU_HOME/data` | Data storage directory |
| `DAGU_ADMIN_LOG_DIR` | `$DAGU_HOME/logs/admin` | Admin logs directory |
| `DAGU_SUSPEND_FLAGS_DIR` | `$DAGU_HOME/suspend` | Suspend flags directory |
| `DAGU_BASE_CONFIG` | `$DAGU_HOME/base.yaml` | Base configuration file path |
| `DAGU_DAG_RUNS_DIR` | — | DAG runs history directory |
| `DAGU_PROC_DIR` | — | Process tracking directory |
| `DAGU_QUEUE_DIR` | — | Queue directory |
| `DAGU_SERVICE_REGISTRY_DIR` | — | Service registry directory |
| `DAGU_ALT_DAGS_DIR` | — | Alternative DAGs directory |
| `DAGU_DOCS_DIR` | — | Documentation directory |
| `DAGU_USERS_DIR` | — | Users directory (for builtin auth) |
| `DAGU_WORKSPACES_DIR` | — | Workspaces directory |
| `DAGU_EXECUTABLE` | — | Path to dagu executable |

### Server

| Variable | Default | Description |
|----------|---------|-------------|
| `DAGU_HOST` | `127.0.0.1` | Server bind address |
| `DAGU_PORT` | `8080` | Server port |
| `DAGU_BASE_PATH` | `""` (empty) | URL base path for reverse proxy setups |
| `DAGU_API_BASE_URL` | — | API base URL override |
| `DAGU_TZ` | system | Timezone for schedules |
| `DAGU_LOG_FORMAT` | `text` | Log format: `text` or `json` |
| `DAGU_ACCESS_LOG_MODE` | `all` | Access log mode: `all`, `non-public`, `none` |
| `DAGU_DEBUG` | `false` | Enable debug mode |
| `DAGU_HEADLESS` | `false` | Run in headless mode (no UI) |
| `DAGU_LATEST_STATUS_TODAY` | `false` | Show only today's latest status |
| `DAGU_SERVER_METRICS` | `private` | Metrics endpoint access: `public` or `private` |
| `DAGU_CACHE` | `normal` | Cache mode: `low`, `normal`, `high` |

### Core

| Variable | Default | Description |
|----------|---------|-------------|
| `DAGU_DEFAULT_SHELL` | — | Default shell for commands |
| `DAGU_SKIP_EXAMPLES` | `false` | Skip creating example DAGs |
| `DAGU_DEFAULT_EXECUTION_MODE` | `local` | Execution mode: `local` or `distributed` |

### Features

| Variable | Default | Description |
|----------|---------|-------------|
| `DAGU_TERMINAL_ENABLED` | `false` | Enable web terminal feature |
| `DAGU_AUDIT_ENABLED` | — | Enable audit logging |
| `DAGU_AUDIT_RETENTION_DAYS` | `7` | Audit log retention in days |
| `DAGU_SESSION_MAX_PER_USER` | `100` | Max agent sessions per user |
| `DAGU_QUEUE_ENABLED` | `true` | Enable queue system |

### Authentication

Auth mode is controlled by `DAGU_AUTH_MODE`. There is no `DAGU_AUTH_BASIC_ENABLED` or `DAGU_AUTH_TOKEN_ENABLED`.

| Variable | Default | Description |
|----------|---------|-------------|
| `DAGU_AUTH_MODE` | `builtin` | Auth mode: `none`, `basic`, `builtin` |
| `DAGU_AUTH_BASIC_USERNAME` | — | Basic auth username (requires `auth.mode=basic`) |
| `DAGU_AUTH_BASIC_PASSWORD` | — | Basic auth password (requires `auth.mode=basic`) |
| `DAGU_AUTH_TOKEN_SECRET` | — | JWT token secret (for builtin auth) |
| `DAGU_AUTH_TOKEN_TTL` | — | Token TTL (for builtin auth) |

### Authentication — OIDC

Available under builtin auth mode (`auth.mode=builtin`) with an OIDC provider configured.

| Variable | Default | Description |
|----------|---------|-------------|
| `DAGU_AUTH_OIDC_CLIENT_ID` | — | OIDC client ID |
| `DAGU_AUTH_OIDC_CLIENT_SECRET` | — | OIDC client secret |
| `DAGU_AUTH_OIDC_CLIENT_URL` | — | OIDC callback URL |
| `DAGU_AUTH_OIDC_ISSUER` | — | OIDC issuer URL |
| `DAGU_AUTH_OIDC_SCOPES` | — | OIDC scopes (e.g., `openid,profile,email`) |
| `DAGU_AUTH_OIDC_WHITELIST` | — | Email whitelist |
| `DAGU_AUTH_OIDC_AUTO_SIGNUP` | — | Auto signup for new OIDC users |
| `DAGU_AUTH_OIDC_ALLOWED_DOMAINS` | — | Allowed email domains |
| `DAGU_AUTH_OIDC_BUTTON_LABEL` | — | OIDC login button label |
| `DAGU_AUTH_OIDC_DEFAULT_ROLE` | — | Default role for OIDC users |
| `DAGU_AUTH_OIDC_GROUPS_CLAIM` | — | Groups claim field name |
| `DAGU_AUTH_OIDC_GROUP_MAPPINGS` | — | Group to role mappings |
| `DAGU_AUTH_OIDC_ROLE_ATTRIBUTE_PATH` | — | jq path for role extraction |
| `DAGU_AUTH_OIDC_ROLE_ATTRIBUTE_STRICT` | — | Strict role attribute validation |
| `DAGU_AUTH_OIDC_SKIP_ORG_ROLE_SYNC` | — | Skip role sync after first login |

### TLS

| Variable | Description |
|----------|-------------|
| `DAGU_CERT_FILE` | TLS certificate file path |
| `DAGU_KEY_FILE` | TLS key file path |

### UI

| Variable | Default | Description |
|----------|---------|-------------|
| `DAGU_UI_NAVBAR_TITLE` | `Dagu` | Navbar title |
| `DAGU_UI_NAVBAR_COLOR` | — | Navbar color (hex) |
| `DAGU_UI_MAX_DASHBOARD_PAGE_LIMIT` | `100` | Max dashboard entries (range: 1–1000) |
| `DAGU_UI_LOG_ENCODING_CHARSET` | system-dependent | Log encoding charset |
| `DAGU_UI_DAGS_SORT_FIELD` | `name` | Default DAGs list sort field |
| `DAGU_UI_DAGS_SORT_ORDER` | `asc` | Default DAGs list sort order |

### Scheduler

| Variable | Default | Description |
|----------|---------|-------------|
| `DAGU_SCHEDULER_PORT` | `8090` | Health check server port (0 to disable) |
| `DAGU_SCHEDULER_LOCK_STALE_THRESHOLD` | `30s` | Lock stale timeout |
| `DAGU_SCHEDULER_LOCK_RETRY_INTERVAL` | `5s` | Lock retry interval |
| `DAGU_SCHEDULER_ZOMBIE_DETECTION_INTERVAL` | `45s` | Zombie process detection interval (0 to disable) |

### Coordinator (Distributed Mode)

| Variable | Default | Description |
|----------|---------|-------------|
| `DAGU_COORDINATOR_ENABLED` | — | Enable coordinator |
| `DAGU_COORDINATOR_HOST` | `127.0.0.1` | Coordinator bind address |
| `DAGU_COORDINATOR_ADVERTISE` | `""` | Advertised address for worker discovery |
| `DAGU_COORDINATOR_PORT` | `50055` | gRPC port |

### Worker (Distributed Mode)

| Variable | Default | Description |
|----------|---------|-------------|
| `DAGU_WORKER_ID` | — | Worker identifier |
| `DAGU_WORKER_MAX_ACTIVE_RUNS` | `100` | Max concurrent DAG runs |
| `DAGU_WORKER_LABELS` | — | Capability labels (`key=value,key2=value2`) |
| `DAGU_WORKER_COORDINATORS` | — | Coordinator addresses to poll |
| `DAGU_WORKER_POSTGRES_POOL_MAX_OPEN_CONNS` | `25` | PostgreSQL pool max open connections |
| `DAGU_WORKER_POSTGRES_POOL_MAX_IDLE_CONNS` | `5` | PostgreSQL pool max idle connections |
| `DAGU_WORKER_POSTGRES_POOL_CONN_MAX_LIFETIME` | `300` | Connection max lifetime (seconds) |
| `DAGU_WORKER_POSTGRES_POOL_CONN_MAX_IDLE_TIME` | `60` | Connection max idle time (seconds) |

### Peer TLS (gRPC)

| Variable | Default | Description |
|----------|---------|-------------|
| `DAGU_PEER_CERT_FILE` | — | Peer TLS certificate |
| `DAGU_PEER_KEY_FILE` | — | Peer TLS key |
| `DAGU_PEER_CLIENT_CA_FILE` | — | Peer CA certificate |
| `DAGU_PEER_SKIP_TLS_VERIFY` | — | Skip TLS verification |
| `DAGU_PEER_INSECURE` | `true` | Use h2c instead of TLS |

### Monitoring

| Variable | Default | Description |
|----------|---------|-------------|
| `DAGU_MONITORING_RETENTION` | `24h` | Metrics retention duration |
| `DAGU_MONITORING_INTERVAL` | `5s` | Metrics collection interval |

### Tunnel (Tailscale)

| Variable | Default | Description |
|----------|---------|-------------|
| `DAGU_TUNNEL_ENABLED` | `false` | Enable Tailscale tunnel |
| `DAGU_TUNNEL` | — | Alias for `DAGU_TUNNEL_ENABLED` |
| `DAGU_TUNNEL_TAILSCALE_AUTH_KEY` | — | Tailscale auth key |
| `DAGU_TUNNEL_TAILSCALE_HOSTNAME` | `dagu` | Tailscale machine name |
| `DAGU_TUNNEL_TAILSCALE_FUNNEL` | — | Enable public funnel |
| `DAGU_TUNNEL_TAILSCALE_HTTPS` | — | Enable HTTPS for tailnet |
| `DAGU_TUNNEL_TAILSCALE_STATE_DIR` | — | Tailscale state directory |
| `DAGU_TUNNEL_ALLOW_TERMINAL` | — | Allow terminal access via tunnel |
| `DAGU_TUNNEL_ALLOWED_IPS` | — | IP allowlist |
| `DAGU_TUNNEL_RATE_LIMITING_ENABLED` | — | Enable rate limiting |
| `DAGU_TUNNEL_RATE_LIMITING_LOGIN_ATTEMPTS` | — | Login attempts threshold |
| `DAGU_TUNNEL_RATE_LIMITING_WINDOW_SECONDS` | — | Rate limiting time window |
| `DAGU_TUNNEL_RATE_LIMITING_BLOCK_DURATION_SECONDS` | — | Block duration after threshold |

### Git Sync

| Variable | Default | Description |
|----------|---------|-------------|
| `DAGU_GITSYNC_ENABLED` | `false` | Enable git sync |
| `DAGU_GITSYNC_REPOSITORY` | — | Repository URL |
| `DAGU_GITSYNC_BRANCH` | — | Branch name |
| `DAGU_GITSYNC_PATH` | — | Subdirectory path within repo |
| `DAGU_GITSYNC_PUSH_ENABLED` | — | Enable pushing changes |
| `DAGU_GITSYNC_AUTH_TYPE` | — | Auth type: `token` or `ssh` |
| `DAGU_GITSYNC_AUTH_TOKEN` | — | Personal access token |
| `DAGU_GITSYNC_AUTH_SSH_KEY_PATH` | — | SSH key file path |
| `DAGU_GITSYNC_AUTH_SSH_PASSPHRASE` | — | SSH key passphrase |
| `DAGU_GITSYNC_AUTOSYNC_ENABLED` | — | Enable automatic sync |
| `DAGU_GITSYNC_AUTOSYNC_ON_STARTUP` | — | Sync on startup |
| `DAGU_GITSYNC_AUTOSYNC_INTERVAL` | — | Auto sync interval (seconds) |
| `DAGU_GITSYNC_COMMIT_AUTHOR_NAME` | — | Git commit author name |
| `DAGU_GITSYNC_COMMIT_AUTHOR_EMAIL` | — | Git commit author email |

### License

| Variable | Description |
|----------|-------------|
| `DAGU_LICENSE_KEY` | License activation key |
| `DAGU_LICENSE_CLOUD_URL` | Cloud license service URL |

## Path Resolution

When `DAGU_HOME` is set, all paths use a **unified structure** under that directory:

```
$DAGU_HOME/
├── dags/          # DAG definitions
├── data/          # Application data
├── logs/          # Logs
│   └── admin/     # Admin logs
├── suspend/       # Suspend flags
└── base.yaml      # Base configuration
```

When `DAGU_HOME` is not set:
1. If `~/.dagu` exists (legacy), it is used as the unified root (with a deprecation warning).
2. Otherwise, XDG-compliant paths are used (`$XDG_CONFIG_HOME/dagu/`, `$XDG_DATA_HOME/dagu/`).

Individual path variables (e.g., `DAGU_DAGS_DIR`) override the defaults regardless of which resolution mode is active.
