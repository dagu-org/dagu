# Testing OpenTelemetry Integration

This guide explains how to test Dagu's OpenTelemetry tracing functionality in various environments.

## Quick Start with Jaeger

The fastest way to test OpenTelemetry tracing locally:

### 1. Start Jaeger

```bash
docker run --rm --name jaeger \
  -p 16686:16686 \
  -p 4317:4317 \
  jaegertracing/all-in-one:latest
```

### 2. Create a Test DAG

```yaml
# test-otel.yaml
otel:
  enabled: true
  endpoint: "localhost:4317"
  insecure: true
  resource:
    service.name: "dagu-${DAG_NAME}"
    service.version: "1.0.0"
    environment: "local"

steps:
  - name: fetch-data
    command: echo "Fetching data..." && sleep 1
  
  - name: process-data
    command: echo "Processing data..." && sleep 2
    depends: fetch-data
  
  - name: analyze-batch-1
    command: echo "Analyzing batch 1..." && sleep 1
    depends: process-data
  
  - name: analyze-batch-2
    command: echo "Analyzing batch 2..." && sleep 1
    depends: process-data
  
  - name: aggregate-results
    command: echo "Aggregating results..."
    depends: [analyze-batch-1, analyze-batch-2]
```

### 3. Run the DAG

```bash
dagu start test-otel.yaml
```

### 4. View Traces

Open http://localhost:16686 in your browser:
1. Select "dagu-otel-test" from the Service dropdown
2. Click "Find Traces"
3. Click on a trace to see the execution timeline

## Testing Different Configurations

### HTTP Endpoint

```yaml
# test-http-endpoint.yaml
otel:
  enabled: true
  endpoint: "http://localhost:4318/v1/traces"
  insecure: true
steps:
  - echo "Testing HTTP endpoint"
```

### With Authentication

```yaml
# test-auth.yaml
env:
  - OTEL_TOKEN: "your-auth-token"
otel:
  enabled: true
  endpoint: "otel-collector:4317"
  headers:
    Authorization: "Bearer ${OTEL_TOKEN}"
steps:
  - echo "Testing with auth"
```

### Custom Resource Attributes

```yaml
# test-resources.yaml
otel:
  enabled: true
  endpoint: "localhost:4317"
  insecure: true
  resource:
    service.name: "dagu-${DAG_NAME}"
    service.version: "2.0.0"
    deployment.environment: "testing"
    team: "platform"
    region: "us-east-1"
steps:
  - echo "Testing resource attributes"
```

## Testing Nested DAGs

### 1. Create Parent DAG

```yaml
# parent-workflow.yaml
otel:
  enabled: true
  endpoint: "localhost:4317"
  insecure: true

steps:
  - echo "Starting parent workflow"
  - call: child-etl.yaml
    params: "SOURCE=production DATE=2024-01-01"
  - call: child-analytics.yaml
    params: "INPUT=${run-etl.output}"
  - echo "Parent workflow complete"
    
```

### 2. Create Sub DAGs

```yaml
# child-etl.yaml
params:
  - SOURCE: dev
  - DATE: today

otel:
  enabled: true
  endpoint: "localhost:4317"
  insecure: true

steps:
  - command: echo "Extracting from ${SOURCE}"
    output: EXTRACTED_DATA
  - echo "Transforming data" && echo "/tmp/data.csv"
```

```yaml
# child-analytics.yaml
params:
  - INPUT: ""

otel:
  enabled: true
  endpoint: "localhost:4317"
  insecure: true

steps:
  - echo "Analyzing ${INPUT}"
```

### 3. Run and Verify

```bash
dagu start parent-workflow.yaml
```

In Jaeger, you should see:
- One trace containing all DAG executions
- Parent-child relationships preserved
- `dag.parent_run_id` attribute on sub DAGs

## Production-Like Testing

### Using OpenTelemetry Collector

