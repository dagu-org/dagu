# Mail Executor

Send emails from your workflows for notifications, alerts, and reports.

## Basic Usage

```yaml
smtp:
  host: "smtp.gmail.com"
  port: "587"
  username: "${SMTP_USER}"
  password: "${SMTP_PASS}"

steps:
  - name: send-notification
    executor:
      type: mail
      config:
        to: recipient@example.com
        from: sender@example.com
        subject: "Workflow Completed"
        message: "The data processing workflow has completed successfully."
```

## SMTP Configuration

### Common Providers

```yaml
# Gmail
smtp:
  host: "smtp.gmail.com"
  port: "587"
  username: "your-email@gmail.com"
  password: "app-specific-password"  # Not regular password

# Office 365
smtp:
  host: "smtp.office365.com"
  port: "587"
  username: "${SMTP_USER}"
  password: "${SMTP_PASS}"

# AWS SES
smtp:
  host: "email-smtp.us-east-1.amazonaws.com"
  port: "587"
  username: "${AWS_SES_SMTP_USER}"
  password: "${AWS_SES_SMTP_PASSWORD}"
```

## Examples

### Multiple Recipients

```yaml
steps:
  - name: notify-team
    executor:
      type: mail
      config:
        to:
          - team@example.com
          - manager@example.com
          - stakeholders@example.com
        from: noreply@example.com
        cc:
          - vp@example.com
          - svp@example.com
        bcc:
          - evp@example.com
        subject: "Daily Report Ready"
        message: "The daily report has been generated."
        
  - name: single-recipient
    executor:
      type: mail
      config:
        to: admin@example.com  # Single recipient still works
        from: system@example.com
        cc: teamlead@example.com
        bcc: manager@example.com 
        subject: "System Update"
        message: "System maintenance completed."
```

### With Variables

```yaml
params:
  - ENVIRONMENT: production

steps:
  - name: deployment-notification
    executor:
      type: mail
      config:
        to: devops@company.com
        from: deploy@company.com
        subject: "Deployed to ${ENVIRONMENT}"
        message: |
          Deployment completed:
          - Environment: ${ENVIRONMENT}
          - Version: ${VERSION}
          - Time: `date`
```

### Success/Failure Notifications

```yaml
handlerOn:
  success:
    executor:
      type: mail
      config:
        to: team@company.com
        from: dagu@company.com
        subject: "✅ Pipeline Success - ${DAG_NAME}"
        message: |
          Pipeline completed successfully.
          Run ID: ${DAG_RUN_ID}
          Logs: ${DAG_RUN_LOG_FILE}
  
  failure:
    executor:
      type: mail
      config:
        to: oncall@company.com
        from: alerts@company.com
        subject: "❌ Pipeline Failed - ${DAG_NAME}"
        message: |
          Pipeline failed.
          Run ID: ${DAG_RUN_ID}
          Check logs: ${DAG_RUN_LOG_FILE}

steps:
  - name: process-data
    command: python process_data.py
```

### Error Alerts

```yaml
errorMail:
  from: alerts@company.com
  to: oncall@company.com
  prefix: "[CRITICAL]"
  attachLogs: true

steps:
  - name: critical-process
    command: python critical_process.py
    mailOnError: true
```

### With Attachments

```yaml
steps:
  - name: generate-report
    command: python generate_report.py > report.pdf
    output: REPORT_FILE

  - name: send-report
    executor:
      type: mail
      config:
        to: management@company.com
        from: reports@company.com
        subject: "Weekly Report"
        message: "Please find the weekly report attached."
        attachments:
          - ${REPORT_FILE}
```

## Common Patterns

### Conditional Alerts

```yaml
steps:
  - name: check-disk
    command: df -h /data | awk 'NR==2 {print $5}' | sed 's/%//'
    output: DISK_USAGE

  - name: alert-if-high
    executor:
      type: mail
      config:
        to: sysadmin@company.com
        from: monitoring@company.com
        subject: "⚠️ High Disk Usage"
        message: "Disk usage: ${DISK_USAGE}%"
    preconditions:
      - condition: "test ${DISK_USAGE} -gt 80"
```

### With Retry

```yaml
steps:
  - name: send-critical-alert
    executor:
      type: mail
      config:
        to: oncall@company.com
        from: alerts@company.com
        subject: "Critical Alert"
        message: "Immediate action required."
    retryPolicy:
      limit: 3
      intervalSec: 60
```

## See Also

- [Notifications](/features/notifications) - Complete notification guide
- [Error Handling](/writing-workflows/error-handling) - Handle failures
