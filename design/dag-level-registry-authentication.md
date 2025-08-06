# DAG-Level Container Registry Authentication Design

## Executive Summary

This document outlines the design for implementing container registry authentication in Dagu using a hybrid approach. Users can either configure authentication at the DAG level for explicit control, or use standard Docker environment variables (`DOCKER_AUTH_CONFIG`) for simplicity. The solution uses Docker's official authentication mechanisms to support all Docker-compatible registries.

## Background

### Current State

Dagu supports Docker container execution through:
- **DAG-level container field** - Defines container configuration at DAG level
- **Docker executor** - Step-level executor for running commands in containers

Current limitations:
- No built-in registry authentication mechanism
- Images can only be pulled from public registries
- Private registry access requires pre-pulled images or manual `docker login`

### Business Requirements

Users need to:
- Pull images from private registries with minimal configuration
- Support both DAG-specific and global authentication methods
- Maintain compatibility with existing Docker tooling
- Secure credential management without plain text exposure

## Proposed Solution

### Hybrid Approach

The solution supports two complementary authentication methods:

1. **DAG-level `registryAuths`** - Explicit per-DAG configuration for fine-grained control
2. **Standard Docker configuration** - Via `DOCKER_AUTH_CONFIG` environment variable or `~/.docker/config.json`

Users can choose either method or combine them based on their needs.

### Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                   Authentication Sources                 │
├─────────────────────────────────────────────────────────┤
│  Priority 1: DAG registryAuths field                    │
│  Priority 2: DOCKER_AUTH_CONFIG env var                 │
│  Priority 3: ~/.docker/config.json                      │
│  Priority 4: Anonymous pull                             │
└─────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────┐
│                   Authentication Manager                 │
├─────────────────────────────────────────────────────────┤
│  1. Extract registry hostname from image reference      │
│  2. Look up credentials in priority order               │
│  3. Use registry.EncodeAuthConfig (official function)   │
│  4. Return base64url encoded auth string                │
└─────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────┐
│                       Docker SDK                         │
├─────────────────────────────────────────────────────────┤
│  cli.ImagePull(ctx, image, image.PullOptions{           │
│    RegistryAuth: encodedAuth                            │
│  })                                                     │
└─────────────────────────────────────────────────────────┘
```

## Implementation Details

### 1. DAG Structure

```go
// DAG contains all information about a DAG
type DAG struct {
    // ... existing fields ...
    
    // RegistryAuths maps registry hostnames to authentication configs
    // Optional: If not specified, falls back to DOCKER_AUTH_CONFIG or docker config
    RegistryAuths map[string]AuthConfig `json:"registryAuths,omitempty"`
    
    // Container contains the container definition for the DAG
    Container *Container `json:"container,omitempty"`
}

// AuthConfig represents Docker registry authentication
// Simplified structure for user convenience
type AuthConfig struct {
    Username string `json:"username,omitempty"`
    Password string `json:"password,omitempty"`
    // Auth can be used instead of username/password for pre-encoded credentials
    Auth string `json:"auth,omitempty"`
}
```

### 2. Authentication Manager

```go
package registry

import (
    "context"
    "encoding/base64"
    "encoding/json"
    "os"
    "strings"
    
    "github.com/docker/docker/api/types/registry"
    dockerconfig "github.com/docker/cli/cli/config"
    "github.com/docker/cli/cli/config/configfile"
    "github.com/google/go-containerregistry/pkg/name"
)

// AuthManager handles registry authentication with multiple sources
type AuthManager struct {
    dagAuths map[string]AuthConfig // From DAG configuration
}

// NewAuthManager creates a new authentication manager
func NewAuthManager(dagAuths map[string]AuthConfig) *AuthManager {
    return &AuthManager{
        dagAuths: dagAuths,
    }
}

