# Requirements: DAG-Level Container Execution

## Overview

This document outlines the requirements for adding DAG-level container support to Dagu, similar to how GitHub Actions runs entire jobs inside containers. This approach solves the Docker-in-Docker complexity and provides a more efficient way to run workflows that use a consistent environment.

## Motivation

When running workflows where most or all steps use the same environment, creating individual containers for each step is inefficient and complex. Additionally, when Dagu itself runs inside a container, step-level containers introduce Docker-in-Docker challenges with volume mounts and networking.

## Current State vs Proposed

### Current State
```yaml
steps:
  - name: install
    executor:
      type: docker
      config:
        image: node:18
    command: npm install
    
  - name: test
    executor:
      type: docker
      config:
        image: node:18
    command: npm test
```

### Proposed Enhancement
```yaml
name: my-workflow
container:
  image: node:18
  
steps:
  - name: install
    command: npm install
    
  - name: test
    command: npm test
```

## Requirements

### 1. DAG-Level Container Configuration

#### Basic Syntax

**Option 1: Using an existing image**
```yaml
name: my-workflow
container:
  image: <image-name>     # Required when not using build
  env:                    # Optional: Environment variables
    - KEY=value
  volumes:               # Optional: Volume mounts
    - host:container
  user: <user>          # Optional: User to run as
  workDir: <path>       # Optional: Working directory in container
  pull: <policy>        # Optional: Pull policy (always/missing/never)
  platform: <platform>  # Optional: Platform (linux/amd64, etc.)
  ports:                 # Optional: Port mappings
    - host:container
  network: <mode>        # Optional: Network mode
  keepContainer: <bool>  # Optional: Keep container after completion
```

**Option 2: Building from Dockerfile**
```yaml
name: my-workflow
container:
  build:                  # Build from Dockerfile instead of using image
    dockerfile: ./Dockerfile  # Path to Dockerfile (default: ./Dockerfile)
    context: .               # Build context (default: .)
    args:                    # Build arguments
      NODE_VERSION: 18
      ENV: production
  # Other container options still apply
  env:
    - NODE_ENV=production
  volumes:
    - .:/workspace
  workDir: /workspace
```

#### Default Values
| Field | Default Value | Description |
|-------|--------------|-------------|
| `image` | (required*) | No default - must specify either `image` or `build` |
| `build.dockerfile` | `./Dockerfile` | Path to Dockerfile when using build |
| `build.context` | `.` | Build context directory |
| `build.args` | `{}` | No build arguments |
| `env` | `[]` | No additional environment variables |
| `volumes` | `[".:/workspace"]` | Current directory mounted to /workspace |
| `user` | (container default) | Use the container's default user |
| `workDir` | `/workspace` | Working directory inside container |
| `pull` | `missing` | Pull only if image not present locally |
| `platform` | (host platform) | Use host's platform/architecture |
| `ports` | `[]` | No port mappings |
| `network` | `bridge` | Default Docker network mode |
| `keepContainer` | `false` | Remove container after completion |

*Note: Either `image` or `build` must be specified, but not both.

#### Behavior
- When `container` is specified at DAG level, all steps run inside this container by default
- The container is created once and reused for all applicable steps
- Steps execute via `docker exec` rather than creating new containers
- Container lifecycle matches DAG execution lifecycle

### 2. Step-Level Overrides

Steps can override the DAG-level container:

```yaml
name: multi-env-workflow
container:
  image: python:3.11
  
steps:
  - name: python-lint
    command: pylint src/
    
  - name: node-build
    container:
      image: node:18
    command: npm run build
    
  - name: python-test
    command: pytest  # Uses DAG-level container
```

### 3. Container Lifecycle Management

#### Creation
- Container is created before first step execution
- Generated using: `{build.Slug}-{randomID}`
- Example: `dagu-a1b2c3d4e5f6`
- Avoids issues with special characters in DAG names
- Container runs with appropriate init system to handle multiple processes

#### Execution
- Each step runs as `docker exec` in the container
- Step working directory is set via `docker exec --workdir`
- Step environment variables are passed via `docker exec --env`

#### Cleanup
- **Default behavior**: Container is stopped and removed after DAG completion
- Cleanup happens even if DAG fails (finally semantics)
- Optional `keepContainer: true` for debugging failed runs
- Rationale: Since DAG-level containers are long-lived (entire workflow), automatic cleanup prevents accumulation
- For scheduled DAGs, this is especially important to avoid container buildup

```yaml
# Default behavior - container is removed
container:
  image: node:18

# Keep container for debugging
container:
  image: node:18
  keepContainer: true  # Container persists after DAG completion
```

### 4. Volume and File Handling

