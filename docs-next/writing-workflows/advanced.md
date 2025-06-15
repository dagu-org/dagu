# Advanced Patterns

Master complex workflow scenarios and optimization techniques.

## Hierarchical Workflows

Dagu's most powerful feature is the ability to compose workflows from other workflows, creating modular, reusable components.

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
    depends: extract
    
  - name: load
    run: workflows/load.yaml
    params: "DATA=${transform.output}"
    depends: transform
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
    depends: build
```

### Sharing Data Between Workflows

```yaml
# parent.yaml
steps:
  - name: prepare-data
    run: child-workflow
    params: "DATASET=customers"
    output: PREPARED_DATA
    
  - name: process
    command: python process.py --input=${PREPARED_DATA.outputs.FILE_PATH}
    depends: prepare-data

# child-workflow.yaml
params:
  - DATASET: ""
steps:
  - name: extract
    command: extract.sh ${DATASET}
    output: FILE_PATH
```

## Dynamic Workflow Generation

### Conditional Sub-workflows

```yaml
name: adaptive-pipeline
params:
  - MODE: "standard"  # standard, fast, thorough

steps:
  - name: determine-workflow
    command: |
      case "${MODE}" in
        fast) echo "workflows/fast-process.yaml" ;;
        thorough) echo "workflows/thorough-process.yaml" ;;
        *) echo "workflows/standard-process.yaml" ;;
      esac
    output: WORKFLOW_PATH
    
  - name: execute
    run: ${WORKFLOW_PATH}
    depends: determine-workflow
```

### Dynamic Step Generation

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
    parallel: ${TASK_LIST}
    params: "FILE=${ITEM}"
    depends: discover-tasks
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
    parallel: ${CHUNKS}
    params: "CHUNK=${ITEM}"
    output: MAP_RESULTS
    depends: split-data
    
  - name: reduce-phase
    command: |
      python reduce.py \
        --inputs='${MAP_RESULTS.outputs}' \
        --output=final-result.json
    depends: map-phase
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
    depends: scatter-requests
```

## Resource Management

### Concurrency Control

```yaml
name: resource-aware-pipeline
maxActiveRuns: 1        # Only one instance of this DAG
maxActiveSteps: 5       # Max 5 steps running concurrently

steps:
  - name: cpu-intensive
    command: ./heavy-computation.sh
    env:
      - GOMAXPROCS: 4   # Limit CPU cores
      
  - name: memory-intensive
    command: ./process-large-data.sh
    env:
      - MEMORY_LIMIT: 8G
      
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
    queue: light-jobs   # Different queue for light tasks
```

## Performance Optimization

### Caching Pattern

```yaml
name: cached-pipeline
steps:
  - name: check-cache
    command: |
      CACHE_KEY=$(echo "${INPUT_PARAMS}" | sha256sum | cut -d' ' -f1)
      if [ -f "/cache/${CACHE_KEY}" ]; then
        echo "CACHE_HIT"
        cat "/cache/${CACHE_KEY}"
      else
        echo "CACHE_MISS"
      fi
    output: CACHE_STATUS
    
  - name: compute
    command: |
      ./expensive-computation.sh > result.json
      CACHE_KEY=$(echo "${INPUT_PARAMS}" | sha256sum | cut -d' ' -f1)
      cp result.json "/cache/${CACHE_KEY}"
      cat result.json
    depends: check-cache
    preconditions:
      - condition: "${CACHE_STATUS}"
        expected: "CACHE_MISS"
    output: RESULT
    
  - name: use-cached
    command: echo "${CACHE_STATUS}" | tail -n +2
    depends: check-cache
    preconditions:
      - condition: "${CACHE_STATUS}"
        expected: "re:CACHE_HIT.*"
    output: RESULT
```

### Lazy Evaluation

```yaml
name: lazy-evaluation
steps:
  - name: check-prerequisites
    command: |
      # Quick checks before expensive operations
      test -f required-file.txt && echo "READY" || echo "NOT_READY"
    output: STATUS
    
  - name: expensive-operation
    command: ./long-running-process.sh
    depends: check-prerequisites
    preconditions:
      - condition: "${STATUS}"
        expected: "READY"
```

## State Management

### Checkpointing

