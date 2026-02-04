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
  misfire: runAll               # Policy for missed schedules (enables catch-up)
  catchupWindow: "24h"          # Only backfill the last 24h of missed intervals
  maxCatchupRuns: 12            # Bound the number of executions per restart
```

Setting `misfire` to any value other than `ignore` enables catch-up processing. There is no separate boolean toggle — the misfire policy is the single opt-in mechanism.

#### Backward Compatibility

Existing simple format remains supported:

```yaml
# These are equivalent:
schedule: "0 9 * * *"

schedule:
  cron: "0 9 * * *"
  misfire: ignore    # Default: current behavior, no catch-up
```

> **Why no `start`/`end` fields?** Dagu already uses `schedule.start/stop/restart` to separate cron expressions by action. Reusing the same keys for ISO timestamps would break every existing DAG. Relative windows (Temporal-style) also avoid the timezone ambiguity and "first deploy in the past" issues that absolute timestamps create.

### Schedule Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `cron` | string | required | Cron expression |
| `misfire` | string | `ignore` | Policy for missed schedules. Any value other than `ignore` enables catch-up |
| `catchupWindow` | duration | `24h` | Lookback duration to search for missed intervals relative to `now`. Applied only when `misfire != ignore` |
| `maxCatchupRuns` | int | `10` | Maximum catch-up runs per schedule entry per scheduler restart. Applied only when `misfire = runAll` (runOnce always dispatches at most 1) |

#### Duration Format

The `catchupWindow` field uses a duration grammar compatible with Go’s `time.ParseDuration`, extended with a `d` suffix for days:

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
- `0` or `0h` is invalid when `misfire != ignore`.

### Misfire Policies

| Policy | Behavior | Use Case |
|--------|----------|----------|
| `ignore` | Skip missed runs (current behavior) | Real-time only jobs, transient data |
| `runOnce` | Execute once if any runs were missed (earliest missed interval) | Most common - ensure job ran at least once |
| `runAll` | Execute all missed runs up to `maxCatchupRuns` | Data pipelines needing complete backfill |

For `runOnce`, the selected run is the **earliest** missed interval within the replay window. Example: if the scheduler was down from 09:00–12:00 and the cron is hourly, `runOnce` fires the 09:00 interval only.
This choice prioritizes completeness over recency; users who want “most recent only” can narrow `catchupWindow` to a small interval.

### Scope

Catch-up applies only to **start** schedules. Stop and restart schedules are excluded:

- A missed **stop** is a no-op if the DAG is not currently running. Retroactively stopping a DAG that already completed naturally would be incorrect.
- A missed **restart** combines a stop and a start. The stop portion has the same issue, and the start portion is covered by catch-up on the start schedule.

If a DAG uses the map-form schedule (`schedule: { start: ..., stop: ..., restart: ... }`), only the `start` entry accepts catch-up fields. The parser rejects catch-up fields on `stop` or `restart` entries.

### Multiple Schedules on the Same DAG

A DAG may have multiple cron expressions (array form):

```yaml
schedule:
  - cron: "0 * * * *"
    misfire: runAll
    catchupWindow: "6h"
  - cron: "30 9 * * *"
    misfire: runOnce
    catchupWindow: "24h"
