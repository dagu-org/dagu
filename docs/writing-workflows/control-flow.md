# Control Flow

Control how your DAGs executes with conditions, dependencies, and repetition.

## Dependencies

Define execution order with step dependencies.

### Basic Dependencies

```yaml
steps:
  - wget https://example.com/data.zip  # Download archive
  - unzip data.zip                     # Extract files
  - python process.py                  # Process data
```

### Multiple Dependencies

```yaml
steps:
  - name: download-a
    command: wget https://example.com/a.zip
    
  - name: download-b
    command: wget https://example.com/b.zip
    
  - command: echo "Merging a.zip and b.zip"
    depends:
      - download-a
      - download-b
```

## Modular Workflows and Iteration Patterns

### Nested Workflows

Run other workflows as steps and compose them hierarchically.

```yaml
steps:
  - call: workflows/extract.yaml
    params: "SOURCE=production"

  - call: workflows/transform.yaml
    params: "INPUT=${extract.output}"
  - call: workflows/load.yaml
    params: "DATA=${transform.output}"
```

### Multiple DAGs in One File

Define multiple DAGs separated by `---` and call by name.

```yaml
steps:
  - call: data-processor
    params: "TYPE=daily"

---

name: data-processor
params:
  - TYPE: "batch"
steps:
  - echo "Extracting ${TYPE} data"
  - echo "Transforming data"
```

### Dynamic Iteration

Discover work at runtime and iterate over it in parallel.

```yaml
steps:
  - command: |
      echo '["file1.csv","file2.csv","file3.csv"]'
    output: TASK_LIST
  - call: worker
    parallel:
      items: ${TASK_LIST}
      maxConcurrent: 1
    params: "FILE=${ITEM}"

---
name: worker
params:
  - FILE: ""
steps:
  - echo "Processing ${FILE}"

```

### Map-Reduce Pattern

Split, map in parallel, then reduce results.

```yaml
steps:
  - command: |
      echo '["chunk1","chunk2","chunk3"]'
    output: CHUNKS
  - call: worker
    parallel:
      items: ${CHUNKS}
      maxConcurrent: 3
    params: "CHUNK=${ITEM}"
    output: MAP_RESULTS

  - |
      echo "Reducing results from ${MAP_RESULTS.outputs}"
---
name: worker
params:
  - CHUNK: ""
steps:
  - command: echo "Processing ${CHUNK}"
    output: RESULT
```

## Conditional Execution

Run steps only when conditions are met.

### Basic Preconditions

```yaml
steps:
  - command: echo "Deploying to production"
    preconditions:
      - condition: "${ENVIRONMENT}"
        expected: "production"
```

### Command Output Conditions

```yaml
steps:
  - command: echo "Deploying application"
    preconditions:
      - condition: "`git branch --show-current`"
        expected: "main"
```

### Regex Matching

```yaml
steps:
  # Run only on weekdays
  - command: echo "Running batch job"
    preconditions:
      - condition: "`date +%u`"
        expected: "re:[1-5]"  # Monday-Friday
```

**Note**: When using regex patterns with command outputs, be aware that:
- Lines over 64KB are automatically handled with larger buffers  
- If the total output exceeds `maxOutputSize` (default 1MB), the step will fail with an error and the output variable won't be set
- For `continueOn.output` patterns in log files, lines up to `maxOutputSize` can be matched

### Multiple Conditions

All conditions must pass:

```yaml
steps:
  - command: echo "Deploying application"
    preconditions:
      - condition: "${ENVIRONMENT}"
        expected: "production"
      - condition: "${APPROVED}"
        expected: "true"
      - condition: "`date +%H`"
        expected: "re:0[8-9]|1[0-7]"  # 8 AM - 5 PM
```

### File/Directory Checks

```yaml
steps:
  - command: echo "Processing"
    preconditions:
      - condition: "test -f /data/input.csv"
      - condition: "test -d /output"
```

## Repetition

Repeat steps with explicit 'while' or 'until' modes for clear control flow.

### Repeat While Mode

The 'while' mode repeats a step while a condition is true.

```yaml
steps:
  - command: nc -z localhost 8080
    repeatPolicy:
      repeat: while
      exitCode: [1]      # Repeat WHILE connection fails (exit code 1)
      intervalSec: 10    # Wait 10 seconds between attempts
      limit: 30          # Maximum 30 attempts
```

### Repeat Until Mode

The 'until' mode repeats a step until a condition becomes true.

```yaml
steps:
  - command: check-job-status.sh
    output: STATUS
    repeatPolicy:
      repeat: until
      condition: "${STATUS}"
      expected: "COMPLETED"   # Repeat UNTIL status is COMPLETED
      intervalSec: 30
      limit: 120              # Maximum 1 hour
```

### Conditional Repeat Patterns