```yaml
name: resumable-pipeline
params:
  - CHECKPOINT_DIR: /tmp/checkpoints

steps:
  - name: stage-1
    command: |
      CHECKPOINT="${CHECKPOINT_DIR}/stage-1.done"
      if [ -f "$CHECKPOINT" ]; then
        echo "Stage 1 already completed"
      else
        ./stage-1-process.sh
        touch "$CHECKPOINT"
      fi
      
  - name: stage-2
    command: |
      CHECKPOINT="${CHECKPOINT_DIR}/stage-2.done"
      if [ -f "$CHECKPOINT" ]; then
        echo "Stage 2 already completed"
      else
        ./stage-2-process.sh
        touch "$CHECKPOINT"
      fi
    depends: stage-1
```

### Distributed Locking

```yaml
name: distributed-job
steps:
  - name: acquire-lock
    command: |
      LOCK_FILE="/tmp/locks/job.lock"
      LOCK_ACQUIRED=false
      
      for i in {1..60}; do
        if mkdir "$LOCK_FILE" 2>/dev/null; then
          LOCK_ACQUIRED=true
          echo $$ > "$LOCK_FILE/pid"
          break
        fi
        sleep 1
      done
      
      if [ "$LOCK_ACQUIRED" != "true" ]; then
        echo "Failed to acquire lock"
        exit 1
      fi
    
  - name: critical-section
    command: ./exclusive-operation.sh
    depends: acquire-lock
    
  - name: release-lock
    command: rm -rf "/tmp/locks/job.lock"
    depends: critical-section
    continueOn:
      failure: true  # Always release lock
```

## Complex Control Flow

### State Machine Pattern

```yaml
name: state-machine
params:
  - STATE: "INIT"

steps:
  - name: init-state
    command: echo "Initializing..."
    preconditions:
      - condition: "${STATE}"
        expected: "INIT"
    output: NEXT_STATE
    
  - name: processing-state
    command: ./process.sh
    preconditions:
      - condition: "${NEXT_STATE:-${STATE}}"
        expected: "PROCESSING"
    depends: init-state
    output: NEXT_STATE
    
  - name: validation-state
    command: ./validate.sh
    preconditions:
      - condition: "${NEXT_STATE:-${STATE}}"
        expected: "VALIDATION"
    depends: processing-state
    output: NEXT_STATE
    
  - name: complete-state
    command: echo "Completed!"
    preconditions:
      - condition: "${NEXT_STATE:-${STATE}}"
        expected: "COMPLETE"
    depends: validation-state
```

### Circuit Breaker

```yaml
name: circuit-breaker-pattern
env:
  - FAILURE_THRESHOLD: 3
  - FAILURE_COUNT_FILE: /tmp/failure_count

steps:
  - name: check-circuit
    command: |
      COUNT=$(cat ${FAILURE_COUNT_FILE} 2>/dev/null || echo 0)
      if [ $COUNT -ge ${FAILURE_THRESHOLD} ]; then
        echo "OPEN"
      else
        echo "CLOSED"
      fi
    output: CIRCUIT_STATE
    
  - name: execute-if-closed
    command: |
      ./risky-operation.sh && echo 0 > ${FAILURE_COUNT_FILE}
    depends: check-circuit
    preconditions:
      - condition: "${CIRCUIT_STATE}"
        expected: "CLOSED"
    continueOn:
      failure: true
      
  - name: increment-failures
    command: |
      COUNT=$(cat ${FAILURE_COUNT_FILE} 2>/dev/null || echo 0)
      echo $((COUNT + 1)) > ${FAILURE_COUNT_FILE}
    depends: execute-if-closed
    preconditions:
      - condition: "${execute-if-closed.exitCode}"
        expected: "re:[1-9][0-9]*"  # Non-zero exit code
```

## Monitoring and Observability

### Metrics Collection

```yaml
name: monitored-pipeline
steps:
  - name: start-metrics
    command: |
      START_TIME=$(date +%s)
      echo "pipeline_start_time{workflow=\"${DAG_NAME}\"} $START_TIME" \
        >> /metrics/dagu.prom
    
  - name: process-with-metrics
    command: |
      START=$(date +%s)
      ./process.sh
      DURATION=$(($(date +%s) - START))
      echo "step_duration_seconds{workflow=\"${DAG_NAME}\",step=\"process\"} $DURATION" \
        >> /metrics/dagu.prom
    depends: start-metrics
    
  - name: export-metrics
    command: |
      curl -X POST http://prometheus-pushgateway:9091/metrics/job/dagu \
        --data-binary @/metrics/dagu.prom
    depends: process-with-metrics
```

### Structured Logging

