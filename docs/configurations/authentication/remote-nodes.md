# Remote Nodes Authentication

Configure authentication for connecting to remote Dagu instances.

## Configuration

### YAML Configuration

```yaml
# ~/.config/dagu/config.yaml
remoteNodes:
  - name: production
    apiBaseURL: https://prod.example.com/api/v2
    isBasicAuth: true
    basicAuthUsername: admin
    basicAuthPassword: prod-password
    
  - name: staging
    apiBaseURL: https://staging.example.com/api/v2
    isAuthToken: true
    authToken: staging-api-token
    
  - name: development
    apiBaseURL: http://dev.example.com:8080/api/v2
    # No auth configured - anonymous access
```

## Authentication Types

### Basic Authentication

```yaml
remoteNodes:
  - name: remote1
    apiBaseURL: https://remote1.example.com/api/v2
    isBasicAuth: true
    basicAuthUsername: admin
    basicAuthPassword: secure-password
```

### Token Authentication

```yaml
remoteNodes:
  - name: remote2
    apiBaseURL: https://remote2.example.com/api/v2
    isAuthToken: true
    authToken: api-token-for-remote2
```

### No Authentication

```yaml
remoteNodes:
  - name: local-dev
    apiBaseURL: http://localhost:8081/api/v2
    # No auth fields - anonymous access
```

## TLS Options

### Skip TLS Verification

```yaml
remoteNodes:
  - name: self-signed
    apiBaseURL: https://internal.example.com/api/v2
    skipTLSVerify: true
    isAuthToken: true
    authToken: token
```
