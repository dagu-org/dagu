# Parameters

Complete guide to using parameters in Dagu workflows - from basic usage to advanced patterns.

## Overview

Parameters allow you to make workflows dynamic and reusable by passing values at runtime. Dagu supports both positional parameters (like shell scripts) and named parameters (like environment variables).

## Parameter Definition

### Named Parameters (Recommended)

Define parameters with names and default values:

```yaml
params:
  - ENVIRONMENT: dev
  - PORT: 8080
  - DEBUG: false

steps:
  - name: start-server
    command: ./server --env=${ENVIRONMENT} --port=${PORT} --debug=${DEBUG}
```

### List Format

```yaml
params:
  - DATABASE: postgres
  - VERSION: latest
  - WORKERS: 4
```

### Map Format

```yaml
params:
  DATABASE: postgres
  VERSION: latest
  WORKERS: 4
```

### Positional Parameters

For simple scripts that expect positional arguments:

```yaml
params: first second third

steps:
  - name: process
    command: ./script.sh $1 $2 $3  # Uses first, second, third
```

### Mixed Parameters

Combine positional and named parameters:

```yaml
params: "config.json ENVIRONMENT=prod DEBUG=true"

steps:
  - name: run
    command: ./app $1 --env=${ENVIRONMENT} --debug=${DEBUG}
```

## Passing Parameters

### Command Line - Named Parameters

Use the `--` separator (recommended):

```bash
# Single parameter
dagu start workflow.yaml -- ENVIRONMENT=production

# Multiple parameters
dagu start workflow.yaml -- ENVIRONMENT=prod PORT=80 DEBUG=false

# With spaces in values
dagu start workflow.yaml -- MESSAGE="Hello World" PATH="/my path/with spaces"
```

Using `--params` flag:

```bash
dagu start workflow.yaml --params="ENVIRONMENT=prod PORT=80"
```

### Command Line - Positional Parameters

```bash
# Pass positional parameters
dagu start workflow.yaml -- input.csv output.json

# Access in workflow
steps:
  - name: process
    command: python process.py $1 $2  # $1=input.csv, $2=output.json
```

### Mixed Command Line Parameters

```bash
# Positional followed by named
dagu start workflow.yaml -- config.json ENVIRONMENT=prod DEBUG=true

# In workflow:
# $1 = config.json
# $ENVIRONMENT = prod
# $DEBUG = true
```

## Dynamic Parameters

### Command Substitution

Use backticks to execute commands and use their output:

```yaml
params:
  - DATE: "`date +%Y-%m-%d`"
  - HOSTNAME: "`hostname -f`"
  - GIT_COMMIT: "`git rev-parse --short HEAD`"
  - TIMESTAMP: "`date +%s`"

steps:
  - name: backup
    command: tar -czf backup-${DATE}-${GIT_COMMIT}.tar.gz data/
```

### Environment Variable Expansion

Reference existing environment variables:

```yaml
params:
  - USER: ${USER}
  - HOME_DIR: ${HOME}
  - WORKSPACE: ${HOME}/projects
  - LOG_PATH: ${LOG_DIR:-/var/log}  # With default
```

### Complex Values

Parameters can contain complex expressions:

```yaml
params:
  - BUILD_TAG: "`git describe --tags --dirty`-`date +%Y%m%d`"
  - SERVER: "${ENVIRONMENT:-dev}.${REGION:-us-east-1}.example.com"
  - OPTIONS: "--verbose --timeout=300 --retries=3"
```

## Using Parameters

### In Commands

```yaml
params:
  - INPUT_FILE: data.csv
  - OUTPUT_DIR: /tmp/output
  - THREADS: 4

steps:
  - name: process
    command: |
      mkdir -p ${OUTPUT_DIR}
      python processor.py \
        --input ${INPUT_FILE} \
        --output ${OUTPUT_DIR}/result.json \
        --threads ${THREADS}
```

### In Environment Variables

Parameters become environment variables:

```yaml
params:
  - API_KEY: secret123
  - API_URL: https://api.example.com

steps:
  - name: call-api
    command: curl -H "Authorization: ${API_KEY}" ${API_URL}/data
    env:
      - REQUEST_TIMEOUT: 30
      - API_VERSION: ${VERSION:-v1}  # Can reference params
```

### In Conditions

Use parameters in preconditions:

```yaml
params:
  - SKIP_TESTS: false
  - ENVIRONMENT: dev

steps:
  - name: run-tests
    command: npm test
    preconditions:
      - condition: "${SKIP_TESTS}"
        expected: "false"
      
  - name: deploy
    command: ./deploy.sh
    preconditions:
      - condition: "${ENVIRONMENT}"
        expected: "prod"
```

