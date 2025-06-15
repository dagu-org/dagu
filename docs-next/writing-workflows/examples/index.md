# Examples

Quick reference for all Dagu features. Each example is minimal and copy-paste ready.

## Basic Workflow Patterns

<div class="examples-grid">

<div class="example-card">

### Basic Sequential Steps

```yaml
steps:
  - name: first
    command: echo "Step 1"
  - name: second
    command: echo "Step 2"
    depends: first
```

Execute steps one after another.

<a href="/writing-workflows/basics#sequential-execution" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Parallel Execution

```yaml
steps:
  - name: process-items
    run: processor
    parallel:
      items: [A, B, C]
      maxConcurrent: 2
```

Process multiple items simultaneously.

<a href="/features/execution-control#parallel" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Complex Dependencies

```yaml
steps:
  - name: setup
    command: ./setup.sh
  - name: test-a
    command: ./test-a.sh
    depends: setup
  - name: test-b
    command: ./test-b.sh
    depends: setup
  - name: deploy
    command: ./deploy.sh
    depends:
      - test-a
      - test-b
```

Define complex dependency graphs.

<a href="/writing-workflows/control-flow#dependencies" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Chain vs Graph Execution

```yaml
# Chain type (default) - automatic dependencies
type: chain
steps:
  - name: download
    command: wget data.csv
  - name: process
    command: python process.py  # Depends on download
  - name: upload
    command: aws s3 cp output.csv s3://bucket/

---

# Graph type - explicit dependencies
type: graph  
steps:
  - name: step1
    command: echo "First"
  - name: step2
    command: echo "Second"
    depends: step1  # Explicit dependency required
```

Control execution flow patterns.

<a href="/writing-workflows/control-flow#execution-types" class="learn-more">Learn more →</a>

</div>

</div>

## Control Flow & Conditions

<div class="examples-grid">

<div class="example-card">

### Conditional Execution

```yaml
steps:
  - name: deploy
    command: ./deploy.sh
    preconditions:
      - condition: "${ENV}"
        expected: "production"
```

Run steps only when conditions are met.

<a href="/writing-workflows/control-flow#conditions" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Advanced Preconditions

```yaml
steps:
  - name: conditional-task
    command: ./process.sh
    preconditions:
      - test -f /data/input.csv
      - test -s /data/input.csv  # File exists and is not empty
      - condition: "${ENVIRONMENT}"
        expected: "production"
      - condition: "`date '+%d'`"
        expected: "re:0[1-9]"  # First 9 days of month
      - condition: "`df -h /data | awk 'NR==2 {print $5}' | sed 's/%//'`"
        expected: "re:^[0-7][0-9]$"  # Less than 80% disk usage
```

Multiple condition types and regex patterns.

<a href="/writing-workflows/control-flow#preconditions" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Repeat Until Condition

```yaml
steps:
  - name: wait-for-service
    command: curl -f http://service/health
    repeatPolicy:
      intervalSec: 10
      exitCode: [1]  # Repeat while exit code is 1
```

Repeat steps until success.

<a href="/writing-workflows/control-flow#repeat" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Repeat Until Success

```yaml
steps:
  - name: wait-for-service
    command: curl -f http://service:8080/health
    repeatPolicy:
      intervalSec: 10
      exitCode: [1]  # Repeat while curl fails
  
  - name: monitor-job
    command: ./check_job_status.sh
    output: JOB_STATUS
    repeatPolicy:
      condition: "${JOB_STATUS}"
      expected: "COMPLETED"
      intervalSec: 30
```

Wait for external dependencies and job completion.

<a href="/writing-workflows/control-flow#repeat" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Repeat Steps

```yaml
steps:
  - name: repeating task
    command: main.sh
    repeatPolicy:
      repeat: true
      intervalSec: 60
```

Execute steps periodically.

<a href="/writing-workflows/control-flow#repeat-basic" class="learn-more">Learn more →</a>

</div>

</div>

## Error Handling & Reliability

<div class="examples-grid">

<div class="example-card">

### Continue on Failure

```yaml
steps:
  - name: optional-task
    command: ./nice-to-have.sh
    continueOn:
      failure: true
  - name: required-task
    command: ./must-succeed.sh
```

Handle non-critical failures gracefully.

<a href="/writing-workflows/error-handling#continue" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Retry on Failure

