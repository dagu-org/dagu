# Writing Workflows

## Workflow Structure

```yaml
name: my-workflow          # Optional: defaults to filename
description: "Process daily data"
schedule: "0 2 * * *"      # Optional: cron schedule
maxActiveRuns: 1           # Optional: concurrency limit

params:                    # Runtime parameters
  - DATE: "`date +%Y-%m-%d`"

env:                       # Environment variables
  - DATA_DIR: /tmp/data

steps:                     # Workflow steps
  - name: process
    command: echo "Processing for date ${DATE}"
```

## Base Configuration

Share common settings across all DAGs using base configuration:

```yaml
# ~/.config/dagu/base.yaml
env:
  - LOG_LEVEL: info
  - AWS_REGION: us-east-1

smtp:
  host: smtp.company.com
  port: "587"
  username: ${SMTP_USER}
  password: ${SMTP_PASS}

errorMail:
  from: alerts@company.com
  to: oncall@company.com
  attachLogs: true

histRetentionDays: 30 # Dagu deletes workflow history and logs older than this
maxActiveRuns: 5
```

DAGs automatically inherit these settings:

```yaml
# my-workflow.yaml
name: data-pipeline

# Inherits all base settings
# Can override specific values:
env:
  - LOG_LEVEL: debug  # Override
  - CUSTOM_VAR: value # Addition

steps:
  - name: process
    command: echo "Processing"
```

Configuration precedence: System defaults → Base config → DAG config

## Guide Sections

1. **[Basics](/writing-workflows/basics)** - Steps, commands, dependencies
2. **[Container](/writing-workflows/container)** - Run workflows in Docker containers
3. **[Control Flow](/writing-workflows/control-flow)** - Parallel execution, conditions, loops
4. **[Data & Variables](/writing-workflows/data-variables)** - Parameters, outputs, data passing
5. **[Error Handling](/writing-workflows/error-handling)** - Retries, failures, notifications
6. **[Advanced Patterns](/writing-workflows/advanced)** - Composition, optimization, best practices

## Complete Example

```yaml
name: data-processor
schedule: "0 2 * * *"

params:
  - DATE: "`date +%Y-%m-%d`"

env:
  - DATA_DIR: /tmp/data/${DATE}

steps:
  - name: download
    command: aws s3 cp s3://bucket/${DATE}.csv ${DATA_DIR}/
    retryPolicy:
      limit: 3
      intervalSec: 60

  - name: validate
    command: python validate.py ${DATA_DIR}/${DATE}.csv
    continueOn:
      failure: false

  - name: process-types
    parallel: [users, orders, products]
    command: python process.py --type=$ITEM --date=${DATE}
    output: RESULT_${ITEM}

  - name: report
    command: python report.py --date=${DATE}

handlerOn:
  failure:
    command: echo "Notifying failure for ${DATE}"
```

## Common Patterns

### Sequential Pipeline
```yaml
steps:
  - name: extract
    command: echo "Extracting data"
    
  - name: transform
    command: echo "Transforming data"
    
  - name: load
    command: echo "Loading data"
```

### Conditional Execution
```yaml
steps:
  - name: test
    command: npm test
    
  - name: deploy
    command: echo "Deploying application"
    preconditions:
      - condition: "${BRANCH}"
        expected: "main"
```

### Parallel Processing
```yaml
steps:
  - name: prepare
    command: echo "Preparing environment"
    
  - name: process-files
    parallel: [file1, file2, file3]
    run: process-file
    params: "FILE=${ITEM}"

---
# A child workflow for processing each file
# This can be in a same file separated by `---` or in a separate file
name: process-file
steps:
  - name: process
    command: echo "Processing" --file ${FILE}
```

The above example runs `process-file` in different processes for each file in parallel.

## Next Steps

- [Basics](/writing-workflows/basics) - Start here
- [Examples](/writing-workflows/examples) - Complete workflow examples
