.. _Yaml Format:

Writing DAGs
===========

.. contents::
    :local:

Introduction
------------
Dagu uses YAML files to define Directed Acyclic Graphs (DAGs) for workflow orchestration. This document covers everything you need to know about writing DAG definitions, from basic usage to advanced features.

Core Concepts
------------
Before diving into specific features, let's understand the basic structure of a DAG file and how steps are defined.

Hello World
~~~~~~~~~~~~

.. code-block:: yaml

  schedule: "* * * * *" # Run the DAG every minute
  params:
    - NAME: "Dagu"
  steps:
    - name: Hello world
      command: echo Hello $NAME
    - name: Done
      command: echo Done!
      depends: Hello world

Using pipes (``|``) in commands:

.. code-block:: yaml

  steps:
    - name: Hello world with pipe
      command: echo hello world | xargs echo

Specifying a shell:

.. code-block:: yaml

  steps:
    - name: Hello world with shell
      command: echo hello world | xargs echo
      shell: bash

Running a script:

.. code-block:: yaml

  steps:
    - name: Hello world with script
      command: bash
      script: |
        echo hello world
        echo goodbye world

Multiple dependencies:

.. code-block:: yaml

  steps:
    - name: step 1
      command: echo hello
    - name: step 2
      command: echo world
    - name: step 3
      command: echo hello world
      depends:
        - step 1
        - step 2

Define steps as map:

.. code-block:: yaml

  steps:
    step1:
      command: echo hello
    step2:
      command: echo world
    step3:
      command: echo hello world
      depends:
        - step1
        - step2

Schema Definition
~~~~~~~~~~~~~~~~
We provide a JSON schema to validate DAG files and enable IDE auto-completion:

.. code-block:: yaml

  # yaml-language-server: $schema=https://raw.githubusercontent.com/dagu-org/dagu/main/schemas/dag.schema.json
  steps:
    - name: step 1
      command: echo hello

The schema is available at `dag.schema.json <https://github.com/dagu-org/dagu/blob/main/schemas/dag.schema.json>`_.

Working Directory
~~~~~~~~~~~~~~~
Control where each step executes:

.. code-block:: yaml

  steps:
    - name: step 1
      dir: /path/to/working/directory
      command: some command

Basic Features
-------------

Environment Variables
~~~~~~~~~~~~~~~~~~~
Define variables accessible throughout the DAG:

.. code-block:: yaml

  env:
    - SOME_DIR: ${HOME}/batch
    - SOME_FILE: ${SOME_DIR}/some_file 
  steps:
    - name: task
      dir: ${SOME_DIR}
      command: python main.py ${SOME_FILE}

Dotenv Files
~~~~~~~~~~~
Specify candidate ``.env`` files to load environment variables from. By default, no env files are loaded unless explicitly specified.

.. code-block:: yaml

  dotenv: .env  # Specify a candidate dotenv file

  # Or specify multiple candidate files
  dotenv:
    - .env
    - .env.local
    - configs/.env.prod

Files can be specified as:

- Absolute paths
- Relative to the DAG file directory
- Relative to the base config directory
- Relative to the user's home directory

Parameters
~~~~~~~~~~
Define default positional parameters that can be overridden:

.. code-block:: yaml

  params: param1 param2  # Default values for $1 and $2
  steps:
    - name: parameterized task
      command: python main.py $1 $2      # Will use command-line args or defaults

Named Parameters
~~~~~~~~~~~~~~
Define default named parameters that can be overridden:

.. code-block:: yaml

  params:
    - FOO: 1           # Default value for ${FOO}
    - BAR: "`echo 2`"  # Default value for ${BAR}, using command substitution
  steps:
    - name: named params task
      command: python main.py ${FOO} ${BAR}  # Will use command-line args or defaults

Code Snippets
~~~~~~~~~~~~

Run shell script with `$SHELL`:

.. code-block:: yaml

  steps:
    - name: script step
      script: |
        cd /tmp
        echo "hello world" > hello
        cat hello

You can run arbitrary script with the `script` field. The script will be executed with the program specified in the `command` field. If `command` is not specified, the default shell will be used.

.. code-block:: yaml

  steps:
    - name: script step
      command: python
      script: |
        import os
        print(os.getcwd())

Output Handling
--------------

Capture Output
~~~~~~~~~~~~~
Store command output in variables:

.. code-block:: yaml

  steps:
    - name: capture
      command: "echo foo"
      output: FOO  # Will contain "foo"

Redirect Output
~~~~~~~~~~~~~
Send output to files:

.. code-block:: yaml

  steps:
    - name: redirect stdout
      command: "echo hello"
      stdout: "/tmp/hello"
    
    - name: redirect stderr
      command: "echo error message >&2"
      stderr: "/tmp/error.txt"

