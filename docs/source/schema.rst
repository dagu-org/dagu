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

``name``
~~~~~~~~
  The name of the DAG. If omitted, Dagu defaults the name to the YAML filename without the extension.
  
  **Example**:

  .. code-block:: yaml

    name: My Daily DAG

``description``
~~~~~~~~~~~~~~
  A short description of what the DAG does.

  **Example**:

  .. code-block:: yaml

    description: This DAG processes daily data and sends notifications.

``schedule``
~~~~~~~~~~~
  A cron expression (``* * * * *``) that determines how often the DAG runs.  
  If omitted, the DAG will only run manually (unless triggered via CLI or another mechanism).

  **Example**:

  .. code-block:: yaml

    schedule: "5 4 * * *"  # runs daily at 04:05

``dotenv``
~~~~~~~~~~
  Path to a `.env` file or a list of paths to load environment variables from.  
  Dagu reads these files before running the DAG.

  **Example**:

  .. code-block:: yaml

    dotenv: /path/to/.env

  Files can be specified as:
  
  - Absolute paths
  - Relative to the DAG file directory
  - Relative to the base config directory
  - Relative to the user's home directory

``skipIfSuccessful``
~~~~~~~~~~~~~~~~~~~
  If true, Dagu checks whether this DAG has already succeeded since the last scheduled time. If it did, Dagu will skip the current scheduled run. Manual triggers always run regardless of this setting.

  **Example**:

  .. code-block:: yaml

    skipIfSuccessful: true

``group``
~~~~~~~~~
  An organizational label you can use to group DAGs (e.g., "DailyJobs", "Analytics").

``tags``
~~~~~~~~
  A comma-separated list of tags. Useful for searching, grouping, or labeling runs (e.g., "finance, daily").

``env``
~~~~~~~
  Environment variables available to all steps in the DAG. These can use shell expansions, references to other environment variables, or command substitutions. They won't be stored in execution history data for security reasons, so if you want to retry a failed run, you need to have the same environment variables available.

  **Example**:

  .. code-block:: yaml

    env:
      - LOG_DIR: ${HOME}/logs
      - PATH: /usr/local/bin:${PATH}

``logDir``
~~~~~~~~~~
  The base directory in which logs for this DAG are stored.

``restartWaitSec``
~~~~~~~~~~~~~~~~~
  Number of seconds to wait before restarting a failed or stopped DAG. Typically used with a process supervisor.

``histRetentionDays``
~~~~~~~~~~~~~~~~~~~~
  How many days of historical run data to retain for this DAG. After this period, older run logs/history can be purged.

``timeoutSec``
~~~~~~~~~~~~~
  Maximum number of seconds for the entire DAG to finish. If the DAG hasn't finished after this time, it's considered timed out.

``delaySec``
~~~~~~~~~~~
  Delay (in seconds) before starting each step in a DAG run. This can be useful to stagger workloads.

``maxActiveRuns``
~~~~~~~~~~~~~~~
  Limit on how many runs of this DAG can be active at once (especially relevant if the DAG has a frequent schedule).

``params``
~~~~~~~~~
  Default parameters for the entire DAG, either positional or named. Steps can reference these as environment variables (``$1, $2, ...`` for positional or ``$KEY`` for named).

  **Example (positional)**:

  .. code-block:: yaml

    params: param1 param2

  **Example (named)**:

  .. code-block:: yaml

    params:
      - FOO: 1
      - BAR: "`echo 2`"

``precondition``
~~~~~~~~~~~~~~~
  The condition(s) that must be satisfied before the DAG can run. Each condition can use shell expansions or command substitutions to validate external states.

  **Example**: Condition based on command exit code:

  .. code-block:: yaml

    precondition:
      - "test -f /path/to/file"
  
    # or more simply
    precondition: "test -f /path/to/file"

  **Example**: Condition based on environment variables:

  .. code-block:: yaml

    precondition:
      - condition: "$ENV_VAR"
        expected: "value"

  **Example**: Condition based on command output (stdout):

  .. code-block:: yaml

    precondition:
      - condition: "`echo $2`" 
        expected: "param2"

  **Example**: Use regular expressions:
  .. code-block:: yaml

    precondition:
      - condition: "`date '+%d'`"
        expected: "re:0[1-9]" # Run only if the day is between 01 and 09
  
  Note: Regular expressions are supported with the ``re:`` prefix (e.g., ``re:[0-9]{3}``) in the format of Golang's ``regexp`` package.

``mailOn``
~~~~~~~~~
  Email notifications at DAG-level events, such as ``failure`` or ``success``. Also supports ``cancel`` and ``exit``.

  **Example**:

  .. code-block:: yaml

    mailOn:
      failure: true
      success: false

``MaxCleanUpTimeSec``
~~~~~~~~~~~~~~~~~~~
  Maximum number of seconds Dagu will spend cleaning up (stopping steps, finalizing logs, etc.) before forcing shutdown.

``handlerOn``
~~~~~~~~~~~~
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

``steps``
~~~~~~~~
  A list of steps (tasks) to execute. Steps define your workflow logic and can depend on each other. See :ref:`Step Fields <step-fields>` below for details.

