# Variables Reference

## Special Environment Variables

Dagu automatically sets these environment variables for every step execution:

| Variable | Description | Example |
|----------|-------------|---------|
| `DAG_NAME` | Name of the current DAG | `my-workflow` |
| `DAG_RUN_ID` | Unique ID for this execution | `20240115_140000_abc123` |
| `DAG_RUN_STEP_NAME` | Name of the current step | `process-data` |
| `DAG_RUN_LOG_FILE` | Path to the main log file | `/logs/my-workflow/20240115_140000.log` |
| `DAG_RUN_STEP_STDOUT_FILE` | Path to step's stdout log | `/logs/my-workflow/process-data.stdout.log` |
| `DAG_RUN_STEP_STDERR_FILE` | Path to step's stderr log | `/logs/my-workflow/process-data.stderr.log` |

Example usage:
```yaml
steps:
  - name: log-context
    command: |
      echo "Running DAG: ${DAG_NAME}"
      echo "Execution ID: ${DAG_RUN_ID}"
      echo "Current step: ${DAG_RUN_STEP_NAME}"
      echo "Logs at: ${DAG_RUN_LOG_FILE}"
```

## Environment Variables

### Defining Environment Variables

Set environment variables available to all steps:

```yaml
env:
  - LOG_LEVEL: debug
  - DATA_DIR: /tmp/data
  - API_URL: https://api.example.com
  - API_KEY: ${SECRET_API_KEY}  # From system environment
```

### Variable Expansion

Reference other variables:

```yaml
env:
  - BASE_DIR: ${HOME}/data
  - INPUT_DIR: ${BASE_DIR}/input
  - OUTPUT_DIR: ${BASE_DIR}/output
  - CONFIG_FILE: ${INPUT_DIR}/config.yaml
```

### Loading from .env Files

Load variables from dotenv files:

```yaml
# Single file
dotenv: .env

# Multiple files (loaded in order)
dotenv:
  - .env
  - .env.local
  - configs/.env.${ENVIRONMENT}
```

Example `.env` file:
```bash
DATABASE_URL=postgres://localhost/mydb
API_KEY=secret123
DEBUG=true
```

## Parameters

### Positional Parameters

Define default positional parameters:

```yaml
params: first second third

steps:
  - name: use-params
    command: echo "Args: $1 $2 $3"
```

Run with custom values:
```bash
dagu start workflow.yaml -- one two three
```

### Named Parameters

Define named parameters with defaults:

```yaml
params:
  - ENVIRONMENT: dev
  - PORT: 8080
  - DEBUG: false

steps:
  - name: start-server
    command: ./server --env=${ENVIRONMENT} --port=${PORT} --debug=${DEBUG}
```

Override at runtime:
```bash
dagu start workflow.yaml -- ENVIRONMENT=prod PORT=80 DEBUG=true
```

### Mixed Parameters

Combine positional and named parameters:

```yaml
params:
  - ENVIRONMENT: dev
  - VERSION: latest

steps:
  - name: deploy
    command: echo "Deploying $1 to ${ENVIRONMENT} version ${VERSION}"
```

Run with:
```bash
dagu start workflow.yaml -- myapp ENVIRONMENT=prod VERSION=1.2.3
```

## Command Substitution

Execute commands and use their output:

```yaml
env:
  - TODAY: "`date +%Y-%m-%d`"
  - HOSTNAME: "`hostname -f`"
  - GIT_COMMIT: "`git rev-parse HEAD`"

params:
  - TIMESTAMP: "`date +%s`"
  - USER_COUNT: "`wc -l < users.txt`"

steps:
  - name: use-substitution
    command: echo "Deploy on ${TODAY} from ${HOSTNAME}"
```

## Output Variables

### Capturing Output

Capture command output to use in later steps:

```yaml
steps:
  - name: get-version
    command: cat VERSION
    output: VERSION
    
  - name: build
    command: docker build -t myapp:${VERSION} .
    depends: get-version
```

### Output Size Limits

Control maximum output size:

```yaml
# Global limit for all steps
maxOutputSize: 5242880  # 5MB

steps:
  - name: large-output
    command: cat large-file.json
    output: FILE_CONTENT  # Fails if > 5MB
```

### Redirecting Output

Redirect to files instead of capturing:

```yaml
steps:
  - name: generate-report
    command: python report.py
    stdout: /tmp/report.txt
    stderr: /tmp/errors.log
```

## JSON Path References

Access nested values in JSON output:

```yaml
steps:
  - name: get-config
    command: |
      echo '{"db": {"host": "localhost", "port": 5432}}'
    output: CONFIG
    
  - name: connect
    command: psql -h ${CONFIG.db.host} -p ${CONFIG.db.port}
    depends: get-config
```

### Sub-workflow Output

Access outputs from nested workflows:

```yaml
steps:
  - name: run-etl
    run: etl-workflow
    params: "DATE=${TODAY}"
    output: ETL_RESULT
    
  - name: process-results
    command: |
      echo "Records processed: ${ETL_RESULT.outputs.record_count}"
      echo "Status: ${ETL_RESULT.outputs.status}"
    depends: run-etl
```

## Step ID References

Reference step properties using IDs:

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
        echo "Success! Output was:"
        cat ${risky.stdout}
      else
        echo "Failed with code ${risky.exitCode}"
        cat ${risky.stderr}
      fi
    depends: risky-operation
```

Available properties:
- `${id.exitCode}` - Exit code of the step
- `${id.stdout}` - Path to stdout log file
- `${id.stderr}` - Path to stderr log file

## Variable Precedence

Variables are resolved in this order (highest to lowest):

1. **Command-line parameters**
   ```bash
   dagu start workflow.yaml -- VAR=override
   ```

2. **Step-level environment**
   ```yaml
   steps:
     - name: step
       env:
         - VAR: step-value
   ```

3. **DAG-level parameters**
   ```yaml
   params:
     - VAR: dag-default
   ```

4. **DAG-level environment**
   ```yaml
   env:
     - VAR: env-value
   ```

5. **Dotenv files**
   ```yaml
   dotenv: .env
   ```

6. **Base configuration**
   ```yaml
   # ~/.config/dagu/base.yaml
   env:
     - VAR: base-value
   ```

7. **System environment**
   ```bash
   export VAR=system-value
   ```

## See Also

- [Writing Workflows](/writing-workflows/data-variables) - Detailed guide on using variables
- [YAML Specification](/reference/yaml) - Complete YAML format reference
- [Configuration Reference](/configurations/reference) - Server configuration variables
