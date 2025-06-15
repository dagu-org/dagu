# DAG Executor

The DAG executor allows you to execute other DAGs as steps within your workflow, enabling powerful workflow composition and modular design.

## Overview

The DAG executor enables you to:

- Execute external DAG files as workflow steps
- Run local DAGs defined in the same file
- Pass parameters to child DAGs
- Capture outputs from child DAGs
- Build hierarchical workflow systems
- Create reusable workflow components
- Maintain full isolation between DAG executions

## Basic Usage

### Execute External DAG

```yaml
steps:
  - name: run-etl
    executor: dag
    command: workflows/etl.yaml
    params: "DATE=2024-01-01 ENV=production"
```

### Execute Local DAG

```yaml
name: main-workflow
steps:
  - name: run-local
    executor: dag
    command: data-processor
    params: "TYPE=daily"

---

name: data-processor
params:
  - TYPE: "batch"
steps:
  - name: process
    command: echo "Processing ${TYPE} data"
```

## Parameter Passing

### Key-Value Parameters

```yaml
steps:
  - name: deploy-app
    executor: dag
    command: deployment/deploy.yaml
    params: "VERSION=1.2.3 ENVIRONMENT=staging TARGET=us-east-1"
```

### Dynamic Parameters

```yaml
env:
  - BASE_PATH: /data/input

steps:
  - name: get-date
    command: date +%Y%m%d
    output: TODAY

  - name: process-daily
    executor: dag
    command: processors/daily.yaml
    params: "DATE=${TODAY} INPUT_PATH=${BASE_PATH}/${TODAY}"
    depends: get-date
```

### Parameter Arrays

```yaml
params:
  - REGIONS: "us-east-1 us-west-2 eu-west-1"

steps:
  - name: multi-region-deploy
    executor: dag
    command: deploy.yaml
    params: "REGIONS='${REGIONS}' VERSION=${VERSION}"
```

## Output Handling

### Capture Child DAG Outputs

```yaml
steps:
  - name: analyze-data
    executor: dag
    command: analyzers/metrics.yaml
    params: "INPUT=/data/sales.csv"
    output: ANALYSIS

  - name: use-results
    command: |
      echo "Analysis completed"
      echo "Status: ${ANALYSIS.status}"
      echo "Record count: ${ANALYSIS.outputs.recordCount}"
      echo "Anomalies: ${ANALYSIS.outputs.anomalyCount}"
    depends: analyze-data
```

### Output Structure

The output from a DAG executor contains:

```json
{
  "name": "metrics",
  "params": "INPUT=/data/sales.csv",
  "status": "succeeded",
  "outputs": {
    "recordCount": "1523",
    "anomalyCount": "7",
    "reportPath": "/data/reports/analysis_20240115.pdf"
  }
}
```

## Real-World Examples

### Modular ETL Pipeline

```yaml
# main-etl.yaml
name: daily-etl-pipeline
schedule: "0 2 * * *"

steps:
  - name: extract-sales
    executor: dag
    command: etl/extract.yaml
    params: "SOURCE=sales_db TABLE=transactions"
    output: SALES_DATA

  - name: extract-inventory
    executor: dag
    command: etl/extract.yaml
    params: "SOURCE=inventory_db TABLE=products"
    output: INVENTORY_DATA

  - name: transform-data
    executor: dag
    command: etl/transform.yaml
    params: |
      SALES_FILE=${SALES_DATA.outputs.file}
      INVENTORY_FILE=${INVENTORY_DATA.outputs.file}
    output: TRANSFORMED
    depends:
      - extract-sales
      - extract-inventory

  - name: load-warehouse
    executor: dag
    command: etl/load.yaml
    params: "INPUT=${TRANSFORMED.outputs.file} TARGET=warehouse"
    depends: transform-data
```

