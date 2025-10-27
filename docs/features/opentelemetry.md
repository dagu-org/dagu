# OpenTelemetry Tracing

Dagu supports OpenTelemetry (OTel) distributed tracing to provide deep visibility into workflow execution. This allows you to monitor performance, debug issues, and understand the flow of complex DAGs across your observability platform.

## Overview

When OpenTelemetry is enabled, Dagu creates:
- A root span for each DAG execution
- Child spans for each step execution
- Proper trace context propagation for nested DAGs
- Key attributes for filtering and analysis

## Configuration

### Basic Configuration

```yaml
name: my-workflow
otel:
  enabled: true
  endpoint: "localhost:4317"  # OTLP gRPC endpoint
steps:
  - python process.py
```

### Full Configuration

```yaml
dotenv:
  - .env
otel:
  enabled: true
  endpoint: "otel-collector:4317"  # OTLP gRPC endpoint
  # endpoint: "http://otel-collector:4318/v1/traces"  # OTLP HTTP endpoint
  headers:
    Authorization: "Bearer ${OTEL_AUTH_TOKEN}"
  insecure: false  # Skip TLS verification (default: false)
  timeout: 30s     # Export timeout
  resource:
    # Resource attributes (all optional)
    service.name: "dagu-${DAG_NAME}"  # Default value
    service.version: "1.0.0"
    deployment.environment: "${ENVIRONMENT}"
    # Custom attributes
    team: "data-engineering"
    cost_center: "analytics"
```

### Configuration Options

| Field | Description | Default |
|-------|-------------|---------|
| `enabled` | Enable/disable OpenTelemetry tracing | `false` |
| `endpoint` | OTLP endpoint URL (gRPC or HTTP) | Required |
| `headers` | HTTP headers for authentication | `{}` |
| `insecure` | Allow insecure connections | `false` |
| `timeout` | Export timeout duration | No default |
| `resource` | Resource attributes map | `{}` |

## Endpoint Configuration

Dagu automatically detects the protocol based on the endpoint:

- **gRPC endpoints**: `host:port` format (e.g., `localhost:4317`)
- **HTTP endpoints**: Must end with `/v1/traces` (e.g., `http://localhost:4318/v1/traces`)

## Trace Structure

### Span Hierarchy

```
DAG: my-workflow (root span)
├── Step: fetch-data
├── Step: validate-data
├── Step: process-batch-1
├── Step: process-batch-2
└── Step: aggregate-results
```

### Span Attributes

#### DAG Root Span
- `dag.name`: Name of the DAG
- `dag.run_id`: Unique execution ID
- `dag.parent_run_id`: Parent DAG run ID (for nested DAGs)
- `dag.status`: Final execution status

#### Step Spans
- `step.name`: Name of the step
- `step.status`: Step execution status
- `step.exit_code`: Process exit code (when applicable)

## Nested DAGs

When a step executes another DAG, the trace context is automatically propagated:

```yaml
steps:
  - call: workflows/child-workflow.yaml
    params: "PARAM1=value1"
```

The sub DAG's root span will be linked to the parent step span, creating a complete trace across all execution levels.

### Trace Context Propagation

Dagu uses the W3C Trace Context standard for propagating trace information between parent and sub DAGs:

- **Automatic propagation**: Trace context is automatically passed to sub DAGs via environment variables
- **W3C format**: Uses standard `TRACEPARENT` and `TRACESTATE` environment variables
- **Cross-process tracing**: Enables distributed tracing across separate processes

Example trace hierarchy:
```
DAG: parent-workflow (trace_id: abc123)
└── Step: run-child-workflow
    └── DAG: child-workflow (same trace_id: abc123)
        ├── Step: child-step-1
        └── Step: child-step-2
```

## Using Base Configuration

You can set default OpenTelemetry configuration in your base DAG:

```yaml
# base.yaml
otel:
  enabled: true
  endpoint: "otel-collector:4317"
  resource:
    deployment.environment: "production"

# my-workflow.yaml (inherits base configuration)
name: my-workflow
otel:
  resource:
    service.name: "dagu-${DAG_NAME}"  # Override specific attributes
steps:
  - echo "Processing with telemetry"
```

## Integration Examples

### Local Development with Jaeger

1. Start Jaeger with OTLP support:
```bash
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 4317:4317 \
  jaegertracing/all-in-one:latest
```

2. Configure your DAG:
```yaml
otel:
  enabled: true
  endpoint: "localhost:4317"
  insecure: true
```

3. View traces at http://localhost:16686

### Production with OpenTelemetry Collector

```yaml
dotenv:
  - .env
otel:
  enabled: true
  endpoint: "otel-collector.monitoring:4317"
  headers:
    Authorization: "Bearer ${OTEL_TOKEN}"
  resource:
    service.name: "dagu-${DAG_NAME}"
    deployment.environment: "production"
    service.version: "${VERSION}"
```
