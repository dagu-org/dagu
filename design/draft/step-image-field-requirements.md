# Requirements: Adding 'image' Field to Dagu Steps

## Overview

This document outlines the requirements for adding an `image` field directly to step definitions in Dagu, making it more cloud and container-native by providing a simpler, more intuitive syntax for running steps in Docker containers.

## Current State

Currently, to run a step in a Docker container, users must use the executor configuration:

```yaml
steps:
  - name: build
    executor:
      type: docker
      config:
        image: node:18
        autoRemove: true
    command: npm run build
```

This syntax, while powerful and flexible, is verbose for the common use case of simply running a command in a container.

## Proposed Enhancement

Add an `image` field directly to step definitions as syntactic sugar for Docker execution:

```yaml
steps:
  - name: build
    image: node:18
    command: npm run build
```

## Requirements

### 1. Syntax and Behavior

#### Core Fields

- **`image`** (string, required for Docker shorthand)
  - Standard Docker image reference format (e.g., `alpine:latest`, `node:18`, `myregistry.com/myimage:tag`)
  - When specified, the step automatically uses the Docker executor
  
- **`env`** (array of strings, optional)
  - Environment variables in `KEY=value` format
  - Supports variable expansion: `API_KEY=${API_KEY}`
  
- **`volumes`** (array of strings, optional)
  - Volume mounts in Docker `-v` syntax: `host:container[:options]`
  - Options: `ro` (read-only), `rw` (read-write, default)
  
- **`user`** (string, optional)
  - User to run the container as
  - Formats: `username`, `UID`, `UID:GID`
  
- **`ports`** (array of strings, optional)
  - Port mappings in `host:container[/protocol]` format
  - Protocol defaults to `tcp` if not specified
  
- **`platform`** (string, optional)
  - Target platform/architecture
  - Examples: `linux/amd64`, `linux/arm64`, `linux/arm/v7`

#### Default Values

| Field | Default Value | Description |
|-------|--------------|-------------|
| `image` | (required with shorthand) | No default - triggers Docker executor |
| `env` | `[]` | Inherits from step/DAG env, no additional |
| `volumes` | `[]` | No volume mounts |
| `user` | (container default) | Use the container's default user |
| `ports` | `[]` | No port mappings |
| `platform` | (host platform) | Use Docker host's platform |
| `autoRemove` | `true` | Remove container after execution |
| `pull` | `missing` | Pull only if image not present locally |
| `dir` | (current directory) | Maps to container's workingDir |

### 2. Compatibility and Precedence

- **Backward Compatibility**: Existing executor configurations must continue to work unchanged
- **Precedence Rules**:
  - If both `image` and `executor` are specified, `executor` takes precedence
  - This allows users to use the shorthand for simple cases but still access full Docker configuration when needed
- **Validation**: If `image` is specified with a non-Docker executor type, log a warning but honor the explicit executor configuration

### 3. Configuration Mapping

When `image` field is used, it creates an implicit executor configuration:

```yaml
# This:
image: alpine:latest
command: echo hello

# Becomes equivalent to:
executor:
  type: docker
  config:
    image: alpine:latest
    autoRemove: true
    pull: missing
command: echo hello

# With all shorthand fields:
image: alpine:latest
env:
  - FOO=bar
volumes:
  - /host:/container
user: "1000:1000"
ports:
  - "8080:80"
platform: linux/amd64
dir: /app
command: echo hello

# Becomes equivalent to:
executor:
  type: docker
  config:
    image: alpine:latest
    autoRemove: true
    pull: missing
    platform: linux/amd64
    container:
      env:
        - FOO=bar
      user: "1000:1000"
      workingDir: /app
    host:
      binds:
        - /host:/container
      portBindings:
        80/tcp:
          - hostPort: "8080"
command: echo hello
```

### 4. Environment Variables and Volumes

#### Environment Variables
Since Docker currently reads environment variables from `config.container.env`, we have three options:

**Option A: Support inline env at step level (Recommended)**
```yaml
steps:
  - name: build
    image: node:18
    env:
      - NODE_ENV=production
      - API_KEY=${API_KEY}
    command: npm run build
```
This would be translated to the container's env configuration internally.

**Option B: No env support with image field**
Require full executor syntax for any environment variables.

**Option C: String array format**
```yaml
steps:
  - name: build
    image: node:18
    env: ["NODE_ENV=production", "API_KEY=${API_KEY}"]
    command: npm run build
```

