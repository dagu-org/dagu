.. _schema-reference:

Schema Reference
================

.. contents::
   :local:

Introduction
------------
Dagu uses YAML files to define Directed Acyclic Graphs (DAGs). This page lists all supported fields in the Dagu schema and explains how each field is used. For hands-on guides, see :ref:`Yaml Format <Yaml Format>`.

The official schema definition is published here:

- JSON Schema URL: 
  `dag.schema.json <https://github.com/dagu-org/dagu/blob/main/schemas/dag.schema.json>`__

You can use a YAML language server with the schema URL to get IDE auto-completion and validation:

.. code-block:: yaml

  # yaml-language-server: $schema=https://raw.githubusercontent.com/dagu-org/dagu/main/schemas/dag.schema.json
  name: MyDAG
  steps:
    - name: step_1
      command: echo "hello"

------------

DAG-Level Fields
----------------
These fields apply to the entire DAG. They appear at the root of the YAML file.

- **name** (string, optional):
  
  The name of the DAG. If omitted, Dagu defaults the name to the YAML filename without the extension.
  
  **Example**:

  .. code-block:: yaml

    name: My Daily DAG

- **description** (string, optional):

  A short description of what the DAG does.

  **Example**:

  .. code-block:: yaml

    description: This DAG processes daily data and sends notifications.

- **schedule** (string, optional):

  A cron expression (``* * * * *``) that determines how often the DAG runs.  
  If omitted, the DAG will only run manually (unless triggered via CLI or another mechanism).

  **Example**:

  .. code-block:: yaml

    schedule: "5 4 * * *"  # runs daily at 04:05

- **skipIfSuccessful** (boolean, default: false):

  If true, Dagu checks whether this DAG has already succeeded since the last scheduled time. If it did, Dagu will skip the current scheduled run. Manual triggers always run regardless of this setting.

  **Example**:

  .. code-block:: yaml

    skipIfSuccessful: true

- **group** (string, optional):

  An organizational label you can use to group DAGs (e.g., "DailyJobs", "Analytics").

- **tags** (string, optional):

  A comma-separated list of tags. Useful for searching, grouping, or labeling runs (e.g., "finance, daily").

- **env** (list of key-value, optional):

  Environment variables available to all steps in the DAG. These can use shell expansions, references to other environment variables, or command substitutions.

  **Example**:

  .. code-block:: yaml

    env:
      - LOG_DIR: ${HOME}/logs
      - PATH: /usr/local/bin:${PATH}

- **logDir** (string, default: ``${HOME}/.local/share/logs``):

  The base directory in which logs for this DAG are stored.

- **restartWaitSec** (integer, optional):

  Number of seconds to wait before restarting a failed or stopped DAG. Typically used with a process supervisor.

- **histRetentionDays** (integer, optional):

  How many days of historical run data to retain for this DAG. After this period, older run logs/history can be purged.

- **timeoutSec** (integer, optional):

  Maximum number of seconds for the entire DAG to finish. If the DAG hasn’t finished after this time, it’s considered timed out.

- **delaySec** (integer, optional):

  Delay (in seconds) before starting each step in a DAG run. This can be useful to stagger workloads.

- **maxActiveRuns** (integer, optional):

  Limit on how many runs of this DAG can be active at once (especially relevant if the DAG has a frequent schedule).

- **params** (string or list of key-value, optional):

  Default parameters for the entire DAG, either positional or named. Steps can reference these as environment variables (``$1, $2, ...`` for positional or ``$KEY`` for named).

  **Example (positional)**:

  .. code-block:: yaml

    params: param1 param2

  **Example (named)**:

  .. code-block:: yaml

    params:
      - FOO: 1
      - BAR: "`echo 2`"

- **preconditions** (list of condition blocks, optional):

  A list of conditions that must be satisfied before the DAG can run. Each condition can use shell expansions or command substitutions to validate external states.

  **Example**:

  .. code-block:: yaml

    preconditions:
      - condition: "`echo $2`" 
        expected: "param2"

