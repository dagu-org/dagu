# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Essential Commands

**Build and Development:**
- `make build` - Build both UI and binary (equivalent to `make ui bin`)
- `make ui` - Build frontend assets (required before running server)
- `make bin` - Build Go binary to `.local/bin/dagu`
- `make run` - Start server and scheduler with hot reload (requires UI built first)
- `make test` - Run all tests with gotestsum
- `make lint` - Run golangci-lint with auto-fixes
- `make golangci-lint` - Run golangci-lint to check for lint issues

**Testing and Quality:**
- `make test-coverage` - Run tests with coverage report
- `make open-coverage` - Open coverage report in browser
- `gotestsum --format=standard-quiet -- -v --race ./...` - Run tests directly

**Frontend Development:**
- Frontend is in `/ui` directory using React/TypeScript
- `cd ui && pnpm install` - Install frontend dependencies  
- `cd ui && pnpm typecheck` - TypeScript checking
- `cd ui && pnpm lint` - ESLint with fixes
- TypeScript types are auto-generated from API schemas in `ui/src/api/v2/schema.ts`

**API Generation:**
- `make api` - Generate API server code from OpenAPI specs (MUST run after modifying API yaml files)
- `make apiv1` - Generate API v1 server code
- API specs are in `/api/v1/api.yaml` and `/api/v2/api.yaml`
- Generated code goes to `api/v1/api.gen.go` and `api/v2/api.gen.go`

## Architecture Overview

**Core Components:**
- **Workflow Engine**: Built around DAG (Directed Acyclic Graph) execution defined in YAML
- **Scheduler**: Cron-based scheduling for workflow execution
- **Web UI**: React-based frontend for monitoring and management
- **REST API**: Two versions (v1, v2) for programmatic access
- **Executors**: Pluggable command execution (shell, Docker, HTTP, SSH, mail, jq)

**Key Packages:**
- `internal/digraph/` - Core DAG definition, parsing, and execution logic
- `internal/scheduler/` - Cron scheduling and job management
- `internal/frontend/` - Web server and API endpoints
  - `internal/frontend/api/v1/` - API v1 handler implementations
  - `internal/frontend/api/v2/` - API v2 handlers and transformers
- `internal/persistence/` - Local storage for DAG runs, processes, and queues
- `internal/agent/` - DAG execution agent and process management
- `internal/cmd/` - CLI command implementations
- `internal/models/` - Core data structures and models

**Data Flow:**
1. YAML DAG definitions are parsed into `digraph.DAG` structs
2. Scheduler queues DAG runs based on cron expressions
3. Agent executes steps using appropriate executors
4. Results stored in local file-based persistence layer
5. Web UI displays real-time status via REST API

**Configuration:**
- Default config location: `~/.config/dagu/`
- DAG files stored in `~/.config/dagu/dags/` by default
- Base configuration can be shared via `~/.config/dagu/base.yaml`
- Environment variables prefixed with `DAGU_` for configuration

**Frontend Build Process:**
- UI assets must be built with `make ui` before running server
- Webpack bundles are copied to `internal/frontend/assets/`
- Go binary embeds frontend assets at build time

## Development Workflow

**When modifying API schemas:**
1. Edit the yaml schema files (`/api/v1/api.yaml` or `/api/v2/api.yaml`)
2. Run `make api` to regenerate API code
3. Update corresponding transformer/handler functions
4. Update UI components if needed
5. Run `make test` and `make golangci-lint` to verify
6. Run `go fmt` to format Go code

**Before committing:**
- Run `make golangci-lint` to check for lint issues
- Run `make test` to ensure all tests pass
- Run `make ui` to ensure frontend builds without errors
- Use pnpm for frontend package management

## Go Code Style Guidelines
- **Use `any` instead of `interface{}`** - Since Go 1.18, prefer `any` for empty interfaces
- Follow the linter suggestions from `make golangci-lint`
- Use descriptive variable and function names
- Keep functions focused and concise
- **Follow existing patterns exactly**
- **Line length: 88 chars maximum**

## Testing Requirements
- Test edge cases and errors
- New features require tests
- Bug fixes require regression tests
- Use `th.RunCommand(t, cmd.CmdStatus(), test.CmdTest{...})` pattern for consistency

