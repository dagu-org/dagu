# REST API Reference

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
  "version": "1.14.5",
  "uptime": 86400,
  "timestamp": "2024-02-11T16:30:45.123456789Z"
}
```

**Response when unhealthy (503)**:
```json
{
  "status": "unhealthy",
  "version": "1.14.5",
  "uptime": 300,
  "timestamp": "2024-02-11T16:30:45.123456789Z",
  "error": "Scheduler not responding"
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

Creates a new DAG file with the specified name. Optionally initializes it with a provided YAML specification.

**Request Body**:
```json
{
  "name": "my-new-dag",
  "spec": "steps:\n  - command: echo hello"  // Optional - YAML spec to initialize the DAG
}
```

**Notes**:
- If `spec` is provided, it will be validated before creation
- If validation fails, returns 400 with error details
- Without `spec`, creates a minimal DAG with a single echo step

**Response (201)**:
```json
{
  "name": "my-new-dag"
}
```

**Error Response (400)**:
```json
{
  "code": "bad_request",
  "message": "Invalid DAG name format"
}
```

**Error Response (409)**:
```json
{
  "code": "already_exists",
  "message": "DAG with this name already exists"
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
    "name": "data_processing_pipeline",
    "group": "ETL",
    "schedule": [
      {"expression": "0 2 * * *"},
      {"expression": "0 14 * * *"}
    ],
    "description": "Daily data processing pipeline for warehouse ETL",
    "env": [
      "DATA_SOURCE=postgres://prod-db:5432/analytics",
      "WAREHOUSE_URL=${WAREHOUSE_URL}"
    ],
    "logDir": "/var/log/dagu/pipelines",
    "handlerOn": {
      "success": {
        "name": "notify_success",
        "command": "notify.sh 'Pipeline completed successfully'"
      },
      "failure": {
        "name": "alert_on_failure",
        "command": "alert.sh 'Pipeline failed' high"
      },
      "exit": {
        "name": "cleanup",
        "command": "cleanup_temp_files.sh"
      }
    },
    "steps": [
      {
        "name": "extract_data",
        "id": "extract",
        "description": "Extract data from source database",
        "dir": "/app/etl",
        "command": "python",
        "args": ["extract.py", "--date", "${date}"],
        "stdout": "/logs/extract.out",
        "stderr": "/logs/extract.err",
        "output": "EXTRACTED_FILE",
        "preconditions": [
          {
            "condition": "test -f /data/ready.flag",
            "expected": ""
          }
        ]
      },
      {
        "name": "transform_data",
        "id": "transform",
        "description": "Apply transformations to extracted data",
        "command": "python transform.py --input=${EXTRACTED_FILE}",
        "depends": ["extract_data"],
        "output": "TRANSFORMED_FILE",
        "repeatPolicy": {
          "repeat": false,
          "interval": 0
        },
        "mailOnError": true
      },
      {
        "name": "load_to_warehouse",
        "id": "load",
        "run": "warehouse-loader",
        "params": "{\"file\": \"${TRANSFORMED_FILE}\", \"table\": \"fact_sales\"}",
        "depends": ["transform_data"]
      }
    ],
    "delay": 30,
    "histRetentionDays": 30,
    "preconditions": [
      {
        "condition": "`date +%u`",
        "expected": "re:[1-5]",
        "error": "Pipeline only runs on weekdays"
      }
    ],
    "maxActiveRuns": 1,
    "maxActiveSteps": 5,
    "params": ["date", "env", "batch_size"],
    "defaultParams": "{\"batch_size\": 1000, \"env\": \"dev\"}",
    "tags": ["production", "etl", "daily"]
  },
  "localDags": [
    {
      "name": "warehouse-loader",
      "dag": {
        "name": "warehouse_loader_subdag",
        "steps": [
          {
            "name": "validate_schema",
            "command": "validate_schema.py"
          },
          {
            "name": "load_data",
            "command": "load_to_warehouse.py",
            "depends": ["validate_schema"]
          }
        ]
      },
      "errors": []
    }
  ],
  "latestDAGRun": {
    "rootDAGRunName": "data_processing_pipeline",
    "rootDAGRunId": "20240211_140000_abc123",
    "parentDAGRunName": "",
    "parentDAGRunId": "",
    "dagRunId": "20240211_140000_abc123",
    "name": "data_processing_pipeline",
    "status": 4,
    "statusLabel": "succeeded",
    "queuedAt": "",
    "startedAt": "2024-02-11T14:00:00Z",
    "finishedAt": "2024-02-11T14:45:30Z",
    "log": "/logs/data_processing_pipeline/20240211_140000_abc123.log",
    "params": "{\"date\": \"2024-02-11\", \"env\": \"production\", \"batch_size\": 5000}"
  },
  "suspended": false,
  "errors": []
}
```

### Delete DAG

**Endpoint**: `DELETE /api/v2/dags/{fileName}`

Permanently removes a DAG definition from the system.

**Response (204)**: No content

**Error Response (404)**:
```json
{
  "code": "not_found",
  "message": "DAG not found"
}
```

**Error Response (403)**:
```json
{
  "code": "forbidden",
  "message": "Permission denied to delete DAGs"
}
```

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

**Response with Validation Errors (200)**:
```json
{
  "errors": [
    "Line 5: Invalid step configuration - missing command",
    "Line 10: Circular dependency detected between step1 and step2"
  ]
}
```

**Error Response (403)**:
```json
{
  "code": "forbidden",
  "message": "Permission denied to edit DAGs"
}
```

### Validate DAG Specification

**Endpoint**: `POST /api/v2/dags/validate`

Validates a DAG YAML specification without persisting any changes. Returns a list of validation errors. When the spec can be partially parsed, the response may include parsed DAG details built with error-tolerant loading.

**Request Body**:
```json
{
  "spec": "steps:\n  - name: step1\n    command: echo hello\n  - name: step2\n    command: echo world\n    depends: [step1]",
  "name": "optional-dag-name"  // Optional - name to use when spec omits a name
}
```

**Response (200)**:
```json
{
  "valid": true,
  "errors": [],
  "dag": {
    "name": "example-dag",
    "group": "default",
    "description": "Validated DAG",
    "schedule": [],
    "params": [],
    "defaultParams": "{}",
    "tags": []
  }
}
```

**Response with errors (200)**:
```json
{
  "valid": false,
  "errors": [
    "Step 'step2' depends on non-existent step 'missing_step'",
    "Invalid cron expression in schedule: '* * * *'"
  ],
  "dag": {
    "name": "example-dag",
    // Partial DAG details when possible
  }
}
```

**Notes**:
- Always returns 200 status - check `valid` field to determine if spec is valid
- `errors` array contains human-readable validation messages
- `dag` field may contain partial DAG details even when validation fails
- Use this endpoint to validate DAG specs before creating or updating DAGs

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

**Error Response (400)**:
```json
{
  "code": "bad_request",
  "message": "Invalid new file name format"
}
```

**Error Response (409)**:
```json
{
  "code": "already_exists",
  "message": "A DAG with the new name already exists"
}
```

## DAG Execution Endpoints

### Start DAG

**Endpoint**: `POST /api/v2/dags/{fileName}/start`

Creates and starts a DAG run with optional parameters.

**Request Body**:
```json
{
  "params": "{\"env\": \"production\", \"version\": \"1.2.3\"}",
  "dagRunId": "custom-run-id",
  "singleton": false
}
```

**Request Fields**:
| Field | Type | Description | Required |
|-------|------|-------------|----------|
| params | string | JSON string of parameters | No |
| dagRunId | string | Custom run ID | No |
| singleton | boolean | If true, prevent starting if DAG is already running (returns 409) | No |

**Response (200)**:
```json
{
  "dagRunId": "20240101_120000_abc123"
}
```

**Response (409)** - When `singleton: true` and DAG is already running:
```json
{
  "code": "already_running",
  "message": "DAG example_dag is already running, cannot start in singleton mode"
}
```

### Enqueue DAG

**Endpoint**: `POST /api/v2/dags/{fileName}/enqueue`

Adds a DAG run to the queue for later execution.

**Request Body**:
```json
{
  "params": "{\"key\": \"value\"}",
  "dagRunId": "optional-custom-id",
  "queue": "optional-queue-override"
}
```

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

**Error Response (404)**:
```json
{
  "code": "not_found",
  "message": "DAG not found"
}
```

## DAG Run History Endpoints

### Get DAG Run History

**Endpoint**: `GET /api/v2/dags/{fileName}/dag-runs`

Fetches execution history of a DAG.

**Response (200)**:
```json
{
  "dagRuns": [
    {
      "rootDAGRunName": "data_processing_pipeline",
      "rootDAGRunId": "20240211_140000_abc123",
      "parentDAGRunName": "",
      "parentDAGRunId": "",
      "dagRunId": "20240211_140000_abc123",
      "name": "data_processing_pipeline",
      "status": 4,
      "statusLabel": "succeeded",
      "queuedAt": "",
      "startedAt": "2024-02-11T14:00:00Z",
      "finishedAt": "2024-02-11T14:45:30Z",
      "log": "/logs/data_processing_pipeline/20240211_140000_abc123.log",
      "params": "{\"date\": \"2024-02-11\", \"env\": \"production\", \"batch_size\": 5000}"
    },
    {
      "rootDAGRunName": "data_processing_pipeline",
      "rootDAGRunId": "20240211_020000_def456",
      "parentDAGRunName": "",
      "parentDAGRunId": "",
      "dagRunId": "20240211_020000_def456",
      "name": "data_processing_pipeline",
      "status": 2,
      "statusLabel": "failed",
      "queuedAt": "",
      "startedAt": "2024-02-11T02:00:00Z",
      "finishedAt": "2024-02-11T02:15:45Z",
      "log": "/logs/data_processing_pipeline/20240211_020000_def456.log",
      "params": "{\"date\": \"2024-02-11\", \"env\": \"production\", \"batch_size\": 5000}"
    },
    {
      "rootDAGRunName": "data_processing_pipeline",
      "rootDAGRunId": "20240210_140000_ghi789",
      "parentDAGRunName": "",
      "parentDAGRunId": "",
      "dagRunId": "20240210_140000_ghi789",
      "name": "data_processing_pipeline",
      "status": 4,
      "statusLabel": "succeeded",
      "queuedAt": "",
      "startedAt": "2024-02-10T14:00:00Z",
      "finishedAt": "2024-02-10T14:42:15Z",
      "log": "/logs/data_processing_pipeline/20240210_140000_ghi789.log",
      "params": "{\"date\": \"2024-02-10\", \"env\": \"production\", \"batch_size\": 5000}"
    }
  ],
  "gridData": [
    {
      "name": "extract_data",
      "history": [4, 2, 4]
    },
    {
      "name": "transform_data",
      "history": [4, 2, 4]
    },
    {
      "name": "load_to_warehouse",
      "history": [4, 0, 4]
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
    "rootDAGRunName": "data_processing_pipeline",
    "rootDAGRunId": "20240211_140000_abc123",
    "parentDAGRunName": "",
    "parentDAGRunId": "",
    "dagRunId": "20240211_140000_abc123",
    "name": "data_processing_pipeline",
    "status": 4,
    "statusLabel": "succeeded",
    "queuedAt": "",
    "startedAt": "2024-02-11T14:00:00Z",
    "finishedAt": "2024-02-11T14:45:30Z",
    "log": "/logs/data_processing_pipeline/20240211_140000_abc123.log",
    "params": "{\"date\": \"2024-02-11\", \"env\": \"production\", \"batch_size\": 5000}",
    "nodes": [
      {
        "step": {
          "name": "extract_data",
          "id": "extract",
          "command": "python",
          "args": ["extract.py", "--date", "2024-02-11"]
        },
        "stdout": "/logs/data_processing_pipeline/20240211_140000_abc123/extract_data.stdout",
        "stderr": "/logs/data_processing_pipeline/20240211_140000_abc123/extract_data.stderr",
        "startedAt": "2024-02-11T14:00:30Z",
        "finishedAt": "2024-02-11T14:15:45Z",
        "status": 4,
        "statusLabel": "succeeded",
        "retryCount": 0,
        "doneCount": 1,
        "subRuns": [],
        "subRunsRepeated": [],
        "error": ""
      },
      {
        "step": {
          "name": "transform_data",
          "id": "transform",
          "command": "python transform.py --input=/tmp/extracted_20240211.csv",
          "depends": ["extract_data"]
        },
        "stdout": "/logs/data_processing_pipeline/20240211_140000_abc123/transform_data.stdout",
        "stderr": "/logs/data_processing_pipeline/20240211_140000_abc123/transform_data.stderr",
        "startedAt": "2024-02-11T14:15:45Z",
        "finishedAt": "2024-02-11T14:30:20Z",
        "status": 4,
        "statusLabel": "succeeded",
        "retryCount": 0,
        "doneCount": 1,
        "subRuns": [],
        "subRunsRepeated": [],
        "error": ""
      },
      {
        "step": {
          "name": "load_to_warehouse",
          "id": "load",
          "run": "warehouse-loader",
          "params": "{\"file\": \"/tmp/transformed_20240211.csv\", \"table\": \"fact_sales\"}",
          "depends": ["transform_data"]
        },
        "stdout": "/logs/data_processing_pipeline/20240211_140000_abc123/load_to_warehouse.stdout",
        "stderr": "/logs/data_processing_pipeline/20240211_140000_abc123/load_to_warehouse.stderr",
        "startedAt": "2024-02-11T14:30:20Z",
        "finishedAt": "2024-02-11T14:45:30Z",
        "status": 4,
        "statusLabel": "succeeded",
        "retryCount": 0,
        "doneCount": 1,
        "subRuns": [
          {
            "dagRunId": "sub_20240211_143020_xyz456",
            "params": "{\"file\": \"/tmp/transformed_20240211.csv\", \"table\": \"fact_sales\"}"
          }
        ],
        "subRunsRepeated": [],
        "error": ""
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
| remoteNode | string | Remote node name | "local" |

**Note**: This endpoint does not support pagination. All matching results are returned.

**Status Values**:
- 0: Not started
- 1: Running
- 2: Failed
- 3: Cancelled
- 4: Success
- 5: Queued
- 6: Partial Success

**Response (200)**:
```json
{
  "dagRuns": [
    {
      "rootDAGRunName": "data_processing_pipeline",
      "rootDAGRunId": "20240211_160000_current",
      "parentDAGRunName": "",
      "parentDAGRunId": "",
      "dagRunId": "20240211_160000_current",
      "name": "data_processing_pipeline",
      "status": 1,
      "statusLabel": "running",
      "queuedAt": "",
      "startedAt": "2024-02-11T16:00:00Z",
      "finishedAt": "",
      "log": "/logs/data_processing_pipeline/20240211_160000_current.log",
      "params": "{\"date\": \"2024-02-11\", \"env\": \"production\", \"batch_size\": 5000}"
    },
    {
      "rootDAGRunName": "database_backup",
      "rootDAGRunId": "20240211_150000_backup",
      "parentDAGRunName": "",
      "parentDAGRunId": "",
      "dagRunId": "20240211_150000_backup",
      "name": "database_backup",
      "status": 4,
      "statusLabel": "succeeded",
      "queuedAt": "",
      "startedAt": "2024-02-11T15:00:00Z",
      "finishedAt": "2024-02-11T15:45:30Z",
      "log": "/logs/database_backup/20240211_150000_backup.log",
      "params": "{\"target_db\": \"production\", \"retention_days\": 30}"
    },
    {
      "rootDAGRunName": "ml_training_pipeline",
      "rootDAGRunId": "20240211_143000_ml",
      "parentDAGRunName": "",
      "parentDAGRunId": "",
      "dagRunId": "20240211_143000_ml",
      "name": "ml_training_pipeline",
      "status": 5,
      "statusLabel": "queued",
      "queuedAt": "2024-02-11T14:30:00Z",
      "startedAt": "",
      "finishedAt": "",
      "log": "/logs/ml_training_pipeline/20240211_143000_ml.log",
      "params": "{\"model\": \"recommendation_v2\", \"dataset\": \"user_interactions\"}"
    }
  ]
}
```

### Get DAG Run Details

**Endpoint**: `GET /api/v2/dag-runs/{name}/{dagRunId}`

Fetches detailed status of a specific DAG run. You can use the special value "latest" as the dagRunId to retrieve the most recent DAG run for the specified DAG.

**Examples**:
- `GET /api/v2/dag-runs/data-pipeline/20240211_120000` - Get a specific run
- `GET /api/v2/dag-runs/data-pipeline/latest` - Get the latest run

**Path Parameters**:
| Parameter | Type | Description |
|-----------|------|-------------|
| name | string | DAG name |
| dagRunId | string | DAG run ID or "latest" |

**Response (200)**:
```json
{
  "dagRun": {
    "dagRunId": "20240211_120000",
    "name": "data-pipeline",
    "status": 4,
    "statusLabel": "succeeded",
    "startedAt": "2024-02-11T12:00:00Z",
    "finishedAt": "2024-02-11T12:15:00Z",
    "params": "{\"date\": \"2024-02-11\", \"env\": \"prod\"}",
    "nodes": [
      {
        "step": {
          "name": "extract",
          "command": "python extract.py"
        },
        "status": 4,
        "statusLabel": "succeeded",
        "startedAt": "2024-02-11T12:00:00Z",
        "finishedAt": "2024-02-11T12:05:00Z",
        "retryCount": 0,
        "stdout": "/logs/data-pipeline/20240211_120000/extract.out",
        "stderr": "/logs/data-pipeline/20240211_120000/extract.err"
      },
      {
        "step": {
          "name": "transform",
          "command": "python transform.py",
          "depends": ["extract"]
        },
        "status": 4,
        "statusLabel": "succeeded",
        "startedAt": "2024-02-11T12:05:00Z",
        "finishedAt": "2024-02-11T12:10:00Z",
        "retryCount": 0
      },
      {
        "step": {
          "name": "load",
          "run": "sub-workflow",
          "params": "TARGET=warehouse"
        },
        "status": 4,
        "statusLabel": "succeeded",
        "startedAt": "2024-02-11T12:10:00Z",
        "finishedAt": "2024-02-11T12:15:00Z",
        "subRuns": [
          {
            "dagRunId": "sub_20240211_121000",
            "name": "sub-workflow",
            "status": 4,
            "statusLabel": "succeeded"
          }
        ]
      }
    ]
  }
}
```

### Stop DAG Run

**Endpoint**: `POST /api/v2/dag-runs/{name}/{dagRunId}/stop`

Forcefully stops a running DAG run.

**Response (200)**: Success

**Error Response (404)**:
```json
{
  "code": "not_found",
  "message": "DAG run not found"
}
```

**Error Response (400)**:
```json
{
  "code": "not_running",
  "message": "DAG is not currently running"
}
```

### Stop All DAG Runs

**Endpoint**: `POST /api/v2/dags/{fileName}/stop-all`

Forcefully stops all currently running instances of a DAG. This is useful when multiple instances of the same DAG are running simultaneously.

**Response (200)**: 
```json
{
  "errors": []
}
```

**Error Response (404)**:
```json
{
  "code": "not_found",
  "message": "DAG not found"
}
```

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

**Error Response (404)**:
```json
{
  "code": "not_found",
  "message": "Original DAG run not found"
}
```

**Error Response (400)**:
```json
{
  "code": "already_running",
  "message": "Another instance of this DAG is already running"
}
```

### Dequeue DAG Run

**Endpoint**: `GET /api/v2/dag-runs/{name}/{dagRunId}/dequeue`

Removes a queued DAG run from the queue.

**Response (200)**: Success

**Error Response (404)**:
```json
{
  "code": "not_found",
  "message": "DAG run not found in queue"
}
```

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
| remoteNode | string | Remote node name | "local" |

**Response (200)**:
```json
{
  "content": "2024-02-11 14:00:00 INFO DAG data_processing_pipeline started\n2024-02-11 14:00:00 INFO Run ID: 20240211_140000_abc123\n2024-02-11 14:00:00 INFO Parameters: {\"date\": \"2024-02-11\", \"env\": \"production\", \"batch_size\": 5000}\n2024-02-11 14:00:00 INFO Checking preconditions...\n2024-02-11 14:00:01 INFO Precondition passed: Weekday check (current day: 7)\n2024-02-11 14:00:01 INFO Starting step: extract_data\n2024-02-11 14:00:30 INFO [extract_data] Executing: python extract.py --date 2024-02-11\n2024-02-11 14:15:45 INFO [extract_data] Step completed successfully\n2024-02-11 14:15:45 INFO [extract_data] Output saved to variable: EXTRACTED_FILE = /tmp/extracted_20240211.csv\n2024-02-11 14:15:45 INFO Starting step: transform_data\n2024-02-11 14:15:45 INFO [transform_data] Executing: python transform.py --input=/tmp/extracted_20240211.csv\n2024-02-11 14:30:20 INFO [transform_data] Step completed successfully\n2024-02-11 14:30:20 INFO [transform_data] Output saved to variable: TRANSFORMED_FILE = /tmp/transformed_20240211.csv\n2024-02-11 14:30:20 INFO Starting step: load_to_warehouse\n2024-02-11 14:30:20 INFO [load_to_warehouse] Running sub DAG: warehouse-loader\n2024-02-11 14:30:20 INFO [load_to_warehouse] Sub DAG started with ID: sub_20240211_143020_xyz456\n2024-02-11 14:45:30 INFO [load_to_warehouse] Sub DAG completed successfully\n2024-02-11 14:45:30 INFO Executing onSuccess handler: notify_success\n2024-02-11 14:45:32 INFO [notify_success] Handler completed\n2024-02-11 14:45:32 INFO Executing onExit handler: cleanup\n2024-02-11 14:45:35 INFO [cleanup] Handler completed\n2024-02-11 14:45:35 INFO DAG completed successfully\n",
  "lineCount": 22,
  "totalLines": 156,
  "hasMore": true,
  "isEstimate": false
}
```

### Get Step Log

**Endpoint**: `GET /api/v2/dag-runs/{name}/{dagRunId}/steps/{stepName}/log`

Fetches the log for a specific step.

**Query Parameters**:
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| stream | string | "stdout" or "stderr" | "stdout" |
| tail | integer | Lines from end | - |
| head | integer | Lines from start | - |
| offset | integer | Start line (1-based) | - |
| limit | integer | Max lines (max 10000) | - |

**Response (200)**:
```json
{
  "content": "2024-02-11 12:05:00 INFO Starting data transformation...\n2024-02-11 12:05:01 INFO Processing 1000 records\n2024-02-11 12:05:05 INFO Transformation complete\n",
  "lineCount": 3,
  "totalLines": 3,
  "hasMore": false,
  "isEstimate": false
}
```

**Response with stderr (200)**:
```json
{
  "content": "2024-02-11 12:05:02 WARNING Duplicate key found, skipping record ID: 123\n2024-02-11 12:05:03 WARNING Invalid date format in record ID: 456\n",
  "lineCount": 2,
  "totalLines": 2,
  "hasMore": false,
  "isEstimate": false
}
```

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

**Error Response (404)**:
```json
{
  "code": "not_found",
  "message": "Step not found in DAG run"
}
```

**Error Response (400)**:
```json
{
  "code": "bad_request",
  "message": "Invalid status value"
}
```

## Search Endpoints

### Search DAGs

**Endpoint**: `GET /api/v2/dags/search`

Performs full-text search across DAG definitions.

**Query Parameters**:
| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| q | string | Search query | Yes |
| remoteNode | string | Remote node name | No |

**Response (200)**:
```json
{
  "results": [
    {
      "name": "database_backup",
      "dag": {
        "name": "database_backup",
        "group": "Operations",
        "schedule": [{"expression": "0 0 * * 0"}],
        "description": "Weekly database backup job",
        "params": ["target_db", "retention_days"],
        "defaultParams": "{\"retention_days\": 30}",
        "tags": ["backup", "weekly", "critical"]
      },
      "matches": [
        {
          "line": "    command: pg_dump ${target_db} | gzip > backup_$(date +%Y%m%d).sql.gz",
          "lineNumber": 25,
          "startLine": 20
        },
        {
          "line": "description: Weekly database backup job",
          "lineNumber": 3,
          "startLine": 1
        }
      ]
    },
    {
      "name": "data_processing_pipeline",
      "dag": {
        "name": "data_processing_pipeline",
        "group": "ETL",
        "schedule": [
          {"expression": "0 2 * * *"},
          {"expression": "0 14 * * *"}
        ],
        "description": "Daily data processing pipeline for warehouse ETL",
        "params": ["date", "env", "batch_size"],
        "defaultParams": "{\"batch_size\": 1000, \"env\": \"dev\"}",
        "tags": ["production", "etl", "daily"]
      },
      "matches": [
        {
          "line": "      command: psql -h ${DB_HOST} -d analytics -c \"COPY data TO STDOUT\"",
          "lineNumber": 45,
          "startLine": 42
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

**Query Parameters**:
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| remoteNode | string | Remote node name | "local" |

**Response (200)**:
```json
{
  "tags": [
    "backup",
    "critical",
    "daily",
    "data-quality",
    "etl",
    "experimental",
    "hourly",
    "maintenance",
    "ml",
    "monitoring",
    "production",
    "reporting",
    "staging",
    "testing",
    "weekly"
  ],
  "errors": []
}
```

**Response with Errors (200)**:
```json
{
  "tags": [
    "backup",
    "critical",
    "daily",
    "etl",
    "production"
  ],
  "errors": [
    "Error reading DAG file: malformed-etl.yaml - yaml: line 15: found unexpected end of stream",
    "Error reading DAG file: invalid-syntax.yaml - yaml: unmarshal errors:\n  line 8: field invalidField not found in type digraph.DAG"
  ]
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
dagu_dag_runs_total{status="succeeded"} 2493
dagu_dag_runs_total{status="failed"} 15
dagu_dag_runs_total{status="canceled"} 7

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

## Sub DAG Run Endpoints

### Get Sub DAG Run Details

**Endpoint**: `GET /api/v2/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}`

Fetches detailed status of a sub DAG run.

**Response (200)**:
```json
{
  "dagRunDetails": {
    "rootDAGRunName": "data_processing_pipeline",
    "rootDAGRunId": "20240211_140000_abc123",
    "parentDAGRunName": "data_processing_pipeline",
    "parentDAGRunId": "20240211_140000_abc123",
    "dagRunId": "sub_20240211_143020_xyz456",
    "name": "warehouse_loader_subdag",
    "status": 4,
    "statusLabel": "succeeded",
    "queuedAt": "",
    "startedAt": "2024-02-11T14:30:20Z",
    "finishedAt": "2024-02-11T14:45:30Z",
    "log": "/logs/warehouse_loader_subdag/sub_20240211_143020_xyz456.log",
    "params": "{\"file\": \"/tmp/transformed_20240211.csv\", \"table\": \"fact_sales\"}",
    "nodes": [
      {
        "step": {
          "name": "validate_schema",
          "command": "validate_schema.py",
          "args": [],
          "depends": []
        },
        "stdout": "/logs/warehouse_loader_subdag/sub_20240211_143020_xyz456/validate_schema.stdout",
        "stderr": "/logs/warehouse_loader_subdag/sub_20240211_143020_xyz456/validate_schema.stderr",
        "startedAt": "2024-02-11T14:30:20Z",
        "finishedAt": "2024-02-11T14:30:35Z",
        "status": 4,
        "statusLabel": "succeeded",
        "retryCount": 0,
        "doneCount": 1,
        "subRuns": [],
        "subRunsRepeated": [],
        "error": ""
      },
      {
        "step": {
          "name": "load_data",
          "command": "load_to_warehouse.py",
          "depends": ["validate_schema"]
        },
        "stdout": "/logs/warehouse_loader_subdag/sub_20240211_143020_xyz456/load_data.stdout",
        "stderr": "/logs/warehouse_loader_subdag/sub_20240211_143020_xyz456/load_data.stderr",
        "startedAt": "2024-02-11T14:30:35Z",
        "finishedAt": "2024-02-11T14:45:30Z",
        "status": 4,
        "statusLabel": "succeeded",
        "retryCount": 0,
        "doneCount": 1,
        "subRuns": [],
        "subRunsRepeated": [],
        "error": ""
      }
    ],
    "onExit": null,
    "onSuccess": null,
    "onFailure": null,
    "onCancel": null,
    "preconditions": []
  }
}
```

### Get Sub DAG Run Log

**Endpoint**: `GET /api/v2/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}/log`

Fetches the log for a sub DAG run.

**Response (200)**:
```json
{
  "content": "2024-02-11 14:30:20 INFO Starting sub DAG: warehouse_loader_subdag\n2024-02-11 14:30:20 INFO Parameters: {\"file\": \"/tmp/transformed_20240211.csv\", \"table\": \"fact_sales\"}\n2024-02-11 14:30:20 INFO Parent DAG: data_processing_pipeline (20240211_140000_abc123)\n2024-02-11 14:30:20 INFO Step 'validate_schema' started\n2024-02-11 14:30:22 INFO Schema validation: Checking table structure for 'fact_sales'\n2024-02-11 14:30:35 INFO Step 'validate_schema' completed successfully\n2024-02-11 14:30:35 INFO Step 'load_data' started\n2024-02-11 14:30:36 INFO Opening file: /tmp/transformed_20240211.csv\n2024-02-11 14:30:37 INFO File contains 50000 records\n2024-02-11 14:30:38 INFO Beginning bulk insert to warehouse.fact_sales\n2024-02-11 14:35:00 INFO Progress: 25000/50000 records loaded (50%)\n2024-02-11 14:40:00 INFO Progress: 45000/50000 records loaded (90%)\n2024-02-11 14:45:28 INFO All 50000 records loaded successfully\n2024-02-11 14:45:29 INFO Committing transaction\n2024-02-11 14:45:30 INFO Step 'load_data' completed successfully\n2024-02-11 14:45:30 INFO Sub DAG completed successfully\n",
  "lineCount": 7,
  "totalLines": 7,
  "hasMore": false,
  "isEstimate": false
}
```

### Get Child Step Log

**Endpoint**: `GET /api/v2/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}/steps/{stepName}/log`

Fetches the log for a step in a sub DAG run.

**Response (200)**:
```json
{
  "content": "2024-02-11 14:30:35 INFO Step 'load_data' started\n2024-02-11 14:30:36 INFO Opening file: /tmp/transformed_20240211.csv\n2024-02-11 14:30:37 INFO File contains 50000 records\n2024-02-11 14:30:38 INFO Beginning bulk insert to warehouse.fact_sales\n2024-02-11 14:30:39 INFO Using batch size: 5000\n2024-02-11 14:30:40 INFO Processing batch 1 of 10\n2024-02-11 14:32:00 INFO Batch 1 complete (5000 records)\n2024-02-11 14:33:20 INFO Processing batch 2 of 10\n2024-02-11 14:34:40 INFO Batch 2 complete (10000 records)\n2024-02-11 14:36:00 INFO Processing batch 3 of 10\n2024-02-11 14:37:20 INFO Batch 3 complete (15000 records)\n2024-02-11 14:38:40 INFO Processing batch 4 of 10\n2024-02-11 14:40:00 INFO Batch 4 complete (20000 records)\n2024-02-11 14:41:20 INFO Processing batch 5 of 10\n2024-02-11 14:42:40 INFO Batch 5 complete (25000 records)\n[... truncated for brevity ...]\n2024-02-11 14:45:28 INFO All 50000 records loaded successfully\n2024-02-11 14:45:29 INFO Committing transaction\n2024-02-11 14:45:30 INFO Step 'load_data' completed successfully\n",
  "lineCount": 50,
  "totalLines": 156,
  "hasMore": true,
  "isEstimate": false
}
```

### Update Child Step Status

**Endpoint**: `PATCH /api/v2/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}/steps/{stepName}/status`

Updates the status of a step in a sub DAG run.

**Request Body**:
```json
{
  "status": 4
}
```

**Response (200)**: Success

**Error Response (404)**:
```json
{
  "code": "not_found",
  "message": "Sub DAG run or step not found"
}
```

## Queue Management Endpoints

### List All Queues

**Endpoint**: `GET /api/v2/queues`

Retrieves all execution queues with their running and queued DAG runs. Queues are organized by queue name, with two types: "custom" (explicitly defined queues) and "dag-based" (DAG name used as queue name).

**Response (200)**:
```json
{
  "queues": [
    {
      "name": "etl-pipeline",
      "type": "custom",
      "maxConcurrency": 2,
      "summary": {
        "running": 1,
        "queued": 2,
        "total": 3
      },
      "runningDAGRuns": [
        {
          "dagRunId": "20240211_140000_abc123",
          "name": "data_processing_pipeline",
          "status": 1,
          "statusLabel": "running",
          "startedAt": "2024-02-11T14:00:00Z",
          "finishedAt": "",
          "log": "/logs/data_processing_pipeline/20240211_140000_abc123.log"
        }
      ],
      "queuedDAGRuns": [
        {
          "dagRunId": "20240211_143000_def456",
          "name": "ml_training_pipeline",
          "status": 5,
          "statusLabel": "queued",
          "startedAt": "",
          "finishedAt": "",
          "log": "/logs/ml_training_pipeline/20240211_143000_def456.log"
        },
        {
          "dagRunId": "20240211_144000_ghi789",
          "name": "analytics_pipeline",
          "status": 5,
          "statusLabel": "queued",
          "startedAt": "",
          "finishedAt": "",
          "log": "/logs/analytics_pipeline/20240211_144000_ghi789.log"
        }
      ]
    },
    {
      "name": "backup_job",
      "type": "dag-based",
      "summary": {
        "running": 1,
        "queued": 0,
        "total": 1
      },
      "runningDAGRuns": [
        {
          "dagRunId": "20240211_150000_backup",
          "name": "backup_job",
          "status": 1,
          "statusLabel": "running",
          "startedAt": "2024-02-11T15:00:00Z",
          "finishedAt": "",
          "log": "/logs/backup_job/20240211_150000_backup.log"
        }
      ],
      "queuedDAGRuns": []
    }
  ]
}
```

**Response Fields**:
- `queues`: Array of queue objects containing running and queued DAG runs
- `name`: Queue name (either custom queue name or DAG name for dag-based queues)
- `type`: Queue type - "custom" for explicitly defined queues, "dag-based" for DAG name queues
- `maxConcurrency`: Maximum concurrent runs (only present for custom queues)
- `summary`: Count summary with running, queued, and total DAG runs
- `runningDAGRuns`: Array of currently running DAG runs (uses DAGRunSummary schema)
- `queuedDAGRuns`: Array of queued DAG runs waiting for execution (uses DAGRunSummary schema)

**Queue Types**:
- **Custom**: Explicitly defined queues with configurable `maxConcurrency`
- **DAG-based**: Implicit queues where DAG name serves as the queue name

**Error Response (500)**:
```json
{
  "code": "internal_error",
  "message": "Failed to retrieve queue information"
}
```

## Additional Endpoints

### List DAG Runs by Name

**Endpoint**: `GET /api/v2/dag-runs/{name}`

Lists all DAG runs for a specific DAG name.

**Path Parameters**:
| Parameter | Type | Description |
|-----------|------|-------------|
| name | string | DAG name |

**Query Parameters**:
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| status | integer | Filter by status (0-6) | - |
| fromDate | integer | Unix timestamp start | - |
| toDate | integer | Unix timestamp end | - |
| dagRunId | string | Filter by run ID | - |
| remoteNode | string | Remote node name | "local" |

**Response (200)**:
```json
{
  "dagRuns": [
    {
      "rootDAGRunName": "data_processing_pipeline",
      "rootDAGRunId": "20240211_140000_abc123",
      "parentDAGRunName": "",
      "parentDAGRunId": "",
      "dagRunId": "20240211_140000_abc123",
      "name": "data_processing_pipeline",
      "status": 4,
      "statusLabel": "succeeded",
      "queuedAt": "",
      "startedAt": "2024-02-11T14:00:00Z",
      "finishedAt": "2024-02-11T14:45:30Z",
      "log": "/logs/data_processing_pipeline/20240211_140000_abc123.log",
      "params": "{\"date\": \"2024-02-11\", \"env\": \"production\", \"batch_size\": 5000}"
    },
    {
      "rootDAGRunName": "data_processing_pipeline",
      "rootDAGRunId": "20240211_020000_def456",
      "parentDAGRunName": "",
      "parentDAGRunId": "",
      "dagRunId": "20240211_020000_def456",
      "name": "data_processing_pipeline",
      "status": 2,
      "statusLabel": "failed",
      "queuedAt": "",
      "startedAt": "2024-02-11T02:00:00Z",
      "finishedAt": "2024-02-11T02:15:45Z",
      "log": "/logs/data_processing_pipeline/20240211_020000_def456.log",
      "params": "{\"date\": \"2024-02-11\", \"env\": \"production\", \"batch_size\": 5000}"
    }
  ]
}
```

## Example Usage

### Start a DAG with Parameters
```bash
curl -X POST "http://localhost:8080/api/v2/dags/data-processing-pipeline/start" \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer your-token" \
     -d '{
       "params": "{\"date\": \"2024-02-11\", \"env\": \"production\", \"batch_size\": 5000}",
       "dagRunId": "manual_20240211_160000"
     }'
```

**Response**:
```json
{
  "dagRunId": "manual_20240211_160000"
}
```

### Start a DAG with Singleton Mode
```bash
curl -X POST "http://localhost:8080/api/v2/dags/critical-job/start" \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer your-token" \
     -d '{
       "singleton": true,
       "params": "{\"priority\": \"high\"}"
     }'
