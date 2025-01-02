.. _cli:

Command Line Interface
======================

The following commands are available for interacting with Dagu:

.. code-block:: sh

  # Runs the DAG
  dagu start <file>
  
  # Runs the DAG with named parameters
  dagu start <file> [-- <key>=<value> ...]
  
  # Runs the DAG with positional parameters
  dagu start <file> [-- value1 value2 ...]
  
  # Displays the current status of the DAG
  dagu status <file>
  
  # Re-runs the specified DAG run
  dagu retry --req=<request-id> <file>
  
  # Stops the DAG execution
  dagu stop <file>
  
  # Restarts the current running DAG
  dagu restart <file>
  
  # Dry-runs the DAG
  dagu dry <file> [-- <key>=<value> ...]
  
  # Launches both the web UI server and scheduler process
  dagu start-all [--host=<host>] [--port=<port>] [--dags=<path to directory>]
  
  # Launches the Dagu web UI server
  dagu server [--host=<host>] [--port=<port>] [--dags=<path to directory>]
  
  # Starts the scheduler process
  dagu scheduler [--dags=<path to directory>]
  
  # Shows the current binary version
  dagu version