# Mail Executor

The mail executor enables you to send emails from your workflows, perfect for notifications, alerts, and report distribution.

## Overview

The mail executor allows you to:

- Send email notifications on workflow events
- Attach files and logs to emails
- Use templates with variable substitution
- Configure SMTP settings globally or per-step
- Send to multiple recipients
- Support HTML and plain text formats

## Basic Usage

### Simple Email

```yaml
smtp:
  host: "smtp.gmail.com"
  port: "587"
  username: "your-email@gmail.com"
  password: "your-app-password"

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

### With Multiple Recipients

```yaml
steps:
  - name: notify-team
    executor:
      type: mail
      config:
        to: "team@example.com, manager@example.com, alerts@example.com"
        from: noreply@example.com
        subject: "Daily Report Ready"
        message: "The daily report has been generated and is ready for review."
```

## SMTP Configuration

### Global SMTP Settings

Configure SMTP settings at the DAG level:

```yaml
smtp:
  host: "smtp.office365.com"
  port: "587"
  username: "${SMTP_USER}"
  password: "${SMTP_PASS}"

steps:
  - name: send-alert
    executor:
      type: mail
      config:
        to: alerts@company.com
        from: dagu@company.com
        subject: "Alert: Process Failed"
        message: "The data import process has failed. Please investigate."
```

### Common SMTP Configurations

#### Gmail

```yaml
smtp:
  host: "smtp.gmail.com"
  port: "587"
  username: "your-email@gmail.com"
  password: "your-app-specific-password"  # Use app password, not regular password
```

#### Office 365 / Outlook

```yaml
smtp:
  host: "smtp.office365.com"
  port: "587"
  username: "your-email@company.com"
  password: "${SMTP_PASSWORD}"
```

#### SendGrid

```yaml
smtp:
  host: "smtp.sendgrid.net"
  port: "587"
  username: "apikey"  # Literal string "apikey"
  password: "${SENDGRID_API_KEY}"
```

#### AWS SES

```yaml
smtp:
  host: "email-smtp.us-east-1.amazonaws.com"
  port: "587"
  username: "${AWS_SES_SMTP_USER}"
  password: "${AWS_SES_SMTP_PASSWORD}"
```

## Email Templates

### Using Variables in Emails

```yaml
params:
  - ENVIRONMENT: production
  - DATE: "`date +%Y-%m-%d`"

steps:
  - name: deployment-notification
    executor:
      type: mail
      config:
        to: devops@company.com
        from: deployments@company.com
        subject: "Deployment to ${ENVIRONMENT} - ${DATE}"
        message: |
          Deployment Details:
          
          Environment: ${ENVIRONMENT}
          Date: ${DATE}
          Version: ${VERSION}
          Deployed by: Dagu Automation
          
          All services have been successfully deployed and are running.
```

### Dynamic Recipients

```yaml
params:
  - RECIPIENT_NAME: "John Doe"
  - RECIPIENT_EMAIL: "john.doe@example.com"

steps:
  - name: personalized-email
    executor:
      type: mail
      config:
        to: ${RECIPIENT_EMAIL}
        from: notifications@company.com
        subject: "Hello ${RECIPIENT_NAME}"
        message: |
          Dear ${RECIPIENT_NAME},
          
          This is a personalized notification just for you.
          
          Best regards,
          The Automation Team
```

## Real-World Examples

### Workflow Status Notifications

```yaml
name: data-pipeline
smtp:
  host: "smtp.gmail.com"
  port: "587"
  username: "${SMTP_USER}"
  password: "${SMTP_PASS}"

handlerOn:
  success:
    executor:
      type: mail
      config:
        to: team@company.com
        from: dagu@company.com
        subject: "✅ Data Pipeline Success - ${DAG_NAME}"
        message: |
          The data pipeline has completed successfully.
          
          Workflow: ${DAG_NAME}
          Run ID: ${DAG_RUN_ID}
          Completion Time: `date`
          
          View logs at: ${DAG_RUN_LOG_FILE}
  
  failure:
    executor:
      type: mail
      config:
        to: "team@company.com, oncall@company.com"
        from: dagu@company.com
        subject: "❌ Data Pipeline Failed - ${DAG_NAME}"
        message: |
          The data pipeline has failed and requires attention.
          
          Workflow: ${DAG_NAME}
          Run ID: ${DAG_RUN_ID}
          Failure Time: `date`
          
          Please check the logs at: ${DAG_RUN_LOG_FILE}