```

**Response if DAG is not running (200)**:
```json
{
  "dagRunId": "20240211_161500_xyz789"
}
```

**Response if DAG is already running (409)**:
```json
{
  "code": "already_running",
  "message": "DAG critical-job is already running, cannot start in singleton mode"
}
```

### Check DAG Run Status
```bash
curl "http://localhost:8080/api/v2/dag-runs/data-processing-pipeline/latest" \
     -H "Authorization: Bearer your-token"
```

**Response**:
```json
{
  "dagRunDetails": {
    "rootDAGRunName": "data_processing_pipeline",
    "rootDAGRunId": "20240211_160000_current",
    "parentDAGRunName": "",
    "parentDAGRunId": "",
    "dagRunId": "20240211_160000_current",
    "name": "data_processing_pipeline",
    "status": 1,
    "statusLabel": "running",
    "queuedAt": "",
    "startedAt": "2024-02-11T16:00:00Z",
    "finishedAt": "",
    "log": "/logs/data_processing_pipeline/20240211_160000_current.log",
    "params": "{\"date\": \"2024-02-11\", \"env\": \"production\", \"batch_size\": 5000}",
    "nodes": [
      {
        "step": {
          "name": "extract_data",
          "id": "extract"
        },
        "status": 4,
        "statusLabel": "succeeded",
        "startedAt": "2024-02-11T16:00:30Z",
        "finishedAt": "2024-02-11T16:15:45Z",
        "retryCount": 0,
        "doneCount": 1
      },
      {
        "step": {
          "name": "transform_data",
          "id": "transform"
        },
        "status": 1,
        "statusLabel": "running",
        "startedAt": "2024-02-11T16:15:45Z",
        "finishedAt": "",
        "retryCount": 0,
        "doneCount": 0
      },
      {
        "step": {
          "name": "load_to_warehouse",
          "id": "load"
        },
        "status": 0,
        "statusLabel": "not_started",
        "startedAt": "",
        "finishedAt": "",
        "retryCount": 0,
        "doneCount": 0
      }
    ]
  }
}
```

### Search for DAGs
```bash
curl "http://localhost:8080/api/v2/dags/search?q=database" \
     -H "Authorization: Bearer your-token"
