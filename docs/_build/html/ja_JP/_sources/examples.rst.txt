Examples
========

.. contents::
    :local:

Hello World
------------

.. code-block:: yaml

  name: hello world
  steps:
    - name: s1
      command: echo hello world
    - name: s2
      command: echo done!
      depends:
        - s1


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

.. image:: https://raw.githubusercontent.com/yohamta/dagu/main/examples/images/conditional.png


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
          image: "denoland/deno:1.10.3"
          host:
            autoRemove: true
      command: run https://examples.deno.land/hello-world.ts

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

.. image:: https://raw.githubusercontent.com/yohamta/dagu/main/examples/images/email.png

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
