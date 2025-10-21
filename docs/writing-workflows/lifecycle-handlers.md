# Lifecycle Handlers

Lifecycle handlers let you run extra steps after the main DAG completes. Use the `handlerOn` block to trigger notifications, clean up resources, or kick off follow-up jobs without duplicating logic inside individual steps. Every handler runs with the canonical `DAG_RUN_STATUS` environment variable so you can branch on the final outcome inside a single script.

## Supported Triggers

| Handler | Trigger | Typical use cases |
|---------|---------|-------------------|
| `success` | All steps completed successfully, or the DAG ended in `partially_succeeded` | Deliver success notifications, enqueue downstream jobs |
| `failure` | The DAG ended with a canonical `failed` status | Page on-call, collect diagnostics |
| `cancel` | A stop request interrupted the run (manual stop, queue eviction, timeout cancellation) | Roll back partial work, release locks |
| `exit` | Always runs after the status-specific handler finishes (including when it fails or is skipped) | File system clean-up, archival tasks |

Only the handlers you define are executed. The scheduler runs the status-specific handler first (if present) and the `exit` handler last.

## Basic Definition

```yaml
# dag.yaml
handlerOn:
  success:
    command: notify.sh "${DAG_NAME} (${DAG_RUN_ID}) succeeded" # runs after a clean finish
  failure:
    command: alert.sh "${DAG_NAME} failed" "logs=${DAG_RUN_LOG_FILE}"
  cancel:
    command: rollback.sh --lock ${LOCK_NAME}
  exit:
    command: rm -rf /tmp/${DAG_RUN_ID} # always runs

steps:
  - command: ./extract.sh
  - command: ./load.sh
```

Each handler is a normal step definition. You can use `command`, `script`, `call` (or the legacy `run`), `executor`, containers, timeouts, or any other step field that makes sense for a single task.

## Execution Model

- The scheduler waits for all main steps to finish before evaluating handlers.
- It chooses the status-specific handler based on the canonical DAG status (`partially_succeeded` behaves like `success`).
- After the status-specific handler finishes (or if none was defined), the `exit` handler runs last.
- Handlers are executed sequentially and synchronously. The DAG is still considered running until they finish.
- If a handler exits with a non-zero status, the overall DAG run ends in `failed`, even if every main step succeeded.
- Handler logs appear alongside other steps in the run history and respect the same log retention policy.
- Each handler receives the `DAG_RUN_STATUS` environment variable so scripts can branch on `succeeded`, `partially_succeeded`, `failed`, or `canceled`.

## Patterns and Integrations

### Send Email with the Mail Executor

```yaml
handlerOn:
  failure:
    executor:
      type: mail
      config:
        to: oncall@company.com
        from: dagu@company.com
        subject: "${DAG_NAME} failed"
        message: |
          Run ID: ${DAG_RUN_ID}
          Logs: ${DAG_RUN_LOG_FILE}
```

### Run a Follow-up DAG

```yaml
handlerOn:
  success:
    call: sync-reporting
    params: |
      parent_run_id: ${DAG_RUN_ID}
```

### Guaranteed Cleanup

```yaml
handlerOn:
  exit:
    command: |
      find /tmp/${DAG_RUN_ID} -maxdepth 1 -type f -delete
```

For the complete schema, refer to the [YAML specification](/reference/yaml#lifecycle-handlers). Combine handlers with the techniques from [Error Handling](/writing-workflows/error-handling) and [Data & Variables](/writing-workflows/data-variables) to build robust workflow lifecycles.
