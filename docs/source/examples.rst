.. _Examples:

Examples
============

.. contents::
    :local:

Hello World
------------

.. code-block:: yaml

  params:
    - NAME: "Dagu"
  steps:
    - name: Hello world
      command: echo Hello $NAME
    - name: Done
      command: echo Done!


Conditional Steps
------------------

.. code-block:: yaml

  type: graph  # Use graph type for parallel execution after step1
  params: foo
  steps:
    - name: step1
      command: echo start
    - name: foo
      command: echo foo
      depends:
        - step1
      preconditions:
        - condition: "$1"
          expected: foo
    - name: bar
      command: echo bar
      depends:
        - step1
      preconditions:
        - condition: "$1"
          expected: bar

.. image:: https://raw.githubusercontent.com/dagu-org/dagu/main/examples/images/conditional.png


File Output
------------

.. code-block:: yaml

  steps:
    - name: write hello to '/tmp/hello.txt'
      command: echo hello
      stdout: /tmp/hello.txt

Passing Output to Next Step
---------------------------

.. code-block:: yaml

  steps:
    - name: pass 'hello'
      command: echo hello
      output: OUT1
    - name: output 'hello world'
      command: bash
      script: |
        echo $OUT1 world

Running a Docker Container
--------------------------

.. code-block:: yaml

  steps:
    - name: deno_hello_world
      executor: 
        type: docker
        config:
          image: "denoland/deno:latest"
          autoRemove: true
      command: run https://docs.deno.com/examples/scripts/hello_world.ts

See :ref:`docker executor` for more details.

Sending HTTP Requests
---------------------

.. code-block:: yaml

  steps:
    - name: get fake json data
      executor: http
      command: GET https://jsonplaceholder.typicode.com/comments
      script: |
        {
          "timeout": 10,
          "headers": {},
          "query": {
            "postId": "1"
          },
          "body": ""
        }

Querying JSON Data with jq
----------------------------

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


Formatting JSON Data with jq
----------------------------

.. code-block:: yaml

  steps:
    - name: format json
      executor: jq
      script: |
        {"id": "sample", "10": {"b": 42}}

Expected Output:

.. code-block:: json

    {
        "10": {
            "b": 42
        },
        "id": "sample"
    }


Outputting Raw Values with jq
-----------------------------

.. code-block:: yaml

  steps:
    - name: output raw value
      executor:
        type: jq
        config:
          raw: true
      command: '.id'
      script: |
        {"id": "sample", "10": {"b": 42}}

Expected Output:

.. code-block:: sh

    sample


Sending Email Notifications
---------------------------

.. image:: https://raw.githubusercontent.com/dagu-org/dagu/main/examples/images/email.png

.. code-block:: yaml

  steps:
    - name: Sending Email on Finish or Error
      command: echo "hello world"

  mailOn:
    failure: true
    success: true

  smtp:
    host: "smtp.foo.bar"
    port: "587"
    username: "<username>"
    password: "<password>"
  errorMail:
    from: "foo@bar.com"
    to: "foo@bar.com"
    prefix: "[Error]"
    attachLogs: true
  infoMail:
    from: "foo@bar.com"
    to: "foo@bar.com"
    prefix: "[Info]"
    attachLogs: true


Sending Email
-------------

.. code-block:: yaml

  smtp:
    host: "smtp.foo.bar"
    port: "587"
    username: "<username>"
    password: "<password>"

  steps:
    - name: step1
      executor:
        type: mail
        config:
          to: <to address>
          from: <from address>
          subject: "Sample Email"
          message: |
            Hello world

Sending Email with Attachments
------------------------------

.. code-block:: yaml

  smtp:
    host: "smtp.foo.bar"
    port: "587"
    username: "<username>"
    password: "<password>"

  steps:
    - name: step1
      executor:
        type: mail
        config:
          to: <to address>
          from: <from address>
          subject: "Sample Email"
          message: |
            Hello world
          attachments:
            - /tmp/email-attachment.txt


Customizing Signal Handling on Stop
-----------------------------------

.. code-block:: yaml

  steps:
    - name: step1
      command: bash
      script: |
        for s in {1..64}; do trap "echo trap $s" $s; done
        sleep 60
      signalOnStop: "SIGINT"

Advanced Step Repetition (repeatPolicy)
---------------------------------------

Dagu supports advanced repeat-until logic for steps using the ``repeatPolicy`` field. You can repeat a step until a command output matches a string or regex, or until a specific exit code is returned.

