# Use Cases

Real-world scenarios where Dagu excels.

## Data Engineering

### ETL Pipelines
Extract, transform, and load data across systems with complex dependencies.

```yaml
name: daily-etl
schedule: "0 2 * * *"

steps:
  - name: extract-sources
    parallel:
      items: [mysql, postgres, mongodb]
    run: extractors/database
    params: "SOURCE=${ITEM}"
    
  - name: transform-data
    command: python transform.py --date=${DATE}
    depends: extract-sources
    
  - name: load-warehouse
    command: |
      spark-submit load_to_warehouse.py \
        --input=/tmp/transformed \
        --output=s3://warehouse/
    depends: transform-data
    
  - name: update-analytics
    command: dbt run --models analytics
    depends: load-warehouse

handlerOn:
  failure:
    command: ./notify-data-team.sh
```

### Data Quality Checks
Automated validation and monitoring of data pipelines.

```yaml
name: data-quality-checks
schedule: "*/30 * * * *"

steps:
  - name: check-freshness
    command: |
      python check_freshness.py \
        --table=orders \
        --threshold=3600
    continueOn:
      failure: true
      
  - name: validate-schemas
    command: great_expectations run
    continueOn:
      failure: true
      
  - name: anomaly-detection
    command: python detect_anomalies.py
    continueOn:
      failure: true
      
  - name: generate-report
    command: python quality_report.py
    depends:
      - check-freshness
      - validate-schemas
      - anomaly-detection
```

## DevOps Automation

### CI/CD Pipelines
Build, test, and deploy applications with sophisticated workflows.

```yaml
name: deploy-application
params:
  - ENVIRONMENT: staging
  - VERSION: latest

steps:
  - name: run-tests
    executor:
      type: docker
      config:
        image: node:18
        volumes:
          - .:/app
    command: |
      cd /app
      npm ci
      npm test
      
  - name: build-image
    command: |
      docker build -t myapp:${VERSION} .
      docker push registry.example.com/myapp:${VERSION}
    depends: run-tests
    
  - name: deploy
    run: deploy/kubernetes
    params: "ENV=${ENVIRONMENT} IMAGE=myapp:${VERSION}"
    depends: build-image
    preconditions:
      - condition: "${ENVIRONMENT}"
        expected: "re:staging|production"
```

### Infrastructure Automation
Manage cloud resources and infrastructure as code.

```yaml
name: provision-infrastructure
params:
  - REGION: us-east-1
  - ENVIRONMENT: dev

steps:
  - name: validate-terraform
    command: |
      cd terraform/
      terraform fmt -check
      terraform validate
      
  - name: plan-changes
    command: |
      terraform plan \
        -var="region=${REGION}" \
        -var="env=${ENVIRONMENT}" \
        -out=tfplan
    depends: validate-terraform
    
  - name: apply-changes
    command: terraform apply -auto-approve tfplan
    depends: plan-changes
    preconditions:
      - condition: "${ENVIRONMENT}"
        expected: "re:dev|staging"
        
  - name: update-inventory
    command: |
      ansible-inventory --refresh-cache
      ansible all -m ping
    depends: apply-changes
```

## Business Process Automation

### Report Generation
Automated business reporting and distribution.

```yaml
name: monthly-reports
schedule: "0 9 1 * *"  # First day of month at 9 AM

steps:
  - name: gather-data
    parallel:
      items:
        - sales: "SELECT * FROM sales WHERE month = CURRENT_MONTH"
        - inventory: "SELECT * FROM inventory"
        - customers: "SELECT * FROM customers WHERE active = true"
    run: queries/extract
    output: REPORT_DATA
    
  - name: generate-reports
    parallel:
      items: [executive, operations, finance]
    run: reports/generate
    params: "TYPE=${ITEM} DATA=${REPORT_DATA}"
    depends: gather-data
    
  - name: distribute-reports
    executor:
      type: mail
      config:
        to: "reports@company.com"
        subject: "Monthly Reports - ${MONTH}"
        attachments:
          - /tmp/reports/*.pdf
    depends: generate-reports
```

### Employee Onboarding
Coordinate complex multi-department processes.

```yaml
name: employee-onboarding
params:
  - EMPLOYEE_ID: ""
  - START_DATE: ""

steps:
  - name: create-accounts
    parallel:
      items: [email, slack, github, aws]
    run: onboarding/create-account
    params: "SYSTEM=${ITEM} EMPLOYEE=${EMPLOYEE_ID}"
    
  - name: assign-equipment
    command: |
      python assign_equipment.py \
        --employee=${EMPLOYEE_ID} \
        --type=developer
    depends: create-accounts
    
  - name: schedule-training
    command: |
      python schedule_training.py \
        --employee=${EMPLOYEE_ID} \
        --start=${START_DATE}
    depends: create-accounts
    
  - name: notify-manager
    executor:
      type: mail
      config:
        to: "{{.MANAGER_EMAIL}}"
        subject: "New Team Member Ready"
    depends:
      - assign-equipment
      - schedule-training
```

