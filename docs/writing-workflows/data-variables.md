# Data and Variables

Dagu provides multiple ways to handle data and variables in your DAGs, from simple environment variables to complex parameter passing between steps.

## Environment Variables

Define variables accessible throughout the DAG:

```yaml
env:
  - SOME_DIR: ${HOME}/batch
  - SOME_FILE: ${SOME_DIR}/some_file 
steps:
  - name: task
    dir: ${SOME_DIR}
    command: python main.py ${SOME_FILE}
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

## Parameters

### Positional Parameters

Define default positional parameters that can be overridden:

```yaml
params: param1 param2  # Default values for $1 and $2
steps:
  - name: parameterized task
    command: python main.py $1 $2      # Will use command-line args or defaults
```

### Named Parameters

Define default named parameters that can be overridden:

```yaml
params:
  - FOO: 1           # Default value for ${FOO}
  - BAR: "`echo 2`"  # Default value for ${BAR}, using command substitution
steps:
  - name: named params task
    command: python main.py ${FOO} ${BAR}  # Will use command-line args or defaults
```

## Output Handling

### Capture Output

Store command output in variables:

```yaml
steps:
  - name: capture
    command: "echo foo"
    output: FOO  # Will contain "foo"
```

**Output Size Limits**: To prevent memory issues from large command outputs, Dagu enforces a size limit on captured output. By default, this limit is 1MB. If a step's output exceeds this limit, the step will fail with an error.

You can configure the maximum output size at the DAG level:

```yaml
# Set maximum output size to 5MB for all steps in this DAG
maxOutputSize: 5242880  # 5MB in bytes

steps:
  - name: large-output
    command: "cat large-file.txt"
    output: CONTENT  # Will fail if file exceeds 5MB
```

### Redirect Output

Send output to files:

```yaml
steps:
  - name: redirect stdout
    command: "echo hello"
    stdout: "/tmp/hello"
  
  - name: redirect stderr
    command: "echo error message >&2"
    stderr: "/tmp/error.txt"
```

### JSON References

You can use JSON references in fields to dynamically expand values from variables. JSON references are denoted using the `${NAME.path.to.value}` syntax, where `NAME` refers to a variable name and `path.to.value` specifies the path in the JSON to resolve. If the data is not JSON format, the value will not be expanded.

Examples:

```yaml
steps:
  - name: child DAG
    run: sub_workflow
    output: SUB_RESULT
  - name: use output
    command: echo "The result is ${SUB_RESULT.outputs.finalValue}"
```

If `SUB_RESULT` contains:

```json
{
  "outputs": {
    "finalValue": "success"
  }
}
```

Then the expanded value of `${SUB_RESULT.outputs.finalValue}` will be `success`.

## Step ID References

You can assign short identifiers to steps and use them to reference step properties in subsequent steps. This is particularly useful when you have long step names or want cleaner variable references:

```yaml
steps:
  - name: extract customer data
    id: extract  # Short identifier
    command: python extract.py
    output: DATA
  
  - name: validate extracted data
    id: validate
    command: python validate.py
    depends:
      - extract  # Can use ID in dependencies
  
  - name: process if valid
    command: |
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
  - name: use date
    command: "echo hello, today is ${TODAY}"
```

## Sub-workflow Data

The result of the sub workflow will be available from the standard output of the sub workflow in JSON format.

```yaml
steps:
  - name: sub workflow
    run: sub_workflow
    params: "FOO=BAR"
    output: SUB_RESULT

  - name: use sub workflow output
    command: echo $SUB_RESULT
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
  - name: get config
    command: |
      echo '{"env": "prod", "replicas": 3, "region": "us-east-1"}'
    output: CONFIG
  
  - name: get secrets
    command: vault read -format=json secret/app
    output: SECRETS
  
  - name: deploy
    command: |
      kubectl set env deployment/app \
        REGION=${CONFIG.region} \
        API_KEY=${SECRETS.data.api_key}
      kubectl scale --replicas=${CONFIG.replicas} deployment/app
    depends: [get config, get secrets]
```

### Through Files

```yaml
steps:
  - name: generate data
    command: python generate.py
    stdout: /tmp/data.json
  
  - name: process data
    command: python process.py < /tmp/data.json
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
