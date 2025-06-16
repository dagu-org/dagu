# Docker Executor

The Docker executor allows you to run commands inside Docker containers, providing isolated environments and ensuring reproducibility across different systems.

> **Note**: The Docker executor requires Docker daemon running on the host.

## Overview

The Docker executor enables you to:

- Run commands in isolated container environments
- Execute commands in existing containers
- Use any Docker image from registries
- Mount volumes and set environment variables
- Configure networking and container options
- Ensure reproducible execution environments

## Basic Usage

### Execute in a New Container

```yaml
steps:
  - name: hello
    executor:
      type: docker
      config:
        image: alpine
        autoRemove: true
    command: echo "hello"
```

### Execute in an Existing Container

```yaml
steps:
  - name: exec-in-existing
    executor:
      type: docker
      config:
        containerName: "my-running-container"
        autoRemove: true
        exec:
          user: root
          workingDir: /app
    command: echo "Hello from existing container"
```

## Image Management

### Pull Policies

Control when and how images are pulled:

```yaml
steps:
  - name: pull-always
    executor:
      type: docker
      config:
        image: node:latest
        pull: always  # Always pull the latest image
        autoRemove: true
    command: node --version

  - name: pull-if-missing
    executor:
      type: docker
      config:
        image: python:3.11
        pull: missing  # Default - only pull if not available locally
        autoRemove: true
    command: python --version

  - name: never-pull
    executor:
      type: docker
      config:
        image: my-local-image
        pull: never  # Use local image only
        autoRemove: true
    command: ./app
```

### Platform Selection

Specify platform for multi-architecture images:

```yaml
steps:
  - name: specific-platform
    executor:
      type: docker
      config:
        image: alpine
        platform: linux/amd64  # or linux/arm64, etc.
        autoRemove: true
    command: uname -m
```

## Container Configuration

### Environment Variables

Pass environment variables to containers:

```yaml
env:
  - HOST_ENV: production

steps:
  - name: with-env
    executor:
      type: docker
      config:
        image: alpine
        autoRemove: true
        container:
          env:
            - APP_ENV=production
            - API_KEY=${HOST_ENV}  # Pass from host
            - DEBUG=true
    command: printenv
```

### Volume Mounts

Mount host directories into containers:

```yaml
steps:
  - name: with-volumes
    executor:
      type: docker
      config:
        image: python:3.11
        autoRemove: true
        host:
          binds:
            - /host/data:/container/data:ro      # Read-only mount
            - /host/output:/container/output:rw   # Read-write mount
            - ./app:/app                          # Relative path
    command: python /app/process.py /container/data
```

### Working Directory

Set the working directory in the container:

```yaml
steps:
  - name: working-dir
    executor:
      type: docker
      config:
        image: node:18
        autoRemove: true
        container:
          workingDir: /usr/src/app
        host:
          binds:
            - ./src:/usr/src/app
    command: npm install
```

### Container Naming

Assign custom names to containers:

```yaml
steps:
  - name: named-container
    executor:
      type: docker
      config:
        image: nginx
        containerName: my-nginx-server
        autoRemove: false  # Keep container after execution
    command: nginx -t
```

## Networking

### Network Configuration

Configure container networking:

```yaml
steps:
  - name: custom-network
    executor:
      type: docker
      config:
        image: alpine
        autoRemove: true
        network:
          EndpointsConfig:
            my-network:
              Aliases:
                - my-service
                - api-server
    command: ping -c 3 my-service
```

### Host Network Mode

Use host networking:

```yaml
steps:
  - name: host-network
    executor:
      type: docker
      config:
        image: nginx
        autoRemove: true
        host:
          networkMode: host
    command: nginx -t
```

## Advanced Container Options

### Resource Limits

Set CPU and memory limits:

```yaml
steps:
  - name: resource-limits
    executor:
      type: docker
      config:
        image: stress
        autoRemove: true
        host:
          cpuShares: 512
          memory: 536870912  # 512MB in bytes
    command: stress --cpu 1 --timeout 10s
```

### Security Options

Configure security settings:

```yaml
steps:
  - name: security-options
    executor:
      type: docker
      config:
        image: alpine
        autoRemove: true
        host:
          capAdd:
            - SYS_ADMIN
          capDrop:
            - MKNOD
          privileged: false
          readonlyRootfs: true
    command: id
```

### User Configuration

Run as specific user:

```yaml
steps:
  - name: run-as-user
    executor:
      type: docker
      config:
        image: alpine
        autoRemove: true
        container:
          user: "1000:1000"  # UID:GID
    command: id
```

## Exec Mode (Existing Containers)

### Basic Exec

Execute commands in running containers:

```yaml
steps:
  - name: exec-example
    executor:
      type: docker
      config:
        containerName: "my-app"
        exec:
          user: appuser
          workingDir: /app/data
          env:
            - TASK=cleanup
    command: ./maintenance.sh
```

### Dynamic Container Selection

Use variables for container names:

```yaml
env:
  - CONTAINER_NAME: production-app

steps:
  - name: dynamic-exec
    executor:
      type: docker
      config:
        containerName: ${CONTAINER_NAME}
        exec:
          user: root
    command: supervisorctl status
```

