.. _REST API:

REST API Documentation (v1)
===========================

Overview
--------

Dagu server provides a comprehensive REST API for querying and controlling DAGs. The API enables programmatic control over workflow orchestration, including DAG management, execution control, monitoring, and system operations.

Base Configuration
----------------

**Base URL**
    ``http://localhost:8080/api/v1``

**Content Types**
    - Request: ``application/json``
    - Response: ``application/json``

**Required Headers**
    - For all requests: ``Accept: application/json``
    - For POST/PUT requests: ``Content-Type: application/json``

Authentication
    Currently, the API does not require authentication.

System Operations
---------------

Health Check ``GET /health``
~~~~~~~~~~~~~~~~~~~~~~~~~~

Checks the health status of the Dagu server and its dependencies.

**URL**
    ``/health``

**Method**
    ``GET``

**Parameters**
    None

**Success Response (200)**

.. code-block:: json

    {
        "status": "healthy",
        "version": "1.0.0",
        "uptime": 3600,
        "timestamp": "2024-02-11T12:00:00Z"
    }

.. list-table:: Response Fields
   :widths: 20 80
   :header-rows: 1

   * - Field
     - Description
   * - status
     - Server health status ("healthy" or "unhealthy")
   * - version
     - Current server version
   * - uptime
     - Server uptime in seconds
   * - timestamp
     - Current server time in ISO 8601 format

**Error Response (503)**

.. code-block:: json

    {
        "status": "unhealthy",
        "version": "1.0.0",
        "uptime": 3600,
        "timestamp": "2024-02-11T12:00:00Z"
    }

DAG Operations
------------

List DAGs ``GET /dags``
~~~~~~~~~~~~~~~~~~~~~

Retrieves a paginated list of available DAGs with optional filtering capabilities.

**URL**
    ``/dags``

**Method**
    ``GET``

.. list-table:: Query Parameters
   :widths: 20 15 50 15
   :header-rows: 1

   * - Parameter
     - Type
     - Description
     - Required
   * - page
     - integer
     - Page number for pagination
     - No
   * - limit
     - integer
     - Number of items per page
     - No
   * - searchName
     - string
     - Filter DAGs by matching name
     - No
   * - searchTag
     - string
     - Filter DAGs by matching tag
     - No

**Success Response (200)**

.. code-block:: json

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
                "Error": "",
            }
        ],
        "Errors": [],
        "HasError": false,
        "PageCount": 1
    }

**Response Fields Description**

DAG Object:
    - ``File``: Path to the DAG definition file
    - ``Dir``: Directory containing the DAG file
    - ``DAG``: DAG configuration and metadata
    - ``Status``: Current execution status
    - ``Suspended``: Whether the DAG is suspended
    - ``Error``: Error message if any

Create DAG ``POST /dags``
~~~~~~~~~~~~~~~~~~~~~~

Creates a new DAG definition.

**URL**
    ``/dags``

**Method**
    ``POST``

**Request Body**

.. code-block:: json

    {
        "action": "create",
        "value": "dag_definition_yaml_content"
    }

.. list-table:: Request Fields
   :widths: 20 15 50 15
   :header-rows: 1

   * - Field
     - Type
     - Description
     - Required
   * - action
     - string
     - Action to perform upon creation
     - Yes
   * - value
     - string
     - DAG definition in YAML format
     - Yes

**Success Response (200)**

.. code-block:: json

    {
        "DagID": "new_dag_123"
    }

Get DAG Details ``GET /dags/{dagId}``
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Retrieves detailed information about a specific DAG.

**URL**
    ``/dags/{dagId}``

**Method**
    ``GET``

.. list-table:: URL Parameters
   :widths: 20 15 50 15
   :header-rows: 1

   * - Parameter
     - Type
     - Description
     - Required
   * - dagId
     - string
     - Unique identifier of the DAG
     - Yes