```yaml
steps:
  - name: api-call
    command: curl https://api.example.com
    retryPolicy:
      limit: 3
      intervalSec: 30
```

Automatically retry failed steps.

<a href="/writing-workflows/error-handling#retry" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Smart Retry Policies

```yaml
steps:
  - name: api-with-smart-retry
    command: ./api_call.sh
    retryPolicy:
      limit: 5
      intervalSec: 30
      exitCodes: [429, 503, 504]  # Rate limit, service unavailable
```

Targeted retry policies for different error types.

<a href="/writing-workflows/error-handling#retry" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Lifecycle Handlers

```yaml
handlerOn:
  success:
    command: ./notify-success.sh
  failure:
    command: ./cleanup-on-fail.sh
  exit:
    command: ./always-cleanup.sh
steps:
  - name: main
    command: ./process.sh
```

Run handlers on workflow events.

<a href="/writing-workflows/error-handling#handlers" class="learn-more">Learn more →</a>

</div>

</div>

## Data & Variables

<div class="examples-grid">

<div class="example-card">

### Environment Variables

```yaml
env:
  - SOME_DIR: ${HOME}/batch
  - SOME_FILE: ${SOME_DIR}/some_file
  - LOG_LEVEL: debug
  - API_KEY: ${SECRET_API_KEY}
steps:
  - name: task
    dir: ${SOME_DIR}
    command: python main.py ${SOME_FILE}
```

Define variables accessible throughout the DAG.

<a href="/writing-workflows/data-variables#env" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Dotenv Files

```yaml
# Specify single dotenv file
dotenv: .env

# Or specify multiple candidate files
dotenv:
  - .env
  - .env.local
  - configs/.env.prod

steps:
  - name: use-env-vars
    command: echo "Database: ${DATABASE_URL}"
```

Load environment variables from .env files.

<a href="/writing-workflows/data-variables#dotenv" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Positional Parameters

```yaml
params: param1 param2  # Default values for $1 and $2
steps:
  - name: parameterized task
    command: python main.py $1 $2
```

Define default positional parameters.

<a href="/writing-workflows/data-variables#params" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Named Parameters

```yaml
params:
  - FOO: 1           # Default value for ${FOO}
  - BAR: "`echo 2`"  # Command substitution in defaults
  - ENVIRONMENT: dev
steps:
  - name: named params task
    command: python main.py ${FOO} ${BAR} --env=${ENVIRONMENT}
```

Define named parameters with defaults.

<a href="/writing-workflows/data-variables#named-params" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Output Variables

```yaml
steps:
  - name: get-version
    command: cat VERSION
    output: VERSION
  - name: use-version
    command: echo "Version is ${VERSION}"
    depends: get-version
```

Pass data between steps.

<a href="/writing-workflows/data-variables#output" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Output Size Limits

```yaml
# Set maximum output size to 5MB for all steps
maxOutputSize: 5242880  # 5MB in bytes

steps:
  - name: large-output
    command: "cat large-file.txt"
    output: CONTENT  # Will fail if file exceeds 5MB
```

Control output size limits to prevent memory issues.

<a href="/writing-workflows/data-variables#output-limits" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Redirect Output to Files

```yaml
steps:
  - name: redirect stdout
    command: "echo hello"
    stdout: "/tmp/hello"
  
  - name: redirect stderr
    command: "echo error message >&2"
    stderr: "/tmp/error.txt"
```

Send output to files instead of capturing.

<a href="/writing-workflows/data-variables#redirect" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### JSON Path References

```yaml
steps:
  - name: child DAG
    run: sub_workflow
    output: SUB_RESULT
  - name: use nested output
    command: echo "Result: ${SUB_RESULT.outputs.finalValue}"
    depends: child DAG
```

Access nested JSON data with path references.

<a href="/writing-workflows/data-variables#json-paths" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Step ID References

```yaml
steps:
  - name: extract customer data
    id: extract
    command: python extract.py
    output: DATA
  - name: process if valid
    command: |
      echo "Exit code: ${extract.exit_code}"
      echo "Stdout path: ${extract.stdout}"
    depends: extract
```

Reference step properties using short IDs.

<a href="/writing-workflows/data-variables#step-references" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Command Substitution

```yaml
env:
  TODAY: "`date '+%Y%m%d'`"
steps:
  - name: use date
    command: "echo hello, today is ${TODAY}"
```

