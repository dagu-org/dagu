# Docker Executor

Run workflow steps in Docker containers for isolated, reproducible execution.

## Container Field

Use the `container` field at the DAG level to run all steps in containers:

```yaml
# All steps run in this container
container:
  image: python:3.11
  volumes:
    - ./data:/data
  env:
    - PYTHONPATH=/app

steps:
  - name: install
    command: pip install -r requirements.txt
    
  - name: process
    command: python process.py /data/input.csv
```

## Step-Level Container Configuration

```yaml
steps:
  - name: hello
    executor:
      type: docker
      config:
        image: alpine
        autoRemove: true
    command: echo "hello from container"
```

## Execute in Existing Container

```yaml
steps:
  - name: exec-in-running
    executor:
      type: docker
      config:
        containerName: my-app
        exec:
          user: root
          workingDir: /app
    command: ./maintenance.sh
```

## Container Field Configuration

The `container` field supports all Docker configuration options:

```yaml
container:
  image: node:20                    # Required
  pullPolicy: missing               # always, missing, never
  env:
    - NODE_ENV=production
    - API_KEY=${API_KEY}           # From host environment
  volumes:
    - ./src:/app                   # Bind mount
    - /data:/data:ro               # Read-only mount
  workDir: /app                    # Working directory
  platform: linux/amd64            # Platform specification
  user: "1000:1000"                # User and group
  ports:
    - "8080:8080"                  # Port mapping
  network: host                    # Network mode
  keepContainer: true              # Keep container running
```

## Configuration Options for Docker Executor

### Image Management

```yaml
executor:
  type: docker
  config:
    image: python:3.13
    pull: always        # always, missing, never
    platform: linux/amd64
    autoRemove: true
```

### Volume Mounts

```yaml
executor:
  type: docker
  config:
    image: node:22
    host:
      binds:
        - /host/data:/container/data:ro # Read-only
        - ${PWD}/app:/app               # Current directory
```

### Environment Variables

```yaml
executor:
  type: docker
  config:
    image: postgres:17
    container:
      env:
        - POSTGRES_PASSWORD=secret
        - POSTGRES_USER=admin
        - DEBUG=${DEBUG}  # From host
```

### Working Directory

```yaml
executor:
  type: docker
  config:
    image: maven:3
    container:
      workingDir: /project
    host:
      binds:
        - ./:/project
```

### Resource Limits

```yaml
executor:
  type: docker
  config:
    image: stress
    host:
      memory: 536870912  # 512MB
      cpuShares: 512
```

## Docker in Docker

### Using Container Field with Docker Socket

```yaml
# Run all steps with Docker access
container:
  image: docker:latest
  volumes:
    - /var/run/docker.sock:/var/run/docker.sock
    - ./workspace:/workspace

steps:
  - name: list-containers
    command: docker ps
    
  - name: build-image
    command: docker build -t myapp:latest /workspace
    
  - name: run-tests
    command: docker run --rm myapp:latest npm test
```

### Using Executor with Docker Socket

```yaml
steps:
  - name: docker-operations
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

### Docker Compose Setup

```yaml
# compose.yml for Dagu with Docker support
services:
  dagu:
    image: ghcr.io/dagu-org/dagu:latest
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./dags:/var/lib/dagu/dags
    user: "0:0"  # Run as root for Docker access
```

## Container Lifecycle Management

The `keepContainer` option prevents the container from being removed after the workflow completes, allowing for debugging and inspection:

```yaml
# Container stays alive for entire workflow
container:
  image: postgres:16
  keepContainer: true
  env:
    - POSTGRES_PASSWORD=secret
  ports:
    - "5432:5432"

steps:
  - name: start-db
    command: docker-entrypoint.sh postgres
    
  - name: wait-for-db
    command: pg_isready -U postgres
    retryPolicy:
      limit: 10
      intervalSec: 2
      
  - name: create-schema
    command: psql -U postgres -c "CREATE DATABASE myapp;"
    
  - name: run-migrations
    command: psql -U postgres myapp -f migrations.sql
```

## Advanced Patterns

### Multi-Stage Processing

```yaml
steps:
  - name: compile
    executor:
      type: docker
      config:
        image: rust:1.88
        autoRemove: true
        host:
          binds:
            - ./src:/src
            - ./target:/target
    command: cargo build --release

  - name: package
    executor:
      type: docker
      config:
        image: alpine
        autoRemove: true
        host:
          binds:
            - ./target/release:/app
    command: tar -czf app.tar.gz /app
```

### Platform-Specific Builds

```yaml
steps:
  - name: build-amd64
    executor:
      type: docker
      config:
        image: golang:1.24
        platform: linux/amd64
        autoRemove: true
    command: GOARCH=amd64 go build

  - name: build-arm64
    executor:
      type: docker
      config:
        image: golang:1.24
        platform: linux/arm64
        autoRemove: true
    command: GOARCH=arm64 go build
```

## See Also

- [Shell Executor](/features/executors/shell) - Run commands directly
- [SSH Executor](/features/executors/ssh) - Execute on remote hosts
- [Execution Control](/features/execution-control) - Advanced patterns
