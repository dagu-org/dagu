# What is Dagu?

Dagu is a powerful, self-contained workflow orchestration engine that runs as a single binary with zero external dependencies. It's designed to solve the complexity of managing workflows without the overhead of traditional orchestration platforms.

## Why Dagu Exists

Picture this: hundreds of cron jobs scattered across multiple servers, written in various languages, with hidden dependencies that only a few people understand. When something breaks at 3 AM, you're SSHing into servers, hunting through logs, trying to piece together what went wrong.

Sound familiar? That's exactly why Dagu was created.

Many organizations still rely on these legacy job scheduling systems. The scripts might be in Perl, Shell, Python, or a mix of everything. Dependencies are implicit—Job B assumes Job A created a certain file, but this isn't documented anywhere. Recovery requires tribal knowledge that disappears when team members leave.

**Dagu eliminates this complexity** by providing a clear, visual, and understandable way to define workflows and manage dependencies.

## Vision & Mission

Dagu sparks an exciting future where workflow engines drive software operations for everyone. 

Our vision is simple but powerful:
- **Free from language lock-in** - Use any tool, any language
- **Runs anywhere** - From your laptop to production servers
- **Minimal overhead** - Single binary, no infrastructure required
- **Accessible to all** - Clear YAML syntax that anyone can understand

We're stripping away unnecessary complexity to make robust workflows accessible to everyone, not just specialized engineers.

## Core Principles

Dagu was born from a simple observation: existing workflow tools either lack features (like cron) or require too much commitment (like Airflow, Prefect, or Temporal forcing you into Python or Go ecosystems).

We built Dagu around six core principles:

### 1. Local First
Define and execute workflows in a single, self-contained environment—no internet required. Whether you're prototyping on your laptop, running on IoT devices, or deploying to air-gapped on-premise servers, Dagu just works.

### 2. Minimal Configuration
Start with just:
- One binary
- One YAML file
- That's it!

No external databases. No message queues. Local file storage handles everything—DAG definitions, logs, and metadata. Complex infrastructure shouldn't be a prerequisite for workflow automation.

### 3. Language Agnostic
Your workflows, your choice:
```yaml
steps:
  - name: python-task
    command: python analyze.py
  - name: bash-task  
    command: ./process.sh
  - name: docker-task
    executor:
      type: docker
      config:
        image: node:18
    command: npm run build
```

Any runtime works without extra layers of complexity. Use the tools your team already knows.

### 4. Keep it Simple
Workflows are defined in clear, human-readable YAML:

```yaml
steps:
  - name: download data
    command: curl https://api.example.com/data.json -o data.json
    
  - name: process data
    command: python process.py data.json
    depends: download data
    
  - name: upload results
    command: aws s3 cp results.csv s3://my-bucket/
    depends: process data
```

Simple to understand, even for non-developers. Fast onboarding for new team members.

### 5. Intelligent Queue Management
Built-in queue system provides flexible concurrency control:
- Manage resource utilization
- Prevent system overload
- Control parallel execution
- No external dependencies needed

Perfect for rate-limited APIs, resource-constrained environments, or when you need fine-grained control over execution flow.

### 6. Community-Driven
As an open-source project, Dagu evolves with its users:
- Contribute new executors
- Integrate emerging technologies
- Propose enhancements via GitHub
- Share workflows and best practices

Real-world use cases drive development, keeping Dagu practical and aligned with what teams actually need.

## The Solution

Dagu takes a radically simple approach:

```bash
# Install
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash

# Run
dagu start my-workflow.yaml
```

That's it. No databases to configure. No Python environments to manage. No cloud accounts to set up.

## Key Features

### **Zero Dependencies**
One binary. Works on Linux, macOS, Windows. No database, no message broker, no runtime dependencies.

### **Hierarchical DAG Composition**
Build complex workflows from simple, reusable components:

```yaml
steps:
  - name: data-pipeline
    run: etl.yaml
    params: "SOURCE=prod"
  - name: ml-training
    run: train.yaml
    depends: data-pipeline
```

### **Built-in Web UI**
Monitor workflows, view logs, and manage executions through a clean, modern interface. No additional setup required.

### **Production Ready**
- Robust error handling with configurable retries
- Comprehensive logging with stdout/stderr separation  
- Graceful shutdown and cleanup
- Process group management
- Signal handling

## Architecture

Dagu's architecture is refreshingly simple:

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Web UI    │────▶│  REST API   │────▶│   Engine    │
└─────────────┘     └─────────────┘     └─────────────┘
                                              │
                                              ▼
                                       ┌─────────────┐
                                       │ File System │
                                       └─────────────┘
```

- **No Database**: All state is stored in files
- **No Message Broker**: Processes communicate via Unix sockets
- **No External Services**: Everything runs in a single process

## When to Use Dagu

Dagu is perfect for:

- **Data Engineering**: ETL pipelines, data processing, batch jobs
- **DevOps Automation**: Deployments, backups, system maintenance
- **Business Automation**: Report generation, data synchronization
- **Local Development**: Test workflows on your laptop before deploying
- **Legacy System Modernization**: Replace scattered cron jobs with managed workflows
- **IoT & Edge Computing**: Run workflows on resource-constrained devices
- **AI/ML Pipelines**: Orchestrate training, evaluation, and deployment workflows

## When NOT to Use Dagu

Dagu might not be the best choice for:

- Workflows requiring sub-second scheduling precision
- Distributed execution across multiple machines (consider Kubernetes-based solutions)
- Workflows with extremely complex state management needs
- Real-time streaming data processing (consider Apache Flink or Spark)

## Ready to Get Started?

Dagu transforms the chaos of scattered scripts and cron jobs into clear, manageable, and reliable workflows. Whether you're modernizing legacy systems, building IoT pipelines, or creating AI workflows, Dagu provides the simplicity and power you need.

[Continue to Getting Started →](/getting-started)

Or explore [real-world examples](/examples) to see what Dagu can do.
