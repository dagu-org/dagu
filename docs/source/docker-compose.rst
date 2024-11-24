.. _Using Docker Compose:
Using Docker Compose
===================================

Here is an example `docker-compose.yml` setup for running Dagu using Docker Compose.

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
            - DAGU_TZ=Asia/Tokyo
            - DAGU_BASE_PATH=/dagu # optional. default is /
            - PUID=1000 # optional. default is 1000
            - PGID=1000 # optional. default is 1000
            - DOCKER_GID=999 # optional. default is -1 and it will be ignored
            volumes:
            - dagu_config:/config
            - /var/run/docker.sock:/var/run/docker.sock # optional. required for docker in docker
        volumes:
        dagu_config: {}

