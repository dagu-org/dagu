# DAG YAML Schema

## Top-Level Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | filename | DAG name |
| `group` | string | — | Group for UI organization |
| `description` | string | — | Description |
| `tags` | map or array | — | Key-value tags (keys max 63 chars, values max 255 chars) |
| `type` | string | `chain` | Execution type: `chain` (sequential), `graph` (parallel/dependency-based) |
| `steps` | array or map | — | Step definitions |
| `schedule` | string or array | — | Cron expression(s) |
| `stop_schedule` | string | — | Cron to stop |
| `restart_schedule` | string | — | Cron to restart |
| `skip_if_successful` | bool | false | Skip scheduled run if already ran successfully |
| `catchup_window` | string | — | Duration (e.g. `6h`) — enables catch-up on scheduler restart |
| `overlap_policy` | string | `skip` | `skip` or `all` when new run triggered while one active |
| `env` | map or array | — | Environment variables. Array: `[{KEY: val}]`, Map: `{KEY: val}` |
| `params` | string or map | — | Parameters (positional, key=value, or JSON) |
| `default_params` | string | — | Default parameter values |
| `shell` | string or array | — | Default shell (e.g. `bash -e` or `["bash", "-e"]`) |
| `working_dir` | string | — | Working directory |
| `dotenv` | string or array | — | Path(s) to .env file(s) |
| `timeout_sec` | int | — | Max execution time (seconds) |
| `delay_sec` | int | — | Delay before first step |
| `restart_wait_sec` | int | — | Wait time before restart |
| `max_active_runs` | int | 1 | Max concurrent DAG runs |
| `max_active_steps` | int | — | Max concurrent steps per run |
| `max_clean_up_time_sec` | int | 5 | Max cleanup time |
| `max_output_size` | int | 1048576 (1MB) | Max step output capture (bytes) |
| `log_dir` | string | — | Log directory |
| `log_output` | string | `separate` | `separate` (.out/.err) or `merged` (.log) |
| `hist_retention_days` | int | 30 | History retention |
| `queue` | string | — | Queue name for this DAG |
| `preconditions` | array | — | DAG-level preconditions (`condition`, `expected`, `negate`) |
| `handler_on` | object | — | Event handlers: `init`, `success`, `failure`, `abort`, `exit`, `wait` (each is a step definition) |
| `smtp` | object | — | SMTP config: `host`, `port`, `username`, `password` |
| `error_mail` | object | — | Error mail: `from`, `to`, `prefix`, `attach_logs` |
| `info_mail` | object | — | Success mail (same structure) |
| `mail_on` | object | — | Mail triggers: `failure`, `success`, `wait` (booleans) |
| `container` | string or object | — | String: exec into existing container. Object: create new (see container fields) |
| `ssh` | object | — | Default SSH config: `user`, `host`, `port`, `key`, `password`, `shell`, `timeout`, `bastion` |
| `s3` | object | — | Default S3 config: `region`, `endpoint`, `bucket`, credentials |
| `redis` | object | — | Default Redis config: `url`/`host`/`port`, `password`, `db`, `mode`, TLS |
| `llm` | object | — | Default LLM config: `provider`, `model`, `system`, `temperature`, `max_tokens`, `tools` |
| `secrets` | array | — | Secret references: `{name, provider, key, options}` |
| `otel` | object | — | OpenTelemetry: `enabled`, `endpoint`, `headers`, `insecure`, `timeout` |
| `worker_selector` | string or map | — | `"local"` or label map for distributed execution |
| `registry_auths` | map | — | Docker registry auth per hostname |
| `defaults` | object | — | Default values for all steps: `continue_on`, `retry_policy`, `repeat_policy`, `timeout_sec`, `signal_on_stop`, `env`, `preconditions` |
| `run_config` | object | — | `disable_param_edit`, `disable_run_id_edit` |

## Step-Level Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | required | Step name |
| `id` | string | — | Unique step ID |
| `command` | string | — | Shell command |
| `script` | string | — | Script content |
| `shell` | string or array | — | Override shell |
| `working_dir` | string | — | Override working directory |
| `env` | map or array | — | Step environment variables |
| `timeout_sec` | int | — | Step timeout |
| `output` | string or object | — | Capture stdout to variable. String: `MY_VAR`. Object: `{name, key, omit}` |
| `stdout` | string | — | File to write stdout |
| `stderr` | string | — | File to write stderr |
| `depends` | string or array | — | Dependency step names. In `graph` mode only. |
| `continue_on` | string or object | — | Continue on: `"skipped"`, `"failed"`, or `{skipped, failed, exit_code: [codes], mark_success}` |
| `preconditions` | array | — | Step conditions: `{condition, expected, negate}` |
| `retry_policy` | object | — | `{limit, interval_sec, exit_code: [codes], backoff, max_interval_sec}` |
| `repeat_policy` | object | — | `{interval_sec, limit, condition, expected, exit_code, backoff}` |
| `signal_on_stop` | string | — | Signal on stop (e.g. `KILL`, `TERM`) |
| `mail_on_error` | bool | — | Send mail on step error |
| `container` | string or object | — | Step-level container config |
| `type` | string | — | Executor type override |
| `config` | map | — | Executor-specific config |
| `call` | string | — | Sub-DAG name (for `dag` executor) |
| `params` | string or map | — | Sub-DAG parameters |
| `parallel` | object or array | — | Parallel execution: `{items, max_concurrent}` (default max_concurrent: 10) |
| `approval` | object | — | Approval gate: `{prompt, input: [fields], required: [fields]}` |
| `llm` | object | — | LLM config for `chat` steps |
| `messages` | array | — | Chat messages: `[{role, content}]` |
| `agent` | object | — | Agent config: `{model, tools, skills, soul, memory, prompt, max_iterations, safe_mode}` |
| `value` | string | — | Router expression |
| `routes` | map | — | Router pattern→steps mappings |
| `worker_selector` | map | — | Labels for distributed execution |
