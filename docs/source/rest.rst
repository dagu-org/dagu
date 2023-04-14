REST API Docs
=============

Dagu server provides simple APIs to query and control workflows.

**Endpoint** : `localhost:8080` (default)

**Required HTTP header** :
   ``Accept: application/json``

API Endpoints
-------------
This document provides information about the following endpoints:

Show DAGs `GET dags/`
---------------------

Return a list of available DAGs.

URL
  : ``/dags/``

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

TBU


Show Workflow Detail `GET dags/:name`
--------------------------------------

Return details about the specified workflow.

URL
  : ``/dags/:name``

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


Show Workflow Spec `GET dags/:name/spec`
----------------------------------------

Return the specifications of the specified workflow.

URL
  : ``/dags/:name/spec``

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


Submit Workflow Action `POST dags/:name`
----------------------------------------

Submit an action to a specified workflow.

URL
  : ``/dags/:name``

URL Parameters
  :name: [string] - Name of the DAG.

Form Parameters
  :action: [string] - Specify 'start', 'stop', or 'retry'.
  :request-id: [string] - Required if action is 'retry'.

Method
  : ``POST``

Success Response
~~~~~~~~~~~~~~~~~

Code: ``200 OK``

Response Body
~~~~~~~~~~~~~

TBU
