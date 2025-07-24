# Parallel Execution Requirements for Dagu

## Overview

This document outlines the requirements for implementing parallel execution functionality in Dagu, allowing users to run multiple instances of commands or sub-DAGs with different parameters.

## Core Requirements

### 1. Basic Parallel Execution

- **Field Name**: `parallel`
- **Purpose**: Execute the same command or DAG multiple times with different parameters
- **Basic Syntax**:
  ```yaml
  steps:
    - name: process-all
      run: workflows/processor
      parallel: ${ITEMS}  # Direct array reference from previous step
                         # Default maxConcurrent: 10
  ```

### 2. Input Sources

#### 2.1 Static List
```yaml
# Simple form - uses default maxConcurrent (10)
parallel:
  - SOURCE: s3://customers
  - SOURCE: s3://products
  - SOURCE: s3://orders

# With maxConcurrent control
parallel:
  maxConcurrent: 2
  items:
    - SOURCE: s3://customers
    - SOURCE: s3://products
    - SOURCE: s3://orders
```

**Note**: When using objects/dictionaries, the keys (like `SOURCE` above) become parameters that are passed to the step execution. This is equivalent to passing `params: "SOURCE=s3://customers"` for each iteration.

#### 2.2 Dynamic from Previous Step
```yaml
steps:
  - name: get-items
    command: echo '["item1", "item2", "item3"]'
    output: ITEMS
    
  - name: process
    run: process.sh ${ITEM}
    parallel: ${ITEMS}
```

#### 2.3 Complex Objects
```yaml
steps:
  - name: get-configs
    command: |
      echo '[
        {"name": "service1", "port": 8080},
        {"name": "service2", "port": 8081}
      ]'
    output: CONFIGS
    
  - name: deploy
    run: deploy.sh
    parallel: ${CONFIGS}
    params:
      - SERVICE_NAME: ${ITEM.name}
      - SERVICE_PORT: ${ITEM.port}
```

### 3. Concurrency Control

```yaml
# For dynamic items
parallel:
  items: ${ITEMS}
  maxConcurrent: 5  # Default: 10, Maximum parallel executions

# For static items (object form)
parallel:
  maxConcurrent: 5
  items:
    - item1
    - item2
    - item3
```

### 4. Parameter Substitution

