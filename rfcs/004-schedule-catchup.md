---
id: "004"
title: "Schedule Catch-up and Backfill"
status: draft
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
| **Temporal** | `catchup_window` + overlap policies | 1-year default catchup window, 6 overlap policies |
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

### New Schedule Configuration

Extend the `schedule` field to support configurable catch-up:

```yaml
name: daily-etl
schedule:
  cron: "0 9 * * *"
  catchup: all                 # Policy for missed schedules (enables catch-up)
  catchupWindow: "24h"         # Only backfill the last 24h of missed intervals
```

Setting `catchup` to any value other than `false` enables catch-up processing. There is no separate boolean toggle — the catchup policy is the single opt-in mechanism.

#### Backward Compatibility

Existing simple format remains supported:

```yaml
# These are equivalent:
schedule: "0 9 * * *"

schedule:
  cron: "0 9 * * *"
  # catchup defaults to false: current behavior, no catch-up
```

> **Why no `start`/`end` fields?** Dagu already uses `schedule.start/stop/restart` to separate cron expressions by action. Reusing the same keys for ISO timestamps would break every existing DAG. Relative windows (Temporal-style) also avoid the timezone ambiguity and "first deploy in the past" issues that absolute timestamps create.

### Schedule Format Reference

The `schedule` field accepts several shapes. The parser determines the form by inspecting the top-level YAML type and the keys present:

| Shape | Example | Meaning |
|-------|---------|---------|
| `schedule: "expr"` | `schedule: "0 9 * * *"` | Start schedule, no catch-up |
| `schedule: ["expr", ...]` | `schedule: ["0 9 * * *", "0 18 * * *"]` | Multiple start schedules, no catch-up |
| `schedule: {cron, catchup?, ...}` | `schedule: { cron: "0 9 * * *", catchup: all }` | Single start entry with catch-up |
| `schedule: [{cron, catchup?, ...}, ...]` | `schedule: [{ cron: "0 * * * *", catchup: all }, ...]` | Multiple start entries with catch-up |
| `schedule: {start, stop?, restart?}` | `schedule: { start: "0 9 * * *", stop: "0 18 * * *" }` | Typed schedules (`start` accepts catch-up) |

### Parser Disambiguation

When the `schedule` value is a **map**, the parser inspects its keys:

- If all keys are in `{cron, catchup, catchupWindow}` → **schedule-entry form** (a single start entry with optional catch-up fields).
- If all keys are in `{start, stop, restart}` → **typed-schedule form** (the existing map form).
- If keys from both sets are present → **validation error**.

When the `schedule` value is an **array**:

- If all items are strings → current behavior (multiple start cron expressions).
- If all items are objects containing a `cron` key → new schedule-entry form (multiple start entries with catch-up).
- Mixed string and object items → **validation error**.

In the **typed-schedule form**, the `start` value is polymorphic and accepts any of:

- `string` — a single cron expression (current behavior)
- `string[]` — multiple cron expressions (current behavior)
- `ScheduleEntry` — a single `{cron, catchup?, catchupWindow?}` object
- `ScheduleEntry[]` — multiple schedule entry objects

The `stop` and `restart` values remain `string | string[]` only. The parser rejects catch-up fields (`catchup`, `catchupWindow`) on `stop` or `restart` entries.

### Schedule Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `cron` | string | required | Cron expression |
| `catchup` | string | `false` | Policy for missed schedules: `false`/omitted (no catch-up), `latest` (latest missed only), `all` or `true` (all missed intervals) |
| `catchupWindow` | duration | `24h` | Lookback duration to search for missed intervals relative to `now`. Applied only when `catchup != false` |

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
- `0` or `0h` is invalid when `catchup != false`.

### Catchup Policies

| Policy | Behavior | Use Case |
|--------|----------|----------|
| `false` (default) | Skip missed runs (current behavior) | Real-time only jobs, transient data |
| `latest` | Execute the **latest** missed interval only | Most common — ensure most recent data is processed |
| `all` / `true` | Execute all missed runs (bounded by global caps) | Data pipelines needing complete backfill |

