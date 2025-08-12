# Data Flow

How data moves through your workflows - from parameters to outputs, between steps, and across workflows.

## Overview

Dagu provides multiple mechanisms for passing data through workflows:

- **Output Variables** - Capture command output for use in later steps
- **Environment Variables** - Define variables accessible to all steps
- **Parameters** - Pass runtime values into workflows
- **File-based Passing** - Redirect output to files
- **JSON Path References** - Access nested data structures
- **Step ID References** - Reference step properties and files
- **Child DAG Outputs** - Capture results from sub-workflows

## Output Variables

Capture command output and use it in subsequent steps:

```yaml
steps:
  - name: get-version
    command: cat VERSION
    output: VERSION
    
  - name: build
    command: docker build -t myapp:${VERSION} .
```

### How It Works

1. Command stdout is captured (up to `maxOutputSize` limit)
2. Stored in the variable name specified by `output`
3. Available to all downstream steps via `${VARIABLE_NAME}`
4. Trailing newlines are automatically trimmed

### Multiple Outputs

Each step can have one output variable:

```yaml
steps:
  - name: count-users
    command: wc -l < users.txt
    output: USER_COUNT
    
  - name: count-orders
    command: wc -l < orders.txt  
    output: ORDER_COUNT
    
  - name: report
    command: |
      echo "Users: ${USER_COUNT}"
      echo "Orders: ${ORDER_COUNT}"
    depends:
      - count-users
      - count-orders
```

## JSON Path References

Access nested values in JSON output using dot notation:

```yaml
steps:
  - name: get-config
    command: |
      echo '{
        "database": {
          "host": "localhost",
          "port": 5432,
          "credentials": {
            "username": "app_user"
          }
        }
      }'
    output: CONFIG
    
  - name: connect
    command: |
      psql -h ${CONFIG.database.host} \
           -p ${CONFIG.database.port} \
           -U ${CONFIG.database.credentials.username}
```

### Array Access

Access array elements by index:

```yaml
steps:
  - name: get-servers
    command: |
      echo '[
        {"name": "web1", "ip": "10.0.1.1"},
        {"name": "web2", "ip": "10.0.1.2"}
      ]'
    output: SERVERS
    
  - name: ping-first
    command: ping -c 1 ${SERVERS[0].ip}
```

## Environment Variables

### DAG-Level Variables

Define variables available to all steps:

```yaml
env:
  - LOG_LEVEL: debug
  - DATA_DIR: /var/data
  - API_URL: https://api.example.com

steps:
  - name: process
    command: python process.py --log=${LOG_LEVEL} --data=${DATA_DIR}
```

### Variable Expansion

Reference other variables:

```yaml
env:
  - BASE_DIR: ${HOME}/project
  - DATA_DIR: ${BASE_DIR}/data
  - OUTPUT_DIR: ${BASE_DIR}/output
  - CONFIG_FILE: ${DATA_DIR}/config.yaml
```

### Command Substitution

Execute commands and use their output:

```yaml
env:
  - TODAY: "`date +%Y-%m-%d`"
  - GIT_COMMIT: "`git rev-parse HEAD`"
  - HOSTNAME: "`hostname -f`"

steps:
  - name: backup
    command: tar -czf backup-${TODAY}-${GIT_COMMIT}.tar.gz data/
```

## Parameters

### Named Parameters

Define parameters with defaults:

```yaml
params:
  - ENVIRONMENT: dev
  - BATCH_SIZE: 100
  - DRY_RUN: false

steps:
  - name: process
    command: |
      echo "Processing data" \
        --env=${ENVIRONMENT} \
        --batch=${BATCH_SIZE} \
        --dry-run=${DRY_RUN}
```

Override at runtime:
```bash
dagu start workflow.yaml -- ENVIRONMENT=prod BATCH_SIZE=500
```

### Dynamic Parameters

Use command substitution in defaults:

```yaml
params:
  - DATE: "`date +%Y-%m-%d`"
  - RUN_ID: "`uuidgen`"
  - USER: "`whoami`"
```

## Step ID References

Reference step properties using the `id` field:

```yaml
steps:
  - name: risky-operation
    id: risky
    command: 'sh -c "if [ $((RANDOM % 2)) -eq 0 ]; then echo Success; else echo Failed && exit 1; fi"'
    continueOn:
      failure: true
      
  - name: check-result
    command: |
      if [ "${risky.exitCode}" = "0" ]; then
        echo "Success! Checking output..."
        cat ${risky.stdout}
      else
        echo "Failed with code ${risky.exitCode}"
        echo "Error log:"
        cat ${risky.stderr}
      fi
```

Available properties:
- `${id.exitCode}` - Exit code of the step
- `${id.stdout}` - Path to stdout log file
- `${id.stderr}` - Path to stderr log file

## Child DAG Outputs

Capture outputs from nested workflows:

### Basic Child Output

```yaml
# parent.yaml
steps:
  - name: run-etl
    run: etl-workflow
    params: "DATE=${TODAY}"
    output: ETL_RESULT
    
  - name: process-results
    command: |
      echo "Status: ${ETL_RESULT.status}"
      echo "Records: ${ETL_RESULT.outputs.record_count}"
      echo "Duration: ${ETL_RESULT.outputs.duration}"
```

### Output Structure

Child DAG output contains:
```json
{
  "name": "etl-workflow",
  "params": "DATE=2024-01-15",
  "status": "succeeded",
  "outputs": {
    "record_count": "1000",
    "duration": "120s"
  }
}
```