// GetAuthForImage returns the encoded auth string for an image
func (am *AuthManager) GetAuthForImage(imageRef string) (string, error) {
    // Extract registry hostname from image reference
    hostname, err := extractRegistryHost(imageRef)
    if err != nil {
        return "", err
    }
    
    // Priority 1: DAG-level registryAuths
    if auth, ok := am.dagAuths[hostname]; ok {
        return am.encodeAuthConfig(auth, hostname)
    }
    
    // Priority 2: DOCKER_AUTH_CONFIG environment variable
    if authConfig := os.Getenv("DOCKER_AUTH_CONFIG"); authConfig != "" {
        if encoded, err := am.getAuthFromDockerConfig(authConfig, hostname); err == nil {
            return encoded, nil
        }
    }
    
    // Priority 3: Default docker config file
    if cfg, err := dockerconfig.Load(""); err == nil {
        if authCfg, exists := cfg.AuthConfigs[hostname]; exists {
            return registry.EncodeAuthConfig(authCfg)
        }
    }
    
    // Priority 4: Anonymous pull
    return "", nil
}

// encodeAuthConfig converts our simplified AuthConfig to Docker format and encodes it
func (am *AuthManager) encodeAuthConfig(auth AuthConfig, hostname string) (string, error) {
    dockerAuth := registry.AuthConfig{
        ServerAddress: hostname,
    }
    
    // If Auth field is provided, decode it to get username:password
    if auth.Auth != "" {
        decoded, err := base64.StdEncoding.DecodeString(auth.Auth)
        if err == nil {
            parts := strings.SplitN(string(decoded), ":", 2)
            if len(parts) == 2 {
                dockerAuth.Username = parts[0]
                dockerAuth.Password = parts[1]
            }
        }
    } else {
        // Use username/password directly
        dockerAuth.Username = auth.Username
        dockerAuth.Password = auth.Password
    }
    
    // Use official Docker encoding function (base64url)
    return registry.EncodeAuthConfig(dockerAuth)
}

// getAuthFromDockerConfig parses DOCKER_AUTH_CONFIG format
func (am *AuthManager) getAuthFromDockerConfig(configJSON string, hostname string) (string, error) {
    var config configfile.ConfigFile
    if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
        return "", err
    }
    
    if authCfg, exists := config.AuthConfigs[hostname]; exists {
        return registry.EncodeAuthConfig(authCfg)
    }
    
    return "", fmt.Errorf("no auth found for %s", hostname)
}

// extractRegistryHost extracts the registry hostname from an image reference
func extractRegistryHost(imageRef string) (string, error) {
    ref, err := name.ParseReference(imageRef)
    if err != nil {
        return "", err
    }
    
    hostname := ref.Context().RegistryStr()
    
    // Handle Docker Hub default (index.docker.io -> docker.io)
    if hostname == name.DefaultRegistry || hostname == "index.docker.io" {
        return "docker.io", nil
    }
    
    return hostname, nil
}
```

### 3. Container Client Integration

```go
// In container/client.go
type Client struct {
    // ... existing fields ...
    authManager *registry.AuthManager
}

// In startNewContainer method:
if pull {
    pullOptions := image.PullOptions{
        Platform: platforms.Format(c.platformO),
    }
    
    // Get auth for this specific image
    if c.authManager != nil {
        if encodedAuth, err := c.authManager.GetAuthForImage(c.image); err == nil {
            pullOptions.RegistryAuth = encodedAuth
        }
    }
    
    reader, err := cli.ImagePull(ctx, c.image, pullOptions)
    // ... handle response
}
```

## Usage Examples

### Option 1: DAG-Level Configuration

Best for: DAG-specific credentials, multi-tenant environments, explicit configuration

```yaml
name: my-dag

# Explicit registry authentication per DAG
registryAuths:
  ghcr.io: ${GHCR_AUTH}  # JSON: {"username":"user","password":"token"}
  123456789012.dkr.ecr.us-east-1.amazonaws.com: ${ECR_AUTH}

container:
  image: ghcr.io/myorg/private-image:latest

steps:
  - name: process
    executor:
      type: docker
      config:
        image: 123456789012.dkr.ecr.us-east-1.amazonaws.com/processor:latest
    command: process-data
```

Environment variables with plain JSON:
```bash
export GHCR_AUTH='{"username":"github_user","password":"ghp_token"}'
export ECR_AUTH='{"username":"AWS","password":"'$(aws ecr get-login-password --region us-east-1)'"}'
```

### Option 2: DOCKER_AUTH_CONFIG (Standard Docker)

Best for: Shared credentials across DAGs, CI/CD environments, existing Docker workflows

```yaml
name: my-dag
# No auth configuration needed in DAG!

