Using Docker Compose
===================================

To automate DAG executions based on cron expressions, it is necessary to run both the ui server and scheduler process. Here is an example `docker-compose.yml` setup for running Dagu using Docker Compose.

.. code-block:: yaml

    version: "3.9"
    services:

      # init container updates permission
      init:
        image: "ghcr.io/dagu-dev/dagu:latest"
        user: root
        volumes:
          - dagu:/home/dagu/.dagu
        command: chown -R dagu /home/dagu/.dagu/

      # ui server process
      server:
        image: "ghcr.io/dagu-dev/dagu:latest"
        environment:
          - DAGU_PORT=8080
          - DAGU_DAGS=/home/dagu/.dagu/dags
        restart: unless-stopped
        ports:
          - "8080:8080"
        volumes:
          - dagu:/home/dagu/.dagu
          - ./dags/:/home/dagu/.dagu/dags
        depends_on:
          - init

      # scheduler process
      scheduler:
        image: "ghcr.io/dagu-dev/dagu:latest"
        environment:
          - DAGU_DAGS=/home/dagu/.dagu/dags
        restart: unless-stopped
        volumes:
          - dagu:/home/dagu/.dagu
          - ./dags/:/home/dagu/.dagu/dags
        command: dagu scheduler
        depends_on:
          - init

    volumes:
      dagu: {}