.. list-table:: Query Parameters
   :widths: 20 15 50 15
   :header-rows: 1

   * - Parameter
     - Type
     - Description
     - Required
   * - tab
     - string
     - Tab name for UI navigation
     - No
   * - file
     - string
     - Specific file related to the DAG
     - No
   * - step
     - string
     - Step name within the DAG
     - No

Perform DAG Action ``POST /dags/{dagId}``
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~


Executes various actions on a specific DAG.

**URL**
    ``/dags/{dagId}``

**Method**
    ``POST``

.. list-table:: URL Parameters
   :widths: 20 15 50 15
   :header-rows: 1

   * - Parameter
     - Type
     - Description
     - Required
   * - dagId
     - string
     - Unique identifier of the DAG
     - Yes

**Request Body**

.. code-block:: json

    {
        "action": "string",
        "value": "string",
        "requestId": "string",
        "step": "string",
        "params": "string"
    }

.. list-table:: Request Fields
   :widths: 20 15 50 15
   :header-rows: 1

   * - Field
     - Type
     - Description
     - Required
   * - action
     - string
     - Action to perform (see Available Actions below)
     - Yes
   * - value
     - string
     - Additional value required by certain actions
     - No
   * - requestId
     - string
     - Required for retry, mark-success, and mark-failed actions
     - Conditional
   * - step
     - string
     - Required for mark-success and mark-failed actions
     - Conditional
   * - params
     - string
     - JSON string of parameters for DAG-run
     - No

Available Actions:
    - ``start``: Begin DAG-run
        - Requires: none
        - Optional: params
        - Fails if DAG is already running
    
    - ``suspend``: Toggle DAG suspension state
        - Requires: value ("true" or "false")
    
    - ``stop``: Stop DAG-run
        - Requires: none
        - Fails if DAG is not running
    
    - ``retry``: Retry a previous execution
        - Requires: requestId
    
    - ``mark-success``: Mark a specific step as successful
        - Requires: requestId, step
        - Fails if DAG is running
    
    - ``mark-failed``: Mark a specific step as failed
        - Requires: requestId, step
        - Fails if DAG is running
    
    - ``save``: Update DAG definition
        - Requires: value (new DAG definition)
    
    - ``rename``: Rename the DAG
        - Requires: value (new name)

**Success Response (200)**

.. code-block:: json

    {
        "newDagId": "string"
    }

.. note::
   The ``newDagId`` field is only included in the response for the ``rename`` action.

**Error Responses**

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

Search Operations
--------------

Search DAGs ``GET /search``
~~~~~~~~~~~~~~~~~~~~~~~

Performs a full-text search across DAG definitions.

**URL**
    ``/search``

**Method**
    ``GET``

.. list-table:: Query Parameters
   :widths: 20 15 50 15
   :header-rows: 1

   * - Parameter
     - Type
     - Description
     - Required
   * - q
     - string
     - Search query string
     - Yes

Error Handling
------------

All endpoints may return error responses in the following format:

.. code-block:: json

    {
        "code": "error_code",
        "message": "Human readable error message",
        "details": {
            "additional": "error details"
        }
    }

.. list-table:: Error Codes
   :widths: 25 75
   :header-rows: 1

   * - Code
     - Description
   * - validation_error
     - Invalid request parameters or body
   * - not_found
     - Requested resource doesn't exist
   * - internal_error
     - Server-side error
   * - unauthorized
     - Authentication/authorization failed
   * - bad_gateway
     - Upstream service error

Example Usage
-----------

.. code-block:: bash

    # Start a DAG with parameters
    curl -X POST "http://localhost:8080/api/v1/dags/example_dag" \
         -H "Content-Type: application/json" \
         -d '{
           "action": "start",
           "params": "{\"param1\": \"value1\"}"
         }'

    # Mark a step as successful
    curl -X POST "http://localhost:8080/api/v1/dags/example_dag" \
         -H "Content-Type: application/json" \
         -d '{
           "action": "mark-success",
           "requestId": "req_123",
           "step": "step1"
         }'

    # Rename a DAG
    curl -X POST "http://localhost:8080/api/v1/dags/example_dag" \
         -H "Content-Type: application/json" \
         -d '{
           "action": "rename",
           "value": "new_dag_name"
         }'

