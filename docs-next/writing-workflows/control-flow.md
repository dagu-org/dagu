# Control Flow

Control how your workflow executes with conditions, dependencies, and repetition.

## Dependencies

Define execution order with step dependencies.

### Basic Dependencies

```yaml
steps:
  - name: download
    command: wget https://example.com/data.zip
    
  - name: extract
    command: unzip data.zip
    depends: download
    
  - name: process
    command: python process.py
    depends: extract
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

Repeat steps until conditions are met.

### Repeat While Exit Code Matches

```yaml
steps:
  - name: wait-for-file
    command: test -f /tmp/ready.flag
    repeatPolicy:
      exitCode: [1]      # Repeat while exit code is 1
      intervalSec: 10    # Wait 10 seconds between attempts
```

### Repeat Until Output Matches

```yaml
steps:
  - name: wait-for-service
    command: curl -s http://service/health
    output: HEALTH_STATUS
    repeatPolicy:
      condition: "${HEALTH_STATUS}"
      expected: "healthy"
      intervalSec: 30
```

### Repeat Until Command Succeeds

```yaml
steps:
  - name: wait-for-database
    command: pg_isready -h localhost
    repeatPolicy:
      condition: "`echo $?`"  # Exit code of last command
      expected: "0"
      intervalSec: 5
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

### Complex Repeat Conditions

```yaml
steps:
  - name: poll-api
    command: |
      curl -s https://api.example.com/job/status | jq -r .status
    output: JOB_STATUS
    repeatPolicy:
      condition: "${JOB_STATUS}"
      expected: "re:COMPLETED|FAILED"  # Stop on these statuses
      intervalSec: 30
```

## Continue On Conditions

Control workflow behavior when steps fail.

### Continue on Failure

```yaml
steps:
  - name: optional-cleanup
    command: ./cleanup.sh
    continueOn:
      failure: true
      
  - name: main-process
    command: ./process.sh
    depends: optional-cleanup
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
    depends: check-optional
```

### Continue on Output Match

```yaml
steps:
  - name: validate
    command: ./validate.sh
    continueOn:
      output: ["WARNING", "SKIP"]
      
  - name: process
    command: ./process.sh
    depends: validate
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

## Workflow-Level Conditions

### Workflow Preconditions

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
skipIfSuccessful: true  # Skip if already ran successfully today

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
    depends: build
    
  - name: deploy
    command: ./deploy.sh
    preconditions:
      - condition: "${ACTION}"
        expected: "deploy"
    depends: test
```

### Retry with Backoff

```yaml
steps:
  - name: api-call
    command: curl https://api.example.com
    retryPolicy:
      limit: 5
      intervalSec: 30
    repeatPolicy:
      exitCode: [1]
      intervalSec: 60  # Additional wait between retries
```

### Polling with Timeout

```yaml
steps:
  - name: start-time
    command: date +%s
    output: START_TIME
    
  - name: poll-status
    command: |
      NOW=$(date +%s)
      ELAPSED=$((NOW - ${START_TIME}))
      if [ $ELAPSED -gt 3600 ]; then
        echo "Timeout after 1 hour"
        exit 1
      fi
      check-status.sh
    repeatPolicy:
      exitCode: [2]  # Status not ready
      intervalSec: 30
    depends: start-time
```

## See Also

- [Error Handling](/writing-workflows/error-handling) - Handle failures gracefully
- [Data & Variables](/writing-workflows/data-variables) - Pass data between steps
- [Basics](/writing-workflows/basics) - Workflow fundamentals