#### While Process is Running
```yaml
steps:
  - command: pgrep -f "my-app"
    repeatPolicy:
      repeat: while
      exitCode: [0]      # Exit code 0 means process found
      intervalSec: 60    # Check every minute
```

#### Until File Exists
```yaml
steps:
  - command: test -f /tmp/output.csv
    repeatPolicy:
      repeat: until
      exitCode: [0]      # Exit code 0 means file exists
      intervalSec: 5
      limit: 60          # Maximum 5 minutes
```

#### While Condition with Output
```yaml
steps:
  - command: curl -s http://api/health
    output: HEALTH_STATUS
    repeatPolicy:
      repeat: while
      condition: "${HEALTH_STATUS}"
      expected: "healthy"
      intervalSec: 30
```

### Exponential Backoff for Repeats

Gradually increase intervals between repeat attempts:

```yaml
steps:
  # Exponential backoff with while mode
  - command: nc -z localhost 8080
    repeatPolicy:
      repeat: while
      exitCode: [1]        # Repeat while connection fails
      intervalSec: 1       # Start with 1 second
      backoff: true        # true = 2.0 multiplier
      limit: 10
      # Intervals: 1s, 2s, 4s, 8s, 16s, 32s...
      
  # Custom backoff multiplier with until mode
  - command: check-job-status.sh
    output: STATUS
    repeatPolicy:
      repeat: until
      condition: "${STATUS}"
      expected: "COMPLETED"
      intervalSec: 5
      backoff: 1.5         # Gentler backoff
      limit: 20
      # Intervals: 5s, 7.5s, 11.25s, 16.875s...
      
  # Backoff with max interval cap
  - command: curl -s https://api.example.com/status
    output: API_STATUS
    repeatPolicy:
      repeat: until
      condition: "${API_STATUS}"
      expected: "ready"
      intervalSec: 2
      backoff: 2.0
      maxIntervalSec: 60   # Never wait more than 1 minute
      limit: 100
      # Intervals: 2s, 4s, 8s, 16s, 32s, 60s, 60s, 60s...
```

**Backoff Formula**: `interval * (backoff ^ attemptCount)`

## Continue On Conditions

### Continue on Failure

```yaml
steps:
  - command: echo "Cleaning up"
    continueOn:
      failure: true
  - echo "Processing"
```

### Continue on Specific Exit Codes

```yaml
steps:
  - command: echo "Checking status"
    continueOn:
      exitCode: [0, 1, 2]  # Continue on these codes
  - echo "Processing"
```

### Continue on Output Match

```yaml
steps:
  - command: echo "Validating"
    continueOn:
      output: 
        - "WARNING"
        - "SKIP"
        - "re:^\[WARN\]"        # Regex: lines starting with [WARN]
        - "re:error.*ignored"   # Regex: error...ignored pattern
  - echo "Processing"
```

### Continue on Skipped

```yaml
steps:
  - command: echo "Enabling feature"
    preconditions:
      - condition: "${FEATURE_FLAG}"
        expected: "enabled"
    continueOn:
      skipped: true  # Continue even if precondition fails
  - echo "Processing"  # Runs regardless of optional feature
```

### Mark as Success

```yaml
steps:
  - command: echo "Running optional task"
    continueOn:
      failure: true
      markSuccess: true  # Mark step as successful
```

### Complex Conditions

Combine multiple conditions for sophisticated control flow:

```yaml
steps:
  # Tool with complex exit code meanings
  - command: echo "Analyzing data"
    continueOn:
      exitCode: [0, 3, 4, 5]  # Various non-error states
      output:
        - "Analysis complete with warnings"
        - "re:Found [0-9]+ minor issues"
      markSuccess: true
      
  # Graceful degradation pattern
  - command: echo "Processing with advanced settings"
    continueOn:
      failure: true
      output: ["FALLBACK REQUIRED", "re:.*not available.*"]
      
  - command: echo "Processing with simple settings"
    preconditions:
      - condition: "${TRY_ADVANCED_METHOD_EXIT_CODE}"
        expected: "re:[1-9][0-9]*"
        
  # Skip pattern with continuation
  - command: echo "Running feature"
    preconditions:
      - condition: "${ENABLE_FEATURE}"
        expected: "true"
    continueOn:
      skipped: true  # Continue if precondition not met
```

See the [Continue On Reference](/reference/continue-on) for complete documentation.

## DAG-Level Conditions

### Preconditions

```yaml
preconditions:
  - condition: "`date +%u`"
    expected: "re:[1-5]"  # Weekdays only

steps:
  - echo "Running daily job"
```

### Skip If Already Successful

```yaml
schedule: "0 * * * *"  # Every hour
skipIfSuccessful: true  # Skip if already ran successfully today (e.g., run manually)

steps:
  - echo "Syncing data"
```