REST API Documentation (v2)
===========================

Overview
--------

Dagu server also provides a v2 REST API with additional endpoints for monitoring and enhanced functionality. The v2 API is available at ``/api/v2``.

Base Configuration
------------------

**Base URL**
    ``http://localhost:8080/api/v2``

**Content Types**
    - Request: ``application/json``
    - Response: ``application/json`` (except metrics endpoint)

Monitoring Operations
--------------------

Metrics Endpoint ``GET /metrics``
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Exposes Prometheus-compatible metrics for monitoring Dagu operations. This endpoint provides real-time insights into DAG executions, system health, and performance metrics.

**URL**
    ``/metrics``

**Method**
    ``GET``

**Parameters**
    None

**Success Response (200)**

.. code-block:: text

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

**Response Headers**
    - ``Content-Type: text/plain; version=0.0.4; charset=utf-8``

Available Metrics
~~~~~~~~~~~~~~~~

.. list-table:: System Metrics
   :widths: 30 20 50
   :header-rows: 1

   * - Metric Name
     - Type
     - Description
   * - dagu_info
     - gauge
     - Build information with version labels
   * - dagu_uptime_seconds
     - gauge
     - Time since server start in seconds
   * - dagu_scheduler_running
     - gauge
     - 1 if scheduler is running, 0 otherwise

.. list-table:: DAG Execution Metrics
   :widths: 30 20 50
   :header-rows: 1

   * - Metric Name
     - Type
     - Description
   * - dagu_dag_runs_currently_running
     - gauge
     - Number of DAG runs currently executing
   * - dagu_dag_runs_queued_total
     - gauge
     - Total number of DAG runs waiting in queue (all DAGs)
   * - dagu_dag_runs_total
     - counter
     - Total number of DAG runs by status (last 24 hours)
   * - dagu_dags_total
     - gauge
     - Total number of registered DAGs

.. note::
   - The ``dagu_dag_runs_total`` metric only includes DAG runs from the last 24 hours due to performance considerations.
   - Queue metrics (``dagu_dag_runs_queued_total``) count all queued items across all DAGs. Future versions may add per-DAG queue metrics.
   - The metrics endpoint is compatible with Prometheus scraping and can be used with standard Prometheus configurations.

Prometheus Configuration Example
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

To scrape Dagu metrics with Prometheus, add the following to your ``prometheus.yml``:

.. code-block:: yaml

    scrape_configs:
      - job_name: 'dagu'
        static_configs:
          - targets: ['localhost:8080']
        metrics_path: '/api/v2/metrics'
        scrape_interval: 15s

Grafana Dashboard
~~~~~~~~~~~~~~~~

You can visualize Dagu metrics in Grafana using queries like:

.. code-block:: promql

    # DAG execution success rate (last 24h)
    rate(dagu_dag_runs_total{status="success"}[5m]) / 
    rate(dagu_dag_runs_total[5m])
    
    # Average queue length
    avg_over_time(dagu_dag_runs_queued_total[5m])
    
    # Scheduler uptime percentage
    avg_over_time(dagu_scheduler_running[5m]) * 100

Example Usage
~~~~~~~~~~~~

.. code-block:: bash

    # Get raw metrics
    curl http://localhost:8080/api/v2/metrics
    
    # Check if scheduler is running
    curl -s http://localhost:8080/api/v2/metrics | grep "dagu_scheduler_running"
    
    # Get current running DAGs count
    curl -s http://localhost:8080/api/v2/metrics | grep "dagu_dag_runs_currently_running"
