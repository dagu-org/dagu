---
id: "010"
title: "Unified Execution Dispatch: workerSelector, Default Mode, and Local Escape Hatch"
status: draft
---

# RFC 010: Unified Execution Dispatch

## Summary

Introduce a unified dispatch model that fixes three gaps:

1. **Bug** — Immediate execution from the UI/API ignores `workerSelector`, running DAGs locally on the wrong node.
2. **New capability** — When a coordinator is configured, DAGs *without* `workerSelector` should optionally be dispatched to any available worker (server-level `defaultExecutionMode: distributed`).
3. **Escape hatch** — A way to force local execution per-DAG (`workerSelector: local`) and per-server (`defaultExecutionMode: local`, the default).

## Motivation

### Bug: UI ignores workerSelector

When a DAG defines `workerSelector` labels it expects to run on a matching worker. This works for scheduled runs, CLI starts, and retries, but the two central API functions — `startDAGRunWithOptions()` and `enqueueDAGRun()` — unconditionally spawn a local subprocess, bypassing the coordinator entirely.

### Missing capability: distribute all DAGs

Users who deploy a coordinator + workers want *all* DAGs dispatched to workers — not just those with explicit `workerSelector` labels. Today there is no server-level knob to opt into this behaviour; every DAG must carry a `workerSelector` block.

### Missing escape hatch

Once the server default is `distributed`, users need a way to pin specific DAGs to the coordinator/scheduler node itself (e.g., lightweight admin scripts). There is no mechanism for this today.

## Proposal

### 1. Server-level config: `defaultExecutionMode`

Add a new field to `config.Config`:

```go
// config.go

type Config struct {
    // ... existing fields ...
    DefaultExecutionMode ExecutionMode // "local" (default) | "distributed"
}

type ExecutionMode string

const (
    ExecutionModeLocal       ExecutionMode = "local"
    ExecutionModeDistributed ExecutionMode = "distributed"
)
```

Environment variable:

```
DAGU_DEFAULT_EXECUTION_MODE=distributed
```

This follows the existing env binding pattern in `internal/cmn/config/loader.go`. Add to `envBindings`:

```go
{key: "defaultExecutionMode", env: "DEFAULT_EXECUTION_MODE"},
```

YAML example:

```yaml
# server config
defaultExecutionMode: distributed   # dispatch all DAGs to workers
coordinator:
  host: 0.0.0.0
  port: 5890
```

Default is `local` — existing deployments are unaffected.

### 2. DAG-level: `workerSelector: local` escape hatch

Today `WorkerSelector` is typed `map[string]string` in both the spec and `core.DAG`. To support the special value `"local"`, change the **spec** field to `any`:

```go
// internal/core/spec/dag.go

type dag struct {
    // ...
    WorkerSelector any `yaml:"workerSelector"` // map[string]string | "local"
}
```

The builder parses this into two fields on `core.DAG`:

```go
// internal/core/dag.go

type DAG struct {
    // ...
    WorkerSelector map[string]string `json:"workerSelector,omitempty"`
    ForceLocal     bool              `json:"forceLocal,omitempty"`
}
```

Builder logic:

```go
// internal/core/spec/dag.go

func buildWorkerSelector(_ BuildContext, d *dag) (map[string]string, bool, error) {
    if d.WorkerSelector == nil {
        return nil, false, nil
    }

    // String "local" → force local execution
    if s, ok := d.WorkerSelector.(string); ok {
        if strings.EqualFold(strings.TrimSpace(s), "local") {
            return nil, true, nil // ForceLocal = true
        }
        return nil, false, fmt.Errorf(
            "workerSelector: unsupported string value %q (only \"local\" is allowed)", s,
        )
    }

    // map[string]string → label selector (existing behaviour)
    raw, ok := d.WorkerSelector.(map[string]any)
    if !ok {
        return nil, false, fmt.Errorf(
            "workerSelector: expected map or \"local\", got %T", d.WorkerSelector,
        )
    }
    ret := make(map[string]string, len(raw))
    for k, v := range raw {
        ret[strings.TrimSpace(k)] = strings.TrimSpace(fmt.Sprint(v))
    }
    return ret, false, nil
}
```

DAG YAML examples:

```yaml
# Dispatch to a worker with matching labels
workerSelector:
  gpu: "true"
  region: us-east-1

# Force local execution (even when server default is distributed)
workerSelector: local
```

