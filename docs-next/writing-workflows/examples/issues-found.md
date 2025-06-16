# Issues Found in Examples Documentation

## 1. Incorrect Field Names

### Parallel Execution Example (Line 34)
**Issue**: Invalid field `parallel`
```yaml
# INCORRECT:
steps:
  - name: process
    parallel: [A, B, C]
    command: echo "Processing $ITEM"
```

**Correct**: Use `run` with a sub-DAG or use the parallel executor type
```yaml
# OPTION 1: Using parallel executor
steps:
  - name: process
    executor:
      type: parallel
      config:
        items: [A, B, C]
    command: echo "Processing $ITEM"

# OPTION 2: Using run with parallel parameter
steps:
  - name: process-items
    run: process-dag
    parallel:
      items: [A, B, C]
```

### Repeat Until Condition (Line 234)
**Issue**: Invalid field `maxRetries` in repeatPolicy
```yaml
# INCORRECT:
repeatPolicy:
  intervalSec: 10
  maxRetries: 30
```

**Correct**: There is no `maxRetries` field in repeatPolicy
```yaml
repeatPolicy:
  intervalSec: 10
  # Use retryPolicy for limiting attempts, or use condition/exitCode for repeat control
```

### Queue Management Example (Line 383)
**Issue**: The `queue` field is not shown in the examples but referenced in descriptions
```yaml
# Should include:
queue: critical-jobs
maxActiveRuns: 1
```

## 2. Missing Important Examples from RST Documentation

### Missing Parallel Execution Examples
The RST documentation has comprehensive parallel execution examples that are missing:
- Parallel execution with object parameters
- Parallel execution with maxConcurrent control
- Dynamic parallel execution with command output

### Missing Advanced RepeatPolicy Examples
The RST documentation shows important repeatPolicy patterns:
- Repeat based on condition evaluation
- Repeat based on exit codes
- Repeat forever with interval

### Missing Queue Management Examples
The RST documentation has detailed queue examples:
- Basic queue assignment
- Disabling queueing with maxActiveRuns: -1
- Global queue configuration

## 3. Incorrect YAML Syntax

### Data Pipeline Example (Line 447)
**Issue**: Using `parallel` with array items incorrectly
```yaml
# INCORRECT:
- name: transform
  parallel: [users, orders, products]
  command: python transform.py --type=$ITEM --input=${RAW_DATA}
```

**Correct**: Should use executor configuration or run with parallel
```yaml
- name: transform
  executor:
    type: parallel
    config:
      items: [users, orders, products]
  command: python transform.py --type=$ITEM --input=${RAW_DATA}
```

## 4. Features That Don't Exist

### Timeout Field (General)
The examples use `timeout:` at the DAG level, but based on the definition.go file, it should be `timeoutSec:` (in seconds).

## 5. Missing Critical Examples

### From CLAUDE.md but not in examples:
1. **nix-shell with packages**:
```yaml
steps:
  - name: run-with-nix
    shell: nix-shell
    shellPackages: [python311, nodejs]
    command: python script.py
```

2. **Signal handling**:
```yaml
steps:
  - name: graceful-shutdown
    command: long-running-process
    signalOnStop: SIGTERM
```

3. **Output size limits**:
```yaml
maxOutputSize: 10485760  # 10MB
steps:
  - name: process
    command: generate-large-output.sh
```

4. **Complex dependency patterns** with type specification:
```yaml
type: graph  # Explicitly set execution type
steps:
  - name: step1
    command: echo start
  - name: step2
    depends: step1
```

## 6. Recommendations

1. **Add Missing Examples**:
   - Advanced parallel execution patterns
   - Queue management with all options
   - Signal handling examples
   - nix-shell usage
   - Output size configuration

2. **Fix Field Names**:
   - Change `timeout` to `timeoutSec` at DAG level
   - Remove `maxRetries` from repeatPolicy examples
   - Fix parallel execution syntax

3. **Add More Real-World Examples**:
   - Multi-environment deployment with hierarchical DAGs
   - Complex conditional workflows
   - Error recovery patterns
   - Resource management examples

4. **Include Important Configuration**:
   - Base configuration inheritance
   - Global vs per-DAG settings
   - Environment variable usage patterns

5. **Add Examples for Advanced Features**:
   - Hierarchical DAG composition
   - Dynamic parameter passing
   - Complex preconditions with regex
   - Continue on specific exit codes

## Summary

The examples need significant updates to:
1. Correct syntax errors and invalid field names
2. Include missing examples from the RST documentation
3. Add examples for advanced features mentioned in CLAUDE.md
4. Ensure all examples are tested and working
5. Provide more complex, real-world scenarios

The most critical issues are the incorrect `parallel` field usage and missing advanced examples that showcase Dagu's full capabilities.
