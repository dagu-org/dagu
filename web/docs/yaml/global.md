# Global Configuration

Creating a global configuration `~/.dagu/config.yaml` is a convenient way to organize shared settings, such as `logDir` or `env`.

Example:

```yaml
logDir: <path-to-write-log>         # base log directory to write logs
histRetentionDays: 30               # history retention days
smtp:                               # [optional] mail server configuration to send notifications
  host: <smtp server host>
  port: <stmp server port>
errorMail:                          # [optional] mail configuration for error-level
  from: <from address>
  to: <to address>
  prefix: <prefix of mail subject>
infoMail:
  from: <from address>              # [optional] mail configuration for info-level
  to: <to address>
  prefix: <prefix of mail subject>
```