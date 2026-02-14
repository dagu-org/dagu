---
id: "016"
title: "Defaults"
status: draft
---

# RFC 016: Step Defaults

## Summary

Add a `defaults` field at the DAG level that lets users define default values for common step configuration fields such as retry policy, continue-on behavior, timeout, preconditions, and environment variables. Steps inherit these defaults automatically and can override them individually.

---

## Motivation

### The Problem

DAG authors frequently need to apply the same configuration across every step — the same retry policy, the same timeout, the same failure behavior. Today this requires repeating the configuration on each step, which is verbose, error-prone, and makes updates tedious.

### Before: Repetitive Configuration

```yaml
name: data-pipeline
steps:
  - name: extract
    command: python extract.py
    retry_policy:
      limit: 3
      interval_sec: 5
    continue_on: failed
    timeout_sec: 600
    mail_on_error: true

  - name: transform
    command: python transform.py
    depends: [extract]
    retry_policy:
      limit: 3
      interval_sec: 5
    continue_on: failed
    timeout_sec: 600
    mail_on_error: true

  - name: validate
    command: python validate.py
    depends: [transform]
    retry_policy:
      limit: 3
      interval_sec: 5
    continue_on: failed
    timeout_sec: 600
    mail_on_error: true

  - name: load
    command: python load.py
    depends: [validate]
    retry_policy:
      limit: 3
      interval_sec: 5
    continue_on: failed
    timeout_sec: 600
    mail_on_error: true
```

Four steps, four identical copies of retry_policy, continue_on, timeout_sec, and mail_on_error. Adding a fifth step requires copying the same block again. Changing the retry limit means editing four places.

### After: With Step Defaults

```yaml
name: data-pipeline

defaults:
  retry_policy:
    limit: 3
    interval_sec: 5
  continue_on: failed
  timeout_sec: 600
  mail_on_error: true

steps:
  - name: extract
    command: python extract.py

  - name: transform
    command: python transform.py
    depends: [extract]

  - name: validate
    command: python validate.py
    depends: [transform]

  - name: load
    command: python load.py
    depends: [validate]
```

Same behavior, dramatically less repetition. Adding steps or changing defaults is a single edit.

---

## Proposal

### New DAG-Level Field

Add `defaults` as a top-level field in the DAG YAML spec. It accepts a subset of step configuration fields. These values are applied to every step that does not explicitly set its own value for that field.

```yaml
defaults:
  <field>: <value>
  ...
```

### Supported Fields

| Field | Type | Description |
|-------|------|-------------|
| `retry_policy` | object | Default retry policy for all steps |
| `continue_on` | string or object | Default continue-on behavior |
| `repeat_policy` | object | Default repeat policy for all steps |
| `timeout_sec` | int | Default timeout in seconds |
| `mail_on_error` | bool | Default mail-on-error flag |
| `signal_on_stop` | string | Default signal to send on stop |
| `env` | list | Default environment variables (additive) |
| `preconditions` | list | Default preconditions (additive) |

### Excluded Fields

These step fields are intentionally **not** supported in `defaults`:

| Field | Reason |
|-------|--------|
| `name`, `id`, `description` | Identity — unique per step |
| `command`, `script` | Defines what the step does — unique per step |
| `depends` | Defines graph structure — unique per step |
| `output`, `output_key`, `output_omit` | Step-specific output capture |
| `call`, `params` | Sub-DAG invocation — step-specific |
| `parallel` | Step-specific parallelism configuration |
| `type`, `config` | Executor-specific — not meaningfully shared |
| `container` | Step-specific container override |
| `llm`, `messages` | Chat-step specific |
| `value`, `routes` | Router-step specific |
| `stdout`, `stderr` | Step-specific file redirects |
| `shell`, `shell_args`, `working_dir`, `log_output` | Already inherited from DAG-level fields via existing mechanisms |
| `shell_packages` | Tightly coupled to shell — rarely shared |
| `worker_selector` | DAG-level `worker_selector` already exists |

---

## Override Semantics

### Full Override (Default Behavior)

For most fields, if a step explicitly sets a value, it **completely replaces** the default. There is no deep merge within compound fields.

```yaml
defaults:
  retry_policy:
    limit: 3
    interval_sec: 5
    backoff: 2.0

steps:
  - name: step1
    command: echo "inherits full retry_policy from defaults"

  - name: step2
    command: echo "uses its own retry_policy entirely"
    retry_policy:
      limit: 10
      interval_sec: 30
      # backoff is NOT inherited from defaults — this is a full replacement
```

### Additive Fields

`env` and `preconditions` use **additive** semantics — defaults are prepended to step-level values. Both the defaults and the step's own values apply.