``smtp``
~~~~~~~~
  SMTP server configuration for sending email notifications. This is necessary if you use the ``mail`` executor or ``mailOn`` field.

  **Example**:

  .. code-block:: yaml

    smtp:
      host: $SMTP_HOST
      port: "587"
      username: $SMTP_USER
      password: $SMTP_PASS

------------

.. _step-fields:

Step Fields
-----------
Each element in the top-level ``steps`` list has its own fields for customization. A step object looks like this:

``name``
~~~~~~~~
  A unique identifier for the step within this DAG.

``description``
~~~~~~~~~~~~~
  Brief description of what this step does.

``dir``
~~~~~~
  Working directory in which this step's command or script is executed.

``command``
~~~~~~~~~~
  The command or executable to run for this step.  
  Examples include ``bash``, ``python``, or direct shell commands like ``echo hello``.

``script``
~~~~~~~~~
  Multi-line inline script content that will be piped into the command.  
  If ``command`` is omitted, the script is executed with the system's default shell.

``stdout``
~~~~~~~~~
  Path to a file in which to store the standard output (STDOUT) of the step's command.

``stderr``
~~~~~~~~~
  Path to a file in which to store the standard error (STDERR) of the step's command.

``output``
~~~~~~~~~
  A variable name to store the command's STDOUT contents. You can reuse this variable in subsequent steps.

``signalOnStop``
~~~~~~~~~~~~~~
  If you manually stop this step (e.g., via CLI), the signal that Dagu sends to kill the process (e.g., ``SIGINT``).

``mailOn``
~~~~~~~~~
  Email notifications at the step level (same structure as DAG-level ``mailOn``).

``continueOn``
~~~~~~~~~~~~
  Controls how Dagu handles cases where the step is skipped or fails.  

  - **failure**: If true, continue the DAG even if this step fails.  
  - **skipped**: If true, continue the DAG even if preconditions cause this step to skip.
  - **output**: Specify text or list of text to continue on. If the output (stdout or stderr) contains this text, the step is considered successful. Regular expressions are supported with the ``re:`` prefix (e.g., ``re:[0-9]{3}``) in the format of Golang's ``regexp`` package.
  - **markSuccess**: If true, mark the step as successful even if it fails.

``retryPolicy``
~~~~~~~~~~~~~
  Defines automatic retries for this step when it fails.  

  - **limit** (integer): How many times to retry.  
  - **intervalSec** (integer): How many seconds to wait between retries.

  .. code-block:: yaml
  
    retryPolicy:
      limit: 3
      intervalSec: 5

``repeatPolicy``
~~~~~~~~~~~~~
  Allows repeating a step multiple times in a single run.  

  - **repeat** (boolean): Whether to repeat.  
  - **intervalSec** (integer): Interval in seconds between repeats.

  .. code-block:: yaml
  
    repeatPolicy:
      repeat: true
      intervalSec: 60  # run every minute

``precondition``
~~~~~~~~~~~~~~
  Condition(s) that must be met for this step to run. It works same as the DAG-level ``precondition`` field. See :ref:`DAG-Level Fields <DAG-Level-Fields>` for examples.

  .. code-block:: yaml
  
    steps:
      # Example 1: based on exit code
      - name: daily task
        command: daily.sh
        precondition: "test -f /path/to/file"

      # Example 2: based on command output (stdout)
      - name: monthly task
        command: monthly.sh
        precondition:
          - condition: "`date '+%d'`"
            expected: "01"
      
      # Example 3: based on environment variables
      - name: weekly task
        command: weekly.sh
        precondition:
          - condition: "$WEEKDAY"
            expected: "Friday"

``depends``
~~~~~~~~~
  Names of other steps that must complete before this step can run. It can be a single step name or a list of step names.

``run``
~~~~~~
  Reference to another YAML file (sub workflow) to run at this step.  
  If present, the sub workflow is executed in place of a command.

  .. code-block:: yaml
  
    steps:
      - name: sub workflow
        run: sub_dag.yaml
        params: FOO=BAR

``params``
~~~~~~~~
  Parameters to pass into a sub workflow if this step references one (via ``run``). You can also treat these as environment variables in the workflow.

``executor``
~~~~~~~~~~
  An executor configuration specifying how the command or script is run (e.g., Docker, SSH, HTTP, Mail, JSON).  
  For more details, see :ref:`Executors <Executors>`.

------------

Global Configuration
--------------------
You can place global defaults in ``$HOME/.config/dagu/base.yaml``. This file can contain:

- Default environment variables or dotenv files
- Email notification settings
- A global ``logDir``
- Common organizational patterns

Example:

.. code-block:: yaml

  # $HOME/.config/dagu/base.yaml
  logDir: /var/log/dagu
  env:
    - GLOBAL_VAR: "HelloFromGlobalConfig"
  dotenv:
    - /path/to/.env
  mailOn:
    success: true
    failure: true

------------

.. _Executors:

Executors
----------

