# Parameters

Make workflows dynamic and reusable with runtime parameters.

## Parameter Definition

```yaml
# Named parameters
params:
  - ENVIRONMENT: dev
  - PORT: 8080
  - DEBUG: false

# Positional parameters (accessed as $1, $2, ...)
params: first second third

# JSON Schema validation
params:
  schema: "./schemas/params.json"  # Local file or remote URL
  values:  # These defaults take precedence over schema defaults
    ENVIRONMENT: dev
    PORT: 8080
    DEBUG: false

steps:
  - echo $1 --env=${ENVIRONMENT} --port=${PORT}
```

## JSON Schema Validation

Validate parameters against a JSON Schema to ensure type safety and enforce constraints:

```yaml
params:
  schema: "https://example.com/schemas/dag-params.json"
  values:
    batch_size: 25
    environment: "staging"
```

The schema can be:
- **Local file**: `"./schemas/params.json"` or `"/absolute/path/to/schema.json"`
- **Remote URL**: `"https://example.com/schemas/params.json"`

### Schema-Only Mode

Define validation without default values:

```yaml
params:
  schema: "./schemas/params.json"
  # No values - all parameters must be provided at runtime
```

### Reserved Keywords

⚠️ **Important**: The following keywords are reserved and cannot be used as parameter names:
- `schema` - References the JSON Schema file

Using these as parameter names could possibly cause parsing errors.

### Example JSON Schema

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "batch_size": {
      "type": "integer",
      "default": 10,
      "minimum": 1,
      "maximum": 100
    },
    "environment": {
      "type": "string",
      "default": "dev",
      "enum": ["dev", "staging", "prod"]
    },
    "debug": {
      "type": "boolean",
      "default": false
    }
  },
  "required": ["batch_size", "environment"]
}
```

### Schema Validation

- **Runtime Validation**: Parameters are validated when the DAG is loaded, before execution
- **CLI Override Validation**: Command-line parameters are also validated against the schema
- **Error Messages**: Clear error messages indicate which parameters failed validation and why
- **Backward Compatibility**: Existing parameter formats continue to work unchanged

**Parameter Precedence**: CLI parameters > YAML values > schema defaults. Schema defaults only fill missing parameters and never override explicit values.

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
  
  # Substring slices & param chaining
  - SOURCE_ID: HBL01_22OCT2025_0536
  - PREFIX: ${SOURCE_ID:0:5}
  - REMAINDER: ${SOURCE_ID:5}
  - ARTIFACT: backup-${PREFIX}-${DATE}.tar.gz

steps:
  - backup-${DATE}-${GIT_COMMIT}.tar.gz
```

## Using Parameters

```yaml
params:
  - INPUT: data.csv
  - THREADS: 4
  - SKIP_TESTS: false

steps:
  # In commands
  - python processor.py --input ${INPUT} --threads ${THREADS}
    
  # In conditions
  - command: npm test
    preconditions:
      - condition: "${SKIP_TESTS}"
        expected: "false"
        
  # In environment
  - env:
      - API_VERSION: ${VERSION:-v1}
    command: ./app
```

## Enforcing Fixed Parameters

Prevent users from modifying critical parameters:

```yaml
runConfig:
  disableParamEdit: true  # Parameters cannot be changed
  disableRunIdEdit: false # Custom run IDs still allowed

params:
  - ENVIRONMENT: production  # Always production
  - DB_HOST: prod.db.example.com
  - SAFETY_MODE: enabled

steps:
  - echo "Deploying to ${ENVIRONMENT} with DB ${DB_HOST}"
```

## Enforcing Run ID Format

Ensure consistent run ID naming:

```yaml
runConfig:
  disableParamEdit: false  # Parameters can be changed
  disableRunIdEdit: true   # Must use auto-generated run IDs

steps:
  - echo "Auditing run ${DAG_RUN_ID}"
```
