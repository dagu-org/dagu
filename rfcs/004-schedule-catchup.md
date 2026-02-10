---
id: "004"
title: "Schedule Catch-up and Backfill"
status: implemented
---

# RFC 004: Schedule Catch-up and Backfill

## Summary

Implement configurable catch-up and backfill mechanisms for scheduled DAGs, allowing missed schedules during scheduler downtime to be automatically executed based on user-defined policies.

## Motivation

Currently, when the Dagu scheduler process is shut down or crashes, scheduled cron jobs are silently skipped with no recovery mechanism. This is problematic for:

1. **Data pipelines** - Missing hourly/daily ETL runs creates data gaps
2. **Report generation** - Scheduled reports are lost during outages
3. **Reliability expectations** - Users expect scheduled jobs to eventually run

### Current Behavior

The scheduler advances by exactly 1 minute per tick. If the scheduler is offline for 2 hours, all jobs scheduled during that period are permanently lost. There is no state tracking of the last processed schedule time and no catch-up mechanism.

## Industry Research

### How Other Workflow Engines Handle This

| Engine | Approach | Key Features |
|--------|----------|--------------|
| **Apache Airflow** | `start_date` + `end_date` + `catchup` | Data intervals, automatic backfill from start_date |
| **Quartz** | Misfire instructions | 6+ granular policies (fire once, fire all, do nothing, etc.) |
| **Temporal** | `catchupWindow` + overlap policies | 1-year default catchup window, 6 overlap policies |
| **Kubernetes CronJob** | `startingDeadlineSeconds` | Grace period for missed schedules, 100-miss limit |
| **Argo Workflows** | `startingDeadlineSeconds` | Single missed execution recovery |
| **Dagster** | Partition-based | Sensors detect and fill missing partitions |
| **Luigi** | `RangeDaily` wrapper | Developer-managed catch-up with task limits |
| **Prefect/n8n** | None | No built-in catch-up (common user complaint) |

### Key Insights

1. **Airflow's model** (`start_date`/`end_date` + `catchup`) is the most intuitive and widely adopted
2. **Quartz's policies** provide fine-grained control for different use cases
3. **Relative lookback windows** (Temporal/K8s) keep scheduler config simple and timezone-agnostic
4. **Rate limiting** is essential to prevent thundering herd on restart

## Proposal

### Two DAG-Level Fields

Add two optional top-level fields to the DAG spec. No changes to the `schedule` field format.

```yaml
name: hourly-etl
schedule: "0 * * * *"           # Unchanged — no new schedule formats needed
catchupWindow: "6h"            # Opt-in: enables catchup, sets lookback horizon
overlapPolicy: all             # What to do when runs pile up: skip | all
```

- **`catchupWindow`** — Duration string. If set, enables catchup. All missed intervals within the window are detected on scheduler restart or DAG re-enable. If omitted, no catchup (current behavior).
- **`overlapPolicy`** — Controls how multiple catch-up runs are handled:
  - `skip` (default) — Skip new run if previous is still running. For catchup: only the first missed interval runs; others are skipped while it's in progress.
  - `all` — Execute all missed runs **sequentially** (queued, one after another in order). Ensures every missed interval is processed.

Both fields apply DAG-wide to all start schedules. There is no per-schedule-entry configuration.

#### Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `catchupWindow` | duration | *(omitted)* | Lookback horizon for missed intervals. If set, enables catchup. If omitted, no catchup (current behavior) |
| `overlapPolicy` | `skip` \| `all` | `skip` | How to handle multiple catch-up runs. `skip`: only first missed interval runs; `all`: every missed interval runs sequentially |

#### Duration Format

The `catchupWindow` field uses a custom duration grammar that supports `m` (minutes), `h` (hours), and `d` (days, = 24h). This is a superset of Go's `time.ParseDuration` (which does not support `d`):

| Suffix | Meaning | Example |
|--------|---------|---------|
| `m` | minutes | `30m` |
| `h` | hours | `6h` |
| `d` | days (= 24h) | `7d` |

Durations are parsed as a **sum of `<number><unit>` tokens** with no separators. Tokens can be repeated and combined across units. Examples:

- `2d12h` = 60h
- `1d30m` = 24h30m
- `90m` = 90 minutes

Validation rules:

