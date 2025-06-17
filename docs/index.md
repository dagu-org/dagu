---
layout: doc
---

<div class="logo-section">
  <div class="logo-container">
    <img src="/logo-light.svg" alt="Dagu Logo" class="logo-light logo-icon">
    <img src="/dagu.webp" alt="Dagu Logo" class="logo-dark logo-icon">
    <span class="logo-text">Dagu</span>
  </div>
</div>

<div class="tagline">
  <h1>A self-contained, powerful alternative to Airflow, Cron, etc.</h1>
  <p>Dagu is a compact, portable workflow engine. It provides a declarative model for orchestrating software across diverse environments, including shell scripts, Python scripts, containerized operations.</p>
</div>


<div class="hero-section">
  <div class="hero-actions">
    <a href="/getting-started/quickstart" class="VPButton brand">Get Started in 3 Minutes</a>
    <a href="/writing-workflows/examples/" class="VPButton alt">View Examples</a>
  </div>
</div>

## Quick Start

Create and run your first Dagu workflow in minutes.

### Step 1: Install Dagu

::: code-group

```bash [Docker]
# Pull the latest image
docker pull ghcr.io/dagu-org/dagu:latest
```

```bash [Binary]
# Install via script (macOS, Linux, WSL)
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash

# Or via Homebrew
brew install dagu-org/brew/dagu
```

:::

### Step 2: Create Your First Workflow

```bash
mkdir -p ~/.dagu/dags && cat > ~/.dagu/dags/hello.yaml << 'EOF'
name: hello-world
description: My first Dagu workflow

steps:
  - name: greet
    command: echo "Hello from Dagu!"
    
  - name: show-date
    command: date
    
  - name: done
    command: echo "Workflow complete! 🎉"
EOF
```

### Step 3: Run the Workflow

::: code-group

```bash [Docker]
docker run \
--rm \
-v ~/.dagu:/app/.dagu \
-e DAGU_HOME=/app/.dagu \
ghcr.io/dagu-org/dagu:latest \
dagu start hello.yaml
```

```bash [Binary]
dagu start ~/.dagu/dags/hello.yaml
```

:::

For Docker: `--rm` removes container after execution, `-v` mounts your DAG directory, `-e` sets the home directory inside container.

### Step 4: View in Web UI

::: code-group

```bash [Docker]
docker run \
--rm \
-v ~/.dagu:/app/.dagu \
-e DAGU_HOME=/app/.dagu \
-p 8080:8080 \
ghcr.io/dagu-org/dagu:latest \
dagu start-all
```

```bash [Binary]
dagu start-all
```

:::

For Docker: `-p 8080:8080` exposes the web interface on port 8080. Open your browser to [http://localhost:8080](http://localhost:8080) to see the Dagu web interface.

## Why Dagu?

### Zero Dependencies
No database, no message broker, no complex setup. Just a single binary that works everywhere.

### Language Agnostic  
Run any command, script, or program. If it runs on your system, Dagu can orchestrate it.

### Powerful Yet Simple
From simple task automation to complex data pipelines, Dagu scales with your needs.

### Built-in Web UI
Monitor workflows, view logs, and manage executions through a clean, modern interface.

### Hierarchical Workflows
Compose workflows from reusable components. Build once, use everywhere.

### Production Ready
Battle-tested with robust error handling, retries, and comprehensive logging.

## Example Workflow

```yaml
name: data-pipeline
schedule: "0 2 * * *"  # 2 AM daily

steps:
  - name: extract
    command: python extract.py --date=${DATE}
    output: RAW_DATA
    
  - name: transform
    run: transform-module
    parallel:
      items: [customers, orders, products]
    params: "TYPE=${ITEM} INPUT=${RAW_DATA}"
    
  - name: load
    command: python load.py --date=${DATE}
    retryPolicy:
      limit: 3
      intervalSec: 30

---

name: transform-module
params:
  - TYPE: "Type of data to transform"
  - INPUT: "Input data file"
steps:
  - name: transform-data
    command: python transform.py --type=${TYPE} --input=${INPUT}
    output: TRANSFORMED_DATA

  - name: save-results
    command: python save.py --data=${TRANSFORMED_DATA}
```

## See Also

<div class="next-steps">
  <div class="step-card">
    <h3><a href="/getting-started/">Getting Started</a></h3>
    <p>Install Dagu and create your first workflow in minutes</p>
  </div>
  <div class="step-card">
    <h3><a href="/writing-workflows/examples/">Examples</a></h3>
    <p>Quick examples for all features with copy-paste ready code</p>
  </div>
  <div class="step-card">
    <h3><a href="/writing-workflows/">Writing Workflows</a></h3>
    <p>Learn how to build powerful, maintainable workflows</p>
  </div>
</div>

## Join the Community

Dagu is open source and welcomes contributions. Join us in making workflow orchestration simple and powerful.

<div class="community-links">
  <a href="https://github.com/dagu-org/dagu" class="community-link">
    <span class="icon">GitHub</span>
  </a>
  <a href="https://discord.gg/gpahPpqyAP" class="community-link">
    <span class="icon">Discord</span>
  </a>
  <a href="https://github.com/dagu-org/dagu/issues" class="community-link">
    <span class="icon">Issues</span>
  </a>
</div>