```yaml
# etl/extract.yaml
name: extract
params:
  - SOURCE: ""
  - TABLE: ""

steps:
  - name: extract-data
    command: |
      OUTPUT_FILE="/tmp/extract_${SOURCE}_${TABLE}_$(date +%Y%m%d).csv"
      ./extract_tool.sh --source ${SOURCE} --table ${TABLE} --output $OUTPUT_FILE
      echo $OUTPUT_FILE
    output: EXTRACTED_FILE

  - name: validate
    command: |
      if [ ! -s "${EXTRACTED_FILE}" ]; then
        echo "ERROR: Extracted file is empty"
        exit 1
      fi
      wc -l "${EXTRACTED_FILE}"
    depends: extract-data

handlerOn:
  exit:
    command: echo "file=${EXTRACTED_FILE}" > $DAG_RUN_OUTPUT_FILE
```

### Multi-Environment Deployment

```yaml
# deploy-all.yaml
name: multi-env-deployment
params:
  - VERSION: latest
  - ENVIRONMENTS: "dev staging prod"

steps:
  - name: build-artifacts
    executor: dag
    command: ci/build.yaml
    params: "VERSION=${VERSION}"
    output: BUILD

  - name: deploy-dev
    executor: dag
    command: deploy/environment.yaml
    params: |
      ENV=dev
      VERSION=${VERSION}
      ARTIFACT=${BUILD.outputs.artifact}
    output: DEV_DEPLOY
    depends: build-artifacts

  - name: run-tests
    executor: dag
    command: tests/integration.yaml
    params: "ENVIRONMENT=dev VERSION=${VERSION}"
    output: TEST_RESULTS
    depends: deploy-dev

  - name: deploy-staging
    executor: dag
    command: deploy/environment.yaml
    params: |
      ENV=staging
      VERSION=${VERSION}
      ARTIFACT=${BUILD.outputs.artifact}
    preconditions:
      - condition: "${TEST_RESULTS.outputs.status}"
        expected: "passed"
    depends: run-tests

  - name: deploy-prod
    executor: dag
    command: deploy/environment.yaml
    params: |
      ENV=prod
      VERSION=${VERSION}
      ARTIFACT=${BUILD.outputs.artifact}
    preconditions:
      - condition: "`date +%u`"
        expected: "re:[1-5]"  # Weekdays only
    depends: deploy-staging
```

### Workflow Factory Pattern

```yaml
# workflow-factory.yaml
name: dynamic-workflow-execution
params:
  - WORKFLOW_TYPE: "standard"
  - CONFIG_FILE: "/etc/workflows/config.json"

steps:
  - name: determine-workflow
    command: |
      case "${WORKFLOW_TYPE}" in
        "standard")
          echo "workflows/standard-process.yaml"
          ;;
        "express")
          echo "workflows/express-process.yaml"
          ;;
        "custom")
          # Read from config
          jq -r '.customWorkflow' ${CONFIG_FILE}
          ;;
        *)
          echo "ERROR: Unknown workflow type: ${WORKFLOW_TYPE}"
          exit 1
          ;;
      esac
    output: WORKFLOW_PATH

  - name: execute-workflow
    executor: dag
    command: ${WORKFLOW_PATH}
    params: "CONFIG=${CONFIG_FILE}"
    depends: determine-workflow
```

### Recursive Processing

```yaml
# batch-processor.yaml
name: batch-processor
params:
  - BATCH_SIZE: 100
  - OFFSET: 0

steps:
  - name: get-batch
    command: |
      # Get next batch of items
      ./get_items.sh --limit ${BATCH_SIZE} --offset ${OFFSET}
    output: ITEMS

  - name: count-items
    command: echo "${ITEMS}" | jq 'length'
    output: ITEM_COUNT
    depends: get-batch

  - name: process-batch
    command: |
      echo "Processing ${ITEM_COUNT} items starting at offset ${OFFSET}"
      echo "${ITEMS}" | ./process_items.sh
    preconditions:
      - condition: "test ${ITEM_COUNT} -gt 0"
    depends: count-items

  - name: process-next-batch
    executor: dag
    command: batch-processor.yaml  # Recursive call
    params: "BATCH_SIZE=${BATCH_SIZE} OFFSET=$((OFFSET + BATCH_SIZE))"
    preconditions:
      - condition: "test ${ITEM_COUNT} -eq ${BATCH_SIZE}"
    depends: process-batch
```

## Advanced Patterns

### Conditional DAG Execution

