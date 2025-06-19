# Docker Executor

Run workflow steps in Docker containers for isolated, reproducible execution.

## Basic Usage

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

## Configuration Options

### Image Management

```yaml
executor:
  type: docker
  config:
    image: python:3.11
    pull: always        # always, missing, never
    platform: linux/amd64
    autoRemove: true
```

### Volume Mounts

```yaml
executor:
  type: docker
  config:
    image: node:18
    host:
      binds:
        - /host/data:/container/data:ro      # Read-only
        - ./output:/output:rw                 # Read-write
        - ${PWD}/app:/app                     # Current directory
```

### Environment Variables

```yaml
executor:
  type: docker
  config:
    image: postgres:15
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

## Real-World Examples

### Data Processing Pipeline

```yaml
steps:
  - name: extract
    executor:
      type: docker
      config:
        image: python:3.11-slim
        autoRemove: true
        host:
          binds:
            - ./data:/data
    command: python extract.py /data/raw.csv

  - name: transform
    executor:
      type: docker
      config:
        image: apache/spark:3.4.0
        autoRemove: true
        host:
          binds:
            - ./data:/data
          memory: 2147483648  # 2GB
    command: spark-submit transform.py

  - name: load
    executor:
      type: docker
      config:
        image: postgres:15
        autoRemove: true
        container:
          env:
            - PGPASSWORD=${DB_PASSWORD}
    command: psql -h db -U user -f load.sql
```

### Build and Deploy

```yaml
steps:
  - name: build
    executor:
      type: docker
      config:
        image: golang:1.21
        autoRemove: true
        host:
          binds:
            - .:/app
        container:
          workingDir: /app
          env:
            - CGO_ENABLED=0
    command: go build -o server .

  - name: test
    executor:
      type: docker
      config:
        image: golang:1.21
        autoRemove: true
        host:
          binds:
            - .:/app
        container:
          workingDir: /app
    command: go test ./...

  - name: package
    executor:
      type: docker
      config:
        image: docker:latest
        autoRemove: true
        host:
          binds:
            - /var/run/docker.sock:/var/run/docker.sock
            - .:/workspace
    command: docker build -t myapp:${VERSION} /workspace
```

### Database Operations

```yaml
steps:
  - name: backup
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

  - name: compress
    executor:
      type: docker
      config:
        image: alpine
        autoRemove: true
        host:
          binds:
            - ./backups:/backups
    command: gzip /backups/backup-*.sql
```

## Docker in Docker

### Using Host Docker Socket

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
# docker-compose.yml for Dagu with Docker support
services:
  dagu:
    image: ghcr.io/dagu-org/dagu:latest
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./dags:/var/lib/dagu/dags
    user: "0:0"  # Run as root for Docker access
```

## Advanced Patterns

### Multi-Stage Processing

```yaml
steps:
  - name: compile
    executor:
      type: docker
      config:
        image: rust:1.70
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
        image: golang:1.21
        platform: linux/amd64
        autoRemove: true
    command: GOARCH=amd64 go build

  - name: build-arm64
    executor:
      type: docker
      config:
        image: golang:1.21
        platform: linux/arm64
        autoRemove: true
    command: GOARCH=arm64 go build
```

## Error Handling

### Continue on Specific Exit Codes

```yaml
steps:
  - name: check-data
    executor:
      type: docker
      config:
        image: alpine
        autoRemove: true
    command: test -f /data/input.csv
    continueOn:
      exitCode: [1]  # File not found is OK

  - name: process-if-exists
    executor:
      type: docker
      config:
        image: python:3.11
        autoRemove: true
    command: python process.py || echo "No data to process"
```

## Performance Tips

1. **Use specific tags** instead of `latest` for reproducibility
2. **Set `pull: missing`** to use cached images
3. **Remove containers** with `autoRemove: true`
4. **Mount only necessary directories** to minimize overhead
5. **Use multi-stage builds** for smaller final images

## Security Considerations

- **Docker socket access** grants full control over host Docker
- **Run as non-root** when possible using `container.user`
- **Use read-only mounts** for input data
- **Avoid mounting sensitive host directories**
- **Set resource limits** to prevent resource exhaustion

## See Also

- [Shell Executor](/features/executors/shell) - Run commands directly
- [SSH Executor](/features/executors/ssh) - Execute on remote hosts
- [Execution Control](/features/execution-control) - Advanced patterns