Use command output in configurations.

<a href="/writing-workflows/data-variables#command-substitution" class="learn-more">Learn more →</a>

</div>

</div>

## Scripts & Code

<div class="examples-grid">

<div class="example-card">

### Shell Scripts

```yaml
steps:
  - name: script step
    script: |
      cd /tmp
      echo "hello world" > hello
      cat hello
      ls -la
```

Run shell script with default shell.

<a href="/writing-workflows/basics#scripts" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Python Scripts

```yaml
steps:
  - name: python script
    command: python
    script: |
      import os
      import datetime
      
      print(f"Current directory: {os.getcwd()}")
      print(f"Current time: {datetime.datetime.now()}")
```

Execute script with specific interpreter.

<a href="/writing-workflows/basics#scripts" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Multi-Step Scripts

```yaml
steps:
  - name: complex-task
    script: |
      #!/bin/bash
      set -e
      
      echo "Starting process..."
      ./prepare.sh
      
      echo "Running main task..."
      ./main-process.sh
      
      echo "Cleaning up..."
      ./cleanup.sh
```

Run multi-line scripts.

<a href="/writing-workflows/basics#scripts" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Working Directory

```yaml
steps:
  - name: step in specific directory
    dir: /path/to/working/directory
    command: pwd && ls -la
```

Control where each step executes.

<a href="/writing-workflows/basics#working-directory" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Shell Selection

```yaml
steps:
  - name: bash specific
    command: echo hello world | xargs echo
    shell: bash
  
  - name: with pipes
    command: echo hello world | xargs echo
```

Specify shell or use pipes in commands.

<a href="/writing-workflows/basics#shell" class="learn-more">Learn more →</a>

</div>

</div>

## Executors & Integrations

<div class="examples-grid">

<div class="example-card">

### Docker Executor

```yaml
steps:
  - name: build
    executor:
      type: docker
      config:
        image: node:18
    command: npm run build
```

Run commands in Docker containers.

<a href="/features/executors/docker" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### SSH Remote Execution

```yaml
steps:
  - name: remote
    executor:
      type: ssh
      config:
        host: server.example.com
        user: deploy
    command: ./deploy.sh
```

Execute commands on remote servers.

<a href="/features/executors/ssh" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### HTTP Requests

```yaml
steps:
  - name: webhook
    executor:
      type: http
      config:
        url: https://api.example.com/webhook
        method: POST
        headers:
          Content-Type: application/json
        body: '{"status": "started"}'
```

Make HTTP API calls.

<a href="/features/executors/http" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### JSON Processing

```yaml
steps:
  - name: get-data
    command: curl -s https://api.example.com/users
    output: API_RESPONSE
  
  - name: extract-emails
    executor:
      type: jq
      config:
        query: '.data[] | select(.active == true) | .email'
    command: echo "${API_RESPONSE}"
    output: USER_EMAILS
```

API integration with JSON processing pipeline.

<a href="/features/executors/jq" class="learn-more">Learn more →</a>

</div>

</div>

## Scheduling & Automation

<div class="examples-grid">

<div class="example-card">

### Basic Scheduling

```yaml
schedule: "5 4 * * *"  # Run at 04:05 daily
steps:
  - name: scheduled job
    command: job.sh
```

Use cron expressions to schedule DAGs.

<a href="/features/scheduling" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Skip Redundant Runs

```yaml
name: Daily Data Processing
schedule: "0 */4 * * *"    # Every 4 hours
skipIfSuccessful: true     # Skip if already succeeded
steps:
  - name: extract
    command: extract_data.sh
  - name: transform
    command: transform_data.sh
  - name: load
    command: load_data.sh
```

Prevent unnecessary executions when already successful.

<a href="/features/scheduling#skip-redundant" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Queue Management

```yaml
name: batch-job
queue: "batch"        # Assign to named queue
maxActiveRuns: 2      # Max concurrent runs
steps:
  - name: process
    command: process_data.sh
```

Control concurrent DAG execution.

<a href="/features/queues" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Global Queue Configuration

```yaml
# Global queue config in ~/.config/dagu/config.yaml
queues:
  enabled: true
  config:
    - name: "critical"
      maxConcurrency: 5
    - name: "batch"
      maxConcurrency: 1

# DAG file
queue: "critical"
maxActiveRuns: 3
steps:
  - name: critical-task
    command: ./process.sh
```