## Git Commit Guidelines
- Keep commit messages to one line unless body is absolutely necessary
- **NEVER EVER use `git add -A` or `git add .`** - ALWAYS stage specific files only
- **CRITICAL: Using `git add -A` is FORBIDDEN. Always use `git add <specific-file>`**
- Follow conventional commit format (fix:, feat:, docs:, etc.)
- For commits fixing bugs or adding features based on user reports add:
  ```
  git commit --trailer "Reported-by:<name>"
  ```
  Where `<name>` is the name of the user
- For commits related to a Github issue, add:
  ```
  git commit --trailer "Github-Issue:#<number>"
  ```
- **NEVER mention co-authored-by or similar aspects**
- **NEVER mention the tool used to create the commit message or PR**
- **NEVER ever include *Generated with* or similar in commit messages**
- **NEVER ever include *Co-Authored-By* or similar in commit messages**

## 🎯 What is Dagu?

Dagu is a **modern, powerful, yet surprisingly simple workflow orchestration engine** that runs as a single binary with zero external dependencies. Born from the frustration of managing hundreds of legacy cron jobs scattered across multiple servers, Dagu brings clarity, visibility, and control to workflow automation.

**The Game Changer**: Unlike traditional workflow engines, Dagu introduces **hierarchical DAG composition** - the ability to nest workflows within workflows to unlimited depth. This transforms how you build and maintain complex systems, enabling true modularity and reusability at scale.

## 🚀 Core Philosophy & Design Principles

### 1. **Local-First Architecture**
- Single binary installation - no databases, message brokers, or external services required
- Works offline and in air-gapped environments
- Sensitive data and workflows stay on your infrastructure
- File-based storage for maximum portability
- Unix socket-based process communication

### 2. **Minimal Configuration**
- Start with just one YAML file
- No complex setup or infrastructure requirements
- Works out of the box with sensible defaults
- Can be running in minutes, not hours or days
- JSON Schema support for IDE auto-completion

### 3. **Language Agnostic**
- Execute ANY command: Python, Bash, Node.js, Go, Rust, or any executable
- No need to learn a new programming language or SDK
- Use your existing scripts and tools as-is
- Perfect for heterogeneous environments
- Shell selection (sh, bash, custom shells, nix-shell with packages)

### 4. **Developer-Friendly**
- Clear, human-readable YAML syntax
- Intuitive web UI with real-time monitoring
- Comprehensive logging with stdout/stderr separation
- Fast onboarding for team members
- Template rendering with Sprig functions

### 5. **Production-Ready**
- Battle-tested in enterprise environments
- Robust error handling and retry mechanisms
- Built-in monitoring and alerting
- Scalable from single workflows to thousands
- Graceful shutdown with configurable cleanup timeouts

## 💪 Comprehensive Features & Capabilities

### 🔄 **Advanced DAG Execution & Control**
- **Directed Acyclic Graphs (DAGs)**: Define complex workflows with dependencies
- **Parallel Execution**: Run multiple steps concurrently with `maxActiveSteps` control
- **Concurrent DAG Runs**: Control parallel runs with `maxActiveRuns`
- **Conditional Execution**: Steps run based on preconditions with:
  - Command exit codes
  - Environment variable checks
  - Command output matching (exact or regex patterns)
  - Command substitution evaluation
- **Dynamic Workflows**: 
  - Pass outputs between steps using `output` field
  - JSON path references for nested data (`${VAR.path.to.value}`)
  - Environment variable expansion
  - Command substitution with backticks
- **🚀 Hierarchical DAG Composition** (Revolutionary Feature!):
  - **Multi-level nesting**: Parent → Child → Grandchild → ... (unlimited depth)
  - **Full hierarchy tracking**: Root, parent, and child relationships maintained
  - **Parameter inheritance**: Pass parameters down the hierarchy chain
  - **Output bubbling**: Access child DAG outputs in parent workflows
  - **Isolated execution**: Each level runs in its own process
  - **Reusable components**: Build a library of composable workflow modules
  - **Dynamic composition**: Conditionally execute different sub-workflows
- **Step Dependencies**: Define complex dependency graphs between steps

