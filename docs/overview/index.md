# What is Dagu?

Dagu is a workflow engine that runs as a single binary with zero dependencies. It replaces complex orchestration platforms with a tool that just works—on your laptop, in containers, or on production servers.

## The Problem

You know the pain: hundreds of cron jobs scattered across servers, written in different languages, with hidden dependencies. When something breaks at 3 AM, you're hunting through logs, trying to understand what went wrong.

Traditional orchestration platforms solve this but introduce new complexity: databases, message queues, language lock-in. You need a team just to manage the infrastructure.

## The Solution

Dagu provides powerful workflow orchestration without the overhead:

```yaml
steps:
  - name: extract
    command: python extract.py
    
  - name: transform
    command: ./transform.sh
    
  - name: load
    command: psql -f load.sql
    retryPolicy:
      limit: 3
      intervalSec: 30
```

Clear dependencies. Visual monitoring. One binary. No database.

## Design Philosophy

### 1. Single Binary
Download and run. No installation process, no dependencies, no containers required.

### 2. Language Agnostic
Use any command, any language. Your existing scripts work without modification.

### 3. File-Based Storage
All state in local files. Version control your workflows. Understand exactly what's happening.

### 4. Production Ready
Built-in scheduling, error handling, retries, logging, and monitoring. Everything you need.

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