```yaml
defaults:
  env:
    - LOG_LEVEL=info
  preconditions:
    - condition: test -f /tmp/ready

steps:
  - name: step1
    command: echo "has LOG_LEVEL=info and the precondition from defaults"

  - name: step2
    command: echo "has both default and step-level env/preconditions"
    env:
      - EXTRA_FLAG=true
    preconditions:
      - condition: test -d /data
```

In `step2`, the effective env is `[LOG_LEVEL=info, EXTRA_FLAG=true]` and preconditions include both the default check and the step's own check.

### Precedence Summary

| Field type | Step has value? | Result |
|------------|-----------------|--------|
| Regular (retry_policy, timeout_sec, etc.) | No | Use default |
| Regular | Yes | Step value wins (full replacement) |
| Additive (env, preconditions) | No | Use defaults only |
| Additive (env, preconditions) | Yes | Defaults prepended + step values |

---

## Scope of Application

### Steps

Defaults apply to all steps in the `steps:` list, regardless of step type (command, docker, ssh, chat, dag, etc.).

### Handler Steps

Defaults also apply to steps defined in `handler_on:` (init, failure, success, exit, wait). Handler steps follow the same override rules.

```yaml
defaults:
  timeout_sec: 300

handler_on:
  failure:
    command: notify.sh  # inherits 300s timeout from defaults
  exit:
    command: cleanup.sh
    timeout_sec: 60     # overrides default with 60s
```

### Base Config Interaction

When a base config file also defines `defaults`, the DAG-level `defaults` overrides the base config's `defaults` on a field-by-field basis. This is consistent with how other DAG-level fields interact with base config.

---

## Examples

### Resilient Pipeline with Retries

```yaml
name: resilient-pipeline

defaults:
  retry_policy:
    limit: 3
    interval_sec: 10
    backoff: 2.0
  timeout_sec: 900
  continue_on: failed
  mail_on_error: true

steps:
  - name: fetch-data
    command: python fetch.py

  - name: process
    command: python process.py
    depends: [fetch-data]

  - name: upload
    command: python upload.py
    depends: [process]
    retry_policy:
      limit: 5
      interval_sec: 30
    # upload gets more retries; all other defaults still apply
```

### Shared Preconditions

```yaml
name: production-workflow

defaults:
  preconditions:
    - condition: curl -sf http://api.internal/health
      expected: "ok"
  env:
    - ENVIRONMENT=production

steps:
  - name: deploy
    command: ./deploy.sh

  - name: migrate
    command: ./migrate.sh
    depends: [deploy]
    preconditions:
      - condition: test -f /data/migration-ready
    # Both the health check (from defaults) AND migration-ready check apply
```

### Monitoring DAG with Repeat Policy

```yaml
name: service-monitor

defaults:
  repeat_policy:
    repeat: while
    interval_sec: 30
    condition: "true"
  timeout_sec: 86400

steps:
  - name: check-api
    command: curl -sf http://api.example.com/health

  - name: check-db
    command: pg_isready -h db.example.com

  - name: check-cache
    command: redis-cli -h cache.example.com ping
```

### Graceful Shutdown with Signal Defaults

```yaml
name: long-running-services

defaults:
  signal_on_stop: SIGTERM
  timeout_sec: 3600

steps:
  - name: worker-1
    command: python worker.py --queue=high

  - name: worker-2
    command: python worker.py --queue=low

  - name: aggregator
    command: python aggregate.py
    signal_on_stop: SIGINT  # overrides default for this step
```

---

## Non-Goals

- **Deep merge of compound fields** — `retry_policy` at step level fully replaces the default. No per-sub-field merging (e.g., only overriding `limit` while inheriting `backoff` from the default). This keeps behavior predictable and avoids subtle merge bugs.
- **Per-step-type defaults** — No `defaults` scoped by executor type (e.g., separate defaults for docker steps vs command steps). All defaults apply uniformly.
- **Conditional defaults** — No support for defaults that vary by condition or tag. All steps in the DAG receive the same defaults.
- **Opt-out syntax** — No explicit way to "unset" a default for a specific step (e.g., `retry_policy: null`). A step either inherits or fully overrides.

---

## Relationship to Existing Features

| Feature | How `defaults` relates |
|---------|-----------------------------|
| **DAG-level `shell`** | `shell` is already a DAG-level field inherited by all steps. Not duplicated in `defaults`. |
| **DAG-level `working_dir`** | Same — already DAG-level. Not in `defaults`. |
| **DAG-level `log_output`** | Same — already has step > DAG > default priority chain. Not in `defaults`. |
| **DAG-level `llm`** | LLM config is already inherited by chat steps. Not in `defaults`. |
| **Base config** | Base config can define `defaults`. DAG overrides base config's `defaults`. |
| **DAG-level `env`** | DAG `env` sets process-level environment. `defaults.env` adds step-scoped env vars. Both can coexist. |