steps:
  - name: process-data
    command: python process_data.py
```

### Report Distribution

```yaml
name: weekly-report
schedule: "0 9 * * MON"  # Every Monday at 9 AM

smtp:
  host: "smtp.office365.com"
  port: "587"
  username: "${SMTP_USER}"
  password: "${SMTP_PASS}"

steps:
  - name: generate-report
    command: |
      cd /opt/reports
      python generate_weekly_report.py
      ls -la weekly_report_*.pdf
    output: REPORT_FILE

  - name: send-to-management
    executor:
      type: mail
      config:
        to: "ceo@company.com, cfo@company.com, cto@company.com"
        from: reports@company.com
        subject: "Weekly Business Report - `date +%Y-%m-%d`"
        message: |
          Dear Leadership Team,
          
          Please find attached the weekly business report for the week ending `date +%Y-%m-%d`.
          
          Report Highlights:
          - Revenue metrics
          - Customer acquisition
          - System performance
          - Team productivity
          
          The full report is attached as a PDF.
          
          Best regards,
          Business Intelligence Team
        attachments:
          - ${REPORT_FILE}
```

### Error Alert with Context

```yaml
name: critical-process
errorMail:
  from: alerts@company.com
  to: oncall@company.com
  prefix: "[CRITICAL]"
  attachLogs: true

steps:
  - name: validate-input
    command: |
      if [ ! -f /data/input.csv ]; then
        echo "ERROR: Input file missing"
        exit 1
      fi
      
      if [ $(wc -l < /data/input.csv) -lt 100 ]; then
        echo "ERROR: Input file has insufficient data"
        exit 1
      fi

  - name: process-critical-data
    command: python critical_process.py
    mailOnError: true

  - name: verify-output
    command: |
      if [ ! -f /data/output.csv ]; then
        echo "ERROR: Output file not generated"
        exit 1
      fi
    mailOnError: true
```

### Customer Notifications

```yaml
name: order-confirmation
params:
  - ORDER_ID: "12345"
  - CUSTOMER_EMAIL: "customer@example.com"
  - CUSTOMER_NAME: "Jane Smith"

steps:
  - name: get-order-details
    command: |
      curl -s https://api.company.com/orders/${ORDER_ID} | \
        jq -r '.total_amount'
    output: ORDER_TOTAL

  - name: send-confirmation
    executor:
      type: mail
      config:
        to: ${CUSTOMER_EMAIL}
        from: orders@company.com
        subject: "Order Confirmation #${ORDER_ID}"
        message: |
          Dear ${CUSTOMER_NAME},
          
          Thank you for your order!
          
          Order Details:
          - Order Number: ${ORDER_ID}
          - Total Amount: $${ORDER_TOTAL}
          - Order Date: `date +"%B %d, %Y"`
          
          Your order is being processed and you will receive a shipping 
          notification once it has been dispatched.
          
          If you have any questions, please don't hesitate to contact 
          our customer service team.
          
          Best regards,
          The Sales Team
```

## Advanced Features

### Conditional Email Sending

```yaml
steps:
  - name: check-threshold
    command: |
      USAGE=$(df -h /data | awk 'NR==2 {print $5}' | sed 's/%//')
      echo $USAGE
    output: DISK_USAGE

  - name: send-alert-if-high
    executor:
      type: mail
      config:
        to: sysadmin@company.com
        from: monitoring@company.com
        subject: "⚠️ High Disk Usage Alert"
        message: |
          Warning: Disk usage on /data has reached ${DISK_USAGE}%
          
          Server: ${HOSTNAME}
          Threshold: 80%
          Current Usage: ${DISK_USAGE}%
          
          Please take action to free up disk space.
    preconditions:
      - condition: "test ${DISK_USAGE} -gt 80"
```

### Email with Multiple Attachments

```yaml
steps:
  - name: prepare-files
    command: |
      # Generate multiple reports
      python generate_sales_report.py > /tmp/sales.csv
      python generate_inventory_report.py > /tmp/inventory.csv
      python generate_metrics_report.py > /tmp/metrics.csv
      
      # Create summary
      echo "Report generated on $(date)" > /tmp/summary.txt

  - name: send-reports
    executor:
      type: mail
      config:
        to: management@company.com
        from: reports@company.com
        subject: "Monthly Reports Package"
        message: |
          Please find attached the monthly reports:
          
          1. Sales Report (sales.csv)
          2. Inventory Report (inventory.csv)
          3. Metrics Report (metrics.csv)
          4. Summary (summary.txt)
        attachments:
          - /tmp/sales.csv
          - /tmp/inventory.csv
          - /tmp/metrics.csv
          - /tmp/summary.txt
