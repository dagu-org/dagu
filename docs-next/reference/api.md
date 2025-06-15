# REST API Reference

This document provides a complete reference for Dagu's REST API endpoints, including detailed parameters, responses, and examples.

## Base Configuration

### API v1
- **Base URL**: `http://localhost:8080/api/v1`
- **Content-Type**: `application/json`
- **Required Headers**: `Accept: application/json`

### API v2
- **Base URL**: `http://localhost:8080/api/v2`
- **Content-Type**: `application/json` (except metrics endpoint)

### Authentication

Currently, the API does not require authentication by default. When authentication is enabled:

- **Basic Auth**: Include `Authorization: Basic <base64(username:password)>` header
- **Token Auth**: Include `Authorization: Bearer <token>` header

## API v1 Endpoints

### System Operations

#### Health Check

**Endpoint**: `GET /health`

Checks the health status of the Dagu server and its dependencies.

**Parameters**: None

**Success Response (200)**:
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime": 3600,
  "timestamp": "2024-02-11T12:00:00Z"
}
```

**Response Fields**:
- `status`: Server health status ("healthy" or "unhealthy")
- `version`: Current server version
- `uptime`: Server uptime in seconds
- `timestamp`: Current server time in ISO 8601 format

**Error Response (503)**:
```json
{
  "status": "unhealthy",
  "version": "1.0.0",
  "uptime": 3600,
  "timestamp": "2024-02-11T12:00:00Z"
}
```

### DAG Operations

#### List DAGs

**Endpoint**: `GET /dags`

Retrieves a paginated list of available DAGs with optional filtering capabilities.

**Query Parameters**:
| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| page | integer | Page number for pagination | No |
| limit | integer | Number of items per page | No |
| searchName | string | Filter DAGs by matching name | No |
| searchTag | string | Filter DAGs by matching tag | No |

**Success Response (200)**:
```json
{
  "DAGs": [
    {
      "File": "example.yaml",
      "Dir": "/dags",
      "DAG": {
        "Group": "default",
        "Name": "example_dag",
        "Schedule": [
          {
            "Expression": "0 * * * *"
          }
        ],
        "Description": "Example DAG",
        "Params": ["param1", "param2"],
        "DefaultParams": "{}",
        "Tags": ["example", "demo"]
      },
      "Status": {
        "RequestId": "req-123",
        "Name": "example_dag",
        "Status": 1,
        "StatusText": "running",
        "Pid": 1234,
        "StartedAt": "2024-02-11T10:00:00Z",
        "FinishedAt": "",
        "Log": "/logs/example_dag.log",
        "Params": "{}"
      },
      "Suspended": false,
      "Error": ""
    }
  ],
  "Errors": [],
  "HasError": false,
  "PageCount": 1
}
```

**Response Fields**:
- `File`: Path to the DAG definition file
- `Dir`: Directory containing the DAG file
- `DAG`: DAG configuration and metadata
- `Status`: Current execution status
- `Suspended`: Whether the DAG is suspended
- `Error`: Error message if any

#### Create DAG

**Endpoint**: `POST /dags`

Creates a new DAG definition.

**Request Body**:
```json
{
  "action": "create",
  "value": "dag_definition_yaml_content"
}
```

**Request Fields**:
| Field | Type | Description | Required |
|-------|------|-------------|----------|
| action | string | Action to perform upon creation | Yes |
| value | string | DAG definition in YAML format | Yes |

**Success Response (200)**:
```json
{
  "DagID": "new_dag_123"
}
```

#### Get DAG Details

**Endpoint**: `GET /dags/{dagId}`

Retrieves detailed information about a specific DAG.

**URL Parameters**:
| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| dagId | string | Unique identifier of the DAG | Yes |

**Query Parameters**:
| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| tab | string | Tab name for UI navigation | No |
| file | string | Specific file related to the DAG | No |
| step | string | Step name within the DAG | No |

#### Perform DAG Action

**Endpoint**: `POST /dags/{dagId}`

Executes various actions on a specific DAG.

**URL Parameters**:
| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| dagId | string | Unique identifier of the DAG | Yes |

**Request Body**:
```json
{
  "action": "string",
  "value": "string",
  "requestId": "string",
  "step": "string",
  "params": "string"
}
```

**Request Fields**:
| Field | Type | Description | Required |
|-------|------|-------------|----------|
| action | string | Action to perform (see Available Actions below) | Yes |
| value | string | Additional value required by certain actions | No |
| requestId | string | Required for retry, mark-success, and mark-failed actions | Conditional |
| step | string | Required for mark-success and mark-failed actions | Conditional |
| params | string | JSON string of parameters for DAG execution | No |

**Available Actions**:

1. **start** - Begin DAG execution
   - Requires: none
   - Optional: params
   - Fails if DAG is already running

2. **suspend** - Toggle DAG suspension state
   - Requires: value ("true" or "false")

3. **stop** - Stop DAG execution
   - Requires: none
   - Fails if DAG is not running

4. **retry** - Retry a previous execution
   - Requires: requestId

5. **mark-success** - Mark a specific step as successful
   - Requires: requestId, step
   - Fails if DAG is running

6. **mark-failed** - Mark a specific step as failed
   - Requires: requestId, step
   - Fails if DAG is running

7. **save** - Update DAG definition
   - Requires: value (new DAG definition)

8. **rename** - Rename the DAG
   - Requires: value (new name)

**Success Response (200)**:
```json
{
  "newDagId": "string"
}
```

> Note: The `newDagId` field is only included in the response for the `rename` action.

**Error Responses**:

- **400 Bad Request**
  - Missing required action parameter
  - Invalid action type
  - DAG already running (for start action)
  - DAG not running (for stop action)
  - Missing required parameters for specific actions
  - Step not found (for mark-success/mark-failed actions)

- **404 Not Found**
  - DAG not found

- **500 Internal Server Error**
  - Failed to execute the requested action
  - Failed to update DAG status
  - Failed to rename DAG

### Search Operations

#### Search DAGs

**Endpoint**: `GET /search`

Performs a full-text search across DAG definitions.

**Query Parameters**:
| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| q | string | Search query string | Yes |

## API v2 Endpoints

### Monitoring Operations

#### Metrics Endpoint

**Endpoint**: `GET /metrics`

Exposes Prometheus-compatible metrics for monitoring Dagu operations. This endpoint provides real-time insights into DAG executions, system health, and performance metrics.

**Parameters**: None

**Success Response (200)**:
```text
# HELP dagu_info Dagu build information
# TYPE dagu_info gauge
dagu_info{version="1.14.0",build_date="2024-01-01T12:00:00Z",go_version="1.21"} 1

