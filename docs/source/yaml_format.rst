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

Using step IDs in dependencies:

.. code-block:: yaml

  steps:
    - name: prepare data
      id: prep
      command: echo "data ready"
    - name: process data
      id: proc
      command: echo "processing"
    - name: finalize
      command: echo "done"
      depends:
        - prep    # Reference by ID
        - proc    # Reference by ID

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

Execution Types
~~~~~~~~~~~~~~~

Dagu supports different execution types that control how steps are executed:

**Chain Type (Default)**

The default execution type where steps execute sequentially in the order they are defined. Each step automatically depends on the previous one:

.. code-block:: yaml

  # type: chain  # Optional, this is now the default
  steps:
    - name: download
      command: wget https://example.com/data.csv
    - name: process
      command: python process.py  # Automatically depends on "download"
    - name: upload
      command: aws s3 cp output.csv s3://bucket/  # Automatically depends on "process"

**Graph Type**

Explicit dependency-based execution where steps run based on their ``depends`` field:

.. code-block:: yaml

  type: graph
  steps:
    - name: step1
      command: echo "First"
    - name: step2
      command: echo "Second"
      depends: step1  # Explicit dependency required
    - name: step3
      command: echo "Third"
      depends: step2

**Overriding Chain Dependencies**

You can still use explicit ``depends`` in chain type to override the automatic dependencies:

.. code-block:: yaml

  type: chain
  steps:
    - name: setup
      command: ./setup.sh
    - name: download-a
      command: wget fileA
    - name: download-b
      command: wget fileB
    - name: process-both
      command: process.py fileA fileB
      depends:  # Override chain to depend on both downloads
        - download-a
        - download-b
    - name: cleanup
      command: rm -f fileA fileB  # Back to chain: depends on "process-both"

**Running Steps Without Dependencies in Chain Mode**

To run a step without any dependencies (even in chain mode), explicitly set ``depends`` to an empty array:

.. code-block:: yaml

  type: chain
  steps:
    - name: step1
      command: echo "First"
    - name: step2
      command: echo "Second - depends on step1"
    - name: step3
      command: echo "Third - runs independently"
      depends: []  # Explicitly no dependencies
    - name: step4
      command: echo "Fourth - depends on step3"

**Agent Type**

Reserved for future agent-based execution (not yet implemented).

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

**Output Size Limits**: To prevent memory issues from large command outputs, Dagu enforces a size limit on captured output. By default, this limit is 1MB. If a step's output exceeds this limit, the step will fail with an error.

You can configure the maximum output size at the DAG level:

.. code-block:: yaml

  # Set maximum output size to 5MB for all steps in this DAG
  maxOutputSize: 5242880  # 5MB in bytes
  
  steps:
    - name: large-output
      command: "cat large-file.txt"
      output: CONTENT  # Will fail if file exceeds 5MB

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
    - name: child DAG
      run: sub_workflow
      output: SUB_RESULT
    - name: use output
      command: echo "The result is ${SUB_RESULT.outputs.finalValue}"

If ``SUB_RESULT`` contains:

.. code-block:: json

  {
    "outputs": {
      "finalValue": "success"
    }
  }

Then the expanded value of ``${SUB_RESULT.outputs.finalValue}`` will be ``success``.

Step ID References
~~~~~~~~~~~~~~~~~
You can assign short identifiers to steps and use them to reference step properties in subsequent steps. This is particularly useful when you have long step names or want cleaner variable references:

.. code-block:: yaml

  steps:
    - name: extract customer data
      id: extract  # Short identifier
      command: python extract.py
      output: DATA
    
    - name: validate extracted data
      id: validate
      command: python validate.py
      depends:
        - extract  # Can use ID in dependencies
    
    - name: process if valid
      command: |
        # Reference step properties using IDs
        echo "Exit code: ${extract.exit_code}"
        echo "Stdout path: ${extract.stdout}"
        echo "Stderr path: ${extract.stderr}"
        echo "Output data: ${extract.outputs.DATA}"
      depends:
        - validate

Available step properties when using ID references:
- ``${id.stdout}``: Path to stdout file
- ``${id.stderr}``: Path to stderr file  
- ``${id.exit_code}``: Exit code of the step
- ``${id.outputs.VARNAME}``: Value of output variable VARNAME

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
      - name: load
        command: load_data.sh

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

Queue Management
~~~~~~~~~~~~~~~
Control concurrent DAG execution with queue configuration:

.. code-block:: yaml

  name: batch-job
  queue: "batch"        # Assign to a named queue (default: DAG name)
  maxActiveRuns: 2      # Max concurrent runs for this DAG (default: 1)
  steps:
    - name: process
      command: process_data.sh

**Queue Features:**

- **Named Queues**: Assign DAGs to specific queues for better resource management
- **Concurrency Control**: Set ``maxActiveRuns`` to control how many instances can run simultaneously
- **Queue Disabling**: Set ``maxActiveRuns: -1`` to disable queueing for a specific DAG
- **Global Configuration**: Define queue settings in the global config file

**Global Queue Configuration (config.yaml):**

.. code-block:: yaml

  queues:
    enabled: true       # Enable/disable queue system globally
    config:
      - name: "critical"
        maxConcurrency: 5    # Allow 5 concurrent runs for critical queue
      - name: "batch"
        maxConcurrency: 1    # Only 1 batch job at a time
      - name: "default"
        maxConcurrency: 3    # Default queue settings

**Queue Priority:**

