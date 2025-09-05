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
  - executor:
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
  - executor:
      type: mail
      config:
        to:
          - team@example.com
          - manager@example.com
          - stakeholders@example.com
        from: noreply@example.com
        subject: "Daily Report Ready"
        message: "The daily report has been generated."
        
  - executor:
      type: mail
      config:
        to: admin@example.com  # Single recipient still works
        from: system@example.com
        subject: "System Update"
        message: "System maintenance completed."
```

### With Variables

```yaml
params:
  - ENVIRONMENT: production

steps:
  - executor:
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
  - echo "Run your main tasks here"
```

### Error Alerts

```yaml
errorMail:
  from: alerts@company.com
  to: oncall@company.com
  prefix: "[CRITICAL]"
  attachLogs: true

steps:
  - command: echo "Run some critical task"
    mailOnError: true
```

### With Attachments

```yaml
steps:
  - echo "Generating report..." > report.txt

  - executor:
      type: mail
      config:
        to: management@company.com
        from: reports@company.com
        subject: "Weekly Report"
        message: "Please find the weekly report attached."
        attachments:
          - report.txt
```

### With Retry

```yaml
steps:
  - executor:
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
