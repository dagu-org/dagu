# REST API Reference

This document provides a complete reference for Dagu's REST API endpoints, including detailed parameters, responses, and examples.

## Base Configuration

- **Base URL**: `http://localhost:8080/api/v2`
- **Content-Type**: `application/json`
- **OpenAPI Version**: 3.0.0

### Authentication

The API supports three authentication methods:

- **Basic Auth**: Include `Authorization: Basic <base64(username:password)>` header
- **Bearer Token**: Include `Authorization: Bearer <token>` header
- **No Authentication**: When auth is disabled (default for local development)

## System Endpoints

### Health Check

**Endpoint**: `GET /api/v2/health`

Checks the health status of the Dagu server.

**Response (200)**:
```json
{
  "status": "healthy",
  "version": "1.14.0",
  "uptime": 3600,
  "timestamp": "2024-02-11T12:00:00Z"
}
```

**Response Fields**:
- `status`: Server health status ("healthy" or "unhealthy")
- `version`: Current server version
- `uptime`: Server uptime in seconds
- `timestamp`: Current server time

## DAG Management Endpoints

### List DAGs

**Endpoint**: `GET /api/v2/dags`

Retrieves DAG definitions with optional filtering by name and tags.

**Query Parameters**:
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| page | integer | Page number (1-based) | 1 |
| perPage | integer | Items per page (max 1000) | 50 |
| name | string | Filter DAGs by name | - |
| tag | string | Filter DAGs by tag | - |
| remoteNode | string | Remote node name | "local" |