#### Automatic Workspace
```yaml
container:
  image: node:18
  # Automatically mounts current directory to /workspace
  # unless explicitly configured otherwise
```

#### Explicit Volume Configuration
```yaml
container:
  image: node:18
  volumes:
    - .:/app                    # Mount current dir
    - /var/data:/data:ro       # Read-only mount
    - myvolume:/cache          # Named volume
```

#### File Sharing Between Steps
Since all steps run in the same container, they naturally share the filesystem:
```yaml
container:
  image: ubuntu

steps:
  - name: create
    command: echo "data" > /tmp/shared.txt
    
  - name: read
    command: cat /tmp/shared.txt  # Works seamlessly
```

### 5. Environment Variables

#### Inheritance Hierarchy
1. DAG-level env vars
2. Container-level env vars
3. Step-level env vars (highest priority)

```yaml
env:
  - GLOBAL=dag_level

container:
  image: node:18
  env:
    - NODE_ENV=production
    - GLOBAL=container_level  # Overrides DAG level

steps:
  - name: test
    env:
      - NODE_ENV=test  # Overrides container level
    command: npm test
```

### 6. Networking

#### Port Exposure
```yaml
container:
  image: nginx
  ports:
    - "8080:80"
    - "8443:443"
```

#### Network Modes
```yaml
container:
  image: app
  network: host  # or bridge, none, container:name
```

### 7. Integration with Existing Features

The DAG-level container must work with:
- Dependencies between steps
- Conditional execution
- Retry/repeat policies
- Output capture
- Log streaming
- Signal handling

### 8. Special Considerations

#### Docker-in-Docker Scenarios
When Dagu itself runs in a container:
- Detect and warn about potential volume mount issues
- Provide clear documentation on using Docker socket mounting
- Support environment variables for path mapping

#### Mixed Execution Modes
Allow disabling container for specific steps:
```yaml
container:
  image: python:3.11

steps:
  - name: python-task
    command: python script.py
    
  - name: host-task
    container: false  # Run on host/Dagu container
    command: docker ps
```

### 9. Comparison with GitHub Actions

| Feature | GitHub Actions | Dagu (Proposed) |
|---------|---------------|-----------------|
| Container Level | Job | DAG |
| Step Override | No | Yes |
| Workspace | Auto-mounted | Configurable |
| Services | Separate containers | Not in scope |
| Network | Custom network | Configurable |

## Implementation Phases

### Phase 1: Basic DAG Container
- Single container for entire DAG
- Basic env and volume support
- No step-level overrides

### Phase 2: Full Features
- Step-level container overrides
- Advanced networking options
- Port mappings

### Phase 3: Enhanced Integration
- Service containers (like GitHub Actions)
- Advanced volume strategies
- Container registries support

## Example Use Cases

### 1. Node.js CI Pipeline
```yaml
name: node-ci
container:
  image: node:18
  volumes:
    - .:/workspace
  workDir: /workspace

steps:
  - name: install
    command: npm ci
    
  - name: lint
    command: npm run lint
    
  - name: test
    command: npm test
    
  - name: build
    command: npm run build
```

### 2. Multi-Language Project
```yaml
name: full-stack-ci
container:
  image: ubuntu:22.04

steps:
  - name: setup
    command: |
      apt-get update
      apt-get install -y python3 nodejs npm
      
  - name: backend-test
    command: python3 -m pytest backend/
    
  - name: frontend-test  
    command: |
      cd frontend
      npm install
      npm test
```

### 3. Database Integration Tests
```yaml
name: integration-tests
container:
  image: python:3.11
  env:
    - DATABASE_URL=postgresql://postgres:password@db:5432/test

steps:
  - name: wait-for-db
    command: |
      pip install psycopg2-binary
      python -c "import psycopg2; psycopg2.connect('$DATABASE_URL')"
    retryPolicy:
      limit: 5
      intervalSec: 5
      
  - name: run-migrations
    command: alembic upgrade head
    
  - name: run-tests
    command: pytest tests/integration/
```

## Success Criteria

1. Workflows using single environment are simpler to write
2. Significant performance improvement for multi-step workflows
3. Docker-in-Docker scenarios work smoothly
4. Clear migration path from step-level to DAG-level containers
5. GitHub Actions users feel familiar with the syntax

## Out of Scope

These features are not part of the initial implementation:
1. Service containers (sidecar pattern)
2. Container health checks
3. Complex networking between multiple containers
4. Building images as part of workflow
5. Container registries authentication

## Conclusion

DAG-level container support provides a simpler, more efficient way to run workflows in containerized environments. By running all steps in a single container, we eliminate Docker-in-Docker complexity, improve performance, and provide a familiar experience for users coming from GitHub Actions or similar platforms.