### ⏰ **Sophisticated Scheduling**
- **Cron-based Scheduling**: Standard cron expressions with timezone support
- **Multiple Schedules**: Define arrays of schedule times
- **Start/Stop/Restart Schedules**: Control long-running processes:
  - `start`: When to start the DAG
  - `stop`: When to send stop signals
  - `restart`: When to restart the DAG
- **Skip Redundant Runs**: `skipIfSuccessful` prevents duplicate executions
- **Restart Wait Time**: Configurable delay before restart
- **Schedule-based Preconditions**: Run only on specific days/times

### 🔧 **Powerful Executors**
- **Shell Executor**: Run any command with shell selection:
  - Default shell (`$SHELL` or `sh`)
  - Custom shells (bash, zsh, etc.)
  - nix-shell with package management
- **Docker Executor**: Full container control:
  - Create new containers or exec into existing ones
  - Volume mounts, environment variables, networking
  - Custom entrypoints and working directories
  - Platform selection and image pull policies
- **HTTP Executor**: Advanced API interactions:
  - All HTTP methods with custom headers
  - Query parameters and request bodies
  - Timeout control and silent mode
- **SSH Executor**: Remote command execution:
  - Key-based authentication
  - Custom ports and users
- **Mail Executor**: Email automation:
  - SMTP configuration
  - Multiple recipients
  - File attachments
- **JQ Executor**: JSON processing and transformation

### 🔁 **Advanced Flow Control**
- **Retry Policies**: 
  - Configurable retry limits and intervals
  - Exit code-based retry triggers
  - Exponential backoff support
- **Repeat Policies**:
  - Repeat indefinitely with intervals
  - Conditional repeats based on:
    - Command output matching
    - Exit codes
    - Command evaluation results
- **Continue On Conditions**:
  - Continue on failure or skipped steps
  - Continue based on specific exit codes
  - Continue based on output patterns (regex supported)
  - `markSuccess` to override step status
- **Lifecycle Hooks** (`handlerOn`):
  - `onSuccess`: Execute when DAG succeeds
  - `onFailure`: Execute when DAG fails
  - `onCancel`: Execute when DAG is cancelled
  - `onExit`: Always execute on DAG completion

### 📊 **Enterprise-Grade Features**
- **Queue Management**: 
  - Enqueue DAG runs with priorities
  - Dequeue by name or DAG run ID
  - Queue inspection and management
- **History Retention**: Configurable retention days for execution history
- **Timeout Management**:
  - DAG-level timeout (`timeout`)
  - Step-level cleanup timeout
  - Maximum cleanup time (`maxCleanUpTime`)
- **Delay Controls**:
  - Initial delay before DAG start
  - Inter-step delays
- **Signal Handling**: Custom stop signals per step (`signalOnStop`)
- **Working Directory Control**: Per-step directory configuration

### 🎨 **Modern Web UI**
- **Real-time Dashboard**: 
  - Status metrics with filtering
  - Timeline visualization
  - Date-range filtering
  - DAG-specific views
- **Interactive DAG Editor**: 
  - Edit workflows directly in browser
  - Syntax highlighting
  - Real-time validation
- **Visual Graph Display**: 
  - Horizontal/vertical orientations
  - Real-time status updates
  - Node status indicators
- **Execution History**: 
  - Advanced filtering by date and status
  - Execution timeline views
  - Performance metrics
- **Log Viewer**: 
  - Real-time log streaming
  - Separate stdout/stderr views
  - Log search capabilities
- **Advanced Search**: Find DAGs by name, tags, or content
- **Remote Node Support**: Manage workflows across multiple environments

### 🔒 **Security & Configuration**
- **Authentication**:
  - Basic authentication (username/password)
  - API token authentication
  - TLS/HTTPS support with cert/key files
- **Permissions**:
  - `writeDAGs`: Control DAG creation/editing/deletion
  - `runDAGs`: Control DAG execution
  - API access control
  - UI permission enforcement
- **Configuration Methods**:
  - Environment variables (DAGU_* prefix)
  - Configuration file (`~/.config/dagu/config.yaml`)
  - Base configuration inheritance
  - Per-DAG overrides
  - Command-line arguments
