# YAML Specification

## Overview

Dagu workflows are defined using YAML files. Each file represents a DAG (Directed Acyclic Graph) that describes your workflow steps and their relationships.

## Basic Structure

```yaml
# Workflow metadata
description: "What this workflow does"
tags: [production, etl]    # Optional: for organization

# Scheduling
schedule: "0 * * * *"      # Optional: cron expression

# Execution control
maxActiveRuns: 1           # Max concurrent runs
maxActiveSteps: 10         # Max parallel steps
timeoutSec: 3600           # Workflow timeout (seconds)

# Parameters
params:
  - KEY: default_value
  - ANOTHER_KEY: "${ENV_VAR}"

# Environment variables
env:
  - VAR_NAME: value
  - PATH: ${PATH}:/custom/path

# Workflow steps
steps:
  - name: step-name        # Optional
    command: echo "Hello"
    depends: previous-step # Optional

# Lifecycle handlers
handlerOn:
  success:
    command: notify-success.sh
  failure:
    command: cleanup-on-failure.sh
```

## Root Fields

### Metadata Fields

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `name` | string | Workflow name | Filename without extension |
| `description` | string | Human-readable description | - |
| `tags` | array | Tags for categorization | `[]` |
| `group` | string | Group name for organization | - |

### Scheduling Fields

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `schedule` | string/array | Cron expression(s) | - |
| `skipIfSuccessful` | boolean | Skip if already succeeded today | `false` |
| `restartWaitSec` | integer | Wait seconds before restart | `0` |

#### Schedule Formats

```yaml
# Single schedule
schedule: "0 2 * * *"

# Multiple schedules
schedule:
  - "0 9 * * MON-FRI"   # 9 AM weekdays
  - "0 14 * * SAT,SUN"  # 2 PM weekends

# With timezone
schedule: "CRON_TZ=America/New_York 0 9 * * *"

# Start/stop schedules
schedule:
  start:
    - "0 8 * * MON-FRI"   # Start at 8 AM
  stop:
    - "0 18 * * MON-FRI"  # Stop at 6 PM
  restart:
    - "0 12 * * MON-FRI"  # Restart at noon
```

### Execution Control Fields

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `maxActiveRuns` | integer | Max concurrent workflow runs (-1 = unlimited) | `1` |
| `maxActiveSteps` | integer | Max parallel steps | `1` |
| `timeoutSec` | integer | Workflow timeout in seconds | `0` (no timeout) |
| `delaySec` | integer | Initial delay before start (seconds) | `0` |
| `maxCleanUpTimeSec` | integer | Max cleanup time (seconds) | `5` |
| `preconditions` | array | Workflow-level preconditions | - |
| `runConfig` | object | User interaction controls when starting DAG | - |

### Data Fields

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `params` | array | Default parameters | `[]` |
| `env` | array | Environment variables | `[]` |
| `secrets` | array | External secret references resolved at runtime and exposed as environment variables | `[]` |
| `dotenv` | string/array | .env files to load | `[".env"]` |
| `workingDir` | string | Working directory for the DAG | Directory of DAG file |
| `logDir` | string | Custom log directory | System default |
| `histRetentionDays` | integer | History retention days | `30` |
| `maxOutputSize` | integer | Max output size per step (bytes) | `1048576` |

### Container Configuration

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `container` | object | Default container configuration for all steps | - |

```yaml
container:
  image: python:3.11
  pullPolicy: missing      # always, missing, never
  env:
    - API_KEY=${API_KEY}
  volumes:
    - /data:/data:ro
  workingDir: /app
  platform: linux/amd64
  user: "1000:1000"
  ports:
    - "8080:8080"
  network: host
  startup: keepalive       # keepalive | entrypoint | command
  command: ["sh", "-c", "my-daemon"]   # when startup: command
  waitFor: running         # running | healthy
  logPattern: "Ready to accept connections"  # optional regex
  restartPolicy: unless-stopped              # optional: no|always|unless-stopped
  keepContainer: false     # Keep container after DAG run
```

