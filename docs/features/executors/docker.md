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
  - pip install -r requirements.txt
  - python process.py /data/input.csv
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
  workingDir: /app                    # Working directory
  platform: linux/amd64            # Platform specification
  user: "1000:1000"                # User and group
  ports:
    - "8080:8080"                  # Port mapping
  network: host                    # Network mode
  keepContainer: true              # Keep container running
```

### How Commands Execute with DAG-level Container

When you configure a DAG‑level `container`, Dagu starts a single persistent
container (kept alive with a simple sleep loop) and executes each step inside
it using `docker exec`.

- Step commands run directly in the running container; the image’s
  `ENTRYPOINT`/`CMD` are not invoked for step commands.
- If your image’s entrypoint is a dispatcher (for example, it expects a job
  name like `sendConfirmationEmails`), call that entrypoint explicitly in your
  step command, or invoke the underlying program yourself.

Example:

```yaml
container:
  image: myorg/myimage:latest

steps:
  # Runs inside the already-running container via `docker exec`
  - my-entrypoint sendConfirmationEmails
```

If you prefer each step to honor the image’s `ENTRYPOINT`/`CMD` as with a
fresh `docker run`, use the step‑level Docker executor instead of a DAG‑level
`container`.

## Registry Authentication

Access private container registries with authentication configured at the DAG level:

```yaml
# Option 1: Structured format with username/password
registryAuths:
  docker.io:
    username: ${DOCKER_USERNAME}
    password: ${DOCKER_PASSWORD}
  ghcr.io:
    username: ${GITHUB_USER}
    password: ${GITHUB_TOKEN}

# Use private images in your workflow
container:
  image: ghcr.io/myorg/private-app:latest

steps:
  - python process.py
```

### Authentication Methods

**Using Environment Variables:**

```yaml
# Set DOCKER_AUTH_CONFIG with standard Docker format
registryAuths: ${DOCKER_AUTH_CONFIG}

# DOCKER_AUTH_CONFIG uses the same format as ~/.docker/config.json:
# {
#   "auths": {
#     "docker.io": {
#       "auth": "base64(username:password)"
#     }
#   }
# }
```

**Pre-encoded Authentication:**

```yaml
registryAuths:
  gcr.io:
    auth: ${GCR_AUTH_BASE64}  # base64(username:password)
```

**Per-Registry JSON Configuration:**

```yaml
registryAuths:
  # JSON string for specific registry
  "123456789012.dkr.ecr.us-east-1.amazonaws.com": |
    {
      "username": "AWS",
      "password": "${ECR_AUTH_TOKEN}"
    }
```

### Authentication Priority

Dagu checks for registry credentials in this order:

1. **DAG-level `registryAuths`** - Configured in your DAG file
2. **`DOCKER_AUTH_CONFIG` environment variable** - Standard Docker authentication (same format as `~/.docker/config.json`)
3. **No authentication** - For public registries

> **Note:** The `DOCKER_AUTH_CONFIG` format is fully compatible with Docker's standard `~/.docker/config.json` file. You can copy the contents of your Docker config file directly into the environment variable.

### Using Docker Config File

You can export your existing Docker configuration as an environment variable:

```bash
# Export your Docker config as an environment variable
export DOCKER_AUTH_CONFIG=$(cat ~/.docker/config.json)

# Then run your DAG - it will use the Docker credentials automatically
dagu run my-workflow.yaml
```

### Example: Multi-Registry Workflow

```yaml
registryAuths:
  docker.io:
    username: ${DOCKERHUB_USER}
    password: ${DOCKERHUB_TOKEN}
  ghcr.io:
    username: ${GITHUB_USER}
    password: ${GITHUB_TOKEN}
  gcr.io:
    auth: ${GCR_AUTH}

steps:
  - executor:
      type: docker
      config:
        image: myorg/processor:latest  # from Docker Hub
    command: process-data
    
  - executor:
      type: docker
      config:
        image: ghcr.io/myorg/analyzer:v2  # from GitHub
    command: analyze-results
    
  - executor:
      type: docker
      config:
        image: gcr.io/myproject/reporter:stable  # from GCR
    command: generate-report
```

## Docker in Docker

You need to mount the Docker socket and run as root to use Docker inside your containers. Example `compose.yml` for running Dagu with Docker support:

```yaml
# compose.yml for Dagu with Docker support
services:
  dagu:
    image: ghcr.io/dagu-org/dagu:latest
    ports:
      - 8080:8080
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./dags:/var/lib/dagu/dags
    entrypoint: ["dagu", "start-all"] # Override default entrypoint
    user: "0:0"                       # Run as root for Docker access

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
```

## Platform-Specific Builds

```yaml
container:
  image: postgres:16
  platform: linux/amd64
  env:
    - POSTGRES_PASSWORD=secret
  ports:
    - "5432:5432"
```
