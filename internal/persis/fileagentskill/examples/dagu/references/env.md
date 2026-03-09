# Environment Variables

## Execution Variables

Set automatically during DAG execution:

| Variable | Description |
|----------|-------------|
| `DAG_NAME` | Name of the executing DAG |
| `DAG_RUN_ID` | Unique run identifier |
| `DAG_RUN_LOG_FILE` | Path to run log file |
| `DAG_RUN_WORK_DIR` | Per-run working directory |
| `DAG_RUN_STEP_NAME` | Current step name |
| `DAG_RUN_STEP_STDOUT_FILE` | Path to step stdout file |
| `DAG_RUN_STEP_STDERR_FILE` | Path to step stderr file |
| `DAG_RUN_STATUS` | Current run status (available in handlers) |
| `DAG_WAITING_STEPS` | Comma-separated waiting steps (available in handlers) |
| `DAG_DOCS_DIR` | Documentation directory |
| `DAGU_PARAMS_JSON` | Resolved parameters as JSON |
| `DAG_PARAMS_JSON` | Same (backward compat) |

## Configuration Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DAGU_HOME` | `~/.dagu` | Base directory for all Dagu data |
| `DAGU_DAGS_DIR` | `$DAGU_HOME/dags` | DAG YAML files directory |
| `DAGU_LOG_DIR` | `$DAGU_HOME/logs` | Log files directory |
| `DAGU_DATA_DIR` | `$DAGU_HOME/data` | Data storage directory |
| `DAGU_HOST` | `localhost` | Server host |
| `DAGU_PORT` | `8080` | Server port |
| `DAGU_BASE_PATH` | `/` | URL base path |
| `DAGU_TZ` | system | Timezone for schedules |
| `DAGU_AUTH_BASIC_ENABLED` | `false` | Enable basic auth |
| `DAGU_AUTH_BASIC_USERNAME` | — | Basic auth username |
| `DAGU_AUTH_BASIC_PASSWORD` | — | Basic auth password |
| `DAGU_AUTH_TOKEN_ENABLED` | `false` | Enable token auth |
| `DAGU_AUTH_TOKEN_VALUE` | — | Auth token value |
| `DAGU_TLS_CERT_FILE` | — | TLS certificate path |
| `DAGU_TLS_KEY_FILE` | — | TLS key path |
| `DAGU_LOG_FORMAT` | `text` | Log format: `text`, `json` |
| `DAGU_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |

All paths derive from `DAGU_HOME` unless explicitly overridden.
