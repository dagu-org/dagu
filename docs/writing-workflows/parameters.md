# Parameters

Make workflows dynamic and reusable with runtime parameters.

## Parameter Definition

```yaml
# Named parameters (recommended)
params:
  - ENVIRONMENT: dev
  - PORT: 8080
  - DEBUG: false

# List format
params:
  - DATABASE: postgres
  - VERSION: latest

# Positional parameters
params: first second third

# Mixed
params: "config.json ENVIRONMENT=prod"

steps:
  - name: use-params
    command: ./app $1 --env=${ENVIRONMENT} --port=${PORT}
```

## Passing Parameters

```bash
# Named parameters
dagu start workflow.yaml -- ENVIRONMENT=prod PORT=80

# Positional parameters  
dagu start workflow.yaml -- input.csv output.json

# Mixed
dagu start workflow.yaml -- config.json ENVIRONMENT=prod

# With spaces
dagu start workflow.yaml -- MESSAGE="Hello World"
```

## Dynamic Parameters

```yaml
params:
  # Command substitution
  - DATE: "`date +%Y-%m-%d`"
  - GIT_COMMIT: "`git rev-parse --short HEAD`"
  
  # Environment variables
  - USER: ${USER}
  - LOG_PATH: ${LOG_DIR:-/var/log}  # With default

steps:
  - name: use
    command: backup-${DATE}-${GIT_COMMIT}.tar.gz
```

## Using Parameters

```yaml
params:
  - INPUT: data.csv
  - THREADS: 4
  - SKIP_TESTS: false

steps:
  # In commands
  - name: process
    command: python processor.py --input ${INPUT} --threads ${THREADS}
    
  # In conditions
  - name: test
    command: npm test
    preconditions:
      - condition: "${SKIP_TESTS}"
        expected: "false"
        
  # In environment
  - name: run
    env:
      - API_VERSION: ${VERSION:-v1}
    command: ./app
```

## See Also

- [Data & Variables](/writing-workflows/data-variables) - Complete variable guide
- [Examples](/writing-workflows/examples) - Real-world examples
