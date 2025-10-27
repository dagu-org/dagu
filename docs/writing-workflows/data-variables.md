# Data and Variables

Dagu provides multiple ways to handle data and variables in your DAGs, from simple environment variables to complex parameter passing between steps.

## Environment Variables

### System Environment Variable Security

For security, Dagu limits which system environment variables are passed to step processes and sub DAGs.

**How It Works:**

System environment variables are available for expansion (`${VAR}`) during DAG configuration parsing, but only filtered variables are passed to the step execution environment.

**Filtered Variables:**

Only these are automatically passed to step processes:
- **Whitelisted:** `PATH`, `HOME`, `LANG`, `TZ`, `SHELL`
- **Allowed Prefixes:** `DAGU_*`, `LC_*`, `DAG_*`

The `DAG_*` prefix includes special variables automatically set by Dagu for each step execution.

**To Use Sensitive Variables:**

You can reference system variables like `${AWS_SECRET_ACCESS_KEY}` in your YAML for substitution, but to make them available in the step process environment, define them in the `env` section:

```yaml
env:
  - AWS_ACCESS_KEY_ID: ${AWS_ACCESS_KEY_ID}      # Available in step environment
  - AWS_SECRET_ACCESS_KEY: ${AWS_SECRET_ACCESS_KEY}
  - DATABASE_URL: ${DATABASE_URL}
```

Or use `.env` files (recommended):

```yaml
dotenv: .env.secrets
```

This prevents accidental exposure of sensitive variables to step processes.

### DAG-Level Environment Variables

Define variables accessible throughout the DAG:

```yaml
env:
  - SOME_DIR: ${HOME}/batch
  - SOME_FILE: ${SOME_DIR}/some_file
steps:
  - workingDir: ${SOME_DIR}
    command: python main.py ${SOME_FILE}
```

### Step-Level Environment Variables

You can also define environment variables specific to individual steps. Step-level variables override DAG-level variables with the same name:

```yaml
env:
  - SHARED_VAR: dag_value
  - DAG_ONLY: dag_only_value

steps:
  - command: echo $SHARED_VAR
    env:
      - SHARED_VAR: step_value  # Overrides the DAG-level value
      - STEP_ONLY: step_only_value
    # Output: step_value
  
  - echo $SHARED_VAR $DAG_ONLY
    # Output: dag_value dag_only_value
```

Step environment variables support the same features as DAG-level variables, including command substitution and references to other variables:

```yaml
env:
  - BASE_PATH: /data

steps:
  - name: process data
    command: python process.py
    env:
      - INPUT_PATH: ${BASE_PATH}/input
      - TIMESTAMP: "`date +%Y%m%d_%H%M%S`"
      - WORKER_ID: worker_${HOSTNAME}
```

## Dotenv Files

Specify `.env` files to load environment variables from. By default, no env files are loaded unless explicitly specified.

```yaml
dotenv: .env  # Specify a candidate dotenv file

# Or specify multiple candidate files
dotenv:
  - .env
  - .env.local
  - configs/.env.prod
```

Files can be specified as:
- Absolute paths
- Relative to the DAG file directory
- Relative to the base config directory
- Relative to the user's home directory

## Secrets

Use the `secrets` block to declare sensitive values without embedding them in YAML. Each secret defines an environment variable that is resolved at runtime from a provider and injected before the DAG runs:

```yaml
secrets:
  - name: API_TOKEN
    provider: env
    key: PROD_API_TOKEN    # Read from process environment
  - name: DB_PASSWORD
    provider: file
    key: secrets/db-pass   # Relative to workingDir, then the DAG file directory

steps:
  - name: migrate
    command: ./migrate.sh
    env:
      - STRICT_MODE: "1"   # Step-level env still overrides secrets if needed
```

### Built-in providers

- `env` reads from existing environment variables. Use it when CI/CD or your process manager injects secrets into the runtime environment.
- `file` reads from files. Relative paths first try the DAG’s `workingDir`, then fall back to the directory containing the DAG file, which makes this provider ideal for Secret Store CSI or Docker secrets mounted beside the DAG.

Providers can expose additional configuration through the optional `options` map. Values must be strings so they can be forwarded to provider-specific clients.

### Resolution and masking

Secrets are evaluated after DAG-level variables and system-provided runtime variables, so they override values defined in `env` or `.env` files unless a step sets its own value. Resolved secrets are never written to disk or the database, and Dagu automatically masks them in step output and scheduler logs.

Read the dedicated [Secrets guide](/writing-workflows/secrets) for provider details, masking behavior, and best practices.

## Parameters

### Positional Parameters

