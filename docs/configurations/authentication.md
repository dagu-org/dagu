# Authentication

Configure authentication and access control for your Dagu instance.

## Available Authentication Methods

- [Basic Authentication](authentication/basic) - Username and password authentication
- [Token Authentication](authentication/token) - API token-based authentication
- [OIDC Authentication](authentication/oidc) - OpenID Connect authentication
- [TLS/HTTPS](authentication/tls) - Encrypted connections
- [Permissions](authentication/permissions) - Access control configuration
- [Remote Nodes](authentication/remote-nodes) - Multi-instance authentication

## Quick Start

### Basic Authentication

```yaml
auth:
  basic:
    username: admin
    password: secure-password
```

### Token Authentication

```yaml
auth:
  token:
    value: your-api-token
```

### OIDC Authentication

```yaml
auth:
  oidc:
    clientId: "your-client-id"
    clientSecret: "your-client-secret"
    clientUrl: "http://localhost:8080"
    issuer: "https://accounts.google.com"
```

## Environment Variables

All authentication methods support environment variable configuration. See individual authentication type documentation for details.
