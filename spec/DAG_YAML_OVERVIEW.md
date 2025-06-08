# Dagu DAG YAML Specification Overview

This document provides a comprehensive overview of the Dagu DAG YAML format, the primary way to define workflows in Dagu. Dagu uses YAML files to describe Directed Acyclic Graphs (DAGs) that orchestrate complex workflows with dependencies, scheduling, and error handling.

## Documentation Structure

- **[DAG_YAML.md](./DAG_YAML.md)** - Complete specification for DAG-level configuration
  - Metadata, scheduling, environment, parameters, handlers, notifications
- **[DAG_YAML_STEP.md](./DAG_YAML_STEP.md)** - Complete specification for step-level configuration  
  - Commands, dependencies, executors, error handling, output management

## Quick Start Examples

### Minimal DAG

The simplest possible DAG requires only a list of steps:

```yaml
steps:
  - name: hello
    command: echo "Hello, World!"
```

### Basic DAG with Dependencies

Steps can depend on other steps to create execution order:

```yaml
steps:
  - name: download
    command: wget https://example.com/data.csv
    
  - name: process
    command: python process.py data.csv
    depends: download
    
  - name: upload
    command: aws s3 cp output.csv s3://bucket/
    depends: process
```

### Scheduled DAG with Error Handling

A production-ready DAG with scheduling and notifications:

```yaml
name: daily-report
schedule: "0 8 * * *"  # Every day at 8 AM
mailOn:
  failure: true

steps:
  - name: generate-report
    command: python generate_report.py
    retryPolicy:
      limit: 3
      intervalSec: 300
    
  - name: send-report
    depends: generate-report
    executor:
      type: mail
      config:
        to: team@company.com
        subject: "Daily Report"
        message: "Please find attached the daily report"
        attachments:
          - /tmp/report.pdf
```

### Advanced DAG with All Features

Comprehensive example showcasing Dagu's capabilities:

```yaml
name: data-pipeline
description: "Production data processing pipeline"
schedule: "0 2 * * *"
group: ETL
tags: [production, critical]

# Environment configuration
env:
  - ENVIRONMENT: production
  - LOG_LEVEL: info
  - DB_HOST: ${DB_HOST:-localhost}

# Dynamic parameters
params:
  - TARGET_DATE: "`date -d '1 day ago' +%Y-%m-%d`"
  - BATCH_SIZE: "1000"

# DAG-level preconditions
preconditions:
  - condition: "`date +%u`"
    expected: "re:[1-5]"  # Weekdays only

# Execution controls
maxActiveRuns: 1
maxActiveSteps: 5
timeout: 7200  # 2 hours
histRetentionDays: 30

# Lifecycle handlers
handlerOn:
  success:
    command: |
      slack-notify.sh "Pipeline completed: ${DAG_NAME}"
  failure:
    command: |
      pagerduty-alert.sh "Pipeline failed: ${DAG_NAME}"
      echo "Check logs at: ${DAG_RUN_LOG_FILE}"
  exit:
    command: cleanup-temp-files.sh

# Email configuration
mailOn:
  failure: true
  success: false
smtp:
  host: smtp.gmail.com
  port: "587"
  username: ${SMTP_USER}
  password: ${SMTP_PASS}
errorMail:
  from: alerts@company.com
  to: oncall@company.com
  attachLogs: true

# Workflow steps
steps:
  # Step 1: Validate prerequisites
  - name: validate
    command: |
      echo "Validating environment..."
      check-db-connection.sh
      check-disk-space.sh
    output: VALIDATION_STATUS
    
  # Step 2: Extract data
  - name: extract
    command: |
      python extract.py \
        --date ${TARGET_DATE} \
        --batch-size ${BATCH_SIZE}
    depends: validate
    preconditions:
      - condition: "${VALIDATION_STATUS}"
        expected: "OK"
    retryPolicy:
      limit: 3
      intervalSec: 300
      exitCode: [1, 255]
    output: EXTRACT_COUNT
    
  # Step 3: Transform data (parallel processing)
  - name: transform-customers
    command: python transform.py --type customers
    depends: extract
    dir: /app/transformers
    
  - name: transform-orders
    command: python transform.py --type orders
    depends: extract
    dir: /app/transformers
    
  - name: transform-products
    command: python transform.py --type products
    depends: extract
    dir: /app/transformers
    
  # Step 4: Quality checks
  - name: quality-check
    command: |
      python quality_check.py \
        --threshold 0.95 \
        --date ${TARGET_DATE}
    depends:
      - transform-customers
      - transform-orders
      - transform-products
    continueOn:
      failure: true
      output: ["WARNING:.*", "NOTICE:.*"]
      markSuccess: true
    output: QC_RESULT
    
  # Step 5: Load to warehouse
  - name: load
    command: |
      python load_to_warehouse.py \
        --date ${TARGET_DATE} \
        --validate
    depends: quality-check
    preconditions:
      - condition: "${QC_RESULT}"
        expected: "re:(PASSED|WARNING)"
    executor:
      type: docker
      config:
        image: company/etl-loader:latest
        volumes:
          - /data:/data:ro
        env:
          - WAREHOUSE_URL=${WAREHOUSE_URL}
          
  # Step 6: Run nested DAG for reporting
  - name: generate-reports
    run: workflows/reporting
    params: "DATE=${TARGET_DATE} STATUS=${QC_RESULT}"
    depends: load
    output: REPORT_RESULTS
    
  # Step 7: Cleanup
  - name: cleanup
    command: |
      rm -f /tmp/etl_${TARGET_DATE}_*.tmp
      echo "Pipeline completed successfully"
    depends: generate-reports
    continueOn:
      failure: true
```

