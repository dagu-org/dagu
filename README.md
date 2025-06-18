<div align="center">
  <img src="./assets/images/dagu-logo.webp" width="480" alt="Dagu Logo">
  
  <h3>Just-in-time orchestration for any workflow</h3>
  <p>A portable, zero-dependency workflow engine that runs anywhere</p>
  
  <p>
    <a href="https://github.com/dagu-org/dagu/releases"><img src="https://img.shields.io/github/release/dagu-org/dagu.svg?style=flat-square" alt="Latest Release"></a>
    <a href="https://github.com/dagu-org/dagu/actions/workflows/ci.yaml"><img src="https://img.shields.io/github/actions/workflow/status/dagu-org/dagu/ci.yaml?style=flat-square" alt="Build Status"></a>
    <a href="https://codecov.io/gh/dagu-org/dagu"><img src="https://img.shields.io/codecov/c/github/dagu-org/dagu?style=flat-square" alt="Code Coverage"></a>
    <a href="https://goreportcard.com/report/github.com/dagu-org/dagu"><img src="https://goreportcard.com/badge/github.com/dagu-org/dagu?style=flat-square" alt="Go Report Card"></a>
    <a href="https://github.com/dagu-org/dagu/blob/main/LICENSE.md"><img src="https://img.shields.io/github/license/dagu-org/dagu?style=flat-square" alt="License"></a>
    <a href="https://discord.gg/gpahPUjGRk"><img src="https://img.shields.io/discord/1115206893105889311?style=flat-square&logo=discord" alt="Discord"></a>
  </p>
  
  <p>
    <a href="https://docs.dagu.cloud">Docs</a> •
    <a href="#-quick-start">Quick Start</a> •
    <a href="#-key-features">Features</a> •
    <a href="#-installation">Installation</a> •
    <a href="https://discord.gg/gpahPUjGRk">Community</a>
  </p>
</div>

## 🎯 What is Dagu?

**Dagu is a powerful workflow engine that's simple by design.** While others require complex setups, Dagu runs as a single binary with zero dependencies. Define your workflows in straightforward YAML, and Dagu handles the orchestration, scheduling, and monitoring. [Learn the core concepts →](https://docs.dagu.cloud/getting-started/concepts)

### Why teams choose Dagu:

- **🚀 Zero Dependencies** - Single binary, no database, no message queue. Just works.
- **🛠 Language Agnostic** - Run any command: Python, Bash, Node.js, or any executable
- **📦 Portable** - Works on laptops, servers, containers, or air-gapped environments
- **🎨 Beautiful UI** - Monitor workflows in real-time with an intuitive web interface
- **⚡ Fast Setup** - From download to running workflows in under 2 minutes

## 📢 Latest Release

