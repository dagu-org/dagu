<div align="center">
  <img src="./assets/images/dagu-logo.webp" width="480" alt="Dagu Logo">
  <br/>

  <p>
    <a href="https://docs.dagu.cloud/reference/changelog"><img src="https://img.shields.io/github/release/dagu-org/dagu.svg?style=flat-square" alt="Latest Release"></a>
    <a href="https://github.com/dagu-org/dagu/actions/workflows/ci.yaml"><img src="https://img.shields.io/github/actions/workflow/status/dagu-org/dagu/ci.yaml?style=flat-square" alt="Build Status"></a>
    <a href="https://discord.gg/gpahPUjGRk"><img src="https://img.shields.io/discord/1095289480774172772?style=flat-square&logo=discord" alt="Discord"></a>
    <a href="https://bsky.app/profile/dagu-org.bsky.social"><img src="https://img.shields.io/badge/Bluesky-0285FF?style=flat-square&logo=bluesky&logoColor=white" alt="Bluesky"></a>
  </p>
  
  <p>
    <a href="https://dagu.cloud/">Website</a> |
    <a href="https://docs.dagu.cloud/writing-workflows/examples">Examples</a> |
    <a href="https://discord.gg/gpahPUjGRk">Discord</a>
  </p>
</div>

## What is Dagu?

Dagu is a lightweight workflow engine with a modern Web UI. Workflows are defined in a simple, declarative YAML format and can be executed on schedule. It supports shell commands, remote execution via SSH, and Docker images. Built-in error notifications and logging out of the box.

