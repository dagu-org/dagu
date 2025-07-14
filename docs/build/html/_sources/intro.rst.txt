.. _INTRO:

Introduction
=============

Background
----------

In many organizations, legacy systems still rely on hundreds of cron jobs running across multiple servers. These jobs are often written in various languages like Perl or Shell scripts, with implicit interdependencies. When one job fails, troubleshooting requires manually logging into servers via SSH and checking individual logs. To perform recovery, one must understand these implicit dependencies, which often rely on tribal knowledge. Dagu was developed to eliminate this complexity by providing a clear and understandable tool for workflow definition and dependency management.

Vision & Mission
----------------

Dagu sparks an exciting future where workflow engines drive software operations for everyone. It’s free from language dependencies, runs locally with minimal overhead, and strips away unnecessary complexity to make robust workflows accessible to all.

Core Principles
----------------

Dagu was born out of a desire to make workflow orchestration feasible in any environment—from long-standing legacy systems to IoT system and AI-driven pipelines. While Cron lacks clarity on dependencies and observability, and other tools such as Airflow, Prefect, or Temporal require you to write code in Python or Go, Dagu offers a simple, language-agnostic alternative. It’s designed with a focus on the following principles:

1. **Local friendly**
   Define and execute workflows in a single, self-contained environment without internet connection, making it simple to install and scale across diverse environments—from IoT devices to on-premise servers.

2. **Minimal Configuration**
   Start with just a single binary and YAML file, using local file storage for DAG definitions, logs, and metadata without requiring external databases.

3. **Language Agnostic**
   Any runtime—Python, Bash, Docker containers, or custom executors—works without extra layers of complexity.

4. **Keep it Simple**  
   Workflows are defined in clear, human-readable YAML with ease of development and fast onboarding. Dagu is designed to be simple to understand and use, even for non-developers.

5. **Community-Driven** 
   As an open-source project, developers can contribute new executors, integrate emerging technologies, or propose enhancements via GitHub. This encourages rapid iteration and keeps Dagu aligned with real-world use cases.

