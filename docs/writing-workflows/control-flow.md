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
    command: ./merge.sh a.zip b.zip
    depends:
      - download-a
      - download-b
```

### Complex Dependency Graphs

```yaml
steps:
  - name: setup
    command: ./setup.sh
    
  - name: test-unit
    command: ./test-unit.sh
    depends: setup
    
  - name: test-integration
    command: ./test-integration.sh
    depends: setup
    
  - name: build
    command: ./build.sh
    depends:
      - test-unit
      - test-integration
      
  - name: deploy
    command: ./deploy.sh
    depends: build
```

## Conditional Execution

Run steps only when conditions are met.

### Basic Preconditions

```yaml
steps:
  - name: production-only
    command: ./deploy-prod.sh
    preconditions:
      - condition: "${ENVIRONMENT}"
        expected: "production"
```

### Command Output Conditions

```yaml
steps:
  - name: check-branch
    command: ./deploy.sh
    preconditions:
      - condition: "`git branch --show-current`"
        expected: "main"
```

### Regex Matching

```yaml
steps:
  - name: weekday-only
    command: ./batch-job.sh
    preconditions:
      - condition: "`date +%u`"
        expected: "re:[1-5]"  # Monday-Friday
```

### Multiple Conditions

All conditions must pass:

```yaml
steps:
  - name: conditional-deploy
    command: ./deploy.sh
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
    command: ./process.sh
    preconditions:
      - condition: "test -f /data/input.csv"
      - condition: "test -d /output"
```

## Repetition

Repeat steps while conditions are met or until a limit is reached.

### Repeat While Exit Code Matches

```yaml
steps:
  - name: wait-for-file
    command: test -f /tmp/ready.flag
    repeatPolicy:
      repeat: true
      exitCode: [1]      # Repeat while exit code is 1
      intervalSec: 10    # Wait 10 seconds between attempts
```

### Repeat Forever

```yaml
steps:
  - name: monitoring
    command: ./check-metrics.sh
    repeatPolicy:
      repeat: true
      intervalSec: 60  # Every minute
```

### Repeat with Limit

```yaml
steps:
  - name: limited-polling
    command: ./check-status.sh
    repeatPolicy:
      repeat: true
      limit: 10        # Maximum 10 executions
      intervalSec: 30  # Every 30 seconds
```

## Continue On Conditions

Control workflow behavior when steps fail or produce specific outputs.

### Continue on Failure

```yaml
steps:
  - name: optional-cleanup
    command: ./cleanup.sh
    continueOn:
      failure: true
      
  - name: main-process
    command: ./process.sh
```

### Continue on Specific Exit Codes

```yaml
steps:
  - name: check-optional
    command: ./check.sh
    continueOn:
      exitCode: [0, 1, 2]  # Continue on these codes
      
  - name: process
    command: ./process.sh
```

### Continue on Output Match

```yaml
steps:
  - name: validate
    command: ./validate.sh
    continueOn:
      output: 
        - "WARNING"
        - "SKIP"
        - "re:^\[WARN\]"        # Regex: lines starting with [WARN]
        - "re:error.*ignored"   # Regex: error...ignored pattern
      
  - name: process
    command: ./process.sh
```

### Continue on Skipped

```yaml
steps:
  - name: optional-feature
    command: ./enable-feature.sh
    preconditions:
      - condition: "${FEATURE_FLAG}"
        expected: "enabled"
    continueOn:
      skipped: true  # Continue even if precondition fails
      
  - name: main-process
    command: ./process.sh  # Runs regardless of optional feature
```

### Mark as Success

```yaml
steps:
  - name: best-effort
    command: ./optional-task.sh
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
    command: ./analyze.sh
    continueOn:
      exitCode: [0, 3, 4, 5]  # Various non-error states
      output:
        - "Analysis complete with warnings"
        - "re:Found [0-9]+ minor issues"
      markSuccess: true
      
  # Graceful degradation pattern
  - name: try-advanced-method
    command: ./process-advanced.sh
    continueOn:
      failure: true
      output: ["FALLBACK REQUIRED", "re:.*not available.*"]
      
  - name: fallback-method
    command: ./process-simple.sh
    preconditions:
      - condition: "${TRY_ADVANCED_METHOD_EXIT_CODE}"
        expected: "re:[1-9][0-9]*"
        
  # Skip pattern with continuation
  - name: optional-feature
    command: ./feature.sh
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
    command: ./daily-job.sh
```

### Skip If Already Successful

```yaml
schedule: "0 * * * *"  # Every hour
skipIfSuccessful: true  # Skip if already ran successfully today (e.g., run manually)

steps:
  - name: hourly-sync
    command: ./sync.sh
```

## Advanced Patterns

### Conditional Branching

```yaml
params:
  - ACTION: "deploy"

steps:
  - name: build
    command: ./build.sh
    preconditions:
      - condition: "${ACTION}"
        expected: "re:build|deploy"
        
  - name: test
    command: ./test.sh
    preconditions:
      - condition: "${ACTION}"
        expected: "re:test|deploy"
    
  - name: deploy
    command: ./deploy.sh
    preconditions:
      - condition: "${ACTION}"
        expected: "deploy"
```

## See Also

- [Error Handling](/writing-workflows/error-handling) - Handle failures gracefully
- [Data & Variables](/writing-workflows/data-variables) - Pass data between steps
- [Basics](/writing-workflows/basics) - Workflow fundamentals
