# Container Field

Run all workflow steps in a shared Docker container for consistent execution environment.

## Basic Usage

```yaml
container:
  image: python:3.11

steps:
  - command: pip install pandas numpy  # Install dependencies
    
  - command: python process.py          # Process data
```

All steps run in the same container instance, sharing the filesystem and installed packages.

## With Volume Mounts

```yaml
container:
  image: node:20
  volumes:
    - ./src:/app
    - ./data:/data
  workDir: /app

steps:
  - command: npm install    # Install dependencies
    
  - command: npm run build  # Build the application
    
  - command: npm test       # Run tests
```

## With Environment Variables

```yaml
container:
  image: postgres:16
  env:
    - POSTGRES_PASSWORD=secret
    - POSTGRES_DB=myapp

steps:
  - name: wait
    command: pg_isready -U postgres
    retryPolicy:
      limit: 10
      
  - name: migrate
    command: psql -U postgres myapp -f schema.sql
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
  - name: run
    command: ./app
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
  workDir: /app             # Working directory
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

## When to Use

Use the `container` field when:
- Multiple steps need the same dependencies
- Steps produce files consumed by later steps
- You want consistent environment across all steps
- You're migrating from traditional scripts

Use step-level Docker executor when:
- Steps need different images
- Steps require isolation from each other
- You need fine-grained container control

## See Also

- [Docker Executor](/features/executors/docker) - Step-level container execution
- [Registry Authentication](/features/executors/docker#registry-authentication) - Private registry setup