- **Global Settings**:
  - Debug mode toggle
  - Log format (json/text)
  - Timezone configuration
  - Working directory defaults
  - Headless mode for automation
- **Path Configuration**:
  - DAGs directory
  - Log directory
  - Data/history directory
  - Suspend flags directory
  - Admin logs directory
  - Queue directory
  - Process directory
- **UI Customization**:
  - Navbar color and title
  - Log encoding charset
  - Dashboard page limits
  - Latest status display options

### 🛠️ **Variable & Parameter Management**
- **Parameter Types**:
  - Positional parameters (`$1`, `$2`, etc.)
  - Named parameters (`${NAME}`)
  - Map-based parameters
  - Command-line overrides
- **Variable Features**:
  - Environment variable expansion
  - Command substitution with backticks
  - JSON path references (`${VAR.nested.field}`)
  - Default values with overrides
- **Special Variables**:
  - `DAG_NAME`: Current DAG name
  - `DAG_RUN_ID`: Unique execution ID
  - `DAG_RUN_LOG_FILE`: Log file path
  - `DAG_RUN_STEP_NAME`: Current step name
  - `DAG_RUN_STEP_STDOUT_FILE`: Step stdout path
  - `DAG_RUN_STEP_STDERR_FILE`: Step stderr path
- **Template Support**:
  - Sprig template functions
  - Custom template functions
  - Variable interpolation

### 📈 **Operational Excellence**
- **Monitoring & Metrics**:
  - Execution time tracking
  - Resource usage monitoring
  - Performance dashboards
  - Status aggregation
- **Log Management**:
  - Configurable retention policies
  - Log rotation
  - Centralized logging
  - Log file attachments in emails
- **Process Management**:
  - Graceful shutdown
  - Process group management
  - Signal propagation
  - Cleanup timeouts
- **Error Handling**:
  - Detailed error messages
  - Error propagation control
  - Recovery mechanisms
  - Notification on errors

## 🎯 Real-World Use Cases

### Data Engineering
- **ETL Pipelines**: Extract, transform, and load data with dependencies
- **Data Processing**: Batch processing with parallel execution
- **Data Quality Checks**: Conditional validation steps
- **Report Generation**: Scheduled analytics and reporting

### DevOps & Infrastructure
- **CI/CD Automation**: Build, test, and deploy workflows
- **Infrastructure Provisioning**: Orchestrate Terraform, Ansible, etc.
- **Backup & Recovery**: Scheduled backups with notifications
- **System Maintenance**: Automated cleanup and optimization tasks

### Business Automation
- **Employee Onboarding/Offboarding**: Multi-step HR processes
- **Financial Processing**: Scheduled reports and reconciliation
- **Customer Data Sync**: Integration between systems
- **Compliance Workflows**: Automated checks and reporting

### AI/ML Operations
- **Model Training Pipelines**: Orchestrate training workflows
- **Data Preparation**: Preprocessing and feature engineering
- **Model Deployment**: Automated deployment pipelines
- **Experiment Tracking**: Scheduled model evaluations

## 🏆 Why Dagu Stands Out

### Vs. Airflow
- **No Python Required**: Use any language or tool
- **Zero Infrastructure**: No database, message broker, or webserver setup
- **Instant Start**: Running in minutes vs. hours of configuration
- **Lightweight**: Single binary vs. complex distributed system

### Vs. Cron
- **Visual Monitoring**: See all jobs in one place
- **Dependency Management**: Define relationships between tasks
- **Error Handling**: Built-in retries and notifications
- **Logging**: Centralized logs with search capabilities

### Vs. GitHub Actions / CI Tools
- **Local Execution**: Run on your infrastructure
- **No Vendor Lock-in**: Portable YAML format
- **Fine-grained Control**: Advanced scheduling and execution options
- **Cost Effective**: No usage-based pricing

## 🔥 Advanced Capabilities & Examples

### Dynamic Workflows with Variable Passing
```yaml
steps:
  - name: get config
    command: echo '{"env": "prod", "replicas": 3, "region": "us-east-1"}'
    output: CONFIG
  
  - name: get secrets
    command: vault read -format=json secret/app
    output: SECRETS
  
  - name: deploy
    command: |
      kubectl set env deployment/app \
        REGION=${CONFIG.region} \
        API_KEY=${SECRETS.data.api_key}
      kubectl scale --replicas=${CONFIG.replicas} deployment/app
    depends: [get config, get secrets]
```

