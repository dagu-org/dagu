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

The scheduler advances by exactly 1 minute per tick in `cronLoop()` (`scheduler.go:282-305`). If the scheduler is offline for 2 hours, all jobs scheduled during that period are permanently lost. There is no state tracking of the last processed schedule time and no catch-up mechanism.

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

Extend the `schedule` field to support configurable catch-up windows:

```yaml
name: daily-etl
schedule:
  cron: "0 9 * * *"
  catchup: true                 # Opt into catch-up processing
  catchupWindow: "24h"          # Only backfill the last 24h of missed intervals
  misfire: runAll               # Policy for missed schedules
  maxCatchupRuns: 12            # Bound the number of executions per restart
```

#### Backward Compatibility

Existing simple format remains supported:

```yaml
# These are equivalent:
schedule: "0 9 * * *"

schedule:
  cron: "0 9 * * *"
  catchup: false    # Default: current behavior
```

### Schedule Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `cron` | string | required | Cron expression |
| `catchup` | bool | `false` | Opt-in for catch-up/backfill behaviour |
| `catchupWindow` | duration | `0` (disabled) | Lookback duration (e.g. `6h`, `24h`, `7d`) to search for missed intervals relative to `now` |
| `misfire` | string | `ignore` | Policy for missed schedules during downtime |
| `maxCatchupRuns` | int | `10` | Maximum runs per schedule when `misfire=runAll` |

> **Why no `start`/`end` fields?** Dagu already uses `schedule.start/stop/restart` to separate cron expressions by action. Reusing the same keys for ISO timestamps would break every existing DAG. Relative windows (Temporal-style) also avoid the timezone ambiguity and “first deploy in the past” issues that absolute timestamps brought.

### Misfire Policies

| Policy | Behavior | Use Case |
|--------|----------|----------|
| `ignore` | Skip missed runs (current behavior) | Real-time only jobs, transient data |
| `runOnce` | Execute once if any runs were missed | Most common - ensure job ran at least once |
| `runAll` | Execute all missed runs up to `maxCatchupRuns` | Data pipelines needing complete backfill |

### Behavior Matrix

| Scenario | `catchup=false` | `catchup=true` |
|----------|-----------------|----------------|
| First deploy | Run from now only | Backfill based on `catchupWindow` (e.g. last 24h) |
| Scheduler restart after 3h downtime | Jobs resume from now | Apply `misfire` + `catchupWindow` to recover the last 3h |
| DAG disabled then re-enabled | Run from now only | Backfill from the last successful run time, respecting `catchupWindow` |

### Catch-up Trigger Points

Catch-up execution begins any time the scheduler detects that real time has advanced beyond the last persisted tick watermark:

1. **Scheduler restarts / unplanned downtime** – when `Start()` loads `state.json` and sees a `LastTickTime` older than “now”, it processes missed intervals for every `catchup=true` schedule before resuming the live loop.
2. **DAG toggles (disable → enable)** – the per-DAG watermark uses the most recent dag-run start time. If a DAG was paused for hours and then re-enabled, its watermark lags behind and the next scheduler tick triggers catch-up within the configured window.
3. **Manual backfills while scheduler is down** – creating dag-run attempts advances the per-DAG watermark even if the scheduler wasn’t running at the time, so subsequent restarts only replay the gap after that manual activity.

No catch-up work happens while the scheduler is healthy and processing ticks in real time; the feature is purely for recovering gaps detected via those watermarks.

### Catch-up Window Boundaries

When a trigger condition is met, the detector calculates the earliest timestamp worth replaying as:

```
replayFrom = max(
    now - catchupWindow,         // user-configured lookback horizon
    state.LastTickTime,          // scheduler-wide watermark
    firstSeenAt(dag),            // first time this DAG was observed (from dag-metadata store)
    latestDagRunStartTime(dag),  // per-DAG watermark derived from history (zero if none)
)
```

This ensures:

- You never replay intervals older than the configured window (aligns with Temporal/K8s best practices).
- Brand-new DAGs are capped at their `firstSeenAt` timestamp, so simply enabling `catchup=true` never replays months of history unless the operator later issues an explicit backfill.
- DAGs that were paused or backfilled manually inherit the timestamp of their latest run, avoiding duplicate work while still recovering the missing gap after that execution.

## Technical Design

### 1. Scheduler State Persistence

Create new persistence layer to track scheduler state:

**Files:** `internal/persis/fileschedulerstate/state.go`, `internal/persis/filedagmeta/store.go`

Two related pieces:

