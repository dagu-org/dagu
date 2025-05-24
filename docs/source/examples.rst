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
      depends:
        - Hello world


Conditional Steps
------------------

.. code-block:: yaml

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
      depends:
        - pass 'hello'

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
      command: echo hello 
      repeatPolicy:
        condition: "hello"
        expected: "hello"
        intervalSec: 10

    - name: repeat-until-exitcode
      command: bash check_status.sh
      repeatPolicy:
        exitCode: [42]
        intervalSec: 5

    - name: repeat-until-shell-output
      command: echo "triggering repeat"
      repeatPolicy:
        condition: "`echo foo`"
        expected: "foo"
        intervalSec: 30

    - name: repeat-forever
      command: echo 'hello'
      repeatPolicy:
        repeat: true
        intervalSec: 60

- ``condition``: Command or expression to evaluate after each run.
- ``expected``: Value or regex to match the output of ``condition``.
- ``exitCode``: Integer or list of integers; repeat if the last command exits with one of these codes.
- ``repeat``/``intervalSec``: repeat unconditionally at the given interval.