Configure queues globally and per-DAG.

<a href="/features/queues#advanced" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Email Notifications

```yaml
mailOn:
  failure: true
  success: true
smtp:
  host: smtp.gmail.com
  port: "587"
  username: "${SMTP_USER}"
  password: "${SMTP_PASS}"
steps:
  - name: critical-job
    command: ./important.sh
    mailOnError: true
```

Send email alerts on events.

<a href="/features/notifications#email" class="learn-more">Learn more →</a>

</div>

</div>

## Advanced Patterns

<div class="examples-grid">

<div class="example-card">

### Nested Workflows

```yaml
steps:
  - name: data-pipeline
    run: workflows/etl.yaml
    params: "ENV=prod DATE=today"
  - name: analyze
    run: workflows/analyze.yaml
    depends: data-pipeline
```

Compose workflows from reusable parts.

<a href="/features/executors/dag" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Multiple DAGs in One File

```yaml
name: main-workflow
steps:
  - name: process-data
    run: data-processor
    params: "TYPE=daily"

---

name: data-processor
params:
  - TYPE: "batch"
steps:
  - name: extract
    command: ./extract.sh ${TYPE}
  - name: transform
    command: ./transform.sh
```

Define multiple workflows in a single file.

<a href="/writing-workflows/advanced#multiple-dags" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Complete DAG Configuration

```yaml
name: production-etl
description: Daily ETL pipeline for analytics
schedule: "0 2 * * *"
skipIfSuccessful: true
group: DataPipelines
tags: daily,critical
queue: etl-queue
maxActiveRuns: 1
maxOutputSize: 5242880  # 5MB
env:
  - LOG_LEVEL: info
  - DATA_DIR: /data/analytics
params:
  - DATE: "`date '+%Y-%m-%d'`"
  - ENVIRONMENT: production
mailOn:
  failure: true
smtp:
  host: smtp.company.com
  port: "587"
handlerOn:
  success:
    command: echo "ETL completed successfully"
  failure:
    command: ./cleanup_on_failure.sh
  exit:
    command: ./final_cleanup.sh
steps:
  - name: validate-environment
    command: ./validate_env.sh ${ENVIRONMENT}
```

Complete DAG with all configuration options.

<a href="/reference/yaml" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Schema Definition Usage

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/dagu-org/dagu/main/schemas/dag.schema.json

name: schema-enabled-dag
description: DAG with IDE auto-completion
steps:
  - name: validated-step
    command: echo "Schema validation active"
```

Enable IDE auto-completion and validation.

<a href="/reference/yaml#schema" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Steps as Map vs Array

```yaml
# Steps as array (recommended)
steps:
  - name: step1
    command: echo "hello"
  - name: step2
    command: echo "world"
    depends: step1

---

# Steps as map (alternative)
steps:
  step1:
    command: echo "hello"
  step2:
    command: echo "world"
    depends: step1
```

Two ways to define steps.

<a href="/writing-workflows/basics#step-definition" class="learn-more">Learn more →</a>

</div>

</div>

## Real-World Examples

### Daily ETL Pipeline

```yaml
name: daily-etl-pipeline
description: Daily data processing with validation and notifications
schedule: "0 2 * * *"
skipIfSuccessful: true
queue: data-processing
maxActiveRuns: 1
params:
  - DATE: "`date +%Y-%m-%d`"
  - ENVIRONMENT: production
env:
  - DATA_SOURCE: s3://company-data/raw
  - DATA_TARGET: s3://company-data/processed
mailOn:
  failure: true
smtp:
  host: smtp.company.com
  port: "587"