```

**Response**:
```json
{
  "results": [
    {
      "name": "database_backup",
      "dag": {
        "name": "database_backup",
        "group": "Operations",
        "schedule": [{"expression": "0 0 * * 0"}],
        "description": "Weekly database backup job",
        "params": ["target_db", "retention_days"],
        "defaultParams": "{\"retention_days\": 30}",
        "tags": ["backup", "weekly", "critical"]
      },
      "matches": [
        {
          "line": "    command: pg_dump ${target_db} | gzip > backup_$(date +%Y%m%d).sql.gz",
          "lineNumber": 25,
          "startLine": 20
        }
      ]
    }
  ],
  "errors": []
}
```

### Get Metrics for Monitoring
```bash
curl "http://localhost:8080/api/v2/metrics" | grep dagu_dag_runs_currently_running
```

### Stop a Running DAG
```bash
curl -X POST "http://localhost:8080/api/v2/dag-runs/data-pipeline/20240211_120000/stop" \
     -H "Authorization: Bearer your-token"
```

### Update Step Status Manually
```bash
# Mark a failed step as successful
curl -X PATCH "http://localhost:8080/api/v2/dag-runs/data-processing-pipeline/20240211_020000_def456/steps/transform_data/status" \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer your-token" \
     -d '{"status": 4}'
