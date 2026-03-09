---
name: dagu
description: Writes, validates, and debugs Dagu DAG workflow definitions in YAML. Covers all executor types, DAG YAML schema, CLI commands, environment variables, and critical pitfalls. Use when creating, editing, or troubleshooting Dagu .yaml DAG files. Do not use for general YAML editing.
---
# Dagu DAG Authoring Reference

Dagu runs workflows defined as DAGs in YAML. Each YAML file defines steps with commands, dependencies, and executor configurations. This reference covers everything needed to write correct DAG files.

## Execution Types

| Type | Behavior |
|------|----------|
| `chain` (default) | Steps run sequentially in definition order. `depends:` is not allowed. |
| `graph` | Steps run based on `depends:` declarations. Steps without `depends:` run immediately in parallel. |

**Always use `type: graph`** in DAG definitions. It supports both sequential (via `depends:`) and parallel execution, making it strictly more capable than `chain`. Every DAG you write should include `type: graph` at the top level.

## Step Execution

Steps can run commands via different executor types. The default executor runs shell commands.

```yaml
type: graph

# Minimal step
steps:
  - name: hello
    command: echo "hello world"

# Script block — depends on hello, so runs after it
  - name: process
    depends: [hello]
    script: |
      set -e
      echo "line 1"
      echo "line 2"
```

## Output Passing

Capture stdout to a variable with `output:`, reference in later steps with `${VAR}`:

```yaml
type: graph
steps:
  - name: get-date
    command: date +%Y-%m-%d
    output: TODAY
  - name: use-date
    command: echo "Today is ${TODAY}"
    depends: [get-date]
```

For JSON output, extract fields with `${VAR.key}`:

```yaml
type: graph
steps:
  - name: get-data
    command: echo '{"host":"db.example.com","port":5432}'
    output: CONFIG
  - name: use-data
    command: echo "Host is ${CONFIG.host}"
    depends: [get-data]
```

## Parameters

```yaml
type: graph
params:
  env: production
  region: us-east-1

steps:
  - name: deploy
    command: deploy --env ${env} --region ${region}
```

Override at runtime: `dagu start my-dag -- env=staging region=eu-west-1`

## Environment Variables

```yaml
type: graph
# Use list-of-maps to preserve ordering (maps iterate randomly in Go):
env:
  - BASE_DIR: /data
  - OUTPUT_DIR: ${BASE_DIR}/output
```

## Lifecycle Hooks

```yaml
type: graph
handler_on:
  init:
    command: echo "starting"
  success:
    command: echo "succeeded"
  failure:
    command: echo "failed with status ${DAG_RUN_STATUS}"
  exit:
    command: echo "always runs"
```

## Retry and Continue

```yaml
type: graph
steps:
  - name: flaky-step
    command: curl http://api.example.com/data
    retry_policy:
      limit: 3
      interval_sec: 10
    continue_on:
      failed: true
```

## Sub-DAGs

```yaml
type: graph
steps:
  - name: run-child
    type: dag
    call: child-workflow
    params:
      input_file: /data/input.csv
```

## Conditional Routing

Routes map patterns to lists of existing step names (not inline step definitions).

```yaml
type: graph
steps:
  - name: check
    command: echo "error"
    output: RESULT

  - name: route
    type: router
    value: ${RESULT}
    routes:
      "ok":
        - success-path
      "re:err.*":
        - error-path
    depends: [check]

  - name: success-path
    command: echo "success"

  - name: error-path
    command: echo "handling error"
```

## Quick Reference Tables

See the `references/` directory for complete details:
- `cli.md` — All 25 CLI subcommands with flags
- `schema.md` — Complete DAG YAML schema (top-level and step-level fields)
- `executors.md` — All 18 executor types with configuration
- `env.md` — Execution and configuration environment variables
- `pitfalls.md` — Critical pitfalls with examples
