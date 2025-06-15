# Examples

Quick reference for all Dagu features. Each example is minimal and copy-paste ready.

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

### Parallel Execution with Run

```yaml
steps:
  - name: process-files
    run: process-file
    parallel:
      items: [file1.csv, file2.csv, file3.csv]
      maxConcurrent: 2
```

Process multiple items simultaneously using sub-DAGs.

<a href="/features/execution-control#parallel" class="learn-more">Learn more →</a>

</div>

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

### Retry on Failure

```yaml
steps:
  - name: api-call
    command: curl https://api.example.com
    retryPolicy:
      limit: 3
      intervalSec: 30
      exitCode: [1, 255]
```

Automatically retry failed steps.

<a href="/writing-workflows/error-handling#retry" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Docker Executor

```yaml
steps:
  - name: build
    executor:
      type: docker
      config:
        image: node:18
        autoRemove: true
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
        key: ~/.ssh/id_rsa
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
        timeout: 30
```

Make HTTP API calls.

<a href="/features/executors/http" class="learn-more">Learn more →</a>

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
  username: ${SMTP_USER}
  password: ${SMTP_PASS}
steps:
  - name: critical-job
    command: ./important.sh
    mailOnError: true
```

Send email alerts on events.

<a href="/features/notifications#email" class="learn-more">Learn more →</a>

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

### Scheduled Workflows

```yaml
schedule: "0 2 * * *"  # 2 AM daily
skipIfSuccessful: true
steps:
  - name: backup
    command: ./backup.sh
```

Run workflows on a schedule, skip if already succeeded.

<a href="/features/scheduling" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Nested Workflows

```yaml
steps:
  - name: data-pipeline
    run: etl.yaml
    params: "ENV=prod DATE=today"
  - name: analyze
    run: analyze.yaml
    depends: data-pipeline
```

Compose workflows from reusable parts.

<a href="/features/executors/dag" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Repeat Until Condition

```yaml
steps:
  - name: wait-for-service
    command: curl -f http://service/health
    repeatPolicy:
      intervalSec: 10
      condition: "curl -f http://service/health"
      expected: "0"  # Exit code 0
```

Repeat steps until condition is met.

<a href="/writing-workflows/control-flow#repeat" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Continue on Failure

```yaml
steps:
  - name: optional-task
    command: ./nice-to-have.sh
    continueOn:
      failure: true
      exitCode: [1, 2]
  - name: required-task
    command: ./must-succeed.sh
```

Handle non-critical failures gracefully.

<a href="/writing-workflows/error-handling#continue" class="learn-more">Learn more →</a>

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

<div class="example-card">

### Environment Variables

```yaml
env:
  - LOG_LEVEL: debug
  - API_KEY: ${SECRET_API_KEY}
  - TIMESTAMP: "`date +%Y%m%d`"
steps:
  - name: api-call
    command: ./call-api.sh
```

Configure workflow environment.

<a href="/writing-workflows/data-variables#env" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### JSON Processing

```yaml
steps:
  - name: get-data
    command: curl https://api.example.com/data
    output: API_RESPONSE
  - name: extract
    executor:
      type: jq
      config:
        query: .users | map(.email)
    command: echo "$API_RESPONSE"
    output: EMAILS
```

Process JSON data with jq.

<a href="/features/executors/jq" class="learn-more">Learn more →</a>

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

### Parameter Substitution

```yaml
params:
  - ENVIRONMENT: dev
  - VERSION: latest
steps:
  - name: deploy
    command: |
      ./deploy.sh \
        --env=${ENVIRONMENT} \
        --version=${VERSION}
```

Pass parameters at runtime.

<a href="/writing-workflows/data-variables#params" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Queue Management

```yaml
queue: critical-jobs
maxActiveRuns: 1
steps:
  - name: exclusive-task
    command: ./critical-process.sh
```

Control workflow concurrency with queues.

<a href="/features/queues" class="learn-more">Learn more →</a>

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

### nix-shell with Packages

```yaml
steps:
  - name: python-analysis
    shell: nix-shell
    shellPackages: [python311, pandas, numpy]
    command: python analyze.py
```

Use nix-shell with specific packages.

<a href="/features/executors/shell#nix-shell" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Signal Handling

```yaml
steps:
  - name: graceful-process
    command: ./long-running-service.sh
    signalOnStop: SIGTERM
```

Control how processes are stopped.

<a href="/writing-workflows/error-handling#signals" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Advanced Repeat Patterns

```yaml
steps:
  - name: monitor
    command: check_status.sh
    output: STATUS
    repeatPolicy:
      intervalSec: 60
      condition: "${STATUS}"
      expected: "COMPLETED"
```

Repeat based on output conditions.

<a href="/writing-workflows/control-flow#repeat-patterns" class="learn-more">Learn more →</a>

</div>

<div class="example-card">

### Output Size Control

```yaml
maxOutputSize: 10485760  # 10MB
steps:
  - name: large-output
    command: ./generate-report.sh
    output: REPORT_DATA