```yaml
steps:
  - name: check-environment
    command: |
      if [ -f /etc/production ]; then
        echo "production"
      else
        echo "development"
      fi
    output: ENVIRONMENT

  - name: run-production-workflow
    executor: dag
    command: workflows/production.yaml
    preconditions:
      - condition: "${ENVIRONMENT}"
        expected: "production"
    depends: check-environment

  - name: run-dev-workflow
    executor: dag
    command: workflows/development.yaml
    preconditions:
      - condition: "${ENVIRONMENT}"
        expected: "development"
    depends: check-environment
```

### Parallel DAG Execution

```yaml
name: parallel-processing
params:
  - REGIONS: "us-east us-west eu-central ap-south"

steps:
  - name: process-us-east
    executor: dag
    command: regional/processor.yaml
    params: "REGION=us-east"

  - name: process-us-west
    executor: dag
    command: regional/processor.yaml
    params: "REGION=us-west"

  - name: process-eu-central
    executor: dag
    command: regional/processor.yaml
    params: "REGION=eu-central"

  - name: process-ap-south
    executor: dag
    command: regional/processor.yaml
    params: "REGION=ap-south"

  - name: aggregate-results
    executor: dag
    command: regional/aggregator.yaml
    depends:
      - process-us-east
      - process-us-west
      - process-eu-central
      - process-ap-south
```

### Error Handling with Child DAGs

```yaml
steps:
  - name: risky-operation
    executor: dag
    command: operations/risky.yaml
    continueOn:
      failure: true
    output: RESULT

  - name: handle-failure
    executor: dag
    command: operations/failure-handler.yaml
    params: "ERROR_TYPE=${RESULT.status}"
    preconditions:
      - condition: "${RESULT.status}"
        expected: "failed"
    depends: risky-operation

  - name: continue-success
    executor: dag
    command: operations/next-step.yaml
    preconditions:
      - condition: "${RESULT.status}"
        expected: "succeeded"
    depends: risky-operation
```

## DAG Composition Patterns

### Library of Reusable Components

```yaml
# lib/validators/file-validator.yaml
name: file-validator
params:
  - FILE_PATH: ""
  - MIN_SIZE: "0"
  - MAX_SIZE: "1073741824"  # 1GB

steps:
  - name: validate
    command: |
      if [ ! -f "${FILE_PATH}" ]; then
        echo "ERROR: File not found: ${FILE_PATH}"
        exit 1
      fi
      
      SIZE=$(stat -f%z "${FILE_PATH}" 2>/dev/null || stat -c%s "${FILE_PATH}")
      
      if [ $SIZE -lt ${MIN_SIZE} ]; then
        echo "ERROR: File too small: $SIZE < ${MIN_SIZE}"
        exit 1
      fi
      
      if [ $SIZE -gt ${MAX_SIZE} ]; then
        echo "ERROR: File too large: $SIZE > ${MAX_SIZE}"
        exit 1
      fi
      
      echo "OK: File size is $SIZE bytes"
```

Using the component:

```yaml
steps:
  - name: validate-input
    executor: dag
    command: lib/validators/file-validator.yaml
    params: "FILE_PATH=/data/input.csv MIN_SIZE=100"

  - name: process-file
    command: python process.py /data/input.csv
    depends: validate-input
```

### Nested DAG Hierarchies

```yaml
# master.yaml
name: master-orchestrator
steps:
  - name: run-pipeline
    executor: dag
    command: pipeline.yaml

# pipeline.yaml
name: pipeline
steps:
  - name: stage-1
    executor: dag
    command: stages/extract.yaml
    
  - name: stage-2
    executor: dag
    command: stages/transform.yaml
    depends: stage-1

# stages/extract.yaml
name: extract
steps:
  - name: extract-source-1
    executor: dag
    command: extractors/database.yaml
    
  - name: extract-source-2
    executor: dag
    command: extractors/api.yaml
```

## Best Practices

### 1. Keep DAGs Focused