### 3. Unified dispatch logic

Every code path that runs a DAG must use the same decision function:

```go
// internal/core/dispatch.go  (new file, ~20 lines)

// ShouldDispatchToCoordinator decides whether a DAG should be dispatched
// to a coordinator for distributed execution.
//
// Decision matrix:
//   dag.ForceLocal                                        → false (always local)
//   coordinatorCli == nil                                 → false (no coordinator)
//   len(dag.WorkerSelector) > 0                           → true  (explicit labels)
//   defaultMode == ExecutionModeDistributed                → true  (server opted in)
//   otherwise                                             → false (local)
func ShouldDispatchToCoordinator(
    dag *DAG,
    hasCoordinator bool,
    defaultMode config.ExecutionMode,
) bool {
    if dag.ForceLocal {
        return false
    }
    if !hasCoordinator {
        return false
    }
    if len(dag.WorkerSelector) > 0 {
        return true
    }
    return defaultMode == config.ExecutionModeDistributed
}
```

### 4. Fix immediate execution paths

#### `startDAGRunWithOptions` (`internal/service/frontend/api/v1/dags.go`)

Before spawning a local subprocess, check dispatch:

```go
func (a *API) startDAGRunWithOptions(ctx context.Context, dag *core.DAG, opts startDAGRunOptions) error {
    if core.ShouldDispatchToCoordinator(dag, a.coordinatorCli != nil, a.defaultExecMode) {
        if a.coordinatorCli == nil {
            return &Error{
                HTTPStatus: http.StatusServiceUnavailable,
                Code:       api.ErrorCodeInternalError,
                Message:    "coordinator not configured for distributed DAG execution",
            }
        }

        taskOpts := []executor.TaskOption{}
        if len(dag.WorkerSelector) > 0 {
            taskOpts = append(taskOpts, executor.WithWorkerSelector(dag.WorkerSelector))
        }

        task := executor.CreateTask(
            dag.Name,
            string(dag.YamlData),
            coordinatorv1.Operation_OPERATION_START,
            opts.dagRunID,
            taskOpts...,
        )
        if err := a.coordinatorCli.Dispatch(ctx, task); err != nil {
            return fmt.Errorf("error dispatching DAG to coordinator: %w", err)
        }

        // Wait for the DAG to start on the remote worker (same timeout as local path)
        ...
        return nil
    }

    // Local execution (existing code, unchanged)
    ...
}
```

This single change covers all callers:

| API Endpoint | Handler |
|---|---|
| Execute saved DAG | `ExecuteDAG` |
| Execute DAG and wait | `ExecuteDAGSync` |
| Execute inline spec | `ExecuteDAGRunFromSpec` |
| Reschedule DAG run | `RescheduleDAGRun` |

#### `enqueueDAGRun` (`internal/service/frontend/api/v1/dags.go`)

Same pattern:

```go
func (a *API) enqueueDAGRun(ctx context.Context, dag *core.DAG, params, dagRunID, nameOverride string, triggerType core.TriggerType) error {
    if core.ShouldDispatchToCoordinator(dag, a.coordinatorCli != nil, a.defaultExecMode) {
        if a.coordinatorCli == nil {
            return &Error{...}
        }

        taskOpts := []executor.TaskOption{}
        if len(dag.WorkerSelector) > 0 {
            taskOpts = append(taskOpts, executor.WithWorkerSelector(dag.WorkerSelector))
        }

        task := executor.CreateTask(
            dag.Name,
            string(dag.YamlData),
            coordinatorv1.Operation_OPERATION_START,
            dagRunID,
            taskOpts...,
        )
        if err := a.coordinatorCli.Dispatch(ctx, task); err != nil {
            return fmt.Errorf("error dispatching DAG to coordinator: %w", err)
        }

        // Wait for status change
        ...
        return nil
    }

    // Local enqueue (existing code, unchanged)
    ...
}
```

Covers:

| API Endpoint | Handler |
|---|---|
| Enqueue saved DAG | `EnqueueDAGDAGRun` |
| Enqueue inline spec | `EnqueueDAGRunFromSpec` |

#### `RetryDAGRun` (`internal/service/frontend/api/v1/dagruns.go`)

Update the existing `len(dag.WorkerSelector) > 0` guard to use `ShouldDispatchToCoordinator`:

