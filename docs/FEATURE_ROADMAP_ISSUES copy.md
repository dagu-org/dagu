# Dagu Feature Roadmap - GitHub Issues

## 1. Distributed Execution & Horizontal Scaling

### Issue: Implement Agent-Based Architecture for Distributed Execution
**Title:** feat: Add agent-based architecture for distributed task execution

**Body:**
## Overview
Implement a lightweight agent system that allows Dagu to distribute task execution across multiple machines while maintaining the simplicity of the current architecture.

## Requirements
- [ ] Design agent communication protocol (gRPC or REST)
- [ ] Implement agent registration and heartbeat mechanism
- [ ] Add work distribution algorithm with task affinity support
- [ ] Maintain backward compatibility for single-node deployments
- [ ] Ensure zero-dependency principle for agent binary

## Technical Design
- Agent should be a separate binary that can run on worker nodes
- Communication via Unix sockets locally, TCP/TLS remotely
- File-based state synchronization for resilience
- Optional: Support for agent auto-discovery

## Success Criteria
- Can execute DAG steps on remote agents
- Graceful handling of agent failures
- Performance overhead < 5% for local execution

---

### Issue: Add Resource Management and Limits
**Title:** feat: Resource limits and management for steps and DAGs

**Body:**
## Overview
Add ability to specify and enforce CPU/memory limits for individual steps and entire DAGs.

## Requirements
- [ ] Add resource specification to YAML schema
- [ ] Implement resource tracking per process
- [ ] Add resource-based scheduling decisions
- [ ] Support for cgroups on Linux
- [ ] Resource usage reporting in UI

## Example Configuration
```yaml
steps:
  - name: heavy-computation
    command: process_data.py
    resources:
      cpu: "2"          # 2 CPU cores
      memory: "4Gi"     # 4GB memory
      timeout: "1h"     # Already supported
```

## Success Criteria
- Resource limits enforced on Linux systems
- Clear error messages when limits exceeded
- Resource usage visible in execution history

---

### Issue: Implement Queue-Based Auto-Scaling
**Title:** feat: Auto-scaling based on queue depth and system load

**Body:**
## Overview
Automatically scale worker capacity based on pending work and system metrics.

## Requirements
- [ ] Monitor queue depth and wait times
- [ ] Track system resource utilization
- [ ] Implement scaling policies (min/max workers)
- [ ] Support for cloud provider auto-scaling groups
- [ ] Configurable scaling thresholds

## Configuration Example
```yaml
autoscaling:
  enabled: true
  minWorkers: 1
  maxWorkers: 10
  scaleUpThreshold: 5     # Queue depth
  scaleDownThreshold: 0   # Idle time in minutes
  cooldownPeriod: 300     # Seconds
```

---

## 2. Native Integrations & Notifications

### Issue: Add Webhook Support for Generic Integrations
**Title:** feat: Webhook executor and webhook notifications

**Body:**
## Overview
Implement a webhook executor for sending notifications and integrating with external systems.

## Requirements
- [ ] New webhook executor type
- [ ] Support for custom headers and authentication
- [ ] Template support for webhook payloads
- [ ] Retry logic with exponential backoff
- [ ] Webhook signature verification for security

## Example Usage
```yaml
handlerOn:
  success:
    executor:
      type: webhook
      config:
        url: "${WEBHOOK_URL}"
        method: POST
        headers:
          Content-Type: application/json
          X-Custom-Header: "${API_KEY}"
        body: |
          {
            "dag": "${DAG_NAME}",
            "status": "success",
            "runId": "${DAG_RUN_ID}",
            "timestamp": "{{ now | date \"2006-01-02T15:04:05Z07:00\" }}"
          }
```

---

### Issue: Native Slack Integration
**Title:** feat: Add native Slack notification support

**Body:**
## Overview
Implement native Slack integration for notifications without requiring external scripts.

## Requirements
- [ ] Slack executor implementation
- [ ] Support for both webhooks and Slack API
- [ ] Rich message formatting with blocks
- [ ] Thread support for grouping related notifications
- [ ] Channel/user mention capabilities

## Configuration
```yaml
notifications:
  slack:
    webhook: "${SLACK_WEBHOOK}"
    channel: "#data-pipelines"
    mentionOn:
      failure: ["@oncall-team"]
    
steps:
  - name: process
    command: ./process.sh
    notifyOn:
      failure: slack
      success: slack
```

