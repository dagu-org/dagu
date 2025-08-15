# Orchestrate workflows without complexity

Dagu is a powerful workflow engine designed to be deployable in environments where Airflow cannot be installed, such as small devices, on-premise servers, and legacy systems. It allows you to declaratively define any batch job as a single DAG (Directed Acyclic Graph) in a simple YAML format.

```yaml
steps:
  - command: sleep 1 && echo "Hello, Dagu!"
    
  - command: sleep 1 && echo "This is a second step"
```

By declaratively defining the processes within a job, complex workflows become visualized, making troubleshooting and recovery easier. Viewing log and retry can be performed from the Web UI, eliminating the need to manually log into a server via SSH.

It is equipped with many features to meet the highly detailed requirements of enterprise environments. It operates even in environments without internet access and, being statically compiled, includes all dependencies, allowing it to be used in any environment, including on-premise, cloud, and IoT devices. It is a lightweight workflow engine that meets enterprise requirements.

Workflow jobs are defined as commands. Therefore, legacy scripts that have been in operation for a long time within a company or organization can be used as-is without modification. There is no need to learn a complex new language, and you can start using it right away.

Dagu is designed for small teams of 1-3 people to easily manage complex workflows. It aims to be an ideal choice for teams that find large-scale, high-cost infrastructure like Airflow to be overkill and are looking for a simpler solution.

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