For a quick feel of how it works, take a look at the [examples](https://docs.dagu.cloud/writing-workflows/examples).

### Motivation

Legacy systems often have complex and implicit dependencies between jobs. When there are hundreds of cron jobs on a server, it can be difficult to keep track of these dependencies and to determine which job to rerun if one fails. It can also be a hassle to SSH into a server to view logs and manually rerun shell scripts one by one. Dagu aims to solve these problems by allowing you to explicitly visualize and manage pipeline dependencies as a DAG, and by providing a web UI for checking dependencies, execution status, and logs and for rerunning or stopping jobs with a simple mouse click.

### Why Not Use an Existing Workflow Scheduler Like Airflow?

There are many existing tools such as Airflow, but many of these require you to write code in a programming language like Python to define your DAG. For systems that have been in operation for a long time, there may already be complex jobs with hundreds of thousands of lines of code written in languages like Perl or Shell Script. Adding another layer of complexity on top of these codes can reduce maintainability. Dagu was designed to be easy to use, self-contained, and require no coding, making it ideal for small projects.

### How it Works
Dagu executes your workflows defined in a simple, declarative YAML format.

For example, a simple sequential DAG:
```yaml
schedule: "0 0 * * *" # Runs at 00:00 everyday

steps:
  - echo "Hello, dagu!"
  - echo "This is a second step"
```

## Highlights
- Install by placing a single binary with zero dependency
- Run without DBMS or any cloud service
- Define workflows (DAG) in a declarative YAML format
- Use existing programs, scripts, and container images without any modification
- Schedule workflows with cron expressions
- Run in HA mode and distribute tasks to workers through built-in queue system.
- Modularize workflows by nesting them with sub DAGs (and inspect each sub-run in the UI).

### CLI Preview
![Demo CLI](./assets/images/demo-cli.webp)

### Web UI Preview
![Demo Web UI](./assets/images/demo-web-ui.webp)

## Table of Contents

- Quick Start
  - [Quick Start](#quick-start)
  - [Docker-Compose](#docker-compose)
- Documentation
  - [Documentation](#documentation)
  - [Environment Variables](#environment-variables)
  - [Architecture](#architecture)
  - [Roadmap](#roadmap)
- Development & Contributing
  - [Development](#development)
  - [Discussion](#discussion)
  - [Contributing](#contributing)
  - [Acknowledgements](#acknowledgements)
- [License](#license)

## Quick Start

### 1. Install dagu

**macOS/Linux**:

```bash
# Install to ~/.local/bin (default, no sudo required)
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash

# Install specific version
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash -s -- --version v1.17.0

# Install to custom directory
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash -s -- --install-dir /usr/local/bin

# Install to custom directory with custom working directory
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash -s -- --install-dir /usr/local/bin --working-dir /var/tmp
```

**Windows (PowerShell)**:

```powershell
# Install latest version to default location (%LOCALAPPDATA%\Programs\dagu)
irm https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.ps1 | iex

# Install specific version
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.ps1))) v1.24.0

# Install to custom directory
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.ps1))) latest "C:\tools\dagu"
```

**Windows (CMD/PowerShell)**:

```cmd
REM Install latest version to default location (%LOCALAPPDATA%\Programs\dagu)
curl -fsSL https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.cmd -o installer.cmd && .\installer.cmd && del installer.cmd

REM Install specific version
curl -fsSL https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.cmd -o installer.cmd && .\installer.cmd v1.24.0 && del installer.cmd

REM Install to custom directory
curl -fsSL https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.cmd -o installer.cmd && .\installer.cmd latest "C:\tools\dagu" && del installer.cmd
```

**Docker**:

```bash
docker run --rm \
  -v ~/.dagu:/var/lib/dagu \
  -p 8080:8080 \
  ghcr.io/dagu-org/dagu:latest \
  dagu start-all
```

Note: see [documentation](https://docs.dagu.cloud/getting-started/installation) for other methods.

**Homebrew**:

```bash
brew update && brew install dagu

# Upgrade to latest version
brew update && brew upgrade dagu
```

**npm**:
```bash
# Install via npm
npm install -g --ignore-scripts=false @dagu-org/dagu
```

### 2. Create your first workflow

> **Note**: When you first start Dagu with an empty DAGs directory, it automatically creates example workflows to help you get started. To skip this, set `DAGU_SKIP_EXAMPLES=true`.

```bash
cat > ./hello.yaml << 'EOF'
steps:
  - echo "Hello from Dagu!"
  - echo "Running step 2"
EOF
```

### 3. Run the workflow

```bash
dagu start hello.yaml
```

### 4. Check the status and view logs

```bash
dagu status hello
```

### 5. Explore the Web UI

```bash
dagu start-all
```

Visit http://localhost:8080

## Docker-Compose

Clone the repository and run with Docker Compose:

```bash
git clone https://github.com/dagu-org/dagu.git
cd dagu
```

Run with minimal setup:

```bash
docker compose -f deploy/docker/compose.minimal.yaml up -d
# Visit http://localhost:8080
```

Stop containers:

```bash
docker compose -f deploy/docker/compose.minimal.yaml down
```

You can also use the production-like configuration `deploy/docker/compose.prod.yaml` with OpenTelemetry, Prometheus, and Grafana:

```bash
docker compose -f deploy/docker/compose.prod.yaml up -d
# Visit UI at http://localhost:8080
# Jaeger at http://localhost:16686, Prometheus at http://localhost:9090, Grafana at http://localhost:3000
```

Note: It's just for demonstration purposes. For production, please customize the configuration as needed.

Looking for Kubernetes? See the example manifests in `deploy/k8s/README.md`.

## Documentation

Full documentation is available at [docs.dagu.cloud](https://docs.dagu.cloud/).

**Helpful Links**:

- [Feature by Examples](https://docs.dagu.cloud/writing-workflows/examples) - Explore useful features with examples
- [Remote Execution via SSH](https://docs.dagu.cloud/features/executors/ssh#ssh-executor) - Run commands on remote machines using SSH
- [Distributed Execution](https://docs.dagu.cloud/features/distributed-execution) - How to run workflows across multiple machines
- [Scheduling](https://docs.dagu.cloud/features/scheduling) - Learn about flexible scheduling options (start, stop, restart) with cron syntax
- [Authentication](https://docs.dagu.cloud/configurations/authentication) - Configure authentication for the Web UI
- [Configuration](https://docs.dagu.cloud/configurations/reference) - Detailed configuration options for customizing Dagu

## Changelog

For a detailed list of changes, bug fixes, and new features, please refer to the [Changelog](https://docs.dagu.cloud/reference/changelog).

## Environment Variables

**Note:** Configuration precedence: Command-line flags > Environment variables > Configuration file

### Frontend Server Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `DAGU_HOST` | `127.0.0.1` | Web UI server host |
| `DAGU_PORT` | `8080` | Web UI server port |
| `DAGU_BASE_PATH` | - | Base path for reverse proxy setup |
| `DAGU_API_BASE_URL` | `/api/v2` | API endpoint base path |
| `DAGU_TZ` | - | Server timezone (e.g., `Asia/Tokyo`) |
| `DAGU_DEBUG` | `false` | Enable debug mode |
| `DAGU_LOG_FORMAT` | `text` | Log format (`text` or `json`) |
| `DAGU_HEADLESS` | `false` | Run without Web UI |
| `DAGU_LATEST_STATUS_TODAY` | `false` | Show only today's latest status |
| `DAGU_WORK_DIR` | - | Default working directory for DAGs |
| `DAGU_DEFAULT_SHELL` | - | Default shell for command execution |
| `DAGU_CERT_FILE` | - | TLS certificate file path |
| `DAGU_KEY_FILE` | - | TLS key file path |

### Path Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `DAGU_HOME` | - | Base directory that overrides all path configurations |
| `DAGU_DAGS_DIR` | `~/.config/dagu/dags` | Directory for DAG definitions |
| `DAGU_LOG_DIR` | `~/.local/share/dagu/logs` | Directory for log files |
| `DAGU_DATA_DIR` | `~/.local/share/dagu/data` | Directory for application data |
| `DAGU_SUSPEND_FLAGS_DIR` | `~/.local/share/dagu/suspend` | Directory for suspend flags |
| `DAGU_ADMIN_LOG_DIR` | `~/.local/share/dagu/logs/admin` | Directory for admin logs |
| `DAGU_BASE_CONFIG` | `~/.config/dagu/base.yaml` | Path to base configuration file |
| `DAGU_EXECUTABLE` | - | Path to dagu executable |
| `DAGU_DAG_RUNS_DIR` | `{dataDir}/dag-runs` | Directory for DAG run data |
| `DAGU_PROC_DIR` | `{dataDir}/proc` | Directory for process data |
| `DAGU_QUEUE_DIR` | `{dataDir}/queue` | Directory for queue data |
| `DAGU_SERVICE_REGISTRY_DIR` | `{dataDir}/service-registry` | Directory for service registry |

### Authentication

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `DAGU_AUTH_BASIC_USERNAME` | - | Basic auth username |
| `DAGU_AUTH_BASIC_PASSWORD` | - | Basic auth password |
| `DAGU_AUTH_OIDC_CLIENT_ID` | - | OIDC client ID |
| `DAGU_AUTH_OIDC_CLIENT_SECRET` | - | OIDC client secret |
| `DAGU_AUTH_OIDC_CLIENT_URL` | - | OIDC client URL |
| `DAGU_AUTH_OIDC_ISSUER` | - | OIDC issuer URL |
| `DAGU_AUTH_OIDC_SCOPES` | - | OIDC scopes (comma-separated) |
| `DAGU_AUTH_OIDC_WHITELIST` | - | OIDC email whitelist (comma-separated) |

### UI Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `DAGU_UI_NAVBAR_COLOR` | `#1976d2` | UI header color (hex or name) |
| `DAGU_UI_NAVBAR_TITLE` | `Dagu` | UI header title |
| `DAGU_UI_LOG_ENCODING_CHARSET` | `utf-8` | Log file encoding |
| `DAGU_UI_MAX_DASHBOARD_PAGE_LIMIT` | `100` | Maximum items on dashboard |
| `DAGU_UI_DAGS_SORT_FIELD` | `name` | Default DAGs sort field |
| `DAGU_UI_DAGS_SORT_ORDER` | `asc` | Default DAGs sort order |

### Scheduler Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `DAGU_SCHEDULER_PORT` | `8090` | Health check server port |
| `DAGU_SCHEDULER_LOCK_STALE_THRESHOLD` | `30s` | Scheduler lock stale threshold |
| `DAGU_SCHEDULER_LOCK_RETRY_INTERVAL` | `5s` | Lock retry interval |
| `DAGU_SCHEDULER_ZOMBIE_DETECTION_INTERVAL` | `45s` | Zombie DAG detection interval (0 to disable) |
| `DAGU_QUEUE_ENABLED` | `true` | Enable queue system |

### Worker Configuration

This configuration is used for worker instances that execute DAGs. See the [Distributed Execution](https://docs.dagu.cloud/features/distributed-execution) documentation for more details.

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `DAGU_COORDINATOR_HOST` | `127.0.0.1` | Coordinator gRPC server bind address |
| `DAGU_COORDINATOR_ADVERTISE` | (auto) | Address to advertise in service registry (default: hostname) |
| `DAGU_COORDINATOR_PORT` | `50055` | Coordinator gRPC server port |
| `DAGU_WORKER_ID` | - | Worker instance ID |
| `DAGU_WORKER_MAX_ACTIVE_RUNS` | `100` | Maximum concurrent runs per worker |
| `DAGU_WORKER_LABELS` | - | Worker labels (format: `key1=value1,key2=value2`, e.g., `gpu=true,memory=64G`) |
| `DAGU_SCHEDULER_PORT` | `8090` | Scheduler health check server port |
| `DAGU_SCHEDULER_LOCK_STALE_THRESHOLD` | `30s` | Time after which scheduler lock is considered stale |
| `DAGU_SCHEDULER_LOCK_RETRY_INTERVAL` | `5s` | Interval between lock acquisition attempts |

### Peer Configuration

This configuration is used for communication between coordinator services and other services (e.g., scheduler, worker, web UI). See the [Distributed Execution](https://docs.dagu.cloud/features/distributed-execution) documentation for more details.

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `DAGU_PEER_CERT_FILE` | - | Peer TLS certificate file |
| `DAGU_PEER_KEY_FILE` | - | Peer TLS key file |
| `DAGU_PEER_CLIENT_CA_FILE` | - | Peer CA certificate file for client verification |
| `DAGU_PEER_SKIP_TLS_VERIFY` | `false` | Skip TLS certificate verification for peer connections |
| `DAGU_PEER_INSECURE` | `true` | Use insecure connection (h2c) instead of TLS |

## Development

### Building from Source

#### Prerequisites
- [Go 1.25+](https://go.dev/doc/install)
- [Node.js](https://nodejs.org/en/download/)
- [pnpm](https://pnpm.io/installation)

#### 1. Clone the repository and build server

```bash
git clone https://github.com/dagu-org/dagu.git && cd dagu
make
```

This will start the dagu server at http://localhost:8080.

#### 2. Run the frontend development server

```bash
cd ui
pnpm install
pnpm dev
```

Navigate to http://localhost:8081 to view the frontend.

### Running Tests

To ensure the integrity of the go code, you can run all Go unit and integration tests.

Run all tests from the project root directory:

```bash
make test
```

To run tests with code coverage analysis:

```bash
make make-coverage
```


## Feature Matrix

Dagu is designed to be a comprehensive solution for workflow orchestration. Below is a detailed list of currently supported capabilities.

| Category                    | Capability                      | Description                                                              | Link |
| --------------------------- | ------------------------------- | ------------------------------------------------------------------------ | ---- |
| **Core Execution**          | Local execution                 | Run workflows locally with CLI / Web UI / API                           | <a href="https://docs.dagu.cloud/overview/cli">CLI</a>, <a href="https://docs.dagu.cloud/overview/web-ui">Web UI</a> |
|                             | Queue based execution           | Dispatch DAG execution to workers with labels and priorities            | <a href="https://docs.dagu.cloud/features/queues">Queues</a> |
|                             | Immediate execution             | Disable queue for immediate execution                                    | <a href="https://docs.dagu.cloud/overview/cli">CLI</a> |
|                             | Idempotency                     | Prevent duplicate DAG execution with same DAG-run ID                |  <a href="https://docs.dagu.cloud/reference/cli#status">`start` command</a> |
|                             | Status management               | queued → running → succeeded/partially_succeeded/failed/aborted              | <a href="http://localhost:5173/getting-started/concepts#status-management">Status Management</a> |
|                             | Cancel propagation              | Cancel signals to sub-DAG                      | |
|                             | Cleanup hooks                   | Define cleanup processing with onExit handlers                           | <a href="https://docs.dagu.cloud/getting-started/concepts#lifecycle-handlers">Lifecycle Handlers</a> |
|                             | Status hooks                    | Define hooks on success / failure / cancel                         |  <a href="https://docs.dagu.cloud/getting-started/concepts#lifecycle-handlers">Lifecycle Handlers</a> |
| **Definition & DSL**        | Declarative YAML DSL            | Validation with JSON Schema, display error locations                     | <a href="https://docs.dagu.cloud/reference/yaml">YAML Specification</a> |
|                             | Environment variables           | Environment variables at DAG and step level, support dotenv      | <a href="https://docs.dagu.cloud/writing-workflows/data-variables#environment-variables">Environment Variables</a> |
|                             | Command substitution            | Use command output as value for variables or parameters                  | <a href="https://docs.dagu.cloud/reference/variables#command-substitution">Command Substitution</a> |
|                             | Shell support                   | Use shell features like pipes, redirects, globbing, etc. | <a href="https://docs.dagu.cloud/features/executors/shell">Shell Executor</a> |
|                             | Script support                  | Use scripts in Python, Bash, etc. as steps                                 | <a href="https://docs.dagu.cloud/writing-workflows/examples#scripts-code">Script Execution</a> |
|                             | Modular DAGs                   | Reusable DAGs with params                                            | <a href="https://docs.dagu.cloud/writing-workflows/#base-configuration">Base Configuration</a> |
|                             | Secrets management              | Reference-only secrets via KMS/Vault/OIDC                                | <a href="https://docs.dagu.io/writing-workflows/secrets">Secrets</a> |
| **Control Structures**      | Fan-out/Fan-in                  | Native parallel branches + join                                          | <a href="https://docs.dagu.cloud/writing-workflows/control-flow#parallel-execution">Parallel Execution</a> |
|                             | Iteration (loop)                | Iteration over list values                                               | <a href="https://docs.dagu.cloud/writing-workflows/control-flow#parallel-execution">Parallel Execution</a> |
|                             | Conditional routes              | Data/expression based routing                                            | <a href="https://docs.dagu.cloud/writing-workflows/control-flow#conditional-execution">Conditional Execution</a> |
|                             | Sub-DAG call                    | Reusable sub-DAG                                                        | <a href="https://docs.dagu.cloud/features/execution-control#parallel-execution">Parallel Execution</a> |
|                             | Worker & Dispatch               | Runs DAG on different nodes with selector conditions                     | <a href="https://docs.dagu.cloud/features/distributed-execution">Distributed Execution</a> |
|                             | Retry policies                  | Retry with backoff/interval                                              | <a href="https://docs.dagu.cloud/writing-workflows/error-handling#retry-policies">Retry Policies</a> |
|                             | Repeat Policies                 | Repeat step until condition is met                                       | <a href="https://docs.dagu.cloud/writing-workflows/control-flow#repetition">Repeat Policies</a> |
|                             | Timeout management              | DAG Execution Timeouts                                                   | <a href="https://docs.dagu.cloud/features/execution-control#workflow-timeout">Workflow Timeout</a> |
| **Triggers & Scheduling**   | Cron expression                 | Schedule to start / stop / restart                                       | <a href="https://docs.dagu.cloud/features/scheduling">Scheduling</a> |
|                             | Multiple schedules              | Multiple schedules per DAG                                              | <a href="https://docs.dagu.cloud/features/scheduling#multiple-schedules">Multiple Schedules</a> |
|                             | Timezone support                | Per-DAG timezone for cron schedules                                 | <a href="https://docs.dagu.cloud/features/scheduling#timezone-support">Timezone Support</a> |
|                             | Skip                            | Skip an execution when a previous manual run was successful                 | <a href="https://docs.dagu.cloud/features/scheduling#skip-redundant-runs">Skip Redundant Runs</a> |
|                             | Zombie detection                | Automatic detection for processes terminated unexpectedly                | <a href="https://docs.dagu.cloud/features/scheduling">Scheduling</a> |
|                             | Trigger via Web API             | Web API to start DAG executions                                               | <a href="https://docs.dagu.cloud/overview/api">Web API</a> |
| **Container Native**        | Step-level container config     | Run steps in Docker containers with granular control                     | <a href="https://docs.dagu.cloud/features/executors/docker">Docker Executor</a> |
|                             | DAG level container config      | Run all steps in a container with shared volumes and env vars            | <a href="https://docs.dagu.cloud/features/executors/docker#container-field">Container Field</a> |
|                             | Authorized registry access      | Access private registries with credentials                                | <a href="https://docs.dagu.cloud/features/executors/docker#registry-authentication">Registry Auth</a> |
| **Data & Artifacts**        | Passing data between steps      | Passing ephemeral data between steps in a DAG                           | <a href="https://docs.dagu.cloud/features/data-flow">Data Flow</a> |
|                             | Secret redaction                | Auto-mask secrets in logs/events                                         | <a href="https://docs.dagu.io/writing-workflows/secrets">Secrets</a> |
|                             | Automatic log cleanup           | Automatic log cleanup based on retention policies                        | <a href="https://docs.dagu.cloud/configurations/operations#log-cleanup">Log Retention</a> |
| **Observability**           | Logging with live streaming     | Structured JSON logs with live tail streaming                            | <a href="https://docs.dagu.cloud/overview/web-ui#log-viewer">Log Viewer</a> |
|                             | Metrics                         | Prometheus metrics                                                       | <a href="https://docs.dagu.cloud/configurations/reference#metrics">Metrics</a> |
|                             | OpenTelemetry                   | Distributed tracing with OpenTelemetry                                    | <a href="https://docs.dagu.cloud/features/opentelemetry">OpenTelemetry</a> |
|                             | DAG Visualization               | DAG / Gantt charts for critical path analysis                            | <a href="https://docs.dagu.cloud/overview/web-ui#dag-visualization">DAG Visualization</a> |
|                             | Email notification              | Email notification on success / failure with the log file attachment      | <a href="https://docs.dagu.cloud/features/email-notifications">Email Notifications</a> |
|                             | Health monitoring               | Health check for scheduler & failover                                   | <a href="https://docs.dagu.cloud/configurations/reference#health-check">Health Check</a> |
|                             | Nested-DAG visualization        | Nested DAG visualization with drill down functionality                  | <a href="https://docs.dagu.cloud/overview/web-ui#nested-dag-visualization">Nested DAG Visualization</a> |
| **Security & Governance**   | Secret injection                | Vault/KMS/OIDC ref-only; short-lived tokens                              | <a href="https://docs.dagu.cloud/writing-workflows/secrets">Secrets</a> |
|                             | Authentication                  | Basic auth / OIDC support for Web UI and API                             | <a href="https://docs.dagu.cloud/configurations/authentication">Authentication</a> |
|                             | HA (High availability) mode     | Control-plane with failover for scheduler / Web UI / Coordinator         | <a href="https://docs.dagu.cloud/features/scheduling#high-availability">High Availability</a> |
| **Executor types**          | `jq`                            | JSON processing with jq queries                                          | <a href="https://docs.dagu.cloud/features/executors/jq">JQ Executor</a> |
|                             | `ssh`                           | Remote command execution via SSH                                         | <a href="https://docs.dagu.cloud/features/executors/ssh">SSH Executor</a> |
|                             | `docker`                        | Container-based task execution                                           | <a href="https://docs.dagu.cloud/features/executors/docker">Docker Executor</a> |
|                             | `http`                          | HTTP/REST API calls with retry                                           | <a href="https://docs.dagu.cloud/features/executors/http">HTTP Executor</a> |
|                             | `mail`                          | Send emails with template                                                | <a href="https://docs.dagu.cloud/features/executors/mail">Mail Executor</a> |
|                             | `archive`                       | Archive/unarchive operations (zip, tar, etc.)                            | <a href="https://docs.dagu.cloud/features/executors/archive">Archive Executor</a> |
| **DevX & Testing**          | Local development               | offline runs                                                       | <a href="https://docs.dagu.cloud/overview/cli">CLI Usage</a> |
|                             | Dry-run                         | DAG level Dry-run                                                        | <a href="https://docs.dagu.cloud/reference/cli#dry">`dry` command</a> |
| **UI & Operations**         | Run / retry / cancel operations | Start / enqueue / retry / stop                                                         | <a href="https://docs.dagu.cloud/overview/web-ui#dag-operations">DAG Operations</a> |
|                             | Automatic parameter forms       | Auto-generate parameter forms for DAGs                             | <a href="https://docs.dagu.cloud/overview/web-ui">Web UI</a> |
|                             | DAG definition search           | Filter by tag / name                                                     | <a href="https://docs.dagu.cloud/overview/web-ui#search">DAG Search</a> |
|                             | Execution history search        | Filter by status / date-range / name                                     | <a href="https://docs.dagu.cloud/overview/web-ui#history">History Search</a> |
|                             | Step-level operations           | Rerun, resume from step                                             | <a href="https://docs.dagu.cloud/overview/web-ui">Web UI</a> |
|                             | Parameter override              | Override parameters for a DAG run                                 | |
|                             | Scheduled DAG management        | Enable/disable schedule for a DAG                                   | <a href="https://docs.dagu.cloud/overview/web-ui">Web UI</a> |
|                             | UI organization                 | Logical DAG grouping                                               | <a href="https://docs.dagu.cloud/overview/web-ui#dag-organization">DAG Organization</a> |
|                             | Windows support                 | Windows support                                                     |  |

## Roadmap

Our roadmap is driven by community feedback and our mission to make Dagu the most developer-friendly workflow engine.

> 💡 **Have an idea?** [Open a Feature Request](https://github.com/dagu-org/dagu/issues/new?template=feature_request.md) or join the [Discord](https://discord.gg/gpahPUjGRk) to discuss!

### 🏗️ In Progress
*Active development focus. These features are expected to land in the upcoming releases.*

- [ ] **Windows Support**: Native support for Windows OS to ensure Dagu runs seamlessly across all major platforms.
- [ ] **JSON Schema Validation** ([#325](https://github.com/dagu-org/dagu/issues/325)): Strict validation for DAG parameters to catch configuration errors early.
- [ ] **Resource Limits**: Configurable CPU and Memory limits per step to prevent resource exhaustion.

### 📅 Planned (Short-term)
*High-priority features selected for the near future.*

**Core Engine**
- [ ] **Human-in-the-loop** ([#978](https://github.com/dagu-org/dagu/issues/978)): Support for manual approval steps (blocking wait) within a workflow.
- [ ] **File Watcher Trigger** ([#372](https://github.com/dagu-org/dagu/issues/372)): Event-driven triggers that start DAGs when specific files change.
- [ ] **Inter-DAG State**: Mechanisms to explicitly share state and data artifacts between different DAG runs.
- [ ] **External Database Backend** ([#539](https://github.com/dagu-org/dagu/issues/539)): Support for PostgreSQL/MySQL persistence for high-scale deployments (removing reliance on local disk).

**Developer Experience**
- [ ] **Edit & Retry** ([#326](https://github.com/dagu-org/dagu/issues/326), [#1058](https://github.com/dagu-org/dagu/issues/1058)): Capability to hot-fix a DAG definition and immediately retry a failed run from the UI.
- [ ] **Single Step Execution** ([#1047](https://github.com/dagu-org/dagu/issues/1047)): Ability to trigger a specific step in isolation for testing and debugging purposes.

**Operations**
- [ ] **Pause/Resume DAGs**: Global or per-DAG controls to pause scheduling without modifying YAML files.
- [ ] **Project / Namespace**: logical grouping of DAGs and execution history per project/team.

### 🔮 Future (Long-term)
*Strategic goals and complex features under consideration.*

- [ ] **Kubernetes Native Execution** ([#837](https://github.com/dagu-org/dagu/issues/837)): Run steps as native K8s jobs/pods for massive scalability.
- [ ] **Catch Up & Backfill** ([#695](https://github.com/dagu-org/dagu/issues/695)): Robust handling of missed schedule runs and historical data processing.
- [ ] **RBAC & Multi-User Auth**: Fine-grained permissions, user roles, and resource quotas.
- [ ] **Audit Logging**: Immutable records of all user actions and system events.
- [ ] **SDK & Plugins** ([#583](https://github.com/dagu-org/dagu/issues/583)): Python/Go/TS SDKs for programmatic DAG generation and a plugin system for custom executors.

## Discussion

For discussions, support, and sharing ideas, join our community on [Discord](https://discord.gg/gpahPUjGRk).

## Recent Updates

Changelog of recent updates can be found in the [Changelog](https://docs.dagu.cloud/reference/changelog) section of the documentation.

## Contributing

We welcome contributions of all kinds! Whether you're a developer, a designer, or a user, your help is valued. Here are a few ways to get involved:

- Star the project on GitHub.
- Suggest new features by opening an issue.
- Join the discussion on our Discord server.
- Contribute code: Check out our issues you can help with.

For more details, see our [Contribution Guide](./CONTRIBUTING.md).

## Acknowledgements

### Contributors

<a href="https://github.com/dagu-org/dagu/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=dagu-org/dagu" />
</a>

Thanks to all the contributors who have helped make Dagu better! Your contributions, whether through code, documentation, or feedback, are invaluable to the project.

### Sponsors & Supporters

<a href="https://github.com/disizmj"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fdisizmj.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@disizmj"></a>
<a href="https://github.com/Arvintian"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2FArvintian.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@Arvintian"></a>
<a href="https://github.com/yurivish"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fyurivish.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@yurivish"></a>
<a href="https://github.com/jayjoshi64"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fjayjoshi64.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@jayjoshi64"></a>

Thanks for supporting Dagu’s development! Join our supporters: [GitHub Sponsors](https://github.com/sponsors/dagu-org)

## License

GNU GPLv3 - See [LICENSE](./LICENSE)