# HELP dagu_uptime_seconds Time since server start
# TYPE dagu_uptime_seconds gauge
dagu_uptime_seconds 3600

# HELP dagu_dag_runs_currently_running Number of currently running DAG runs
# TYPE dagu_dag_runs_currently_running gauge
dagu_dag_runs_currently_running 5

# HELP dagu_dag_runs_queued_total Total number of DAG runs in queue
# TYPE dagu_dag_runs_queued_total gauge
dagu_dag_runs_queued_total 8

# HELP dagu_dag_runs_total Total number of DAG runs by status (last 24 hours)
# TYPE dagu_dag_runs_total counter
dagu_dag_runs_total{status="success"} 2493
dagu_dag_runs_total{status="error"} 15
dagu_dag_runs_total{status="cancelled"} 7
dagu_dag_runs_total{status="running"} 5
dagu_dag_runs_total{status="queued"} 3
dagu_dag_runs_total{status="none"} 1

# HELP dagu_dags_total Total number of DAGs
# TYPE dagu_dags_total gauge
dagu_dags_total 45

# HELP dagu_scheduler_running Whether the scheduler is running
# TYPE dagu_scheduler_running gauge
dagu_scheduler_running 1
```

**Response Headers**:
- `Content-Type: text/plain; version=0.0.4; charset=utf-8`

**Available Metrics**:

**System Metrics**:
| Metric Name | Type | Description |
|-------------|------|-------------|
| dagu_info | gauge | Build information with version labels |
| dagu_uptime_seconds | gauge | Time since server start in seconds |
| dagu_scheduler_running | gauge | 1 if scheduler is running, 0 otherwise |

**DAG Execution Metrics**:
| Metric Name | Type | Description |
|-------------|------|-------------|
| dagu_dag_runs_currently_running | gauge | Number of DAG runs currently executing |
| dagu_dag_runs_queued_total | gauge | Total number of DAG runs waiting in queue (all DAGs) |
| dagu_dag_runs_total | counter | Total number of DAG runs by status (last 24 hours) |
| dagu_dags_total | gauge | Total number of registered DAGs |

> **Notes**: 
> - The `dagu_dag_runs_total` metric only includes DAG runs from the last 24 hours due to performance considerations.
> - Queue metrics (`dagu_dag_runs_queued_total`) count all queued items across all DAGs.
> - The metrics endpoint is compatible with Prometheus scraping and can be used with standard Prometheus configurations.

## Error Handling

All endpoints may return error responses in the following format:

```json
{
  "code": "error_code",
  "message": "Human readable error message",
  "details": {
    "additional": "error details"
  }
}
```

**Error Codes**:
| Code | Description |
|------|-------------|
| validation_error | Invalid request parameters or body |
| not_found | Requested resource doesn't exist |
| internal_error | Server-side error |
| unauthorized | Authentication/authorization failed |
| bad_gateway | Upstream service error |

## Example Usage

### Start a DAG with Parameters
```bash
curl -X POST "http://localhost:8080/api/v1/dags/example_dag" \
     -H "Content-Type: application/json" \
     -d '{
       "action": "start",
       "params": "{\"param1\": \"value1\"}"
     }'