### Advanced Conditional Execution
```yaml
steps:
  - name: check prerequisites
    command: check_system.sh
    output: CHECK_RESULT
  
  - name: process data
    command: process.py
    preconditions:
      - condition: "${CHECK_RESULT}"
        expected: "ready"
      - condition: "`df -h / | awk 'NR==2 {print $5}' | sed 's/%//'`"
        expected: "re:^[0-7][0-9]$"  # Less than 80% disk usage
      - condition: "`date +%u`"
        expected: "re:[1-5]"  # Weekdays only
  
  - name: handle errors gracefully
    command: cleanup.sh
    continueOn:
      exitCode: [1, 2, 3]  # Expected exit codes
      output:
        - "no data found"
        - "re:WARNING:.*"
        - "re:SKIP:.*"
      markSuccess: true
```

### Sophisticated Retry and Repeat Patterns
```yaml
steps:
  - name: api call with retry
    command: curl -f https://api.example.com/data
    retryPolicy:
      limit: 5
      intervalSec: 30
      exitCode: [429, 503]  # Retry only on rate limit or service unavailable
  
  - name: wait for service
    command: nc -z localhost 8080
    repeatPolicy:
      exitCode: [1]  # Repeat while connection fails
      intervalSec: 10
  
  - name: monitor until complete
    command: check_job_status.sh
    output: JOB_STATUS
    repeatPolicy:
      condition: "${JOB_STATUS}"
      expected: "COMPLETED"
      intervalSec: 60
```

### Complex DAG Composition
```yaml
name: data-pipeline
schedule: "0 2 * * *"
skipIfSuccessful: true
maxActiveRuns: 1
maxActiveSteps: 5

handlerOn:
  success:
    command: notify.sh "Pipeline completed successfully"
  failure:
    executor:
      type: mail
      config:
        to: oncall@company.com
        subject: "Pipeline Failed: ${DAG_NAME}"
        message: "Check logs at ${DAG_RUN_LOG_FILE}"
  exit:
    command: cleanup_temp_files.sh

steps:
  - name: validate inputs
    command: validate.py
    preconditions:
      - test -f /data/input.csv
      - test -s /data/input.csv
  
  - name: process in parallel
    depends: validate inputs
    run: etl/transform
    params: "INPUT=/data/input.csv PARALLEL=true"
    output: TRANSFORM_RESULT
  
  - name: quality check
    command: quality_check.py
    depends: process in parallel
    continueOn:
      failure: true
    mailOnError: true
```

### Advanced Scheduling Patterns
```yaml
# Multiple schedules with timezone
schedule:
  - "CRON_TZ=America/New_York 0 9 * * MON-FRI"  # 9 AM ET on weekdays
  - "CRON_TZ=Europe/London 0 14 * * MON-FRI"    # 2 PM GMT on weekdays

# Start/stop pattern for resource management
schedule:
  start: 
    - "0 8 * * MON-FRI"   # Start at 8 AM on weekdays
  stop: 
    - "0 18 * * MON-FRI"  # Stop at 6 PM on weekdays
  restart:
    - "0 12 * * MON-FRI"  # Restart at noon

restartWaitSec: 60  # Wait 1 minute before restart
```

### 🌟 The Power of Nested DAGs - Build Modular, Reusable Workflows

Nested DAGs are not just a feature - they're a paradigm shift in workflow design. Instead of monolithic workflows, you can build a library of reusable components that compose into complex systems.

