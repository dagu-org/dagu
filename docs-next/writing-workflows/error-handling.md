# Error Handling and Recovery

Dagu provides comprehensive error handling capabilities to ensure your workflows are resilient and can recover from failures gracefully. This guide covers all aspects of error handling including retry policies, continue conditions, lifecycle handlers, and email notifications.

## Table of Contents

- [Retry Policies](#retry-policies)
- [Continue On Conditions](#continue-on-conditions)
- [Lifecycle Handlers](#lifecycle-handlers)
- [Email Notifications](#email-notifications)
- [Cleanup and Timeouts](#cleanup-and-timeouts)
- [Signal Handling](#signal-handling)
- [Error Handling Patterns](#error-handling-patterns)

## Retry Policies

Automatically retry failed steps with configurable limits and intervals.

### Basic Retry Configuration

```yaml
steps:
  - name: flaky-api-call
    command: curl https://api.example.com/data
    retryPolicy:
      limit: 3          # Retry up to 3 times
      intervalSec: 5    # Wait 5 seconds between retries
```

### Retry with Specific Exit Codes

Only retry when specific exit codes are returned:

```yaml
steps:
  - name: api-call
    command: make-api-request
    retryPolicy:
      limit: 3
      intervalSec: 30
      exitCodes: [429, 503]  # Only retry on rate limit (429) or service unavailable (503)
```

If `exitCodes` is not specified, any non-zero exit code will trigger a retry. When specified, only the listed exit codes will trigger retries.

### Advanced Retry Example

```yaml
steps:
  - name: database-operation
    command: psql -c "INSERT INTO table VALUES (...)"
    retryPolicy:
      limit: 5
      intervalSec: 10
      exitCodes: [1, 2]  # Retry only on connection errors
    continueOn:
      exitCode: [3]      # Continue if data already exists
```

## Continue On Conditions

Control workflow execution flow when steps fail or meet certain conditions.

### Continue on Failure

```yaml
steps:
  - name: optional-cleanup
    command: rm -f /tmp/cache/*
    continueOn:
      failure: true  # Continue even if cleanup fails
```

### Continue on Skipped Steps

```yaml
steps:
  - name: conditional-task
    command: process-data.sh
    preconditions:
      - condition: "`date +%u`"
        expected: "1"  # Only run on Mondays
    continueOn:
      skipped: true    # Continue if precondition not met
```

### Continue Based on Exit Codes

```yaml
steps:
  - name: check-status
    command: check-system-status.sh
    continueOn:
      exitCode: [0, 1, 2]  # Continue on success (0) or known warnings (1, 2)
```

### Continue Based on Output

```yaml
steps:
  - name: data-processing
    command: process.py
    continueOn:
      output: "warning"  # Continue if output contains "warning"
```

### Continue with Regular Expressions

```yaml
steps:
  - name: validation
    command: validate-data.sh
    continueOn:
      output: "re:WARN.*|INFO.*"  # Continue if output matches warning or info patterns
```

### Multiple Output Conditions

```yaml
steps:
  - name: complex-task
    command: complex-operation.sh
    continueOn:
      output:
        - "partially complete"
        - "re:processed [0-9]+ of [0-9]+ items"
        - "skipping optional step"
```

### Mark as Success

Override the step status to success even if it fails:

```yaml
steps:
  - name: best-effort-task
    command: optional-optimization.sh
    continueOn:
      failure: true
      markSuccess: true  # Mark as successful even if it fails
```

## Lifecycle Handlers

React to DAG state changes with custom handlers.

### DAG-Level Handlers

```yaml
handlerOn:
  success:
    command: |
      curl -X POST https://api.example.com/notify \
        -d "status=success&dag=${DAG_NAME}"
  
  failure:
    command: |
      echo "DAG failed at $(date)" >> /var/log/failures.log
      ./alert-oncall.sh "${DAG_NAME} failed"
  
  cancel:
    command: cleanup-resources.sh
  
  exit:
    command: |
      # Always runs, regardless of success/failure
      rm -rf /tmp/dag-${DAG_RUN_ID}
      echo "Finished ${DAG_NAME} with status $?"

steps:
  - name: main-task
    command: process-data.sh
```

### Handler with Email Notification

```yaml
handlerOn:
  failure:
    executor:
      type: mail
      config:
        to: oncall@company.com
        from: dagu@company.com
        subject: "DAG Failed: ${DAG_NAME}"
        message: |
          The DAG ${DAG_NAME} failed at ${DAG_RUN_ID}.
          
          Please check the logs at: ${DAG_RUN_LOG_FILE}
```

### Handler Execution Order

1. Step execution completes
2. Step-level handlers run (if any)
3. DAG determines final status
4. DAG-level handlers run in this order:
   - `success` or `failure` or `cancel` (based on status)
   - `exit` (always runs last)

## Email Notifications

Configure email alerts for workflow events.

### Basic Email Configuration

```yaml
# SMTP Configuration
smtp:
  host: "smtp.gmail.com"
  port: "587"
  username: "alerts@company.com"
  password: "${SMTP_PASSWORD}"

# Email notification triggers
mailOn:
  failure: true
  success: false

# Error email settings
errorMail:
  from: "dagu@company.com"
  to: "oncall@company.com"
  prefix: "[CRITICAL]"
  attachLogs: true

# Success email settings  
infoMail:
  from: "dagu@company.com"
  to: "team@company.com"
  prefix: "[SUCCESS]"
  attachLogs: false
```

### Step-Level Email Notifications

```yaml
smtp:
  host: "smtp.company.com"
  port: "587"
  username: "notifications"
  password: "${SMTP_PASS}"

steps:
  - name: critical-operation
    command: backup-database.sh
    mailOn:
      failure: true
      success: true
    
  - name: optional-task
    command: optimize-indexes.sh
    # No email notifications for this step
```

### Email with Attachments

```yaml
steps:
  - name: generate-report
    command: create-report.py
    stdout: /tmp/report.txt
    
  - name: email-report
    executor:
      type: mail
      config:
        to: "stakeholders@company.com"
        from: "reports@company.com"
        subject: "Daily Report - $(date +%Y-%m-%d)"
        message: |
          Please find attached the daily report.
          
          Generated at: $(date)
        attachments:
          - /tmp/report.txt
          - /tmp/summary.csv
```

### Dynamic Email Recipients

```yaml
params:
  - ONCALL_EMAIL: "default@company.com"

steps:
  - name: get-oncall
    command: ./get-current-oncall.sh
    output: CURRENT_ONCALL
    
  - name: critical-task
    command: critical-operation.sh
    continueOn:
      failure: true
      
  - name: notify-on-failure
    command: echo "Task failed"
    depends: critical-task
    preconditions:
      - condition: "${critical-task.exit_code}"
        expected: "re:[1-9][0-9]*"
    executor:
      type: mail
      config:
        to: "${CURRENT_ONCALL}"
        from: "alerts@company.com"
        subject: "Critical Task Failed"
        message: "Please investigate immediately"
```

## Cleanup and Timeouts

### Maximum Cleanup Time

Control how long Dagu waits for cleanup operations:

```yaml
MaxCleanUpTimeSec: 300  # Wait up to 5 minutes for cleanup

steps:
  - name: long-running-task
    command: process-large-dataset.sh
    signalOnStop: SIGTERM  # Send SIGTERM on stop
```

### DAG Timeout

Set a global timeout for the entire DAG:

```yaml
timeoutSec: 3600  # 1 hour timeout for entire DAG

steps:
  - name: data-processing
    command: process.sh
```

### Step-Level Cleanup

```yaml
steps:
  - name: create-temp-resources
    command: setup-environment.sh
    
  - name: main-process
    command: run-analysis.sh
    continueOn:
      failure: true
      
  - name: cleanup
    command: cleanup-environment.sh
    depends:
      - main-process
    # This step always runs due to continueOn in previous step
```

## Signal Handling

### Custom Stop Signals

```yaml
steps:
  - name: graceful-shutdown
    command: long-running-service.sh
    signalOnStop: SIGTERM  # Default is SIGTERM
    
  - name: immediate-stop
    command: batch-processor.sh
    signalOnStop: SIGKILL
    
  - name: custom-signal
    command: |
      trap 'echo "Caught SIGUSR1"; cleanup' USR1
      # Long running process
      while true; do
        process_item
        sleep 1
      done
    signalOnStop: SIGUSR1
```

## Error Handling Patterns

### Pattern 1: Retry with Exponential Backoff

While Dagu doesn't have built-in exponential backoff, you can implement it:

```yaml
steps:
  - name: try-1
    command: api-call.sh
    continueOn:
      failure: true
      
  - name: try-2
    command: |
      sleep 2  # 2 second delay
      api-call.sh
    depends: try-1
    preconditions:
      - condition: "${try-1.exit_code}"
        expected: "re:[1-9][0-9]*"
    continueOn:
      failure: true
      
  - name: try-3
    command: |
      sleep 4  # 4 second delay
      api-call.sh
    depends: try-2
    preconditions:
      - condition: "${try-2.exit_code}"
        expected: "re:[1-9][0-9]*"
```

### Pattern 2: Circuit Breaker

```yaml
env:
  - FAILURE_THRESHOLD: 3
  - FAILURE_COUNT_FILE: /tmp/circuit-breaker-${DAG_NAME}

steps:
  - name: check-circuit
    command: |
      if [ -f "${FAILURE_COUNT_FILE}" ]; then
        count=$(cat "${FAILURE_COUNT_FILE}")
        if [ "$count" -ge "${FAILURE_THRESHOLD}" ]; then
          echo "Circuit breaker OPEN - too many failures"
          exit 99
        fi
      fi
    continueOn:
      exitCode: [0, 99]
      
  - name: main-operation
    command: risky-operation.sh
    depends: check-circuit
    preconditions:
      - condition: "${check-circuit.exit_code}"
        expected: "0"
    continueOn:
      failure: true
      
  - name: update-circuit
    command: |
      if [ "${main-operation.exit_code}" != "0" ]; then
        # Increment failure count
        count=$(cat "${FAILURE_COUNT_FILE}" 2>/dev/null || echo 0)
        echo $((count + 1)) > "${FAILURE_COUNT_FILE}"
      else
        # Reset on success
        rm -f "${FAILURE_COUNT_FILE}"
      fi
    depends: main-operation
```

### Pattern 3: Graceful Degradation

```yaml
steps:
  - name: try-primary
    command: connect-primary-db.sh
    output: DB_RESULT
    continueOn:
      failure: true
      
  - name: try-secondary
    command: connect-secondary-db.sh
    depends: try-primary
    preconditions:
      - condition: "${try-primary.exit_code}"
        expected: "re:[1-9][0-9]*"
    output: DB_RESULT
    continueOn:
      failure: true
      
  - name: use-cache
    command: read-from-cache.sh
    depends: try-secondary
    preconditions:
      - condition: "${try-secondary.exit_code}"
        expected: "re:[1-9][0-9]*"
    output: DB_RESULT
```

### Pattern 4: Compensating Transactions

```yaml
steps:
  - name: create-resources
    command: terraform apply -auto-approve
    continueOn:
      failure: true
      
  - name: run-tests
    command: integration-tests.sh
    depends: create-resources
    continueOn:
      failure: true
      
  - name: rollback-on-failure
    command: terraform destroy -auto-approve
    depends: run-tests
    preconditions:
      - condition: "${run-tests.exit_code}"
        expected: "re:[1-9][0-9]*"
```

### Pattern 5: Health Check with Retry

```yaml
steps:
  - name: deploy-service
    command: kubectl apply -f service.yaml
    
  - name: wait-for-healthy
    command: |
      kubectl wait --for=condition=ready pod -l app=myapp --timeout=300s
    depends: deploy-service
    retryPolicy:
      limit: 5
      intervalSec: 30
      exitCodes: [1]  # Retry if pods not ready
```

## Best Practices

1. **Use specific exit codes** for different error types in your scripts
2. **Combine retry policies with continue conditions** for maximum flexibility
3. **Always include cleanup in exit handlers** to prevent resource leaks
4. **Use meaningful email prefixes** to help with filtering and alerting
5. **Test error paths** by intentionally failing steps during development
6. **Document expected failures** in step descriptions
7. **Use markSuccess judiciously** - only for truly optional operations
8. **Set reasonable cleanup timeouts** based on your workload characteristics

## Complete Example

Here's a comprehensive example combining multiple error handling features:

```yaml
name: robust-data-pipeline
schedule: "0 2 * * *"
maxCleanUpTimeSec: 600
timeoutSec: 7200

smtp:
  host: "${SMTP_HOST}"
  port: "587"
  username: "${SMTP_USER}"
  password: "${SMTP_PASS}"

mailOn:
  failure: true
  success: false

errorMail:
  from: "dagu@company.com"
  to: "data-team@company.com"
  prefix: "[DATA-PIPELINE-FAILURE]"
  attachLogs: true

handlerOn:
  failure:
    command: |
      # Log failure to monitoring system
      curl -X POST https://monitoring.company.com/api/v1/events \
        -H "Content-Type: application/json" \
        -d '{
          "severity": "error",
          "service": "data-pipeline",
          "dag": "${DAG_NAME}",
          "run_id": "${DAG_RUN_ID}",
          "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
        }'
  
  exit:
    command: |
      # Cleanup temporary files
      rm -rf /tmp/pipeline-${DAG_RUN_ID}
      # Release distributed lock if held
      ./release-lock.sh pipeline-lock

steps:
  - name: acquire-lock
    command: ./acquire-lock.sh pipeline-lock
    retryPolicy:
      limit: 5
      intervalSec: 60
      exitCodes: [1]  # Retry if lock busy
    
  - name: extract-data
    command: extract-from-source.py
    output: EXTRACT_RESULT
    retryPolicy:
      limit: 3
      intervalSec: 30
      exitCodes: [111, 112]  # Network errors
    signalOnStop: SIGTERM
    
  - name: validate-data
    command: validate.py ${EXTRACT_RESULT}
    depends: extract-data
    continueOn:
      exitCode: [2]  # Continue on validation warnings
      markSuccess: true
    
  - name: transform-data
    command: transform.py
    depends: validate-data
    retryPolicy:
      limit: 2
      intervalSec: 10
    
  - name: load-primary
    command: load-to-primary-db.py
    depends: transform-data
    continueOn:
      failure: true
    
  - name: load-backup
    command: load-to-backup-db.py
    depends: transform-data
    preconditions:
      - condition: "${load-primary.exit_code}"
        expected: "re:[1-9][0-9]*"
    
  - name: verify-load
    command: verify-data-loaded.py
    depends:
      - load-primary
      - load-backup
    mailOn:
      failure: true
```

This example demonstrates:
- Distributed locking with retries
- Network-aware retry policies
- Validation with warning tolerance
- Primary/backup loading pattern
- Comprehensive cleanup in handlers
- Step and DAG-level email notifications
- Proper signal handling for graceful shutdown