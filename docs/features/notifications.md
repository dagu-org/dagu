# Email Notifications

Email notifications can be sent when a DAG run ends in a canonical `failed` or `succeeded` state. Configure the `smtp` field and related fields in the DAG specs to enable them. You can use any email delivery service (e.g., Sendgrid, Mailgun, etc.).

## Configuration

```yaml
# Email notification settings
mailOn:
  failure: true
  success: true

# SMTP server settings
smtp:
  host: "smtp.foo.bar"
  port: "587"
  username: "<username>"
  password: "<password>"

# Error mail configuration
errorMail:
  from: "foo@bar.com"
  to: "foo@bar.com"
  prefix: "[Error]"
  attachLogs: true

# Info mail configuration
infoMail:
  from: "foo@bar.com"
  to: "foo@bar.com"
  prefix: "[Info]"
  attachLogs: true
```

## Global Configuration

If you want to use the same settings for all DAGs, set them to the base configuration file at `~/.config/dagu/base.yaml`.

## Notification Types

### Success Notifications

Set `mailOn.success: true` to receive notifications when a DAG completes successfully. These notifications use the `infoMail` configuration.

### Failure Notifications

Set `mailOn.failure: true` to receive notifications when a DAG fails. These notifications use the `errorMail` configuration.

## SMTP Configuration

The `smtp` section configures the email server settings:

- `host`: SMTP server hostname
- `port`: SMTP server port (commonly 587 for TLS, 465 for SSL, 25 for plain)
- `username`: SMTP authentication username
- `password`: SMTP authentication password

## Email Configuration

### Error Mail Settings

The `errorMail` section configures notifications for failed DAG runs:

- `from`: Sender email address
- `to`: Recipient email address
- `prefix`: Subject line prefix (e.g., "[Error]")
- `attachLogs`: Whether to attach log files to the email

### Info Mail Settings

The `infoMail` section configures notifications for successful DAG runs:

- `from`: Sender email address
- `to`: Recipient email address  
- `prefix`: Subject line prefix (e.g., "[Info]")
- `attachLogs`: Whether to attach log files to the email

## Email Service Providers

Dagu works with any SMTP-compatible email service, including:

- **Gmail**: Use `smtp.gmail.com:587` with app-specific passwords
- **Sendgrid**: Use `smtp.sendgrid.net:587` with API key authentication
- **Mailgun**: Use your Mailgun SMTP credentials
- **Office 365**: Use `smtp.office365.com:587`
- **Custom SMTP servers**: Any RFC-compliant SMTP server
