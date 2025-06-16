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
  
  # Re-runs the specified DAG-run
  dagu retry --run-id=<request-id> <file or dag-name>
  
  # Stops the DAG-run
  dagu stop <file>
  
  # Restarts the current running DAG
  dagu restart <file>
  
  # Dry-runs the DAG
  dagu dry <file> [-- <key>=<value> ...]
  
  # Enqueues a DAG-run to the queue
  dagu enqueue <file> [--run-id=<run-id>] [-- <key>=<value> ...]
  
  # Dequeues a DAG-run from the queue
  dagu dequeue --dag-run=<dag-name>:<run-id>
  
  # Launches both the web UI server and scheduler process
  dagu start-all [--host=<host>] [--port=<port>] [--dags=<path to directory>]
  
  # Launches the Dagu web UI server
  dagu server [--host=<host>] [--port=<port>] [--dags=<path to directory>]
  
  # Starts the scheduler process
  dagu scheduler [--dags=<path to directory>]
  
  # Shows the current binary version
  dagu version
