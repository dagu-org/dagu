.. _Special Environment Variables:

Special Environment Variables
==============================

.. contents::
    :local:

Inside a DAG, you can use the following environment variables to access special values:

- ``DAG_NAME``: The name of the current DAG.
- ``DAG_RUN_STEP_NAME``: The name of the current step.
- ``DAG_RUN_ID``: The unique ID for the current execution request.
- ``DAG_RUN_LOG_FILE``: The path to the log file for the scheduler.
- ``DAG_RUN_STEP_STDOUT_FILE``: The path to the log file for the current step's stdout output.
- ``DAG_RUN_STEP_STDERR_FILE``: The path to the log file for the current step's stderr output.

Example Usage
~~~~~~~~~~~~~

.. code-block:: yaml

  name: special-envs
  steps:
  - name: print values
    command: bash
    script: |
      echo DAG_NAME=$DAG_NAME
      echo DAG_RUN_ID=$DAG_RUN_ID
      echo DAG_RUN_LOG_FILE=$DAG_RUN_LOG_FILE
      echo DAG_RUN_STEP_STDOUT_FILE=$DAG_RUN_STEP_STDOUT_FILE
      echo DAG_RUN_STEP_STDERR_FILE=$DAG_RUN_STEP_STDERR_FILE

**Example Output**

.. code-block:: bash

  DAG_NAME=special-envs
  DAG_RUN_ID=0cf64f67-a1d6-4764-b5e0-0ea92c3089e2
  DAG_RUN_LOG_FILE=/path/to/logs/special-envs/step1.20241001.22:31:29.167.0cf64f67.log
  DAG_RUN_STEP_STDOUT_FILE=/path/to/logs/special-envs/start_special-envs.20241001.22:31:29.163.0cf64f67.out
  DAG_RUN_STEP_STDERR_FILE=/path/to/logs/special-envs/start_special-envs.20241001.22:31:29.163.0cf64f67.err