```

Each schedule entry is evaluated independently during catch-up detection. Overlapping schedule times do **not** deduplicate—each schedule can produce its own run.

Cap ordering:

1. Apply `maxCatchupRuns` per schedule entry.
2. Merge runs **across schedule entries for the same DAG** ordered by scheduled time.
3. Apply the global catch-up cap across all DAGs.

`maxCatchupRuns` is a **per-restart budget** applied to the candidate list computed from the current watermarks. After a partial failure, the next restart recomputes from the last persisted watermark and applies a fresh budget to the remaining candidates.

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

Catch-up dispatch is fire-and-forget; it does **not** wait for completion. Existing guards (for example, `skipIfSuccessful` or “already running” checks) are applied when dispatching. If a run is skipped because a guard indicates it has already been handled, it is treated as handled for watermark advancement.
As a result, `runAll` can yield concurrent executions **only when the DAG allows it**; if a concurrency guard blocks new starts, missed intervals will be skipped and the watermark will advance past them.

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

### Scheduler-Level Configuration

Operator-level guardrails independent of per-DAG settings:

| Setting | Default | Description |
|---------|---------|-------------|
| `scheduler.maxGlobalCatchupRuns` | `100` | Maximum total catch-up runs per scheduler restart |
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

| Scenario | `misfire=ignore` (default) | `misfire=runOnce` | `misfire=runAll` |
|----------|----------------------------|-------------------|------------------|
| First deploy (no prior runs) | Run from now only | Run from now only (`firstSeenAt = now`, nothing to backfill) | Run from now only (`firstSeenAt = now`, nothing to backfill) |
| Scheduler restart after 3h downtime | Jobs resume from now | Run the **earliest** missed interval within `catchupWindow` | Run **all** missed intervals within `catchupWindow` (bounded by caps) |
| DAG disabled then re-enabled | Run from now only | Backfill from the last dag-run start time (earliest missed within window) | Backfill all missed runs since last dag-run (bounded by caps) |

## Safety Mechanisms

1. **Rate Limiting** — configurable delay between catch-up dispatches (default 100ms)
2. **Global Limit** — configurable max total catch-up runs per restart (default 100)
3. **Per-Schedule Limit** — `maxCatchupRuns` (default 10) bounds `runAll` per schedule entry
4. **Duplicate Prevention** — check if a dag-run already exists before dispatching
5. **Time Boundaries** — `catchupWindow` truncates the replay horizon
6. **Graceful Degradation** — missing watermark file = no catch-up (safe default)
7. **Dispatch Atomicity** — watermark advances per successful dispatch; failures leave it at the last successful time
8. **Scope Restriction** — only start schedules participate in catch-up

## Migration

1. **No breaking changes** — existing DAGs keep running unchanged.
2. **Default behavior preserved** — `misfire: ignore` is the default, so nothing replays unless explicitly configured.
3. **Opt-in** — users enable catch-up per DAG by setting `misfire`.
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

### Daily Report with Run-Once

```yaml
name: daily-report
schedule:
  cron: "0 9 * * *"
  misfire: runOnce
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
  misfire: runAll
  catchupWindow: "3d"
  maxCatchupRuns: 48
steps:
  - name: etl
    command: python etl.py --hour=${DAGU_SCHEDULED_TIME}
```

### Multiple Schedules with Different Policies

```yaml
name: mixed-schedule
schedule:
  - cron: "0 * * * *"
    misfire: runAll
    catchupWindow: "6h"
    maxCatchupRuns: 6
  - cron: "30 9 * * *"
    misfire: runOnce
    catchupWindow: "1d"
steps:
  - name: process
    command: python process.py --time=${DAGU_SCHEDULED_TIME}
```

## Future Enhancements

1. **UI for Backfill** — manual backfill trigger from web UI
2. **CLI Backfill Command** — `dagu backfill --start 2026-01-01 --end 2026-02-01 my-dag`
3. **Dry-Run Preview** — `dagu catchup --dry-run my-dag`
4. **Partition Support** — Dagster-style partition definitions
5. **Catchup Progress** — track and display catch-up progress in UI
6. **Watermark Inspection** — CLI/API to view and reset scheduler/DAG watermarks
7. **Catch-up Lag Metadata** — expose the catch-up gap duration (env var or status field) for logging/alerting

## References

- [Airflow Catchup & Backfill](https://medium.com/nerd-for-tech/airflow-catchup-backfill-demystified-355def1b6f92)
- [Quartz Misfire Instructions](https://nurkiewicz.com/2012/04/quartz-scheduler-misfire-instructions.html)
- [Temporal Schedule](https://docs.temporal.io/schedule)
- [Kubernetes CronJob](https://docs.cloud.google.com/kubernetes-engine/docs/how-to/cronjobs)
- [Dagster Backfills](https://docs.dagster.io/concepts/partitions-schedules-sensors/backfills)
