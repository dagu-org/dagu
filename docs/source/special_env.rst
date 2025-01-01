.. _Special Environment Variables:

Special Environment Variables
==============================

.. contents::
    :local:

Inside a DAG, you can use the following environment variables to access special values:

- ``DAG_NAME``: The name of the current DAG.
- ``DAG_STEP_NAME``: The name of the current step.
- ``DAG_REQUEST_ID``: The unique ID for the current execution request.
- ``DAG_EXECUTION_LOG_PATH``: The path to the log file for the current step.
- ``DAG_STEP_LOG_PATH``: The path to the log file for the scheduler.

Example Usage
~~~~~~~~~~~~~

.. code-block:: yaml

  name: special-envs
  steps:
  - name: print values
    command: bash
    script: |
      echo DAG_NAME=$DAG_NAME
      echo DAG_REQUEST_ID=$DAG_REQUEST_ID
      echo DAG_EXECUTION_LOG_PATH=$DAG_EXECUTION_LOG_PATH
      echo DAG_STEP_LOG_PATH=$DAG_STEP_LOG_PATH

**Example Output**

.. code-block:: bash

  DAG_NAME=special-envs
  DAG_REQUEST_ID=0cf64f67-a1d6-4764-b5e0-0ea92c3089e2
  DAG_EXECUTION_LOG_PATH=/path/to/logs/special-envs/step1.20241001.22:31:29.167.0cf64f67.log
  DAG_STEP_LOG_PATH=/path/to/logs/special-envs/start_special-envs.20241001.22:31:29.163.0cf64f67.log