## Real-World Examples

### Data Processing Pipeline

```yaml
steps:
  - name: extract-data
    executor:
      type: docker
      config:
        image: python:3.11-slim
        autoRemove: true
        host:
          binds:
            - ./data:/data
            - ./scripts:/scripts
        container:
          env:
            - PYTHONPATH=/scripts
    command: python /scripts/extract.py

  - name: transform-data
    executor:
      type: docker
      config:
        image: apache/spark:3.4.0
        autoRemove: true
        host:
          binds:
            - ./data:/data
          memory: 2147483648  # 2GB
    command: spark-submit /data/transform.py
    depends: extract-data

  - name: load-data
    executor:
      type: docker
      config:
        image: postgres:15
        autoRemove: true
        container:
          env:
            - PGPASSWORD=${DB_PASSWORD}
        network:
          endpointsConfig:
            db-network: {}
    command: psql -h db -U user -d analytics -f /data/load.sql
    depends: transform-data
```

### Multi-Stage Build Process

```yaml
steps:
  - name: build-frontend
    executor:
      type: docker
      config:
        image: node:18-alpine
        autoRemove: true
        host:
          binds:
            - ./frontend:/app
        container:
          workingDir: /app
    command: |
      npm ci
      npm run build

  - name: build-backend
    executor:
      type: docker
      config:
        image: golang:1.21
        autoRemove: true
        host:
          binds:
            - ./backend:/app
        container:
          workingDir: /app
          env:
            - CGO_ENABLED=0
            - GOOS=linux
    command: go build -o server .

  - name: package-app
    executor:
      type: docker
      config:
        image: docker:dind
        autoRemove: true
        host:
          binds:
            - /var/run/docker.sock:/var/run/docker.sock
            - .:/workspace
        container:
          workingDir: /workspace
    command: docker build -t myapp:latest .
    depends:
      - build-frontend
      - build-backend
```

### Database Operations

```yaml
steps:
  - name: backup-database
    executor:
      type: docker
      config:
        image: mysql:8.0
        autoRemove: true
        container:
          env:
            - MYSQL_PWD=${DB_PASSWORD}
        host:
          binds:
            - ./backups:/backups
    command: |
      mysqldump -h db.example.com -u root \
        --all-databases --single-transaction \
        > /backups/backup-$(date +%Y%m%d).sql

  - name: compress-backup
    executor:
      type: docker
      config:
        image: alpine
        autoRemove: true
        host:
          binds:
            - ./backups:/backups
    command: gzip /backups/backup-*.sql
    depends: backup-database
```

## Docker in Docker (DinD)

### Using Host Docker Socket

Mount the Docker socket to use host's Docker:

```yaml
steps:
  - name: docker-in-docker
    executor:
      type: docker
      config:
        image: docker:latest
        autoRemove: true
        host:
          binds:
            - /var/run/docker.sock:/var/run/docker.sock
    command: docker ps
```

### With Docker Compose

For Dagu running in a container:

```yaml
# docker-compose.yml
services:
  dagu:
    image: ghcr.io/dagu-org/dagu:latest
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./dags:/home/dagu/.config/dagu/dags
    environment:
      - DOCKER_HOST=unix:///var/run/docker.sock
    group_add:
      - ${DOCKER_GID}  # Pass host's docker group ID
```

### Using TCP Socket

Connect to remote Docker daemon:

```yaml
env:
  - DOCKER_HOST: tcp://docker-host:2376

steps:
  - name: remote-docker
    executor:
      type: docker
      config:
        image: alpine
        autoRemove: true
    command: echo "Using remote Docker"
```

## Error Handling

### Container Exit Codes

Handle specific container exit codes:

```yaml
steps:
  - name: handle-exit-codes
    executor:
      type: docker
      config:
        image: alpine
        autoRemove: true
    command: exit 42
    continueOn:
      exitCode: [42]  # Continue if exit code is 42
```

### Cleanup on Failure

Ensure cleanup even on errors:

```yaml
steps:
  - name: with-cleanup
    executor:
      type: docker
      config:
        image: busybox
        containerName: temp-processor
        autoRemove: true  # Always remove, even on failure
    command: process_data.sh
```

## Performance Optimization

### Image Caching

```yaml
steps:
  - name: use-cache
    executor:
      type: docker
      config:
        image: my-app:latest
        pull: missing  # Use cached image if available
        autoRemove: true
    command: ./run.sh
```

### Layer Caching for Builds

```yaml
steps:
  - name: efficient-build
    executor:
      type: docker
      config:
        image: docker:latest
        autoRemove: true
        host:
          binds:
            - /var/run/docker.sock:/var/run/docker.sock
            - .:/app
    command: |
      docker build \
        --cache-from my-app:latest \
        -t my-app:latest \
        /app
```

## See Also

- Learn about [SSH Executor](/features/executors/ssh) for remote execution
- Explore [HTTP Executor](/features/executors/http) for API interactions
- Check out [Execution Control](/features/execution-control) for advanced patterns