```yaml
# etl/extract.yaml - Reusable extraction module
name: extract-module
params:
  - SOURCE: s3://bucket/data
  - FORMAT: parquet
steps:
  - name: validate source
    command: aws s3 ls ${SOURCE}
    continueOn:
      failure: false
  
  - name: extract data
    command: extract.py --source=${SOURCE} --format=${FORMAT}
    output: EXTRACTED_PATH
    depends: validate source

# etl/transform.yaml - Reusable transformation module
name: transform-module  
params:
  - INPUT_PATH: /tmp/input
  - RULES: default
steps:
  - name: apply transformations
    command: transform.py --input=${INPUT_PATH} --rules=${RULES}
    output: TRANSFORMED_PATH

# etl/master-pipeline.yaml - Compose modules into complete pipeline
name: data-pipeline
schedule: "0 2 * * *"
steps:
  - name: extract customer data
    run: etl/extract
    params: "SOURCE=s3://data/customers FORMAT=csv"
    output: CUSTOMER_DATA
  
  - name: extract product data
    run: etl/extract  
    params: "SOURCE=s3://data/products FORMAT=json"
    output: PRODUCT_DATA
  
  - name: transform customer data
    run: etl/transform
    params: "INPUT_PATH=${CUSTOMER_DATA.outputs.EXTRACTED_PATH} RULES=customer_rules"
    output: TRANSFORMED_CUSTOMERS
    depends: extract customer data
  
  - name: transform product data
    run: etl/transform
    params: "INPUT_PATH=${PRODUCT_DATA.outputs.EXTRACTED_PATH} RULES=product_rules"  
    output: TRANSFORMED_PRODUCTS
    depends: extract product data
  
  - name: join datasets
    command: |
      join_data.py \
        --customers=${TRANSFORMED_CUSTOMERS.outputs.TRANSFORMED_PATH} \
        --products=${TRANSFORMED_PRODUCTS.outputs.TRANSFORMED_PATH}
    depends:
      - transform customer data
      - transform product data
```

**Why This Changes Everything:**
- **Module Library**: Build once, use everywhere
- **Team Collaboration**: Different teams maintain different modules
- **Testing**: Test modules independently
- **Versioning**: Version control individual components
- **Dynamic Pipelines**: Choose modules based on conditions
- **Error Isolation**: Failures contained within module boundaries
- **Parallel Development**: Teams work on modules simultaneously

**Real-World Example: Multi-Environment Deployment**
```yaml
# deploy/base.yaml - Base deployment module
name: deployment-base
params:
  - APP_NAME: myapp
  - VERSION: latest
steps:
  - name: health check
    command: check_health.sh ${APP_NAME}
  - name: deploy
    command: deploy.sh ${APP_NAME} ${VERSION}
    depends: health check

# master-deployment.yaml - Environment-specific orchestration
name: multi-env-deployment
steps:
  - name: deploy to dev
    run: deploy/base
    params: "APP_NAME=myapp-dev VERSION=${VERSION}"
    output: DEV_RESULT
  
  - name: run integration tests
    command: test_integration.sh dev
    depends: deploy to dev
    output: TEST_RESULT
  
  - name: deploy to staging
    run: deploy/base
    params: "APP_NAME=myapp-staging VERSION=${VERSION}"
    depends: run integration tests
    preconditions:
      - condition: "${TEST_RESULT}"
        expected: "PASSED"
  
  - name: deploy to production
    run: deploy/base
    params: "APP_NAME=myapp-prod VERSION=${VERSION}"
    depends: deploy to staging
    preconditions:
      - condition: "`date +%u`"
        expected: "re:[1-5]"  # Weekdays only
```

### Docker Integration with Advanced Options
```yaml
steps:
  - name: run in container
    executor:
      type: docker
      config:
        image: python:3.11-slim
        pull: missing
        autoRemove: true
        platform: linux/amd64
        host:
          binds:
            - /data:/data:ro
            - /output:/output:rw
        container:
          env:
            - PYTHONPATH=/app
            - ENV=${ENV}
          workingDir: /app
    command: python process.py
  
  - name: exec in existing container
    executor:
      type: docker
      config:
        containerName: my-app
        exec:
          user: appuser
          workingDir: /app/data
    command: ./update_cache.sh
```

### Template Rendering with Sprig Functions
```yaml
env:
  - TIMESTAMP: "`date +%Y%m%d_%H%M%S`"
  - ENVIRONMENT: production

steps:
  - name: generate config
    command: |
      cat > config.json << EOF
      {
        "timestamp": "${TIMESTAMP}",
        "environment": "${ENVIRONMENT}",
        "hostname": "{{ .HOSTNAME | default "localhost" }}",
        "date": "{{ now | date "2006-01-02" }}",
        "random_id": "{{ uuidv4 }}"
      }
      EOF
    output: CONFIG_FILE
```