## System Maintenance

### Backup and Recovery
Automated backup strategies with verification.

```yaml
name: backup-critical-systems
schedule: "0 3 * * *"

steps:
  - name: backup-databases
    parallel:
      items: 
        - name: postgres
          host: db1.internal
          path: /backups/postgres
        - name: mysql
          host: db2.internal
          path: /backups/mysql
    run: backup/database
    output: BACKUP_RESULTS
    
  - name: backup-files
    command: |
      rsync -avz /var/app/ /backups/files/
      tar -czf /backups/app-$(date +%Y%m%d).tar.gz /backups/files/
    depends: backup-databases
    
  - name: verify-backups
    command: python verify_backups.py ${BACKUP_RESULTS}
    depends: backup-files
    retryPolicy:
      limit: 3
      
  - name: upload-offsite
    command: |
      aws s3 sync /backups/ s3://backup-bucket/ \
        --storage-class GLACIER
    depends: verify-backups
```

### Log Rotation and Cleanup
Maintain system health with automated maintenance.

```yaml
name: system-maintenance
schedule: "0 4 * * SUN"

steps:
  - name: rotate-logs
    command: |
      find /var/log -name "*.log" -mtime +7 -exec gzip {} \;
      find /var/log -name "*.gz" -mtime +30 -delete
      
  - name: clean-temp
    command: |
      find /tmp -type f -atime +7 -delete
      find /var/tmp -type f -atime +7 -delete
      
  - name: optimize-databases
    parallel:
      items: [postgres, mysql, redis]
    command: optimize-${ITEM}.sh
    
  - name: system-updates
    command: |
      apt-get update
      apt-get upgrade -y
      apt-get autoremove -y
    depends:
      - rotate-logs
      - clean-temp
      - optimize-databases
```

## Scientific Computing

### Research Pipelines
Orchestrate complex computational workflows.

```yaml
name: genomics-analysis
params:
  - SAMPLE_ID: ""
  - REFERENCE: "hg38"

steps:
  - name: quality-control
    executor:
      type: docker
      config:
        image: biocontainers/fastqc:latest
    command: fastqc /data/${SAMPLE_ID}.fastq
    
  - name: alignment
    executor:
      type: docker
      config:
        image: biocontainers/bwa:latest
        cpus: 8
        memory: 32g
    command: |
      bwa mem -t 8 \
        /ref/${REFERENCE}.fa \
        /data/${SAMPLE_ID}.fastq > aligned.sam
    depends: quality-control
    
  - name: variant-calling
    command: |
      samtools sort aligned.sam > sorted.bam
      bcftools mpileup -f /ref/${REFERENCE}.fa sorted.bam | \
        bcftools call -mv > variants.vcf
    depends: alignment
    
  - name: annotation
    command: vep --input_file variants.vcf --output_file annotated.vcf
    depends: variant-calling
```

### Machine Learning Pipelines
Automate model training and deployment.

```yaml
name: ml-training-pipeline
params:
  - EXPERIMENT: "model_v2"
  - DATASET: "2024_Q1"

steps:
  - name: prepare-data
    command: |
      python prepare_dataset.py \
        --input=s3://raw-data/${DATASET} \
        --output=/data/prepared/
    output: PREPARED_DATA
    
  - name: train-models
    parallel:
      items:
        - model: xgboost
          params: "max_depth=6 learning_rate=0.1"
        - model: lightgbm
          params: "num_leaves=31 learning_rate=0.05"
        - model: catboost
          params: "depth=6 learning_rate=0.03"
    run: ml/train
    params: "DATA=${PREPARED_DATA} MODEL=${ITEM.model} PARAMS=${ITEM.params}"
    depends: prepare-data
    
  - name: evaluate-models
    command: |
      python evaluate_models.py \
        --experiment=${EXPERIMENT} \
        --metrics="accuracy,f1,roc_auc"
    depends: train-models
    output: BEST_MODEL
    
  - name: deploy-winner
    command: |
      python deploy_model.py \
        --model=${BEST_MODEL} \
        --endpoint=production
    depends: evaluate-models
    preconditions:
      - condition: "${BEST_MODEL.accuracy}"
        expected: "re:0\\.[89]\\d+"  # > 80% accuracy
```

## Integration Scenarios

### Multi-System Synchronization
Keep multiple systems in sync with complex orchestration.

