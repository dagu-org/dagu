# Remote Nodes

## Multi-Environment

By setting up remote nodes, you can run workflows on different Dagu server environments (e.g., development, staging, production) in a single Dagu server Web UI.

```yaml
remoteNodes:
  - name: "development"
    apiBaseURL: "http://dev.internal:8080/api/v2"
    isBasicAuth: true
    basicAuthUsername: "dev"
    basicAuthPassword: "${DEV_PASSWORD}"
    
  - name: "staging"
    apiBaseURL: "https://staging.example.com/api/v2"
    isAuthToken: true
    authToken: "${STAGING_TOKEN}"
    
  - name: "production"
    apiBaseURL: "https://prod.example.com/api/v2"
    isAuthToken: true
    authToken: "${PROD_TOKEN}"
```

## Secure Access using mTLS

```yaml
# mTLS configuration
remoteNodes:
  - name: "secure-prod"
    apiBaseURL: "https://secure.example.com/api/v2"
    tlsConfig:
      certFile: "/etc/dagu/certs/client.crt"
      keyFile: "/etc/dagu/certs/client.key"
      caFile: "/etc/dagu/certs/ca.crt"
```

## See Also

- [Server Configuration](/configurations/server)