```yaml
# Good - Single responsibility
# auth/validate.yaml
name: validate-credentials
params:
  - USERNAME: ""
  - TOKEN: ""
steps:
  - name: validate
    command: ./auth_check.sh "${USERNAME}" "${TOKEN}"

# Bad - Mixed responsibilities
name: do-everything
steps:
  - name: validate
    command: ./auth_check.sh
  - name: process-data
    command: ./process.sh
  - name: send-email
    command: ./notify.sh
```

### 2. Use Clear Naming Conventions

```yaml
# Good naming
steps:
  - name: extract-customer-data
    executor: dag
    command: etl/customers/extract.yaml
    
  - name: validate-customer-data
    executor: dag
    command: validators/customers/schema.yaml

# Poor naming
steps:
  - name: step1
    executor: dag
    command: proc1.yaml
    
  - name: step2
    executor: dag
    command: validate.yaml
```

### 3. Handle Outputs Consistently

```yaml
# Child DAG should always produce outputs
handlerOn:
  exit:
    command: |
      # Always write outputs, even on failure
      cat > $DAG_RUN_OUTPUT_FILE <<EOF
      status=${DAG_STATUS:-unknown}
      recordCount=${RECORD_COUNT:-0}
      errors=${ERROR_COUNT:-0}
      EOF
```

### 4. Version Your DAG Libraries

```yaml
steps:
  - name: use-versioned-dag
    executor: dag
    command: lib/v2/processors/etl.yaml  # Version in path
    params: "VERSION=2.0"
```

## Performance Considerations

### Minimize DAG Overhead

```yaml
# For lightweight operations, use shell instead of DAG executor
steps:
  # Good - simple command
  - name: quick-check
    command: test -f /data/input.csv
    
  # Avoid - unnecessary DAG overhead
  - name: overengineered-check
    executor: dag
    command: validators/file-exists.yaml
    params: "FILE=/data/input.csv"
```

### Batch Operations

```yaml
# Good - batch processing in child DAG
steps:
  - name: batch-process
    executor: dag
    command: processors/batch.yaml
    params: "FILES='/data/*.csv' BATCH_SIZE=100"

# Avoid - individual DAG calls per item
steps:
  - name: process-file-1
    executor: dag
    command: process.yaml
    params: "FILE=/data/file1.csv"
    
  - name: process-file-2
    executor: dag
    command: process.yaml
    params: "FILE=/data/file2.csv"
```

## Troubleshooting

### Debug DAG Execution

```yaml
steps:
  - name: debug-child-dag
    executor: dag
    command: complex-workflow.yaml
    params: "DEBUG=true VERBOSE=true"
    env:
      - DAGU_LOG_LEVEL: debug
```

### Check DAG Outputs

```yaml
steps:
  - name: run-dag
    executor: dag
    command: processor.yaml
    output: RESULT

  - name: debug-output
    command: |
      echo "Full output structure:"
      echo '${RESULT}' | jq '.'
      
      echo "Status: ${RESULT.status}"
      echo "Outputs: ${RESULT.outputs}"
    depends: run-dag
```

### Handle Missing DAGs

```yaml
steps:
  - name: check-dag-exists
    command: |
      if [ ! -f "workflows/optional.yaml" ]; then
        echo "Optional workflow not found, skipping"
        exit 0
      fi
      echo "found"
    output: DAG_EXISTS

  - name: run-if-exists
    executor: dag
    command: workflows/optional.yaml
    preconditions:
      - condition: "${DAG_EXISTS}"
        expected: "found"
    depends: check-dag-exists
```

## DAG Executor vs Run Field

The DAG executor provides explicit control and advanced features:

```yaml
# Using executor - full control
steps:
  - name: with-executor
    executor: dag
    command: workflow.yaml
    params: "KEY=value"
    retryPolicy:
      limit: 3
    continueOn:
      failure: true

# Using run field - simplified syntax
steps:
  - name: with-run
    run: workflow.yaml
    params: "KEY=value"
```

Use the DAG executor when you need:
- Retry policies
- Error handling
- Conditional execution
- Output capture
- Dynamic DAG selection

## Next Steps

- Explore [Writing Workflows](/writing-workflows/) for composition patterns
- Learn about [Hierarchical Workflows](/features/hierarchical-workflows) for advanced nesting
- Check out [Examples](/examples/) for real-world use cases