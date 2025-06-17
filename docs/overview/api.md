# REST API

Dagu provides a comprehensive REST API for programmatic control over workflow orchestration. The API enables DAG management, execution control, monitoring, and system operations through standard HTTP endpoints.

::: tip API Reference
For the complete API documentation with all endpoints, see [REST API Reference](/reference/api).
:::

## Base Configuration

- **Base URL**: `http://localhost:8080/api/v2`
- **Content-Type**: `application/json`
- **Required Headers**: `Accept: application/json`

## Authentication

The REST API supports three authentication methods:

- **Basic Authentication**: Include `Authorization: Basic <base64(username:password)>` header
- **Bearer Token**: Include `Authorization: Bearer <token>` header  
- **No Authentication**: When auth is disabled (default for local development)

## Core Operations

### Health Monitoring

Check server health and status:

```bash
curl http://localhost:8080/api/v2/health
```

Response includes server status, version, uptime, and timestamp.

### DAG Management

#### List DAGs
```bash
# Get all DAGs
curl http://localhost:8080/api/v2/dags

# With filtering and pagination
curl "http://localhost:8080/api/v2/dags?page=1&perPage=10&name=example&tag=prod"
```

#### Get DAG Details
```bash
curl http://localhost:8080/api/v2/dags/my-dag.yaml
```

#### Create New DAG
```bash
curl -X POST http://localhost:8080/api/v2/dags \
  -H "Content-Type: application/json" \
  -d '{"name": "my-new-dag"}'
```

### DAG Execution Control

#### Start DAG
```bash
curl -X POST http://localhost:8080/api/v2/dags/my-dag.yaml/start \
  -H "Content-Type: application/json" \
  -d '{
    "params": "{\"env\": \"production\"}",
    "dagRunId": "custom-run-id"
  }'
```

#### Enqueue DAG
```bash
curl -X POST http://localhost:8080/api/v2/dags/my-dag.yaml/enqueue \
  -H "Content-Type: application/json" \
  -d '{
    "params": "{\"env\": \"production\"}"
  }'
```

#### Suspend/Resume DAG
```bash
# Suspend
curl -X POST http://localhost:8080/api/v2/dags/my-dag.yaml/suspend \
  -H "Content-Type: application/json" \
  -d '{"suspend": true}'

# Resume
curl -X POST http://localhost:8080/api/v2/dags/my-dag.yaml/suspend \
  -H "Content-Type: application/json" \
  -d '{"suspend": false}'
```

### DAG Run Management

#### List DAG Runs
```bash
# Get all DAG runs
curl http://localhost:8080/api/v2/dag-runs

# Filter by name and status
curl "http://localhost:8080/api/v2/dag-runs?name=my-dag&status=2"
```

#### Stop DAG Run
```bash
curl -X POST http://localhost:8080/api/v2/dag-runs/my-dag/20240101_120000/stop
```

#### Retry DAG Run
```bash
curl -X POST http://localhost:8080/api/v2/dag-runs/my-dag/20240101_120000/retry \
  -H "Content-Type: application/json" \
  -d '{"dagRunId": "new-run-id"}'
```

### Step Management

#### Update Step Status
```bash
# Mark step as successful
curl -X PATCH http://localhost:8080/api/v2/dag-runs/my-dag/20240101_120000/steps/step1/status \
  -H "Content-Type: application/json" \
  -d '{"status": 4}'  # 4 = Success

# Mark step as failed
curl -X PATCH http://localhost:8080/api/v2/dag-runs/my-dag/20240101_120000/steps/step1/status \
  -H "Content-Type: application/json" \
  -d '{"status": 2}'  # 2 = Failed
```

### Search Operations

Search across DAG definitions:

```bash
curl "http://localhost:8080/api/v2/dags/search?q=database"
```

## Monitoring and Metrics

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
- `bad_request`: Invalid request parameters
- `not_found`: Resource doesn't exist
- `internal_error`: Server-side error
- `unauthorized`: Authentication failed
- `forbidden`: Insufficient permissions
- `already_running`: DAG is already running
- `not_running`: DAG is not running

## Response Formats

### DAG List Response
```json
{
  "dags": [
    {
      "fileName": "example.yaml",
      "dag": {
        "name": "example_dag",
        "schedule": [{"expression": "0 * * * *"}],
        "description": "Example DAG",
        "tags": ["example", "demo"]
      },
      "latestDAGRun": {
        "dagRunId": "20240101_120000",
        "name": "example_dag",
        "status": 1,
        "statusLabel": "running",
        "startedAt": "2024-01-01T12:00:00Z"
      },
      "suspended": false
    }
  ],
  "errors": [],
  "pagination": {
    "totalRecords": 45,
    "currentPage": 1,
    "totalPages": 5,
    "nextPage": 2,
    "prevPage": null
  }
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
curl -X POST http://localhost:8080/api/v2/dags/deploy-app.yaml/start \
  -H "Content-Type: application/json" \
  -d '{
    "params": "{\"version\": \"v1.2.3\", \"environment\": \"production\"}",
    "dagRunId": "deploy-v1.2.3"
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
# Create and start new DAG
curl -X POST http://localhost:8080/api/v2/dags \
  -H "Content-Type: application/json" \
  -d '{"name": "automated-dag"}'

curl -X POST http://localhost:8080/api/v2/dags/automated-dag.yaml/start \
  -H "Content-Type: application/json" \
  -d '{"params": "{}"}'
```

The REST API provides a powerful foundation for integrating Dagu into your existing toolchain and building automated workflow management solutions.