---

### Issue: Prometheus Metrics Exporter
**Title:** feat: Export metrics in Prometheus format

**Body:**
## Overview
Expose Dagu metrics in Prometheus format for monitoring and alerting.

## Requirements
- [ ] `/metrics` endpoint with Prometheus format
- [ ] DAG execution metrics (success/failure/duration)
- [ ] Queue depth and wait time metrics
- [ ] Resource utilization metrics
- [ ] Custom business metrics from DAG outputs

## Metrics Examples
```
dagu_dag_execution_total{dag="data-pipeline",status="success"} 142
dagu_dag_execution_duration_seconds{dag="data-pipeline",quantile="0.99"} 325.5
dagu_queue_depth{queue="default"} 5
dagu_active_runs{dag="data-pipeline"} 2
```

---

## 3. Event-Driven Triggers

### Issue: File Watcher Trigger Implementation
**Title:** feat: File-based triggers for DAG execution

**Body:**
## Overview
Trigger DAG execution when files are created/modified in specified locations.

## Requirements
- [ ] Local filesystem watching using fsnotify
- [ ] S3 event notifications support
- [ ] GCS pub/sub integration
- [ ] Azure blob storage events
- [ ] Debouncing and pattern matching

## Configuration
```yaml
triggers:
  - type: file
    config:
      path: /data/incoming
      pattern: "*.csv"
      event: create
      debounce: 10s
  - type: s3
    config:
      bucket: my-bucket
      prefix: data/
      event: s3:ObjectCreated:*
```

---

### Issue: Webhook Receiver for External Triggers
**Title:** feat: HTTP endpoint to trigger DAGs via webhooks

**Body:**
## Overview
Expose HTTP endpoints that can receive webhooks to trigger DAG execution.

## Requirements
- [ ] Dynamic endpoint registration per DAG
- [ ] Request validation and authentication
- [ ] Parameter extraction from webhook payload
- [ ] Support for different content types
- [ ] Webhook signature verification

## Example Usage
```yaml
triggers:
  - type: webhook
    config:
      path: /hooks/deploy
      auth:
        type: signature
        secret: "${WEBHOOK_SECRET}"
      parameterMapping:
        version: "$.release.version"
        environment: "$.release.environment"
```

---

### Issue: Message Queue Trigger Support
**Title:** feat: Trigger DAGs from message queue events

**Body:**
## Overview
Support triggering DAGs from popular message queue systems.

## Requirements
- [ ] Kafka consumer implementation
- [ ] RabbitMQ/AMQP support
- [ ] AWS SQS integration
- [ ] Redis pub/sub support
- [ ] Message acknowledgment handling

## Configuration
```yaml
triggers:
  - type: kafka
    config:
      brokers: ["localhost:9092"]
      topic: events
      consumerGroup: dagu-processor
      parameterMapping:
        orderId: "$.order_id"
        amount: "$.total_amount"
```

---

## 4. SDK & Programmatic DAG Generation

### Issue: Python SDK for DAG Generation
**Title:** feat: Python SDK for programmatic DAG creation

**Body:**
## Overview
Create a Python SDK that allows users to generate DAGs programmatically with type safety and IDE support.

## Requirements
- [ ] Pythonic API design
- [ ] Type hints and IDE autocomplete
- [ ] Validation at construction time
- [ ] YAML generation and direct API submission
- [ ] Compatibility with existing Python workflows

## Example Usage
```python
from dagu import DAG, Step, Schedule

with DAG(name="data-pipeline") as dag:
    dag.schedule = Schedule.cron("0 2 * * *")
    
    extract = Step("extract", command="python extract.py")
    transform = Step("transform", command="python transform.py")
    load = Step("load", command="python load.py")
    
    extract >> transform >> load
    
# Generate YAML or submit directly
dag.to_yaml("data-pipeline.yaml")
dag.submit()
```

---

### Issue: GraphQL API for Complete DAG Management
**Title:** feat: GraphQL API for programmatic DAG operations

**Body:**
## Overview
Implement a GraphQL API alongside the REST API for more flexible querying and mutations.

## Requirements
- [ ] GraphQL schema definition
- [ ] Query support for DAGs, executions, and logs
- [ ] Mutations for CRUD operations
- [ ] Subscriptions for real-time updates
- [ ] Backward compatibility with REST API