## Parameter Inheritance

### Parent to Child DAGs

Parameters flow from parent to child workflows:

```yaml
# parent.yaml
params:
  - ENVIRONMENT: staging
  - VERSION: 1.0.0

steps:
  - name: build
    run: build-workflow
    params: "VERSION=${VERSION}"  # Pass specific param
    
  - name: deploy
    run: deploy-workflow
    params: "ENVIRONMENT=${ENVIRONMENT} VERSION=${VERSION}"
```

```yaml
# build-workflow.yaml
params:
  - VERSION: latest  # Default overridden by parent

steps:
  - name: build
    command: docker build -t app:${VERSION} .
```

### Override Child Parameters

Parent can override child's default parameters:

```yaml
# parent.yaml
steps:
  - name: process-prod
    run: processor
    params: "ENV=production THREADS=8"
    
  - name: process-dev
    run: processor
    params: "ENV=development THREADS=2"
```

```yaml
# processor.yaml
params:
  - ENV: local
  - THREADS: 4

steps:
  - name: process
    command: ./process.sh --env=${ENV} --threads=${THREADS}
```

## Advanced Patterns

### Conditional Parameters

Set parameters based on conditions:

```yaml
params:
  - ENVIRONMENT: dev

steps:
  - name: set-config
    command: |
      if [ "${ENVIRONMENT}" = "prod" ]; then
        echo "production.conf"
      else
        echo "development.conf"
      fi
    output: CONFIG_FILE
    
  - name: run
    command: ./app --config=${CONFIG_FILE}
    depends: set-config
```

### Parameter Validation

Validate required parameters:

```yaml
params:
  - REQUIRED_PARAM: ""
  - OPTIONAL_PARAM: "default"

steps:
  - name: validate
    command: |
      if [ -z "${REQUIRED_PARAM}" ]; then
        echo "ERROR: REQUIRED_PARAM must be provided"
        exit 1
      fi
      
      # Validate format
      if ! [[ "${PORT:-8080}" =~ ^[0-9]+$ ]]; then
        echo "ERROR: PORT must be a number"
        exit 1
      fi
```

### Parameter Templates

Use parameters to template configurations:

```yaml
params:
  - APP_NAME: myapp
  - NAMESPACE: default
  - REPLICAS: 3

steps:
  - name: generate-config
    command: |
      cat > deployment.yaml <<EOF
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: ${APP_NAME}
        namespace: ${NAMESPACE}
      spec:
        replicas: ${REPLICAS}
        selector:
          matchLabels:
            app: ${APP_NAME}
      EOF
```

### Dynamic Parameter Lists

Generate parameter lists dynamically:

```yaml
steps:
  - name: get-environments
    command: |
      echo "dev staging prod"
    output: ENVIRONMENTS
    
  - name: deploy-all
    run: deploy-app
    parallel: ${ENVIRONMENTS}
    params: "ENVIRONMENT=${ITEM}"
```

### Parameter Groups

Organize related parameters:

```yaml
params:
  # Database settings
  - DB_HOST: localhost
  - DB_PORT: 5432
  - DB_NAME: myapp
  - DB_USER: appuser
  
  # API settings
  - API_PORT: 8080
  - API_TIMEOUT: 30
  - API_WORKERS: 4

steps:
  - name: start-services
    command: |
      # Start database
      ./start-db.sh \
        --host=${DB_HOST} \
        --port=${DB_PORT} \
        --name=${DB_NAME}
      
      # Start API
      ./start-api.sh \
        --port=${API_PORT} \
        --workers=${API_WORKERS}
```

## Special Syntax

### Quotes and Spaces

Handle values with spaces:

```yaml
params:
  - MESSAGE: "Hello World"
  - PATH: "/path/with spaces/to/file"
  - COMMAND: 'echo "nested quotes"'

steps:
  - name: use-quoted
    command: |
      echo "${MESSAGE}"
      cd "${PATH}"
      eval "${COMMAND}"
```

### Escaping Special Characters

```yaml
params:
  - DOLLAR: "Price: \$100"  # Escape dollar sign
  - QUOTE: "He said \"Hello\""  # Escape quotes
  - BACKTICK: "Use \`command\` syntax"  # Escape backtick
```

### Empty vs Unset

```yaml
params:
  - EMPTY: ""  # Empty string
  - UNSET:     # Null/undefined

steps:
  - name: check
    command: |
      # Check if empty
      if [ -z "${EMPTY}" ]; then
        echo "EMPTY is empty string"
      fi
      
      # Provide default for unset
      echo "${UNSET:-default value}"
```

## Best Practices

### 1. Use Descriptive Names