```yaml
name: structured-logging
env:
  - LOG_FORMAT: json

steps:
  - name: log-context
    command: |
      jq -n \
        --arg workflow "${DAG_NAME}" \
        --arg run_id "${DAG_RUN_ID}" \
        --arg step "process" \
        --arg level "info" \
        --arg message "Starting processing" \
        '{timestamp: now|todate, workflow: $workflow, run_id: $run_id, step: $step, level: $level, message: $message}'
    
  - name: process-with-logging
    command: |
      ./process.sh 2>&1 | while read line; do
        jq -n \
          --arg workflow "${DAG_NAME}" \
          --arg run_id "${DAG_RUN_ID}" \
          --arg step "${DAG_RUN_STEP_NAME}" \
          --arg output "$line" \
          '{timestamp: now|todate, workflow: $workflow, run_id: $run_id, step: $step, output: $output}'
      done
```

## Best Practices

### 1. Modular Design
- Build small, focused workflows
- Compose complex behavior from simple parts
- Use sub-workflows for reusability

### 2. Idempotency
- Design steps to be safely re-runnable
- Use checksums and timestamps
- Implement proper cleanup

### 3. Resource Awareness
- Set appropriate concurrency limits
- Monitor resource usage
- Use queues for resource management

### 4. Error Boundaries
- Isolate risky operations
- Implement circuit breakers
- Use graceful degradation

### 5. Observability
- Add structured logging
- Export metrics
- Track business KPIs

## Real-World Example

Here's a complete example combining multiple advanced patterns:

```yaml
name: advanced-data-pipeline
description: Production data pipeline with advanced patterns
schedule: "0 */4 * * *"  # Every 4 hours

params:
  - ENVIRONMENT: production
  - PARALLEL_WORKERS: 10
  - CACHE_TTL: 3600

env:
  - CHECKPOINT_DIR: /var/lib/dagu/checkpoints/${DAG_NAME}
  - METRICS_ENDPOINT: http://prometheus:9091

steps:
  # Check circuit breaker
  - name: check-health
    command: |
      FAILURES=$(redis-cli get "pipeline:failures" || echo 0)
      if [ $FAILURES -gt 5 ]; then
        echo "Circuit open - too many failures"
        exit 1
      fi
    
  # Discover and validate inputs
  - name: discover-inputs
    command: |
      python discover_inputs.py \
        --source=s3://data-lake/raw/ \
        --since="4 hours ago" \
        --format=json
    output: INPUT_FILES
    depends: check-health
    retryPolicy:
      limit: 3
      intervalSec: 60
    
  # Process in parallel with caching
  - name: process-batch
    run: processors/cached-transform
    parallel:
      items: ${INPUT_FILES}
      maxConcurrent: ${PARALLEL_WORKERS}
    params: |
      FILE=${ITEM}
      CACHE_TTL=${CACHE_TTL}
      CHECKPOINT_DIR=${CHECKPOINT_DIR}
    depends: discover-inputs
    output: PROCESSED_FILES
    
  # Aggregate results
  - name: aggregate
    command: |
      python aggregate_results.py \
        --inputs='${PROCESSED_FILES.outputs}' \
        --output=/tmp/aggregated.parquet \
        --checkpoint=${CHECKPOINT_DIR}/aggregate.ckpt
    depends: process-batch
    output: AGGREGATE_FILE
    
  # Quality checks
  - name: quality-check
    run: quality/comprehensive-check
    params: "FILE=${AGGREGATE_FILE} ENVIRONMENT=${ENVIRONMENT}"
    depends: aggregate
    continueOn:
      failure: false
    
  # Publish results
  - name: publish
    command: |
      aws s3 cp ${AGGREGATE_FILE} \
        s3://data-warehouse/processed/$(date +%Y/%m/%d)/ \
        --metadata workflow=${DAG_NAME},run_id=${DAG_RUN_ID}
    depends: quality-check
    retryPolicy:
      limit: 5
      intervalSec: 30
      exitCode: [1, 255]

handlerOn:
  success:
    command: |
      # Reset circuit breaker
      redis-cli del "pipeline:failures"
      # Export metrics
      curl -X POST ${METRICS_ENDPOINT}/metrics/job/dagu \
        -d "pipeline_success{workflow=\"${DAG_NAME}\"} 1"
  failure:
    command: |
      # Increment circuit breaker
      redis-cli incr "pipeline:failures"
      # Alert
      ./send-alert.sh "Pipeline failed: ${DAG_NAME}"
  exit:
    command: |
      # Cleanup
      find ${CHECKPOINT_DIR} -mtime +1 -delete
```

## Next Steps

- [Examples](/writing-workflows/examples/) - See these patterns in action
- [Reference](/reference/yaml) - Complete YAML specification
- [Features](/features/) - Explore all capabilities