You can use JSON references in fields to dynamically expand values from variables. JSON references are denoted using the ``${NAME.path.to.value}`` syntax, where ``NAME`` refers to a variable name and ``path.to.value`` specifies the path in the JSON to resolve. If the data is not JSON format, the value will not be expanded.

Examples:

.. code-block:: yaml

  steps:
    - name: sub workflow
      run: sub_workflow
      output: SUB_RESULT
    - name: use output
      command: echo "The result is ${SUB_RESULT.outputs.finalValue}"
      depends:
        - sub workflow

If ``SUB_RESULT`` contains:

.. code-block:: json

  {
    "outputs": {
      "finalValue": "success"
    }
  }

Then the expanded value of ``${SUB_RESULT.outputs.finalValue}`` will be ``success``.

Conditional Execution
------------------

Precondition
~~~~~~~~~~~~
Run steps only when conditions are met:

.. code-block:: yaml

  steps:
    - name: monthly task
      command: monthly.sh
      preconditions: "test -f file.txt" # Run only if the file exists

Use multiple conditions:

.. code-block:: yaml

  steps:
    - name: monthly task
      command: monthly.sh
      preconditions: # Run only if all commands exit with 0
        - "test -f file.txt"
        - "test -d dir"

Use environment variables in conditions:

.. code-block:: yaml

  steps:
    - name: monthly task
      command: monthly.sh
      preconditions:
        - condition: "${TODAY}" # Run only if TODAY is set as "01"
          expected: "01"


Use command substitution in conditions:

.. code-block:: yaml

  steps:
    - name: monthly task
      command: monthly.sh
      preconditions:
        - condition: "`date '+%d'`"
          expected: "01"

Use regex in conditions:

.. code-block:: yaml

  steps:
    - name: monthly task
      command: monthly.sh
      preconditions:
        - condition: "`date '+%d'`"
          expected: "re:0[1-9]" # Run only if the day is between 01 and 09

Continue on Failure
~~~~~~~~~~~~~~~~~

Continue to the next step even if the current step fails: 

.. code-block:: yaml

  steps:
    - name: optional task
      command: task.sh
      continueOn:
        failure: true

Continue to the next step even if the current step skipped by preconditions:

.. code-block:: yaml

  steps:
    - name: optional task
      command: task.sh
      preconditions:
        - condition: "`date '+%d'`"
          expected: "01"
      continueOn:
        skipped: true

Based on exit code:

.. code-block:: yaml

  steps:
    - name: optional task
      command: task.sh
      continueOn:
        exitCode: [1, 2] # Continue if exit code is 1 or 2
  
Based on output:

.. code-block:: yaml

  steps:
    - name: optional task
      command: task.sh
      continueOn:
        output: "error" # Continue if output (stdout or stderr) contains "error"  

Use regular expressions:

.. code-block:: yaml

  steps:
    - name: optional task
      command: task.sh
      continueOn:
        output: "re:SUCCE.*" # Continue if output (stdout or stderr) matches "SUCCE.*"

Multiple output conditions:

.. code-block:: yaml

  steps:
    - name: optional task
      command: task.sh
      continueOn:
        output:
          - "complete"
          - "re:SUCCE.*"

Mark as Success even if the step fails but continue to the next step:

.. code-block:: yaml

  steps:
    - name: optional task
      command: task.sh
      continueOn:
        output: "complete"
        markSuccess: true # default is false

Scheduling
---------

Basic Scheduling
~~~~~~~~~~~~~~
Use cron expressions to schedule DAGs:

.. code-block:: yaml

  schedule: "5 4 * * *"  # Run at 04:05
  steps:
    - name: scheduled job
      command: job.sh

Skip Redundant Runs
~~~~~~~~~~~~~~~~~
Prevent unnecessary executions:

.. code-block:: yaml

    name: Daily Data Processing
    schedule: "0 */4 * * *"    
    skipIfSuccessful: true     
    steps:
      - name: extract
        command: extract_data.sh
      - name: transform
        command: transform_data.sh
        depends:
          - extract
      - name: load
        command: load_data.sh
        depends:
          - transform

When ``skipIfSuccessful`` is ``true``, Dagu checks if there's already been a successful run since the last scheduled time. If yes, it skips the execution. This is useful for:

- Resource-intensive tasks
- Data processing jobs that shouldn't run twice
- Tasks that are expensive to run

Note: Manual triggers always execute regardless of this setting.