- **mailOn** (dictionary, optional):

  Email notifications at DAG-level events, such as ``failure`` or ``success``. Also supports ``cancel`` and ``exit``.

  **Example**:

  .. code-block:: yaml

    mailOn:
      failure: true
      success: false

- **MaxCleanUpTimeSec** (integer, optional):

  Maximum number of seconds Dagu will spend cleaning up (stopping steps, finalizing logs, etc.) before forcing shutdown.

- **handlerOn** (dictionary, optional):

  Lifecycle event hooks at the DAG level. For each event (``success``, ``failure``, ``cancel``, ``exit``), you can run an additional command or script.

  **Example**:

  .. code-block:: yaml

    handlerOn:
      success:
        command: echo "succeeded!"
      failure:
        command: echo "failed!"
      cancel:
        command: echo "canceled!"
      exit:
        command: echo "all done!"

- **steps** (list of step objects, required):

  A list of steps (tasks) to execute. Steps define your workflow logic and can depend on each other. See :ref:`Step Fields <step-fields>` below for details.


Example DAG-Level Config
~~~~~~~~~~~~~~~~~~~~~~~~

.. code-block:: yaml

    name: DAG name
    description: run a DAG               
    schedule: "0 * * * *"                
    group: DailyJobs                     
    tags: example                        
    env:                                 
      - LOG_DIR: ${HOME}/logs
      - PATH: /usr/local/bin:${PATH}
    logDir: ${LOG_DIR}                   
    restartWaitSec: 60                   
    histRetentionDays: 3
    timeoutSec: 3600
    delaySec: 1                          
    maxActiveRuns: 1                     
    params: param1 param2                
    preconditions:                       
      - condition: "`echo $2`"           
        expected: "param2"               
    mailOn:
      failure: true                      
      success: true                      
    MaxCleanUpTimeSec: 300               
    handlerOn:                           
      success:
        command: echo "succeed"          
      failure:
        command: echo "failed"           
      cancel:
        command: echo "canceled"         
      exit:
        command: echo "finished"         
    steps:
      - name: main
        command: echo "Hello!"

------------

.. _step-fields:

Step Fields
-----------
Each element in the top-level ``steps`` list has its own fields for customization. A step object looks like this:

- **name** (string, required):

  A unique identifier for the step within this DAG.

- **description** (string, optional):

  Brief description of what this step does.

- **dir** (string, optional):

  Working directory in which this step’s command or script is executed.

- **command** (string, optional if ``script`` is used; otherwise required):

  The command or executable to run for this step.  
  Examples include ``bash``, ``python``, or direct shell commands like ``echo hello``.

- **script** (string, optional):

  Multi-line inline script content that will be piped into the command.  
  If ``command`` is omitted, the script is executed with the system’s default shell.

- **stdout** (string, optional):

  Path to a file in which to store the standard output (STDOUT) of the step’s command.

- **stderr** (string, optional):

  Path to a file in which to store the standard error (STDERR) of the step’s command.

- **output** (string, optional):

  A variable name to store the command’s STDOUT contents. You can reuse this variable in subsequent steps.

- **signalOnStop** (string, optional):

  If you manually stop this step (e.g., via CLI), the signal that Dagu sends to kill the process (e.g., ``SIGINT``).

- **mailOn** (dictionary, optional):

  Email notifications at the step level (same structure as DAG-level ``mailOn``).

- **continueOn** (dictionary, optional):

  Controls how Dagu handles cases where the step is skipped or fails.  
  - **failure**: If true, continue the DAG even if this step fails.  
  - **skipped**: If true, continue the DAG even if preconditions cause this step to skip.

- **retryPolicy** (dictionary, optional):

  Defines automatic retries for this step when it fails.  
  - **limit** (integer): How many times to retry.  
  - **intervalSec** (integer): How many seconds to wait between retries.