**Recommendation**: Option A provides the best balance of simplicity and functionality.

#### Volume Mounts
For volume mounts, we have similar options:

**Option A: Simple volumes field (Recommended)**
```yaml
steps:
  - name: build
    image: node:18
    volumes:
      - ./src:/app/src:ro
      - ./dist:/app/dist:rw
    command: npm run build
```

**Option B: No volume support with image field**
Require full executor syntax for any volume mounts.

**Option C: Separate source/target syntax**
```yaml
steps:
  - name: build
    image: node:18
    volumes:
      - source: ./src
        target: /app/src
        readonly: true
    command: npm run build
```

**Recommendation**: Option A follows Docker's familiar `-v` syntax and is more concise.

#### Working Directory
The step's `dir` field should be automatically passed to the container as the working directory:
```yaml
steps:
  - name: build
    image: node:18
    dir: /app
    command: npm run build
```
This sets the container's working directory to `/app`.

#### User
The `user` field specifies which user to run the container as:
```yaml
steps:
  - name: build
    image: node:18
    user: "1000:1000"  # UID:GID format
    command: npm run build
    
  - name: test
    image: postgres:15
    user: postgres     # Username format
    command: psql -c "SELECT version()"
```

#### Ports
The `ports` field exposes container ports to the host:
```yaml
steps:
  - name: web-server
    image: nginx
    ports:
      - "8080:80"      # host:container
      - "8443:443/tcp" # with protocol
    command: nginx -g "daemon off;"
```

#### Platform
The `platform` field specifies the target architecture:
```yaml
steps:
  - name: build
    image: node:18
    platform: linux/amd64  # or linux/arm64, etc.
    command: npm run build
```

### 5. Advanced Docker Features

Users needing advanced Docker features beyond the basic fields should use the full executor syntax:
- Network configuration
- Custom entrypoints
- Resource limits (CPU, memory)
- Security options (capabilities, seccomp)
- Advanced volume configurations (drivers, etc.)
- Health checks
- Restart policies

The shorthand fields (`image`, `env`, `volumes`, `user`, `ports`, `platform`) are designed for the common cases.

### 6. Integration with Existing Features

The `image` field must work seamlessly with all existing step features:
- Dependencies (`depends`)
- Output capture (`output`)
- Retry/repeat policies
- Preconditions
- Working directory (`dir`)
- Shell selection (`shell`)
- Continue-on conditions
- Signal handling

### 7. Error Handling

- **Invalid Image Format**: Validate image reference format and provide clear error messages
- **Pull Failures**: Handle Docker pull errors gracefully with informative messages
- **Missing Docker**: Detect if Docker is not available and provide helpful error message

## Benefits

### 1. Improved Developer Experience
- Simpler, more intuitive syntax for the common case
- Reduced cognitive load for new users
- Faster workflow creation

### 2. Cloud-Native Alignment
- Makes containerized execution a first-class citizen
- Aligns with modern cloud-native practices where containers are the default
- Reduces barrier to container adoption

### 3. Consistency with Other Tools
- Similar syntax to popular CI/CD tools (GitHub Actions, GitLab CI)
- Familiar to developers coming from other platforms
- Follows industry patterns

## Implementation Considerations

### 1. YAML Schema Updates
- Add new fields to step definition schema:
  - `image` (string)
  - `volumes` (array of strings)
  - `user` (string)
  - `ports` (array of strings)
  - `platform` (string)
- Update JSON Schema for IDE autocomplete support
- Ensure OpenAPI specifications reflect the new fields

### 2. Documentation Updates
- Add examples to CLAUDE.md showing the new syntax
- Update getting started guides
- Create migration guide for users wanting to simplify existing workflows

### 3. Testing Strategy
- Test interaction between `image` and `executor` fields
- Verify all step features work with image-based steps
- Test error cases (invalid images, missing Docker, etc.)
- Ensure backward compatibility with existing workflows

## Example Usage Patterns

### Simple Container Execution
```yaml
steps:
  - name: build
    image: node:18
    command: npm run build
```

### Container with Environment Variables
```yaml
steps:
  - name: test
    image: node:18
    env:
      - NODE_ENV=test
      - CI=true
    command: npm test
```

### Container with Volume Mounts
```yaml
steps:
  - name: build
    image: golang:1.21
    volumes:
      - ./src:/go/src/app
      - ./bin:/go/bin
    dir: /go/src/app
    command: go build -o /go/bin/app
```