- Tokens must be positive integers followed by a unit.
- Empty strings, missing units (e.g. `2d12`), and negative values are invalid.
- `0` or `0h` is invalid when `catchupWindow` is set.

#### Backward Compatibility

Existing DAGs are completely unaffected. Both new fields are optional and default to no-catchup behavior:

```yaml
# These behave identically (no catchup):
schedule: "0 9 * * *"

schedule:
  start: "0 9 * * *"
  stop: "0 18 * * *"
```

No changes to any existing `schedule` format — string, array, or map forms all continue to work exactly as before.

#### Scope

Catch-up applies only to **start** schedules. Stop and restart schedules are excluded:

- A missed **stop** is a no-op if the DAG is not currently running. Retroactively stopping a DAG that already completed naturally would be incorrect.
- A missed **restart** combines a stop and a start. The stop portion has the same issue, and the start portion is covered by catch-up on the start schedule.

#### Multiple Schedules on the Same DAG

When a DAG has multiple start schedules, `catchupWindow` and `overlapPolicy` apply uniformly to all of them. Each schedule entry is evaluated independently during catch-up detection. Overlapping schedule times do **not** deduplicate—each schedule can produce its own run.

```yaml
# Both schedules use the same catchupWindow and overlapPolicy
name: multi-schedule
schedule:
  - "0 * * * *"
  - "30 9 * * *"
catchupWindow: "6h"
overlapPolicy: all
```

Each schedule entry is evaluated independently and the resulting candidates are merged in chronological order before dispatch.

## Design

### Watermarks

Scheduler state is persisted via a `WatermarkStore` interface (`internal/core/exec/watermark.go`) with a file-based implementation (`internal/persis/filewatermark/store.go`). The state file (`<dataDir>/scheduler/state.json`) contains:

```json
{
  "version": 1,
  "lastTick": "2026-02-07T12:00:00Z",
  "dags": {
    "hourly-etl": {"lastScheduledTime": "2026-02-07T12:00:00Z"},
    "daily-report": {"lastScheduledTime": "2026-02-07T09:00:00Z"}
  }
}
```

Two watermarks determine what needs replaying:

1. **Scheduler watermark** (`lastTick`) — the last tick time the scheduler processed. On restart, the gap between this timestamp and `now` is the recovery window. If the state file is missing or corrupt, it is treated as empty (no catch-up). Written atomically via temp file + rename.
2. **Per-DAG watermark** (`dags[name].lastScheduledTime`) — tracks the most recent dispatched scheduled time per DAG. New DAGs get `lastScheduledTime = now` when first observed via the filesystem watcher. Deleted DAGs are pruned from state on startup.

State is mutated in memory on every dispatch (zero disk I/O). A background goroutine flushes to disk every 5 seconds if dirty, with a final flush on shutdown. This bounds disk writes to ~12/minute regardless of DAG count.

### Replay Boundaries

When catch-up is triggered, the earliest timestamp worth replaying is:

```
replayFrom = max(
    now - catchupWindow,             // user-configured lookback horizon
    state.LastTick,                  // last tick the scheduler dispatched
    state.DAGs[name].LastScheduledTime, // per-DAG watermark from state store
)
```

This ensures:

- You never replay intervals older than the configured window.
- Brand-new DAGs get `LastScheduledTime = now` when first observed (via filesystem watcher), so catch-up never replays history that predates the DAG's existence.
- DAGs that were paused or backfilled manually inherit the timestamp of their latest dispatched run, avoiding duplicate work.

### Catch-up Trigger Points

1. **Scheduler restart** — `state.LastTick` lags behind `now`, catch-up queues are populated before the live loop starts.
2. **Manual backfill while scheduler is down** — advances the per-DAG watermark via the run dispatch, so the subsequent restart only replays the remaining gap.

No catch-up work happens while the scheduler is healthy and processing ticks in real time.

### Ordering Guarantees

Catch-up uses **per-DAG in-memory queues** (`internal/service/scheduler/dagqueue.go`) that unify catch-up and live-tick runs into a single dispatch mechanism per DAG:

1. Load scheduler state from disk (missing/corrupt = empty, no catch-up).
2. For each DAG with `catchupWindow > 0`, compute missed intervals and create a buffered channel-based queue.
3. Send all catch-up items into the queue, then start a consumer goroutine.
4. Enter the live cron loop. Live-tick jobs for DAGs with queues are routed through the same queue; DAGs without `catchupWindow` dispatch directly (backward-compatible).
5. Consumer goroutines process items in FIFO order, enforcing `overlapPolicy` per DAG.