For `latest`, the selected run is the **latest** missed interval within the replay window. Example: if the scheduler was down from 09:00–12:00 and the cron is hourly, `latest` fires the 11:00 interval only. This is the most common choice when only the most recent data matters (e.g., dashboard refreshes, status reports).

### Scope

Catch-up applies only to **start** schedules. Stop and restart schedules are excluded:

- A missed **stop** is a no-op if the DAG is not currently running. Retroactively stopping a DAG that already completed naturally would be incorrect.
- A missed **restart** combines a stop and a start. The stop portion has the same issue, and the start portion is covered by catch-up on the start schedule.

If a DAG uses the map-form schedule (`schedule: { start: ..., stop: ..., restart: ... }`), only the `start` entry accepts catch-up fields. The parser rejects catch-up fields on `stop` or `restart` entries.

```yaml
# Typed schedule with catch-up on start
schedule:
  start:
    cron: "0 9 * * *"
    catchup: all
    catchupWindow: "6h"
  stop: "0 18 * * *"

# Multiple start entries + stop
schedule:
  start:
    - cron: "0 * * * *"
      catchup: all
    - cron: "30 9 * * *"
      catchup: latest
  stop: "0 18 * * *"

# Backward compatible: string values still work
schedule:
  start: "0 9 * * *"
  stop: "0 18 * * *"
```

### Multiple Schedules on the Same DAG

A DAG may have multiple cron expressions (array form):

```yaml
schedule:
  - cron: "0 * * * *"
    catchup: all
    catchupWindow: "6h"
  - cron: "30 9 * * *"
    catchup: latest
    catchupWindow: "24h"
```

Each schedule entry is evaluated independently during catch-up detection. Overlapping schedule times do **not** deduplicate—each schedule can produce its own run.

Cap ordering:

1. Apply catchup policy per schedule entry (e.g. `latest` keeps only the most recent candidate).
2. Merge runs **across schedule entries for the same DAG** ordered by scheduled time.
3. Apply `scheduler.maxCatchupRunsPerDAG` (default 20) across all entries for the same DAG.
4. Apply `scheduler.maxGlobalCatchupRuns` across all DAGs.

These caps are **per-restart budgets** applied to the candidate list computed from the current watermarks. After a partial failure, the next restart recomputes from the last persisted watermark and applies a fresh budget to the remaining candidates.

## Design

### Watermarks

Two watermarks determine what needs replaying:

1. **Scheduler watermark** — the last tick time the scheduler successfully dispatched, persisted to disk. On restart, the gap between this timestamp and `now` is the recovery window. If the watermark file is missing or corrupt, it is treated as `now` (no catch-up). The watermark is stored as a small JSON file under the scheduler data directory (e.g. `<dataDir>/scheduler/state.json`) and written atomically.
2. **Per-DAG watermark** — derived from the most recent dag-run start time. Handles DAGs that were disabled/re-enabled independently of the scheduler lifecycle. Manual/API/inline runs with the same DAG name also advance this watermark.

### Replay Boundaries

When catch-up is triggered, the earliest timestamp worth replaying is:

```
replayFrom = max(
    now - catchupWindow,         // user-configured lookback horizon
    schedulerWatermark,          // last tick the scheduler dispatched
    firstSeenAt(dag),            // first time this DAG was observed
    latestDagRunStartTime(dag),  // per-DAG watermark from history
)
```

This ensures:

- You never replay intervals older than the configured window.
- Brand-new DAGs with **no prior runs** get `firstSeenAt = now` on first observation, so catch-up never replays history that predates the DAG's existence.
- If a DAG has prior run history, `latestDagRunStartTime` supersedes `firstSeenAt` for replay boundaries.
- DAGs that were paused or backfilled manually inherit the timestamp of their latest run, avoiding duplicate work.

