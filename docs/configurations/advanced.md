# Advanced Setup

Advanced patterns and integrations.

## Remote Nodes

### Multi-Environment

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

### Secure Access

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

## Queue Management

```yaml
queues:
  enabled: true
  config:
    - name: "cpu-intensive"
      maxConcurrency: 2    # CPU cores
      
    - name: "io-intensive"
      maxConcurrency: 20   # High I/O
      
    - name: "batch"
      maxConcurrency: 1    # Sequential
      
    - name: "default"
      maxConcurrency: 5
```

Per-DAG queue:
```yaml
# In DAG file
queue: "cpu-intensive"
```

## CI/CD Integration

### GitHub Actions

```yaml
# .github/workflows/validate-dags.yml
name: Validate DAGs

on:
  push:
    paths: ['dags/**']

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Install Dagu
        run: |
          curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash
          
      - name: Validate DAGs
        run: |
          for dag in dags/*.yaml; do
            dagu dry "$dag"
          done
```

## Performance Optimization

### Parallel Batch Processing

```yaml
name: batch-processor
params:
  - BATCH_SIZE: 1000
  - PARALLELISM: 10

steps:
  - name: split
    command: split -l ${BATCH_SIZE} input.csv batch_
    
  - name: process
    command: echo "Processing batch files"
    maxActiveSteps: ${PARALLELISM}
    
  - name: merge
    command: cat batch_*.result > output.csv
```

## See Also

- [Operations Guide](/configurations/operations)
- [API Reference](/reference/api)
- [Server Configuration](/configurations/server)