Define default positional parameters that can be overridden:

```yaml
params: param1 param2     # Default values for $1 and $2
steps:
  - python main.py $1 $2  # Will use command-line args or defaults
```

### Named Parameters

Define default named parameters that can be overridden:

```yaml
params:
  - FOO: 1           # Default value for ${FOO}
  - BAR: "`echo 2`"  # Default value for ${BAR}, using command substitution
steps:
  - python main.py ${FOO} ${BAR}  # Will use command-line args or defaults
```

## Output Handling

### Capture Output

Store command output in variables:

```yaml
steps:
  - command: "echo foo"
    output: FOO  # Will contain "foo"
```

**Output Size Limits**: To prevent memory issues from large command outputs, Dagu enforces a size limit on captured output. By default, this limit is 1MB. If a step's output exceeds this limit, the step will fail with an error.

You can configure the maximum output size at the DAG level:

```yaml
# Set maximum output size to 5MB for all steps in this DAG
maxOutputSize: 5242880  # 5MB in bytes

steps:
  - command: "cat large-file.txt"
    output: CONTENT  # Will fail if file exceeds 5MB
```

### Redirect Output

Send output to files:

```yaml
steps:
  - command: "echo hello"
    stdout: "/tmp/hello"
  - command: "echo error message >&2"
    stderr: "/tmp/error.txt"
```

### JSON References

You can use JSON references in fields to dynamically expand values from variables. JSON references are denoted using the `${NAME.path.to.value}` syntax, where `NAME` refers to a variable name and `path.to.value` specifies the path in the JSON to resolve. If the data is not JSON format, the value will not be expanded.

Examples:

```yaml
steps:
  - call: sub_workflow
    output: SUB_RESULT
  - echo "The result is ${SUB_RESULT.outputs.finalValue}"
```

If `SUB_RESULT` contains:

```json
{
  "outputs": {
    "finalValue": "succeeded"
  }
}
```

Then the expanded value of `${SUB_RESULT.outputs.finalValue}` will be `succeeded`.

## Step ID References

You can assign short identifiers to steps and use them to reference step properties in subsequent steps. This is particularly useful when you have long step names or want cleaner variable references:

```yaml
steps:
  - id: extract  # Short identifier
    command: python extract.py
    output: DATA
  
  - id: validate
    command: python validate.py
    depends:
      - extract  # Can use ID in dependencies
  
  - |
      # Reference step properties using IDs
      echo "Exit code: ${extract.exitCode}"
      echo "Stdout path: ${extract.stdout}"
      echo "Stderr path: ${extract.stderr}"
```

Available step properties when using ID references:
- `${id.stdout}`: Path to stdout file
- `${id.stderr}`: Path to stderr file  
- `${id.exitCode}`: Exit code of the step

## Command Substitution

Use command output in configurations:

```yaml
env:
  TODAY: "`date '+%Y%m%d'`"
steps:
  - "echo hello, today is ${TODAY}"
```

## Sub-workflow Data

The result of the sub workflow will be available from the standard output of the sub workflow in JSON format.

```yaml
steps:
  - call: sub_workflow
    params: "FOO=BAR"
    output: SUB_RESULT
  - echo $SUB_RESULT
```

Example output format:

```json
{
  "name": "sub_workflow",
  "params": "FOO=BAR",
  "outputs": {
    "RESULT": "ok"
  }
}
```

## Passing Data Between Steps

### Through Output Variables

```yaml
steps:
  - command: |
      echo '{"env": "prod", "replicas": 3, "region": "us-east-1"}'
    output: CONFIG
  
  - command: vault read -format=json secret/app
    output: SECRETS
  
  - command: |
      kubectl set env deployment/app \
        REGION=${CONFIG.region} \
        API_KEY=${SECRETS.data.api_key}
      kubectl scale --replicas=${CONFIG.replicas} deployment/app
    depends: [get config, get secrets]
```

### Through Files

```yaml
steps:
  - command: python generate.py
    stdout: /tmp/data.json
  
  - python process.py < /tmp/data.json
```

## Global Configuration

Common settings can be shared using `$HOME/.config/dagu/base.yaml`. This is useful for setting default values for:
- `env` - Shared environment variables
- `params` - Default parameters
- `logDir` - Default log directory
- Other organizational defaults

Example base configuration:

```yaml
# ~/.config/dagu/base.yaml
env:
  - ENVIRONMENT: production
  - API_ENDPOINT: https://api.example.com
params:
  - DEFAULT_BATCH_SIZE: 100
logDir: /var/log/dagu
```

Individual DAGs inherit these settings and can override them as needed.