### Catch-up Trigger Points

1. **Scheduler restart** — scheduler watermark lags behind `now`, catch-up runs before the live loop starts.
2. **DAG re-enable** — per-DAG watermark lags behind `now`, catch-up runs inline on the next tick.
3. **Manual backfill while scheduler is down** — advances the per-DAG watermark, so the subsequent restart only replays the remaining gap.

No catch-up work happens while the scheduler is healthy and processing ticks in real time.

### Ordering Guarantees

Catch-up runs execute **synchronously before the live cron loop starts**:

1. Load scheduler watermark from disk (missing/corrupt = `now`).
2. Snapshot all DAGs.
3. Capture `catchupTo = now`.
4. Dispatch catch-up runs sequentially with rate limiting, advancing the watermark after each successful dispatch to the scheduled time.
5. If all catch-up dispatches succeed, set the watermark to `catchupTo`.
6. Enter the live loop.

This guarantees no live tick fires while catch-up is in progress.

For DAG re-enable events, catch-up runs are dispatched inline within the current tick, before any live-scheduled jobs. Existing duplicate-execution guards prevent conflicts if a catch-up run and a live tick target the same schedule time.

**Cron changes during downtime**: catch-up uses the **current** schedule expression from the snapshot. If a cron expression changed while the scheduler was down, missed intervals are computed against the new expression (not historical ones). This is a known limitation and a deliberate design choice to keep the scheduler stateless with respect to past cron definitions.

### Watermark Semantics

The scheduler watermark tracks **dispatch**, not execution completion. This matches the existing fire-and-forget pattern where jobs are launched asynchronously.

A catch-up run that fails at execution time will **not** be retried on next restart — the watermark has already moved past it. This is intentional: retrying failed runs is a separate concern (retry policies, alerting) and conflating it with catch-up would risk infinite retry loops. Users who need retry-on-failure should configure step-level retries within the DAG.

During catch-up, the watermark advances **per successful dispatch** to the scheduled time. If a catch-up run fails to **dispatch** (e.g. the persistence layer is unavailable), the watermark does not advance past that time and catch-up stops. On next restart the same interval will be retried, providing at-least-once dispatch semantics.

> A dispatch is considered **successful** when the DAG run is persisted to the queue store. Subsequent failure to reach the coordinator is handled by the queue retry mechanism and does not affect the watermark.

Catch-up dispatch is fire-and-forget; it does **not** wait for completion. Existing guards (for example, `skipIfSuccessful` or "already running" checks) are applied when dispatching. If a run is skipped because a guard indicates it has already been handled, it is treated as handled for watermark advancement.
As a result, `all` can yield concurrent executions **only when the DAG allows it**; if a concurrency guard blocks new starts, missed intervals will be skipped and the watermark will advance past them.

### Environment Variables

Catch-up runs expose their intended schedule time so DAGs can process historical data correctly:

| Variable | Value | Description |
|----------|-------|-------------|
| `DAGU_SCHEDULED_TIME` | RFC 3339 timestamp | The time this run was originally scheduled for |
| `DAGU_IS_CATCHUP` | `"true"` | Indicates this is a catch-up run, not a live run |

```yaml
steps:
  - name: etl
    command: python etl.py --date=${DAGU_SCHEDULED_TIME}
```

These environment variables are the **in-process** signals for the DAG's own steps. For **external** observability (operators monitoring the system), see the run identification fields and structured logs described in the [Observability](#observability) section.

### Run Identification

Catch-up runs must be distinguishable from normal scheduled runs everywhere they appear — UI, API, CLI, and logs.

**New trigger type.** Add `catchup` to the existing trigger type enum (`scheduler`, `manual`, `webhook`, `subdag`, `retry`). Catch-up dispatches use `TriggerTypeCatchUp` instead of `TriggerTypeScheduler`. This makes catch-up runs filterable in every surface without inspecting metadata.

