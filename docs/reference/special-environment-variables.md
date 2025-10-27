# Special Environment Variables

Dagu injects a small set of read-only environment variables whenever it runs a workflow. These variables carry metadata about the current DAG run (name, run identifier, log locations, status) and are available for interpolation inside commands, arguments, lifecycle handlers, and most other places where you can reference environment variables.

## Availability

- **Step execution** – Every step receives the run-level variables plus a step-specific name and log file paths while it executes.
- **Lifecycle handlers** – `onExit`, `onSuccess`, `onFailure`, and `onCancel` handlers inherit the same variables. They additionally receive the final `DAG_RUN_STATUS` so that post-run automation can branch on success or failure.
- **Nested contexts** – When a step launches a sub DAG through the `dagu` CLI, the sub run gets its own identifiers and log locations; the parent identifiers remain accessible in the parent process for chaining or notifications.

Values are refreshed for each step, so `DAG_RUN_STEP_NAME`, `DAG_RUN_STEP_STDOUT_FILE`, and `DAG_RUN_STEP_STDERR_FILE` always point at whichever step (or handler) is currently running.

## Reference

| Variable | Provided In | Description | Example |
|----------|-------------|-------------|---------|
| `DAG_NAME` | All steps & handlers | Name of the DAG definition being executed. | `daily-backup` |
| `DAG_RUN_ID` | All steps & handlers | Unique identifier for the current run. Combines timestamp and a short suffix. | `20241012_040000_c1f4b2` |
| `DAG_RUN_LOG_FILE` | All steps & handlers | Absolute path to the aggregated DAG run log. Useful for attaching to alerts. | `/var/log/dagu/daily-backup/20241012_040000.log` |
| `DAG_RUN_STEP_NAME` | Current step or handler only | Name field of the step that is currently executing. | `upload-artifacts` |
| `DAG_RUN_STEP_STDOUT_FILE` | Current step or handler only | File path backing the step’s captured stdout stream. | `/var/log/dagu/daily-backup/upload-artifacts.stdout.log` |
| `DAG_RUN_STEP_STDERR_FILE` | Current step or handler only | File path backing the step’s captured stderr stream. | `/var/log/dagu/daily-backup/upload-artifacts.stderr.log` |
| `DAG_RUN_STATUS` | Lifecycle handlers only | Canonical completion status (`succeeded`, `partially_succeeded`, `failed`, `canceled`). | `failed` |