Executors are specialized modules for handling different types of tasks, including :code:`docker`, :code:`http`, :code:`mail`, :code:`ssh`, and :code:`jq` (JSON) executors. You can configure an executor in any step by specifying:

.. code-block:: yaml

  steps:
    - name: example
      executor:
        type: docker
        config:
          image: "alpine:latest"
      command: echo "Hello from Docker!"

Contributions of new `executors <https://github.com/dagu-org/dagu/tree/main/internal/dag/executor>`_ are welcome.

Docker Executor
~~~~~~~~~~~~~~~
.. _docker-executor:

**Execute an Image**

*Note: Requires Docker daemon running on the host.*

The ``docker`` executor runs commands inside Docker containers. This can help you isolate environments or ensure reproducibility. Example:

.. code-block:: yaml

   steps:
     - name: deno_hello_world
       executor:
         type: docker
         config:
           image: "denoland/deno:latest"
           autoRemove: true
       command: run https://raw.githubusercontent.com/denoland/deno-docs/main/by-example/hello-world.ts

By default, Dagu pulls the Docker image. If you're using a local image, set :code:`pull: false`.

You can also configure volumes, environment variables, etc.:

.. code-block:: yaml

    steps:
      - name: deno_hello_world
        executor:
          type: docker
          config:
            image: "denoland/deno:latest"
            container:
              volumes:
                /app:/app:
              env:
                - FOO=BAR
            autoRemove: true
        command: run https://raw.githubusercontent.com/denoland/deno-docs/main/by-example/hello-world.ts


**Execute Commands in Existing Containers**

You can also run commands in existing containers (like `docker exec`):

.. code-block:: yaml

   steps:
     - name: exec-in-existing
       executor:
         type: docker
         config:
           containerName: "my-running-container"
           autoRemove: true
           exec:
             user: root
             workingDir: /app
             env:
               - MY_VAR=value
       command: echo "Hello from existing container"

**exec** config includes:

- `containerName`: Name or ID of the existing container (required)
- `user`: Username or UID
- `workingDir`: Directory in which the command runs
- `env`: Environment variables

Use Host's Docker Environment
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
If Dagu itself runs in a container, you can still communicate with the host Docker:

1. Mount Docker socket and set the group ID, or
2. Run a `socat` container:

.. code-block:: sh

  docker run -v /var/run/docker.sock:/var/run/docker.sock -p 2376:2375 bobrik/socat \
    TCP4-LISTEN:2375,fork,reuseaddr UNIX-CONNECT:/var/run/docker.sock

Then set `DOCKER_HOST`:

.. code-block:: yaml

  env:
    - DOCKER_HOST: "tcp://host.docker.internal:2376"
  steps:
    - name: deno_hello_world
      executor:
        type: docker
        config:
          image: "denoland/deno:1.10.3"
          autoRemove: true
      command: run https://examples.deno.land/hello-world.ts

HTTP Executor
~~~~~~~~~~~~~
The ``http`` executor can make arbitrary HTTP requests. This is handy for interacting with web services or APIs.

.. code-block:: yaml

   steps:
     - name: send POST request
       command: POST https://foo.bar.com
       executor:
         type: http
         config:
           timeout: 10
           headers:
             Authorization: "Bearer $TOKEN"
           silent: true
           query:
             key: "value"
           body: "post body"

Mail Executor
~~~~~~~~~~~~~
The ``mail`` executor sends email—useful for notifications or alerts.

.. code-block:: yaml

    smtp:
      host: "smtp.foo.bar"
      port: "587"
      username: "<username>"
      password: "<password>"

    params: RECIPIENT=XXX

    steps:
      - name: step1
        executor:
          type: mail
          config:
            to: <to address>
            from: <from address>
            subject: "Exciting New Features Now Available"
            message: |
              Hello [RECIPIENT],

              We hope you're enjoying your experience with MyApp!
              We're thrilled to announce that MyApp v2.0 is now available,
              and we've added some fantastic new features based on
              your valuable feedback.

              Thank you for choosing MyApp and for your continued support.

              Best regards,
              The Team

SSH Executor
~~~~~~~~~~~~~
.. _command-execution-over-ssh:

Run commands on remote hosts via SSH.

.. code-block:: yaml

    steps:
      - name: step1
        executor: 
          type: ssh
          config:
            user: dagu
            ip: XXX.XXX.XXX.XXX
            port: 22
            key: /Users/dagu/.ssh/private.pem
        command: /usr/sbin/ifconfig

JSON Executor
-------------

The ``jq`` executor can be used to transform, query, and format JSON.

Querying data
~~~~~~~~~~~~~
.. code-block:: yaml

  steps:
    - name: run query
      executor: jq
      command: '{(.id): .["10"].b}'
      script: |
        {"id": "sample", "10": {"b": 42}}

Expected Output:

.. code-block:: json

    {
        "sample": 42
    }

Formatting JSON
~~~~~~~~~~~~~~~

.. code-block:: yaml

    steps:
      - name: format json
        executor: jq
        script: |
          {"id": "sample", "10": {"b": 42}}

Output:

.. code-block:: json

    {
        "10": {
            "b": 42
        },
        "id": "sample"
    }