**New `scheduledTime` field.** Add a `scheduledTime` field (RFC 3339 timestamp) to the DAG run record. This records the cron slot the run was intended for, distinct from `startedAt` (when it actually executed). The field is set for **all** scheduled and catch-up runs:

| Run type | `triggerType` | `scheduledTime` | `startedAt` |
|----------|---------------|-----------------|-------------|
| Live scheduled run | `scheduler` | `2026-02-07T09:00:00Z` | `2026-02-07T09:00:02Z` |
| Catch-up run | `catchup` | `2026-02-07T09:00:00Z` | `2026-02-07T12:15:03Z` |
| Manual run | `manual` | *(empty)* | `2026-02-07T14:30:00Z` |

For catch-up runs, the gap between `scheduledTime` and `startedAt` immediately tells the user how late the run is.

### Scheduler-Level Configuration

Operator-level guardrails independent of per-DAG settings:

| Setting | Default | Description |
|---------|---------|-------------|
| `scheduler.maxGlobalCatchupRuns` | `100` | Maximum total catch-up runs per scheduler restart |
| `scheduler.maxCatchupRunsPerDAG` | `20` | Maximum catch-up runs per DAG per scheduler restart |
| `scheduler.catchupRateLimit` | `100ms` | Delay between catch-up dispatches |

### DAG Metadata Store

The current per-DAG `.suspend` flag files are migrated to a per-DAG metadata store that also tracks `firstSeenAt`. Using per-DAG files (rather than a single shared file) preserves the natural atomicity of the current approach — concurrent writes from CLI, API, and scheduler never contend unless they target the same DAG.

Legacy `.suspend` flag files are lazily imported on first access and then removed.

### DAG Identity and Manual Runs

Watermarks and run history are tracked by **DAG name**. Any run that uses the same DAG name (including inline-spec or manual runs) updates the per-DAG watermark and can affect catch-up for scheduled runs of that name.

Operational guidance:

- Ad-hoc/inline runs that should **not** influence catch-up must use a different DAG name.
- If the run is the same logical DAG, sharing the name is expected and correct.

### Distributed Scheduler / Failover

The scheduler watermark is tied to the directory lock (`dirLock`). Only the lock holder reads and writes it. When a new instance acquires the lock after a crash, it inherits whatever watermark the previous holder left — if the previous instance crashed without updating it, the new instance detects the gap and runs catch-up. This is the desired behavior.

## Behavior Matrix

| Scenario | `catchup: false` (default) | `catchup: latest` | `catchup: all` |
|----------|----------------------------|-------------------|----------------|
| First deploy (no prior runs) | Run from now only | Run from now only (`firstSeenAt = now`, nothing to backfill) | Run from now only (`firstSeenAt = now`, nothing to backfill) |
| Scheduler restart after 3h downtime | Jobs resume from now | Run the **latest** missed interval within `catchupWindow` | Run **all** missed intervals within `catchupWindow` (bounded by caps) |
| DAG disabled then re-enabled | Run from now only | Run the **latest** missed interval since last dag-run within window | Backfill all missed runs since last dag-run (bounded by caps) |

## Safety Mechanisms