```

### Mark a Step as Successful
```bash
curl -X POST "http://localhost:8080/api/v1/dags/example_dag" \
     -H "Content-Type: application/json" \
     -d '{
       "action": "mark-success",
       "requestId": "req_123",
       "step": "step1"
     }'
```

### Rename a DAG
```bash
curl -X POST "http://localhost:8080/api/v1/dags/example_dag" \
     -H "Content-Type: application/json" \
     -d '{
       "action": "rename",
       "value": "new_dag_name"
     }'
```

### Get Metrics
```bash
curl http://localhost:8080/api/v2/metrics
```

### Check Scheduler Status
```bash
curl -s http://localhost:8080/api/v2/metrics | grep "dagu_scheduler_running"
```

### Get Current Running DAGs Count
```bash
curl -s http://localhost:8080/api/v2/metrics | grep "dagu_dag_runs_currently_running"
```

## Prometheus Integration

### Configuration Example

To scrape Dagu metrics with Prometheus, add the following to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'dagu'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/api/v2/metrics'
    scrape_interval: 15s
```

### Grafana Dashboard Queries

Create monitoring dashboards with queries like:

```promql
# DAG execution success rate (last 24h)
rate(dagu_dag_runs_total{status="success"}[5m]) / 
rate(dagu_dag_runs_total[5m])

# Average queue length
avg_over_time(dagu_dag_runs_queued_total[5m])

# Scheduler uptime percentage
avg_over_time(dagu_scheduler_running[5m]) * 100
```

## Rate Limiting and Best Practices

- Use appropriate polling intervals for monitoring endpoints
- Implement exponential backoff for failed requests
- Cache responses when appropriate to reduce server load
- Use the search endpoint for discovery rather than listing all DAGs repeatedly
- Monitor your API usage to avoid overwhelming the server

## SDK and Client Libraries

While Dagu doesn't provide official SDKs, the REST API follows standard HTTP conventions and can be easily integrated with any HTTP client library in your preferred programming language.

Example client implementations:

### Python
```python
import requests
import json

class DaguClient:
    def __init__(self, base_url="http://localhost:8080"):
        self.base_url = base_url
        
    def start_dag(self, dag_id, params=None):
        url = f"{self.base_url}/api/v1/dags/{dag_id}"
        payload = {"action": "start"}
        if params:
            payload["params"] = json.dumps(params)
        
        response = requests.post(url, json=payload)
        return response.json()
    
    def get_health(self):
        url = f"{self.base_url}/api/v1/health"
        response = requests.get(url)
        return response.json()
```

### JavaScript/Node.js
```javascript
class DaguClient {
  constructor(baseUrl = 'http://localhost:8080') {
    this.baseUrl = baseUrl;
  }
  
  async startDag(dagId, params = null) {
    const url = `${this.baseUrl}/api/v1/dags/${dagId}`;
    const payload = { action: 'start' };
    if (params) {
      payload.params = JSON.stringify(params);
    }
    
    const response = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    
    return response.json();
  }
  
  async getHealth() {
    const response = await fetch(`${this.baseUrl}/api/v1/health`);
    return response.json();
  }
}
```

This completes the comprehensive REST API reference documentation for Dagu.