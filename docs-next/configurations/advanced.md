# Advanced Setup

Advanced configuration patterns and integration strategies for Dagu.

## High Availability Patterns

### Active-Passive Setup

Deploy Dagu in an active-passive configuration for failover:

```yaml
# Primary node config
# /etc/dagu/config-primary.yaml
host: 0.0.0.0
port: 8080
ui:
  navbarTitle: "Dagu - Primary"
  navbarColor: "#1976d2"

# Shared storage (NFS, EFS, etc.)
dagsDir: /mnt/shared/dagu/dags
dataDir: /mnt/shared/dagu/data
logDir: /mnt/shared/dagu/logs
```

```yaml
# Secondary node config
# /etc/dagu/config-secondary.yaml
host: 0.0.0.0
port: 8080
ui:
  navbarTitle: "Dagu - Secondary"
  navbarColor: "#757575"

# Same shared storage
dagsDir: /mnt/shared/dagu/dags
dataDir: /mnt/shared/dagu/data
logDir: /mnt/shared/dagu/logs

# Start in read-only mode
permissions:
  writeDAGs: false
  runDAGs: false
```

Health check script for load balancer:
```bash
#!/bin/bash
# /opt/dagu/health_check.sh

# Check if this is the active node
if [ -f /mnt/shared/dagu/.active_node ]; then
    ACTIVE_NODE=$(cat /mnt/shared/dagu/.active_node)
    CURRENT_NODE=$(hostname)
    
    if [ "$ACTIVE_NODE" = "$CURRENT_NODE" ]; then
        # Check if Dagu is running
        if curl -f -s http://localhost:8080/api/v1/dags > /dev/null; then
            exit 0  # Healthy
        fi
    fi
fi

exit 1  # Not active or unhealthy
```

### Load Balanced Setup

For read-heavy workloads, use multiple read-only instances:

```nginx
# nginx.conf
upstream dagu_read {
    least_conn;
    server dagu-read-1:8080 weight=1;
    server dagu-read-2:8080 weight=1;
    server dagu-read-3:8080 weight=1;
}

upstream dagu_write {
    server dagu-primary:8080;
}

server {
    listen 80;
    
    # Read operations
    location ~ ^/api/v1/(dags|status|logs) {
        proxy_pass http://dagu_read;
        proxy_cache dagu_cache;
        proxy_cache_valid 200 10s;
    }
    
    # Write operations
    location ~ ^/api/v1/(dags/.*/start|dags/.*/stop) {
        proxy_pass http://dagu_write;
    }
    
    # UI
    location / {
        proxy_pass http://dagu_read;
    }
}
```

### Multi-Region Setup

Deploy Dagu across regions with data replication:

```yaml
# Region 1 (Primary)
remoteNodes:
  - name: "us-west-2"
    apiBaseURL: "https://dagu-us-west-2.company.com/api/v1"
    isAuthToken: true
    authToken: "${REGION_2_TOKEN}"
    
  - name: "eu-central-1"
    apiBaseURL: "https://dagu-eu-central-1.company.com/api/v1"
    isAuthToken: true
    authToken: "${REGION_3_TOKEN}"
```

Sync script for DAGs:
```bash
#!/bin/bash
# Sync DAGs across regions
REGIONS=("us-east-1" "us-west-2" "eu-central-1")
SOURCE_BUCKET="s3://dagu-dags-primary"

for region in "${REGIONS[@]}"; do
    echo "Syncing to $region..."
    aws s3 sync $SOURCE_BUCKET s3://dagu-dags-$region \
        --region $region \
        --delete \
        --exclude "*.tmp"
done
```

## Remote Nodes Configuration

### Multi-Environment Management

Configure Dagu to manage multiple environments from a single UI:

```yaml
# ~/.config/dagu/config.yaml
remoteNodes:
  - name: "development"
    apiBaseURL: "http://dev.internal:8080/api/v1"
    isBasicAuth: true
    basicAuthUsername: "dev_user"
    basicAuthPassword: "${DEV_PASSWORD}"
    
  - name: "staging"
    apiBaseURL: "https://staging.company.com/api/v1"
    isAuthToken: true
    authToken: "${STAGING_TOKEN}"
    skipTLSVerify: false
    
  - name: "production"
    apiBaseURL: "https://prod.company.com/api/v1"
    isAuthToken: true
    authToken: "${PROD_TOKEN}"
    skipTLSVerify: false
    
  - name: "disaster-recovery"
    apiBaseURL: "https://dr.company.com/api/v1"
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
    apiBaseURL: "https://secure.company.com/api/v1"
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
            APIBaseURL: fmt.Sprintf("http://%s:%d/api/v1", 
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
    # High-priority queue for critical jobs
    - name: "critical"
      maxConcurrency: 10
      priority: 100
      
    # CPU-intensive jobs queue
    - name: "cpu-intensive"
      maxConcurrency: 2  # Limit to number of CPU cores
      priority: 50
      
    # I/O-intensive jobs queue
    - name: "io-intensive"
      maxConcurrency: 20  # Can handle more concurrent I/O
      priority: 50
      
    # Batch processing queue
    - name: "batch"
      maxConcurrency: 1   # Sequential processing
      priority: 10
      
    # Default queue
    - name: "default"
      maxConcurrency: 5
      priority: 30
```

### Queue Selection Strategy

Base configuration for queue assignment:

```yaml
# base.yaml - Shared by all DAGs
# Assign queue based on tags
tags:
  - type: batch
  - priority: low
  
queue: "{{ if contains .Tags \"critical\" }}critical{{ else if contains .Tags \"batch\" }}batch{{ else }}default{{ end }}"
```

### Dynamic Queue Scaling

Monitor and adjust queue concurrency:

```bash
#!/bin/bash
# /opt/dagu/scripts/queue_autoscale.sh

# Monitor CPU usage and adjust queue concurrency
CPU_USAGE=$(top -bn1 | grep "Cpu(s)" | awk '{print $2}' | cut -d'%' -f1)
CURRENT_CONCURRENCY=$(grep -A1 "cpu-intensive" /etc/dagu/config.yaml | grep maxConcurrency | awk '{print $2}')

if (( $(echo "$CPU_USAGE > 80" | bc -l) )); then
    # Reduce concurrency
    NEW_CONCURRENCY=$((CURRENT_CONCURRENCY - 1))
elif (( $(echo "$CPU_USAGE < 50" | bc -l) )); then
    # Increase concurrency
    NEW_CONCURRENCY=$((CURRENT_CONCURRENCY + 1))
fi

# Update configuration
sed -i "/cpu-intensive/,/maxConcurrency/ s/maxConcurrency: .*/maxConcurrency: $NEW_CONCURRENCY/" /etc/dagu/config.yaml

# Reload Dagu
systemctl reload dagu
```

## CI/CD Integration

### GitHub Actions

Deploy DAGs automatically on push:

```yaml
# .github/workflows/deploy-dags.yml
name: Deploy DAGs

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
          
  deploy:
    needs: validate
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Deploy to Staging
        run: |
          rsync -avz --delete dags/ user@staging:/opt/dagu/dags/
          
      - name: Reload Dagu
        run: |
          ssh user@staging "sudo systemctl reload dagu"
          
      - name: Run smoke tests
        run: |
          ./scripts/smoke_tests.sh staging
          
      - name: Deploy to Production
        if: success()
        run: |
          rsync -avz --delete dags/ user@production:/opt/dagu/dags/
          ssh user@production "sudo systemctl reload dagu"
```

### GitLab CI/CD

```yaml
# .gitlab-ci.yml
stages:
  - validate
  - deploy-staging
  - test
  - deploy-production

validate-dags:
  stage: validate
  image: alpine:latest
  before_script:
    - apk add --no-cache curl bash
    - curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash
  script:
    - |
      for dag in dags/*.yaml; do
        dagu dry "$dag" || exit 1
      done

deploy-staging:
  stage: deploy-staging
  script:
    - rsync -avz --delete dags/ $STAGING_HOST:/opt/dagu/dags/
    - ssh $STAGING_HOST "sudo systemctl reload dagu"
  only:
    - main

integration-tests:
  stage: test
  script:
    - ./tests/integration_tests.sh $STAGING_URL
  needs:
    - deploy-staging

deploy-production:
  stage: deploy-production
  script:
    - rsync -avz --delete dags/ $PROD_HOST:/opt/dagu/dags/
    - ssh $PROD_HOST "sudo systemctl reload dagu"
  when: manual
  only:
    - main
```

### Jenkins Pipeline

