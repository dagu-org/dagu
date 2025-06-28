# Advanced Patterns

Master complex workflow scenarios and optimization techniques.

## Hierarchical Workflows

Dagu's most powerful feature is the ability to compose DAGs from other DAGs. The web UI allows you to drill down into nested workflows, making it easy to manage complex systems.

### Basic Nested Workflow

```yaml
# main.yaml
name: data-pipeline
steps:
  - name: extract
    run: workflows/extract.yaml
    params: "SOURCE=production"
    
  - name: transform
    run: workflows/transform.yaml
    params: "INPUT=${extract.output}"
    
  - name: load
    run: workflows/load.yaml
    params: "DATA=${transform.output}"
```

### Multi-Level Composition

Build complex systems from simple components:

```yaml
# deployment-orchestrator.yaml
name: multi-env-deployment
params:
  - VERSION: latest
  - ENVIRONMENTS: '["dev", "staging", "prod"]'

steps:
  - name: build
    run: ci/build
    params: "VERSION=${VERSION}"
    output: BUILD_ARTIFACT
    
  - name: deploy-environments
    run: deploy/environment
    parallel: ${ENVIRONMENTS}
    params: |
      ENV=${ITEM}
      ARTIFACT=${BUILD_ARTIFACT}
      VERSION=${VERSION}
```

### Sharing Data Between Workflows

```yaml
steps:
  - name: prepare-data
    run: child-workflow
    params: "DATASET=customers"
    output: PREPARED_DATA
    
  - name: process
    command: python process.py --input=${PREPARED_DATA.outputs.FILE_PATH}

---
name: child-workflow
params:
  - DATASET: ""
steps:
  - name: extract
    command: extract.sh ${DATASET}
    output: FILE_PATH
```

## Dynamic Iteration

```yaml
name: dynamic-processor
steps:
  - name: discover-tasks
    command: |
      # Discover files to process
      find /data -name "*.csv" -type f | jq -R -s -c 'split("\n")[:-1]'
    output: TASK_LIST
    
  - name: process-dynamic
    run: processors/csv-handler
    parallel: 
      items: ${TASK_LIST}
      maxConcurrent: 1
    params: "FILE=${ITEM}"
```

## Parallel Processing Patterns

### Map-Reduce Pattern

```yaml
name: map-reduce-aggregation
steps:
  - name: split-data
    command: |
      split -n 10 large-file.txt chunk_
      ls chunk_* | jq -R -s -c 'split("\n")[:-1]'
    output: CHUNKS
    
  - name: map-phase
    run: mappers/process-chunk
    parallel:
      items: ${CHUNKS}
      maxConcurrent: 3
    params: "CHUNK=${ITEM}"
    output: MAP_RESULTS
    
  - name: reduce-phase
    command: |
      python reduce.py \
        --inputs='${MAP_RESULTS.outputs}' \
        --output=final-result.json
```

### Fork-Join Pattern

```yaml
name: fork-join-analysis
steps:
  - name: prepare
    command: ./prepare-dataset.sh
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
name: scatter-gather
params:
  - REGIONS: '["us-east-1", "eu-west-1", "ap-south-1"]'

steps:
  - name: scatter-requests
    run: regional/data-collector
    parallel: ${REGIONS}
    params: "REGION=${ITEM}"
    output: REGIONAL_DATA
    
  - name: gather-results
    command: |
      python aggregate_regional.py \
        --data='${REGIONAL_DATA.outputs}' \
        --output=global-summary.json
```

## Resource Management

### Concurrency Control

```yaml
name: resource-aware-pipeline
maxActiveRuns: 1        # Only one instance of this DAG
maxActiveSteps: 1       # Max 1 steps running concurrently

steps:
  - name: cpu-intensive
    command: ./heavy-computation.sh
      
  - name: memory-intensive
    command: ./process-large-data.sh
      
  - name: io-intensive
    parallel:
      items: ${FILE_LIST}
      maxConcurrent: 3  # Limit parallel I/O
    command: ./process-file.sh ${ITEM}
```

### Queue-Based Resource Management

```yaml
name: queue-managed-workflow
queue: heavy-jobs       # Assign to specific queue
maxActiveRuns: 2        # Queue allows 2 concurrent

steps:
  - name: resource-heavy
    command: ./intensive-task.sh
    
  - name: light-task
    command: echo "Quick task"
```

## See Also

- [Examples](/writing-workflows/examples) - See these patterns in action
- [Reference](/reference/yaml) - Complete YAML specification
- [Features](/features/) - Explore all capabilities