**Response (200)**:
```json
{
  "dags": [
    {
      "fileName": "example.yaml",
      "dag": {
        "name": "example_dag",
        "group": "default",
        "schedule": [{"expression": "0 * * * *"}],
        "description": "Example DAG",
        "params": ["param1", "param2"],
        "defaultParams": "{}",
        "tags": ["example", "demo"]
      },
      "latestDAGRun": {
        "dagRunId": "20240101_120000",
        "name": "example_dag",
        "status": 1,
        "statusLabel": "running",
        "startedAt": "2024-01-01T12:00:00Z",
        "finishedAt": "",
        "log": "/logs/example_dag.log"
      },
      "suspended": false,
      "errors": []
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

### Create DAG

**Endpoint**: `POST /api/v2/dags`

Creates a new empty DAG file with the specified name.

**Request Body**:
```json
{
  "name": "my-new-dag"
}
```

**Response (201)**:
```json
{
  "name": "my-new-dag"
}
```

### Get DAG Details

**Endpoint**: `GET /api/v2/dags/{fileName}`

Fetches detailed information about a specific DAG.

**Path Parameters**:
| Parameter | Type | Description | Pattern |
|-----------|------|-------------|---------|
| fileName | string | DAG file name | `^[a-zA-Z0-9_-]+$` |

**Response (200)**:
```json
{
  "dag": {
    "name": "example_dag",
    "schedule": [{"expression": "0 * * * *"}],
    "steps": [
      {
        "name": "step1",
        "command": "echo hello"
      }
    ]
  },
  "localDags": [],
  "latestDAGRun": {
    "dagRunId": "20240101_120000",
    "status": 4,
    "statusLabel": "finished"
  },
  "suspended": false,
  "errors": []
}
```

### Delete DAG

**Endpoint**: `DELETE /api/v2/dags/{fileName}`

Permanently removes a DAG definition from the system.

**Response (204)**: No content

### Get DAG Specification

**Endpoint**: `GET /api/v2/dags/{fileName}/spec`

Fetches the YAML specification of a DAG.

**Response (200)**:
```json
{
  "dag": {
    "name": "example_dag"
  },
  "spec": "name: example_dag\nsteps:\n  - name: hello\n    command: echo Hello",
  "errors": []
}
```

### Update DAG Specification

**Endpoint**: `PUT /api/v2/dags/{fileName}/spec`

Updates the YAML specification of a DAG.

**Request Body**:
```json
{
  "spec": "name: example_dag\nsteps:\n  - name: hello\n    command: echo Hello World"
}
```

**Response (200)**:
```json
{
  "errors": []
}
```

### Rename DAG

**Endpoint**: `POST /api/v2/dags/{fileName}/rename`

Changes the file ID of the DAG definition.

**Request Body**:
```json
{
  "newFileName": "new-dag-name"
}
```

**Response (200)**: Success

## DAG Execution Endpoints

### Start DAG

**Endpoint**: `POST /api/v2/dags/{fileName}/start`

Creates and starts a DAG run with optional parameters.

**Request Body**:
```json
{
  "params": "{\"env\": \"production\", \"version\": \"1.2.3\"}",
  "dagRunId": "custom-run-id"
}
```

**Request Fields**:
| Field | Type | Description | Required |
|-------|------|-------------|----------|
| params | string | JSON string of parameters | No |
| dagRunId | string | Custom run ID | No |

**Response (200)**:
```json
{
  "dagRunId": "20240101_120000_abc123"
}
```

### Enqueue DAG

**Endpoint**: `POST /api/v2/dags/{fileName}/enqueue`

Adds a DAG run to the queue for later execution.

**Request Body**: Same as Start DAG

**Response (200)**:
```json
{
  "dagRunId": "20240101_120000_abc123"
}
```

### Toggle DAG Suspension

**Endpoint**: `POST /api/v2/dags/{fileName}/suspend`

Controls whether the scheduler creates runs from this DAG.

**Request Body**:
```json
{
  "suspend": true
}
```

**Response (200)**: Success

## DAG Run History Endpoints

### Get DAG Run History

**Endpoint**: `GET /api/v2/dags/{fileName}/dag-runs`

Fetches execution history of a DAG.

**Response (200)**:
```json
{
  "dagRuns": [
    {
      "dagRunId": "20240101_120000",
      "name": "example_dag",
      "status": 4,
      "statusLabel": "finished",
      "startedAt": "2024-01-01T12:00:00Z",
      "finishedAt": "2024-01-01T12:05:00Z"
    }
  ],
  "gridData": [
    {
      "name": "step1",
      "history": [4, 4, 2, 4]
    }
  ]
}
```

### Get Specific DAG Run

**Endpoint**: `GET /api/v2/dags/{fileName}/dag-runs/{dagRunId}`

Gets detailed status of a specific DAG run.

**Response (200)**:
```json
{
  "dagRun": {
    "dagRunId": "20240101_120000",
    "nodes": [
      {
        "step": {
          "name": "step1",
          "command": "echo hello"
        },
        "status": 4,
        "statusLabel": "finished",
        "startedAt": "2024-01-01T12:00:00Z",
        "finishedAt": "2024-01-01T12:00:05Z"
      }
    ]
  }
}
```

## DAG Run Management Endpoints

### List All DAG Runs

**Endpoint**: `GET /api/v2/dag-runs`

Retrieves all DAG runs with optional filtering.

**Query Parameters**:
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| name | string | Filter by DAG name | - |
| status | integer | Filter by status (0-6) | - |
| fromDate | integer | Unix timestamp start | - |
| toDate | integer | Unix timestamp end | - |
| dagRunId | string | Filter by run ID | - |

**Status Values**:
- 0: Not started
- 1: Running
- 2: Failed
- 3: Cancelled
- 4: Success
- 5: Queued
- 6: Partial Success

### Get DAG Run Details

**Endpoint**: `GET /api/v2/dag-runs/{name}/{dagRunId}`

Fetches detailed status of a specific DAG run.

**Path Parameters**:
| Parameter | Type | Description |
|-----------|------|-------------|
| name | string | DAG name |
| dagRunId | string | DAG run ID or "latest" |

### Stop DAG Run

**Endpoint**: `POST /api/v2/dag-runs/{name}/{dagRunId}/stop`

Forcefully stops a running DAG run.

**Response (200)**: Success

### Retry DAG Run

**Endpoint**: `POST /api/v2/dag-runs/{name}/{dagRunId}/retry`

Creates a new DAG run based on a previous execution.

**Request Body**:
```json
{
  "dagRunId": "new-run-id"
}
```

**Response (200)**: Success

### Dequeue DAG Run

**Endpoint**: `GET /api/v2/dag-runs/{name}/{dagRunId}/dequeue`

Removes a queued DAG run from the queue.

**Response (200)**: Success

## Log Endpoints

### Get DAG Run Log

**Endpoint**: `GET /api/v2/dag-runs/{name}/{dagRunId}/log`

Fetches the execution log for a DAG run.

**Query Parameters**:
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| tail | integer | Lines from end | - |
| head | integer | Lines from start | - |
| offset | integer | Start line (1-based) | - |
| limit | integer | Max lines (max 10000) | - |

**Response (200)**:
```json
{
  "content": "2024-01-01 12:00:00 INFO Starting DAG...",
  "lineCount": 100,
  "totalLines": 500,
  "hasMore": true,
  "isEstimate": false
}
```

### Get Step Log

**Endpoint**: `GET /api/v2/dag-runs/{name}/{dagRunId}/steps/{stepName}/log`

Fetches the log for a specific step.

**Additional Query Parameters**:
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| stream | string | "stdout" or "stderr" | "stdout" |

## Step Management Endpoints

### Update Step Status

**Endpoint**: `PATCH /api/v2/dag-runs/{name}/{dagRunId}/steps/{stepName}/status`

Manually updates a step's execution status.

**Request Body**:
```json
{
  "status": 4
}
```

**Status Values**:
- 0: Not started
- 1: Running
- 2: Failed
- 3: Cancelled
- 4: Success
- 5: Skipped

**Response (200)**: Success

## Search Endpoints

### Search DAGs

**Endpoint**: `GET /api/v2/dags/search`

Performs full-text search across DAG definitions.

**Query Parameters**:
| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| q | string | Search query | Yes |

**Response (200)**:
```json
{
  "results": [
    {
      "name": "example_dag",
      "dag": {
        "name": "example_dag"
      },
      "matches": [
        {
          "line": "    command: database backup",
          "lineNumber": 15,
          "startLine": 10
        }
      ]
    }
  ],
  "errors": []
}
```

### Get All Tags

**Endpoint**: `GET /api/v2/dags/tags`

Retrieves all unique tags used across DAGs.

**Response (200)**:
```json
{
  "tags": ["production", "daily", "etl", "critical"],
  "errors": []
}
```

## Monitoring Endpoints

### Prometheus Metrics

**Endpoint**: `GET /api/v2/metrics`

Returns Prometheus-compatible metrics.

**Response (200)** (text/plain):
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

# HELP dagu_dag_runs_total Total number of DAG runs by status
# TYPE dagu_dag_runs_total counter
dagu_dag_runs_total{status="success"} 2493
dagu_dag_runs_total{status="error"} 15
dagu_dag_runs_total{status="cancelled"} 7

# HELP dagu_dags_total Total number of DAGs
# TYPE dagu_dags_total gauge
dagu_dags_total 45

# HELP dagu_scheduler_running Whether the scheduler is running
# TYPE dagu_scheduler_running gauge
dagu_scheduler_running 1
```

