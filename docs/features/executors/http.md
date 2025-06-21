# HTTP Executor

Execute HTTP requests to web services and APIs.

## Basic Usage

```yaml
steps:
  - name: get-data
    executor: http
    command: GET https://api.example.com/data
```

## Configuration

| Field | Description | Example |
|-------|-------------|---------|
| `headers` | Request headers | `Authorization: Bearer token` |
| `query` | URL parameters | `page: "1"` |
| `body` | Request body | `{"name": "value"}` |
| `timeout` | Timeout in seconds | `30` |
| `silent` | Return body only | `true` |
| `skipTLSVerify` | Skip TLS certificate verification | `true` |

## Examples

### POST with JSON

```yaml
steps:
  - name: create-resource
    executor:
      type: http
      config:
        body: '{"name": "New Resource"}'
        headers:
          Content-Type: application/json
    command: POST https://api.example.com/resources
```

### Authentication

```yaml
steps:
  - name: bearer-auth
    executor:
      type: http
      config:
        headers:
          Authorization: "Bearer ${API_TOKEN}"
    command: GET https://api.example.com/protected
```

### Query Parameters

```yaml
steps:
  - name: search
    executor:
      type: http
      config:
        query:
          q: "search term"
          limit: "10"
    command: GET https://api.example.com/search
```

### Capture Response

```yaml
steps:
  - name: get-user
    executor:
      type: http
      config:
        silent: true
    command: GET https://api.example.com/user
    output: USER_DATA

  - name: process
    command: echo "${USER_DATA}" | jq '.email'
```

## Common Patterns

### API Chaining

```yaml
steps:
  - name: get-token
    executor:
      type: http
      config:
        silent: true
        body: '{"username": "api", "password": "${API_PASS}"}'
        headers:
          Content-Type: application/json
    command: POST https://api.example.com/auth
    output: AUTH

  - name: use-token
    executor:
      type: http
      config:
        headers:
          Authorization: "Bearer ${AUTH.token}"
    command: GET https://api.example.com/data
```

### Error Handling

```yaml
steps:
  - name: api-call
    executor:
      type: http
      config:
        timeout: 30
    command: GET https://api.example.com/data
    retryPolicy:
      limit: 3
      intervalSec: 5
    continueOn:
      exitCode: [1]  # Non-2xx status codes
```

### Webhook Notification

```yaml
handlerOn:
  success:
    executor:
      type: http
      config:
        body: '{"status": "completed", "dag": "${DAG_NAME}"}'
        headers:
          Content-Type: application/json
    command: POST https://hooks.example.com/workflow-complete
```

### Self-Signed Certificates

```yaml
steps:
  - name: internal-api
    executor:
      type: http
      config:
        skipTLSVerify: true  # Allow self-signed certificates
        headers:
          Authorization: "Bearer ${INTERNAL_TOKEN}"
    command: GET https://internal-api.company.local/data
```

## See Also

- [SSH Executor](/features/executors/ssh) - Remote command execution
- [Data Flow](/features/data-flow) - Working with outputs
