# REST API

Dagu provides comprehensive REST APIs for programmatic control over workflow orchestration. The APIs enable DAG management, execution control, monitoring, and system operations through standard HTTP endpoints.

## API Versions

Dagu offers two API versions:

- **v1 API**: Core DAG operations and management
- **v2 API**: Enhanced monitoring and metrics capabilities

## Base Configuration

### v1 API
- **Base URL**: `http://localhost:8080/api/v1`
- **Content-Type**: `application/json`
- **Required Headers**: `Accept: application/json`

### v2 API
- **Base URL**: `http://localhost:8080/api/v2`
- **Content-Type**: `application/json` (except metrics endpoint)

## Authentication

Currently, the REST API does not require authentication by default. However, when authentication is enabled in the server configuration, you'll need to provide appropriate credentials:

- **Basic Authentication**: Username and password
- **Token Authentication**: API token in headers

## Core Operations

### Health Monitoring

Check server health and status:

```bash
curl http://localhost:8080/api/v1/health
```

Response includes server status, version, uptime, and timestamp.

### DAG Management

#### List DAGs
```bash
# Get all DAGs
curl http://localhost:8080/api/v1/dags

# With filtering and pagination
curl "http://localhost:8080/api/v1/dags?page=1&limit=10&searchName=example"
```

#### Get DAG Details
```bash
curl http://localhost:8080/api/v1/dags/my-dag
```

#### Create New DAG
```bash
curl -X POST http://localhost:8080/api/v1/dags \
  -H "Content-Type: application/json" \
  -d '{
    "action": "create",
    "value": "name: my-new-dag\nsteps:\n  - name: hello\n    command: echo Hello"
  }'
```

### DAG Execution Control

#### Start DAG
```bash
curl -X POST http://localhost:8080/api/v1/dags/my-dag \
  -H "Content-Type: application/json" \
  -d '{
    "action": "start",
    "params": "{\"env\": \"production\"}"
  }'
```

#### Stop DAG
```bash
curl -X POST http://localhost:8080/api/v1/dags/my-dag \
  -H "Content-Type: application/json" \
  -d '{"action": "stop"}'
```

#### Retry Failed DAG
```bash
curl -X POST http://localhost:8080/api/v1/dags/my-dag \
  -H "Content-Type: application/json" \
  -d '{
    "action": "retry",
    "requestId": "req_123"
  }'
```

#### Suspend/Resume DAG
```bash
# Suspend
curl -X POST http://localhost:8080/api/v1/dags/my-dag \
  -H "Content-Type: application/json" \
  -d '{
    "action": "suspend",
    "value": "true"
  }'

# Resume
curl -X POST http://localhost:8080/api/v1/dags/my-dag \
  -H "Content-Type: application/json" \
  -d '{
    "action": "suspend",
    "value": "false"
  }'
```

### Step Management

#### Mark Step as Success
```bash
curl -X POST http://localhost:8080/api/v1/dags/my-dag \
  -H "Content-Type: application/json" \
  -d '{
    "action": "mark-success",
    "requestId": "req_123",
    "step": "step1"
  }'
```

#### Mark Step as Failed
```bash
curl -X POST http://localhost:8080/api/v1/dags/my-dag \
  -H "Content-Type: application/json" \
  -d '{
    "action": "mark-failed",
    "requestId": "req_123",
    "step": "step1"
  }'
```

### Search Operations

Search across DAG definitions:

```bash
curl "http://localhost:8080/api/v1/search?q=database"
```

## Monitoring and Metrics (v2 API)

### Prometheus Metrics

Dagu exposes Prometheus-compatible metrics for comprehensive monitoring:

```bash
curl http://localhost:8080/api/v2/metrics
```

Available metrics include:

- **System Metrics**: Build info, uptime, scheduler status
- **Execution Metrics**: Running DAGs, queued runs, execution totals by status
- **Performance Metrics**: Success rates, queue lengths

### Prometheus Integration

Configure Prometheus to scrape Dagu metrics:

```yaml
scrape_configs:
  - job_name: 'dagu'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/api/v2/metrics'
    scrape_interval: 15s
```

### Grafana Dashboards

Create monitoring dashboards with queries like:

```promql
# DAG execution success rate
rate(dagu_dag_runs_total{status="success"}[5m]) / rate(dagu_dag_runs_total[5m])

# Current queue length
dagu_dag_runs_queued_total

# Scheduler uptime percentage
avg_over_time(dagu_scheduler_running[5m]) * 100
```

## Error Handling

All API endpoints return structured error responses:

```json
{
  "code": "error_code",
  "message": "Human readable error message",
  "details": {
    "additional": "error details"
  }
}
```

Common error codes:
- `validation_error`: Invalid request parameters
- `not_found`: Resource doesn't exist
- `internal_error`: Server-side error
- `unauthorized`: Authentication failed

## Response Formats

### DAG List Response
```json
{
  "DAGs": [
    {
      "File": "example.yaml",
      "Dir": "/dags",
      "DAG": {
        "Name": "example_dag",
        "Schedule": [{"Expression": "0 * * * *"}],
        "Description": "Example DAG",
        "Tags": ["example", "demo"]
      },
      "Status": {
        "RequestId": "req-123",
        "Name": "example_dag",
        "Status": 1,
        "StatusText": "running",
        "StartedAt": "2024-02-11T10:00:00Z"
      },
      "Suspended": false
    }
  ],
  "PageCount": 1
}
```

### Health Check Response
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime": 3600,
  "timestamp": "2024-02-11T12:00:00Z"
}
```

## Use Cases

### CI/CD Integration
Use the API to trigger DAGs from CI/CD pipelines:

```bash
# Trigger deployment DAG with parameters
curl -X POST http://localhost:8080/api/v1/dags/deploy-app \
  -H "Content-Type: application/json" \
  -d '{
    "action": "start",
    "params": "{\"version\": \"v1.2.3\", \"environment\": \"production\"}"
  }'
```

### Monitoring Integration
Integrate with monitoring systems to track workflow health:

```bash
# Check for failed DAGs
curl -s http://localhost:8080/api/v2/metrics | grep 'dagu_dag_runs_total{status="error"}'
```

### Automation Scripts
Build automation scripts for DAG lifecycle management:

```bash
#!/bin/bash
# Deploy and start new DAG
curl -X POST http://localhost:8080/api/v1/dags \
  -d '{"action": "create", "value": "'$(cat new-dag.yaml)'"}'

curl -X POST http://localhost:8080/api/v1/dags/new-dag \
  -d '{"action": "start"}'
```

The REST API provides a powerful foundation for integrating Dagu into your existing toolchain and building automated workflow management solutions.