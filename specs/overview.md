# Dagu Codebase – Architectural Overview

Dagu is a compact, portable workflow engine implemented in Go. It provides a declarative, YAML-based model for orchestrating tasks (as Directed Acyclic Graphs, DAGs) across diverse runtime environments including shell scripts, python, containerized, and remote commands. Dagu requires no external database, running as a single Go binary with a built-in modern web UI for management and monitoring.

## Purpose and Core Concepts
- Workflow Engine: Users define workflows as DAGs in YAML files. Each workflow consists of steps (tasks), parameters,
environment config, scheduling rules, and dependencies.
- Zero/Local Dependencies: Runs as a single process, with file-based persistence for definitions and execution
tatus/logs, making it suitable for air-gapped or local-first environments.
- Web UI & CLI: Both a rich browser UI (React/TS) and a Go-based CLI are provided for authoring, running, monitoring,
and managing workflows.
- API: REST/OpenAPI endpoints provide remote and programmatic control.

## Directory & Module Structure

### Top-Level

- `cmd/`

   Entrypoint(s) for CLI commands; typically a small `main.go` wiring up the root command using Cobra, importing "internal" modules.

- `internal/`

  Main backend application logic, divided into submodules:

  * `agent/` – Agents for workflow and status reporting.
  * `build/` – Build system versioning, codegen utilities.
  * `client/` – Code for interacting with the server's API programmatically (SDK).
  * `cmd/` – CLI subcommand implementations.
  * `cmdutil/` – Shared command-line utility helpers.
  * `config/` – Configuration loading and schema validation.
  * `digraph/` – **Core workflow engine:** parsing/validating DAG definitions, DAG and step structures, execution logic, schedules, parameter handling, and execution graph logic (DAG/step types). Includes `executor/` (task execution support incl. shell, docker, http, mail, ssh, child workflow, jq, etc) and `scheduler/` (dependency/schedule graph).
  * `fileutil/` – Helpers for file system management, file path resolution, etc.
  * `frontend/` – HTTP server for the web UI and APIs; API handlers (two versions: v1, v2), asset serving, uthentication, templates, and health checks.
  * `integration/` – Integration test support logic.
  * `logger/` – Logging framework and adapters.
  * `mailer/` – SMTP/email notification logic.
  * `persistence/` – Local file-based, and extensible storage layers for workflow state/history. Includes `jsondb/`, `local/`, file caches, grep etc.
  * `scheduler/` – Standalone and reusable scheduling logic; cron support, runner/job abstraction.
  * `sock/` – Unix socket client/server code; internal communication.
  * `stringutil/` – String matching, parameter expansion, etc.
  * `test/` – Helper code for test bootstrapping.
  * `testdata/` – Sample workflows, configs, and data for integration and regression tests.

- `api/`

   OpenAPI YAML specs for the public REST API (two versions) and configs; generates Go server/client code.

- `schemas/`

   JSON Schemas for validating workflow/YAML (DAG) files.

- `ui/`

   The web UI (React/TypeScript app) including sources, build, assets, models, hooks, and feature modules for DAG
isting, editor, visualization, logs, etc.

- `docs/`

   Sphinx-based documentation for usage, quickstarts, guides, API, and YAML/DAG specification reference.

- `examples/`

   Example DAGs and reference workflows.

- `config/`

   Config templates and supplementary config files (e.g., for SSL).

### Key Architectural Pieces

- DAGs & Steps:

  Central abstraction (`internal/digraph`) with YAML/JSON serialization. Steps can invoke commands, scripts, containers, http, mail, or dependent sub-workflows.

- Execution Engine:

  DAGs are loaded, parsed, parameterized, and resolved to an execution plan. Steps are scheduled based on dependencies/preconditions, executed by the appropriate executor, and status/results are persisted via the local JSONDB/persistence layer.

- Scheduling:

  Fine-grained cron-style or manual scheduling is supported (via `internal/digraph/scheduler` and `internal/scheduler`). Scheduled and ad-hoc DAGs interact via the scheduler and persistence layers.

- Persistence:

  All DAG definitions, run metadata, step logs, and history are stored on disk (by default via
internal/history/jsondb` and friends). No reliance on external DB or services.

- API + Web Server:

  The frontend exposes a REST API (in two versions for evolution), health endpoints, static file serving, and server-sent web UI integration for real-time monitoring.

- Web UI:

  Modern SPA (React/TS) that consumes the backend API to display DAGs, execution state/history/graphs, log streams, and allows CRUD/editing of workflows.

## Relationships and Data Flow

1. Authoring: User creates/revises YAML DAGs, either via file, CLI (dagu start ...), or web UI (which writes files on disk via backend APIs).
2. Execution: CLI or scheduler triggers DAG -> engine resolves dependencies, schedules and runs steps using built-in or plugin executors.
3. Observation: UI and API surface current and historical DAG/step state, logs, error diagnostics, and dependency graphs.
4. Persistence: All definitions and run results are committed to local storage, supporting air-gapped/offline-first workflows.

## Extensibility and Notable Features

- New executor types can be added with minimal changes.
- The engine supports subworkflows, conditional execution, parameterization, dynamic environments, and advanced retry/notification/handler mechanisms.
- Web UI is decoupled from the Go backend and is served as a built asset.
- Multiple API versions supported for gradual backend evolution.
