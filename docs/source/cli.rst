Command Line Interface
======================

- ``dagu start [--params=<params>] <file>`` - Runs the DAG
- ``dagu status <file>`` - Displays the current status of the DAG
- ``dagu retry --req=<request-id> <file>`` - Re-runs the specified DAG run
- ``dagu stop <file>`` - Stops the DAG execution by sending TERM signals
- ``dagu restart <file>`` - Restarts the current running DAG
- ``dagu dry [--params=<params>] <file>`` - Dry-runs the DAG
- ``dagu server [--host=<host>] [--port=<port>] [--dags=<path/to/the DAGs directory>]`` - Launches the Dagu web UI server
- ``dagu scheduler [--dags=<path/to/the DAGs directory>]`` - Starts the scheduler process
- ``dagu version`` - Shows the current binary version

For example:

.. code-block:: bash

   dagu server --config=~/.dagu/dev.yaml
   dagu scheduler --config=~/.dagu/dev.yaml