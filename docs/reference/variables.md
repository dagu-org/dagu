# Variables Reference

For a complete list of the automatically injected run metadata, see [Special Environment Variables](/reference/special-environment-variables).

## Environment Variables

### System Environment Variable Filtering

For security, Dagu filters which system environment variables are passed to step processes and sub DAGs.

**How It Works:**

System environment variables are available for expansion (`${VAR}`) when the DAG configuration is parsed, but only filtered variables are passed to the actual step execution environment.

**Filtered Variables:**

Only these system environment variables are automatically passed to step processes and sub DAGs:

- **Whitelisted:** `PATH`, `HOME`, `LANG`, `TZ`, `SHELL`
- **Allowed Prefixes:** `DAGU_*`, `LC_*`, `DAG_*`

The `DAG_*` prefix includes the special environment variables that Dagu automatically sets (see below).

**What This Means:**

You can use `${AWS_SECRET_ACCESS_KEY}` in your DAG YAML for variable expansion, but the `AWS_SECRET_ACCESS_KEY` variable itself won't be available in the environment when your step commands run unless you explicitly define it in the `env` section.

### Defining Environment Variables

Set environment variables available to all steps:

```yaml
env:
  - LOG_LEVEL: debug
  - DATA_DIR: /tmp/data
  - API_URL: https://api.example.com
  - API_KEY: ${SECRET_API_KEY}  # Explicitly reference system environment
```

**Important:** To use sensitive system environment variables in your workflows, you must explicitly reference them in your `env` section as shown above. They will not be automatically available.

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
  - echo "Args: $1 $2 $3"
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
  - ./server --env=${ENVIRONMENT} --port=${PORT} --debug=${DEBUG}
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
  - echo "Deploying $1 to ${ENVIRONMENT} version ${VERSION}"
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
  - echo "Deploy on ${TODAY} from ${HOSTNAME}"
```

## Output Variables

### Capturing Output

Capture command output to use in later steps:

```yaml
steps:
  - command: cat VERSION
    output: VERSION
  - docker build -t myapp:${VERSION} .
```

### Output Size Limits

Control maximum output size:

```yaml
# Global limit for all steps
maxOutputSize: 5242880  # 5MB

steps:
  - command: cat large-file.json
    output: FILE_CONTENT  # Fails if > 5MB
```

### Redirecting Output

Redirect to files instead of capturing:

```yaml
steps:
  - command: python report.py
    stdout: /tmp/report.txt
    stderr: /tmp/errors.log
```

## JSON Path References

Access nested values in JSON output:

```yaml
steps:
  - command: |
      echo '{"db": {"host": "localhost", "port": 5432}}'
    output: CONFIG
    
  - psql -h ${CONFIG.db.host} -p ${CONFIG.db.port}
```

### Sub-workflow Output

Access outputs from nested workflows:

```yaml
steps:
  - call: etl-workflow
    params: "DATE=${TODAY}"
    output: ETL_RESULT
    
  - |
      echo "Records processed: ${ETL_RESULT.outputs.record_count}"
      echo "Status: ${ETL_RESULT.outputs.status}"
```

## Step ID References

Reference step properties using IDs:

```yaml
steps:
  - id: risky
    command: 'sh -c "if [ $((RANDOM % 2)) -eq 0 ]; then echo Success; else echo Failed && exit 1; fi"'
    continueOn:
      failure: true
      
  - |
      if [ "${risky.exitCode}" = "0" ]; then
        echo "Success! Output was:"
        cat ${risky.stdout}
      else
        echo "Failed with code ${risky.exitCode}"
        cat ${risky.stderr}
      fi
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
     - env:
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

## See Also

- [Writing Workflows](/writing-workflows/data-variables) - Detailed guide on using variables
- [YAML Specification](/reference/yaml) - Complete YAML format reference
- [Configuration Reference](/configurations/reference) - Server configuration variables