Example timeline:
- Schedule: Every 4 hours (00:00, 04:00, 08:00, ...)
- At 04:00: Runs successfully
- At 05:00: Manual trigger → Runs (manual triggers always run)
- At 06:00: Schedule trigger → Skips (already succeeded since 04:00)
- At 08:00: Schedule trigger → Runs (new schedule window)

Retry Policies
~~~~~~~~~~~~
Automatically retry failed steps:

.. code-block:: yaml

  steps:
    - name: retryable task
      command: main.sh
      retryPolicy:
        limit: 3
        intervalSec: 5

Advanced Features
---------------

Running sub workflows
~~~~~~~~~~~~~~~~~~~~~~~~
Organize complex workflows using sub workflow:

.. code-block:: yaml

  steps:
    - name: sub workflow
      run: sub_workflow
      params: "FOO=BAR"

The result of the sub workflow will be available from the standard output of the sub workflow in JSON format.

Example:

.. code-block:: json

  {
    "name": "sub_workflow"
    "params": "FOO=BAR",
    "outputs": {
      "RESULT": "ok",
    }
  }

You can access the output of the sub workflow using the `output` field:

.. code-block:: yaml

  steps:
    - name: sub workflow
      run: sub_workflow
      params: "FOO=BAR"
      output: SUB_RESULT

    - name: use sub workflow output
      command: echo $SUB_RESULT
      depends:
        - sub workflow

Command Substitution
~~~~~~~~~~~~~~~~~
Use command output in configurations:

.. code-block:: yaml

  env:
    TODAY: "`date '+%Y%m%d'`"
  steps:
    - name: use date
      command: "echo hello, today is ${TODAY}"

Lifecycle Hooks
~~~~~~~~~~~~~
React to DAG state changes:

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
    - name: main task
      command: echo hello

Repeat Steps
~~~~~~~~~~
Execute steps periodically:

.. code-block:: yaml

  steps:
    - name: repeating task
      command: main.sh
      repeatPolicy:
        repeat: true
        intervalSec: 60

Field Reference
-------------

Quick Reference
~~~~~~~~~~~~~
Common fields you'll use most often:

- ``name``: DAG name
- ``schedule``: Cron schedule
- ``steps``: Task definitions
- ``depends``: Step dependencies
- ``skipIfSuccessful``: Skip redundant runs
- ``env``: Environment variables
- ``retryPolicy``: Retry configuration

DAG Fields
~~~~~~~~~
Complete list of DAG-level configuration options:

- ``name``: The name of the DAG (optional, defaults to filename)
- ``description``: Brief description of the DAG
- ``schedule``: Cron expression for scheduling
- ``skipIfSuccessful``: Skip if already succeeded since last schedule time (default: false)
- ``group``: Optional grouping for organization
- ``tags``: Comma-separated categorization tags
- ``env``: Environment variables
- ``logDir``: Output directory (default: ${HOME}/.local/share/logs)
- ``restartWaitSec``: Seconds to wait before restart
- ``histRetentionDays``: Days to keep execution history
- ``timeoutSec``: DAG timeout in seconds
- ``delaySec``: Delay between steps
- ``maxActiveRuns``: Maximum parallel steps
- ``params``: Default parameters
- ``precondition``: DAG-level conditions
- ``mailOn``: Email notification settings
- ``MaxCleanUpTimeSec``: Cleanup timeout
- ``handlerOn``: Lifecycle event handlers
- ``steps``: List of steps to execute
- ``smtp``: SMTP settings

Example DAG configuration:

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
    precondition:                       
      - condition: "`echo $2`"           
        expected: "param2"               
      - command: "test -f file.txt"
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
    smtp:
      host: "smtp.foo.bar"
      port: "587"
      username: "<username>"
      password: "<password>"

Step Fields
~~~~~~~~~
Configuration options available for individual steps:

- ``name``: Step name (required)
- ``description``: Step description
- ``dir``: Working directory
- ``command``: Command to execute
- ``stdout``: Standard output file
- ``output``: Output variable name
- ``script``: Inline script content
- ``signalOnStop``: Stop signal (e.g., SIGINT)
- ``mailOn``: Step-level notifications
- ``continueOn``: Failure handling
- ``retryPolicy``: Retry configuration
- ``repeatPolicy``: Repeat configuration
- ``preconditions``: Step conditions
- ``depends``: Dependencies
- ``run``: Sub workflow name
- ``params``: Sub workflow parameters

Example step configuration:

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
          exitCode: [1, 2]
          markSuccess: true
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
        run: sub_dag
        params: "FOO=BAR"

Global Configuration
------------------
Common settings can be shared using ``$HOME/.config/dagu/base.yaml``. This is useful for setting default values for:
- ``logDir``
- ``env``
- Email settings
- Other organizational defaults