# Advanced Patterns

Master complex workflow scenarios and optimization techniques.

## Hierarchical Workflows

Dagu's most powerful feature is the ability to compose DAGs from other DAGs. The web UI allows you to drill down into nested workflows, making it easy to manage complex systems.

### Basic Nested Workflow

```yaml
# main.yaml
steps:
  - run: workflows/extract.yaml
    params: "SOURCE=production"
    
  - run: workflows/transform.yaml
    params: "INPUT=${extract.output}"
    
  - run: workflows/load.yaml
    params: "DATA=${transform.output}"
```

### Multi-Level Composition

Build complex systems from simple components:

```yaml
# deployment-orchestrator.yaml
params:
  - VERSION: latest
  - ENVIRONMENTS: '["dev", "staging", "prod"]'

steps:
  - run: ci/build
    params: "VERSION=${VERSION}"
    output: BUILD_ARTIFACT
    
  - run: deploy/environment
    parallel: ${ENVIRONMENTS}
    params: |
      ENV=${ITEM}
      ARTIFACT=${BUILD_ARTIFACT}
      VERSION=${VERSION}
```

### Sharing Data Between Workflows

```yaml
steps:
  - run: child-workflow
    params: "DATASET=customers"
    output: PREPARED_DATA
    
  - python process.py --input=${PREPARED_DATA.outputs.FILE_PATH}

---
name: child-workflow
params:
  - DATASET: ""
steps:
  - command: extract.sh ${DATASET}
    output: FILE_PATH
```

## Dynamic Iteration

```yaml
steps:
  - command: |
      # Discover files to process
      find /data -name "*.csv" -type f | jq -R -s -c 'split("\n")[:-1]'
    output: TASK_LIST
    
  - run: processors/csv-handler
    parallel: 
      items: ${TASK_LIST}
      maxConcurrent: 1
    params: "FILE=${ITEM}"
```

## Parallel Processing Patterns

### Map-Reduce Pattern

```yaml
steps:
  - command: |
      split -n 10 large-file.txt chunk_
      ls chunk_* | jq -R -s -c 'split("\n")[:-1]'
    output: CHUNKS
    
  - run: mappers/process-chunk
    parallel:
      items: ${CHUNKS}
      maxConcurrent: 3
    params: "CHUNK=${ITEM}"
    output: MAP_RESULTS
    
  - |
      python reduce.py \
        --inputs='${MAP_RESULTS.outputs}' \
        --output=final-result.json
```

### Fork-Join Pattern

```yaml
steps:
  - name: prepare
    command: echo "Preparing dataset"
    output: DATASET
    
  # Fork: Parallel analysis
  - name: statistical-analysis
    command: python stats.py ${DATASET}
    depends: prepare
    output: STATS
    
  - name: ml-analysis
    command: python ml_model.py ${DATASET}
    depends: prepare
    output: ML_RESULTS
    
  - name: visualization
    command: python visualize.py ${DATASET}
    depends: prepare
    output: CHARTS
    
  # Join: Combine results
  - name: generate-report
    command: |
      python generate_report.py \
        --stats=${STATS} \
        --ml=${ML_RESULTS} \
        --charts=${CHARTS}
    depends:
      - statistical-analysis
      - ml-analysis
      - visualization
```

### Scatter-Gather Pattern

```yaml
params:
  - REGIONS: '["us-east-1", "eu-west-1", "ap-south-1"]'

steps:
  - run: regional/data-collector
    parallel: ${REGIONS}
    params: "REGION=${ITEM}"
    output: REGIONAL_DATA
    
  - |
      python aggregate_regional.py \
        --data='${REGIONAL_DATA.outputs}' \
        --output=global-summary.json
```

## Restricting Runtime Parameters

Control how users can interact with your DAGs at runtime using the `runConfig` field:

### Enforcing Fixed Parameters

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

### Enforcing Run ID Format

Ensure consistent run ID naming:

```yaml
runConfig:
  disableParamEdit: false  # Parameters can be changed
  disableRunIdEdit: true   # Must use auto-generated run IDs

steps:
  - echo "Auditing run ${DAG_RUN_ID}"
```

### Use Cases

- **Production Workflows**: Prevent accidental parameter changes in critical workflows
- **Compliance**: Ensure audit trails use consistent, auto-generated run IDs
- **Safety**: Lock down dangerous parameters while allowing safe customization
- **Templates**: Create reusable workflows with fixed configurations

## Resource Management

### Concurrency Control

```yaml
maxActiveRuns: 1        # Only one instance of this DAG
maxActiveSteps: 1       # Max 1 steps running concurrently

steps:
  - sh -c "echo Starting heavy computation; sleep 3; echo Completed"
  - echo "Processing large dataset"
  - parallel:
      items: ${FILE_LIST}
      maxConcurrent: 3  # Limit parallel I/O
    command: echo "Processing file ${ITEM}"
```

### Queue-Based Resource Management

```yaml
queue: heavy-jobs       # Assign to specific queue
maxActiveRuns: 2        # Queue allows 2 concurrent

steps:
  - sh -c "echo Starting intensive task; sleep 10; echo Done"
  - echo "Quick task"
```

## See Also

- [Examples](/writing-workflows/examples) - See these patterns in action
- [Reference](/reference/yaml) - Complete YAML specification
- [Features](/features/) - Explore all capabilities