```

### Dynamic Email Content

```yaml
steps:
  - name: gather-metrics
    command: |
      cat <<EOF
      {
        "total_orders": 1523,
        "revenue": 125430.50,
        "new_customers": 89,
        "avg_order_value": 82.30
      }
      EOF
    output: METRICS

  - name: send-daily-summary
    command: |
      # Parse metrics
      ORDERS=$(echo "${METRICS}" | jq -r '.total_orders')
      REVENUE=$(echo "${METRICS}" | jq -r '.revenue')
      CUSTOMERS=$(echo "${METRICS}" | jq -r '.new_customers')
      AOV=$(echo "${METRICS}" | jq -r '.avg_order_value')
      
      # Generate email body
      cat <<EOF > /tmp/email_body.txt
      Daily Business Summary - $(date +"%B %d, %Y")
      
      📊 Today's Performance:
      
      Orders: ${ORDERS}
      Revenue: \$${REVENUE}
      New Customers: ${CUSTOMERS}
      Average Order Value: \$${AOV}
      
      ${ORDERS:+✅} Orders are ${ORDERS:-❌ No orders received}
      ${REVENUE:+✅} Revenue target achieved
      
      View full dashboard at: https://dashboard.company.com
      EOF
      
      cat /tmp/email_body.txt
    output: EMAIL_BODY

  - name: send-summary
    executor:
      type: mail
      config:
        to: executives@company.com
        from: analytics@company.com
        subject: "📈 Daily Summary - Strong Performance"
        message: ${EMAIL_BODY}
```

## Error Handling

### Handling Mail Failures

```yaml
steps:
  - name: primary-notification
    executor:
      type: mail
      config:
        to: primary@company.com
        from: notifications@company.com
        subject: "Important Update"
        message: "This is a critical notification."
    continueOn:
      failure: true
    output: MAIL_RESULT

  - name: fallback-notification
    command: |
      # Use alternative notification method
      curl -X POST https://hooks.slack.com/services/YOUR/WEBHOOK \
        -H 'Content-type: application/json' \
        -d '{"text":"Failed to send email - Important Update"}'
    preconditions:
      - condition: "${MAIL_RESULT}"
        expected: ""
```

### Retry Email Sending

```yaml
steps:
  - name: send-with-retry
    executor:
      type: mail
      config:
        to: important@company.com
        from: system@company.com
        subject: "Critical System Alert"
        message: "System requires immediate attention."
    retryPolicy:
      limit: 3
      intervalSec: 60  # Wait 1 minute between retries
```

## Integration Examples

### With Monitoring Systems

```yaml
steps:
  - name: check-service-health
    command: |
      STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://api.company.com/health)
      echo $STATUS
    output: HTTP_STATUS

  - name: alert-if-down
    executor:
      type: mail
      config:
        to: "oncall@company.com, sre@company.com"
        from: monitoring@company.com
        subject: "🚨 URGENT: API Service Down"
        message: |
          The API service is not responding.
          
          Status Code: ${HTTP_STATUS}
          Checked at: `date`
          Endpoint: http://api.company.com/health
          
          Please investigate immediately.
    preconditions:
      - condition: "test ${HTTP_STATUS} -ne 200"
```

### With CI/CD Pipelines

```yaml
name: build-notification
steps:
  - name: run-tests
    command: |
      cd /project
      npm test
      echo $? > /tmp/test_result.txt

  - name: notify-build-status
    executor:
      type: mail
      config:
        to: developers@company.com
        from: ci@company.com
        subject: "Build ${BUILD_NUMBER} - ${BUILD_STATUS}"
        message: |
          Build Details:
          
          Build Number: ${BUILD_NUMBER}
          Branch: ${GIT_BRANCH}
          Commit: ${GIT_COMMIT}
          Author: ${GIT_AUTHOR}
          
          Test Results: ${TEST_RESULT}
          
          View full build log at: ${BUILD_URL}
```

## See Also

- Learn about [JQ Executor](/features/executors/jq) for JSON processing
- Check out [Notifications](/features/notifications) for more alerting options