### Container with Both Env and Volumes
```yaml
steps:
  - name: integration-test
    image: python:3.11
    env:
      - DATABASE_URL=postgres://localhost:5432/test
      - PYTHONPATH=/app
    volumes:
      - ./tests:/app/tests:ro
      - ./src:/app/src:ro
      - ./test-results:/app/results:rw
    command: pytest /app/tests --junit-xml=/app/results/junit.xml
```

### Container with User and Ports
```yaml
steps:
  - name: web-server
    image: nginx:alpine
    user: "nginx"
    ports:
      - "8080:80"
      - "8443:443"
    volumes:
      - ./static:/usr/share/nginx/html:ro
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
    command: nginx -g "daemon off;"
```

### Multi-Architecture Build
```yaml
steps:
  - name: build-amd64
    image: golang:1.21
    platform: linux/amd64
    env:
      - GOOS=linux
      - GOARCH=amd64
    command: go build -o dist/app-amd64
    
  - name: build-arm64
    image: golang:1.21
    platform: linux/arm64
    env:
      - GOOS=linux
      - GOARCH=arm64
    command: go build -o dist/app-arm64
```

### Database Service with Ports
```yaml
steps:
  - name: postgres-service
    image: postgres:15
    user: postgres
    env:
      - POSTGRES_PASSWORD=secret
      - POSTGRES_DB=testdb
    ports:
      - "5432:5432"
    volumes:
      - ./data:/var/lib/postgresql/data
    command: postgres

### Multiple Container Steps
```yaml
steps:
  - name: test backend
    image: golang:1.21
    command: go test ./...
    
  - name: test frontend
    image: node:18
    command: npm test
```

### Container with Working Directory
```yaml
steps:
  - name: analyze
    image: python:3.11
    dir: /app
    command: python analyze.py
```

### Override with Full Configuration
```yaml
steps:
  - name: complex build
    image: node:18  # This is ignored due to executor
    executor:
      type: docker
      config:
        image: node:18-alpine
        autoRemove: false
        container:
          env:
            - NODE_ENV=production
        host:
          binds:
            - ./config:/app/config:ro
    command: npm run build:prod
```

## Success Criteria

1. Users can run steps in containers with minimal syntax
2. Existing workflows continue to function without modification
3. Clear documentation and examples available
4. Intuitive error messages for common issues
5. Performance overhead is negligible compared to current Docker executor

## Future Enhancements (Out of Scope)

These are not part of the initial implementation but could be considered later:

1. **Common Image Shortcuts**: Predefined aliases like `image: python` → `python:latest`
2. **Image Templating**: Support for variables in image names like `image: myapp:${VERSION}`
3. **Registry Configuration**: Global registry settings for private registries
4. **Image Caching Strategies**: Advanced caching configurations
5. **Multi-Platform Builds**: Native support for building images as part of workflow

## Design Decisions and Trade-offs

### Environment Variables and Volumes Support

After careful consideration, the recommended approach is to support both `env` and `volumes` fields alongside the `image` field. This decision is based on:

1. **Usage Patterns**: Most containerized workflows need environment variables and volume mounts
2. **User Experience**: Requiring full executor syntax for these common needs would defeat the purpose of simplification
3. **Familiarity**: The syntax mirrors Docker CLI conventions that users already know
4. **80/20 Rule**: These features cover ~80% of container use cases with ~20% of the complexity

### What's Included vs Excluded

**Included in shorthand syntax:**
- `image`: Container image reference
- `env`: Environment variables (array format)
- `volumes`: Volume mounts (Docker -v syntax)
- `user`: User to run container as (UID:GID or username)
- `ports`: Port mappings (host:container format)
- `platform`: Target architecture (linux/amd64, linux/arm64, etc.)
- `dir`: Working directory (existing field)

**Requires full executor syntax:**
- Network configuration
- Resource limits (CPU, memory)
- Security options (capabilities, seccomp, etc.)
- Custom entrypoints
- Health checks
- Complex volume configurations
- Restart policies

This split provides a clean escalation path: start simple, upgrade to full syntax when needed.

## Conclusion

Adding the `image` field to Dagu steps represents a significant improvement in usability for container-based workflows. By providing a simple, intuitive syntax for the common case while maintaining full flexibility through the executor system, Dagu becomes more accessible to cloud-native workflows without sacrificing its powerful configuration capabilities.

The addition of `env` and `volumes` support ensures the shorthand syntax is useful for real-world workflows while keeping the syntax clean and familiar to Docker users.