- **Special Variable**: `${ITEM}` - represents current item in iteration (uppercase to match Dagu's convention for special variables like `${DAG_NAME}`, `${DAG_RUN_ID}`)
- **Parameter Passing**: When items are objects/dictionaries, their key-value pairs are automatically available as parameters in the child DAG/command
- **Available in**:
  - `run` field
  - `params` field
  - `env` field
  - `command` field (when `run` is a command)

Example:
```yaml
parallel:
  - SOURCE: s3://customers
    TYPE: csv
  - SOURCE: s3://products
    TYPE: json

# In the child DAG, these are available as:
# ${SOURCE} and ${TYPE} parameters
```

### 5. Auto-Detection: Command vs Sub-DAG

Dagu should automatically detect whether `run` value is a command or sub-DAG:

#### Sub-DAG Detection Rules:
- Contains `/` (path separator): `workflows/extract`
- Starts with `./` or `../`: `./local-workflow`
- Ends with `.yaml` or `.yml`: `process.yaml`
- Exists as a file in DAGs directory

#### Command Detection Rules:
- Contains spaces (unless quoted path): `python script.py`
- Starts with shell built-ins: `echo`, `cd`, etc.
- Everything else treated as command

### 6. Error Handling

#### 6.1 Basic Configuration
```yaml
parallel:
  items: ${ITEMS}
  continueOnError: true  # Default: false
```

#### 6.2 Advanced Configuration
```yaml
parallel:
  items: ${ITEMS}
  continueOnError: 
    enabled: true
    maxFailures: 3  # Stop after N failures
    failureRate: 0.5  # Stop if >50% fail
```

### 7. Output Aggregation

```yaml
steps:
  - name: parallel-process
    run: process.sh ${ITEM}
    parallel: ${ITEMS}
    output: RESULTS  # Array of outputs from each execution
    
  - name: use-results
    command: echo '${RESULTS}'  # Access as array
    env:
      - FIRST_RESULT: ${RESULTS[0]}
      - ALL_RESULTS: ${RESULTS}
```

### 8. Advanced Configuration (Optional)

```yaml
parallel:
  items: ${ITEMS}
  maxConcurrent: 5
  
  # Timeout per item
  itemTimeout: 300  # seconds
  
  # Retry per item
  retryPolicy:
    limit: 3
    intervalSec: 30
    
  # Progress tracking
  showProgress: true  # Show "5/20 completed" in UI
  
  # Execution order
  ordered: false  # Default: false (parallel), true = sequential
```

## Implementation Phases

### Phase 1: MVP
- Direct array reference (`parallel: ${ITEMS}`)
- Simple ${ITEM} substitution in `run` field
- Basic concurrency control (`maxConcurrent`)
- Auto-detection of command vs sub-DAG

### Phase 2: Enhanced Control
- `continueOnError` support
- Output aggregation as array
- ${ITEM} substitution in params/env
- Complex object support (${ITEM.field})

### Phase 3: Advanced Features
- Retry policy per item
- Progress tracking
- Timeout per item
- Failure rate limits

## Usage Examples

### Simple Command Parallel
```yaml
steps:
  - name: get-files
    command: find /data -name "*.csv" -printf "%f\n"
    output: FILES
    
  - name: process-files
    run: python process.py /data/${ITEM}
    parallel: ${FILES}
    maxConcurrent: 10
```

### Sub-DAG Parallel
```yaml
steps:
  - name: get-regions
    command: echo '["us-east-1", "eu-west-1", "ap-south-1"]'
    output: REGIONS
    
  - name: deploy-all
    run: workflows/deploy-region
    parallel: ${REGIONS}
    params:
      - REGION: ${ITEM}
    maxConcurrent: 2
```

### Mixed Execution
```yaml
steps:
  - name: get-tasks
    command: |
      echo '[
        {"type": "backup", "target": "db1", "method": "pg_dump db1"},
        {"type": "backup", "target": "files", "method": "workflows/file-backup"}
      ]'
    output: TASKS
    
  - name: run-backups
    run: ${ITEM.method}
    parallel: ${TASKS}
    params:
      - TARGET: ${ITEM.target}
      - BACKUP_ID: ${DAG_RUN_ID}_${ITEM.target}
```

## Edge Cases to Handle

1. **Empty Array**: Skip execution if parallel items is empty
2. **Circular Dependencies**: Prevent DAG from calling itself in parallel
3. **Resource Control**: Use `maxConcurrent` to prevent system overload
4. **Memory Management**: Handle output aggregation efficiently (can defer optimization)

## UI Considerations

1. Show parallel execution as grouped items in DAG visualization
2. Display progress: "15/100 completed (2 failed)"
3. Allow drilling down into individual executions
4. Aggregate logs with filtering by item

## Backward Compatibility

- New `parallel` field is optional
- Existing DAGs continue to work unchanged
- No breaking changes to current syntax

## Success Metrics

1. Users can parallelize workflows with minimal configuration
2. Performance scales linearly with `maxConcurrent`
3. Clear error messages for configuration issues
4. Intuitive UI representation of parallel execution

## Known Limitations & Future Considerations

1. **Large Array Handling**: Initial implementation processes all items in memory. For very large arrays (1000+ items), future versions may implement:
   - Streaming/pagination of items
   - Chunked processing
   - Memory-efficient output aggregation
   
2. **Resource Monitoring**: Currently relies on `maxConcurrent` for resource control. Future versions may add:
   - Per-process memory/CPU limits
   - System load monitoring
   - Dynamic throttling