# Container Field

Run all workflow steps in a shared Docker container for consistent execution environment.

## Basic Usage

```yaml
container:
  image: python:3.11

steps:
  - pip install pandas numpy  # Install dependencies
  - python process.py          # Process data
```

All steps run in the same container instance, sharing the filesystem and installed packages.

## With Volume Mounts

```yaml
container:
  image: node:20
  volumes:
    - ./src:/app
    - ./data:/data
  workingDir: /app

steps:
  - npm install    # Install dependencies
  - npm run build  # Build the application
  - npm test       # Run tests
```

## With Environment Variables

```yaml
container:
  image: postgres:16
  env:
    - POSTGRES_PASSWORD=secret
    - POSTGRES_DB=myapp

steps:
  - command: pg_isready -U postgres
    retryPolicy:
      limit: 10
      
  - psql -U postgres myapp -f schema.sql
```

## Private Registry Authentication

```yaml
# For private images
registryAuths:
  ghcr.io:
    username: ${GITHUB_USER}
    password: ${GITHUB_TOKEN}

container:
  image: ghcr.io/myorg/private-app:latest

steps:
  - ./app
```

Or use `DOCKER_AUTH_CONFIG` environment variable (same format as `~/.docker/config.json`).

## Configuration Options

```yaml
container:
  image: ubuntu:22.04        # Required
  pullPolicy: missing        # always | missing | never
  volumes:                   # Volume mounts
    - /host:/container
  env:                       # Environment variables
    - KEY=value
  workingDir: /app             # Working directory
  user: "1000:1000"         # User/group
  platform: linux/amd64     # Platform
  ports:                    # Port mappings
    - "8080:8080"
  network: host             # Network mode
  keepContainer: true       # Keep after workflow
```

## Key Benefits

- **Shared Environment**: All steps share the same filesystem and installed dependencies
- **Performance**: No container startup overhead between steps
- **Consistency**: Guaranteed same environment for all steps
- **Simplicity**: No need to configure Docker executor for each step

## Execution Model and Entrypoint Behavior

- **How it runs:** When you set a DAG‑level `container`, Dagu starts one
  long‑lived container for the workflow and keeps it alive (using a simple
  sleep loop). Each step then runs inside that container via `docker exec`.
- **Entrypoint/CMD not used for steps:** Because steps are executed with
  `docker exec`, your image’s `ENTRYPOINT` or `CMD` are not invoked for step
  commands. Steps run directly in the running container process context.
- **Implication:** If your image’s entrypoint is a dispatcher that expects a
  subcommand (for example, `my-entrypoint sendConfirmationEmails` which then
  calls `npm run sendConfirmationEmails`), the step command must invoke that
  dispatcher explicitly.

### Examples

Image entrypoint expects a job name as its first argument:

```yaml
container:
  image: myorg/myimage:latest

steps:
  # This will NOT pass through the image ENTRYPOINT automatically.
  # Explicitly call the entrypoint script or the underlying command.
  - my-entrypoint sendConfirmationEmails
  # Or call the underlying command directly, if appropriate
  - npm run sendConfirmationEmails
```

If your step needs a shell to interpret operators (like `&&`, redirects,
or environment expansion), wrap it explicitly:

```yaml
steps:
  - sh -c "npm run prep && npm run sendConfirmationEmails"
```

### When to use step-level Docker instead

If you want each step to run via the image’s `ENTRYPOINT`/`CMD` (as with a
fresh `docker run` per step), prefer the step‑level Docker executor instead of
the DAG‑level `container`:

```yaml
steps:
  - name: send-confirmation-emails
    executor:
      type: docker
      config:
        image: myorg/myimage:latest
        autoRemove: true
    # Here, Docker will honor ENTRYPOINT/CMD by default
    # (or you can override using executor config options)
    command: sendConfirmationEmails
```

## See Also

- [Docker Executor](/features/executors/docker) - Step-level container execution
- [Registry Authentication](/features/executors/docker#registry-authentication) - Private registry setup