1. Global queue config (if queue name matches)
2. DAG's ``maxActiveRuns`` setting
3. Base configuration ``maxActiveRuns`` (from ``~/.config/dagu/base.yaml``)
4. Default value (1)

**Using Base Configuration for Unified Queue Management:**

You can use the base configuration file to assign all DAGs to the same queue:

.. code-block:: yaml

  # ~/.config/dagu/base.yaml
  queue: "global-queue"
  maxActiveRuns: 3

This ensures all DAGs share the same queue unless explicitly overridden.

Retry Policies
~~~~~~~~~~~~
Automatically retry failed steps with configurable error codes:

.. code-block:: yaml

  steps:
    - name: retryable task
      command: main.sh
      retryPolicy:
        limit: 3
        intervalSec: 5
        exitCodes: [1, 2]  # Optional: List of exit codes that should trigger a retry

The retry policy supports the following parameters:

- ``limit``: Maximum number of retry attempts (required)
- ``intervalSec``: Time in seconds to wait between retries (required)
- ``exitCodes``: List of exit codes that should trigger a retry (optional)

If ``exitCodes`` is not specified, any non-zero exit code will trigger a retry. When ``exitCodes`` is specified, only the listed exit codes will trigger a retry.

Example with custom error codes:

.. code-block:: yaml

  steps:
    - name: api call
      command: make-api-request
      retryPolicy:
        limit: 3
        intervalSec: 30
        exitCodes: [429, 503]  # Retry on rate limit and service unavailable errors

In this example:
- The command will be retried up to 3 times
- There will be a 30-second wait between retries
- Retries will only occur if the command exits with code 429 (Too Many Requests) or 503 (Service Unavailable)
- Other error codes will cause immediate failure

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

.. note::
   For executing the same child DAG multiple times with different parameters in parallel, see :ref:`Parallel Execution`.

Multiple DAGs in a Single File
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
Dagu supports defining multiple DAGs within a single YAML file, separated by ``---``. This feature enables:

- Better organization of related workflows
- Reusable workflow components
- Modular workflow design

**Basic Example:**

.. code-block:: yaml

    # main.yaml
    name: main-workflow
    steps:
      - name: process-data
        run: data-processor
        params: "TYPE=daily"
    
    ---
    
    name: data-processor
    params:
      - TYPE: "batch"
    steps:
      - name: extract
        command: ./extract.sh ${TYPE}
      - name: transform
        command: ./transform.sh

When the main workflow executes, it can reference ``data-processor`` as a local DAG defined in the same file.

**Complex Example with Multiple Local DAGs:**

.. code-block:: yaml

    # etl-pipeline.yaml
    name: etl-orchestrator
    schedule: "0 2 * * *"
    steps:
      - name: validate
        run: validator
        output: VALIDATION_RESULT
      
      - name: process
        run: processor
        params: "VALIDATION=${VALIDATION_RESULT}"
        depends: validate
      
      - name: notify
        run: notifier
        params: "STATUS=completed"
        depends: process
    
    ---
    
    name: validator
    steps:
      - name: check-source
        command: test -f /data/input.csv
      - name: validate-format
        command: python validate.py /data/input.csv
        output: IS_VALID
    
    ---
    
    name: processor
    params:
      - VALIDATION: ""
    steps:
      - name: process-data
        command: python process.py
        preconditions:
          - condition: "${VALIDATION}"
            expected: "true"
    
    ---
    
    name: notifier
    params:
      - STATUS: ""
    steps:
      - name: send-notification
        command: |
          curl -X POST https://api.example.com/notify \
            -d "status=${STATUS}"

**Key Points:**

- Each DAG must have a unique ``name`` within the file
- Local DAGs are only accessible within the same file
- The first DAG in the file is considered the main/parent DAG
- Local DAGs can accept parameters and return outputs just like external DAGs
- Local DAGs are executed in separate processes for isolation

**When to Use Multiple DAGs:**

- **Modular Workflows**: Break complex workflows into manageable components
- **Reusable Logic**: Define common patterns once and reuse within the file
- **Testing**: Keep test workflows together with the main workflow
- **Related Processes**: Group related workflows that share common logic

.. note::
   Local DAGs defined with ``---`` separator are different from external DAG files. They exist only within the context of the file where they are defined and cannot be referenced from other DAG files.

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
- ``type``: Execution type - ``chain`` (default), ``graph``, or ``agent``
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
- ``maxActiveSteps``: Maximum parallel steps (default: no limit)
- ``maxActiveRuns``: Maximum concurrent runs of this DAG (default: 1, negative values disable queueing)
- ``queue``: Queue name for this DAG (default: DAG name)
- ``maxOutputSize``: Maximum size in bytes for step output capture (default: 1048576, which is 1MB)
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
    maxActiveSteps: 1                     
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
- ``id``: Optional short identifier for variable references (e.g., ${id.stdout})
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
- ``queue`` - Assign all DAGs to the same queue by default
- ``maxActiveRuns`` - Default concurrent execution limit
- ``logDir`` - Default log directory
- ``env`` - Shared environment variables
- Email settings
- Other organizational defaults

Example base configuration for queue management:

.. code-block:: yaml

  # ~/.config/dagu/base.yaml
  queue: "global-queue"    # All DAGs use this queue by default
  maxActiveRuns: 2         # Default max concurrent runs
  logDir: /var/log/dagu

Individual DAGs can override these settings:

.. code-block:: yaml

  # my-critical-dag.yaml
  name: critical-process
  queue: "critical"        # Override to use critical queue
  maxActiveRuns: -1        # Override to disable queueing
  steps:
    - name: process
      command: critical_job.sh
