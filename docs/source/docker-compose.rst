Using Docker Compose
===================================

To automate DAG executions based on cron expressions, it is necessary to run both the ui server and scheduler process. Here is an example `docker-compose.yml` setup for running Dagu using Docker Compose.

.. code-block:: yaml

  version: "3.9"
  services:
      # init container updates permission
      init:
          image: "ghcr.io/dagu-org/dagu:latest"
          user: root
          volumes:
              - dagu_config:/home/dagu/.config/dagu
              - dagu_data:/home/dagu/.local/share
          command: chown -R dagu /home/dagu/.config/dagu/ /home/dagu/.local/share
      # ui server process
      server:
          image: "ghcr.io/dagu-org/dagu:latest"
          environment:
              - DAGU_PORT=8080
          restart: unless-stopped
          ports:
              - "8080:8080"
          volumes:
              - dagu_config:/home/dagu/.config/dagu
              - dagu_data:/home/dagu/.local/share
          command: dagu server
          depends_on:
              - init
      # scheduler process
      scheduler:
          image: "ghcr.io/dagu-org/dagu:latest"
          restart: unless-stopped
          volumes:
              - dagu_config:/home/dagu/.config/dagu
              - dagu_data:/home/dagu/.local/share
          command: dagu scheduler
          depends_on:
              - init
  volumes:
      dagu_config: {}
      dagu_data: {}

