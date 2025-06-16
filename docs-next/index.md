---
layout: doc
---

# Dagu

**Workflows that just work** - Zero dependencies. Single binary. Infinite possibilities.

<div class="hero-section">
  <div class="hero-actions">
    <a href="/getting-started/" class="VPButton brand">Get Started in 3 Minutes</a>
    <a href="/writing-workflows/examples/" class="VPButton alt">View Examples</a>
  </div>
</div>

## Quick Start

<div class="code-group">
<div class="code-group-item active">

```bash
# Install Dagu
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash

# Create your first workflow
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
```

</div>
</div>

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

## Feature Comparison

<div class="comparison-table">

| Feature | Dagu | Airflow | Cron | GitHub Actions |
|---------|------|---------|------|----------------|
| **Dependencies** | <span class="feature-yes">None</span> | Python, Database, Message Broker | <span class="feature-yes">None</span> | GitHub |
| **Installation** | <span class="feature-yes">Single Binary</span> | Complex Setup | Built-in | N/A |
| **Web UI** | <span class="feature-yes">✓</span> | <span class="feature-yes">✓</span> | <span class="feature-no">✗</span> | <span class="feature-yes">✓</span> |
| **Language** | <span class="feature-yes">Any</span> | Python | <span class="feature-yes">Any</span> | <span class="feature-yes">Any</span> |
| **Local Development** | <span class="feature-yes">✓</span> | Difficult | <span class="feature-yes">✓</span> | <span class="feature-no">✗</span> |
| **Workflow Visualization** | <span class="feature-yes">✓</span> | <span class="feature-yes">✓</span> | <span class="feature-no">✗</span> | <span class="feature-yes">✓</span> |
| **Error Handling** | <span class="feature-yes">✓</span> | <span class="feature-yes">✓</span> | Limited | <span class="feature-yes">✓</span> |
| **Nested Workflows** | <span class="feature-yes">✓</span> | Limited | <span class="feature-no">✗</span> | Limited |

</div>

## Core Features

<div class="features-grid">

### Scheduling
Run workflows on a schedule with cron expressions. Skip redundant runs, set timezone, and more.

### Parallel Execution
Process multiple items concurrently. Control concurrency limits and aggregate results.

### Error Handling
Retry failed steps, continue on failure, send notifications, and run cleanup handlers.

### Multiple Executors
Run commands locally, in Docker containers, over SSH, make HTTP requests, send emails, and more.

### Rich Web UI
Monitor workflows in real-time, view logs, check history, and manage executions.

### Developer Friendly
Clear YAML syntax, comprehensive logging, fast onboarding, and excellent documentation.

</div>

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

handlerOn:
  failure:
    command: ./notify-failure.sh

---

name: transform-module
params:
  - INPUT: "Input data file"
steps:
  - name: transform-customers
    command: python transform_customers.py --input=${INPUT}
    
  - name: transform-orders
    command: python transform_orders.py --input=${INPUT}
    
  - name: transform-products
    command: python transform_products.py --input=${INPUT}
```

## Next Steps

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