steps:
  - name: validate-prerequisites
    command: |
      # Check data source availability
      aws s3 ls ${DATA_SOURCE}/${DATE}/ || exit 1
      # Check disk space
      df -h /tmp | awk 'NR==2 {print $5}' | sed 's/%//' | \
        awk '{if($1 > 80) exit 1}'
    
  - name: extract-data
    command: python extract.py --date=${DATE} --source=${DATA_SOURCE}
    output: EXTRACTION_RESULT
    depends: validate-prerequisites
    retryPolicy:
      limit: 3
      intervalSec: 300
      exitCodes: [1, 2]  # Retry on connection errors
    
  - name: validate-data-quality
    command: python validate.py ${EXTRACTION_RESULT}
    depends: extract-data
    continueOn:
      failure: false  # Stop pipeline if data quality fails
      
  - name: transform-parallel
    run: data-transformer
    parallel:
      items: [users, orders, products, analytics]
    params: "TYPE=${ITEM} INPUT=${EXTRACTION_RESULT} DATE=${DATE}"
    depends: validate-data-quality
    
  - name: load-to-warehouse
    command: |
      python load.py \
        --input=/tmp/transformed \
        --target=${DATA_TARGET}/${DATE} \
        --environment=${ENVIRONMENT}
    depends: transform-parallel
    retryPolicy:
      limit: 2
      intervalSec: 60
      
handlerOn:
  success:
    executor:
      type: http
      config:
        url: https://api.company.com/webhooks/etl-success
        method: POST
        headers:
          Content-Type: application/json
        body: '{"pipeline": "daily-etl", "date": "${DATE}", "status": "success"}'
  failure:
    executor:
      type: mail
      config:
        to: data-team@company.com
        subject: "🚨 ETL Pipeline Failed - ${DATE}"
        body: |
          The daily ETL pipeline failed on ${DATE}.
          
          Environment: ${ENVIRONMENT}
          Check logs: ${DAG_RUN_LOG_FILE}
```

Production-ready ETL pipeline with comprehensive error handling.

### CI/CD Deployment Pipeline

```yaml
name: deploy-application
description: Full CI/CD pipeline with testing, building, and deployment
params:
  - BRANCH: main
  - ENVIRONMENT: staging
  - VERSION: "`git describe --tags --always`"
env:
  - DOCKER_REGISTRY: registry.company.com
  - KUBECONFIG: /etc/kubernetes/config
queue: deployment
maxActiveRuns: 1

steps:
  - name: validate-inputs
    command: |
      # Validate branch exists
      git ls-remote --heads origin ${BRANCH} | grep -q ${BRANCH} || exit 1
      # Validate environment
      echo "${ENVIRONMENT}" | grep -E "^(staging|production)$" || exit 1
    
  - name: checkout-code
    command: |
      git fetch origin
      git checkout ${BRANCH}
      git pull origin ${BRANCH}
    depends: validate-inputs
    
  - name: run-tests
    executor:
      type: docker
      config:
        image: node:18-alpine
        host:
          binds:
            - "${PWD}:/app"
        container:
          workingDir: /app
          env:
            - NODE_ENV=test
    command: |
      npm ci
      npm run test:unit
      npm run test:integration
      npm run lint
    depends: checkout-code
    
  - name: build-application
    executor:
      type: docker
      config:
        image: node:18-alpine
        host:
          binds:
            - "${PWD}:/app"
        container:
          workingDir: /app
          env:
            - NODE_ENV=production
    command: |
      npm run build
      echo "build-${VERSION}-$(date +%s)" > build-id.txt
    output: BUILD_ID
    depends: run-tests
    
  - name: deploy-to-environment
    executor:
      type: ssh
      config:
        host: deploy.${ENVIRONMENT}.company.com
        user: deploy
        key: /home/ci/.ssh/deploy_key
    command: |
      kubectl set image deployment/myapp \
        app=${DOCKER_REGISTRY}/myapp:${BUILD_ID} \
        --namespace=${ENVIRONMENT}
      kubectl rollout status deployment/myapp --namespace=${ENVIRONMENT}
    depends: build-application
    retryPolicy:
      limit: 2
      intervalSec: 30

handlerOn:
  success:
    executor:
      type: http
      config:
        url: https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK
        method: POST
        headers:
          Content-Type: application/json
        body: |
          {
            "text": "🚀 Deployed ${BRANCH} (${VERSION}) to ${ENVIRONMENT}"
          }
  failure:
    executor:
      type: http
      config:
        url: https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK
        method: POST
        headers:
          Content-Type: application/json
        body: |
          {
            "text": "❌ Deployment failed for ${BRANCH} to ${ENVIRONMENT}"
          }
```

Complete CI/CD pipeline with Docker, Kubernetes, and notifications.

## Learn More

- [Writing Workflows Guide](/writing-workflows/) - Complete guide to building workflows
- [Feature Documentation](/features/) - Deep dive into all features
- [Configuration Reference](/reference/yaml) - Complete YAML specification