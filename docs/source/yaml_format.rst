.. _Yaml Format:

Writing DAGs
==========================

.. contents::
    :local:

Basics
--------

Minimal Example
~~~~~~~~~~~~~~~~

The minimal example of a DAG file is as follows:

.. code-block:: yaml

  steps:
    - name: step 1
      command: echo hello
    - name: step 2
      command: echo world
      depends:
        - step 1

The command can be string or list of strings. The list of strings is useful when you want to pass arguments to the command.

.. code-block:: yaml

  steps:
    - name: step 1
      command: [echo, hello]

.. _specifying working dir:

Schema Definition
~~~~~~~~~~~~~~~~~~

We have a schema definition for the DAG file. The schema definition is used to validate the DAG file. The schema definition is available at `dag.schema.json <https://github.com/daguflow/dagu/blob/main/schemas/dag.schema.json>`_. This schema can be used by IDEs to provide auto-completion and validation. I.e.,:

.. code-block:: yaml

  # yaml-language-server: $schema=https://raw.githubusercontent.com/daguflow/dagu/main/schemas/dag.schema.json
  steps:
    - name: step 1
      command: echo hello

Working Directory
~~~~~~~~~~~~~~~~~~

You can specify the working directory for each step using the ``dir`` field.

.. code-block:: yaml

  steps:
    - name: step 1
      dir: /path/to/working/directory
      command: some command

Code Snippet
~~~~~~~~~~~~~

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

Environment Variables
~~~~~~~~~~~~~~~~~~~~~~~

You can define environment variables using the ``env`` field. The environment variables can be accessed by the DAG and its steps.


.. code-block:: yaml

  env:
    - SOME_DIR: ${HOME}/batch
    - SOME_FILE: ${SOME_DIR}/some_file 
  steps:
    - name: some task in some dir
      dir: ${SOME_DIR}
      command: python main.py ${SOME_FILE}

Parameters
~~~~~~~~~~~

You can pass parameters to the DAG and its steps using the ``params`` field. The parameters can be accessed by the steps using ``$1``, ``$2``, and so on.

.. code-block:: yaml

  params: param1 param2
  steps:
    - name: some task with parameters
      command: python main.py $1 $2

Named Parameters
~~~~~~~~~~~~~~~~

You can also use named parameters in the ``params`` field. The named parameters can be accessed by the steps using ``${FOO}``, ``${BAR}``, and so on.

.. code-block:: yaml

  params: FOO=1 BAR=`echo 2`
  steps:
    - name: some task with parameters
      command: python main.py ${FOO} ${BAR}

Conditional Logic
~~~~~~~~~~~~~~~~~~

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

Capture Output
~~~~~~~~~~~~~~

The ``output`` field can be used to set an environment variable with standard output. Leading and trailing space will be trimmed automatically. The environment variables can be used in subsequent steps.

.. code-block:: yaml

  steps:
    - name: step 1
      command: "echo foo"
      output: FOO # will contain "foo"

Redirect Standard Output and Error
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

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

Retry Policy
~~~~~~~~~~~~~

You can set a retry policy for a step using the ``retryPolicy`` field. The step will be retried if it fails.

.. code-block:: yaml

  steps:
    - name: A task
      command: main.sh
      retryPolicy:
        limit: 3
        intervalSec: 5

Running Sub-DAG
~~~~~~~~~~~~~~~~

You can run a sub-DAG from a DAG file. The sub-DAG is defined in a separate file and can be called using the `run` field.

.. code-block:: yaml

  steps:
    - name: A task
      run: <DAG file name>  # e.g., sub_dag, sub_dag.yaml, /path/to/sub_dag.yaml
      params: "FOO=BAR"     # optional


Schedule
~~~~~~~~~~

You can use the `schedule` field to schedule a DAG with Cron expression.

.. code-block:: yaml

  schedule: "5 4 * * *" # Run at 04:05.
  steps:
    - name: scheduled job
      command: job.sh

See :ref:`scheduler configuration` for more details.

Executors
~~~~~~~~~~

The `executor` field allows you to handle different types of tasks such as running Docker containers, making HTTP requests, and executing commands over SSH.

Please see :ref:`executors` for more details.

Advanced
--------

Command Substitution
~~~~~~~~~~~~~~~~~~~~~~~~~~~

You can use command substitution in field values. I.e., a string enclosed in backquotes (`) is evaluated as a command and replaced with the result of standard output.

.. code-block:: yaml

  env:
    TODAY: "`date '+%Y%m%d'`"
  steps:
    - name: hello
      command: "echo hello, today is ${TODAY}"

Lifecycle Hooks
~~~~~~~~~~~~~~~~

It is often desirable to take action when a specific event happens, for example, when a DAG fails. To achieve this, you can use `handlerOn` fields.

.. code-block:: yaml

  handlerOn:
    success:
      command: echo "succeeded!"
    cancel:
      command: echo "cancelled!"
    failure:
      command: echo "failed!"
    exit:
      command: echo "exited!"
  steps:
    - name: step1
      command: echo hello


Repeat a Step
~~~~~~~~~~~~~~

If you want a task to repeat execution at regular intervals, you can use the `repeatPolicy` field. If you want to stop the repeating task, you can use the `stop` command to gracefully stop the task.

.. code-block:: yaml

  steps:
    - name: A task
      command: main.sh
      repeatPolicy:
        repeat: true
        intervalSec: 60

User Defined Functions
~~~~~~~~~~~~~~~~~~~~~~~

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


All Available Fields
--------------------

DAG
~~~~

This section provides a comprehensive list of available fields that can be used to configure DAGs and their steps in detail. Each field serves a specific purpose, enabling granular control over how the DAG runs. The fields include:

- ``name``: The name of the DAG, which is optional. The default name is the name of the file.
- ``description``: A brief description of the DAG.
- ``schedule``: The execution schedule of the DAG in Cron expression format.
- ``group``: The group name to organize DAGs, which is optional.
- ``tags``: Free tags that can be used to categorize DAGs, separated by commas.
- ``env``: Environment variables that can be accessed by the DAG and its steps.
- ``logDir``: The directory where the standard output is written. The default value is ``${HOME}/.local/share/logs``.
- ``restartWaitSec``: The number of seconds to wait after the DAG process stops before restarting it.
- ``histRetentionDays``: The number of days to retain execution history (not for log files).
- ``timeout``: The timeout of the DAG, which is optional. Unit is seconds.
- ``delaySec``: The interval time in seconds between steps.
- ``maxActiveRuns``: The maximum number of parallel running steps.
- ``params``: The default parameters that can be referred to by ``$1``, ``$2``, and so on.
- ``preconditions``: The conditions that must be met before a DAG or step can run.
- ``mailOn``: Whether to send an email notification when a DAG or step fails or succeeds.
- ``MaxCleanUpTimeSec``: The maximum time to wait after sending a TERM signal to running steps before killing them.
- ``handlerOn``: The command to execute when a DAG or step succeeds, fails, cancels, or exits.
- ``steps``: A list of steps to execute in the DAG.

In addition, a global configuration file, ``$HOME/.config/dagu/base.yaml``, can be used to gather common settings, such as ``logDir`` or ``env``.

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
    timeout: 3600
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

Step
~~~~

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
- ``run``: The sub-DAG to run.
- ``params``: The parameters to pass to the sub-DAG.

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
        run: sub_dag
        params: "FOO=BAR"
