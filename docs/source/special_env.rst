.. _Special Environment Variables:

Special Environment Variables
==============================

.. contents::
    :local:

Inside a DAG, you can use the following environment variables to access special values:

- ``WORKFLOW_NAME``: The name of the current DAG.
- ``WORKFLOW_STEP_NAME``: The name of the current step.
- ``WORKFLOW_ID``: The unique ID for the current execution request.
- ``WORKFLOW_LOG_FILE``: The path to the log file for the scheduler.
- ``WORKFLOW_STEP_STDOUT_FILE``: The path to the log file for the current step's stdout output.
- ``WORKFLOW_STEP_STDERR_FILE``: The path to the log file for the current step's stderr output.

Example Usage
~~~~~~~~~~~~~

.. code-block:: yaml

  name: special-envs
  steps:
  - name: print values
    command: bash
    script: |
      echo WORKFLOW_NAME=$WORKFLOW_NAME
      echo WORKFLOW_ID=$WORKFLOW_ID
      echo WORKFLOW_LOG_FILE=$WORKFLOW_LOG_FILE
      echo WORKFLOW_STEP_STDOUT_FILE=$WORKFLOW_STEP_STDOUT_FILE
      echo WORKFLOW_STEP_STDERR_FILE=$WORKFLOW_STEP_STDERR_FILE

**Example Output**

.. code-block:: bash

  WORKFLOW_NAME=special-envs
  WORKFLOW_ID=0cf64f67-a1d6-4764-b5e0-0ea92c3089e2
  WORKFLOW_LOG_FILE=/path/to/logs/special-envs/step1.20241001.22:31:29.167.0cf64f67.log
  WORKFLOW_STEP_STDOUT_FILE=/path/to/logs/special-envs/start_special-envs.20241001.22:31:29.163.0cf64f67.out
  WORKFLOW_STEP_STDERR_FILE=/path/to/logs/special-envs/start_special-envs.20241001.22:31:29.163.0cf64f67.err