container:
  image: ghcr.io/myorg/private-image:latest

steps:
  - name: process
    executor:
      type: docker
      config:
        image: 123456789012.dkr.ecr.us-east-1.amazonaws.com/processor:latest
    command: process-data
```

Set DOCKER_AUTH_CONFIG with all registries:
```bash
export DOCKER_AUTH_CONFIG='{
  "auths": {
    "ghcr.io": {
      "auth": "'$(echo -n "username:pat_token" | base64)'"
    },
    "123456789012.dkr.ecr.us-east-1.amazonaws.com": {
      "auth": "'$(echo -n "AWS:$(aws ecr get-login-password)" | base64)'"
    },
    "docker.io": {
      "auth": "'$(echo -n "dockerhub_user:dockerhub_token" | base64)'"
    }
  }
}'
```

### Option 3: Docker Config File

Best for: Local development, persistent credentials

```bash
# Login using docker CLI (creates/updates ~/.docker/config.json)
docker login ghcr.io
docker login 123456789012.dkr.ecr.us-east-1.amazonaws.com

# DAGs will automatically use these credentials
```

### Option 4: Mixed Approach

Combine methods for flexibility:

```yaml
name: my-dag

# Override specific registry in DAG
registryAuths:
  special.registry.com: ${SPECIAL_AUTH}

# Other registries use DOCKER_AUTH_CONFIG or ~/.docker/config.json

container:
  image: special.registry.com/image:latest  # Uses DAG auth

steps:
  - name: process
    executor:
      type: docker
      config:
        image: ghcr.io/org/processor:latest  # Uses DOCKER_AUTH_CONFIG
    command: process-data
```

## Authentication Priority

The system checks for credentials in this order:

1. **DAG-level `registryAuths`** - Highest priority, explicit configuration
2. **`DOCKER_AUTH_CONFIG` environment variable** - Standard Docker format
3. **`~/.docker/config.json`** - Default Docker configuration
4. **Anonymous pull** - No authentication

This ensures backward compatibility while providing flexibility.

## Security Considerations

1. **Credential Protection**
   - Never commit credentials to version control
   - Use environment variables for sensitive data
   - Credentials are never logged or displayed in UI

2. **Token Management**
   - Short-lived tokens (ECR ~12h, GCP OAuth ~1h) may need refresh
   - Consider using service accounts for long-running DAGs
   - `docker login` can be run periodically to refresh ~/.docker/config.json

3. **Minimal Exposure**
   - Credentials are only used for image pull operations
   - Auth strings are masked in error messages
   - Registry hostname is logged, not credentials

## Implementation Summary

The hybrid approach requires minimal code changes:

1. **Add `RegistryAuths` field to DAG struct** (~5 lines)
2. **Create AuthManager with priority lookup** (~100 lines)
3. **Integrate with Container Client** (~10 lines)
4. **Use `registry.EncodeAuthConfig` for encoding** (official Docker function)

Total implementation: ~150 lines including error handling and comments

## Benefits

1. **Flexibility**: Users choose the authentication method that fits their workflow
2. **Simplicity**: Zero configuration needed with DOCKER_AUTH_CONFIG
3. **Compatibility**: Works with existing Docker tooling and CI/CD systems
4. **Security**: Follows Docker's security best practices
5. **Standards Compliance**: Uses official Docker libraries and formats
6. **Multi-Registry Support**: Handle multiple registries seamlessly

## Migration Path

For existing users:
- **No breaking changes**: Public registries continue to work
- **Gradual adoption**: Add authentication only when needed
- **Existing docker login works**: ~/.docker/config.json is automatically used
- **CI/CD friendly**: DOCKER_AUTH_CONFIG is already supported by most CI systems

## Conclusion

The hybrid approach provides maximum flexibility while maintaining simplicity. Users can start with zero configuration using standard Docker methods, and add DAG-specific authentication when needed. The implementation leverages Docker's official libraries, ensuring compatibility with all Docker-compatible registries.