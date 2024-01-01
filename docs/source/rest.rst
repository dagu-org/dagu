.. _REST API:

REST API Docs
=============

Dagu server provides simple APIs to query and control workflows.

See the `OpenAPI Schema for Dagu <https://github.com/dagu-dev/dagu/blob/main/swagger.yaml>`_ for more details.

**Endpoint** : `localhost:8080` (default)

**Required HTTP header** :
   ``Accept: application/json``

API Endpoints
-------------
This document provides information about the following endpoints:

Show DAGs `GET /api/v1/dags/`
---------------------

Return a list of available DAGs.

URL
  : ``/api/v1/dags/``

Method
  : ``GET``

Header
  : ``Accept: application/json``

Query Parameters:

- ``group=[string]`` where group is the subdirectory name that the DAG is in.

Success Response
~~~~~~~~~~~~~~~~~

Code: ``200 OK``

Response Body
~~~~~~~~~~~~~


Show Workflow Detail `GET /api/v1/dags/:name`
--------------------------------------

Return details about the specified workflow.

URL
  : ``/api/v1/dags/:name``

URL Parameters
  :name: [string] - Name of the DAG.

Method
  : ``GET``

Header
  : ``Accept: application/json``

Success Response
~~~~~~~~~~~~~~~~~

Code: ``200 OK``

Response Body
~~~~~~~~~~~~~

TBU


Show Workflow Spec `GET /api/v1/dags/:name/spec`
----------------------------------------

Return the specifications of the specified workflow.

URL
  : ``/api/v1/dags/:name/spec``

URL Parameters
  :name: [string] - Name of the DAG.

Method
  : ``GET``

Header
  : ``Accept: application/json``

Success Response
~~~~~~~~~~~~~~~~~

Code: ``200 OK``

Response Body
~~~~~~~~~~~~~

TBU


Submit Workflow Action `POST /api/v1/dags/:name`
----------------------------------------

Submit an action to a specified workflow.

URL
  : ``/api/v1/dags/:name``

URL Parameters
  :name: [string] - Name of the DAG.

Form Parameters
  :action: [string] - Specify 'start', 'stop', or 'retry'.
  :request-id: [string] - Required if action is 'retry'.
  :params: [string] - Parameters for the DAG execution.

Method
  : ``POST``

Success Response
~~~~~~~~~~~~~~~~~

Code: ``200 OK``

Response Body
~~~~~~~~~~~~~

TBU