## Error Handling

All endpoints return structured error responses:

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
| forbidden | Insufficient permissions |
| bad_request | Invalid request parameters |
| not_found | Resource doesn't exist |
| internal_error | Server-side error |
| unauthorized | Authentication failed |
| bad_gateway | Upstream service error |
| remote_node_error | Remote node connection failed |
| already_running | DAG is already running |
| not_running | DAG is not running |
| already_exists | Resource already exists |

## Child DAG Run Endpoints

### Get Child DAG Run Details

**Endpoint**: `GET /api/v2/dag-runs/{name}/{dagRunId}/children/{childDAGRunId}`

Fetches detailed status of a child DAG run.

### Get Child DAG Run Log

**Endpoint**: `GET /api/v2/dag-runs/{name}/{dagRunId}/children/{childDAGRunId}/log`

Fetches the log for a child DAG run.

### Get Child Step Log

**Endpoint**: `GET /api/v2/dag-runs/{name}/{dagRunId}/children/{childDAGRunId}/steps/{stepName}/log`

Fetches the log for a step in a child DAG run.

### Update Child Step Status

**Endpoint**: `PATCH /api/v2/dag-runs/{name}/{dagRunId}/children/{childDAGRunId}/steps/{stepName}/status`

Updates the status of a step in a child DAG run.

## Example Usage

### Start a DAG with Parameters
```bash
curl -X POST "http://localhost:8080/api/v2/dags/etl-pipeline.yaml/start" \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer your-token" \
     -d '{
       "params": "{\"date\": \"2024-01-01\", \"env\": \"prod\"}",
       "dagRunId": "etl-20240101"
     }'
```

### Check DAG Run Status
```bash
curl "http://localhost:8080/api/v2/dag-runs/etl-pipeline/latest"
```

### Search for DAGs
```bash
curl "http://localhost:8080/api/v2/dags/search?q=database+backup"
```

### Get Metrics for Monitoring
```bash
curl "http://localhost:8080/api/v2/metrics" | grep dagu_dag_runs_currently_running
```
