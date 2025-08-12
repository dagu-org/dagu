# Control Flow

Control how your DAGs executes with conditions, dependencies, and repetition.

## Dependencies

Define execution order with step dependencies.

### Basic Dependencies

```yaml
steps:
  - name: download
    command: wget https://example.com/data.zip
    
  - name: extract
    command: unzip data.zip
    
  - name: process
    command: python process.py
```

### Multiple Dependencies

```yaml
steps:
  - name: download-a
    command: wget https://example.com/a.zip
    
  - name: download-b
    command: wget https://example.com/b.zip
    
  - name: merge
    command: echo "Merging a.zip and b.zip"
    depends:
      - download-a
      - download-b
```

### Complex Dependency Graphs

```yaml
steps:
  - name: setup
    command: echo "Setting up environment"
    
  - name: test-unit
    command: echo "Running unit tests"
    depends: setup
    
  - name: test-integration
    command: echo "Running integration tests"
    depends: setup
    
  - name: build
    command: echo "Building application"
    depends:
      - test-unit
      - test-integration
      
  - name: deploy
    command: echo "Deploying application"
    depends: build
```

## Conditional Execution

Run steps only when conditions are met.

### Basic Preconditions

```yaml
steps:
  - name: production-only
    command: echo "Deploying to production"
    preconditions:
      - condition: "${ENVIRONMENT}"
        expected: "production"
```

### Command Output Conditions

```yaml
steps:
  - name: check-branch
    command: echo "Deploying application"
    preconditions:
      - condition: "`git branch --show-current`"
        expected: "main"
```

### Regex Matching

```yaml
steps:
  - name: weekday-only
    command: echo "Running batch job"
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
  - name: conditional-deploy
    command: echo "Deploying application"
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
  - name: process-if-exists
    command: echo "Processing"
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
  - name: wait-for-service
    command: nc -z localhost 8080
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
  - name: wait-for-completion
    command: check-job-status.sh
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
  - name: monitor-process
    command: pgrep -f "my-app"
    repeatPolicy:
      repeat: while
      exitCode: [0]      # Exit code 0 means process found
      intervalSec: 60    # Check every minute
```

#### Until File Exists
```yaml
steps:
  - name: wait-for-output
    command: test -f /tmp/output.csv
    repeatPolicy:
      repeat: until
      exitCode: [0]      # Exit code 0 means file exists
      intervalSec: 5
      limit: 60          # Maximum 5 minutes
```

#### While Condition with Output
```yaml
steps:
  - name: keep-alive
    command: curl -s http://api/health
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
  - name: wait-for-service-backoff
    command: nc -z localhost 8080
    repeatPolicy:
      repeat: while
      exitCode: [1]        # Repeat while connection fails
      intervalSec: 1       # Start with 1 second
      backoff: true        # true = 2.0 multiplier
      limit: 10
      # Intervals: 1s, 2s, 4s, 8s, 16s, 32s...
      
  # Custom backoff multiplier with until mode
  - name: monitor-job-backoff
    command: check-job-status.sh
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
  - name: poll-api-capped
    command: curl -s https://api.example.com/status
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

**Use Cases**:
- **Service startup**: Start checking frequently, then reduce load
- **API polling**: Avoid rate limits with increasing intervals  
- **Resource monitoring**: Balance responsiveness with efficiency

### Legacy Format (Deprecated)

The old boolean format is still supported but deprecated:

```yaml
steps:
  - name: old-style
    command: echo "Checking status"
    repeatPolicy:
      repeat: true       # Deprecated: use 'while' or 'until' instead
      intervalSec: 60
```

## Continue On Conditions

Control workflow behavior when steps fail or produce specific outputs.

### Continue on Failure

```yaml
steps:
  - name: optional-cleanup
    command: echo "Cleaning up"
    continueOn:
      failure: true
      
  - name: main-process
    command: echo "Processing"
```

### Continue on Specific Exit Codes

```yaml
steps:
  - name: check-optional
    command: echo "Checking status"
    continueOn:
      exitCode: [0, 1, 2]  # Continue on these codes
      
  - name: process
    command: echo "Processing"
```

### Continue on Output Match

```yaml
steps:
  - name: validate
    command: echo "Validating"
    continueOn:
      output: 
        - "WARNING"
        - "SKIP"
        - "re:^\[WARN\]"        # Regex: lines starting with [WARN]
        - "re:error.*ignored"   # Regex: error...ignored pattern
      
  - name: process
    command: echo "Processing"
```

### Continue on Skipped

```yaml
steps:
  - name: optional-feature
    command: echo "Enabling feature"
    preconditions:
      - condition: "${FEATURE_FLAG}"
        expected: "enabled"
    continueOn:
      skipped: true  # Continue even if precondition fails
      
  - name: main-process
    command: echo "Processing"  # Runs regardless of optional feature
```

### Mark as Success

```yaml
steps:
  - name: best-effort
    command: echo "Running optional task"
    continueOn:
      failure: true
      markSuccess: true  # Mark step as successful
```

### Complex Conditions

Combine multiple conditions for sophisticated control flow:

```yaml
steps:
  # Tool with complex exit code meanings
  - name: analysis-tool
    command: echo "Analyzing data"
    continueOn:
      exitCode: [0, 3, 4, 5]  # Various non-error states
      output:
        - "Analysis complete with warnings"
        - "re:Found [0-9]+ minor issues"
      markSuccess: true
      
  # Graceful degradation pattern
  - name: try-advanced-method
    command: echo "Processing with advanced settings"
    continueOn:
      failure: true
      output: ["FALLBACK REQUIRED", "re:.*not available.*"]
      
  - name: fallback-method
    command: echo "Processing with simple settings"
    preconditions:
      - condition: "${TRY_ADVANCED_METHOD_EXIT_CODE}"
        expected: "re:[1-9][0-9]*"
        
  # Skip pattern with continuation
  - name: optional-feature
    command: echo "Running feature"
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
  - name: process
    command: echo "Running daily job"
```

### Skip If Already Successful

```yaml
schedule: "0 * * * *"  # Every hour
skipIfSuccessful: true  # Skip if already ran successfully today (e.g., run manually)

steps:
  - name: hourly-sync
    command: echo "Syncing data"
```

## Advanced Patterns

### Conditional Branching

```yaml
params:
  - ACTION: "deploy"

steps:
  - name: build
    command: echo "Building application"
    preconditions:
      - condition: "${ACTION}"
        expected: "re:build|deploy"
        
  - name: test
    command: echo "Running tests"
    preconditions:
      - condition: "${ACTION}"
        expected: "re:test|deploy"
    
  - name: deploy
    command: echo "Deploying application"
    preconditions:
      - condition: "${ACTION}"
        expected: "deploy"
```

## See Also

- [Error Handling](/writing-workflows/error-handling) - Handle failures gracefully
- [Data & Variables](/writing-workflows/data-variables) - Pass data between steps
- [Basics](/writing-workflows/basics) - Workflow fundamentals
