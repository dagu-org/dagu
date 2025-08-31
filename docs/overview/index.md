# What is Dagu?

Dagu is a lightweight workflow engine built in a single binary with modern Web UI. Define any workflow in a simple, declarative YAML format and execute arbitrary workflows on schedule. Natively support shell commands, remote execution via SSH, and docker image. Dagu is a lightweight alternative to Cron, Airflow, Rundeck, etc.

### How it Works
Dagu executes your workflows, which are defined as a series of steps in a YAML file. These steps form a Directed Acyclic Graph (DAG), ensuring a clear and predictable flow of execution.

For example, a simple sequential DAG:
```yaml
steps:
  - echo "Hello, dagu!"
  - echo "This is a second step"
```

With parallel steps:
```yaml
steps:
  - echo "Step 1"
  - 
    - echo "Step 2a - runs in parallel"
    - echo "Step 2b - runs in parallel"
  - echo "Step 3 - waits for parallel steps"
```

Or with explicit dependencies:
```yaml
steps:
  - name: step 1
    command: echo "Hello, dagu!"
  - name: step 2
    command: echo "This is a second step"
    depends: step 1
```

## Demo

**CLI Demo**: Create a simple DAG workflow and execute it using the command line interface.

![Demo CLI](/demo-cli.webp)

**Web UI Demo**: Create and manage workflows using the web interface, including real-time monitoring and control.

[Docs on CLI](/overview/cli)

![Demo Web UI](/demo-web-ui.webp)

[Docs on Web UI](/overview/web-ui)

## When to Use Dagu

**Perfect for:**
- Data pipelines and ETL
- DevOps automation
- Scheduled jobs and batch processing
- Replacing cron with something manageable
- Local development and testing

**Not ideal for:**
- Sub-second scheduling requirements
- Real-time stream processing

## Quick Comparison

| Feature | Cron | Airflow | Dagu |
|---------|------|---------|------|
| Dependencies | ❌ Manual | ✅ Python only | ✅ Any language |
| Monitoring | ❌ Log files | ✅ Web UI | ✅ Web UI |
| Setup Time | ✅ Minutes | ❌ Hours/Days | ✅ Minutes |
| Infrastructure | ✅ None | ❌ Database, Queue | ✅ None |
| Error Handling | ❌ Manual | ✅ Built-in | ✅ Built-in |
| Scheduling | ✅ Basic | ✅ Advanced | ✅ Advanced |

## Learn More

- [Quick Start Guide](/getting-started/quickstart) - Up and running in 2 minutes
- [Core Concepts](/getting-started/concepts) - Understand how Dagu works
- [Examples](/writing-workflows/examples) - Ready-to-use workflow patterns
