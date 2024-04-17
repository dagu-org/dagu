Executors
=========

.. contents::
    :local:

The `executor` field provides different execution methods for each step. These executors are responsible for executing the commands or scripts specified in the command or script field of the step. Below are the available executors and their use cases.

In the `examples <./examples/>`_ directory, you can find a collection of sample DAGs that demonstrate how to use executors.

.. _docker executor:

Running Docker Containers
-------------------------

*Note: It requires Docker daemon running on the host.*

The `docker` executor allows us to run Docker containers instead of bare commands. This can be useful for running commands in isolated environments or for reproducibility purposes.

In the example below, it pulls and runs `Deno's docker image <https://hub.docker.com/r/denoland/deno>`_ and prints 'Hello World'.

.. code-block:: yaml

   steps:
     - name: deno_hello_world
       executor:
         type: docker
         config:
           image: "denoland/deno:1.10.3"
           autoRemove: true
       command: run https://examples.deno.land/hello-world.ts

Example Log output:

.. image:: https://raw.githubusercontent.com/yohamta/dagu/main/examples/images/docker.png

Configuring Container Volumes, Environment Variables, and More
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

You can config the Docker container (e.g., `volumes`, `env`, etc) by passing more detailed options.

For example:

.. code-block:: yaml

    steps:
      - name: deno_hello_world
        executor:
          type: docker
          config:
            image: "denoland/deno:1.10.3"
            container:
              volumes:
                /app:/app:
              env:
                - FOO=BAR
            host:
              autoRemove: true
        command: run https://examples.deno.land/hello-world.ts

See the Docker's API documentation for all available options.

- For `container`, see `ContainerConfig <https://pkg.go.dev/github.com/docker/docker/api/types/container#Config>`_.
- For `host`, see `HostConfig <https://pkg.go.dev/github.com/docker/docker/api/types/container#HostConfig>`_.


Running Containers on the Host's Docker Environment
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

If you are running `dagu` using a container, you need the setup below.

1. Run a `socat` container with the command below.

.. code-block:: sh

    docker run -v /var/run/docker.sock:/var/run/docker.sock -p 2376:2375 bobrik/socat TCP4-LISTEN:2375,fork,reuseaddr UNIX-CONNECT:/var/run/docker.sock

2. Then you can set the `DOCKER_HOST` environment as follows.

.. code-block:: yaml

    env:
      - DOCKER_HOST : "tcp://host.docker.internal:2376"
    steps:
      - name: deno_hello_world
        executor:
          type: docker
          config:
            image: "denoland/deno:1.10.3"
            autoRemove: true
        command: run https://examples.deno.land/hello-world.ts

For more details, see `this page <https://forums.docker.com/t/remote-api-with-docker-for-mac-beta/15639/2>`_.

Making HTTP Requests
--------------------

The `http` executor allows us to make an arbitrary HTTP request. This can be useful for interacting with web services or APIs.

.. code-block:: yaml

   steps:
     - name: send POST request
       command: POST https://foo.bar.com
       executor:
         type: http
         config:
           timeout: 10,
           headers:
             Authorization: "Bearer $TOKEN"
           silent: true # If silent is true, it outputs response body only.
           query:
             key: "value"
           body: "post body"

Sending Email
-------------

The `mail` executor can be used to send email. This can be useful for sending notifications or alerts.

Example:

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
              We're thrilled to announce that [] v2.0 is now available,
              and we've added some fantastic new features based on your
              valuable feedback.

              Thank you for choosing MyApp and for your continued support.
              We look forward to hearing from you and providing you with
              an even better MyApp experience.

              Best regards,

Executing jq Command
---------------------

The `jq` executor can be used to transform, query, and format JSON. This can be useful for working with JSON data in pipelines or for data processing.

Query Example
~~~~~~~~~~~~~

.. code-block:: yaml

    steps:
      - name: run query
        executor: jq
        command: '{(.id): .["10"].b}'
        script: |
          {"id": "sample", "10": {"b": 42}}

**Output:**

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

**Output:**

.. code-block:: json

    {
        "10": {
            "b": 42
        },
        "id": "sample"
    }

.. _command-execution-over-ssh:

Command Execution over SSH
--------------------------

The `ssh` executor allows us to execute commands on remote hosts over SSH.

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

