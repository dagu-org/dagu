# Executors Reference

## Overview

Executors extend Dagu's capabilities beyond simple shell commands. Available executors:

- [Shell](/features/executors/shell) (default) - Execute shell commands
- [Docker](/features/executors/docker) - Run commands in Docker containers
- [SSH](/features/executors/ssh) - Execute commands on remote hosts
- [HTTP](/features/executors/http) - Make HTTP requests
- [Mail](/features/executors/mail) - Send emails
- [JQ](/features/executors/jq) - Process JSON data
- [GitHub Actions (_experimental_)](/features/executors/github-actions) - Run marketplace actions locally with nektos/act

::: tip
For detailed documentation on each executor, click the links above to visit the feature pages.
:::

## Shell Executor (Default)

::: info
For detailed Shell executor documentation, see [Shell Executor Guide](/features/executors/shell).
:::

The default executor runs commands in the system shell.

```yaml
steps:
  - command: echo "Hello World"
    
  - command: echo $BASH_VERSION
    shell: bash  # Use specific shell
```

### Shell Selection

```yaml
steps:
  - name: default-shell
    command: echo "Uses $SHELL or /bin/sh"
    
  - name: bash-specific
    shell: bash
    command: echo "Uses bash features"
    
  - name: custom-shell
    shell: /usr/bin/zsh
    command: echo "Uses zsh"
```

## Docker Executor

::: info
For detailed Docker executor documentation, see [Docker Executor Guide](/features/executors/docker).
:::

Run commands in Docker containers for isolation and reproducibility.

### Create and Run Container

```yaml
steps:
  - name: run-in-container
    executor:
      type: docker
      config:
        image: alpine:latest
        autoRemove: true
    command: echo "Hello from container"
```

::: tip
If `command` is omitted for a step that creates a new container (`config.image`), Docker uses the image’s default `ENTRYPOINT`/`CMD`.
:::

### Image Pull Options

```yaml
steps:
  - name: pull-always
    executor:
      type: docker
      config:
        image: myapp:latest
        pull: always      # Always pull from registry
        autoRemove: true
    command: ./app
    
  - name: pull-if-missing
    executor:
      type: docker
      config:
        image: myapp:latest
        pull: missing     # Default - pull only if not local
        autoRemove: true
    command: ./app
    
  - name: never-pull
    executor:
      type: docker
      config:
        image: local-image:dev
        pull: never       # Use local image only
        autoRemove: true
    command: ./test
```

### Registry Authentication

```yaml
# Configure authentication for private registries
registryAuths:
  docker.io:
    username: ${DOCKER_USERNAME}
    password: ${DOCKER_PASSWORD}
  ghcr.io:
    username: ${GITHUB_USER}
    password: ${GITHUB_TOKEN}

steps:
  - name: use-private-image
    executor:
      type: docker
      config:
        image: ghcr.io/myorg/private-app:latest
        autoRemove: true
    command: echo "Running"
```

Authentication can also be configured via `DOCKER_AUTH_CONFIG` environment variable.

### Volume Mounts

```yaml
steps:
  - name: with-volumes
    executor:
      type: docker
      config:
        image: python:3.13
        autoRemove: true
        host:
          binds:
            - /host/data:/container/data:ro      # Read-only
            - /host/output:/container/output:rw  # Read-write
            - ./config:/app/config               # Relative path
    command: python process.py /container/data
```

## GitHub Actions Executor

::: info
For the full guide, see [GitHub Actions Executor](/features/executors/github-actions).
:::

Run marketplace actions (e.g. `actions/checkout@v4`) inside Dagu steps.

```yaml
secrets:
  - name: GITHUB_TOKEN
    provider: env
    key: GITHUB_TOKEN

steps:
  - name: checkout
    command: actions/checkout@v4
    executor:
      type: gha             # Aliases: github_action, github-action
      config:
        runner: node:24-bookworm
    params:
      repository: dagu-org/dagu
      ref: main
      token: "${GITHUB_TOKEN}"
```

