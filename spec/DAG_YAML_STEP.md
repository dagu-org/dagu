# Step-Level YAML Specification

This document provides the complete specification for step-level fields in Dagu YAML files, including implementation details and behaviors discovered through source code analysis.

## Table of Contents

- [Step-Level YAML Specification](#step-level-yaml-specification)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
    - [Step Definition Formats](#step-definition-formats)
      - [Array Format (Recommended)](#array-format-recommended)
      - [Map Format](#map-format)
    - [Key Concepts](#key-concepts)
  - [Basic Step Fields](#basic-step-fields)
    - [`name` (string, required)](#name-string-required)
    - [`description` (string, optional)](#description-string-optional)
    - [`dir` (string, optional)](#dir-string-optional)
    - [`shell` (string, optional)](#shell-string-optional)
    - [`packages` (array, optional)](#packages-array-optional)
  - [Command Execution](#command-execution)
    - [`command` (string or array, optional\*)](#command-string-or-array-optional)
    - [`script` (string, optional)](#script-string-optional)
    - [`stdout` (string, optional)](#stdout-string-optional)
    - [`stderr` (string, optional)](#stderr-string-optional)
  - [Dependencies and Flow Control](#dependencies-and-flow-control)
    - [`depends` (string or array, optional)](#depends-string-or-array-optional)
    - [`preconditions` (same format as DAG-level)](#preconditions-same-format-as-dag-level)
  - [Error Handling and Recovery](#error-handling-and-recovery)
    - [`continueOn` (object, optional)](#continueon-object-optional)
      - [`failure` (boolean)](#failure-boolean)
      - [`skipped` (boolean)](#skipped-boolean)
      - [`exitCode` (array of integers)](#exitcode-array-of-integers)
      - [`output` (string or array)](#output-string-or-array)
      - [`markSuccess` (boolean)](#marksuccess-boolean)
    - [`mailOnError` (boolean, optional)](#mailonerror-boolean-optional)
    - [`signalOnStop` (string, optional)](#signalonstop-string-optional)
  - [Retry and Repeat Policies](#retry-and-repeat-policies)
    - [`retryPolicy` (object, optional)](#retrypolicy-object-optional)
      - [Fields:](#fields)
    - [`repeatPolicy` (object, optional)](#repeatpolicy-object-optional)
      - [Fields:](#fields-1)
  - [Output Capture and Variables](#output-capture-and-variables)
    - [`output` (string, optional)](#output-string-optional)
  - [Executors](#executors)
    - [`executor` (string or object, optional)](#executor-string-or-object-optional)
    - [Available Executors](#available-executors)
      - [Command Executor (default)](#command-executor-default)
      - [Docker Executor](#docker-executor)
      - [HTTP Executor](#http-executor)
      - [SSH Executor](#ssh-executor)
      - [Mail Executor](#mail-executor)
      - [JQ Executor](#jq-executor)
  - [Child DAG Execution](#child-dag-execution)
    - [`run` (string, optional)](#run-string-optional)
    - [`params` (string, optional)](#params-string-optional)
    - [Child DAG Output Access](#child-dag-output-access)
  - [Advanced Features](#advanced-features)
    - [Variable Resolution](#variable-resolution)
    - [Special Variables](#special-variables)
    - [Command Evaluation](#command-evaluation)
  - [Implementation Details](#implementation-details)
    - [Step Execution Lifecycle](#step-execution-lifecycle)
    - [Log File Naming](#log-file-naming)
    - [Output Capture Implementation](#output-capture-implementation)
    - [State Management](#state-management)
    - [Panic Recovery](#panic-recovery)
  - [Quick Reference Card](#quick-reference-card)
  - [Common Patterns](#common-patterns)
    - [Sequential Pipeline](#sequential-pipeline)
    - [Parallel Processing with Fan-in](#parallel-processing-with-fan-in)
    - [Error Recovery Pattern](#error-recovery-pattern)
    - [Conditional Execution Pattern](#conditional-execution-pattern)

## Overview

Steps are the individual units of work in a DAG. They can execute commands, run child DAGs, make HTTP requests, or perform other actions through various executors.

### Step Definition Formats

Steps can be defined in two formats:

#### Array Format (Recommended)
```yaml
steps:
  - name: step1
    command: echo "Hello"
  - name: step2
    command: echo "World"
    depends: step1
```

#### Map Format
```yaml
steps:
  step1:
    command: echo "Hello"
  step2:
    command: echo "World"
    depends: step1
```

### Key Concepts

- **Execution Order**: Determined by dependencies
- **Parallel Execution**: Steps without dependencies run concurrently
- **Error Propagation**: Failed steps cause dependent steps to skip (unless configured otherwise)
- **Output Sharing**: Steps can pass data to subsequent steps

## Basic Step Fields

These fields provide basic configuration for each step.

### `name` (string, required)

**Description**: Unique name for the step.

**Constraints**: 
- Maximum length: 40 characters
- Must be unique within the DAG
- Used in dependencies and logging
- Validation: Build fails on duplicate names

**Example**:
```yaml
name: process-customer-data
```

### `description` (string, optional)

**Description**: Human-readable description of the step.

**Features**:
- No length constraints
- Used in UI display and logging
- Supports multi-line strings

**Example**:
```yaml
description: "Process and validate customer data from CSV"
```

### `dir` (string, optional)

**Description**: Working directory for the step.

**Default**: DAG file directory

**Path Resolution**:
1. Evaluate variables/commands
2. Resolve relative paths
3. Expand home directory

**Error Handling**: Step fails if directory doesn't exist

**Examples**:
```yaml
# Absolute path
dir: /app/data

# With variable
dir: "${PROJECT_ROOT}/data"

# Relative to DAG file
dir: ./scripts

# Home directory
dir: ~/workspace
```

### `shell` (string, optional)

**Description**: Shell to use for command execution.

**Default Priority**:
1. Step's `shell` field
2. `$SHELL` environment variable
3. Fallback to `sh`

**Common Values**:
- `bash` - Bash shell
- `sh` - POSIX shell
- `zsh` - Z shell
- `nix-shell` - Nix shell (use with `packages`)

**Example**:
```yaml
# Use bash for array features
shell: bash
command: echo "${array[@]}"
```

### `packages` (array, optional)

**Description**: Packages to install (only with `shell: nix-shell`).

**Requirements**: 
- Must set `shell: nix-shell`
- Nix package manager must be installed

**Example**:
```yaml
# Python data science environment
shell: nix-shell
packages: [python3, numpy, pandas, matplotlib]
command: python analyze.py

# Node.js development
shell: nix-shell
packages: [nodejs, yarn]
command: yarn test
```

## Command Execution

These fields control how commands are executed within a step.

### `command` (string or array, optional*)

**Description**: Command to execute.

**Required**: Unless using `script`, `executor`, or `run`

**Formats**:

1. **String** (shell interpretation):
   ```yaml
   command: "echo hello world"
   # Parsed using shell rules
   ```

2. **Array** (direct execution):
   ```yaml
   command: ["python", "script.py", "--arg", "value with spaces"]
   # First element: command
   # Rest: arguments
   ```

**Command Parsing**:
- String format uses `SplitCommand()` which respects quotes
- Handles escaped quotes
- Preserves quoted arguments

**Examples**:
```yaml
# Simple command
command: echo "Hello World"

# Command with pipes
command: "cat data.csv | grep ERROR | wc -l"

# Array format for precise control
command: ["python", "script.py", "--input", "file with spaces.txt"]

# Using variables
command: "process.sh ${INPUT_FILE} ${OUTPUT_DIR}"
```

### `script` (string, optional)

**Description**: Inline script to execute.

**Implementation**:
1. Written to temporary file
2. Made executable
3. Executed via shell
4. Temp file removed after execution

**Variable Expansion**: 
- Command executors: Only variable replacement
- Other executors: Full evaluation

**Examples**:
```yaml
# Bash script
shell: bash
script: |
  echo "Starting process"
  for i in {1..10}; do
    echo "Processing $i"
  done

# Python script
script: |
  #!/usr/bin/env python3
  import json
  data = {"status": "success", "count": 42}
  print(json.dumps(data))

# Complex shell script with error handling
script: |
  set -euo pipefail
  
  if [ ! -f "${INPUT_FILE}" ]; then
    echo "Error: Input file not found"
    exit 1
  fi
  
  process_data.sh "${INPUT_FILE}" || {
    echo "Processing failed"
    cleanup.sh
    exit 2
  }
```

### `stdout` (string, optional)

**Description**: File path to redirect stdout.

**Features**:
- Path resolution relative to step's `dir`
- Creates file if doesn't exist
- Uses MultiWriter to log to both file and standard log

**Example**:
```yaml
stdout: output.log
# Or with absolute path
stdout: /var/log/app/process.out
```

### `stderr` (string, optional)

**Description**: File path to redirect stderr.

**Features**: Same as `stdout`

**Example**:
```yaml
stderr: errors.log
# Separate stdout and stderr
stdout: process.out
stderr: process.err
```

## Dependencies and Flow Control

These fields control the execution order and conditional execution of steps.

### `depends` (string or array, optional)

**Description**: Step dependencies.

**Formats**:
- String: `depends: step1`
- Array: `depends: [step1, step2]`

**Execution Rules**:
- Step waits for all dependencies to complete
- Fails if any dependency fails (unless `continueOn` configured)
- Skips if any dependency skipped (unless `continueOn` configured)

**Examples**:
```yaml
# Single dependency
depends: validate-input

# Multiple dependencies
depends: [download-data, check-space]

# Complex dependency graph
steps:
  - name: download
    command: wget data.csv
  
  - name: validate
    command: validate.py data.csv
    depends: download
  
  - name: process-a
    command: process_a.py
    depends: validate
  
  - name: process-b
    command: process_b.py
    depends: validate
  
  - name: combine
    command: combine.py
    depends: [process-a, process-b]
```

### `preconditions` (same format as DAG-level)

**Description**: Conditions that must be met before step execution.

**Evaluation Context**: Includes output variables from dependencies

**Failure Behavior**: Step marked as `skipped`

**Formats**: Same as DAG-level preconditions

**Examples**:
```yaml
# Check previous step output
preconditions:
  - condition: "${PREV_OUTPUT}"
    expected: "success"

# Multiple conditions
preconditions:
  # File must exist
  - test -f input.csv
  
  # Environment check
  - condition: "${ENVIRONMENT}"
    expected: "production"
  
  # Output from previous step
  - condition: "${VALIDATION_RESULT}"
    expected: "re:(PASSED|WARNING)"
  
  # Dynamic check
  - condition: "`curl -s localhost:8080/health`"
    expected: "healthy"
```

## Error Handling and Recovery

These fields provide robust error handling and recovery mechanisms.

### `continueOn` (object, optional)

**Description**: Conditions to continue execution despite failures.

**Fields**:

#### `failure` (boolean)
Continue if dependency failed

#### `skipped` (boolean)
Continue if dependency was skipped

#### `exitCode` (array of integers)
Continue on specific exit codes

#### `output` (string or array)
- Continue if output contains pattern
- Supports regex with `re:` prefix
- Searches both stdout and stderr

#### `markSuccess` (boolean)
- Mark step as success when condition met
- Useful for expected failures

**Examples**:
```yaml
# Continue on any failure
continueOn:
  failure: true

# Continue on specific exit codes
continueOn:
  exitCode: [0, 1, 2]
  markSuccess: true  # Treat as success

# Continue on expected output patterns
continueOn:
  output: 
    - "WARNING: No data found"
    - "re:SKIP:.*"
    - "re:^INFO: Already processed"
  markSuccess: true

# Complex error handling
continueOn:
  failure: true
  skipped: true
  exitCode: [0, 1, 255]
  output: ["WARNING", "NOTICE"]
```

### `mailOnError` (boolean, optional)

**Description**: Send email notification on step error.

**Default**: `false`

**Requirements**: SMTP configuration at DAG level

**Example**:
```yaml
# Critical step with email alerts
name: process-payments
command: process_payments.py
mailOnError: true
```

### `signalOnStop` (string, optional)

**Description**: Signal to send when step is stopped.

**Default**: Same signal as parent process

**Validation**: Must be valid Unix signal

**Common Signals**:
- `SIGTERM` - Graceful termination (default)
- `SIGINT` - Interrupt (Ctrl+C)
- `SIGKILL` - Force kill (cannot be caught)
- `SIGUSR1`/`SIGUSR2` - User-defined signals

**Example**:
```yaml
# Graceful shutdown for web server
name: web-server
command: python app.py
signalOnStop: SIGTERM

# Custom signal for special handling
name: data-processor
command: processor.sh
signalOnStop: SIGUSR1
```

## Retry and Repeat Policies

These fields enable automatic retry and repeat behaviors for steps.

### `retryPolicy` (object, optional)
- **Description**: Retry configuration for failed steps.

#### Fields:
- **`limit`** (integer or string)
  - Maximum retry attempts
  - String allows variable: `"${MAX_RETRIES}"`
  
- **`intervalSec`** (integer or string)
  - Seconds between retries
  - String allows variable: `"${RETRY_INTERVAL}"`
  
- **`exitCode`** (array of integers)
  - Only retry on these exit codes
  - Empty = retry on any non-zero

**Retry Decision Logic**:
- If `exitCode` specified: Retry only on those codes
- If `exitCode` empty: Retry on any non-zero exit code

**Exit Code Detection**:
1. `exec.ExitError.ExitCode()` - Standard method
2. Parse "exit status N" from error message
3. Signal termination returns -1
4. Unknown errors default to 1

**Example**:
```yaml
# Retry on any failure
retryPolicy:
  limit: 3
  intervalSec: 60

# Retry only on specific exit codes
retryPolicy:
  limit: 5
  intervalSec: 30
  exitCode: [1, 255]  # Network errors

# Dynamic retry configuration
retryPolicy:
  limit: "${MAX_RETRIES}"
  intervalSec: "${RETRY_INTERVAL}"
  exitCode: [429, 503]  # Rate limit and unavailable
```

### `repeatPolicy` (object, optional)
- **Description**: Configuration for repeating steps with explicit modes.

#### Fields:
- **`repeat`** (string)
  - Repeat mode: `"while"` or `"until"`
  - `while`: Repeats while condition is true
  - `until`: Repeats until condition is true
  
- **`intervalSec`** (integer)
  - Seconds between repetitions
  
- **`limit`** (integer)
  - Maximum number of repetitions
  
- **`condition`** (string)
  - Condition to evaluate for repetition
  
- **`expected`** (string)
  - Expected value for condition
  
- **`exitCode`** (array of integers)
  - Exit codes that trigger repeat

**Evaluation Logic**:

- **`while` mode**:
  - With condition: Repeats while condition evaluates to 0 (success)
  - With condition + expected: Repeats while condition matches expected
  - With exitCode: Repeats while exit code is in the list
  
- **`until` mode**:
  - With condition: Repeats until condition evaluates to 0 (success)
  - With condition + expected: Repeats until condition matches expected
  - With exitCode: Repeats until exit code is in the list

**Examples**:
```yaml
# Wait UNTIL service is ready
repeatPolicy:
  repeat: until
  intervalSec: 60
  condition: "`curl -s api/status | jq -r .state`"
  expected: "ready"
  limit: 30  # Maximum 30 minutes

# Repeat WHILE connection fails
repeatPolicy:
  repeat: while
  intervalSec: 30
  exitCode: [1]  # Connection refused
  limit: 60  # Maximum attempts

# Monitor UNTIL job completes
repeatPolicy:
  repeat: until
  intervalSec: 120
  condition: "${JOB_STATUS}"
  expected: "re:(COMPLETED|FAILED)"

# Keep alive WHILE process runs
repeatPolicy:
  repeat: while
  intervalSec: 300  # Every 5 minutes
  exitCode: [0]     # Process exists
```

## Output Capture and Variables

These fields allow steps to capture output and share data.

### `output` (string, optional)

**Description**: Variable name to store command output.

**Capture Process**:
  1. Pipe created before execution
  2. Stdout captured via MultiWriter
  3. Trimmed and stored after completion
  4. Available to downstream steps

**Variable Access**:
```yaml
steps:
  - name: get-data
    command: echo '{"count": 42, "status": "ok"}'
    output: RESULT
  
  - name: process
    command: echo "Count is ${RESULT}"
    env:
      - COUNT: "${RESULT.count}"      # JSON path: 42
      - STATUS: "${RESULT.status}"    # JSON path: "ok"
```

**Implementation Details**:
- Storage format: key=value pairs
- Stored in OutputVariables SyncMap
- Propagated to downstream steps
- Trimmed of whitespace before storage

**Advanced Examples**:
```yaml
# Capture and parse JSON
- name: api-call
  command: curl -s https://api.example.com/data
  output: API_RESPONSE

- name: process-data
  command: process.py
  env:
    - ITEM_COUNT: "${API_RESPONSE.items.length}"
    - FIRST_ID: "${API_RESPONSE.items[0].id}"

# Capture command output for decisions
- name: check-status
  command: systemctl is-active myservice || echo "inactive"
  output: SERVICE_STATUS

- name: restart-if-needed
  command: systemctl restart myservice
  preconditions:
    - condition: "${SERVICE_STATUS}"
      expected: "inactive"
```

## Executors

Executors provide different ways to run step commands beyond simple shell execution.

### `executor` (string or object, optional)
- **Description**: Custom executor configuration.
- **Default**: `command` executor
- **Formats**:

1. **String** (executor type only):
   ```yaml
   executor: docker
   ```

2. **Object** (with configuration):
   ```yaml
   executor:
     type: docker
     config:
       image: python:3.11
   ```

### Available Executors

#### Command Executor (default)
Executes shell commands. This is the default when no executor is specified.

#### Docker Executor

**Description**: Run commands in Docker containers.

**Configuration**:
```yaml
executor:
  type: docker
  config:
    image: python:3.11-slim
    pull: missing     # always, never, missing
    autoRemove: true
    platform: linux/amd64
    containerName: my-container  # For exec mode
    exec:            # Exec into existing container
      user: root
      workingDir: /app
    host:
      binds:
        - /data:/data:ro
        - /output:/output:rw
    container:
      env:
        - PYTHONPATH=/app
      workingDir: /app
```

**Examples**:
```yaml
# Run Python script in container
executor:
  type: docker
  config:
    image: python:3.11-alpine
    autoRemove: true
    host:
      binds:
        - ./scripts:/scripts:ro
        - ./data:/data:rw
command: python /scripts/process.py

# Exec into running container
executor:
  type: docker
  config:
    containerName: my-app
    exec:
      user: app
command: /app/bin/maintenance.sh
```

#### HTTP Executor

**Description**: Make HTTP requests.

**Configuration**:
```yaml
executor:
  type: http
  config:
    timeout: 30      # seconds
    headers:
      Authorization: "Bearer ${TOKEN}"
      Content-Type: "application/json"
    query:
      page: "1"
      limit: "100"
    body: |
      {"key": "value"}
    silent: false    # suppress output
    debug: false     # show request details
    json: true       # output as JSON
command: GET https://api.example.com/data
```

**HTTP Methods**: GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS

**Examples**:
```yaml
# GET request with query parameters
executor:
  type: http
  config:
    query:
      status: active
      limit: "50"
command: GET https://api.example.com/users
output: USERS

# POST request with JSON body
executor:
  type: http
  config:
    headers:
      Content-Type: application/json
    body: |
      {
        "name": "${USER_NAME}",
        "email": "${USER_EMAIL}"
      }
command: POST https://api.example.com/users

# Webhook notification
executor:
  type: http
  config:
    timeout: 10
    silent: true
command: POST https://hooks.slack.com/services/xxx
```

#### SSH Executor

**Description**: Execute commands on remote servers via SSH.

**Configuration**:
```yaml
executor:
  type: ssh
  config:
    host: server.example.com
    port: "22"
    user: deploy
    key: /home/user/.ssh/id_rsa
    # or password: secret
```

**Authentication**: Key-based (recommended) or password

**Examples**:
```yaml
# Remote command execution
executor:
  type: ssh
  config:
    host: prod-server.example.com
    user: deploy
    key: ~/.ssh/deploy_key
command: |
  cd /app &&
  git pull &&
  docker-compose restart

# Multiple servers (using step repetition)
steps:
  - name: deploy-web1
    executor:
      type: ssh
      config:
        host: web1.example.com
        user: deploy
        key: ~/.ssh/id_rsa
    command: systemctl restart webapp
    
  - name: deploy-web2
    executor:
      type: ssh
      config:
        host: web2.example.com
        user: deploy
        key: ~/.ssh/id_rsa
    command: systemctl restart webapp
```

#### Mail Executor

**Description**: Send emails with optional attachments.

**Configuration**:
```yaml
executor:
  type: mail
  config:
    to: "team@company.com,manager@company.com"
    from: dagu@company.com
    subject: "Pipeline Report"
    message: |
      Pipeline completed successfully.
      Results: ${RESULTS}
    attachments:
      - /logs/pipeline.log
      - ${OUTPUT_FILE}
```

**Requirements**: SMTP configuration at DAG level

**Examples**:
```yaml
# Send report with attachments
executor:
  type: mail
  config:
    to: "${REPORT_RECIPIENTS}"
    subject: "Daily Report - ${DAG_RUN_START_TIME}"
    message: |
      Daily processing complete.
      
      Summary:
      - Records processed: ${RECORD_COUNT}
      - Errors: ${ERROR_COUNT}
      - Duration: ${DURATION}
      
      See attached files for details.
    attachments:
      - /tmp/report.pdf
      - /tmp/errors.csv

# Simple notification
executor:
  type: mail
  config:
    to: oncall@company.com
    subject: "Alert: ${ALERT_TYPE}"
    message: "Alert triggered at ${DAG_RUN_START_TIME}"
```

#### JQ Executor

**Description**: Process JSON data using jq expressions.

**Configuration**:
```yaml
executor: jq
script: '.data | map(select(.active == true)) | length'
command: /data/users.json  # input file
```

**Examples**:
```yaml
# Count active users
executor: jq
script: '.users | map(select(.status == "active")) | length'
command: users.json
output: ACTIVE_COUNT

# Extract and transform data
executor: jq
script: |
  .orders
  | map({
      id: .order_id,
      total: .items | map(.price * .quantity) | add,
      customer: .customer.email
    })
  | sort_by(.total)
  | reverse
command: orders.json
output: TOP_ORDERS

# Filter and format
executor: jq
script: |
  .logs
  | map(select(.level == "ERROR"))
  | group_by(.category)
  | map({category: .[0].category, count: length})
command: application.log
```

## Child DAG Execution

These fields enable hierarchical DAG composition by running other DAGs as steps.

### `run` (string, optional)

**Description**: Run a child DAG as a step.

**Path Resolution**: Relative to DAGs directory

**Implementation**: Converted to DAG executor internally

**Example**:
```yaml
run: workflows/etl/process-data
```

### `params` (string, optional)

**Description**: Parameters to pass to child DAG.

**Format**: Space-separated key=value pairs

**Examples**:
```yaml
# Simple parameters
params: "env=prod date=${DATE}"

# Pass output from previous steps
params: "input_file=${DOWNLOAD_PATH} count=${RECORD_COUNT}"

# Complex parameter passing
steps:
  - name: prepare
    command: prepare_data.sh
    output: PREP_RESULT
    
  - name: run-etl
    run: etl/transform
    params: "config=${PREP_RESULT} batch_size=1000 parallel=true"
    output: ETL_OUTPUT
```

### Child DAG Output Access

**Execution Process**:
1. Child DAG runs as separate process
2. All step outputs collected
3. Returned as JSON structure

**Output Format**:
```json
{
  "name": "child-dag",
  "dagRunId": "550e8400-e29b-41d4-a716",
  "params": "env=prod",
  "outputs": {
    "record_count": "1523",
    "status": "success",
    "details": {
      "processed": 1500,
      "skipped": 23
    }
  }
}
```

**Parent Access Examples**:
```yaml
steps:
  - name: run-etl
    run: etl/process
    params: "date=${TARGET_DATE}"
    output: ETL_RESULT
  
  - name: validate
    command: validate.py
    env:
      - TOTAL_COUNT: "${ETL_RESULT.outputs.record_count}"
      - PROCESSED: "${ETL_RESULT.outputs.details.processed}"
      - SKIPPED: "${ETL_RESULT.outputs.details.skipped}"
    preconditions:
      - condition: "${ETL_RESULT.outputs.status}"
        expected: "success"
  
  - name: report
    run: reporting/generate
    params: |
      dag_run_id=${ETL_RESULT.dagRunId} 
      record_count=${ETL_RESULT.outputs.record_count}
```

**Hierarchical DAG Example**:
```yaml
# Parent DAG
name: master-pipeline
steps:
  - name: stage-1
    run: pipelines/extract
    output: EXTRACT
    
  - name: stage-2
    run: pipelines/transform
    params: "input=${EXTRACT.outputs.file_path}"
    output: TRANSFORM
    
  - name: stage-3
    run: pipelines/load
    params: |
      source=${TRANSFORM.outputs.output_path}
      row_count=${TRANSFORM.outputs.row_count}
```

## Advanced Features

Advanced capabilities for complex workflows.

### Variable Resolution

**Priority Order** (highest to lowest):
1. Step-specific variables
2. Output variables from dependencies
3. DAG-level parameters
4. DAG-level environment
5. System environment

**JSON Path Access**:
```yaml
# If RESULT = {"data": {"items": [{"id": 1}, {"id": 2}]}}
# Then ${RESULT.data.items[0].id} = 1
```

**Implementation**:
- Uses `gojq` for path evaluation
- Invalid paths return original placeholder
- Non-string values converted to string

### Special Variables

Available in step context:
- `DAG_NAME`: Current DAG name
- `DAG_RUN_ID`: Execution ID
- `DAG_RUN_LOG_FILE`: DAG log path
- `DAG_RUN_STEP_NAME`: Current step name
- `DAG_RUN_STEP_STDOUT_FILE`: Step stdout path
- `DAG_RUN_STEP_STDERR_FILE`: Step stderr path

### Command Evaluation

**Evaluation Phases**:
1. **Build Time**: Parameters, environment setup
2. **Execution Time**: Step commands and arguments

**Evaluation Options by Context**:
- Command executor with shell: No environment expansion
- Script field: Variable replacement only for command executors
- Other fields: Full evaluation

## Implementation Details

Internal implementation details for advanced users and contributors.

### Step Execution Lifecycle

1. **Setup Phase**:
   ```go
   // 1. Generate log file names
   // 2. Create output pipes if needed
   // 3. Setup retry policy
   // 4. Evaluate working directory
   ```

2. **Execution Phase**:
   ```go
   // 1. Check preconditions
   // 2. Setup executor
   // 3. Evaluate command/args
   // 4. Run executor
   // 5. Capture output
   ```

3. **Teardown Phase**:
   ```go
   // 1. Flush all writers
   // 2. Close pipes
   // 3. Store output variables
   // 4. Update state
   ```

### Log File Naming

Pattern: `{safe_name}.{timestamp}.{runid_prefix}`
- Safe name: Alphanumeric + limited special chars
- Timestamp: `20060102.15:04:05.000`
- RunID prefix: First 8 chars of DAG run ID

### Output Capture Implementation

```go
// MultiWriter setup
stdout := io.MultiWriter(logWriter, outputPipe)

// After execution
output := strings.TrimSpace(capturedOutput)
node.OutputVariables.Store(outputVar, output)
```

### State Management

```go
type NodeStatus int
const (
    NodeStatusNone     // Initial
    NodeStatusRunning  // Executing
    NodeStatusError    // Failed
    NodeStatusCancel   // Cancelled
    NodeStatusSuccess  // Completed
    NodeStatusSkipped  // Preconditions not met
)
```

### Panic Recovery

Each step execution wrapped in panic recovery:
```go
defer func() {
    if panicObj := recover(); panicObj != nil {
        stack := debug.Stack()
        err := fmt.Errorf("panic: %v\n%s", panicObj, stack)
        node.MarkError(err)
    }
}()
```

## Quick Reference Card

```yaml
# Step definition
name: string               # Required, unique identifier
description: string        # Human-readable description
dir: string               # Working directory
shell: string             # Shell to use (bash, sh, zsh, nix-shell)
packages: [array]         # Packages for nix-shell

# Command execution (one of these)
command: string|array     # Command to execute
script: string           # Inline script
run: string              # Child DAG path
executor: string|object  # Executor configuration

# I/O
stdout: string           # Redirect stdout to file
stderr: string           # Redirect stderr to file  
output: string           # Capture output to variable

# Flow control
depends: string|array    # Step dependencies
preconditions: array     # Pre-execution checks

# Error handling
continueOn:              # Continue despite failures
  failure: bool
  skipped: bool
  exitCode: [ints]
  output: string|array
  markSuccess: bool
mailOnError: bool        # Send email on error
signalOnStop: string     # Signal to send on stop

# Retry/Repeat
retryPolicy:
  limit: int|string
  intervalSec: int|string
  exitCode: [ints]
repeatPolicy:
  repeat: string        # "while" or "until"
  intervalSec: int
  limit: int           # Maximum repetitions
  condition: string
  expected: string
  exitCode: [ints]

# Child DAG
params: string           # Parameters for child DAG
```

## Common Patterns

### Sequential Pipeline
```yaml
steps:
  - name: extract
    command: extract_data.sh
    output: DATA_FILE
    
  - name: transform
    command: transform.py ${DATA_FILE}
    depends: extract
    output: PROCESSED_FILE
    
  - name: load
    command: load_to_db.sh ${PROCESSED_FILE}
    depends: transform
```

### Parallel Processing with Fan-in
```yaml
steps:
  - name: download
    command: download_all.sh
    
  - name: process-us
    command: process.py --region us
    depends: download
    
  - name: process-eu
    command: process.py --region eu  
    depends: download
    
  - name: process-asia
    command: process.py --region asia
    depends: download
    
  - name: aggregate
    command: aggregate_results.py
    depends: [process-us, process-eu, process-asia]
```

### Error Recovery Pattern
```yaml
steps:
  - name: risky-operation
    command: might_fail.sh
    retryPolicy:
      limit: 3
      intervalSec: 60
    continueOn:
      exitCode: [1, 2]  # Known recoverable errors
      
  - name: cleanup-on-failure
    command: cleanup.sh
    depends: risky-operation
    continueOn:
      failure: true  # Always run cleanup
```

### Conditional Execution Pattern
```yaml
steps:
  - name: check-environment
    command: detect_env.sh
    output: ENV_TYPE
    
  - name: dev-workflow
    run: workflows/development
    depends: check-environment
    preconditions:
      - condition: "${ENV_TYPE}"
        expected: "development"
        
  - name: prod-workflow
    run: workflows/production
    depends: check-environment
    preconditions:
      - condition: "${ENV_TYPE}"
        expected: "production"
```

---

This specification covers all step-level fields and their implementation details based on source code analysis. For DAG-level configuration, see [DAG_YAML.md](./DAG_YAML.md).
