.. _Special Environment Variables:

Special Environment Variables
==============================

.. contents::
    :local:

Inside a DAG, you can use the following environment variables to access special values:

- ``DAG_EXECUTION_LOG_PATH``: The path to the log file for the current step.
- ``DAG_SCHEDULER_LOG_PATH``: The path to the log file for the scheduler.
- ``DAG_REQUEST_ID``: The unique ID for the current execution request.

Example Usage
~~~~~~~~~~~~~

.. code-block:: yaml

  steps:
  - name: print values
    command: bash
    script: |
      echo DAG_EXECUTION_LOG_PATH=$DAG_EXECUTION_LOG_PATH
      echo DAG_SCHEDULER_LOG_PATH=$DAG_SCHEDULER_LOG_PATH
      echo DAG_REQUEST_ID=$DAG_REQUEST_ID

**Example Output**

.. code-block:: bash

  DAG_EXECUTION_LOG_PATH=/path/to/logs/special-envs/step1.20241001.22:31:29.167.0cf64f67.log
  DAG_SCHEDULER_LOG_PATH=/path/to/logs/special-envs/start_special-envs.20241001.22:31:29.163.0cf64f67.log
  DAG_REQUEST_ID=0cf64f67-a1d6-4764-b5e0-0ea92c3089e2