```yaml
# Good
params:
  - DEPLOYMENT_ENVIRONMENT: production
  - DATABASE_CONNECTION_STRING: postgres://...
  - ENABLE_DEBUG_LOGGING: false

# Avoid
params:
  - ENV: prod
  - CONN: postgres://...
  - DBG: 0
```

### 2. Document Parameters

```yaml
# Data Processing Pipeline
# Required parameters:
#   - INPUT_PATH: S3 path to input data
#   - OUTPUT_PATH: S3 path for results
# Optional parameters:
#   - BATCH_SIZE: Number of records per batch (default: 1000)
#   - WORKERS: Parallel workers (default: 4)

params:
  - INPUT_PATH: ""      # Required - will fail if not provided
  - OUTPUT_PATH: ""     # Required - will fail if not provided
  - BATCH_SIZE: 1000    # Optional - has default
  - WORKERS: 4          # Optional - has default
```

### 3. Group Related Parameters

```yaml
params:
  # Source configuration
  - SOURCE_TYPE: s3
  - SOURCE_BUCKET: my-bucket
  - SOURCE_PREFIX: data/raw/
  
  # Processing options
  - PROCESS_MODE: batch
  - PROCESS_THREADS: 8
  - PROCESS_TIMEOUT: 3600
  
  # Output configuration
  - OUTPUT_FORMAT: parquet
  - OUTPUT_COMPRESSION: snappy
```

### 4. Validate Early

```yaml
steps:
  - name: validate-params
    command: |
      # Check required parameters
      for param in INPUT_PATH OUTPUT_PATH; do
        if [ -z "${!param}" ]; then
          echo "ERROR: $param is required"
          exit 1
        fi
      done
      
      # Validate formats
      if ! [[ "${WORKERS}" =~ ^[0-9]+$ ]]; then
        echo "ERROR: WORKERS must be a positive integer"
        exit 1
      fi
```

### 5. Use Parameter Files

For complex configurations, load from files:

```yaml
params:
  - CONFIG_FILE: config.yaml

steps:
  - name: load-config
    command: |
      # Load additional parameters from file
      source <(yq eval '.params | to_entries | .[] | "export " + .key + "=\"" + .value + "\""' ${CONFIG_FILE})
```

## Common Patterns

### Environment-Specific Parameters

```yaml
params:
  - ENVIRONMENT: dev

steps:
  - name: load-env-config
    command: |
      case ${ENVIRONMENT} in
        dev)
          export API_URL="https://api-dev.example.com"
          export DB_HOST="dev-db.example.com"
          ;;
        prod)
          export API_URL="https://api.example.com"
          export DB_HOST="prod-db.example.com"
          ;;
      esac
```

### Feature Flags

```yaml
params:
  - ENABLE_CACHE: true
  - ENABLE_METRICS: true
  - DRY_RUN: false

steps:
  - name: process
    command: |
      ARGS=""
      [ "${ENABLE_CACHE}" = "true" ] && ARGS="$ARGS --cache"
      [ "${ENABLE_METRICS}" = "true" ] && ARGS="$ARGS --metrics"
      [ "${DRY_RUN}" = "true" ] && ARGS="$ARGS --dry-run"
      
      ./process.sh $ARGS
```

### Parameter Chaining

```yaml
params:
  - BASE_URL: "https://api.example.com"
  - API_VERSION: "v2"
  - ENDPOINT: "users"

steps:
  - name: build-url
    command: echo "${BASE_URL}/${API_VERSION}/${ENDPOINT}"
    output: FULL_URL
    
  - name: call-api
    command: curl "${FULL_URL}"
    depends: build-url
```

## Troubleshooting

### Parameter Not Found

```yaml
steps:
  - name: debug-params
    command: |
      echo "All parameters:"
      env | grep -E "^[A-Z_]+=" | sort
      
      echo "Specific param: ${MY_PARAM:-NOT SET}"
```

### Override Not Working

Check parameter precedence:
1. Command line parameters (highest)
2. DAG-defined parameters
3. Parent DAG parameters
4. Default values (lowest)

### Spaces in Parameters

```bash
# Correct - quotes preserve spaces
dagu start workflow.yaml -- MESSAGE="Hello World"

# Wrong - treated as separate parameters
dagu start workflow.yaml -- MESSAGE=Hello World
```

### Command Substitution Issues

```yaml
# Correct - backticks for command substitution
params:
  - DATE: "`date +%Y-%m-%d`"

# Wrong - literal string
params:
  - DATE: "date +%Y-%m-%d"
```

## See Also

- [Data Flow](/features/data-flow) - How data moves through workflows
- [Variables Reference](/reference/variables) - All variable types
- [Writing Workflows](/writing-workflows/) - Complete workflow guide
- [Examples](/writing-workflows/examples/) - Parameter examples