> Note: A DAG‑level `container` is started once and kept alive while the
> workflow runs; each step executes via `docker exec` inside that container.
> This means step commands do not pass through the image’s `ENTRYPOINT`/`CMD`.
> If your image’s entrypoint dispatches subcommands, invoke it explicitly in
> the step command (see [Execution Model and Entrypoint Behavior](/writing-workflows/container#execution-model-and-entrypoint-behavior)).
> Readiness waiting (running/healthy and optional logPattern) times out after
> 120 seconds with a clear error including the last known state.

### Secrets

The `secrets` block defines environment variables whose values are fetched at runtime from external providers. Each item is an object with the following fields:

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Environment variable name exposed to steps (required) |
| `provider` | string | Secret provider identifier (required). Built-in providers are `env` and `file`. |
| `key` | string | Provider-specific key (required). For the `env` provider this is the source environment variable; for `file` it is a file path. |
| `options` | object | Provider-specific options (optional). Values must be strings. |

Example:

```yaml
secrets:
  - name: DB_PASSWORD
    provider: env
    key: PROD_DB_PASSWORD        # Read from process environment
  - name: API_TOKEN
    provider: file
    key: ../secrets/api-token    # Relative paths resolve using workingDir then DAG file directory
```

Secret values are injected after DAG-level variables and built-in runtime variables, meaning they take precedence over everything except step-level overrides. Resolved values never touch persistent storage and are automatically masked in logs and captured step output.

See [Secrets](/writing-workflows/secrets) for provider reference, masking behavior, and extension tips.

### SSH Configuration

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `ssh` | object | Default SSH configuration for all steps | - |

```yaml
ssh:
  user: deploy
  host: production.example.com
  port: "22"           # Optional, defaults to "22"
  key: ~/.ssh/id_rsa   # Optional, defaults to standard keys
  password: "${SSH_PASSWORD}" # Optional; prefer keys for security
  strictHostKey: true  # Optional, defaults to true for security
  knownHostFile: ~/.ssh/known_hosts  # Optional, defaults to ~/.ssh/known_hosts
```

When configured at the DAG level, all steps using SSH executor will inherit these settings:

```yaml
# DAG-level SSH configuration
ssh:
  user: deploy
  host: app.example.com
  key: ~/.ssh/deploy_key

steps:
  # These steps inherit the DAG-level SSH configuration
  - systemctl status myapp
  - systemctl restart myapp
  
  # Step-level config overrides DAG-level
  - executor:
      type: ssh
      config:
        user: backup      # Override user
        host: db.example.com  # Override host
        key: ~/.ssh/backup_key  # Override key
    command: mysqldump mydb > backup.sql
```

**Important Notes:**
- SSH and container fields are mutually exclusive at the DAG level
- Step-level SSH configuration completely overrides DAG-level configuration (no partial overrides)
- Password authentication is supported but not recommended; prefer key-based auth
- Default SSH keys are tried if no key is specified: `~/.ssh/id_rsa`, `~/.ssh/id_ecdsa`, `~/.ssh/id_ed25519`, `~/.ssh/id_dsa`

### Working Directory and Volume Resolution

When using container volumes with relative paths, the paths are resolved relative to the DAG's `workingDir`:

```yaml
# DAG with working directory and container volumes
workingDir: /app/project
container:
  image: python:3.11
  volumes:
    - ./data:/data        # Resolves to /app/project/data:/data
    - .:/workspace        # Resolves to /app/project:/workspace
    - /abs/path:/other   # Absolute paths are unchanged

steps:
  - python process.py
```

**Working Directory Inheritance:**
- Steps inherit `workingDir` from the DAG if not explicitly set
- Step-level `workingDir` overrides DAG-level `workingDir`
- Both `dir` and `workingDir` set the working directory (use one or the other)

```yaml
# Example of workingDir inheritance
workingDir: /project          # DAG-level working directory

steps:
  - pwd                   # Outputs: /project
  - workingDir: /custom   # Override DAG workingDir
    command: pwd          # Outputs: /custom
```

### Queue Configuration

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `queue` | string | Queue name | - |

### OpenTelemetry Configuration

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `otel` | object | OpenTelemetry tracing configuration | - |

```yaml
otel:
  enabled: true
  endpoint: "localhost:4317"  # OTLP gRPC endpoint
  headers:
    Authorization: "Bearer ${OTEL_TOKEN}"
  insecure: false
  timeout: 30s
  resource:
    service.name: "dagu-${DAG_NAME}"
    service.version: "1.0.0"
    deployment.environment: "${ENVIRONMENT}"
```

See [OpenTelemetry Tracing](../features/opentelemetry.md) for detailed configuration.

### Notification Fields

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `mailOn` | object | Email notification triggers | - |
| `errorMail` | object | Error email configuration | - |
| `infoMail` | object | Info email configuration | - |
| `smtp` | object | SMTP server configuration | - |

```yaml
mailOn:
  success: true
  failure: true
  
errorMail:
  from: alerts@example.com
  to: oncall@example.com  # Single recipient (string)
  # Or multiple recipients (array):
  # to:
  #   - oncall@example.com
  #   - manager@example.com
  prefix: "[ALERT]"
  attachLogs: true
  
infoMail:
  from: notifications@example.com
  to: team@example.com  # Single recipient (string)
  # Or multiple recipients (array):
  # to:
  #   - team@example.com
  #   - stakeholders@example.com
  prefix: "[INFO]"
  attachLogs: false
  
smtp:
  host: smtp.gmail.com
  port: "587"
  username: notifications@example.com
  password: ${SMTP_PASSWORD}
```

### Handler Fields

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `handlerOn` | object | Lifecycle event handlers | - |

```yaml
handlerOn:
  success:
    command: echo "Workflow succeeded"
  failure:
    command: echo "Notifying failure"
  cancel:
    command: echo "Cleaning up"
  exit:
    command: echo "Always running"
```

### RunConfig

The `runConfig` field allows you to control user interactions when starting DAG runs:

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `disableParamEdit` | boolean | Prevent parameter editing when starting DAG | `false` |
| `disableRunIdEdit` | boolean | Prevent custom run ID input when starting DAG | `false` |

Example usage:

```yaml
# Prevent users from modifying parameters at runtime
runConfig:
  disableParamEdit: true
  disableRunIdEdit: false

params:
  - ENVIRONMENT: production  # Users cannot change this
  - VERSION: 1.0.0           # This is fixed
```

This is useful when:
- You want to enforce specific parameter values for production workflows
- You need consistent run IDs for tracking purposes
- You want to prevent accidental parameter changes

## Step Fields

Each step in the `steps` array can have these fields:

### Basic Fields

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `name` | string | Step name (optional - auto-generated if not provided) | Auto-generated |
| `command` | string | Command to execute. Multi-line strings run as inline scripts (honors shebang) | - |
| `script` | string | Inline script (alternative to command). Honors shebang when no shell is set. | - |
| `run` (legacy) | string | Deprecated alias for `call` | - |
| `depends` | string/array | Step dependencies | - |

### Step Definition Formats

Steps can be defined in multiple formats:

#### Standard Format
```yaml
steps:
  - echo "Hello"
```

#### Shorthand String Format
```yaml
steps:
  - echo "Hello"     # Equivalent to: {command: echo "Hello"}
  - ls -la          # Equivalent to: {command: ls -la}
```

#### Nested Array Format (Parallel Steps)
```yaml
steps:
  - echo "Sequential step 1"
  - 
    - echo "Parallel step 2a"
    - echo "Parallel step 2b"
  - echo "Sequential step 3"
```

In the nested array format:
- Steps within a nested array run in parallel
- They automatically depend on the previous sequential step
- The next sequential step automatically depends on all parallel steps in the group
- Auto-generated names follow the pattern: `parallel_{group}_{command}_{index}`

### Execution Fields

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `dir` | string | Working directory | Current directory |
| `workingDir` | string | Working directory (alternative to `dir`, inherits from DAG) | DAG's workingDir |
| `shell` | string | Shell to use | System default |
| `stdout` | string | Redirect stdout to file | - |
| `stderr` | string | Redirect stderr to file | - |
| `output` | string | Capture output to variable | - |
| `env` | array/object | Step-specific environment variables (overrides DAG-level) | - |
| `call` | string | Name of a DAG to execute as a sub DAG-run | - |
| `params` | string/object | Parameters passed to sub DAGs (`run`) or executor-specific inputs (e.g., GitHub Actions `with:` map) | - |

> ℹ️ The legacy `run` field is still accepted for backward compatibility until v1.24.0, but it will be removed in a future release. Prefer `call` for new workflows.

### Parallel Execution

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `parallel` | array | Items to process in parallel | - |
| `maxConcurrent` | integer | Max parallel executions | No limit |

```yaml
steps:
  - call: file-processor
    parallel:
      items: [file1.csv, file2.csv, file3.csv]
      maxConcurrent: 2
    params: "FILE=${ITEM}"
```

### Conditional Execution

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `preconditions` | array | Conditions to check before execution | - |
| `continueOn` | object | Continue workflow on certain conditions | - |

#### ContinueOn Fields

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `failure` | boolean | Continue execution when step fails | `false` |
| `skipped` | boolean | Continue when step is skipped due to preconditions | `false` |
| `exitCode` | array | List of exit codes that allow continuation | `[]` |
| `output` | array | List of stdout patterns that allow continuation (supports regex with `re:` prefix) | `[]` |
| `markSuccess` | boolean | Mark step as successful when continue conditions are met | `false` |

```yaml
steps:
  - command: echo "Deploying"
    preconditions:
      - condition: "${ENVIRONMENT}"
        expected: "production"
      - condition: "`git branch --show-current`"
        expected: "main"
    
  - command: echo "Running optional task"
    continueOn:
      failure: true
      skipped: true
      exitCode: [0, 1, 2]
      output: ["WARNING", "SKIP", "re:^INFO:.*"]
      markSuccess: true
```

See the [Continue On Reference](/reference/continue-on) for detailed documentation.

### Error Handling

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `retryPolicy` | object | Retry configuration | - |
| `repeatPolicy` | object | Repeat configuration | - |
| `mailOnError` | boolean | Send email on error | `false` |
| `signalOnStop` | string | Signal to send on stop | `SIGTERM` |

#### Retry Policy Fields

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `limit` | integer | Maximum retry attempts | - |
| `intervalSec` | integer | Base interval between retries (seconds) | - |
| `backoff` | any | Exponential backoff multiplier. `true` = 2.0, or specify custom number > 1.0 | - |
| `maxIntervalSec` | integer | Maximum interval between retries (seconds) | - |
| `exitCode` | array | Exit codes that trigger retry | All non-zero |

**Exponential Backoff**: When `backoff` is set, intervals increase exponentially using the formula:  
`interval * (backoff ^ attemptCount)`

#### Repeat Policy Fields

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `repeat` | string | Repeat mode: `"while"` or `"until"` | - |
| `intervalSec` | integer | Base interval between repetitions (seconds) | - |
| `backoff` | any | Exponential backoff multiplier. `true` = 2.0, or specify custom number > 1.0 | - |
| `maxIntervalSec` | integer | Maximum interval between repetitions (seconds) | - |
| `limit` | integer | Maximum number of executions | - |
| `condition` | string | Condition to evaluate | - |
| `expected` | string | Expected value/pattern | - |
| `exitCode` | array | Exit codes that trigger repeat | - |

**Repeat Modes:**
- `while`: Repeats while the condition is true or exit code matches
- `until`: Repeats until the condition is true or exit code matches

**Exponential Backoff**: When `backoff` is set, intervals increase exponentially using the formula:  
`interval * (backoff ^ attemptCount)`
```yaml
steps:
  - command: curl https://api.example.com
    retryPolicy:
      limit: 3
      intervalSec: 30
      exitCode: [1, 255]  # Retry only on specific codes
      
  - command: curl https://api.example.com
    retryPolicy:
      limit: 5
      intervalSec: 2
      backoff: true        # Exponential backoff (2.0x multiplier)
      maxIntervalSec: 60   # Cap at 60 seconds
      exitCode: [429, 503] # Rate limit or unavailable
    
  - command: check-process.sh
    repeatPolicy:
      repeat: while        # Repeat WHILE process is running
      exitCode: [0]        # Exit code 0 means process found
      intervalSec: 60
      limit: 30
      
  - command: echo "Checking status"
    output: STATUS
    repeatPolicy:
      repeat: until        # Repeat UNTIL status is ready
      condition: "${STATUS}"
      expected: "ready"
      intervalSec: 5
      backoff: 1.5         # Custom backoff multiplier
      maxIntervalSec: 300  # Cap at 5 minutes
      limit: 60
```

### Executor Configuration

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `executor` | object | Executor configuration | Shell executor |

```yaml
steps:
  - executor:
      type: docker
      config:
        image: python:3.11
        volumes:
          - /data:/data:ro
        env:
          - API_KEY=${API_KEY}
    command: python process.py
```

### Distributed Execution

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `workerSelector` | object | Worker label requirements for distributed execution | - |

When using distributed execution, specify `workerSelector` to route tasks to workers with matching labels:

```yaml
steps:
  - call: gpu-training
---
# Run on a worker with gpu
name: gpu-training
workerSelector:
  gpu: "true"
  memory: "64G"
steps:
  - python train_model.py
```

**Worker Selection Rules:**
- All labels in `workerSelector` must match exactly on the worker
- Label values are case-sensitive strings
- Steps without `workerSelector` can run on any available worker
- If no workers match the selector, the task waits until a matching worker is available

See [Distributed Execution](/features/distributed-execution) for complete documentation.

## Variable Substitution

### Parameter References

```yaml
params:
  - USER: john
  - DOMAIN: example.com

steps:
  - echo "Hello ${USER} from ${DOMAIN}"
```

### Environment Variables

```yaml
env:
  - API_URL: https://api.example.com
  - API_KEY: ${SECRET_API_KEY}  # From system env

steps:
  - curl -H "X-API-Key: ${API_KEY}" ${API_URL}
```

### Loading Environment from .env Files

The `dotenv` field allows loading environment variables from `.env` files:

```yaml
# Default behavior - loads .env file if it exists
# No dotenv field needed, defaults to [".env"]

# Load specific .env file
dotenv: .env.production

# Load multiple .env files (later files override earlier ones)
dotenv:
  - .env.defaults
  - .env.local

# Disable .env loading
dotenv: []
```

**Important Notes:**
- If `dotenv` is not specified, Dagu automatically tries to load `.env` file
- Files are loaded relative to the DAG's `workingDir`
- Later files in the array override variables from earlier files
- System environment variables take precedence over .env file variables
- .env files are loaded at DAG startup, before any steps execute

**Example .env file:**
```bash
# .env file
DATABASE_URL=postgres://localhost/mydb
API_KEY=secret123
DEBUG=true
```

```yaml
# DAG using .env variables
workingDir: /app
dotenv: .env          # Optional, this is the default

steps:
  - psql ${DATABASE_URL}
  - echo "Debug is ${DEBUG}"
```

### Command Substitution

```yaml
steps:
  - echo "Today is `date +%Y-%m-%d`"
    
  - command: deploy.sh
    preconditions:
      - condition: "`git branch --show-current`"
        expected: "main"
```

### Output Variables

```yaml
steps:
  - command: cat VERSION
    output: VERSION
  - command: docker build -t app:${VERSION} .
```

### JSON Path Access

```yaml
steps:
  - command: cat config.json
    output: CONFIG
  - command: echo "Port is ${CONFIG.server.port}"
```

## Special Variables

These variables are automatically available:

| Variable | Description |
|----------|-------------|
| `DAG_NAME` | Current DAG name |
| `DAG_RUN_ID` | Unique run identifier |
| `DAG_RUN_LOG_FILE` | Path to workflow log |
| `DAG_RUN_STEP_NAME` | Current step name |
| `DAG_RUN_STEP_STDOUT_FILE` | Step stdout file path |
| `DAG_RUN_STEP_STDERR_FILE` | Step stderr file path |
| `ITEM` | Current item in parallel execution |

## Complete Example

```yaml
name: production-etl
description: Daily ETL pipeline for production data
tags: [production, etl, critical]
schedule: "0 2 * * *"

maxActiveRuns: 1
maxActiveSteps: 5
timeoutSec: 7200
histRetentionDays: 90

params:
  - DATE: "`date +%Y-%m-%d`"
  - ENVIRONMENT: production

env:
  - DATA_DIR: /data/etl
  - LOG_LEVEL: info
  
dotenv:
  - /etc/dagu/production.env

# Default container for all steps
container:
  image: python:3.11-slim
  pullPolicy: missing
  env:
    - PYTHONUNBUFFERED=1
  volumes:
    - ./data:/data
    - ./scripts:/scripts:ro

preconditions:
  - condition: "`date +%u`"
    expected: "re:[1-5]"  # Weekdays only

steps:
  - ./scripts/validate.sh
    
  - command: python extract.py --date=${DATE}
    depends: validate-environment
    output: RAW_DATA_PATH
    retryPolicy:
      limit: 3
      intervalSec: 300
    
  - call: transform-module
    parallel:
      items: [customers, orders, products]
      maxConcurrent: 2
    params: "TYPE=${ITEM} INPUT=${RAW_DATA_PATH}"
    continueOn:
      failure: false

 # Use different executor for this step   
  - executor:
      type: docker
      config:
        image: postgres:16
        env:
          - PGPASSWORD=${DB_PASSWORD}
    command: psql -h ${DB_HOST} -U ${DB_USER} -f load.sql
    
  - command: python validate_results.py --date=${DATE}
    depends: load-data
    mailOnError: true

handlerOn:
  success:
    command: |
      echo "ETL completed successfully for ${DATE}"
      ./scripts/notify-success.sh
  failure:
    executor:
      type: mail
      config:
        to: data-team@example.com
        subject: "ETL Failed - ${DATE}"
        body: "Check logs at ${DAG_RUN_LOG_FILE}"
        attachLogs: true
  exit:
    command: ./scripts/cleanup.sh ${DATE}

mailOn:
  failure: true
  
smtp:
  host: smtp.company.com
  port: "587"
  username: etl-notifications@company.com
```