::: warning
This executor is experimental. It depends on Docker, downloads images on demand, and currently supports single-action invocations per step.
:::

### Environment Variables

```yaml
env:
  - API_KEY: secret123

steps:
  - name: with-env
    executor:
      type: docker
      config:
        image: node:22
        autoRemove: true
        container:
          env:
            - NODE_ENV=production
            - API_KEY=${API_KEY}  # Pass from DAG env
            - DB_HOST=postgres
    command: npm start
```

### Network Configuration

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
                - my-alias
    command: ping other-service
```

### Platform Selection

```yaml
steps:
  - name: specific-platform
    executor:
      type: docker
      config:
        image: myapp:latest
        platform: linux/amd64  # Force platform
        autoRemove: true
    command: ./app
```

### Working Directory

```yaml
steps:
  - name: custom-workdir
    executor:
      type: docker
      config:
        image: python:3.13
        autoRemove: true
        container:
          workingDir: /app
          env:
            - PYTHONPATH=/app
        host:
          binds:
            - ./src:/app
    command: python main.py
```

### Execute in Existing Container

```yaml
steps:
  - name: exec-in-running
    executor:
      type: docker
      config:
        containerName: my-app-container
        exec:
          user: root
          workingDir: /app
          env:
            - DEBUG=true
    command: echo "Debug mode"
```

::: info
Validation: Set at least `config.image` or `config.containerName`. If both are omitted, the step fails validation. Supplying only `containerName` requires the container to already be running. When both are set, Dagu first tries to exec into the named container; if it is missing or stopped, Dagu creates it using the provided image (applying any `container`/`host`/`network` settings) before running the command.
:::

::: warning
When a DAG‑level `container:` is configured, Docker‑executor steps run inside that shared container via `docker exec`. In this case, the step’s Docker `config` (including `image`, `container/host/network`, and `exec`) is ignored; only the step’s `command` and `args` are used.
:::

### Complete Docker Example

```yaml
steps:
  - name: complex-docker
    executor:
      type: docker
      config:
        image: postgres:17
        containerName: test-db
        pull: missing
        platform: linux/amd64
        autoRemove: false
        container:
          env:
            - POSTGRES_USER=test
            - POSTGRES_PASSWORD=test
            - POSTGRES_DB=testdb
          exposedPorts:
            5432/tcp: {}
        host:
          binds:
            - postgres-data:/var/lib/postgresql/data
          portBindings:
            5432/tcp:
              - hostIP: "127.0.0.1"
                hostPort: "5432"
        network:
          EndpointsConfig:
            bridge:
              Aliases:
                - postgres-test
    command: postgres
```

## SSH Executor

::: info
For detailed SSH executor documentation, see [SSH Executor Guide](/features/executors/ssh).
:::

Execute commands on remote hosts over SSH.

### Basic SSH

```yaml
steps:
  - name: remote-command
    executor:
      type: ssh
      config:
        user: deploy
        host: server.example.com
        port: 22
        key: /home/user/.ssh/id_rsa
    command: ls -la /var/www
```

### With Environment

```yaml
steps:
  - name: remote-with-env
    executor:
      type: ssh
      config:
        user: deploy
        host: 192.168.1.100
        key: ~/.ssh/deploy_key
    command: |
      export APP_ENV=production
      cd /opt/app
      echo "Deploying"
```

### Multiple Commands

```yaml
steps:
  - name: remote-script
    executor:
      type: ssh
      config:
        user: admin
        host: backup.server.com
        key: ${SSH_KEY_PATH}
    script: |
      #!/bin/bash
      set -e
      
      echo "Starting backup..."
      tar -czf /backup/app-$(date +%Y%m%d).tar.gz /var/www
      
      echo "Cleaning old backups..."
      find /backup -name "app-*.tar.gz" -mtime +7 -delete
      
      echo "Backup complete"