## 📝 Complete YAML Configuration Reference

### DAG-Level Configuration
```yaml
# Metadata
name: my-workflow                    # DAG name (defaults to filename)
description: "Data processing pipeline"
group: ETL                          # Group for UI organization
tags: [daily, critical]             # Tags for filtering/search

# Scheduling
schedule: "0 2 * * *"               # Cron expression
skipIfSuccessful: true              # Skip if already succeeded
restartWaitSec: 60                  # Wait before restart

# Execution Control
maxActiveRuns: 1                    # Max concurrent DAG runs
maxActiveSteps: 10                  # Max parallel steps
timeout: 3600                       # DAG timeout in seconds
delay: 10                           # Initial delay in seconds
maxCleanUpTime: 300                 # Cleanup timeout in seconds

# Data Management
histRetentionDays: 30               # History retention
logDir: /custom/logs                # Custom log directory

# Environment
dotenv:                             # Load env files
  - .env
  - .env.production
env:                                # Environment variables
  - ENVIRONMENT: production
  - API_KEY: "`vault read -field=key secret/api`"

# Notifications
mailOn:
  success: true
  failure: true
smtp:
  host: smtp.gmail.com
  port: "587"
  username: ${SMTP_USER}
  password: ${SMTP_PASS}
errorMail:
  from: alerts@company.com
  to: oncall@company.com
  prefix: "[ALERT]"
  attachLogs: true

# Lifecycle Handlers
handlerOn:
  success:
    command: notify.sh success
  failure:
    command: alert.sh failure
  cancel:
    command: cleanup.sh cancelled
  exit:
    command: finalize.sh

# Parameters
params:                             # Default parameters
  - ENVIRONMENT: dev
  - BATCH_SIZE: 100
  - PROCESS_DATE: "`date +%Y-%m-%d`"

# Preconditions
preconditions:                      # DAG-level checks
  - condition: "`date +%u`"
    expected: "re:[1-5]"            # Weekdays only
```

### Step Configuration
```yaml
steps:
  - name: process-data
    description: "Process daily data batch"
    
    # Execution
    command: process.py             # Command to run
    shell: bash                     # Shell to use
    script: |                       # Inline script
      echo "Processing..."
      python process.py --batch
    dir: /app/data                  # Working directory
    
    # Dependencies
    depends:                        # Step dependencies
      - validate-input
      - check-resources
    
    # Input/Output
    output: RESULT                  # Capture output
    stdout: /logs/process.out       # Redirect stdout
    stderr: /logs/process.err       # Redirect stderr
    
    # Conditions
    preconditions:
      - condition: test -f input.csv
      - condition: "${READY}"
        expected: "true"
    
    # Error Handling
    continueOn:
      failure: true
      skipped: true
      exitCode: [0, 1, 2]
      output: ["WARNING", "SKIP"]
      markSuccess: true
    
    # Retry/Repeat
    retryPolicy:
      limit: 3
      intervalSec: 60
      exitCode: [1, 255]
    
    repeatPolicy:
      intervalSec: 300
      condition: "${STATUS}"
      expected: "PENDING"
    
    # Notifications
    mailOnError: true
    
    # Process Control
    signalOnStop: SIGTERM
    
    # Executors
    executor:
      type: docker
      config:
        image: python:3.11
```

## 🌟 The Dagu Advantage

### Architectural Excellence
- **Zero Dependencies**: Single binary, no database, no message broker
- **File-Based Storage**: Portable, version-controllable, easy backup
- **Unix Philosophy**: Do one thing well, compose with other tools
- **Language Agnostic**: Use any tool, any language, any framework

### Operational Benefits
- **Instant Setup**: Running in seconds, not hours
- **Low Resource Usage**: Minimal CPU and memory footprint
- **High Reliability**: Simple architecture means fewer failure points
- **Easy Debugging**: Clear logs, simple process model

### Developer Experience
- **Intuitive YAML**: Human-readable, self-documenting
- **Rich CLI**: Full control from command line
- **Modern UI**: Beautiful, responsive web interface
- **Great Documentation**: Comprehensive guides and examples

