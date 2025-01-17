.. _Executors:

Executors
==========

.. contents::
    :local:

Executors are specialized modules for handling different types of tasks, including :code:`docker`, :code:`http`, :code:`mail`, :code:`ssh`, and :code:`jq` (JSON) executors. Contributions of new `executors <https://github.com/dagu-org/dagu/tree/main/internal/dag/executor>`_ are very welcome.

.. _docker executor:

Docker Executor
----------------

Execute an Image
~~~~~~~~~~~~~~~~~

*Note: It requires Docker daemon running on the host.*

The `docker` executor allows us to run Docker containers instead of bare commands. This can be useful for running commands in isolated environments or for reproducibility purposes.

.. code-block:: yaml

    steps:
      - name: hello
        executor:
          type: docker
          config:
            image: alpine
            autoRemove: true
        command: echo "hello"

Example Log output:

.. image:: https://raw.githubusercontent.com/dagu-org/dagu/main/examples/images/docker.png

By default, Dagu will try to pull the Docker image. For images built locally this will fail. If you want to skip image pull, pass :code:`pull: false` in executor config.

.. code-block:: yaml

    steps:
      - name: hello
        executor:
          type: docker
          config:
            image: alpine
            pull: false
            autoRemove: true
        command: echo "hello"


You can config the Docker container (e.g., `volumes`, `env`, etc) by passing more detailed options.

For example:

.. code-block:: yaml

    steps:
      - name: hello
        executor:
          type: docker
          config:
            image: alpine
            pull: false
            container:
              volumes:
                /app:/app:
              env:
                - FOO=BAR
            autoRemove: true
        command: echo "${FOO}"

See the Docker's API documentation for all available options.

- For `container`, see `ContainerConfig <https://pkg.go.dev/github.com/docker/docker/api/types/container#Config>`_.
- For `host`, see `HostConfig <https://pkg.go.dev/github.com/docker/docker/api/types/container#HostConfig>`_.

Execute Commands in Existing Containers
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

The Docker executor also supports executing commands in already-running containers using Docker's exec functionality, similar to `docker exec`. This is useful when you need to run commands in containers that are already running as part of your infrastructure.

.. code-block:: yaml

   steps:
     - name: exec-in-existing
       executor:
         type: docker
         config:
           containerName: "my-running-container"  # Name of existing container
           autoRemove: true
           exec:
             user: root          # Optional: user to run as
             workingDir: /app   # Optional: working directory
             env:               # Optional: environment variables
               - MY_VAR=value
       command: echo "Hello from existing container"

Available exec configuration options:

- `containerName`: Name or ID of the existing container (required)
- `exec`:
    - `user`: Username or UID to execute command as (optional)
    - `workingDir`: Working directory for command execution (optional)
    - `env`: List of environment variables (optional)

For comparison, here's how you would create and run in a new container:

.. code-block:: yaml

   steps:
     - name: create-new
       executor:
         type: docker
         config:
           image: alpine:latest
           autoRemove: true
       command: echo "Hello from new container"


Use Host's Docker Environment
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

If you are running `dagu` using a container, there are two options to use the host's Docker environment.

1. Mount the Docker socket to the container and pass through the host's docker group id. See the example in :ref:`Using Docker Compose <Using Docker Compose>`

Or

1. Run a `socat` container with the command below.

.. code-block:: sh

    docker run -v /var/run/docker.sock:/var/run/docker.sock -p 2376:2375 bobrik/socat TCP4-LISTEN:2375,fork,reuseaddr UNIX-CONNECT:/var/run/docker.sock

2. Then you can set the `DOCKER_HOST` environment as follows.

.. code-block:: yaml

    env:
      - DOCKER_HOST : "tcp://host.docker.internal:2376"
    steps:
      - name: hello
        executor:
          type: docker
          config:
            image: alpine
            autoRemove: true
        command: echo "hello"

For more details, see `this page <https://forums.docker.com/t/remote-api-with-docker-for-mac-beta/15639/2>`_.

HTTP Executor
--------------

The `http` executor allows us to make an arbitrary HTTP request. This can be useful for interacting with web services or APIs.

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
           silent: true # If silent is true, it outputs response body only.
           query:
             key: "value"
           body: "post body"

Mail Executor
--------------

The `mail` executor can be used to send email. This can be useful for sending notifications or alerts.

Example:

.. code-block:: yaml

    smtp:
      host: "smtp.foo.bar"
      port: "587"
      username: "<username>"
      password: "<password>"
    
    params:
      - RECIPIENT_NAME: XXX
      - RECIPIENT_EMAIL: example@company.com
      - MESSAGE: "Hello [RECIPIENT_NAME]"

    steps:
      - name: step1
        executor:
          type: mail
          config:
            to: $RECIPIENT_EMAIL
            from: dagu@dagu.com
            subject: "Hello [RECIPIENT_NAME]"
            message: $MESSAGE

.. _command-execution-over-ssh:

SSH Executor
-------------

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

JSON Executor
-----------------

The `jq` executor can be used to transform, query, and format JSON. This can be useful for working with JSON data in pipelines or for data processing.

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