1. **Rate Limiting** — configurable delay between catch-up dispatches (default 100ms)
2. **Global Limit** — `scheduler.maxGlobalCatchupRuns` (default 100) caps total catch-up runs per restart
3. **Per-DAG Limit** — `scheduler.maxCatchupRunsPerDAG` (default 20) prevents one DAG from consuming the global budget
4. **Duplicate Prevention** — check if a dag-run already exists before dispatching
5. **Time Boundaries** — `catchupWindow` truncates the replay horizon
6. **Graceful Degradation** — missing watermark file = no catch-up (safe default)
7. **Dispatch Atomicity** — watermark advances per successful dispatch; failures leave it at the last successful time
8. **Scope Restriction** — only start schedules participate in catch-up

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
level=INFO msg="Catch-up planned" dag="hourly-etl" policy="all" candidates=3 window="6h"
```

**Per-dispatch** (once per catch-up run dispatched):
```
level=INFO msg="Catch-up run dispatched" dag="hourly-etl" scheduled_time="2026-02-07T09:00:00Z" run_id="abc123"
```

**Skipped runs** (when caps or guards suppress a candidate):
```
level=INFO msg="Catch-up run skipped" dag="hourly-etl" scheduled_time="2026-02-07T10:00:00Z" reason="cap_exceeded"
```

Skip reasons: `cap_exceeded` (per-DAG/global cap hit), `already_exists` (duplicate prevention), `guard_blocked` (concurrency guard).

**Catch-up completion summary** (once per restart):
```
level=INFO msg="Catch-up completed" dispatched=12 skipped=3 duration="1.2s"
```

If no DAGs have catch-up configured or no intervals were missed, no catch-up log messages are emitted.

### Web UI

**Trigger badge in run list.** The DAG run table adds a trigger type badge column. Catch-up runs display a distinct "Catch-up" badge, visually differentiated from "Scheduled", "Manual", "Webhook", etc.

**"Scheduled For" column.** A new column shows the `scheduledTime` — the intended cron slot. For live scheduled runs, this is approximately equal to "Started At". For catch-up runs, the gap between the two columns immediately communicates how late the run is.

**Catch-up banner.** When a DAG has recent catch-up runs (within the last hour), the DAG detail page shows an informational banner:

> 3 catch-up runs were dispatched for missed schedules between 09:00 and 12:00.

This banner is informational only and auto-dismisses after 1 hour.

### API

The API surfaces catch-up metadata:

- `DAGRunSummary` includes `triggerType: "catchup"` (new enum value) and `scheduledTime` (new RFC 3339 field).
- The list runs endpoint supports filtering by trigger type: `?triggerType=catchup` returns only catch-up runs.

### CLI

**Catch-up dry-run preview.** `dagu catchup --dry-run <dag>` computes and displays what catch-up *would* dispatch without actually dispatching anything. Output is a table:

```
$ dagu catchup --dry-run hourly-etl
Catch-up preview for "hourly-etl" (policy: all, window: 6h)

  Scheduled Time           Action
  2026-02-07T09:00:00Z     dispatch
  2026-02-07T10:00:00Z     dispatch
  2026-02-07T11:00:00Z     dispatch

3 runs would be dispatched.
```

This lets users verify catch-up behavior before enabling `catchup` on a DAG or after changing its configuration.

**Status display.** `dagu status` shows catch-up state when the scheduler is actively processing catch-up:

```
Scheduler: running (catch-up in progress: 5/15 dispatched, 2 skipped)
```

## Migration

1. **No breaking changes** — existing DAGs keep running unchanged.
2. **Default behavior preserved** — `catchup: false` is the default, so nothing replays unless explicitly configured.
3. **Opt-in** — users enable catch-up per DAG by setting `catchup`.
4. **Pre-existing DAGs** — on first startup after migration, DAGs with prior runs are bounded by the latest run time; DAGs with no runs get `firstSeenAt = now`, preventing replay of ancient schedules.
5. **Legacy flag files** — lazily imported into the new metadata store, then removed.

## Examples

### Simple Daily Job (No Catch-up)

```yaml
name: cleanup
schedule: "0 3 * * *"
steps:
  - name: cleanup
    command: rm -rf /tmp/old-files
```

### Daily Report with Latest

```yaml
name: daily-report
schedule:
  cron: "0 9 * * *"
  catchup: latest
  catchupWindow: "12h"
steps:
  - name: generate
    command: python report.py
```

### Hourly ETL with Full Backfill

```yaml
name: hourly-etl
schedule:
  cron: "0 * * * *"
  catchup: all
  catchupWindow: "3d"
steps:
  - name: etl
    command: python etl.py --hour=${DAGU_SCHEDULED_TIME}
