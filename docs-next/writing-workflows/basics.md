# Workflow Basics

Learn the fundamentals of writing Dagu workflows.

## Your First Workflow

Create a file named `hello.yaml`:

```yaml
steps:
  - name: say-hello
    command: echo "Hello from Dagu!"
```

Run it:
```bash
dagu start hello.yaml
```

## Workflow Structure

Every workflow consists of:
- **Metadata**: Name, description, tags
- **Configuration**: Schedule, timeouts, parameters
- **Steps**: The actual tasks to execute
- **Handlers**: Optional lifecycle hooks

```yaml
# Metadata
name: my-workflow
description: "Process daily data"
tags: [etl, production]

# Configuration  
schedule: "0 2 * * *"  # 2 AM daily
params:
  - DATE: "`date +%Y-%m-%d`"

# Steps
steps:
  - name: process
    command: python process.py --date=${DATE}
```

## Steps

Steps are the building blocks of workflows.

### Basic Command

```yaml
steps:
  - name: simple-command
    command: echo "Hello World"
```

### Multi-line Scripts

Use the `script` field for complex commands:

```yaml
steps:
  - name: complex-script
    script: |
      #!/bin/bash
      set -e
      
      echo "Starting process..."
      
      # Download data
      curl -o data.json https://api.example.com/data
      
      # Process data
      python process.py data.json
      
      # Cleanup
      rm data.json
      
      echo "Process complete!"
```

### Shell Selection

Specify which shell to use:

```yaml
steps:
  - name: bash-script
    shell: bash
    command: echo $BASH_VERSION
    
  - name: python-inline
    shell: python3
    script: |
      import sys
      print(f"Python {sys.version}")
      
  - name: custom-shell
    shell: /usr/local/bin/zsh
    command: echo $ZSH_VERSION
```

## Sequential Execution

By default, steps run one after another:

```yaml
steps:
  - name: first
    command: echo "Step 1"
    
  - name: second
    command: echo "Step 2"
    depends: first
    
  - name: third
    command: echo "Step 3"
    depends: second
```

## Parallel Execution

Steps without dependencies run in parallel:

```yaml
steps:
  - name: download-a
    command: wget https://example.com/file-a.zip
    
  - name: download-b
    command: wget https://example.com/file-b.zip
    
  - name: download-c
    command: wget https://example.com/file-c.zip
    
  - name: process-all
    command: ./process.sh
    depends:
      - download-a
      - download-b
      - download-c
```

## Working Directory

Set working directory per step:

```yaml
steps:
  - name: in-project-dir
    dir: /home/user/project
    command: ./build.sh
    
  - name: in-temp-dir
    dir: /tmp
    command: echo "Working in $(pwd)"
```

## Environment Variables

### Workflow-level Environment

```yaml
env:
  - API_KEY: ${SECRET_API_KEY}
  - LOG_LEVEL: debug

steps:
  - name: use-env
    command: echo "API Key length: ${#API_KEY}"
```

### Step-level Environment

```yaml
steps:
  - name: custom-env
    env:
      - NODE_ENV: production
      - PORT: 3000
    command: node server.js
```

### Loading from .env Files

```yaml
dotenv:
  - .env
  - .env.production

steps:
  - name: use-dotenv
    command: echo "Loaded from .env: $MY_VAR"
```

## Parameters

Pass parameters to workflows:

```yaml
params:
  - NAME: World
  - GREETING: Hello

steps:
  - name: greet
    command: echo "${GREETING}, ${NAME}!"
```

Run with custom parameters:
```bash
dagu start hello.yaml -- NAME=Dagu GREETING=Hi
```

## Capturing Output

Save command output for later use:

```yaml
steps:
  - name: get-date
    command: date +%Y-%m-%d
    output: TODAY
    
  - name: use-date
    command: echo "Today is ${TODAY}"
    depends: get-date
```

## File Output

Redirect output to files:

```yaml
steps:
  - name: save-logs
    command: ./generate-report.sh
    stdout: /logs/report.out
    stderr: /logs/report.err
```

## Running Sub-workflows

Execute other DAG files:

```yaml
steps:
  - name: run-etl
    run: etl.yaml
    params: "DATE=${DATE} SOURCE=production"
    
  - name: run-analytics
    run: analytics.yaml
    depends: run-etl
```

## Workflow Metadata

Add metadata for organization:

```yaml
name: data-pipeline
description: "Daily data processing pipeline"
tags:
  - production
  - etl
  - critical
group: "Data Team"

steps:
  - name: process
    command: ./process.sh
```

## Common Patterns

### Setup and Teardown

```yaml
steps:
  - name: setup
    command: ./setup.sh
    
  - name: main-process
    command: ./process.sh
    depends: setup
    
  - name: cleanup
    command: ./cleanup.sh
    depends: main-process
    continueOn:
      failure: true  # Always run cleanup
```

### Conditional Processing

```yaml
params:
  - ENVIRONMENT: dev

steps:
  - name: validate
    command: ./validate.sh
    
  - name: deploy
    command: ./deploy.sh
    depends: validate
    preconditions:
      - condition: "${ENVIRONMENT}"
        expected: "production"
```

### Batch Processing

```yaml
steps:
  - name: get-files
    command: ls /data/*.csv
    output: FILES
    
  - name: process-files
    run: process-single-file
    parallel: ${FILES}
    depends: get-files
```

## See Also

- [Control Flow](/writing-workflows/control-flow) - Dependencies and conditions
- [Data & Variables](/writing-workflows/data-variables) - Working with data
- [Error Handling](/writing-workflows/error-handling) - Handle failures