```

## HTTP Executor

::: info
For detailed HTTP executor documentation, see [HTTP Executor Guide](/features/executors/http).
:::

Make HTTP requests to APIs and web services.

### GET Request

```yaml
steps:
  - name: simple-get
    executor:
      type: http
      config:
        silent: true  # Output body only
    command: GET https://api.example.com/status
```

### POST with Body

```yaml
steps:
  - name: post-json
    executor:
      type: http
      config:
        headers:
          Content-Type: application/json
          Authorization: Bearer ${API_TOKEN}
        body: |
          {
            "name": "test",
            "value": 123
          }
        timeout: 30
    command: POST https://api.example.com/data
```

### Query Parameters

```yaml
steps:
  - name: search-api
    executor:
      type: http
      config:
        query:
          q: "dagu workflow"
          limit: "10"
          offset: "0"
        silent: true
    command: GET https://api.example.com/search
```

### Form Data

```yaml
steps:
  - name: form-submit
    executor:
      type: http
      config:
        headers:
          Content-Type: application/x-www-form-urlencoded
        body: "username=user&password=pass&remember=true"
    command: POST https://example.com/login
```

### Self-Signed Certificates

```yaml
steps:
  - name: internal-api
    executor:
      type: http
      config:
        skipTLSVerify: true  # Skip certificate verification
        headers:
          Authorization: Bearer ${INTERNAL_TOKEN}
    command: GET https://internal-api.local/data
```

### Complete HTTP Example

```yaml
steps:
  - name: api-workflow
    executor:
      type: http
      config:
        headers:
          Accept: application/json
          X-API-Key: ${API_KEY}
        timeout: 60
        silent: false
    command: GET https://api.example.com/data
    output: API_RESPONSE
    
  - name: process-response
    command: echo "${API_RESPONSE}" | jq '.data[]'
```

## Mail Executor

::: info
For detailed Mail executor documentation, see [Mail Executor Guide](/features/executors/mail).
:::

Send emails for notifications and alerts.

### Basic Email

```yaml
smtp:
  host: smtp.gmail.com
  port: "587"
  username: sender@gmail.com
  password: ${SMTP_PASSWORD}

steps:
  - name: send-notification
    executor:
      type: mail
      config:
        to: recipient@example.com
        from: sender@gmail.com
        subject: "Workflow Completed"
        message: "The data processing workflow has completed successfully."
```

### With Attachments

```yaml
steps:
  - name: send-report
    executor:
      type: mail
      config:
        to: team@company.com
        from: reports@company.com
        subject: "Daily Report - ${TODAY}"
        message: |
          Please find attached the daily report.
          
          Generated at: ${TIMESTAMP}
        attachments:
          - /tmp/daily-report.pdf
          - /tmp/summary.csv
```

### Multiple Recipients

```yaml
steps:
  - name: alert-team
    executor:
      type: mail
      config:
        to: 
          - ops@company.com
          - alerts@company.com
          - oncall@company.com
        from: dagu@company.com
        subject: "[ALERT] Process Failed"
        message: |
          The critical process has failed.
          
          Error: ${ERROR_MESSAGE}
          Time: ${TIMESTAMP}
```

### HTML Email

```yaml
steps:
  - name: send-html
    executor:
      type: mail
      config:
        to: marketing@company.com
        from: notifications@company.com
        subject: "Weekly Stats"
        contentType: text/html
        message: |
          <html>
          <body>
            <h2>Weekly Statistics</h2>
            <p>Users: <strong>${USER_COUNT}</strong></p>
            <p>Revenue: <strong>${REVENUE}</strong></p>
          </body>
          </html>
```

## JQ Executor

::: info
For detailed JQ executor documentation, see [JQ Executor Guide](/features/executors/jq).
:::

Process and transform JSON data using jq syntax.

### Format JSON

```yaml
steps:
  - name: pretty-print
    executor: jq
    script: |
      {"name":"test","values":[1,2,3],"nested":{"key":"value"}}