### Nested DAG Outputs

Access outputs from deeply nested workflows:

```yaml
steps:
  - name: orchestrate
    run: main-pipeline
    output: PIPELINE
    
  - name: report
    command: |
      # Access nested outputs
      echo "ETL Status: ${PIPELINE.outputs.ETL_OUTPUT.status}"
      echo "ML Score: ${PIPELINE.outputs.ML_OUTPUT.outputs.accuracy}"
```

## Parallel Execution Outputs

When running parallel executions, outputs are aggregated:

```yaml
steps:
  - name: process-regions
    run: region-processor
    parallel:
      items: ["us-east", "us-west", "eu-central"]
    output: RESULTS
    
  - name: summarize
    command: |
      echo "Total regions: ${RESULTS.summary.total}"
      echo "Succeeded: ${RESULTS.summary.succeeded}"
      echo "Failed: ${RESULTS.summary.failed}"
      
      # Access individual results
      echo "US-East revenue: ${RESULTS.outputs[0].revenue}"
      echo "US-West revenue: ${RESULTS.outputs[1].revenue}"
```

### Parallel Output Structure

```json
{
  "summary": {
    "total": 3,
    "succeeded": 3,
    "failed": 0
  },
  "results": [
    {
      "params": "us-east",
      "status": "succeeded",
      "outputs": {
        "revenue": "1000000"
      }
    }
    // ... more results
  ],
  "outputs": [
    {"revenue": "1000000"},
    {"revenue": "750000"},
    {"revenue": "500000"}
  ]
}
```

## File-Based Data Passing

### Output Redirection

Redirect output to files for large data:

```yaml
steps:
  - name: generate-report
    command: python generate_report.py
    stdout: /tmp/report.txt
    
  - name: email-report
    command: mail -s "Report" user@example.com < /tmp/report.txt
```

### Working with Files

```yaml
steps:
  - name: extract
    command: |
      tar -xzf data.tar.gz -C /tmp/
      ls /tmp/data/ > /tmp/filelist.txt
    
  - name: process-files
    command: |
      while read file; do
        process.sh "/tmp/data/$file"
      done < /tmp/filelist.txt
```

## Special Environment Variables

Dagu automatically sets these variables:

| Variable | Description |
|----------|-------------|
| `DAG_NAME` | Name of the current DAG |
| `DAG_RUN_ID` | Unique execution ID |
| `DAG_RUN_STEP_NAME` | Current step name |
| `DAG_RUN_LOG_FILE` | Main log file path |
| `DAG_RUN_STEP_STDOUT_FILE` | Step's stdout log path |
| `DAG_RUN_STEP_STDERR_FILE` | Step's stderr log path |

Example usage:
```yaml
steps:
  - name: backup-logs
    command: |
      echo "Backing up logs for ${DAG_NAME} run ${DAG_RUN_ID}"
      cp ${DAG_RUN_LOG_FILE} /backup/
```

## Output Size Limits

Control maximum output size to prevent memory issues:

```yaml
# Set 5MB limit for all steps
maxOutputSize: 5242880

steps:
  - name: large-output
    command: cat large-file.json
    output: DATA  # Fails if output > 5MB
    
  - name: unlimited-file
    command: generate-huge-file.sh
    stdout: /tmp/huge.txt  # No size limit with file redirection
```

## Variable Resolution Order

Variables are resolved in this precedence (highest to lowest):

1. **Step-level environment**
2. **Output variables** from dependencies
3. **DAG-level parameters**
4. **DAG-level environment**
5. **dotenv files**
6. **Base configuration**
7. **System environment**

Example:
```yaml
env:
  - MESSAGE: "DAG level"

params:
  - MESSAGE: "Param default"

steps:
  - name: show-precedence
    env:
      - MESSAGE: "Step level"  # This wins
    command: echo "${MESSAGE}"
```

## Advanced Patterns

### Dynamic Data Processing

Process files discovered at runtime:

```yaml
steps:
  - name: find-csvs
    command: find /data -name "*.csv" -type f
    output: CSV_FILES
    
  - name: process-each
    run: csv-processor
    parallel: ${CSV_FILES}
    params: "FILE=${ITEM}"
```

### Conditional Data Flow

Route data based on conditions:

```yaml
steps:
  - name: check-env
    command: |
      if [ "${ENVIRONMENT}" = "prod" ]; then
        echo "production-config.json"
      else
        echo "staging-config.json"
      fi
    output: CONFIG_FILE
    
  - name: load-config
    command: process --config=${CONFIG_FILE}
```

### Data Validation Pipeline

```yaml
steps:
  - name: fetch-data
    command: curl -s https://api.example.com/data
    output: RAW_DATA
    
  - name: validate-json
    command: echo '${RAW_DATA}' | jq empty
    continueOn:
      failure: false  # Stop if invalid JSON
    depends: fetch-data
    
  - name: extract-fields
    command: |
      echo '${RAW_DATA}' | jq '{
        id: .id,
        total: .items | length,
        sum: .items | map(.value) | add
      }'
    output: PROCESSED
    depends: validate-json
    
  - name: save-results
    command: echo '${PROCESSED}' > results.json
    depends: extract-fields
```

## See Also

- [Writing Workflows](/writing-workflows/data-variables) - Detailed variable guide
- [Variables Reference](/reference/variables) - Complete variable reference
- [Examples](/writing-workflows/examples) - Data flow examples