## 📋 Complete Server Configuration

### Configuration File Example (`~/.config/dagu/config.yaml`)
```yaml
# Server Configuration
host: 127.0.0.1              # Server host
port: 8080                   # Server port
basePath: /dagu              # Base path for reverse proxy
debug: false                 # Debug mode
logFormat: json              # Log format (json/text)
tz: America/New_York         # Timezone
headless: false              # Headless mode

# Permissions
permissions:
  writeDAGs: true            # Allow DAG creation/editing
  runDAGs: true              # Allow DAG execution

# Authentication
auth:
  basic:
    enabled: true
    username: admin
    password: secure123
  token:
    enabled: true
    value: your-api-token

# TLS Configuration
tls:
  certFile: /path/to/cert.pem
  keyFile: /path/to/key.pem

# Path Configuration
paths:
  dagsDir: ~/.config/dagu/dags
  logDir: ~/.local/share/dagu/logs
  dataDir: ~/.local/share/dagu/history
  suspendFlagsDir: ~/.config/dagu/suspend
  adminLogsDir: ~/.local/share/admin
  baseConfig: ~/.config/dagu/base.yaml

# UI Configuration
ui:
  navbarColor: "#1976d2"
  navbarTitle: "Dagu Production"
  logEncodingCharset: utf-8
  maxDashboardPageLimit: 100

# Remote Nodes
remoteNodes:
  - name: staging
    apiBaseURL: https://staging.example.com/api/v1
    isBasicAuth: true
    basicAuthUsername: admin
    basicAuthPassword: password
  - name: production
    apiBaseURL: https://prod.example.com/api/v1
    isAuthToken: true
    authToken: prod-token
    skipTLSVerify: false
```

## 🎬 Getting Started is Dead Simple

```bash
# Install (macOS, Linux, Windows WSL)
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash

# Or via Homebrew
brew install dagu-org/brew/dagu

# Start server and scheduler
dagu start-all

# Or with custom options
dagu start-all --host=0.0.0.0 --port=9000 --dags=/path/to/dags

# Create your first DAG
cat > hello.yaml << EOF
steps:
  - name: hello
    command: echo "Hello from Dagu!"
  - name: world
    command: echo "Welcome to simple, powerful workflows!"
    depends: hello
EOF

# Run it
dagu start hello.yaml

# Run with parameters
dagu start hello.yaml -- NAME=World TYPE=awesome

# Check status
dagu status hello.yaml

# Retry a failed run
dagu retry --run-id=<run-id> hello.yaml

# Stop a running DAG
dagu stop hello.yaml

# Restart a DAG
dagu restart hello.yaml

# Dry run (test without executing)
dagu dry hello.yaml

# Queue management
dagu enqueue --run-id=custom-id hello.yaml
dagu dequeue hello.yaml

# View in browser
open http://localhost:8080
```

## 🚀 Why Teams Choose Dagu

### For Developers
- **No Learning Curve**: If you can write YAML, you can use Dagu
- **Use Your Tools**: No need to rewrite scripts in Python/Go
- **Local Development**: Test workflows on your laptop
- **Version Control**: DAGs are just text files

### For Operations
- **Easy Deployment**: Single binary, systemd/Docker ready
- **Low Maintenance**: No database to manage
- **Resource Efficient**: Run on minimal hardware
- **Secure by Default**: No external dependencies

### For Business
- **Fast Time-to-Value**: Implement workflows in minutes
- **No Vendor Lock-in**: Open source, standard formats
- **Cost Effective**: No licensing, minimal infrastructure
- **Proven Reliability**: Used in production worldwide

## 🌈 The Future of Workflow Orchestration

Dagu proves that powerful doesn't mean complex. While other tools require teams of engineers to maintain, Dagu empowers individual developers to build robust automation. It's the missing link between simple cron jobs and heavyweight orchestration platforms.

In a world of over-engineered solutions, Dagu stands out by doing less, but doing it perfectly. It's not trying to be everything to everyone - it's focused on being the best tool for running workflows reliably and simply.

**Join the movement. Simplify your workflows. Choose Dagu.**