- **repeatPolicy** (dictionary, optional):

  Allows repeating a step multiple times in a single run.  
  - **repeat** (boolean): Whether to repeat.  
  - **intervalSec** (integer): Interval in seconds between repeats.

- **preconditions** (list of condition blocks, optional):

  Conditions that must be met for this step to run. Each condition block has:
  - **condition** (string): A command or expression to evaluate.
  - **expected** (string): The expected output. If the output matches, the step runs; otherwise, it is skipped.

- **depends** (list of strings, optional):

  Names of other steps that must complete before this step can run.

- **run** (string, optional):

  Reference to another YAML file (sub workflow) to run at this step.  
  If present, the sub workflow is executed in place of a command.

- **params** (string or list of key-value, optional):

  Parameters to pass into a sub workflow if this step references one (via ``run``). If you’re just using ``command``, you can also treat these as environment variables for this step.

Example Step Config
~~~~~~~~~~~~~~~~~~
.. code-block:: yaml

  steps:
    - name: complete example
      description: demonstrates all fields
      dir: ${HOME}/logs
      command: bash
      stdout: /tmp/outfile
      output: RESULT_VARIABLE
      script: |
        echo "any script"
      signalOnStop: "SIGINT"
      mailOn:
        failure: true
        success: true
      continueOn:
        failure: true
        skipped: true
      retryPolicy:
        limit: 2
        intervalSec: 5
      repeatPolicy:
        repeat: true
        intervalSec: 60
      preconditions:
        - condition: "`echo $1`"
          expected: "param1"
      depends:
        - other_step_name
      run: sub_dag.yaml
      params: "FOO=BAR"

------------

Additional Constructs
---------------------

Parameters
~~~~~~~~~~
Dagu supports both positional and named parameters at the DAG level. Steps can then override or add parameters. Access them in commands/scripts as environment variables.

.. code-block:: yaml

  params: param1 param2

  steps:
    - name: example
      command: echo "First param: $1, second param: $2"

Or with named parameters:

.. code-block:: yaml

  params:
    - FOO: 1
    - BAR: "`echo 2`"

  steps:
    - name: named example
      command: echo "FOO is ${FOO}, BAR is ${BAR}"

Preconditions
~~~~~~~~~~~~~
You can define preconditions at both DAG and step levels. Each precondition runs a shell expression and checks if its output matches an ``expected`` string. If it doesn’t match, the DAG or step is skipped (unless otherwise controlled by ``continueOn``).

Retry Policy
~~~~~~~~~~~~
Define how many times a failing step should retry, plus a wait interval:

.. code-block:: yaml

  retryPolicy:
    limit: 3
    intervalSec: 5

Repeat Policy
~~~~~~~~~~~~~
Run the same step multiple times in a single DAG run, with a configurable delay between repeats:

.. code-block:: yaml

  repeatPolicy:
    repeat: true
    intervalSec: 60  # run every minute

Sub-Worfklows
~~~~~~~~~~~~~~~
Use the ``run`` field within a step to call another YAML file. This helps organize large workflows. You can pass parameters:

.. code-block:: yaml

  steps:
    - name: sub workflow
      run: sub_dag.yaml
      params: FOO=BAR

Lifecycle Hooks (handlerOn)
~~~~~~~~~~~~~~~~~~~~~~~~~~~
React to DAG-wide events like success, failure, cancel, and exit:

.. code-block:: yaml

  handlerOn:
    success:
      command: echo "DAG succeeded!"
    failure:
      command: echo "DAG failed!"
    exit:
      command: echo "DAG exited!"

Global Configuration
--------------------
You can place global defaults in ``$HOME/.config/dagu/base.yaml``. This file can contain:

- Default environment variables
- Email notification settings
- A global ``logDir``
- Common organizational patterns

Example:

.. code-block:: yaml

  # $HOME/.config/dagu/base.yaml
  logDir: /var/log/dagu
  env:
    - GLOBAL_VAR: "HelloFromGlobalConfig"
  mailOn:
    success: true
    failure: true