**[v1.17.0](https://github.com/dagu-org/dagu/releases/tag/v1.17.0) - June 17, 2025**

Major improvements including hierarchical DAG execution, enhanced UI, performance optimizations, and partial success status. [See full changelog →](https://docs.dagu.cloud/reference/changelog#v1-17-0)

## 🔑 Key Features

- **📊 DAG-based Workflows** - Define complex dependencies with simple YAML
- **⏰ [Advanced Scheduling](https://docs.dagu.cloud/features/scheduling)** - Cron expressions, timezones, multiple schedules
- **🔄 Smart Execution** - [Retries](https://docs.dagu.cloud/writing-workflows/error-handling), [conditional steps](https://docs.dagu.cloud/writing-workflows/control-flow), [parallel execution](https://docs.dagu.cloud/features/parallel-execution)
- **📝 Parameters & Variables** - Pass data between steps ([variables](https://docs.dagu.cloud/writing-workflows/data-variables)), configure workflows ([parameters](https://docs.dagu.cloud/writing-workflows/parameters))
- **🔍 Comprehensive Monitoring** - Real-time logs, execution history, status tracking

### Advanced Capabilities
- **🐳 [Docker Integration](https://docs.dagu.cloud/features/executors/docker)** - Run steps in containers with full Docker control
- **🔒 [Enterprise Security](https://docs.dagu.cloud/features/authentication)** - Authentication, TLS, permissions, API tokens
- **🛡️ [Robust Error Handling](https://docs.dagu.cloud/writing-workflows/error-handling)** - Retries, lifecycle handlers, cleanup management
- **🚦 [Flow Control](https://docs.dagu.cloud/writing-workflows/control-flow)** - Preconditions, repeat policies, continue-on conditions
- **📧 [Email Notifications](https://docs.dagu.cloud/features/email-notifications)** - SMTP integration for workflow alerts
- **🌐 Multiple Executors** - [SSH](https://docs.dagu.cloud/features/executors/ssh), [HTTP](https://docs.dagu.cloud/features/executors/http), [Mail](https://docs.dagu.cloud/features/executors/mail), [JQ](https://docs.dagu.cloud/features/executors/jq)

## 🚀 Quick Start

```bash
# Install
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash

# Create a home directory
mkdir -p ~/.config/dagu/dags

# Create your first workflow
cat > ~/.config/dagu/dags/hello-world.yaml << 'EOF'
steps:
  - name: hello
    command: echo "Hello from Dagu!"
    
  - name: world  
    command: echo "Running step 2"
EOF

# Run it
dagu start hello-world

# Check status
dagu status hello-world

# Start server
dagu start-all
```

Visit `http://localhost:8080` to see your workflow in action!

## 📦 Installation

### macOS / Linux / WSL
```bash
# Latest version
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash

# Specific version
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash -s -- --version v1.17.0

# Install to a specific directory
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash -s -- --prefix /path/to/install
```

### Homebrew
```bash
brew install dagu-org/brew/dagu
```

### Docker
```bash
docker run -d \
  --name dagu \
  -p 8080:8080 \
  -v ~/.dagu:/dagu \
  ghcr.io/dagu-org/dagu:latest dagu start-all
```

### Binary
Download from [releases page](https://github.com/dagu-org/dagu/releases) and place in your PATH.

## 📚 Documentation

- **[Getting Started Guide](https://docs.dagu.cloud/getting-started/quickstart)** - Step-by-step tutorial
- **[Core Concepts](https://docs.dagu.cloud/getting-started/concepts)** - Understand Dagu's architecture
- **[Writing Workflows](https://docs.dagu.cloud/writing-workflows/)** - Complete workflow authoring guide
- **[CLI Reference](https://docs.dagu.cloud/reference/cli)** - All command-line options
- **[API Reference](https://docs.dagu.cloud/reference/api)** - REST API documentation
- **[Configuration Guide](https://docs.dagu.cloud/reference/config)** - Server and workflow configuration
- **[YAML Specification](https://docs.dagu.cloud/reference/yaml)** - Complete workflow YAML reference

## 🎯 Examples

Find more examples in our [examples documentation](https://docs.dagu.cloud/writing-workflows/examples/).

### Data Pipeline
```yaml
name: daily-etl
schedule: "0 2 * * *"
steps:
  - name: extract
    command: python extract.py
    output: DATA_FILE
    
  - name: validate
    command: python validate.py ${DATA_FILE}
    
  - name: transform
    command: python transform.py ${DATA_FILE}
    retryPolicy:
      limit: 3
      
  - name: load
    command: python load.py ${DATA_FILE}
```

### Hierarchical DAGs

```yaml
steps:
  - name: data-pipeline
    run: etl
    params: "ENV=dev REGION=us-west-2"
    
  - name: parallel-processing
    run: batch
    parallel:
      items: ["task1", "task2", "task3"]
      maxConcurrency: 1
    params: "TASK=${ITEM}"
---
name: etl
params:
  ENV: dev
  REGION: us-west-2
steps:
  - name: etl
    command: python etl.py ${ENV} ${REGION}
---
name: batch
params:
  - TASK: task1
steps:
  - name: process
    command: python process.py ${TASK}
```

### DevOps Automation

```yaml
name: deployment
params:
  - VERSION: latest
steps:
  - name: test
    command: make test
    
  - name: build
    command: docker build -t app:${VERSION} .
    
  - name: deploy
    command: kubectl set image deployment/app app=app:${VERSION}
    preconditions:
      - condition: "`date +%u`"
        expected: "re:[1-5]"  # Weekdays only
```

## 📸 Web Interface

[Learn more about the Web UI →](https://docs.dagu.cloud/overview/web-ui)

<div align="center">
  <img src="docs/public/dashboard.png" width="720" alt="Dashboard">
  <p><i>Real-time dashboard showing all workflows and their status</i></p>
</div>

<div align="center">
  <img src="docs/public/dag-editor.png" width="720" alt="DAG Editor">
  <p><i>Visual workflow editor with validation and auto-completion</i></p>
</div>

<div align="center">
  <img src="docs/public/dag-logs.png" width="720" alt="Log Viewer">
  <p><i>Detailed logs with separate stdout/stderr streams</i></p>
</div>

## 🤝 Contributing

We welcome contributions! Whether it's bug reports, feature requests, documentation improvements, or code contributions.

- 💬 [Join our Discord](https://discord.gg/gpahPUjGRk) for discussions
- 🐛 [Report issues](https://github.com/dagu-org/dagu/issues) on GitHub
- 📖 [Read our docs](https://docs.dagu.cloud) for detailed information

### Building from Source

Prerequisites:
- Go 1.24+
- Node.js and pnpm (for web UI)
- Make

```bash
# Clone repository
git clone https://github.com/dagu-org/dagu.git && cd dagu

# Build everything
make build

# Run locally
make run
```

## 🌟 Contributors

Thanks to all our amazing contributors!

<a href="https://github.com/dagu-org/dagu/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=dagu-org/dagu" />
</a>

### Special Thanks

The v1.17.0 release was made possible by:

- [@jerry-yuan](https://github.com/jerry-yuan) - Docker optimization
- [@vnghia](https://github.com/vnghia) - Container enhancements ([#898])
- [@thefishhat](https://github.com/thefishhat) - Repeat policies & partial success ([#1011])
- [@kriyanshii](https://github.com/kriyanshii) - Queue functionality
- [@ghansham](https://github.com/ghansham) - Reviews & feedback

## 📄 License

Dagu is open source under the [GNU GPLv3](./LICENSE.md).

---

<div align="center">
  <p>⭐ If you find Dagu useful, please star the repository!</p>
  <p>Built with ❤️ by the Dagu community</p>
</div>

[#898]: https://github.com/dagu-org/dagu/issues/898
[#1011]: https://github.com/dagu-org/dagu/issues/1011