```

Output:
```json
{
  "name": "test",
  "values": [1, 2, 3],
  "nested": {
    "key": "value"
  }
}
```

### Query JSON

```yaml
steps:
  - name: extract-value
    executor: jq
    command: '.data.users[] | select(.active == true) | .email'
    script: |
      {
        "data": {
          "users": [
            {"id": 1, "email": "user1@example.com", "active": true},
            {"id": 2, "email": "user2@example.com", "active": false},
            {"id": 3, "email": "user3@example.com", "active": true}
          ]
        }
      }
```

Output:
```
"user1@example.com"
"user3@example.com"
```

### Transform JSON

```yaml
steps:
  - name: transform-data
    executor: jq
    command: '{id: .id, name: .name, total: (.items | map(.price) | add)}'
    script: |
      {
        "id": "order-123",
        "name": "Test Order",
        "items": [
          {"name": "Item 1", "price": 10.99},
          {"name": "Item 2", "price": 25.50},
          {"name": "Item 3", "price": 5.00}
        ]
      }
```

Output:
```json
{
  "id": "order-123",
  "name": "Test Order",
  "total": 41.49
}
```

### Complex Processing

```yaml
steps:
  - name: analyze-logs
    executor: jq
    command: |
      group_by(.level) | 
      map({
        level: .[0].level,
        count: length,
        messages: map(.message)
      })
    script: |
      [
        {"level": "ERROR", "message": "Connection failed"},
        {"level": "INFO", "message": "Process started"},
        {"level": "ERROR", "message": "Timeout occurred"},
        {"level": "INFO", "message": "Process completed"}
      ]
```

## DAG Executor

::: info
DAG executor allows running other workflows as steps. See [Nested Workflows](/writing-workflows/control-flow#nested-workflows).
:::

Execute other workflows as steps, enabling workflow composition.

### Execute External DAG

```yaml
steps:
  - name: run-etl
    executor: dag
    command: workflows/etl-pipeline.yaml
    params: "DATE=${TODAY} ENV=production"
```

### Execute Local DAG

```yaml
name: main-workflow
steps:
  - name: prepare-data
    executor: dag
    command: data-prep
    params: "SOURCE=/data/raw"

---

name: data-prep
params:
  - SOURCE: /tmp
steps:
  - name: validate
    command: validate.sh ${SOURCE}
  - name: clean
    command: clean.py ${SOURCE}
```

### Capture DAG Output

```yaml
steps:
  - name: analyze
    executor: dag
    command: analyzer.yaml
    params: "FILE=${INPUT_FILE}"
    output: ANALYSIS
    
  - name: use-results
    command: |
      echo "Status: ${ANALYSIS.outputs.status}"
      echo "Count: ${ANALYSIS.outputs.record_count}"
```

### Error Handling

```yaml
steps:
  - name: may-fail
    executor: dag
    command: risky-process.yaml
    continueOn:
      failure: true
    retryPolicy:
      limit: 3
      intervalSec: 300
```

### Dynamic DAG Selection

```yaml
steps:
  - name: choose-workflow
    command: |
      if [ "${ENVIRONMENT}" = "prod" ]; then
        echo "production-workflow.yaml"
      else
        echo "staging-workflow.yaml"
      fi
    output: WORKFLOW_FILE
    
  - name: run-selected
    executor: dag
    command: ${WORKFLOW_FILE}
    params: "ENV=${ENVIRONMENT}"
```

## See Also

- [Shell Executor](/features/executors/shell) - Shell command execution details
- [Docker Executor](/features/executors/docker) - Container execution guide
- [SSH Executor](/features/executors/ssh) - Remote execution guide
- [HTTP Executor](/features/executors/http) - API interaction guide
- [Mail Executor](/features/executors/mail) - Email notification guide
- [JQ Executor](/features/executors/jq) - JSON processing guide
- [Writing Workflows](/writing-workflows/) - Using executors in workflows
- [Examples](https://github.com/dagu-org/dagu/tree/main/examples) - Real-world executor usage