1. **Scheduler watermark** – keep a lightweight `state.json` under `${cfg.Paths.DataDir}/scheduler/state.json` that records `LastTickTime`, `InstanceID`, and `UpdatedAt`. This is scheduler-owned and only written from the cron loop.
2. **DAG metadata store** – introduce a shared metadata file under `${cfg.Paths.DataDir}/scheduler/dags.json` managed by a new `filedagmeta` package. Each entry tracks `suspended`, `firstSeenAt`, and future attributes (`lastManualRun`, etc.). `exec.DAGStore.ToggleSuspend/IsSuspended` migrate from per-DAG flag files to this store (with legacy import on first access), while the scheduler reads the same metadata to determine `firstSeenAt`.

This split keeps scheduler-specific state isolated while giving all components (CLI, API, scheduler) a single source of truth for DAG metadata.

### 2. Core Types

- Introduce a `ScheduleConfig` struct on each parsed schedule entry that carries the cron expression plus `catchup`, `catchupWindow`, `misfire`, and `maxCatchupRuns`. This keeps all runtime decisions driven by structured data instead of raw YAML fragments.
- Define a `MisfirePolicy` enum (`ignore`, `runOnce`, `runAll`) so the scheduler can branch on typed constants.
- Keep the parsed `cron.Schedule` next to the config to avoid re-parsing expressions during catch-up detection.

### 3. Spec Parser

**File: `internal/core/spec/dag.go`**

Support both simple and extended schedule formats:

- Extend the `scheduleConfig` YAML struct with the new fields and keep the legacy string form for backward compatibility.
- `ScheduleValue` now yields either a simple cron expression or a structured object. During DAG loading, we convert the latter into `core.ScheduleConfig`, parsing `catchupWindow` via `time.ParseDuration` and validating `maxCatchupRuns`.
- Invalid inputs (bad duration strings, negative limits, unknown misfire policy) become build-time errors surfaced to the user before deployment.

### 4. Catch-up Detection

- Introduce a dedicated `CatchupDetector` that: (a) clamps the replay window to `now - catchupWindow`, (b) walks cron fire times between that bound and `now` in the configured timezone, and (c) filters out timestamps that already have successful dag-runs on record. It returns an ordered list of candidate times, leaving policy decisions to the scheduler.

### 5. Scheduler Integration

**File: `internal/service/scheduler/scheduler.go`**

- **Startup sequencing** – initialize the entry reader and load the last persisted scheduler state before processing catch-up, then take a snapshot of DAGs for deterministic replay.
- **Watermark selection** – compute the lower bound per DAG by taking the max of `state.LastTickTime`, `dagMetaStore.FirstSeenAt(dag)`, and the most recent dag-run start time from `DAGRunStore`. This covers brand-new deployments, re-enabled DAGs, and manual backfills uniformly.
- **Catch-up execution** – feed that lower bound into `CatchupDetector`, then apply the configured `misfire` policy plus `maxCatchupRuns`, a global cap (e.g., 100 runs per restart), and a small ticker-based rate limiter (~100 ms) to smooth the load.
- **State persistence** – advance `state.LastTickTime` only after each `invokeJobs` cycle succeeds, ensuring crashes trigger replays instead of silently skipping work.

### 6. Environment Variables for Catch-up Runs

Catch-up runs should have access to their scheduled time:

```go
// Set for catch-up runs
env["DAGU_SCHEDULED_TIME"] = scheduledTime.Format(time.RFC3339)
env["DAGU_IS_CATCHUP"] = "true"
```

This allows DAGs to process historical data correctly:

```yaml
steps:
  - name: process-data
    command: python etl.py --date=${DAGU_SCHEDULED_TIME}
```

## Files to Modify/Create

| File | Action | Description |
|------|--------|-------------|
| `internal/core/dag.go` | Modify | Add `ScheduleConfig`, `MisfirePolicy` types |
| `internal/core/spec/dag.go` | Modify | Add `scheduleConfig` spec, update parser |
| `internal/core/spec/types/schedule.go` | Create | `ScheduleValue` type for string/object parsing |
| `internal/persis/fileschedulerstate/state.go` | Create | Scheduler watermark persistence (`LastTickTime`, etc.) |
| `internal/persis/filedagmeta/store.go` | Create | Shared DAG metadata store (suspended, firstSeenAt, future attributes) |
| `internal/persis/filedag/store.go` | Modify | Move `ToggleSuspend`/`IsSuspended` to metadata store, migrate legacy flag files |
| `internal/service/scheduler/catchup.go` | Create | Catch-up detection logic |
| `internal/service/scheduler/scheduler.go` | Modify | Integrate state tracking and catch-up |
| `internal/service/scheduler/entryreader.go` | Modify | Add `GetAllDAGs()` method |

## Example Configurations

