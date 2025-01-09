.. _Using Docker Compose:
Using Docker Compose
===================================

Here is an example `docker-compose.yml` setup for running Dagu using Docker Compose.

Running Dagu with Docker Compose
---------------------------------

.. code-block:: yaml

    services:
      dagu:
        image: "ghcr.io/dagu-org/dagu:latest"
        container_name: dagu
        hostname: dagu
        ports:
          - "8080:8080"
        environment:
          - DAGU_PORT=8080 # optional. default is 8080
          - DAGU_TZ=Asia/Tokyo # optional. default is local timezone
          - DAGU_BASE_PATH=/dagu # optional. default is /
          - PUID=1000 # optional. default is 1000
          - PGID=1000 # optional. default is 1000
        volumes:
          - dagu_config:/config
    volumes:
      dagu_config: {}


Enable Docker in Docker (DinD) support
---------------------------------------

.. code-block:: yaml

    services:
      dagu:
        image: "ghcr.io/dagu-org/dagu:latest"
        container_name: dagu
        hostname: dagu
        ports:
          - "8080:8080"
        environment:
          - DAGU_PORT=8080 # optional. default is 8080
          - DAGU_TZ=Asia/Tokyo # optional. default is local timezone
        volumes:
          - dagu_config:/config
          - /var/run/docker.sock:/var/run/docker.sock # optional. required for docker in docker
        command: dagu start-all
        user: "0:0"
        entrypoint: [] # Override any default entrypoint
    volumes:
      dagu_config: {}