```

**Response (200)**: Success (empty response body)

### Enqueue a DAG Run
```bash
curl -X POST "http://localhost:8080/api/v2/dags/ml-training-pipeline/enqueue" \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer your-token" \
     -d '{
       "params": "{\"model\": \"recommendation_v3\", \"dataset\": \"user_interactions_2024\"}",
       "dagRunId": "ml_train_20240211_170000",
       "queue": "gpu-jobs"
     }'
```

**Response**:
```json
{
  "dagRunId": "ml_train_20240211_170000"
}
```

### Get Logs with Pagination
```bash
# Get last 100 lines of a DAG run log
curl "http://localhost:8080/api/v2/dag-runs/etl-pipeline/20240211_120000/log?tail=100"

# Get specific step's stderr output
curl "http://localhost:8080/api/v2/dag-runs/etl-pipeline/20240211_120000/steps/transform/log?stream=stderr"

# Get logs with offset and limit
curl "http://localhost:8080/api/v2/dag-runs/etl-pipeline/20240211_120000/log?offset=1000&limit=500"
```

### Working with Sub DAGs
```bash
# Get sub DAG run details
curl "http://localhost:8080/api/v2/dag-runs/data-processing-pipeline/20240211_140000_abc123/sub-dag-runs/sub_20240211_143020_xyz456" \
     -H "Authorization: Bearer your-token"

