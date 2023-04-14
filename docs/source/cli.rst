Command Line Interface
======================

The following commands are available for interacting with Dagu:

- ``dagu start [--params=<params>] <file>``: Runs the DAG.
- ``dagu status <file>``: Displays the current status of the DAG.
- ``dagu retry --req=<request-id> <file>``: Re-runs the specified DAG run.
- ``dagu stop <file>``: Stops the DAG execution by sending TERM signals.
- ``dagu restart <file>``: Restarts the current running DAG.
- ``dagu dry [--params=<params>] <file>``: Dry-runs the DAG.
- ``dagu server``: Launches the Dagu web UI server.
- ``dagu scheduler``: Starts the scheduler process.
- ``dagu version``: Shows the current binary version.