```go
if core.ShouldDispatchToCoordinator(dag, a.coordinatorCli != nil, a.defaultExecMode) {
    // ... existing coordinator dispatch logic (unchanged) ...
}
```

### 5. Fix all other paths for consistency

#### Scheduler (`internal/service/scheduler/dag_executor.go`)

Replace `shouldUseDistributedExecution`:

```go
func (e *DAGExecutor) shouldUseDistributedExecution(dag *core.DAG) bool {
    return core.ShouldDispatchToCoordinator(dag, e.coordinatorCli != nil, e.defaultExecMode)
}
```

The `DAGExecutor` struct gains a `defaultExecMode config.ExecutionMode` field, injected at construction.

#### CLI (`internal/cmd/start.go`)

Update `tryExecuteDAG`:

```go
func tryExecuteDAG(ctx *Context, dag *core.DAG, dagRunID string, root exec.DAGRunRef, workerID string, triggerType core.TriggerType) error {
    // Already running on a worker — never re-dispatch
    if workerID != "local" {
        // ... existing local execution ...
    }

    if core.ShouldDispatchToCoordinator(dag, ctx.NewCoordinatorClient() != nil, ctx.Config.DefaultExecutionMode) {
        coordinatorCli := ctx.NewCoordinatorClient()
        if coordinatorCli == nil {
            return fmt.Errorf("coordinator required for distributed execution; configure peer settings")
        }
        return dispatchToCoordinatorAndWait(ctx, dag, dagRunID, coordinatorCli)
    }

    // ... existing local execution ...
}
```

#### Sub-DAG executor (`internal/runtime/executor/dag_runner.go`)

Update `SubDAGExecutor.Execute`:

```go
func (e *SubDAGExecutor) Execute(ctx context.Context, runParams RunParams, workDir string) (*exec1.RunStatus, error) {
    rCtx := exec.GetContext(ctx)
    if core.ShouldDispatchToCoordinator(e.DAG, e.coordinatorCli != nil, rCtx.DefaultExecMode) {
        // ... existing distributed dispatch ...
    }

    // ... existing local execution ...
}
```

### 6. Passing `defaultExecMode` through the system

The `config.DefaultExecutionMode` value must reach every dispatch site:

| Component | How it receives the value |
|---|---|
| `API` struct | New field, set during server init from `config.DefaultExecutionMode` |
| `DAGExecutor` | New constructor param from scheduler setup |
| CLI `tryExecuteDAG` | Read from `ctx.Config.DefaultExecutionMode` |
| `SubDAGExecutor` | Via `exec.Context.DefaultExecMode` (new field on runtime context) |

### 7. DAG JSON schema update

Update `internal/cmn/schema/dag.schema.json` to accept both map and string for `workerSelector` (appears twice — DAG-level and step-level):

```json
"workerSelector": {
  "oneOf": [
    {
      "type": "object",
      "additionalProperties": {
        "type": "string"
      },
      "description": "Key-value pairs specifying worker label requirements."
    },
    {
      "type": "string",
      "enum": ["local"],
      "description": "The string \"local\" forces execution on the scheduler/coordinator node."
    }
  ],
  "description": "Worker label requirements for distributed execution, or the string \"local\" to force local execution even when defaultExecutionMode is distributed."
}
```

### 8. Documentation updates

The following docs reference `workerSelector` and must be updated to document the `"local"` string value and `defaultExecutionMode`:

| File | What to add |
|---|---|
| `docs/features/distributed-execution.md` | Add `defaultExecutionMode` server config section; document `workerSelector: local` escape hatch |
| `docs/features/worker-labels.md` | Add `workerSelector: local` to label matching rules; note it bypasses distributed dispatch |
| `docs/reference/yaml.md` | Update `workerSelector` field type from `object` to `object \| "local"`; add example |
| `docs/configurations/base-config.md` | Add `workerSelector: local` example alongside existing label example |
| `docs/overview/architecture.md` | Mention `defaultExecutionMode` in task routing description |
| `docs/writing-workflows/examples.md` | Add example showing `workerSelector: local` in a distributed deployment |

## Files Changed

