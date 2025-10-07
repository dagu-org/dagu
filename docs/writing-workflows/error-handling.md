# Error Handling

Build resilient workflows with retries, handlers, and notifications.

## Retry Policies

### Basic Retry

```yaml
steps:
  # Basic retry with fixed interval
  - command: curl https://api.example.com
    retryPolicy:
      limit: 3
      intervalSec: 5
      
  # Retry specific errors
  - command: make-request
    retryPolicy:
      limit: 3
      intervalSec: 30
      exitCodes: [429, 503]  # Rate limit or unavailable
```

### Exponential Backoff

Increase retry intervals exponentially to avoid overwhelming failed services:

```yaml
steps:
  # Exponential backoff with default multiplier (2.0)
  - command: curl https://api.example.com/data
    retryPolicy:
      limit: 5
      intervalSec: 2
      backoff: true        # true = 2.0 multiplier
      # Intervals: 2s, 4s, 8s, 16s, 32s
      
  # Custom backoff multiplier
  - command: echo "Checking service health"
    retryPolicy:
      limit: 4
      intervalSec: 1
      backoff: 1.5         # Custom multiplier
      # Intervals: 1s, 1.5s, 2.25s, 3.375s
      
  # Backoff with max interval cap
  - command: echo "Syncing data"
    retryPolicy:
      limit: 10
      intervalSec: 1
      backoff: 2.0
      maxIntervalSec: 30   # Cap at 30 seconds
      # Intervals: 1s, 2s, 4s, 8s, 16s, 30s, 30s, 30s...
```

**Backoff Formula**: `interval * (backoff ^ attemptCount)`

**Note**: Backoff values must be greater than 1.0 for exponential growth.

## Continue On Conditions

Control workflow execution flow when steps encounter errors or specific conditions.

### Basic Usage

```yaml
steps:
  # Continue on any failure
  - command: rm -f /tmp/cache/*
    continueOn:
      failure: true
      
  # Continue on specific exit codes
  - command: echo "Checking status"
    continueOn:
      exitCode: [0, 1, 2]  # 0=success, 1=warning, 2=info
      
  # Continue on output patterns
  - command: validate.sh
    continueOn:
      output: 
        - "WARNING"
        - "SKIP"
        - "re:^INFO:.*"      # Regex pattern
        - "re:WARN-[0-9]+"   # Another regex
      
  # Mark as success when continuing
  - command: optimize.sh
    continueOn:
      failure: true
      markSuccess: true  # Shows as successful in UI
```

### Advanced Patterns

```yaml
steps:
  # Database migration with known warnings
  - command: echo "Running migration"
    continueOn:
      output:
        - "re:WARNING:.*already exists"
        - "re:NOTICE:.*will be created"
      exitCode: [0, 1]
      
  # Service health check with fallback
  - command: curl -f https://primary.example.com/health
    continueOn:
      exitCode: [0, 22, 7]  # 22=HTTP error, 7=connection failed
      
  # Conditional cleanup
  - command: find /tmp -name "*.tmp" -mtime +7 -delete
    continueOn:
      failure: true       # Continue even if cleanup fails
      exitCode: [0, 1]   # find returns 1 if no files found
      
  # Tool with non-standard exit codes
  - command: security-scanner --strict
    continueOn:
      exitCode: [0, 4, 8]  # 0=clean, 4=warnings, 8=info
      output:
        - "re:LOW SEVERITY:"
        - "re:INFORMATIONAL:"
```

See the [Continue On Reference](/reference/continue-on) for complete documentation.

## Lifecycle Handlers

Lifecycle handlers fire after the main steps complete and let you add notifications or cleanup logic based on the final DAG status. See the [Lifecycle Handlers guide](/writing-workflows/lifecycle-handlers) for execution order, context access, and additional patterns. Quick examples:

```yaml
handlerOn:
  success:
    command: notify-success.sh
  failure:
    command: alert-oncall.sh "${DAG_NAME} failed"
  cancel:
    command: cleanup.sh
  exit:
    command: rm -rf /tmp/dag-${DAG_RUN_ID}  # Always runs

# With email
handlerOn:
  failure:
    executor:
      type: mail
      config:
        to: oncall@company.com
        from: dagu@company.com
        subject: "Failed: ${DAG_NAME}"
        message: "Check logs: ${DAG_RUN_LOG_FILE}"
```

## Email Notifications

```yaml
# Global configuration
smtp:
  host: "smtp.gmail.com"
  port: "587"
  username: "${SMTP_USER}"
  password: "${SMTP_PASS}"

mailOn:
  failure: true
  success: false

errorMail:
  from: "dagu@company.com"
  to: "oncall@company.com"
  prefix: "[ALERT]"
  attachLogs: true

# Step-level
steps:
  - command: backup.sh
    mailOnError: true
    
  # Send custom email
  - executor:
      type: mail
      config:
        to: team@company.com
        from: dagu@company.com
        subject: "Report Ready"
        message: "See attached"
        attachments:
          - /tmp/report.pdf
```

## Timeouts and Cleanup

```yaml
# DAG timeout
timeoutSec: 3600  # 1 hour

# Cleanup timeout
maxCleanUpTimeSec: 300  # 5 minutes

steps:
  # Step with graceful shutdown
  - command: server.sh
    signalOnStop: SIGTERM  # Default
    
  # Always cleanup
  - command: analyze.sh
    continueOn:
      failure: true
      
  - cleanup.sh  # Runs even if process fails
```