```

### Multiple Schedules with Different Policies

```yaml
name: mixed-schedule
schedule:
  - cron: "0 * * * *"
    catchup: all
    catchupWindow: "6h"
  - cron: "30 9 * * *"
    catchup: latest
    catchupWindow: "1d"
steps:
  - name: process
    command: python process.py --time=${DAGU_SCHEDULED_TIME}
```

### Observing Catch-up in Practice

This walkthrough shows what a user sees end-to-end when catch-up fires.

**Setup:** An hourly ETL DAG with catch-up enabled:

```yaml
name: hourly-etl
schedule:
  cron: "0 * * * *"
  catchup: all
  catchupWindow: "6h"
steps:
  - name: etl
    command: python etl.py --hour=${DAGU_SCHEDULED_TIME}
```

**Scenario:** The scheduler goes down at 09:05 and restarts at 12:02. Three hourly slots were missed: 10:00, 11:00, 12:00.

**1. Scheduler logs on restart:**

```
level=INFO msg="Catch-up started" dags_with_catchup=1 total_candidates=3 window_start="2026-02-07T09:05:00Z" window_end="2026-02-07T12:02:00Z"
level=INFO msg="Catch-up planned" dag="hourly-etl" policy="all" candidates=3 window="6h"
level=INFO msg="Catch-up run dispatched" dag="hourly-etl" scheduled_time="2026-02-07T10:00:00Z" run_id="run-a1b2"
level=INFO msg="Catch-up run dispatched" dag="hourly-etl" scheduled_time="2026-02-07T11:00:00Z" run_id="run-c3d4"
level=INFO msg="Catch-up run dispatched" dag="hourly-etl" scheduled_time="2026-02-07T12:00:00Z" run_id="run-e5f6"
level=INFO msg="Catch-up completed" dispatched=3 skipped=0 duration="0.3s"
```

**2. Web UI — run list for hourly-etl:**

| Run ID | Trigger | Scheduled For | Started At | Status |
|--------|---------|---------------|------------|--------|
| run-a1b2 | **Catch-up** | 2026-02-07 10:00 | 2026-02-07 12:02 | Success |
| run-c3d4 | **Catch-up** | 2026-02-07 11:00 | 2026-02-07 12:02 | Success |
| run-e5f6 | **Catch-up** | 2026-02-07 12:00 | 2026-02-07 12:02 | Success |
| run-g7h8 | Scheduled | 2026-02-07 13:00 | 2026-02-07 13:00 | Success |

The "Catch-up" badge and the gap between "Scheduled For" and "Started At" make it immediately clear which runs were backfilled. The 13:00 run is a normal live-tick run with "Scheduled" badge.

**3. Inside each catch-up run**, the ETL step sees:
- `DAGU_SCHEDULED_TIME=2026-02-07T10:00:00Z` (its intended slot)
- `DAGU_IS_CATCHUP=true`

So `python etl.py --hour=2026-02-07T10:00:00Z` processes the correct historical hour.

**4. Dry-run preview** (before enabling catch-up, or after config changes):

```
$ dagu catchup --dry-run hourly-etl
Catch-up preview for "hourly-etl" (policy: all, window: 6h)

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
5. **Catch-up Lag Metadata** — expose the catch-up gap duration (env var or status field) for logging/alerting

## References

- [Airflow Catchup & Backfill](https://medium.com/nerd-for-tech/airflow-catchup-backfill-demystified-355def1b6f92)
- [Quartz Misfire Instructions](https://nurkiewicz.com/2012/04/quartz-scheduler-misfire-instructions.html)
- [Temporal Schedule](https://docs.temporal.io/schedule)
- [Kubernetes CronJob](https://docs.cloud.google.com/kubernetes-engine/docs/how-to/cronjobs)
- [Dagster Backfills](https://docs.dagster.io/concepts/partitions-schedules-sensors/backfills)