| File | Change |
|---|---|
| `internal/cmn/config/config.go` | Add `DefaultExecutionMode` field and `ExecutionMode` type |
| `internal/cmn/config/loader.go` | Add `DAGU_DEFAULT_EXECUTION_MODE` env binding |
| `internal/cmn/schema/dag.schema.json` | Update `workerSelector` schema to `oneOf: [object, "local"]` (DAG-level and step-level) |
| `internal/core/dag.go` | Add `ForceLocal bool` field to `DAG` struct |
| `internal/core/dispatch.go` | **New file** — `ShouldDispatchToCoordinator()` |
| `internal/core/spec/dag.go` | Change `WorkerSelector` spec type to `any`; update `buildWorkerSelector` to parse `"local"` |
| `internal/service/frontend/api/v1/dags.go` | Add coordinator dispatch to `startDAGRunWithOptions` and `enqueueDAGRun` |
| `internal/service/frontend/api/v1/dagruns.go` | Update `RetryDAGRun` to use `ShouldDispatchToCoordinator` |
| `internal/service/scheduler/dag_executor.go` | Update `shouldUseDistributedExecution` to delegate to `ShouldDispatchToCoordinator`; add `defaultExecMode` field |
| `internal/cmd/start.go` | Update `tryExecuteDAG` to use `ShouldDispatchToCoordinator` |
| `internal/runtime/executor/dag_runner.go` | Update `SubDAGExecutor.Execute` to use `ShouldDispatchToCoordinator` |
| `docs/features/distributed-execution.md` | Document `defaultExecutionMode` and `workerSelector: local` |
| `docs/features/worker-labels.md` | Add `workerSelector: local` to matching rules |
| `docs/reference/yaml.md` | Update `workerSelector` type and add example |
| `docs/configurations/base-config.md` | Add `workerSelector: local` example |
| `docs/overview/architecture.md` | Mention `defaultExecutionMode` in task routing |
| `docs/writing-workflows/examples.md` | Add `workerSelector: local` example |

## Consistency

After this change, every execution path uses `ShouldDispatchToCoordinator`:

| Code Path | Uses shared logic | Respects ForceLocal | Respects defaultMode |
|---|---|---|---|
| Scheduler | Yes | Yes | Yes |
| CLI | Yes | Yes | Yes |
| API — RetryDAGRun | Yes | Yes | Yes |
| API — startDAGRunWithOptions | Yes | Yes | Yes |
| API — enqueueDAGRun | Yes | Yes | Yes |
| Sub-DAG executor | Yes | Yes | Yes |

## Decision Matrix

| `dag.ForceLocal` | `coordinatorCli` | `defaultExecutionMode` | `len(WorkerSelector)` | Result |
|---|---|---|---|---|
| `true` | any | any | any | **local** |
| `false` | `nil` | any | any | **local** |
| `false` | present | any | `> 0` | **coordinator** |
| `false` | present | `distributed` | `0` | **coordinator** |
| `false` | present | `local` | `0` | **local** |

## Design Decisions

1. **Single decision function** — `ShouldDispatchToCoordinator` is the only place dispatch logic lives. Every call site delegates to it, eliminating inconsistencies between paths.

2. **`defaultExecutionMode` defaults to `local`** — Existing deployments (with or without a coordinator) are completely unaffected. Users must explicitly opt in to `distributed`.

3. **`workerSelector: local` as escape hatch** — Reuses the existing YAML field rather than adding a new top-level key. The string `"local"` is unambiguous and won't collide with label maps.

4. **`ForceLocal` on `core.DAG`, not on the spec** — The spec field stays flexible (`any`), while the runtime struct has a clean boolean. Parsing happens once in the builder.

5. **Error when coordinator is not configured** — If `ShouldDispatchToCoordinator` returns `true` but no coordinator client exists, return `503 Service Unavailable` rather than silently falling back. Silent fallback would mask misconfiguration.

6. **Tasks without WorkerSelector get no label filter** — When `defaultExecutionMode: distributed` dispatches a DAG that has no `workerSelector` labels, the task is created without `WithWorkerSelector`. The coordinator assigns it to any available worker.

## Migration

| Scenario | Before | After |
|---|---|---|
| No coordinator configured | All DAGs run locally | No change (default mode is `local`) |
| Coordinator + DAGs with `workerSelector` | Scheduled/CLI/retry dispatch; **UI runs locally (bug)** | All paths dispatch correctly |
| Coordinator + want all DAGs distributed | Must add `workerSelector` to every DAG | Set `defaultExecutionMode: distributed` once |
| Coordinator + distributed default + one DAG must stay local | Not possible | Add `workerSelector: local` to that DAG |