This ensures:
- Catch-up runs for a given DAG execute in chronological order.
- Multiple DAGs catch up concurrently (each DAG has its own queue).
- Live ticks merge naturally into the queue after catch-up items.
- `overlapPolicy` is enforced at the queue level: `skip` discards items when a run is active; `all` waits for the active run to complete before dispatching the next.

**Cron changes during downtime**: catch-up uses the **current** schedule expression. If a cron expression changed while the scheduler was down, missed intervals are computed against the new expression (not historical ones). This is a deliberate design choice to keep the scheduler stateless with respect to past cron definitions.

### Watermark Semantics

The scheduler watermark tracks **dispatch**, not execution completion. This matches the existing fire-and-forget pattern where jobs are launched asynchronously.

A catch-up run that fails at execution time will **not** be retried on next restart — the watermark has already moved past it. This is intentional: retrying failed runs is a separate concern (retry policies, alerting) and conflating it with catch-up would risk infinite retry loops. Users who need retry-on-failure should configure step-level retries within the DAG.

During catch-up, the watermark advances **per successful dispatch** to the scheduled time. If a catch-up run fails to **dispatch** (e.g. the persistence layer is unavailable), the watermark does not advance past that time and catch-up stops. On next restart the same interval will be retried, providing at-least-once dispatch semantics.

> A dispatch is considered **successful** when the DAG run is persisted to the queue store. Subsequent failure to reach the coordinator is handled by the queue retry mechanism and does not affect the watermark.

Catch-up dispatch is fire-and-forget; it does **not** wait for completion. Existing guards (for example, `skipIfSuccessful` or "already running" checks) are applied when dispatching. If a run is skipped because a guard indicates it has already been handled, it is treated as handled for watermark advancement.

When `overlapPolicy` is `skip`, catch-up dispatches are subject to "already running" checks — if the previous catch-up run is still executing, subsequent missed intervals are skipped and the watermark advances past them. When `overlapPolicy` is `all`, all missed intervals are queued sequentially and execute one after another in chronological order.

### Run Identification

Catch-up runs must be distinguishable from normal scheduled runs everywhere they appear — UI, API, CLI, and logs.

**New trigger type.** Add `catchup` to the existing trigger type enum (`scheduler`, `manual`, `webhook`, `subdag`, `retry`). Catch-up dispatches use `TriggerTypeCatchUp` instead of `TriggerTypeScheduler`. This makes catch-up runs filterable in every surface without inspecting metadata.

**`scheduledTime` field.** The `scheduledTime` field (RFC 3339 timestamp) on the DAG run record and API schema is reserved for recording the cron slot a run was intended for. The field exists in the data model but is not yet populated — a proper mechanism to set it (at the scheduler level, not via CLI flags) is a future enhancement.

### DAG Identity and Manual Runs

Watermarks and run history are tracked by **DAG name**. Any run that uses the same DAG name (including inline-spec or manual runs) updates the per-DAG watermark and can affect catch-up for scheduled runs of that name.

Operational guidance:

- Ad-hoc/inline runs that should **not** influence catch-up must use a different DAG name.
- If the run is the same logical DAG, sharing the name is expected and correct.

### Distributed Scheduler / Failover

The scheduler watermark is tied to the directory lock (`dirLock`). Only the lock holder reads and writes it. When a new instance acquires the lock after a crash, it inherits whatever watermark the previous holder left — if the previous instance crashed without updating it, the new instance detects the gap and runs catch-up. This is the desired behavior.

## Behavior Matrix

| Scenario | No `catchupWindow` (default) | `overlapPolicy: skip` | `overlapPolicy: all` |
|----------|-------------------------------|------------------------|----------------------|
| First deploy (no prior runs) | Run from now only | Run from now only (new DAG gets `lastScheduledTime = now`, nothing to backfill) | Run from now only (new DAG gets `lastScheduledTime = now`, nothing to backfill) |
| Scheduler restart after 3h downtime | Jobs resume from now | Run the **first** missed interval; skip others while it runs | Run **all** missed intervals sequentially within `catchupWindow` |
| DAG disabled then re-enabled | Run from now only | Run the **first** missed interval within window | Backfill all missed runs within window |