# Get sub DAG step log
curl "http://localhost:8080/api/v2/dag-runs/data-processing-pipeline/20240211_140000_abc123/sub-dag-runs/sub_20240211_143020_xyz456/steps/load_data/log" \
     -H "Authorization: Bearer your-token"

# Update sub DAG step status
curl -X PATCH "http://localhost:8080/api/v2/dag-runs/data-processing-pipeline/20240211_140000_abc123/sub-dag-runs/sub_20240211_143020_xyz456/steps/load_data/status" \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer your-token" \
     -d '{"status": 4}'
```

### Rename a DAG
```bash
curl -X POST "http://localhost:8080/api/v2/dags/old-pipeline-name/rename" \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer your-token" \
     -d '{"newFileName": "new-pipeline-name"}'
```

**Response (200)**: Success (empty response body)

### Delete a DAG
```bash
curl -X DELETE "http://localhost:8080/api/v2/dags/deprecated-pipeline" \
     -H "Authorization: Bearer your-token"
```

**Response (204)**: No content (successful deletion)

### Get DAG Specification YAML
```bash
curl "http://localhost:8080/api/v2/dags/data-processing-pipeline/spec" \
     -H "Authorization: Bearer your-token"
```

**Response**:
```json
{
  "dag": {
    "name": "data_processing_pipeline",
    "group": "ETL"
  },
  "spec": "name: data_processing_pipeline\ngroup: ETL\nschedule:\n  - \"0 2 * * *\"\n  - \"0 14 * * *\"\ndescription: Daily data processing pipeline for warehouse ETL\nenv:\n  - DATA_SOURCE=postgres://prod-db:5432/analytics\n  - WAREHOUSE_URL=${WAREHOUSE_URL}\nlogDir: /var/log/dagu/pipelines\nhistRetentionDays: 30\nmaxActiveRuns: 1\nmaxActiveSteps: 5\nparams:\n  - date\n  - env\n  - batch_size\ndefaultParams: |\n  batch_size: 1000\n  env: dev\ntags:\n  - production\n  - etl\n  - daily\npreconditions:\n  - condition: \"`date +%u`\"\n    expected: \"re:[1-5]\"\n    error: Pipeline only runs on weekdays\nsteps:\n  - name: extract_data\n    id: extract\n    description: Extract data from source database\n    dir: /app/etl\n    command: python\n    args:\n      - extract.py\n      - --date\n      - ${date}\n    stdout: /logs/extract.out\n    stderr: /logs/extract.err\n    output: EXTRACTED_FILE\n    preconditions:\n      - condition: test -f /data/ready.flag\n  - name: transform_data\n    id: transform\n    description: Apply transformations to extracted data\n    command: python transform.py --input=${EXTRACTED_FILE}\n    depends:\n      - extract_data\n    output: TRANSFORMED_FILE\n    mailOnError: true\n  - name: load_to_warehouse\n    id: load\n    run: warehouse-loader\n    params: |\n      file: ${TRANSFORMED_FILE}\n      table: fact_sales\n    depends:\n      - transform_data\nhandlerOn:\n  success:\n    command: notify.sh 'Pipeline completed successfully'\n  failure:\n    command: alert.sh 'Pipeline failed' high\n  exit:\n    command: cleanup_temp_files.sh\n",
  "errors": []
}
```

### Complex Filtering Examples
```bash
# Get all failed DAG runs in the last 24 hours
curl "http://localhost:8080/api/v2/dag-runs?status=2&fromDate=$(date -d '24 hours ago' +%s)" \
     -H "Authorization: Bearer your-token"

