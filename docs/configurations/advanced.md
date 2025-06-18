# Advanced Setup

Advanced configuration patterns and integration strategies for Dagu.

## Remote Nodes Configuration

### Multi-Environment Management

Configure Dagu to manage multiple environments from a single UI:

```yaml
# ~/.config/dagu/config.yaml
remoteNodes:
  - name: "development"
    apiBaseURL: "http://dev.internal:8080/api/v2"
    isBasicAuth: true
    basicAuthUsername: "dev_user"
    basicAuthPassword: "${DEV_PASSWORD}"
    
  - name: "staging"
    apiBaseURL: "https://staging.company.com/api/v2"
    isAuthToken: true
    authToken: "${STAGING_TOKEN}"
    skipTLSVerify: false
    
  - name: "production"
    apiBaseURL: "https://prod.company.com/api/v2"
    isAuthToken: true
    authToken: "${PROD_TOKEN}"
    skipTLSVerify: false
    
  - name: "disaster-recovery"
    apiBaseURL: "https://dr.company.com/api/v2"
    isAuthToken: true
    authToken: "${DR_TOKEN}"
    skipTLSVerify: false
```

### Secure Remote Access

Set up secure remote node access with mTLS:

```yaml
# Remote node with client certificates
remoteNodes:
  - name: "secure-prod"
    apiBaseURL: "https://secure.company.com/api/v2"
    tlsConfig:
      certFile: "/etc/dagu/certs/client.crt"
      keyFile: "/etc/dagu/certs/client.key"
      caFile: "/etc/dagu/certs/ca.crt"
```

### Dynamic Node Discovery

Use service discovery for dynamic environments:

```go
// Custom node discovery (example concept)
// This would require custom development

func discoverNodes() []RemoteNode {
    // Query service registry (Consul, etcd, etc.)
    services := consul.GetServices("dagu")
    
    var nodes []RemoteNode
    for _, service := range services {
        nodes = append(nodes, RemoteNode{
            Name: service.Name,
            APIBaseURL: fmt.Sprintf("http://%s:%d/api/v2", 
                service.Address, service.Port),
            AuthToken: os.Getenv(service.Name + "_TOKEN"),
        })
    }
    return nodes
}
```

## Queue Management

### Advanced Queue Configuration

Configure sophisticated queue patterns:

```yaml
# config.yaml
queues:
  enabled: true
  config:
    # CPU-intensive jobs queue
    - name: "cpu-intensive"
      maxConcurrency: 2  # Limit to number of CPU cores
      
    # I/O-intensive jobs queue
    - name: "io-intensive"
      maxConcurrency: 20  # Can handle more concurrent I/O
      
    # Batch processing queue
    - name: "batch"
      maxConcurrency: 1   # Sequential processing
      
    # Default queue
    - name: "default"
      maxConcurrency: 5
```

### Queue Selection Strategy

Base configuration for queue assignment:

```yaml
# base.yaml - Shared by all DAGs
queue: "default"  # Default queue for all DAGs
```

## CI/CD Integration

### GitHub Actions

Deploy DAGs automatically on push:

```yaml
# .github/workflows/validate-dags.yml
name: Validate DAGs

on:
  push:
    branches: [main]
    paths:
      - 'dags/**'

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
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

### Parallel Processing Patterns

Optimize large-scale parallel processing:

```yaml
# batch-processor.yaml
name: batch-processor
params:
  - BATCH_SIZE: 1000
  - PARALLELISM: 10

steps:
  - name: prepare-batches
    command: |
      # Split input into batches
      split -l ${BATCH_SIZE} input.csv batch_
      ls batch_* > batches.txt
    output: BATCH_FILES
    
  - name: process-batches
    run: process-single-batch
    parallel:
      items: "$(cat batches.txt)"
      maxConcurrent: ${PARALLELISM}
    params: "BATCH_FILE=${ITEM}"
    
  - name: merge-results
    command: |
      cat batch_*.result > final_result.csv
      rm batch_*
    depends: process-batches
```

## See Also

- Review [security best practices](/configurations/operations#security-hardening)
- Explore [API documentation](/reference/api) for integration
- Set up [monitoring and alerting](/configurations/operations#monitoring)
- Configure [backup and recovery](/configurations/operations#backup-and-recovery)