## Safety Mechanisms

1. **Duplicate Prevention** — check if a dag-run already exists before dispatching
2. **Time Boundaries** — `catchupWindow` truncates the replay horizon
3. **Graceful Degradation** — missing watermark file = no catch-up (safe default)
4. **Dispatch Atomicity** — watermark advances per successful dispatch; failures leave it at the last successful time
5. **Scope Restriction** — only start schedules participate in catch-up

## Observability

Catch-up must never be silent. Users and operators need clear signals across every surface so they understand **what** the scheduler is doing, **why**, and **for which time slots**.

### Scheduler Logs

The scheduler emits structured log messages at each phase of catch-up processing:

**Catch-up start summary** (once per restart, only if catch-up work exists):
```
level=INFO msg="Catch-up started" dags_with_catchup=3 total_candidates=15 window_start="2026-02-07T09:00:00Z" window_end="2026-02-07T12:00:00Z"
```

**Per-DAG summary** (once per DAG that has catch-up work):
```
level=INFO msg="Catch-up planned" dag="hourly-etl" overlapPolicy="all" candidates=3 window="6h"
```

**Per-dispatch** (once per catch-up run dispatched):
```
level=INFO msg="Catch-up run dispatched" dag="hourly-etl" scheduled_time="2026-02-07T09:00:00Z" run_id="abc123"
```

**Skipped runs** (when guards suppress a candidate):
```
level=INFO msg="Catch-up run skipped" dag="hourly-etl" scheduled_time="2026-02-07T10:00:00Z" reason="already_exists"
```

Skip reasons: `already_exists` (duplicate prevention), `guard_blocked` (concurrency guard / overlapPolicy=skip).

**Catch-up completion summary** (once per restart):
```
level=INFO msg="Catch-up completed" dispatched=12 skipped=3 duration="1.2s"
```

If no DAGs have `catchupWindow` set or no intervals were missed, no catch-up log messages are emitted.

### Web UI

**Trigger badge in run list.** The DAG run table adds a trigger type badge column. Catch-up runs display a distinct "Catch-up" badge, visually differentiated from "Scheduled", "Manual", "Webhook", etc.

### API

The API surfaces catch-up metadata:

- `DAGRunSummary` includes `triggerType: "catchup"` (new enum value). The `scheduledTime` field is reserved for future use.
- The list runs endpoint supports filtering by trigger type: `?triggerType=catchup` returns only catch-up runs.

### CLI

**Catch-up dry-run preview.** `dagu catchup --dry-run <dag>` computes and displays what catch-up *would* dispatch without actually dispatching anything. Output is a table:

```
$ dagu catchup --dry-run hourly-etl
Catch-up preview for "hourly-etl" (overlapPolicy: all, window: 6h)

  Scheduled Time           Action
  2026-02-07T09:00:00Z     dispatch
  2026-02-07T10:00:00Z     dispatch
  2026-02-07T11:00:00Z     dispatch

3 runs would be dispatched.
```

This lets users verify catch-up behavior before adding `catchupWindow` to a DAG or after changing its configuration.

**Status display.** `dagu status` shows catch-up state when the scheduler is actively processing catch-up:

```
Scheduler: running (catch-up in progress: 5/15 dispatched, 2 skipped)
```

## Migration

1. **No breaking changes** — existing DAGs keep running unchanged.
2. **Default behavior preserved** — omitting `catchupWindow` means no catchup, matching current behavior.
3. **Opt-in** — users enable catch-up per DAG by setting `catchupWindow`.
4. **First startup** — if the watermark state file doesn't exist, it is initialized empty. New DAGs are tracked from the moment they are first observed (via filesystem watcher), preventing replay of ancient schedules.

## Examples

### Simple Daily Job (No Catch-up)

```yaml
name: cleanup
schedule: "0 3 * * *"
steps:
  - name: cleanup
    command: rm -rf /tmp/old-files
```

### Daily Report with Skip Policy

```yaml
name: daily-report
schedule: "0 9 * * *"
catchupWindow: "12h"
overlapPolicy: skip
steps:
  - name: generate
    command: python report.py
```