# Get DAG runs for a specific DAG with pagination
curl "http://localhost:8080/api/v2/dag-runs?name=data-processing-pipeline&page=2&perPage=20" \
     -H "Authorization: Bearer your-token"

# Search for DAGs with specific tags
curl "http://localhost:8080/api/v2/dags?tag=production&page=1&perPage=50" \
     -H "Authorization: Bearer your-token"

# Get running DAG runs
curl "http://localhost:8080/api/v2/dag-runs?status=1" \
     -H "Authorization: Bearer your-token"

# Get queued DAG runs
curl "http://localhost:8080/api/v2/dag-runs?status=5" \
     -H "Authorization: Bearer your-token"

# View all execution queues with running and queued DAG runs
curl "http://localhost:8080/api/v2/queues" \
     -H "Authorization: Bearer your-token"
```

### Suspend/Resume DAG Scheduling
```bash
# Suspend a DAG
curl -X POST "http://localhost:8080/api/v2/dags/data-processing-pipeline/suspend" \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer your-token" \
     -d '{"suspend": true}'

# Resume a DAG
curl -X POST "http://localhost:8080/api/v2/dags/data-processing-pipeline/suspend" \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer your-token" \
     -d '{"suspend": false}'
```

**Response (200)**: Success (empty response body)

## API Response Status Codes Summary

| Status Code | Description | Common Scenarios |
|-------------|-------------|------------------|
| 200 | Success | Successful GET, POST, PUT, PATCH requests |
| 201 | Created | New DAG created successfully |
| 204 | No Content | Successful DELETE operation |
| 400 | Bad Request | Invalid parameters, malformed JSON, invalid DAG name format |
| 401 | Unauthorized | Missing or invalid authentication token |
| 403 | Forbidden | Insufficient permissions (e.g., no write access) |
| 404 | Not Found | DAG, DAG run, or resource doesn't exist |
| 409 | Conflict | Resource already exists (e.g., DAG name conflict) |
| 500 | Internal Error | Server-side processing error |
| 503 | Service Unavailable | Server unhealthy or scheduler not responding |

## Workers Endpoints

### List Workers

**Endpoint**: `GET /api/v2/workers`

Retrieves information about connected workers in the distributed execution system.

**Query Parameters**:
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| remoteNode | string | Remote node name | "local" |

**Response (200)**:
```json
{
  "workers": [
    {
      "id": "worker-gpu-01",
      "labels": {
        "gpu": "true",
        "cuda": "11.8",
        "memory": "64G",
        "region": "us-east-1"
      },
      "health_status": "WORKER_HEALTH_STATUS_HEALTHY",
      "last_heartbeat": "2024-02-11T12:00:00Z",
      "running_tasks": [
        {
          "dagName": "ml-training-pipeline",
          "dagRunId": "20240211_120000_abc123",
          "rootDagRunName": "ml-training-pipeline",
          "rootDagRunId": "20240211_120000_abc123",
          "parentDagRunName": "",
          "parentDagRunId": "",
          "startedAt": "2024-02-11T12:00:00Z"
        }
      ]
    },
    {
      "id": "worker-cpu-02",
      "labels": {
        "cpu-arch": "amd64",
        "cpu-cores": "32",
        "region": "us-east-1"
      },
      "health_status": "WORKER_HEALTH_STATUS_WARNING",
      "last_heartbeat": "2024-02-11T11:59:50Z",
      "running_tasks": []
    },
    {
      "id": "worker-eu-01",
      "labels": {
        "region": "eu-west-1",
        "compliance": "gdpr"
      },
      "health_status": "WORKER_HEALTH_STATUS_UNHEALTHY",
      "last_heartbeat": "2024-02-11T11:59:30Z",
      "running_tasks": [
        {
          "dagName": "data-processor",
          "dagRunId": "20240211_113000_def456",
          "rootDagRunName": "data-pipeline",
          "rootDagRunId": "20240211_110000_xyz789",
          "parentDagRunName": "data-pipeline",
          "parentDagRunId": "20240211_110000_xyz789",
          "startedAt": "2024-02-11T11:30:00Z"
        }
      ]
    }
  ]
}
```

**Worker Health Status Values**:
- `WORKER_HEALTH_STATUS_HEALTHY`: Last heartbeat < 5 seconds ago
- `WORKER_HEALTH_STATUS_WARNING`: Last heartbeat 5-15 seconds ago
- `WORKER_HEALTH_STATUS_UNHEALTHY`: Last heartbeat > 15 seconds ago

**Running Task Fields**:
- `dagName`: Name of the DAG being executed
- `dagRunId`: ID of the current DAG run
- `rootDagRunName`: Name of the root DAG (for nested workflows)
- `rootDagRunId`: ID of the root DAG run
- `parentDagRunName`: Name of the immediate parent DAG (empty for root DAGs)
- `parentDagRunId`: ID of the immediate parent DAG run
- `startedAt`: When the task execution started

**Error Response (503)** (when coordinator is not running):
```json
{
  "code": "service_unavailable",
  "message": "Coordinator service is not available"
}
```

## API Versioning

- Current version: v2
- Legacy v1 endpoints are deprecated but still available
- Version is included in the URL path: `/api/v2/`
- Breaking changes will result in a new API version

## Remote Node Support

Most endpoints support the `remoteNode` query parameter for multi-environment setups:

```bash
# Query a remote node
curl "http://localhost:8080/api/v2/dags?remoteNode=production" \
     -H "Authorization: Bearer your-token"
```

Remote nodes are configured in the server configuration file and allow managing DAGs across multiple Dagu instances from a single interface.