## Example Queries
```graphql
query GetDAGExecutions($dagName: String!, $limit: Int) {
  dag(name: $dagName) {
    executions(limit: $limit) {
      id
      status
      startedAt
      steps {
        name
        status
        duration
      }
    }
  }
}

mutation TriggerDAG($name: String!, $params: JSON) {
  startDAG(name: $name, parameters: $params) {
    runId
    status
  }
}
```

---

## 5. Cloud-Native Features

### Issue: Kubernetes Operator for Dagu
**Title:** feat: Kubernetes operator for native Dagu deployment

**Body:**
## Overview
Create a Kubernetes operator that manages Dagu deployments and DAGs as custom resources.

## Requirements
- [ ] CRD definitions for DaguServer and DaguDAG
- [ ] Operator implementation in Go
- [ ] Automatic scaling and rolling updates
- [ ] Integration with K8s RBAC
- [ ] Helm chart for easy installation

## Example Resources
```yaml
apiVersion: dagu.io/v1
kind: DaguServer
metadata:
  name: dagu-production
spec:
  replicas: 3
  image: dagu:latest
  storage:
    type: persistent
    size: 10Gi
---
apiVersion: dagu.io/v1
kind: DaguDAG
metadata:
  name: data-pipeline
spec:
  schedule: "0 2 * * *"
  dagSpec: |
    steps:
      - name: process
        command: python process.py
```

---

### Issue: Cloud Function Executors
**Title:** feat: Native execution on serverless platforms

**Body:**
## Overview
Add executors for running DAG steps as serverless functions on major cloud providers.

## Requirements
- [ ] AWS Lambda executor
- [ ] Google Cloud Functions executor
- [ ] Azure Functions executor
- [ ] Automatic dependency packaging
- [ ] Cost optimization through request batching

## Example Configuration
```yaml
steps:
  - name: process-data
    executor:
      type: lambda
      config:
        runtime: python3.9
        handler: handler.process
        memory: 512
        timeout: 300
        environment:
          ENV: production
```

---

## 6. Enterprise Security

### Issue: Role-Based Access Control (RBAC)
**Title:** feat: Implement RBAC for fine-grained permissions

**Body:**
## Overview
Add role-based access control to manage permissions at a granular level.

## Requirements
- [ ] Role definition system
- [ ] Permission model (read, write, execute, admin)
- [ ] DAG-level permissions
- [ ] Group/tag-based permissions
- [ ] API key scoping

## Example Configuration
```yaml
rbac:
  roles:
    - name: developer
      permissions:
        - resource: dags
          actions: [read, write]
          filter: "group in ['dev', 'staging']"
    - name: operator
      permissions:
        - resource: dags
          actions: [read, execute]
          filter: "*"
  users:
    - username: john
      roles: [developer]
    - username: jane
      roles: [operator, developer]
```

---

### Issue: SSO/SAML/OIDC Authentication
**Title:** feat: Enterprise SSO authentication support

**Body:**
## Overview
Implement SSO authentication using SAML 2.0 and OpenID Connect protocols.

## Requirements
- [ ] SAML 2.0 service provider implementation
- [ ] OpenID Connect client
- [ ] Support for major providers (Okta, Auth0, Azure AD)
- [ ] Group/claim mapping to roles
- [ ] Session management

## Configuration
```yaml
auth:
  oidc:
    enabled: true
    issuer: https://company.okta.com
    clientId: "${OIDC_CLIENT_ID}"
    clientSecret: "${OIDC_CLIENT_SECRET}"
    scopes: ["openid", "profile", "email", "groups"]
    groupClaim: groups
    roleMapping:
      admin: ["dagu-admins"]
      operator: ["dagu-operators"]
```

---

### Issue: Secrets Management Integration
**Title:** feat: Native integration with secret management systems

**Body:**
## Overview
Integrate with popular secret management systems for secure credential handling.

## Requirements
- [ ] HashiCorp Vault integration
- [ ] AWS Secrets Manager support
- [ ] Azure Key Vault integration
- [ ] Kubernetes secrets support
- [ ] Runtime secret injection

## Example Usage
```yaml
env:
  - DB_PASSWORD: "vault://secret/database/password"
  - API_KEY: "aws-secrets://prod/api/key"
  - CERT: "azurekv://mykeyvault/certificates/ssl-cert"

steps:
  - name: connect
    command: psql -U user -p "$DB_PASSWORD"
    secrets:
      provider: vault
      path: secret/database
      keys: ["password", "username"]
```