If the scheduler was down from 09:00–15:00, only the 09:00 run fires. The 12:00 run is skipped because the 09:00 run is still in progress (or already completed and the window only had one missed slot).

### Hourly ETL with Full Backfill

```yaml
name: hourly-etl
schedule: "0 * * * *"
catchupWindow: "3d"
overlapPolicy: all
steps:
  - name: etl
    command: python etl.py
```

All missed hourly intervals within 3 days are queued and executed sequentially in chronological order.

### Map-Form Schedule with Catch-up

```yaml
name: business-hours-etl
schedule:
  start: "0 * * * *"
  stop: "0 18 * * *"
catchupWindow: "6h"
overlapPolicy: all
steps:
  - name: etl
    command: python etl.py
```

Catch-up applies to the `start` schedule only. The `stop` schedule is unaffected.

### Observing Catch-up in Practice

This walkthrough shows what a user sees end-to-end when catch-up fires.

**Setup:** An hourly ETL DAG with catch-up enabled:

```yaml
name: hourly-etl
schedule: "0 * * * *"
catchupWindow: "6h"
overlapPolicy: all
steps:
  - name: etl
    command: python etl.py
```

**Scenario:** The scheduler goes down at 09:05 and restarts at 12:02. Three hourly slots were missed: 10:00, 11:00, 12:00.

**1. Scheduler logs on restart:**

```
level=INFO msg="Catch-up started" dags_with_catchup=1 total_candidates=3 window_start="2026-02-07T09:05:00Z" window_end="2026-02-07T12:02:00Z"
level=INFO msg="Catch-up planned" dag="hourly-etl" overlapPolicy="all" candidates=3 window="6h"
level=INFO msg="Catch-up run dispatched" dag="hourly-etl" scheduled_time="2026-02-07T10:00:00Z" run_id="run-a1b2"
level=INFO msg="Catch-up run dispatched" dag="hourly-etl" scheduled_time="2026-02-07T11:00:00Z" run_id="run-c3d4"
level=INFO msg="Catch-up run dispatched" dag="hourly-etl" scheduled_time="2026-02-07T12:00:00Z" run_id="run-e5f6"
level=INFO msg="Catch-up completed" dispatched=3 skipped=0 duration="0.3s"
```

**2. Web UI — run list for hourly-etl:**

| Run ID | Trigger | Started At | Status |
|--------|---------|------------|--------|
| run-a1b2 | **Catch-up** | 2026-02-07 12:02 | Success |
| run-c3d4 | **Catch-up** | 2026-02-07 12:02 | Success |
| run-e5f6 | **Catch-up** | 2026-02-07 12:02 | Success |
| run-g7h8 | Scheduled | 2026-02-07 13:00 | Success |

The "Catch-up" badge makes it immediately clear which runs were backfilled. The 13:00 run is a normal live-tick run with "Scheduled" badge.

**3. Dry-run preview** (before enabling catch-up, or after config changes):

```
$ dagu catchup --dry-run hourly-etl
Catch-up preview for "hourly-etl" (overlapPolicy: all, window: 6h)

  Scheduled Time           Action
  2026-02-07T10:00:00Z     dispatch
  2026-02-07T11:00:00Z     dispatch
  2026-02-07T12:00:00Z     dispatch

3 runs would be dispatched.
```

## Future Enhancements

1. **UI for Backfill** — manual backfill trigger from web UI
2. **CLI Backfill Command** — `dagu backfill --start 2026-01-01 --end 2026-02-01 my-dag`
3. **Partition Support** — Dagster-style partition definitions
4. **Watermark Inspection** — CLI/API to view and reset scheduler/DAG watermarks
5. **Catch-up Lag Metadata** — expose the catch-up gap duration as a status field for logging/alerting

## References

- [Airflow Catchup & Backfill](https://medium.com/nerd-for-tech/airflow-catchup-backfill-demystified-355def1b6f92)
- [Quartz Misfire Instructions](https://nurkiewicz.com/2012/04/quartz-scheduler-misfire-instructions.html)
- [Temporal Schedule](https://docs.temporal.io/schedule)
- [Kubernetes CronJob](https://docs.cloud.google.com/kubernetes-engine/docs/how-to/cronjobs)
- [Dagster Backfills](https://docs.dagster.io/concepts/partitions-schedules-sensors/backfills)
