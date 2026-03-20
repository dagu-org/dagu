---
name: dagu
description: Writes, validates, and debugs Dagu DAG workflow definitions in YAML. Covers all executor types, DAG YAML schema, CLI commands, environment variables, and critical pitfalls. Use when creating, editing, or troubleshooting Dagu .yaml DAG files. Do not use for general YAML editing.
---
# Dagu DAG Authoring Reference

Dagu runs workflows defined as DAGs in YAML. Each YAML file defines steps with commands, dependencies, and executor configurations.

## Execution Types

| Type | Behavior |
|------|----------|
| `chain` (default) | Steps run sequentially in definition order. `depends:` is not allowed. |
| `graph` | Steps run based on `depends:` declarations. Steps without `depends:` run immediately in parallel. |

**Always use `type: graph`** — it supports both sequential (via `depends:`) and parallel execution, making it strictly more capable than `chain`.

## Step Identity: `id` vs `name`

**Always set `id` on every step. Omit `name` — it is redundant.** When `name` is omitted, it defaults to the `id` value. Without both, a name is auto-generated but the step cannot be referenced.

`id` is required for output references (`${step_id.stdout}`, `${step_id.exit_code}`) and dependencies (`depends: [step_id]`).

**Step ID rules:** regex `^[a-zA-Z][a-zA-Z0-9_]*$`, max 40 chars, **no hyphens** (use underscores). Reserved words: `env`, `params`, `args`, `stdout`, `stderr`, `output`, `outputs`.

## Step Execution

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

Capture stdout **content** to a named variable with `output:`, reference with `${VAR}`. For JSON output, extract fields with `${VAR.key}`.

### Step reference properties (`${step_id.XXX}`)

| Reference | Value |
|-----------|-------|
| `${step_id.stdout}` | **File path** to the step's stdout log |
| `${step_id.stderr}` | **File path** to the step's stderr log |
| `${step_id.exit_code}` | Exit code as a string |

**Important:** `${step_id.stdout}` returns the **file path**, not the content. Use `output:` to capture **content**. Slicing is supported: `${step_id.stdout:start:length}`.

### Combined example

```yaml
type: graph
steps:
  - id: get_config
    command: echo '{"host":"db.example.com","port":5432}'
    output: CONFIG

  - id: consumer
    depends: [get_config]
    script: |
      # Content via output: variable (direct value + JSON field access)
      echo "Full: ${CONFIG}"
      echo "Host: ${CONFIG.host}"

      # File paths to log files
      echo "Stdout file: ${get_config.stdout}"
      echo "Stderr file: ${get_config.stderr}"
      echo "Exit code: ${get_config.exit_code}"
```

Resolution priority: when `${foo.bar}` is evaluated, step references are checked first, then JSON path on variables.

## Parameters and Environment Variables

```yaml
type: graph
params:
  env: production
  region: us-east-1

# Use list-of-maps to preserve ordering (maps iterate randomly in Go):
env:
  - BASE_DIR: /data
  - OUTPUT_DIR: ${BASE_DIR}/output

steps:
  - id: deploy
    command: deploy --env ${env} --region ${region} --out ${OUTPUT_DIR}
```

Override params at runtime: `dagu enqueue my-dag -- env=staging region=eu-west-1`

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

## Dynamic Fan-Out with `parallel:`

Use `parallel:` to iterate over dynamic output from a previous step. Each item gets its own sub-DAG run with retry, timeout, and UI visibility. Requires `call:` (see pitfalls). Do NOT use bash `for` loops over output variables.

```yaml
type: graph
steps:
  - id: get_items
    command: gh api repos/org/repo/tags --jq '.[].name'
    output: TAG_LIST

  - id: process_each
    depends: [get_items]
    call: process-tag
    parallel:
      items: ${TAG_LIST}     # auto-splits on newlines, commas, or spaces
      max_concurrent: 5
    params:
      tag: ${ITEM}
---
name: process-tag
steps:
  - id: run
    command: echo "Processing tag ${tag}"
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

## CLI Quick Reference

```bash
dagu config                            # Show resolved paths (DAGs dir, logs, data, etc.)
dagu schema dag                        # All DAG root-level fields
dagu schema dag steps.container        # Drill into nested fields with dot paths
dagu schema config                     # All config fields
dagu enqueue my-dag                    # Preferred over `dagu start` for agent use
dagu status my-dag                     # Latest run status
dagu status --run-id=<id> my-dag       # Specific run
dagu history my-dag --last 7d --status failed  # Find past run IDs
```

Prefer `dagu enqueue` over `dagu start`. Do not check whether the DAG is already running before enqueueing unless the user explicitly asks.

## Quick Reference Tables

See the `references/` directory:
- `cli.md` — CLI subcommands and flags
- `schema.md` — Complete DAG YAML schema
- `executors.md` — All executor types with configuration
- `env.md` — Environment variables
- `pitfalls.md` — Critical pitfalls with examples
- `codingagent.md` — AI coding agent integration