---

## 7. Data Pipeline Primitives

### Issue: Built-in Database Connectors
**Title:** feat: Native database connectors for common operations

**Body:**
## Overview
Add built-in connectors for common database operations without requiring custom scripts.

## Requirements
- [ ] PostgreSQL connector
- [ ] MySQL/MariaDB connector
- [ ] MongoDB connector
- [ ] Redis connector
- [ ] Snowflake connector
- [ ] Connection pooling and management

## Example Usage
```yaml
steps:
  - name: extract-users
    executor:
      type: postgres
      config:
        connection: "${DATABASE_URL}"
        query: |
          SELECT * FROM users 
          WHERE created_at >= '{{ .LAST_RUN }}'
        output: query_result
    output: USERS_DATA
  
  - name: load-to-warehouse
    executor:
      type: snowflake
      config:
        account: myaccount
        warehouse: COMPUTE_WH
        database: ANALYTICS
        schema: RAW
        table: users_staging
        mode: overwrite
        input: "${USERS_DATA}"
```

---

### Issue: Backfill Operations Support
**Title:** feat: Built-in support for backfill operations

**Body:**
## Overview
Add native support for backfilling historical data with date range iteration.

## Requirements
- [ ] Date range specification in DAG
- [ ] Parallel backfill execution
- [ ] Progress tracking and resumption
- [ ] Rate limiting for API calls
- [ ] Backfill-specific UI

## Example Configuration
```yaml
backfill:
  enabled: true
  startDate: "2024-01-01"
  endDate: "2024-12-31"
  interval: daily
  parallel: 5
  params:
    - DATE: "{{ .BackfillDate }}"

steps:
  - name: process-daily-data
    command: |
      python process.py --date=${DATE}
```

---

### Issue: SLA Tracking and Alerting
**Title:** feat: SLA monitoring with alerting capabilities

**Body:**
## Overview
Implement SLA (Service Level Agreement) tracking for DAGs with configurable alerting.

## Requirements
- [ ] SLA definition per DAG/step
- [ ] Real-time SLA violation detection
- [ ] Historical SLA compliance reporting
- [ ] Integration with alerting systems
- [ ] SLA dashboard in UI

## Configuration
```yaml
sla:
  expectedDuration: 30m
  expectedCompletionTime: "08:00"
  alertOn:
    - violation
    - missedRun
  
steps:
  - name: critical-step
    command: process.sh
    sla:
      maxDuration: 5m
      alertChannels: ["pagerduty", "slack"]
```

---

### Issue: Data Quality Assertions
**Title:** feat: Built-in data quality checks and assertions

**Body:**
## Overview
Add native support for data quality assertions as part of the pipeline execution.

## Requirements
- [ ] Built-in assertion types (row count, null checks, etc.)
- [ ] Custom assertion support
- [ ] Assertion results in UI
- [ ] Quality metrics tracking
- [ ] Integration with data observability tools

## Example Usage
```yaml
steps:
  - name: validate-data
    command: echo "1000"
    output: ROW_COUNT
    assertions:
      - type: numeric_range
        value: "${ROW_COUNT}"
        min: 900
        max: 1100
        message: "Row count outside expected range"
  
  - name: check-quality
    executor:
      type: data_quality
      config:
        source: postgres://localhost/db
        checks:
          - table: users
            assertions:
              - no_nulls: [email, created_at]
              - unique: [email]
              - freshness: 
                  column: updated_at
                  threshold: 1h
```

---

## Implementation Priority

### Phase 1 (Foundation)
1. Webhook support (enables many integrations)
2. Python SDK (developer adoption)
3. File watcher triggers (common use case)
4. Prometheus metrics (operations visibility)

### Phase 2 (Scale)
1. Agent architecture (distributed execution)
2. Kubernetes operator (cloud-native deployment)
3. RBAC implementation (enterprise requirement)
4. Database connectors (data pipeline use cases)

### Phase 3 (Enterprise)
1. SSO/SAML support (enterprise authentication)
2. Cloud function executors (cost optimization)
3. SLA tracking (production requirements)
4. Advanced integrations (Slack, PagerDuty, etc.)

Each issue should be created with appropriate labels:
- `enhancement` for new features
- `priority:high/medium/low` based on impact
- `size:S/M/L/XL` for effort estimation
- `needs-design` for features requiring design discussion