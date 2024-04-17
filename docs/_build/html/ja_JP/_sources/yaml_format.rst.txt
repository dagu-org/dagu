.. _Yaml Format:

YAML Format
============

.. contents::
    :local:

Minimal DAG Definition
-----------------------

The minimal DAG definition is as simple as follows.

.. code-block:: yaml

  steps:
    - name: step 1
      command: echo hello
    - name: step 2
      command: echo world
      depends:
        - step 1

.. _specifying working dir:

Specifying Working Directory
------------------------------

.. code-block:: yaml

  steps:
    - name: step 1
      dir: /path/to/working/directory
      command: some command

Running Arbitrary Code Snippets
-------------------------------

``script`` field provides a way to run arbitrary snippets of code in any language.

.. code-block:: yaml

  steps:
    - name: step 1
      command: "bash"
      script: |
        cd /tmp
        echo "hello world" > hello
        cat hello
      output: RESULT
    - name: step 2
      command: echo ${RESULT} # hello world
      depends:
        - step 1

Defining Environment Variables
-------------------------------

You can define environment variables and refer to them using the ``env`` field.

.. code-block:: yaml

  env:
    - SOME_DIR: ${HOME}/batch
    - SOME_FILE: ${SOME_DIR}/some_file 
  steps:
    - name: some task in some dir
      dir: ${SOME_DIR}
      command: python main.py ${SOME_FILE}

Defining and Using Parameters
------------------------------

You can define parameters using the ``params`` field and refer to each parameter as $1, $2, etc. Parameters can also be command substitutions or environment variables. It can be overridden by the ``--params=`` parameter of the ``start`` command.

.. code-block:: yaml

  params: param1 param2
  steps:
    - name: some task with parameters
      command: python main.py $1 $2

Named parameters are also available as follows.

.. code-block:: yaml

  params: ONE=1 TWO=`echo 2`
  steps:
    - name: some task with parameters
      command: python main.py $ONE $TWO

Using Command Substitution
--------------------------

