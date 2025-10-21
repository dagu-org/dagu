---
layout: doc
---

<img src="/dagu-logo.webp" alt="dagu Logo" style="display: block; margin: 0 auto; max-width: 100%">

<div class="tagline" style="text-align: center;">
  <h2>Lightweight Workflow Engine Alternative to Airflow & Cron</h2>
  <p>Single binary with Web UI. Execute workflows defined in a simple, declarative YAML on a schedule. Natively support shell commands, remote execution via SSH, and docker image.</p>
</div>


<div class="hero-section">
  <div class="hero-actions">
    <a href="/getting-started/quickstart" class="VPButton brand">Get Started</a>
    <a href="/writing-workflows/examples/" class="VPButton alt">View Examples</a>
  </div>
</div>

## Quick Start

Install and run your first workflow in under 2 minutes.

### Install

::: code-group

```bash [macOS/Linux]
# Install to ~/.local/bin (default, no sudo required)
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash

# Install specific version
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash -s -- --version v1.17.0

# Install to custom directory
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash -s -- --install-dir /usr/local/bin
```

```bash [Docker]
docker pull ghcr.io/dagu-org/dagu:latest
```

```bash [Homebrew]
brew update && brew install dagu
```

```bash [npm]
npm install -g --ignore-scripts=false @dagu-org/dagu
```

:::

### 1. Create a workflow

::: code-group

```bash [Binary]
mkdir -p ~/.config/dagu/dags && cat > ~/.config/dagu/dags/hello.yaml << 'EOF'
steps:
  - echo "Hello from Dagu!"
  - echo "Running step 2"
EOF
```

```bash [Docker]
mkdir -p ~/.dagu/dags && cat > ~/.dagu/dags/hello.yaml << 'EOF'
steps:
  - echo "Hello from Dagu!"
  - echo "Running step 2"
EOF
```

:::

### 2. Run it

::: code-group

```bash [Binary]
dagu start hello
```

```bash [Docker]
docker run --rm \
  -v ~/.dagu:/var/lib/dagu \
  ghcr.io/dagu-org/dagu:latest \
  dagu start hello
```

:::

Output:
```
┌─ DAG: hello ─────────────────────────────────────────────────────┐
│ Status: Success ✓           | Started: 23:34:57 | Elapsed: 471ms │
└──────────────────────────────────────────────────────────────────┘

Progress: ████████████████████████████████████████ 100% (2/2 steps)
```

*Note: The output may vary if you are using Docker.*

:::

### 3. Check the status

::: code-group

```bash [Binary]
dagu status hello
```

```bash [Docker]
docker run --rm \
  -v ~/.dagu:/var/lib/dagu \
  ghcr.io/dagu-org/dagu:latest \
  dagu status hello
```

:::

### 4. Start the UI

::: code-group

```bash [Binary]
dagu start-all
```

```bash [Docker]
docker run -d \
  -p 8080:8080 \
  -v ~/.dagu:/var/lib/dagu \
  ghcr.io/dagu-org/dagu:latest \
  dagu start-all
```

:::

Visit [http://localhost:8080](http://localhost:8080) to see your workflows.

## Why Dagu?

### Zero Dependencies
Single binary. No database, no message broker. Deploy anywhere in seconds.

### Language Agnostic  
Execute any command. Your existing scripts work without modification.

### Production Ready
Battle-tested error handling, retries, logging, and monitoring.

### Hierarchical Workflows
Compose workflows from smaller workflows. Build modular, reusable components.

## Example: Data Pipeline with Nested Workflows

```yaml
schedule: "0 2 * * *"  # 2 AM daily

steps:
  - command: python extract.py --date=${DATE}
    output: RAW_DATA
    
  - call: transform-data
    parallel:
      items: [customers, orders, products]
    params: "TYPE=${ITEM} INPUT=${RAW_DATA}"
    
  - command: python load.py
    retryPolicy:
      limit: 3
      intervalSec: 2
      backoff: true      # Exponential backoff

---
name: transform-data
params: [TYPE, INPUT]
steps:
  - python transform.py --type=${TYPE} --input=${INPUT}
```

## Learn More

<div class="next-steps">
  <div class="step-card">
    <h3><a href="/getting-started/">Getting Started</a></h3>
    <p>Installation and first workflow</p>
  </div>
  <div class="step-card">
    <h3><a href="/writing-workflows/">Writing Workflows</a></h3>
    <p>Complete workflow authoring guide</p>
  </div>
  <div class="step-card">
    <h3><a href="/reference/yaml">YAML Reference</a></h3>
    <p>All configuration options</p>
  </div>
</div>

## Community

<div class="community-links">
  <a href="https://github.com/dagu-org/dagu" class="community-link">
    <span class="icon">GitHub</span>
  </a>
  <a href="https://discord.gg/gpahPUjGRk" class="community-link">
    <span class="icon">Discord</span>
  </a>
  <a href="https://bsky.app/profile/dagu-org.bsky.social" class="community-link">
    <span class="icon">Bluesky</span>
  </a>
  <a href="https://github.com/dagu-org/dagu/issues" class="community-link">
    <span class="icon">Issues</span>
  </a>
</div>
