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

## Step Execution

Steps can run commands via different executor types. The default executor runs shell commands.

```yaml
# Minimal step
steps:
  - name: hello
    command: echo "hello world"

# Script block
steps:
  - name: process
    script: |
      set -e
      echo "line 1"
      echo "line 2"
```

## Output Passing

Capture stdout to a variable with `output:`, reference in later steps with `${VAR}`:

```yaml
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
# Use list-of-maps to preserve ordering (maps iterate randomly in Go):
env:
  - BASE_DIR: /data
  - OUTPUT_DIR: ${BASE_DIR}/output
```

## Lifecycle Hooks

```yaml
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
steps:
  - name: run-child
    type: dag
    call: child-workflow
    params:
      input_file: /data/input.csv
```

## Conditional Routing

```yaml
steps:
  - name: check
    command: echo "error"
    output: RESULT
  - name: route
    type: router
    value: ${RESULT}
    routes:
      "ok":
        - name: success-path
          command: echo "success"
      "re:err.*":
        - name: error-path
          command: echo "handling error"
    depends: [check]
```

## Quick Reference Tables

See the `references/` directory for complete details:
- `cli.md` — All 25 CLI subcommands with flags
- `schema.md` — Complete DAG YAML schema (top-level and step-level fields)
- `executors.md` — All 18 executor types with configuration
- `env.md` — Execution and configuration environment variables
- `pitfalls.md` — Critical pitfalls with examples