```groovy
// Jenkinsfile
pipeline {
    agent any
    
    environment {
        DAGU_STAGING = credentials('dagu-staging')
        DAGU_PROD = credentials('dagu-production')
    }
    
    stages {
        stage('Validate') {
            steps {
                sh '''
                    curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash
                    for dag in dags/*.yaml; do
                        ./dagu dry "$dag"
                    done
                '''
            }
        }
        
        stage('Deploy to Staging') {
            steps {
                sshagent(['staging-ssh']) {
                    sh '''
                        rsync -avz --delete dags/ staging:/opt/dagu/dags/
                        ssh staging "sudo systemctl reload dagu"
                    '''
                }
            }
        }
        
        stage('Integration Tests') {
            steps {
                sh './tests/run_integration_tests.sh staging'
            }
        }
        
        stage('Deploy to Production') {
            when {
                branch 'main'
            }
            input {
                message "Deploy to production?"
                ok "Deploy"
            }
            steps {
                sshagent(['production-ssh']) {
                    sh '''
                        rsync -avz --delete dags/ production:/opt/dagu/dags/
                        ssh production "sudo systemctl reload dagu"
                    '''
                }
            }
        }
    }
    
    post {
        failure {
            emailext (
                to: 'ops-team@company.com',
                subject: "Dagu Deployment Failed: ${env.JOB_NAME}",
                body: "The deployment has failed. Check ${env.BUILD_URL}"
            )
        }
    }
}
```

### ArgoCD Integration

Deploy Dagu and DAGs using GitOps:

```yaml
# argocd-app.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: dagu
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/company/dagu-config
    targetRevision: HEAD
    path: k8s
  destination:
    server: https://kubernetes.default.svc
    namespace: dagu
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
    - CreateNamespace=true
```

## API Integration

### REST API Client Examples

Python client:
```python
import requests
from typing import Dict, List

class DaguClient:
    def __init__(self, base_url: str, token: str):
        self.base_url = base_url
        self.headers = {"Authorization": f"Bearer {token}"}
    
    def list_dags(self) -> List[Dict]:
        response = requests.get(
            f"{self.base_url}/api/v1/dags",
            headers=self.headers
        )
        response.raise_for_status()
        return response.json()
    
    def start_dag(self, dag_name: str, params: Dict = None) -> Dict:
        response = requests.post(
            f"{self.base_url}/api/v1/dags/{dag_name}/runs",
            headers=self.headers,
            json={"params": params} if params else {}
        )
        response.raise_for_status()
        return response.json()
    
    def get_status(self, dag_name: str) -> Dict:
        response = requests.get(
            f"{self.base_url}/api/v1/dags/{dag_name}/status",
            headers=self.headers
        )
        response.raise_for_status()
        return response.json()

# Usage
client = DaguClient("http://localhost:8080", "your-token")
dags = client.list_dags()
run = client.start_dag("etl-pipeline", {"date": "2024-01-15"})
```

### Webhook Integration

Trigger DAGs from external events:

```yaml
# webhook-handler.yaml
name: webhook-handler
steps:
  - name: receive-webhook
    executor:
      type: http
      config:
        url: "http://localhost:8080/api/v1/dags/process-order/runs"
        method: POST
        headers:
          Authorization: "Bearer ${DAGU_TOKEN}"
        body: |
          {
            "params": {
              "order_id": "${WEBHOOK_ORDER_ID}",
              "customer_id": "${WEBHOOK_CUSTOMER_ID}"
            }
          }
```

### Event-Driven Architecture

Use Dagu with message queues:

```python
# sqs_trigger.py
import boto3
import requests
import json

sqs = boto3.client('sqs')
queue_url = 'https://sqs.us-east-1.amazonaws.com/123456789012/dagu-triggers'

def process_messages():
    while True:
        response = sqs.receive_message(
            QueueUrl=queue_url,
            MaxNumberOfMessages=10,
            WaitTimeSeconds=20
        )
        
        for message in response.get('Messages', []):
            body = json.loads(message['Body'])
            
            # Trigger DAG
            dag_response = requests.post(
                f"http://localhost:8080/api/v1/dags/{body['dag_name']}/runs",
                headers={"Authorization": f"Bearer {os.getenv('DAGU_TOKEN')}"},
                json={"params": body.get('params', {})}
            )
            
            if dag_response.status_code == 200:
                sqs.delete_message(
                    QueueUrl=queue_url,
                    ReceiptHandle=message['ReceiptHandle']
                )
```

