# Workflow Basics

Learn the fundamentals of writing Dagu workflows.

## Your First Workflow

Create `hello.yaml`:

```yaml
steps:
  - echo "Hello from Dagu!"
```

Run it:
```bash
dagu start hello.yaml
```

## Workflow Structure

A complete workflow contains:

```yaml
# Metadata
name: data-pipeline
description: Process daily data
tags: [etl, production]

# Configuration  
schedule: "0 2 * * *"
params:
  - DATE: ${DATE:-today}

# Steps
steps:
  - name: process
    command: python process.py ${DATE}

# Handlers
handlerOn:
  failure:
    command: notify-error.sh
```

## Steps

The basic unit of execution.

### Step Names

Step names are optional. When omitted, Dagu automatically generates names based on the step type:

```yaml
steps:
  - echo "First step"              # Auto-named: cmd_1
  - script: |                      # Auto-named: script_2
      echo "Multi-line"
      echo "Script"
  - name: explicit-name            # Explicit name
    command: echo "Third step"
  - executor:                      # Auto-named: http_4
      type: http
      config:
        url: https://api.example.com
  - call: child-workflow            # Auto-named: dag_5
```

Auto-generated names follow the pattern `{type}_{number}`:
- `cmd_N` - Command steps
- `script_N` - Script steps  
- `http_N` - HTTP executor steps
- `dag_N` - DAG executor steps
- `container_N` - Docker/container steps
- `ssh_N` - SSH executor steps
- `mail_N` - Mail executor steps
- `jq_N` - JQ executor steps

For parallel steps (see below), the pattern is `parallel_{group}_{type}_{index}`.

### Shorthand Command Syntax

For simple commands, you can use an even more concise syntax:

```yaml
steps:
  - echo "Hello World"
  - ls -la
  - python script.py
```

This is equivalent to:

```yaml
steps:
  - name: step 1
    command: echo "Hello World"
  - name: step 2
    command: ls -la
    depends: step 1
  - name: step 3
    command: python script.py
    depends: step 2
```

### Multi-line Scripts

```yaml
steps:
  - script: |
      #!/bin/bash
      set -e
      
      echo "Processing..."
      python analyze.py data.csv
      echo "Complete"
```

If you omit `shell`, Dagu uses the interpreter declared in the script's shebang (`#!`) when present.

### Shell Selection

```yaml
steps:
  - name: bash-task
    shell: bash
    command: echo $BASH_VERSION
    
  - name: python-task
    shell: python3
    script: |
      import pandas as pd
      df = pd.read_csv('data.csv')
      print(df.head())
```

## Dependencies

Steps run sequentially by default. Use `depends` for parallel execution or to control order.

```yaml
steps:
  - name: download
    command: wget data.csv
    
  - name: process
    command: python process.py
    
  - name: upload
    command: aws s3 cp output.csv s3://bucket/
```

### Parallel Execution

You can run steps in parallel using explicit dependencies:

```yaml
steps:
  - name: setup
    command: echo "Setup"
    
  - name: task1
    command: echo "Task 1"
    depends: setup
    
  - name: task2
    command: echo "Task 2"
    depends: setup
    
  - name: finish
    command: echo "All tasks complete"
    depends: [task1, task2]
```

### Shorthand Parallel Syntax

For simpler cases, you can use nested arrays to define parallel steps with automatic dependency management:

```yaml
steps:
  - echo "Step 1: Sequential"
  - 
    - echo "Step 2a: Parallel"
    - echo "Step 2b: Parallel"
    - echo "Step 2c: Parallel"
  - echo "Step 3: Sequential"
```

In this syntax:
- Steps in a nested array run in parallel
- They automatically depend on the previous sequential step
- The next sequential step automatically depends on all parallel steps

This is equivalent to:

