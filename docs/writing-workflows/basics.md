# Workflow Basics

Learn the fundamentals of writing Dagu workflows.

## Your First Workflow

Create `hello.yaml`:

```yaml
steps:
  - name: hello
    command: echo "Hello from Dagu!"
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

### Simple Commands

```yaml
steps:
  - name: download
    command: wget https://example.com/data.csv
```

### Multi-line Scripts

```yaml
steps:
  - name: process
    script: |
      #!/bin/bash
      set -e
      
      echo "Processing..."
      python analyze.py data.csv
      echo "Complete"
```

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

## Working Directory

Set where commands execute:

```yaml
steps:
  - name: in-project
    dir: /home/user/project
    command: python main.py
    
  - name: in-data
    dir: /data/input
    command: ls -la
```

## Environment Variables

### Global Environment

```yaml
env:
  - API_KEY: secret123
  - ENV: production

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
    command: ./process.sh
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