## Custom Executors

### Creating Custom Executors

While Dagu doesn't support plugins, you can create wrapper scripts:

```bash
#!/bin/bash
# /opt/dagu/executors/kafka_executor.sh

# Parse arguments
TOPIC=$1
MESSAGE=$2
KAFKA_BROKER=${KAFKA_BROKER:-localhost:9092}

# Send to Kafka
echo "$MESSAGE" | kafka-console-producer \
    --broker-list $KAFKA_BROKER \
    --topic $TOPIC

# Use in DAG
# steps:
#   - name: send-to-kafka
#     command: /opt/dagu/executors/kafka_executor.sh orders "Order processed"
```

### Executor Patterns

Common patterns for extending functionality:

1. **Database Executor**
   ```bash
   #!/bin/bash
   # db_executor.sh
   DB_TYPE=$1
   QUERY=$2
   
   case $DB_TYPE in
     postgres)
       psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "$QUERY"
       ;;
     mysql)
       mysql -h $DB_HOST -u $DB_USER -p$DB_PASS $DB_NAME -e "$QUERY"
       ;;
     mongodb)
       mongo --host $DB_HOST -u $DB_USER -p $DB_PASS $DB_NAME --eval "$QUERY"
       ;;
   esac
   ```

2. **Cloud Service Executor**
   ```bash
   #!/bin/bash
   # cloud_executor.sh
   SERVICE=$1
   ACTION=$2
   PARAMS=$3
   
   case $SERVICE in
     lambda)
       aws lambda invoke --function-name $ACTION \
         --payload "$PARAMS" /tmp/lambda-response.json
       ;;
     gcp-function)
       gcloud functions call $ACTION --data "$PARAMS"
       ;;
     azure-function)
       az functionapp function invoke -n $ACTION \
         --function-name $ACTION --data "$PARAMS"
       ;;
   esac
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

### Resource Management

Control resource usage per DAG:

```yaml
# resource-intensive.yaml
name: resource-intensive-job
env:
  # Limit memory usage
  - GOMEMLIMIT: 2GiB
  # CPU affinity
  - GOMAXPROCS: 4
  
steps:
  - name: memory-check
    command: |
      # Check available memory before starting
      available=$(free -m | awk 'NR==2{print $7}')
      if [ $available -lt 2048 ]; then
        echo "Insufficient memory"
        exit 1
      fi
      
  - name: cpu-intensive-task
    command: |
      # Use taskset to limit CPU cores
      taskset -c 0-3 ./heavy_computation.sh
```

## Security Patterns

### Secrets Management

Integrate with secret management systems:

```yaml
# vault-integration.yaml
name: secure-workflow
steps:
  - name: fetch-secrets
    command: |
      # Fetch from HashiCorp Vault
      export VAULT_TOKEN=$(cat /etc/dagu/vault-token)
      DB_PASSWORD=$(vault kv get -field=password secret/database)
      API_KEY=$(vault kv get -field=api_key secret/external-api)
      
      # Export for next steps
      echo "DB_PASSWORD=$DB_PASSWORD" > /tmp/secrets.env
      echo "API_KEY=$API_KEY" >> /tmp/secrets.env
    
  - name: use-secrets
    command: |
      source /tmp/secrets.env
      ./process_with_secrets.sh
      rm /tmp/secrets.env
    depends: fetch-secrets
```

### Audit Compliance

Implement audit trails:

```yaml
# audit-wrapper.yaml
name: audited-workflow
handlerOn:
  start:
    command: |
      echo "$(date) - Workflow started by ${DAGU_USER:-system}" >> /var/log/dagu/audit.log
      
  exit:
    command: |
      echo "$(date) - Workflow completed with status ${DAG_STATUS}" >> /var/log/dagu/audit.log
      
steps:
  - name: sensitive-operation
    command: |
      echo "$(date) - Executing sensitive operation" >> /var/log/dagu/audit.log
      ./sensitive_operation.sh
```

## Next Steps

- Review [security best practices](/configurations/operations#security-hardening)
- Explore [API documentation](/reference/api) for integration
- Set up [monitoring and alerting](/configurations/operations#monitoring)
- Configure [backup and recovery](/configurations/operations#backup-and-recovery)