```yaml
# compose.yaml
version: '3.8'
services:
  jaeger:
    image: jaegertracing/all-in-one:latest
    ports:
      - "16686:16686"
      - "14250:14250"
  
  otel-collector:
    image: otel/opentelemetry-collector:latest
    command: ["--config=/etc/otel-collector.yaml"]
    volumes:
      - ./otel-collector.yaml:/etc/otel-collector.yaml
    ports:
      - "4317:4317"   # gRPC
      - "4318:4318"   # HTTP
    depends_on:
      - jaeger

  # Optional: Prometheus for metrics
  prometheus:
    image: prom/prometheus:latest
    volumes:
      - ./prometheus.yaml:/etc/prometheus/prometheus.yml
    ports:
      - "9090:9090"
```

```yaml
# otel-collector.yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:
    timeout: 1s

exporters:
  debug:
    verbosity: detailed
  jaeger:
    endpoint: jaeger:14250
    tls:
      insecure: true
  prometheus:
    endpoint: "0.0.0.0:8889"

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug, jaeger]
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [prometheus]
```

Start the stack:
```bash
docker compose up -d
```

## Debugging OpenTelemetry Issues

### 1. Enable Debug Logging

```bash
# Run with debug flag
dagu start --debug test-otel.yaml

# Or set environment variable
export DAGU_DEBUG=true
dagu start test-otel.yaml
```

### 2. Test Connectivity

```bash
# Test gRPC endpoint
grpcurl -plaintext localhost:4317 list

# Test HTTP endpoint
curl -X POST http://localhost:4318/v1/traces \
  -H "Content-Type: application/json" \
  -d '{}'
```

### 3. Common Issues and Solutions

#### No Traces Appearing

Check:
1. Is `enabled: true` set in your DAG?
2. Is the collector/Jaeger running?
3. Is the endpoint correct?
4. Any firewall blocking the ports?

#### Authentication Errors

```yaml
# Debug with curl
curl -X POST https://otel-collector:4318/v1/traces \
  -H "Authorization: Bearer your-token" \
  -H "Content-Type: application/json" \
  -d '{}'
```

#### TLS/Certificate Issues

```yaml
# For testing, use insecure mode
otel:
  enabled: true
  endpoint: "otel-collector:4317"
  insecure: true  # Skip certificate validation
```

## Performance Testing

### 1. Baseline Performance

```bash
# Create a DAG without OTel
cat > perf-test-no-otel.yaml << 'EOF'
name: perf-test
steps:
  - echo "Step 1"
  - echo "Step 2"  # Runs sequentially after Step 1
EOF

# Time execution without OTel
time dagu start perf-test-no-otel.yaml
```

### 2. With OTel Enabled

```bash
# Create same DAG with OTel
cat > perf-test-with-otel.yaml << 'EOF'
otel:
  enabled: true
  endpoint: "localhost:4317"
  insecure: true
steps:
  - echo "Step 1"
  - echo "Step 2"  # Runs sequentially after Step 1
EOF

# Time execution with OTel
time dagu start perf-test-with-otel.yaml
```

### 3. Load Testing

```bash
# Run multiple DAGs concurrently
for i in {1..10}; do
  dagu start test-otel.yaml &
done
wait

# Check Jaeger for all traces
```

## Integration Testing

### Test with Your Observability Stack

If you have an existing observability platform:

```yaml
# production-like-test.yaml
otel:
  enabled: true
  endpoint: "${OTEL_ENDPOINT}"
  headers:
    Authorization: "Bearer ${OTEL_TOKEN}"
  timeout: 30s
  resource:
    service.name: "dagu-${DAG_NAME}"
    service.version: "${APP_VERSION}"
    deployment.environment: "${ENVIRONMENT}"
steps:
  - echo "Testing production setup"
```

Run with environment variables:
```bash
export OTEL_ENDPOINT="your-collector.example.com:4317"
export OTEL_TOKEN="your-token"
export APP_VERSION="1.0.0"
export ENVIRONMENT="staging"

dagu start production-like-test.yaml
```