```yaml
name: sync-customer-data
schedule: "*/15 * * * *"  # Every 15 minutes

steps:
  - name: extract-changes
    command: |
      python extract_changes.py \
        --source=salesforce \
        --since=$(date -d '15 minutes ago' +%Y-%m-%dT%H:%M:%S)
    output: CHANGES
    
  - name: transform-data
    command: python transform_customer_data.py
    depends: extract-changes
    continueOn:
      failure: false
      
  - name: update-systems
    parallel:
      items: [crm, billing, analytics, support]
    run: sync/update-system
    params: "SYSTEM=${ITEM} DATA=${CHANGES}"
    depends: transform-data
    
  - name: verify-sync
    command: python verify_consistency.py
    depends: update-systems
    retryPolicy:
      limit: 3
      intervalSec: 60
```

### API Integration Pipeline
Orchestrate complex API workflows with error handling.

```yaml
name: api-integration
params:
  - API_KEY: "${SECRET_API_KEY}"
  - BATCH_SIZE: 100

steps:
  - name: authenticate
    executor:
      type: http
      config:
        url: https://api.example.com/auth
        method: POST
        headers:
          Content-Type: application/json
        body: '{"api_key": "${API_KEY}"}'
    output: AUTH_TOKEN
    
  - name: fetch-data
    command: |
      python fetch_paginated.py \
        --token=${AUTH_TOKEN} \
        --batch=${BATCH_SIZE}
    depends: authenticate
    retryPolicy:
      limit: 5
      intervalSec: 30
      exitCode: [429]  # Rate limit
      
  - name: process-batches
    parallel: ${BATCHES}
    run: process/batch
    params: "BATCH=${ITEM} TOKEN=${AUTH_TOKEN}"
    depends: fetch-data
    
  - name: aggregate-results
    command: python aggregate.py
    depends: process-batches
```

## Legacy System Modernization

### Gradual Migration
Orchestrate complex system migrations with fallback strategies.

```yaml
name: migrate-to-cloud
params:
  - MIGRATION_PHASE: "phase1"
  - ROLLBACK: "false"

steps:
  - name: backup-legacy
    command: ./backup-legacy-system.sh
    
  - name: migrate-data
    command: |
      python migrate_data.py \
        --phase=${MIGRATION_PHASE} \
        --source=legacy \
        --target=cloud
    depends: backup-legacy
    output: MIGRATION_RESULT
    
  - name: validate-migration
    command: python validate_data.py ${MIGRATION_RESULT}
    depends: migrate-data
    continueOn:
      failure: false
      
  - name: switch-traffic
    command: |
      ./update-load-balancer.sh \
        --legacy=20 \
        --cloud=80
    depends: validate-migration
    preconditions:
      - condition: "${ROLLBACK}"
        expected: "false"
        
  - name: rollback
    command: ./rollback-migration.sh
    depends: validate-migration
    preconditions:
      - condition: "${ROLLBACK}"
        expected: "true"
```

## Best Practices for Different Use Cases

### 1. Data Engineering
- Use `output` variables to pass data between steps
- Implement proper error handling with `continueOn`
- Set appropriate retry policies for transient failures
- Use parallel execution for independent data sources

### 2. DevOps
- Leverage Docker executor for consistent environments
- Use preconditions to gate production deployments
- Implement approval workflows with manual triggers
- Store artifacts in external systems (S3, Artifactory)

### 3. Business Automation
- Schedule workflows according to business hours
- Use email notifications for stakeholder communication
- Implement audit logging for compliance
- Design for graceful degradation

### 4. Scientific Computing
- Use appropriate executors for compute requirements
- Implement checkpointing for long-running processes
- Design workflows to be restartable
- Consider resource limits and quotas

## When Dagu is the Right Choice

✅ **Perfect for:**
- Complex multi-step workflows
- Heterogeneous environments (multiple languages/tools)
- Replacing scattered cron jobs
- Local/on-premise requirements
- Quick prototyping and iteration

❌ **Consider alternatives for:**
- Massive scale (millions of tasks)
- Real-time streaming pipelines
- Pure container orchestration (use K8s)
- Simple single-step cron jobs

## Success Stories

### Case 1: E-commerce Platform
**Challenge**: Coordinate inventory, pricing, and catalog updates across multiple systems.

**Solution**: Dagu workflows that sync data every 15 minutes, handle failures gracefully, and provide full audit trails.

**Result**: 99.9% sync reliability, 70% reduction in manual interventions.

### Case 2: Financial Services
**Challenge**: Complex end-of-day reconciliation across trading systems.

**Solution**: Hierarchical DAGs that process millions of transactions with proper error handling and notifications.

**Result**: Processing time reduced from 6 hours to 45 minutes.

### Case 3: Healthcare Research
**Challenge**: Orchestrate genomics pipelines across different compute environments.

**Solution**: Dagu with mixed executors (local, Docker, HPC) for optimal resource usage.

**Result**: 10x increase in research throughput with same infrastructure.

## Next Steps

- [Installation](/getting-started/installation) - Get Dagu running
- [Examples](/writing-workflows/examples/) - See more workflow patterns
- [Writing Workflows](/writing-workflows/) - Build your own workflows