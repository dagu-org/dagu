# Executors

Executors define how steps in your DAG are executed. Dagu provides multiple executor types to handle different execution environments and use cases.

## Available Executors

### Shell Executor (Default)

The shell executor runs commands in the system shell.

```yaml
steps:
  - name: shell step
    command: echo "Hello from shell"
    shell: bash  # Optional: specify shell (sh, bash, zsh)
```

#### Shell Selection

```yaml
steps:
  - name: custom shell
    shell: /usr/local/bin/fish
    command: echo "Running in Fish shell"
```

### Docker Executor

Run commands inside Docker containers.

```yaml
steps:
  - name: docker step
    executor:
      type: docker
      config:
        image: python:3.11-slim
        pull: missing  # always, never, missing
        autoRemove: true
        container:
          env:
            - PYTHONPATH=/app
          workingDir: /app
        host:
          binds:
            - /local/data:/container/data:ro
    command: python script.py
```

#### Exec into Existing Container

```yaml
steps:
  - name: exec in container
    executor:
      type: docker
      config:
        containerName: my-running-app
        exec:
          user: appuser
          workingDir: /app
    command: ./update-cache.sh
```

### SSH Executor

Execute commands on remote servers via SSH.

```yaml
steps:
  - name: remote command
    executor:
      type: ssh
      config:
        user: deploy
        host: server.example.com
        port: 22
        key: /home/user/.ssh/id_rsa
    command: systemctl restart myapp
```

### HTTP Executor

Make HTTP requests and process responses.

```yaml
steps:
  - name: api call
    executor:
      type: http
      config:
        method: POST
        url: https://api.example.com/webhook
        headers:
          Content-Type: application/json
          Authorization: Bearer ${API_TOKEN}
        body: |
          {
            "status": "completed",
            "timestamp": "${TIMESTAMP}"
          }
        timeout: 30
        silent: false
```

### Mail Executor

Send emails with optional attachments.

```yaml
steps:
  - name: send notification
    executor:
      type: mail
      config:
        to: team@example.com
        from: alerts@example.com
        subject: "Build ${BUILD_ID} completed"
        message: |
          Build completed successfully.
          Time: ${BUILD_TIME}
        attachments:
          - ${DAG_RUN_LOG_FILE}
```

### JQ Executor

Process JSON data using jq queries.

```yaml
steps:
  - name: process json
    executor:
      type: jq
      config:
        input: ${API_RESPONSE}
        query: .results[] | select(.status == "active") | .id
```

### Nested DAG Executor

Execute another DAG as a step.

```yaml
steps:
  - name: run sub-workflow
    run: workflows/data-transform
    params: "INPUT=${DATA_PATH} OUTPUT=${OUTPUT_PATH}"
    output: TRANSFORM_RESULT
```

## Executor Configuration

### Global Executor Settings

Set default executor for all steps:

```yaml
defaults:
  executor:
    type: docker
    config:
      image: myapp:latest

steps:
  - name: step1
    command: echo "Runs in Docker"
  
  - name: step2
    executor:
      type: shell  # Override default
    command: echo "Runs in shell"
```

### Environment Variables

All executors support environment variables:

```yaml
steps:
  - name: with env
    executor:
      type: docker
      config:
        image: alpine
        container:
          env:
            - APP_ENV=production
            - DEBUG=false
    command: printenv
```

### Working Directory

Set working directory for any executor:

```yaml
steps:
  - name: in directory
    dir: /app/workspace
    command: ls -la
```

## Best Practices

1. **Choose the Right Executor**
   - Use shell for simple local commands
   - Use Docker for isolated environments
   - Use SSH for remote operations
   - Use HTTP for API integrations

2. **Security Considerations**
   - Store SSH keys securely
   - Use environment variables for secrets
   - Limit container permissions
   - Use read-only mounts when possible

3. **Performance Tips**
   - Reuse running containers with exec
   - Cache Docker images locally
   - Use connection pooling for SSH
   - Set appropriate timeouts

## Advanced Examples

### Multi-Stage Docker Pipeline

```yaml
steps:
  - name: build
    executor:
      type: docker
      config:
        image: golang:1.21
        host:
          binds:
            - .:/workspace
        container:
          workingDir: /workspace
    command: go build -o app

  - name: test
    executor:
      type: docker
      config:
        image: golang:1.21
        host:
          binds:
            - .:/workspace:ro
        container:
          workingDir: /workspace
    command: go test ./...
    depends: build

  - name: deploy
    executor:
      type: docker
      config:
        image: alpine
        host:
          binds:
            - ./app:/app:ro
    command: cp /app /deployment/
    depends: test
```

### Conditional Executor Selection

```yaml
env:
  - ENVIRONMENT: ${ENVIRONMENT}

steps:
  - name: deploy
    executor:
      type: "`[ \"$ENVIRONMENT\" = \"local\" ] && echo shell || echo ssh`"
      config:
        $if: ${ENVIRONMENT} != "local"
        user: deploy
        host: ${DEPLOY_HOST}
    command: ./deploy.sh
```