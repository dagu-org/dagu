---
name: Container Workflow Patterns
description: Docker container configuration for DAG workflows
version: 1.0.0
author: Dagu
tags:
  - docker
  - container
---
# Container Workflow Patterns

## DAG-Level Container

Set `container:` at the DAG level so all steps run inside the same container:

```yaml
container:
  image: python:3.13
  volumes:
    - ./data:/data
  env:
    - PYTHONPATH: /app
  working_dir: /app

steps:
  - python script1.py      # runs inside the container
  - python script2.py      # same container
```

### Startup Modes

The `startup` field controls how the DAG-level container is initialized:

- **`keepalive`** (default): Uses an internal keepalive process. Steps execute via `docker exec`.
- **`entrypoint`**: Honors the image's ENTRYPOINT/CMD. The first step waits for the entrypoint to complete.
- **`command`**: Runs the provided `command` array as the container process.

```yaml
container:
  image: postgres:16
  startup: entrypoint       # let postgres start normally
  ports:
    - "5432:5432"
  env:
    - POSTGRES_PASSWORD: secret
```

### Exec Into Existing Container

Run steps in an already-running container:

```yaml
container:
  exec: my-running-container  # container name or ID
  working_dir: /app

steps:
  - ./run-tests.sh
```

## Per-Step Container Override

Individual steps can use a different container with `type: docker`:

```yaml
container:
  image: node:20

steps:
  - name: build
    command: npm run build

  - name: test-python
    type: docker
    command: pytest /tests
    config:
      image: python:3.13
      volumes:
        - ./tests:/tests

  - name: deploy
    command: npm run deploy
```

### Docker Executor Config

```yaml
steps:
  - name: process
    type: docker
    command: python /app/process.py
    config:
      image: python:3.13
      volumes:
        - ./data:/data:ro          # read-only mount
        - ./output:/output         # read-write (default)
      auto_remove: true            # remove container after exit
      pull: missing                # always | never | missing (default)
      working_dir: /app
      platform: linux/amd64        # cross-platform builds
```

### Exec Into Running Container

Use `container_name` to exec into an existing container instead of creating a new one:

```yaml
steps:
  - name: run-migration
    type: docker
    command: python manage.py migrate
    config:
      container_name: my-django-app
```

## Volume Patterns

### Shared Data Between Steps

Use a shared volume to pass data between container steps:

```yaml
container:
  image: alpine:latest
  volumes:
    - /tmp/pipeline:/shared

steps:
  - echo "data" > /shared/output.txt
  - cat /shared/output.txt
```

### Relative Paths

Relative paths (starting with `./` or `.`) resolve relative to the DAG's working directory:

```yaml
container:
  image: python:3.13
  volumes:
    - ./src:/app/src        # {working_dir}/src → /app/src
    - ./config:/app/config
```

## Registry Authentication

Pull images from private registries:

```yaml
registry_auths:
  ghcr.io:
    username: ${GITHUB_USER}
    password: ${GITHUB_TOKEN}
  123456789.dkr.ecr.us-east-1.amazonaws.com:
    username: AWS
    password: ${ECR_TOKEN}

container:
  image: ghcr.io/my-org/my-app:latest
```

Or use a Docker auth config string:

```yaml
registry_auths: ${DOCKER_AUTH_CONFIG}
```

## Multi-Container Pipeline

Run different steps in different containers:

```yaml
type: graph

steps:
  - name: build
    type: docker
    command: npm run build
    config:
      image: node:20
      volumes:
        - ./dist:/dist
    output: BUILD_STATUS

  - name: test
    type: docker
    command: pytest --junitxml=/results/report.xml
    config:
      image: python:3.13
      volumes:
        - ./dist:/app/dist:ro
        - ./results:/results
    depends:
      - build

  - name: deploy
    type: docker
    command: /deploy.sh
    config:
      image: bitnami/kubectl:latest
      volumes:
        - ./dist:/dist:ro
        - ./k8s:/k8s:ro
    depends:
      - test
```

## Environment Variables

```yaml
container:
  image: python:3.13
  env:
    # Map format
    - DATABASE_URL: ${DATABASE_URL}
    - API_KEY: ${API_KEY}
    # String format also works
    - "DEBUG=true"
```

## Container with Error Handling

```yaml
steps:
  - name: risky-step
    type: docker
    command: python /app/risky.py
    config:
      image: python:3.13
      auto_remove: true
    continue_on:
      failure: true             # continue even if container exits non-zero
    retry_policy:
      limit: 3
      interval_sec: 10
```

## Ports and Networking

```yaml
container:
  image: nginx:latest
  ports:
    - "8080:80"
  network: my-network         # custom Docker network
```