```

Control maximum output capture size.

<a href="/reference/yaml#output-size" class="learn-more">Learn more →</a>

</div>

</div>

## Real-World Examples

### Data Pipeline with Error Handling

```yaml
name: daily-etl-pipeline
schedule: "0 2 * * *"
skipIfSuccessful: true
params:
  - DATE: "`date +%Y-%m-%d`"
timeoutSec: 7200  # 2 hours
maxOutputSize: 5242880  # 5MB

steps:
  - name: extract
    command: python extract.py --date=${DATE}
    output: RAW_DATA
    retryPolicy:
      limit: 3
      intervalSec: 300
    
  - name: validate
    command: python validate.py ${RAW_DATA}
    depends: extract
    continueOn:
      failure: false
      
  - name: transform-parallel
    run: transform-worker
    parallel:
      items: [users, orders, products]
      maxConcurrent: 2
    params: "INPUT=${RAW_DATA}"
    depends: validate
    
  - name: load
    command: python load.py --date=${DATE}
    depends: transform-parallel
    retryPolicy:
      limit: 3
      intervalSec: 60
      exitCode: [1, 255]
      
handlerOn:
  failure:
    executor:
      type: mail
      config:
        to: data-team@example.com
        subject: "ETL Pipeline Failed - ${DATE}"
        attachLogs: true
  success:
    command: |
      echo "Pipeline completed successfully for ${DATE}"
      ./notify-dashboard.sh --status=success --date=${DATE}
```

### CI/CD Pipeline with Conditional Deployment

```yaml
name: deploy-application
params:
  - BRANCH: main
  - ENVIRONMENT: staging

steps:
  - name: checkout
    command: git checkout ${BRANCH}
    
  - name: test
    executor:
      type: docker
      config:
        image: node:18
        autoRemove: true
        host:
          binds:
            - ./:/app:ro
        container:
          workingDir: /app
    command: npm test
    depends: checkout
    
  - name: build
    executor:
      type: docker
      config:
        image: node:18
        autoRemove: true
    command: npm run build
    depends: test
    output: BUILD_ID
    
  - name: deploy
    executor:
      type: ssh
      config:
        host: ${ENVIRONMENT}.example.com
        user: deploy
        key: /home/deploy/.ssh/id_rsa
    command: |
      ./deploy.sh --build=${BUILD_ID}
    depends: build
    preconditions:
      - condition: "${ENVIRONMENT}"
        expected: "re:staging|production"
```

### Parallel Processing with Dynamic Items

```yaml
name: batch-processor
steps:
  - name: get-files
    command: find /data -name "*.csv" -mtime -1
    output: FILES
    
  - name: process-batch
    run: process-single-file
    parallel:
      items: ${FILES}
      maxConcurrent: 5
    output: RESULTS
    
  - name: aggregate
    command: |
      echo "${RESULTS}" | jq -s 'map(.summary) | add'
    output: TOTAL_SUMMARY
    depends: process-batch
    
  - name: report
    executor:
      type: mail
      config:
        to: reports@example.com
        subject: "Batch Processing Complete"
        message: |
          Processing complete.
          Summary: ${TOTAL_SUMMARY}
```

### Advanced Queue Management

```yaml
name: resource-intensive-job
queue: heavy-compute
maxActiveRuns: 1  # Only one instance at a time
histRetentionDays: 7

steps:
  - name: check-resources
    command: ./check-resources.sh
    output: RESOURCES_OK
    preconditions:
      - condition: "${RESOURCES_OK}"
        expected: "true"
    
  - name: process
    command: ./heavy-computation.sh
    depends: check-resources
    timeoutSec: 3600
    continueOn:
      exitCode: [143]  # SIGTERM is ok
      markSuccess: true
```

### Hierarchical DAG Composition

```yaml
name: multi-region-deployment
params:
  - VERSION: v1.2.0

steps:
  - name: build-artifacts
    run: ci/build
    params: "VERSION=${VERSION}"
    output: ARTIFACTS
    
  - name: deploy-us-east
    run: deploy/regional
    params: |
      REGION=us-east-1
      ARTIFACTS=${ARTIFACTS.outputs.location}
      VERSION=${VERSION}
    depends: build-artifacts
    
  - name: deploy-eu-west
    run: deploy/regional
    params: |
      REGION=eu-west-1
      ARTIFACTS=${ARTIFACTS.outputs.location}
      VERSION=${VERSION}
    depends: build-artifacts
    
  - name: verify-deployments
    run: deploy/verify
    params: "VERSION=${VERSION}"
    depends:
      - deploy-us-east
      - deploy-eu-west
```

## Learn More

- [Writing Workflows Guide](/writing-workflows/) - Complete guide to building workflows
- [Feature Documentation](/features/) - Deep dive into all features
- [Configuration Reference](/reference/yaml) - Complete YAML specification
- [Best Practices](/guides/best-practices) - Tips for production use