```yaml
steps:
  - name: cmd_1
    command: echo "Step 1: Sequential"
    
  - name: parallel_2_echo_1
    command: echo "Step 2a: Parallel"
    depends: cmd_1
    
  - name: parallel_2_echo_2
    command: echo "Step 2b: Parallel"
    depends: cmd_1
    
  - name: parallel_2_echo_3
    command: echo "Step 2c: Parallel"
    depends: cmd_1
    
  - name: cmd_3
    command: echo "Step 3: Sequential"
    depends: [parallel_2_echo_1, parallel_2_echo_2, parallel_2_echo_3]
```

You can have multiple parallel groups:

```yaml
steps:
  - echo "Start"
  - 
    - echo "First parallel group - task 1"
    - echo "First parallel group - task 2"
  - echo "Middle sequential step"
  -
    - echo "Second parallel group - task 1"
    - echo "Second parallel group - task 2"
    - echo "Second parallel group - task 3"
  - echo "End"
```

You can also mix shorthand and standard syntax:

```yaml
steps:
  - name: setup
    command: ./setup.sh
  - 
    - echo "Parallel task 1"
    - name: test
      command: npm test
      env:
        - NODE_ENV: test
  - name: cleanup
    command: ./cleanup.sh
```

## Working Directory

Set where commands execute:

```yaml
steps:
  - name: in-project
    workingDir: /home/user/project
    command: python main.py
    
  - name: in-data
    workingDir: /data/input
    command: ls -la
```

## Environment Variables

### System Environment Security

Dagu filters environment variables passed to step processes for security:
- System variables work in DAG YAML for expansion: `${VAR}`
- Only filtered variables are passed to step execution environment
- **Whitelisted:** `PATH`, `HOME`, `LANG`, `TZ`, `SHELL`
- **Allowed Prefixes:** `DAGU_*`, `LC_*`, `DAG_*`

To make other variables available in step processes, define them in `env`.

### Global Environment

```yaml
env:
  - API_KEY: secret123
  - ENV: production
  - AWS_ACCESS_KEY_ID: ${AWS_ACCESS_KEY_ID}  # Explicit reference required

steps:
  - name: use-env
    command: echo "Running in $ENV"
```

### Step-Level Environment

Steps can have their own environment variables that override DAG-level ones:

```yaml
env:
  - ENV: production

steps:
  - name: dev-test
    command: echo "Running in $ENV"
    env:
      - ENV: development  # Overrides DAG-level
      - TEST_FLAG: true
    # Output: Running in development
```

### Load from .env Files

```yaml
dotenv:
  - .env
  - .env.production

steps:
  - name: use-dotenv
    command: echo $DATABASE_URL
```

## Capturing Output

Store command output in variables:

```yaml
steps:
  - name: get-version
    command: git rev-parse --short HEAD
    output: VERSION
    
  - name: build
    command: docker build -t app:${VERSION} .
```

## Basic Error Handling

### Continue on Failure

```yaml
steps:
  - name: optional-step
    command: maybe-fails.sh
    continueOn:
      failure: true
      
  - name: always-runs
    command: cleanup.sh
```

### Simple Retry

```yaml
steps:
  - name: flaky-api
    command: curl https://unstable-api.com
    retryPolicy:
      limit: 3
```

## Timeouts

Prevent steps from running forever:

```yaml
steps:
  - name: long-task
    command: echo "Processing data"
    timeoutSec: 300  # 5 minutes
```

## Step Descriptions

Document your steps:

```yaml
steps:
  - name: etl-process
    description: |
      Extract data from API, transform to CSV,
      and load into data warehouse
    command: python etl.py
```

## Tags and Organization

Group related workflows:

```yaml
name: customer-report
tags: 
  - reports
  - customer
  - daily

group: Analytics  # UI grouping
```

## See Also

- [Control Flow](/writing-workflows/control-flow) - Conditionals and loops
- [Data & Variables](/writing-workflows/data-variables) - Pass data between steps
- [Error Handling](/writing-workflows/error-handling) - Advanced error recovery
- [Parameters](/writing-workflows/parameters) - Make workflows configurable
