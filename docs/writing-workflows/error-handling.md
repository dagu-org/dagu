# Error Handling

Build resilient workflows with retries, handlers, and notifications.

## Retry Policies

```yaml
steps:
  # Basic retry
  - name: flaky-api
    command: curl https://api.example.com
    retryPolicy:
      limit: 3
      intervalSec: 5
      
  # Retry specific errors
  - name: api-call
    command: make-request
    retryPolicy:
      limit: 3
      intervalSec: 30
      exitCodes: [429, 503]  # Rate limit or unavailable
```

## Continue On Conditions

```yaml
steps:
  # Continue on failure
  - name: optional
    command: rm -f /tmp/cache/*
    continueOn:
      failure: true
      
  # Continue on specific exit codes
  - name: check
    command: check-status.sh
    continueOn:
      exitCode: [0, 1, 2]  # Success or warnings
      
  # Continue on output patterns
  - name: validate
    command: validate.sh
    continueOn:
      output: "re:WARN.*|INFO.*"
      
  # Mark as success
  - name: best-effort
    command: optimize.sh
    continueOn:
      failure: true
      markSuccess: true
```

## Lifecycle Handlers

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

Execution order: step → status handlers → exit handler

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
  - name: critical
    command: backup.sh
    mailOnError: true
    
  # Send custom email
  - name: notify
    executor:
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
  - name: service
    command: server.sh
    signalOnStop: SIGTERM  # Default
    
  # Always cleanup
  - name: process
    command: analyze.sh
    continueOn:
      failure: true
      
  - name: cleanup
    command: cleanup.sh  # Runs even if process fails
```