You can use command substitution in field values. I.e., a string enclosed in backquotes (`) is evaluated as a command and replaced with the result of standard output.

.. code-block:: yaml

  env:
    TODAY: "`date '+%Y%m%d'`"
  steps:
    - name: hello
      command: "echo hello, today is ${TODAY}"

Adding Conditional Logic
------------------------

Sometimes you have parts of a DAG that you only want to run under certain conditions. You can use the ``preconditions`` field to add conditional branches to your DAG.

For example, the task below only runs on the first date of each month.

.. code-block:: yaml

  steps:
    - name: A monthly task
      command: monthly.sh
      preconditions:
        - condition: "`date '+%d'`"
          expected: "01"

If you want the DAG to continue to the next step regardless of the step's conditional check result, you can use the ``continueOn`` field:

.. code-block:: yaml

  steps:
    - name: A monthly task
      command: monthly.sh
      preconditions:
        - condition: "`date '+%d'`"
          expected: "01"
      continueOn:
        skipped: true

User Defined Functions
-----------------------

You can define functions in the DAG file and call them in steps. The ``params`` field is required for functions. The ``args`` field is used to pass arguments to functions. The arguments can be command substitutions or environment variables.

.. code-block:: yaml

  functions:
    - name: my_function
      params: param1 param2
      command: python main.py $param1 $param2

  steps:
    - name: step 1
      call:
        function: my_function
        args:
          param1: 1
          param2: 2

Setting Environment Variables with Standard Output
---------------------------------------------------

The ``output`` field can be used to set an environment variable with standard output. Leading and trailing space will be trimmed automatically. The environment variables can be used in subsequent steps.

.. code-block:: yaml

  steps:
    - name: step 1
      command: "echo foo"
      output: FOO # will contain "foo"

Redirecting Stdout and Stderr
-----------------------------

The `stdout` field can be used to write standard output to a file.

.. code-block:: yaml

  steps:
    - name: create a file
      command: "echo hello"
      stdout: "/tmp/hello" # the content will be "hello\n"

The `stderr` field allows to redirect stderr to other file without writing to the normal log file.

.. code-block:: yaml

  steps:
    - name: output error file
      command: "echo error message >&2"
      stderr: "/tmp/error.txt"


Adding Lifecycle Hooks
----------------------

It is often desirable to take action when a specific event happens, for example, when a DAG fails. To achieve this, you can use `handlerOn` fields.

.. code-block:: yaml

  handlerOn:
    failure:
      command: notify_error.sh
    exit:
      command: cleanup.sh
  steps:
    - name: A task
      command: main.sh

Repeating a Task at Regular Intervals
-------------------------------------

If you want a task to repeat execution at regular intervals, you can use the `repeatPolicy` field. If you want to stop the repeating task, you can use the `stop` command to gracefully stop the task.

.. code-block:: yaml

  steps:
    - name: A task
      command: main.sh
      repeatPolicy:
        repeat: true
        intervalSec: 60

Scheduling a DAG with Cron Expression
--------------------------------------

You can use the `schedule` field to schedule a DAG with Cron expression.

.. code-block:: yaml

  schedule: "5 4 * * *" # Run at 04:05.
  steps:
    - name: scheduled job
      command: job.sh

See :ref:`scheduler configuration` for more details.

All Available Fields for DAGs
-------------------------------

This section provides a comprehensive list of available fields that can be used to configure DAGs and their steps in detail. Each field serves a specific purpose, enabling granular control over how the DAG runs. The fields include:

- ``name``: The name of the DAG, which is optional. The default name is the name of the file.
- ``description``: A brief description of the DAG.
- ``schedule``: The execution schedule of the DAG in Cron expression format.
- ``group``: The group name to organize DAGs, which is optional.
- ``tags``: Free tags that can be used to categorize DAGs, separated by commas.
- ``env``: Environment variables that can be accessed by the DAG and its steps.
- ``logDir``: The directory where the standard output is written. The default value is ``${DAGU_HOME}/logs/dags``.
- ``restartWaitSec``: The number of seconds to wait after the DAG process stops before restarting it.
- ``histRetentionDays``: The number of days to retain execution history (not for log files).
- ``delaySec``: The interval time in seconds between steps.
- ``maxActiveRuns``: The maximum number of parallel running steps.
- ``params``: The default parameters that can be referred to by ``$1``, ``$2``, and so on.
- ``preconditions``: The conditions that must be met before a DAG or step can run.
- ``mailOn``: Whether to send an email notification when a DAG or step fails or succeeds.
- ``MaxCleanUpTimeSec``: The maximum time to wait after sending a TERM signal to running steps before killing them.
- ``handlerOn``: The command to execute when a DAG or step succeeds, fails, cancels, or exits.
- ``steps``: A list of steps to execute in the DAG.

In addition, a global configuration file, ``$DAGU_HOME/config.yaml``, can be used to gather common settings, such as ``logDir`` or ``env``.

Note: If ``DAGU_HOME`` environment variable is not set, the default path is ``$HOME/.dagu/config.yaml``.

Example: 

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
        command: "echo succeed"          
      failure:
        command: "echo failed"           
      cancel:
        command: "echo canceled"         
      exit:
        command: "echo finished"         

All Available Fields for Steps
--------------------------------

Each step can have its own set of configurations, including:

- ``name``: The name of the step.
- ``description``: A brief description of the step.
- ``dir``: The working directory for the step.
- ``command``: The command and parameters to execute.
- ``stdout``: The file to which the standard output is written.
- ``output``: The variable to which the result is written.
- ``script``: The script to execute.
- ``signalOnStop``: The signal name (e.g., ``SIGINT``) to be sent when the process is stopped.
- ``mailOn``: Whether to send an email notification when the step fails or succeeds.
- ``continueOn``: Whether to continue to the next step, regardless of whether the step failed or not or the preconditions are met or not.
- ``retryPolicy``: The retry policy for the step.
- ``repeatPolicy``: The repeat policy for the step.
- ``preconditions``: The conditions that must be met before a step can run.
- ``depends``: The step depends on the other step.

Example:

.. code-block:: yaml

    steps:
      - name: some task                  
        description: some task           
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
          -  some task name step          