## Key Concepts

### DAG Structure
- **DAG**: Directed Acyclic Graph defining workflow execution order
- **Steps**: Individual units of work (commands, scripts, or nested DAGs)
- **Dependencies**: Define execution order between steps

### Execution Model
- Steps run in order based on dependencies
- Parallel execution when dependencies allow
- Each step runs in its own process
- Output can be captured and passed between steps

### Error Handling
- Retry policies for transient failures
- Continue-on conditions for graceful degradation
- Lifecycle handlers for cleanup and notifications
- Email alerts with log attachments

### Scheduling
- Cron-based scheduling with timezone support
- Skip redundant runs
- Control concurrent executions
- History retention management

## Common Patterns

### Sequential Processing
```yaml
steps:
  - name: step1
    command: echo "First"
  - name: step2
    command: echo "Second"
    depends: step1
  - name: step3
    command: echo "Third"
    depends: step2
```

### Parallel Processing
```yaml
steps:
  - name: download
    command: download-data.sh
  - name: process-a
    command: process-a.py
    depends: download
  - name: process-b
    command: process-b.py
    depends: download
  - name: combine
    command: combine-results.py
    depends: [process-a, process-b]
```

### Conditional Execution
```yaml
steps:
  - name: check
    command: check-condition.sh
    output: SHOULD_PROCEED
  - name: process
    command: process.py
    depends: check
    preconditions:
      - condition: "${SHOULD_PROCEED}"
        expected: "YES"
```

### Dynamic Workflows
```yaml
steps:
  - name: detect-env
    command: detect-environment.sh
    output: ENV_TYPE
  - name: run-dev-workflow
    run: workflows/dev
    depends: detect-env
    preconditions:
      - condition: "${ENV_TYPE}"
        expected: "development"
  - name: run-prod-workflow
    run: workflows/prod
    depends: detect-env
    preconditions:
      - condition: "${ENV_TYPE}"
        expected: "production"
```

## Best Practices

1. **Use descriptive names** for DAGs and steps
2. **Set appropriate timeouts** to prevent hanging workflows
3. **Implement retry policies** for network-dependent operations
4. **Use preconditions** to validate requirements before execution
5. **Capture outputs** for debugging and step communication
6. **Configure notifications** for critical workflows
7. **Use tags and groups** for organization
8. **Implement cleanup handlers** to prevent resource leaks
9. **Version control** your DAG definitions
10. **Test with dry runs** before deploying to production

## Additional Technical Documentation

For deep implementation details and advanced topics:
- [Parsing and Loading Architecture](./DAG_YAML.md#implementation-details)
- [Variable System Deep Dive](./DAG_YAML_STEP.md#variable-resolution)
- [Child DAG Output Handling](./DAG_YAML_STEP.md#child-dag-output-access)
- [Execution Flow and State Management](./DAG_YAML_STEP.md#step-execution-lifecycle)

## Next Steps

For complete details on all available options:
- Review [DAG_YAML.md](./DAG_YAML.md) for DAG-level configuration
- Review [DAG_YAML_STEP.md](./DAG_YAML_STEP.md) for step-level configuration
- Check the [examples/](../examples/) directory for real-world use cases