### 1. Simple Daily Job (No Catch-up)

```yaml
name: cleanup
schedule: "0 3 * * *"  # 3 AM daily
steps:
  - name: cleanup
    command: rm -rf /tmp/old-files
```

### 2. Daily Report with Run-Once Catch-up

```yaml
name: daily-report
schedule:
  cron: "0 9 * * *"
  catchup: true
  catchupWindow: "12h"
  misfire: runOnce
steps:
  - name: generate
    command: python report.py
```

### 3. Hourly ETL with Full Backfill

```yaml
name: hourly-etl
schedule:
  cron: "0 * * * *"
  catchup: true
  catchupWindow: "72h"
  misfire: runAll
  maxCatchupRuns: 48
steps:
  - name: etl
    command: python etl.py --hour=${DAGU_SCHEDULED_TIME}
```

### 4. Limited-Time Campaign

```yaml
name: promo-job
schedule:
  cron: "0 12 * * *"
  catchup: true
  catchupWindow: "30d"
  misfire: runAll
steps:
  - name: send-promo
    command: python send_promo.py
```

## Safety Mechanisms

1. **Rate Limiting**: 100ms delay between catch-up executions
2. **Global Limit**: Maximum 100 catch-up runs per scheduler restart
3. **Per-DAG Limit**: `maxCatchupRuns` (default 10) for `runAll` policy
4. **Duplicate Prevention**: Check if run already exists before executing
5. **Time Boundaries**: `catchupWindow` truncates replay horizon per schedule
6. **Graceful Degradation**: Missing state file = no catch-up (safe default)

## Migration

1. **No breaking changes** – Existing DAGs keep running; the new metadata store lazily imports any legacy `.suspend` flag files the first time it sees a DAG, then deletes the old artifact.
2. **Default behavior preserved** – `catchup: false` and `misfire: ignore` remain defaults, so nothing replays unless explicitly configured.
3. **Opt-in feature** – Users explicitly enable catch-up per DAG and can continue toggling suspension via CLI/API without caring about the underlying storage change.

## Testing Strategy

### Unit Tests

- `internal/core/spec/schedule_test.go` - Parse both simple and extended formats
- `internal/persis/fileschedulerstate/state_test.go` - State persistence
- `internal/service/scheduler/catchup_test.go` - Missed schedule detection

### Integration Tests

```go
func TestSchedulerCatchup(t *testing.T) {
    // 1. Create DAG with catchup=true, catchupWindow=1h
    // 2. Start scheduler
    // 3. Verify backfill runs were created
}

func TestMisfireRunOnce(t *testing.T) {
    // 1. Create state file with lastTickTime = 3 hours ago
    // 2. Create DAG with catchupWindow=24h, misfire=runOnce
    // 3. Start scheduler
    // 4. Verify exactly one catch-up run
}

func TestMisfireRunAll(t *testing.T) {
    // 1. Create state file with lastTickTime = 3 hours ago
    // 2. Create DAG with catchupWindow=24h, misfire=runAll, maxCatchupRuns=24
    // 3. Start scheduler with hourly DAG
    // 4. Verify 3 catch-up runs
}
```

### Manual Verification

```bash
# Create test DAG
cat > ~/.config/dagu/dags/test-catchup.yaml << 'EOF'
name: test-catchup
schedule:
  cron: "* * * * *"
  catchup: true
  catchupWindow: "10m"
  misfire: runOnce
steps:
  - name: log
    command: echo "ran at $(date), scheduled for ${DAGU_SCHEDULED_TIME}" >> /tmp/catchup.log
EOF

# Start scheduler, let run for 3 minutes
make run-scheduler
# Stop scheduler for 3 minutes
# Restart scheduler
# Check /tmp/catchup.log for catch-up execution
```

## Future Enhancements

1. **UI for Backfill**: Manual backfill trigger from web UI (like Airflow)
2. **CLI Backfill Command**: `dagu backfill --start 2026-01-01 --end 2026-02-01 my-dag`
3. **Partition Support**: Dagster-style partition definitions
4. **Catchup Progress**: Track and display catch-up progress in UI

## References

- [Airflow Catchup & Backfill](https://medium.com/nerd-for-tech/airflow-catchup-backfill-demystified-355def1b6f92)
- [Quartz Misfire Instructions](https://nurkiewicz.com/2012/04/quartz-scheduler-misfire-instructions.html)
- [Temporal Schedule](https://docs.temporal.io/schedule)
- [Kubernetes CronJob](https://docs.cloud.google.com/kubernetes-engine/docs/how-to/cronjobs)
- [Dagster Backfills](https://docs.dagster.io/concepts/partitions-schedules-sensors/backfills)
