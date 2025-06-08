# DAG-Level YAML Specification

This document provides the complete specification for DAG-level fields in Dagu YAML files, including implementation details and behaviors discovered through source code analysis.

## Table of Contents

1. [Overview](#overview)
2. [Metadata Fields](#metadata-fields)
   - [name](#name-string-optional)
   - [group](#group-string-optional)
   - [description](#description-string-optional)
   - [tags](#tags-string-or-array-optional)
3. [Scheduling Configuration](#scheduling-configuration)
   - [schedule](#schedule-string-array-or-object-optional)
   - [skipIfSuccessful](#skipifsuccessful-boolean-optional)
   - [restartWaitSec](#restartwaitsec-integer-optional)
4. [Environment Configuration](#environment-configuration)
   - [env](#env-map-or-array-of-maps-optional)
   - [dotenv](#dotenv-string-or-array-optional)
5. [Execution Control](#execution-control)
   - [timeout](#timeout-integer-optional)
   - [delay](#delay-integer-optional)
   - [maxActiveRuns](#maxactiveruns-integer-optional)
   - [maxActiveSteps](#maxactivesteps-integer-optional)
   - [maxCleanUpTime](#maxcleanuptime-integer-optional)
6. [Data Management](#data-management)
   - [logDir](#logdir-string-optional)
   - [histRetentionDays](#histretentiondays-integer-optional)
7. [Parameters](#parameters)
   - [params](#params-string-array-or-map-optional)
8. [Conditions](#conditions)
   - [preconditions](#preconditions-string-map-array-optional)
9. [Event Handlers](#event-handlers)
   - [handlerOn](#handleron-object-optional)
10. [Email Configuration](#email-configuration)
    - [smtp](#smtp-object-optional)
    - [mailOn](#mailon-object-optional)
    - [errorMail](#errormail-object-optional)
    - [infoMail](#infomail-object-optional)
11. [Implementation Details](#implementation-details)
    - [Loading and Parsing](#loading-and-parsing)
    - [Socket Communication](#socket-communication)
    - [Variable Evaluation](#variable-evaluation-context)
    - [Base Configuration](#base-configuration-inheritance)

## Overview

A DAG (Directed Acyclic Graph) definition file contains configuration that applies to the entire workflow. These fields control scheduling, execution behavior, environment setup, and workflow-level handlers.

### Basic Structure

```yaml
# DAG-level fields (all optional)
name: my-workflow                    # Workflow identifier
schedule: "0 1 * * *"               # Cron expression
env:                                # Environment variables
  - KEY: value

# Steps are defined separately (required)
steps:
  - name: step1
    command: echo "Hello"
```

### Key Concepts

- **All DAG-level fields are optional** - Only `steps` is required
- **Defaults are sensible** - Works out of the box with minimal configuration
- **Variables are expanded** - Environment variables and command substitution supported
- **Configuration is inherited** - Base configurations can be shared across DAGs

## Metadata Fields

Metadata fields provide identification, organization, and documentation for your DAGs.

### `name` (string, optional)

**Description**: Name of the DAG. Defaults to filename without extension if not specified.

**Constraints**: 
- Maximum length: 40 characters
- Allowed characters: `[a-zA-Z0-9_.-]` (regex: `^[a-zA-Z0-9_.-]+$`)
- Used for socket naming, file paths, and identification

**Default**: `defaultName(file)` - filename without extension

**Example**: 
```yaml
name: data-pipeline
```

### `group` (string, optional)

**Description**: Group name for organizing DAGs in the UI.

**Features**:
- No validation constraints
- Used for UI organization and filtering
- Can be any string value

**Example**: 
```yaml
group: ETL
```

### `description` (string, optional)

**Description**: Human-readable description of the DAG.

**Features**:
- No length limits
- Displayed in UI and status outputs
- Supports multi-line strings

**Example**: 
```yaml
description: "Daily customer data processing pipeline"
# Or multi-line
description: |
  This pipeline processes customer data daily:
  - Extracts from multiple sources
  - Transforms and validates
  - Loads to data warehouse
```

### `tags` (string or array, optional)

**Description**: Tags for filtering and categorization.

**Processing**: 
- Converted to lowercase
- Whitespace trimmed
- Empty strings filtered out
- Duplicates allowed

**Formats**:

1. **String** (comma-separated):
   ```yaml
   tags: "daily,critical,etl"
   ```

2. **Array**:
   ```yaml
   tags: [daily, critical, etl]
   ```

3. **Mixed types** (converted to strings):
   ```yaml
   tags: [prod, 123, true]  # Becomes ["prod", "123", "true"]
   ```

## Scheduling Configuration

Scheduling fields control when and how often your DAG runs.

### `schedule` (string, array, or object, optional)
- **Description**: Cron expression(s) for scheduling the DAG.
- **Parser**: Standard 5-field cron (minute hour day month weekday)
- **Formats**:

1. **String**: Single cron expression
   ```yaml
   schedule: "0 1 * * *"
   ```

2. **Array**: Multiple cron expressions
   ```yaml
   schedule:
     - "0 1 * * *"
     - "0 13 * * *"
   ```

3. **Object**: Start/stop/restart schedules
   ```yaml
   schedule:
     start: "0 8 * * *"     # Start at 8 AM
     stop: "0 18 * * *"     # Stop at 6 PM
     restart:                # Multiple restart times
       - "0 12 * * *"       # Restart at noon
       - "0 15 * * *"       # Restart at 3 PM
   ```

**Timezone Support**: 
```yaml
schedule: "CRON_TZ=America/New_York 0 9 * * *"  # 9 AM ET
# Or with multiple schedules
schedule:
  - "CRON_TZ=America/New_York 0 9 * * MON-FRI"   # 9 AM ET weekdays
  - "CRON_TZ=Europe/London 0 14 * * MON-FRI"     # 2 PM GMT weekdays
```

**Validation**: 
- Uses `robfig/cron` parser
- Standard 5-field cron format
- Invalid expressions cause build failure

### `skipIfSuccessful` (boolean, optional)

**Description**: Skip scheduled execution if DAG was already executed successfully.

**Default**: `false`

**Use Cases**:
- Prevent duplicate runs when manually triggered before schedule
- Implement "at-most-once" semantics for critical processes
- Save resources by avoiding redundant executions

**Implementation**: Checks last run status before executing

**Example**:
```yaml
schedule: "0 8 * * *"
skipIfSuccessful: true  # Won't run at 8 AM if already ran successfully today
```

### `restartWaitSec` (integer, optional)

**Description**: Seconds to wait before restarting the DAG.

**Default**: `0`

**Applies to**: Restart schedules only

**Use Cases**:
- Allow graceful cleanup time
- Prevent resource contention
- Implement cooling-off periods

**Example**:
```yaml
schedule:
  start: "0 8 * * *"
  restart: "0 12 * * *"
restartWaitSec: 60  # Wait 1 minute before restart
```

## Environment Configuration

Environment fields set up the execution context for all steps in your DAG.

### `env` (map or array of maps, optional)
- **Description**: Environment variables for the DAG.
- **Variable Resolution Order**:
  1. Command substitution (`` `cmd` ``)
  2. Variable references (`${VAR}`)
  3. Environment expansion

- **Formats**:
1. **Map** (order not guaranteed):
   ```yaml
   env:
     KEY1: value1
     KEY2: ${HOME}/data
   ```

2. **Array of maps** (preserves order):
   ```yaml
   env:
     - KEY1: value1
     - KEY2: "`date +%Y%m%d`"
     - KEY3: ${KEY1}_suffix
   ```

- **Evaluation Context**: 
  - Evaluated during DAG build
  - Available to all steps
  - Set as actual environment variables

### `dotenv` (string or array, optional)
- **Description**: Path(s) to .env file(s) to load.
- **Formats**:
  - String: `dotenv: .env`
  - Array: `dotenv: [.env, .env.production]`
- **Path Resolution**: 
  - Relative to DAG file location
  - Can use environment variables
- **Loading**: Uses `godotenv.Overload` (overwrites existing)
- **Evaluation**: Can contain command substitution
- **Error Handling**: Missing files skipped silently

## Execution Control

Execution control fields manage how your DAG runs, including concurrency, timeouts, and cleanup behavior.

### `timeout` (integer, optional)
- **Description**: Maximum execution time in seconds for the entire DAG.
- **Default**: `0` (no timeout)
- **Implementation**: Context with timeout
- **Behavior**: All running steps cancelled on timeout
- **Example**: `timeout: 3600`

### `delay` (integer, optional)
- **Description**: Initial delay in seconds before starting the DAG.
- **Default**: `0`
- **Applies after**: Schedule trigger or manual start
- **Example**: `delay: 10`

### `maxActiveRuns` (integer, optional)
- **Description**: Maximum number of concurrent DAG runs.
- **Default**: `0` (unlimited)
- **Implementation**: Checked before creating new run
- **Note**: Deprecated in favor of `maxActiveSteps` for step concurrency
- **Example**: `maxActiveRuns: 1`

### `maxActiveSteps` (integer, optional)
- **Description**: Maximum number of steps running concurrently.
- **Default**: `0` (unlimited)
- **Implementation**: 
  ```go
  if maxActiveRuns > 0 && maxActiveSteps == 0 {
      maxActiveSteps = maxActiveRuns // Backward compatibility
  }
  ```
- **Example**: `maxActiveSteps: 5`

### `maxCleanUpTime` (integer, optional)
- **Description**: Maximum seconds to wait for cleanup when DAG is stopped.
- **Default**: `60`
- **Process**:
  1. Send configured signal (or SIGTERM)
  2. Wait up to maxCleanUpTime
  3. Force kill (SIGKILL) after timeout
- **Example**: `maxCleanUpTime: 300`

## Data Management

Data management fields control where logs are stored and how long execution history is retained.

### `logDir` (string, optional)
- **Description**: Custom directory for storing logs.
- **Default**: System default (`~/.local/share/dagu/logs/`)
- **Variable Expansion**: Supported
- **Directory Creation**: Automatic with 0750 permissions
- **Example**: `logDir: /var/log/dagu/my-dag`

### `histRetentionDays` (integer, optional)
- **Description**: Days to retain execution history.
- **Default**: `30`
- **Cleanup**: Runs on scheduler start
- **Affects**: DAG run history and logs
- **Example**: `histRetentionDays: 90`

## Parameters

Parameters allow dynamic configuration of your DAG at runtime.

### `params` (string, array, or map, optional)
- **Description**: Default parameters for the DAG.
- **Variable Access**:
  - Positional: `$1`, `$2`, etc.
  - Named: `${name}`
  - Environment: Set as env vars

- **Formats**:
1. **String** (space-separated, handles quotes):
   ```yaml
   params: "env=prod batch_size=100 path=\"/data/my path\""
   ```
   - Parsing regex: `` `(?:([^\s=]+)=)?("(?:\\"|[^"])*"|`(?:\\"|[^"])*`|[^"\s]+)` ``
   - Supports quoted values with spaces

2. **Array** (positional or named):
   ```yaml
   params:
     - value1              # $1
     - name=value2         # ${name}
     - "complex=foo bar"   # ${complex}
   ```

3. **Map**:
   ```yaml
   params:
     env: prod
     batch_size: 100
     date: "`date +%Y%m%d`"
   ```

**Command Substitution**: Backticks evaluated during build

**Override**: CLI params override defaults

**Implementation Details**:
- Stored as `DefaultParams` string
- Converted to array for execution
- Named params set as environment variables

**Advanced Example**:
```yaml
# Dynamic parameters with fallbacks
params:
  DATE: "`date -d '1 day ago' +%Y-%m-%d`"
  ENV: "${ENVIRONMENT:-development}"
  BATCH_SIZE: "${BATCH_SIZE:-1000}"
  
# CLI override:
# dagu start my-dag.yaml -- DATE=2024-01-01 BATCH_SIZE=5000
```

## Conditions

Conditions allow you to control whether a DAG should execute based on preconditions.

### `preconditions` (string, map, array, optional)
- **Description**: Conditions that must be met before DAG execution.
- **Evaluation**: Before any steps run
- **Failure**: DAG marked as skipped/failed

- **Formats**:
1. **String** (command):
   ```yaml
   preconditions: "test -f /data/input.csv"
   ```

2. **Map** (condition/expected):
   ```yaml
   preconditions:
     condition: "${ENV}"
     expected: "production"
   ```

3. **Array** (multiple conditions, ALL must pass):
   ```yaml
   preconditions:
     - test -f /data/input.csv
     - condition: "${READY}"
       expected: "true"
     - condition: "`curl -s api/status`"
       expected: "re:^(active|ready)$"
   ```

**Evaluation Rules**:
- Command: Success = exit code 0
- Condition without expected: Success = exit code 0
- Condition with expected: String comparison (regex with `re:` prefix)
- All conditions must pass (AND logic)

**Advanced Example**:
```yaml
preconditions:
  # File existence check
  - test -f /data/input.csv
  
  # Environment check
  - condition: "${ENVIRONMENT}"
    expected: "production"
    
  # Day of week check (Mon-Fri only)
  - condition: "`date +%u`"
    expected: "re:[1-5]"
    
  # API health check
  - condition: "`curl -s https://api.example.com/health | jq -r .status`"
    expected: "healthy"
    
  # Disk space check (less than 90% used)
  - condition: "`df -h /data | awk 'NR==2 {print $5}' | sed 's/%//'`"
    expected: "re:^[0-8][0-9]$"
```

## Event Handlers

Event handlers execute special steps based on DAG lifecycle events.

### `handlerOn` (object, optional)
- **Description**: Steps to execute on DAG lifecycle events.
- **Execution Order**: success/failure/cancel → exit
- **Context**: All step outputs available
- **Implementation**: 
  ```go
  // Handlers are special steps with reserved names
  step.Name = "onSuccess" // or onFailure, onCancel, onExit
  ```

- **Fields**:
  - `success`: Execute when DAG completes successfully
  - `failure`: Execute when DAG fails
  - `cancel`: Execute when DAG is cancelled
  - `exit`: Always execute on DAG completion

- **Example**:
  ```yaml
  handlerOn:
    success:
      command: notify.sh "Success"
      executor:
        type: mail
        config:
          to: team@company.com
          subject: "Pipeline Success"
    failure:
      command: alert.sh "Failed"
      output: FAILURE_DETAILS
    exit:
      command: cleanup.sh
      continueOn:
        failure: true
  ```

**Handler Features**:
- Full step capabilities (executors, outputs, etc.)
- Access to all step output variables
- Cannot be depended upon by other steps
- Exit handler runs even on panic
- Handlers can use all step features (retry, preconditions, etc.)

**Complete Example**:
```yaml
handlerOn:
  success:
    command: |
      echo "Pipeline completed successfully"
      echo "Total records: ${TOTAL_RECORDS}"
    executor:
      type: mail
      config:
        to: team@company.com
        subject: "${DAG_NAME} Success"
        message: "Pipeline completed. Records: ${TOTAL_RECORDS}"
        
  failure:
    command: |
      echo "Pipeline failed at step: ${FAILED_STEP:-unknown}"
      collect-diagnostics.sh
    output: DIAGNOSTICS
    executor:
      type: http
      config:
        url: https://api.pagerduty.com/incidents
        method: POST
        headers:
          Authorization: "Token ${PAGERDUTY_TOKEN}"
        body: |
          {
            "incident": {
              "type": "incident",
              "title": "DAG ${DAG_NAME} failed",
              "body": {
                "details": "${DIAGNOSTICS}"
              }
            }
          }
          
  cancel:
    command: |
      echo "Pipeline was cancelled"
      cleanup-partial-data.sh
      
  exit:
    command: |
      echo "Cleaning up temporary files..."
      rm -rf /tmp/${DAG_NAME}_*
    continueOn:
      failure: true  # Cleanup even if it fails
```

## Email Configuration

Email configuration enables notifications and alerts for your DAG execution.

### `smtp` (object, optional)
- **Description**: SMTP server configuration.
- **Variable Expansion**: All fields support expansion
- **Required for**: Email executor and notifications
- **Fields**:
  - `host`: SMTP server hostname
  - `port`: SMTP server port (string)
  - `username`: SMTP username
  - `password`: SMTP password

- **Example**:
  ```yaml
  smtp:
    host: smtp.gmail.com
    port: "587"
    username: ${SMTP_USER}
    password: ${SMTP_PASS}
  ```

### `mailOn` (object, optional)
- **Description**: When to send email notifications.
- **Requires**: SMTP configuration
- **Fields**:
  - `success`: Send on success (boolean)
  - `failure`: Send on failure (boolean)

- **Note**: Uses pointer to allow explicit false override
- **Example**:
  ```yaml
  mailOn:
    success: false
    failure: true
  ```

### `errorMail` (object, optional)
- **Description**: Email configuration for errors.
- **Triggered**: On DAG failure when `mailOn.failure: true`
- **Fields**:
  - `from`: Sender email address
  - `to`: Recipient email address (comma-separated for multiple)
  - `prefix`: Subject line prefix
  - `attachLogs`: Attach log files (boolean)

- **Example**:
  ```yaml
  errorMail:
    from: dagu@company.com
    to: "oncall@company.com,team@company.com"
    prefix: "[ERROR]"
    attachLogs: true
  ```

### `infoMail` (object, optional)
- **Description**: Email configuration for informational messages.
- **Triggered**: On DAG success when `mailOn.success: true`
- **Fields**: Same as `errorMail`

## Implementation Details

This section covers internal implementation details for advanced users and contributors.

### Loading and Parsing

1. **Load Order**:
   ```go
   1. resolveYamlFilePath(nameOrPath)
   2. LoadBaseConfig(baseConfig) // if specified
   3. readYAMLFile(dagPath)
   4. decode(raw) → definition
   5. build(definition) → DAG
   6. merge(base, target)
   7. initializeDefaults()
   ```

2. **Default Values**:
   ```go
   const (
       defaultDAGRunRetentionDays = 30
       defaultMaxCleanUpTime      = 60 * time.Second
   )
   ```

3. **Validation Timing**:
   - Name validation: During build
   - Schedule parsing: During build
   - Step validation: After all steps built

### Socket Communication

**Unix Socket Naming**:
```go
// Pattern: @dagu_{name}_{hash}.sock
// Hash: MD5(name + dagRunID)[:6]
func SockAddr(name, dagRunID string) string {
    combined := name + dagRunID
    hash := md5.Sum([]byte(combined))[:6]
    // Max socket name: 50 chars
    // Truncate name if needed
}
```

### Variable Evaluation Context

**DAG-Level Evaluation**:
- Environment variables evaluated during build
- Parameters evaluated during build
- Available to all steps and handlers
- Set as actual OS environment variables

### Base Configuration Inheritance

**Merge Behavior**:
- Target DAG overrides base config
- Arrays replaced, not appended
- Maps merged with override
- Special handling for `mailOn` pointer

**Complete Example**:
```yaml
# base.yaml - Shared configuration
env:
  - ENVIRONMENT: production
  - LOG_LEVEL: info
smtp:
  host: smtp.company.com
  port: "587"
  username: ${SMTP_USER}
  password: ${SMTP_PASS}
mailOn:
  failure: true
errorMail:
  from: alerts@company.com
  to: oncall@company.com
  attachLogs: true

# my-dag.yaml - Inherits and overrides base
name: data-pipeline
group: ETL
tags: [daily, critical]

# Override specific env vars
env:
  - ENVIRONMENT: staging  # Overrides base
  - DB_HOST: staging.db.company.com  # Additional var
  
schedule: "0 2 * * *"
skipIfSuccessful: true

steps:
  - name: process
    command: process.sh
```

**Inheritance Rules**:
- Scalar values: Target overrides base
- Arrays: Target replaces base (no merging)
- Maps: Deep merge with target priority
- Special handling for pointer fields (mailOn)

---

## Quick Reference Card

```yaml
# All fields are optional
name: string                # Max 40 chars, [a-zA-Z0-9_.-]
group: string              # UI grouping
description: string        # Human-readable description
tags: string|array         # Filtering tags

schedule: string|array|obj # Cron expression(s)
skipIfSuccessful: bool     # Skip if already succeeded
restartWaitSec: int        # Wait before restart

env: map|array            # Environment variables
dotenv: string|array      # .env file(s) to load

timeout: int              # Total DAG timeout (seconds)
delay: int                # Initial delay (seconds)
maxActiveRuns: int        # Max concurrent runs
maxActiveSteps: int       # Max parallel steps
maxCleanUpTime: int       # Cleanup timeout (seconds)

logDir: string            # Custom log directory
histRetentionDays: int    # History retention (default: 30)

params: string|array|map  # Default parameters
preconditions: various    # Pre-execution checks

handlerOn:                # Lifecycle handlers
  success: step          # On success
  failure: step          # On failure
  cancel: step           # On cancel
  exit: step             # Always (cleanup)

smtp: object             # SMTP configuration
mailOn:                  # When to send mail
  success: bool
  failure: bool
errorMail: object        # Error email config
infoMail: object         # Info email config

steps: [required]        # Workflow steps
```

---

This specification covers all DAG-level fields and their implementation details based on source code analysis. For step-level configuration, see [DAG_YAML_STEP.md](./DAG_YAML_STEP.md).