.. code-block:: yaml

  steps:
    - name: repeat-until-string-match
      command: echo foo
      output: RESULT
      repeatPolicy:
        condition: "$RESULT"
        expected: "foo"
        intervalSec: 30

    - name: repeat-until-condition-exits-non-zero
      command: echo "checking"
      repeatPolicy:
        condition: "test -f /tmp/flag"
        intervalSec: 1

    - name: repeat-while-exitcode-matches
      command: test -f /tmp/flag
      repeatPolicy:
        exitCode: [0]
        intervalSec: 5

    - name: repeat-forever
      command: echo 'hello'
      repeatPolicy:
        repeat: true
        intervalSec: 60

- ``condition``: Command or expression to evaluate after each run.
- ``expected``: Value or regex to match the output of ``condition``.
- ``exitCode``: Integer or list of integers; repeat if the last command exits with one of these codes.
- ``repeat``: Boolean; if true, repeat the step unconditionally. This is equivalent to setting ``condition: "true"``.
- ``intervalSec``: Time in seconds to wait before repeating the step.

.. note::

   **repeatPolicy precedence and semantics (Dagu 2025.05):**

   1. If both ``condition`` and ``expected`` are set:
      - After the step runs, evaluate ``condition`` (may be a shell command, env var, or expression).
      - Compare its output to ``expected``. Repeat as long as the comparison does not match.
   2. If only ``condition`` is set (and ``expected`` is empty):
      - Repeat as long as ``condition`` (may be a shell command, env var, or expression) evaluates to exit code 0.
   3. If ``exitCode`` is specified (and ``condition`` is not set):
      - Repeat as long as the last step’s exit code matches any value in the list.
   4. If only ``repeat: true``, repeat unconditionally at the given interval.

   The evaluation order is: ``condition`` > ``exitCode`` > ``repeat``. This mirrors the ``precondition`` logic for consistency.


Parallel Execution
------------------

Execute the same workflow with different parameters in parallel:

.. code-block:: yaml

  name: batch-processing
  
  steps:
    - name: get-files
      command: ls /data/*.csv | head -10
      output: FILES
    
    - name: process-files-parallel
      run: process-csv
      parallel: ${FILES}
      output: RESULTS
    
    - name: summary
      command: |
        echo "Processed files:"
        echo "${RESULTS}" | jq '.summary'

Process multiple items with object parameters:

.. code-block:: yaml

  name: multi-region-deployment
  
  steps:
    - name: deploy-to-regions
      run: deploy-stack
      parallel:
        items:
          - REGION: "us-east-1"
            STACK: "web-app"
            VERSION: "v1.2.0"
          - REGION: "eu-west-1"
            STACK: "web-app"
            VERSION: "v1.2.0"
          - REGION: "ap-south-1"
            STACK: "web-app"
            VERSION: "v1.1.9"
        maxConcurrent: 2  # Deploy to 2 regions at a time
      output: DEPLOY_RESULTS
    
    - name: verify-deployments
      command: |
        FAILED=$(echo "${DEPLOY_RESULTS}" | jq '.summary.failed')
        if [ "$FAILED" -gt 0 ]; then
          echo "Some deployments failed!"
          exit 1
        fi

For more details on parallel execution, see :ref:`Parallel Execution`.

Queue Management
---------------

Control concurrent execution of DAGs using queue configuration:

**Basic Queue Assignment:**

.. code-block:: yaml

  name: data-processing
  queue: "batch"          # Assign to "batch" queue
  maxActiveRuns: 2        # Allow max 2 concurrent runs
  schedule: "*/10 * * * *"  # Run every 10 minutes
  
  steps:
    - name: process
      command: process_data.sh

**Disable Queueing for Critical Jobs:**

.. code-block:: yaml

  name: critical-alert
  maxActiveRuns: -1       # Disable queueing - always run immediately
  
  steps:
    - name: check-alerts
      command: check_alerts.sh
    - name: notify
      command: send_notifications.sh

**Unified Queue Management via Base Configuration:**

.. code-block:: yaml

  # ~/.config/dagu/base.yaml
  # All DAGs will use these settings by default
  queue: "global-queue"
  maxActiveRuns: 2

**Global Queue Configuration (config.yaml):**

.. code-block:: yaml

  # Global queue settings
  queues:
    enabled: true
    config:
      - name: "critical"
        maxConcurrency: 5    # Critical jobs can run 5 at a time
      - name: "batch"
        maxConcurrency: 1    # Batch jobs run one at a time
      - name: "reporting"
        maxConcurrency: 3    # Reporting can run 3 at a time
      - name: "global-queue"
        maxConcurrency: 10   # Shared queue for all DAGs by default

**Manual Queue Management:**

.. code-block:: sh

  # Enqueue a DAG run with custom run ID
  dagu enqueue batch-job.yaml --run-id=batch-2024-01-15
  
  # Enqueue with parameters
  dagu enqueue process.yaml -- INPUT=/data/file.csv OUTPUT=/results/
  
  # Remove from queue
  dagu dequeue --dag-run=batch-job:batch-2024-01-15
