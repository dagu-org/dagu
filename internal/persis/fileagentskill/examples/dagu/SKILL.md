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

## Step Identity: `id` vs `name`

**Always set `id` on every step. Omit `name` — it is redundant.** When `name` is omitted, it is automatically set to the `id` value. When both are omitted, a name is auto-generated (e.g., `cmd_1`, `docker_1`), but the step cannot be referenced by other steps.

The `id` field is required for:
- Referencing step outputs via `${step_id.stdout}`, `${step_id.stderr}`, `${step_id.exit_code}`
- Dependencies via `depends: [step_id]`

**Step ID rules:**
- Regex: `^[a-zA-Z][a-zA-Z0-9_]*$`
- Must start with a letter, followed by letters, digits, or underscores
- Max length: 40 characters
- **No hyphens** — use underscores instead
- Reserved words (cannot be used as IDs): `env`, `params`, `args`, `stdout`, `stderr`, `output`, `outputs`

## Step Execution

Steps can run commands via different executor types. The default executor runs shell commands.

```yaml
type: graph
steps:
  - id: hello
    command: echo "hello world"

  - id: process
    depends: [hello]
    script: |
      set -e
      echo "line 1"
      echo "line 2"
```

## Output Passing

### Captured output variables (`output:`)

Capture stdout **content** to a named variable with `output:`, reference with `${VAR}`:

```yaml
type: graph
steps:
  - id: get_date
    command: date +%Y-%m-%d
    output: TODAY

  - id: use_date
    command: echo "Today is ${TODAY}"
    depends: [get_date]
```

For JSON output, extract fields with `${VAR.key}`:

```yaml
type: graph
steps:
  - id: get_config
    command: echo '{"host":"db.example.com","port":5432}'
    output: CONFIG

  - id: use_config
    command: echo "Host is ${CONFIG.host}"
    depends: [get_config]
```

### Step reference properties (`${step_id.XXX}`)

Steps with an `id` expose three properties to downstream steps:

| Reference | Value |
|-----------|-------|
| `${step_id.stdout}` | **File path** to the step's stdout log |
| `${step_id.stderr}` | **File path** to the step's stderr log |
| `${step_id.exit_code}` | Exit code as a string |

**Important:** `${step_id.stdout}` returns the **file path**, not the content. Use `output:` to capture **content** into a variable.

Slicing syntax is supported: `${step_id.stdout:start:length}`

```yaml
type: graph
steps:
  - id: producer
    command: echo "hello world"
    output: CONTENT

  - id: consumer
    depends: [producer]
    script: |
      # Content captured by output: field
      echo "Content: ${CONTENT}"

      # File paths to log files
      echo "Stdout file: ${producer.stdout}"
      echo "Stderr file: ${producer.stderr}"

      # Exit code
      echo "Exit code: ${producer.exit_code}"

      # Slicing: first 5 chars of stdout path
      echo "Prefix: ${producer.stdout:0:5}"
```

Resolution priority: when `${foo.bar}` is evaluated, step references are checked first, then JSON path on variables.

## Parameters

```yaml
type: graph
params:
  env: production
  region: us-east-1

steps:
  - id: deploy
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
  - id: flaky_step
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
  - id: run_child
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
  - id: check
    command: echo "error"
    output: RESULT

  - id: route
    type: router
    value: ${RESULT}
    routes:
      "ok":
        - success_path
      "re:err.*":
        - error_path
    depends: [check]

  - id: success_path
    command: echo "success"

  - id: error_path
    command: echo "handling error"
```

## Quick Reference Tables

See the `references/` directory for complete details:
- `cli.md` — All 25 CLI subcommands with flags
- `schema.md` — Complete DAG YAML schema (top-level and step-level fields)
- `executors.md` — All 18 executor types with configuration
- `env.md` — Execution and configuration environment variables
- `pitfalls.md` — Critical pitfalls with examples
