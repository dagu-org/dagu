# Scheduler Refactoring: Unified Tick Planner

## Context

The catchup mechanism is broken because `CatchupManager.Init()` takes a static DAG snapshot at startup and never reacts to DAG lifecycle changes (add/update/delete). But this is a symptom of a deeper architectural flaw: **the scheduler has two parallel dispatch paths with different behaviors**. Fixing catchup properly requires unifying these paths.

## The Architectural Problem

The cronLoop currently runs:
```go
s.invokeJobs(ctx, tickTime)              // Path 1: live schedule → direct dispatch
s.catchupManager.ProcessBuffers(ctx)     // Path 2: catchup buffer → buffer dispatch
s.catchupManager.AdvanceWatermark(tickTime)
```

Two paths produce the same output (dispatched runs) but behave differently:

| | Path 1 (live) | Path 2 (catchup) |
|---|---|---|
| Tracks per-DAG watermark | No | Yes |
| Enforces overlap policy | No | Yes |
| Reacts to DAG changes | Yes | No |
| Checks guards (isRunning, skipIfSuccessful) | Yes (in DAGRunJob) | Partial (only isRunning) |

`RouteToBuffer` exists solely to bridge these paths — a code smell that reveals the split shouldn't exist.

## Design: Unified Tick Planner

Replace the dual-path with a single deep module — **TickPlanner** — that answers: *"What should I dispatch this tick?"*

### Architecture

```
              ┌──────────────────┐
              │    DAG Source     │  watches filesystem, emits events
              │  (EntryReader)   │
              └────────┬─────────┘
                       │ chan DAGChangeEvent
                       ▼
              ┌──────────────────┐
              │   Tick Planner   │  THE deep module
              │                  │
              │ • Cron eval      │  which DAGs are due this tick
              │ • Catchup detect │  which DAGs missed runs
              │ • Guard checks   │  isRunning, skipIfSuccessful, dedup
              │ • Overlap policy │  skip/all enforcement
              │ • Buffer mgmt   │  per-DAG FIFO queues
              │ • Watermark      │  per-DAG + global tracking
              └────────┬─────────┘
                       │ Plan(tick) → []PlannedRun
                       ▼
              ┌──────────────────┐
              │   Dispatcher     │  executes runs (local/distributed)
              │  (DAGExecutor)   │
              └──────────────────┘
```

### The cronLoop becomes trivial

```go
for tick := range ticks {
    runs := planner.Plan(ctx, tick)       // single call: what to dispatch?
    for _, run := range runs {
        go dispatch(ctx, run)             // fire-and-forget
    }
    planner.Advance(tick)                 // record progress
}
```

Stop/restart schedules stay in the existing direct-dispatch path (they don't participate in catchup per RFC 004).

### Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Plan() scope | Start schedules only | Matches RFC 004 scope. Stop/restart are fire-and-forget, no catchup needed. |
| Guard logic | Moves into Plan() | Centralizes ALL "should this run?" logic. Plan returns only actionable runs. |
| Event delivery | Channel-based | Decouples DAG Source from Planner. Events drained synchronously at tick start. |

## Module 1: DAG Source (refactored EntryReader)

### Responsibility
Know what DAGs exist. Watch filesystem. Emit lifecycle events on a channel.

### Interface Changes

Remove `Next(ctx, now)` — schedule evaluation moves to TickPlanner.

Add event channel:

```
Init(ctx) error
Start(ctx)
Stop()
DAGs() []*core.DAG
Events() <-chan DAGChangeEvent        // NEW: receive-only event channel
```

Remove `SetChangeHandler` — replaced by the channel.

### DAGChangeEvent

```go
type DAGChangeEvent struct {
    Type    DAGChangeType
    DAG     *core.DAG   // non-nil for Added/Updated
    DAGName string      // always set (needed for delete)
}

type DAGChangeType int
const (
    DAGChangeAdded   DAGChangeType = iota
    DAGChangeUpdated
    DAGChangeDeleted
)
```

### Start() emits events

On Create/Write: determine add vs update by checking registry existence before updating. Send event on channel.

On Rename/Remove: capture DAG name from registry before deleting. Send delete event on channel.

Events sent **after** releasing the registry lock (non-blocking send on buffered channel).

### Channel sizing

Buffer size = 256. This handles bursts (e.g., bulk DAG file copy) without blocking the file watcher. If the channel is full, log a warning and drop the event — the Planner will pick up the state on next init or the next file modification.

## Module 2: Tick Planner (replaces CatchupManager + schedule eval from EntryReader)

### Responsibility
Given the current time, determine which start-schedule runs should dispatch. Track progress. React to DAG changes.

### Interface

```go
type TickPlanner struct { ... }

// Init loads watermark state and computes catchup buffers for existing DAGs.
func (tp *TickPlanner) Init(ctx context.Context, dags []*core.DAG) error

// Plan drains queued DAG events, then returns ordered runs to dispatch this tick.
// Includes live scheduled runs and catchup runs. Only returns runs that pass
// all guards (not running, not finished, not skipped). The caller just dispatches.
func (tp *TickPlanner) Plan(ctx context.Context, now time.Time) []PlannedRun

// Advance records that this tick was processed. Updates global and per-DAG watermarks.
func (tp *TickPlanner) Advance(now time.Time)

// Flush persists watermark state to disk if dirty.
func (tp *TickPlanner) Flush(ctx context.Context)

// StartFlusher runs periodic watermark persistence. Blocks until ctx done.
func (tp *TickPlanner) StartFlusher(ctx context.Context)
```

### PlannedRun

```go
type PlannedRun struct {
    DAG           *core.DAG
    RunID         string
    ScheduledTime time.Time
    TriggerType   core.TriggerType  // TriggerTypeScheduler or TriggerTypeCatchUp
}
```

### Dependencies

The TickPlanner needs these to check guards:

```go
type TickPlannerConfig struct {
    WatermarkStore  WatermarkStore
    DAGStore        exec.DAGStore      // IsSuspended checks
    GetLatestStatus func(ctx, dag) (exec.DAGRunStatus, error)  // guard checks
    IsRunning       IsRunningFunc      // overlap policy
    GenRunID        RunIDFunc
    Dispatch        DispatchFunc       // for stop/restart passthrough (NOT for start)
    Clock           Clock
    Events          <-chan DAGChangeEvent  // from DAG Source
}
```

### Internal State

```go
type TickPlanner struct {
    // config (immutable after construction)
    cfg TickPlannerConfig

    // watermark state (protected by mu)
    mu             sync.RWMutex
    watermarkState *SchedulerState
    watermarkDirty atomic.Bool

    // per-DAG tracking (accessed only from cronLoop, no lock needed)
    entries map[string]*plannerEntry  // dagName → entry
    buffers map[string]*ScheduleBuffer

    // last plan result (for Advance to update watermarks)
    lastPlanResult []PlannedRun
}

type plannerEntry struct {
    dag       *core.DAG
    schedules []core.Schedule
}
```

**Key insight:** `entries` and `buffers` are only accessed from `Plan()` and `drainEvents()`, both called from the cronLoop goroutine. No lock needed. Only `watermarkState` needs the mutex (shared with the flusher goroutine).

### Plan() Algorithm

```
Plan(ctx, now):
  1. drainEvents(ctx)              // process queued DAG changes from channel
  2. candidates := []PlannedRun{}
  3. for each entry in entries:
       a. if suspended → skip
       b. if buffer exists and non-empty:
            peek front item
            if guards pass → pop, add to candidates as catchup run
            else if overlap=skip → pop and discard
            else (overlap=all) → leave in buffer
            if catchup item was produced → skip live eval (catchup has priority)
       c. evaluate cron schedules against now:
            for each schedule: if due:
              if guards pass → add to candidates as live run
  4. sort candidates by scheduledTime
  5. store as lastPlanResult (for Advance)
  6. return candidates
```

### Guard Checks (moved from DAGRunJob)

All three guards currently in `DAGRunJob.Start()` / `Ready()` / `skipIfSuccessful()` move into a single method:

```go
func (tp *TickPlanner) shouldRun(ctx context.Context, dag *core.DAG, scheduledTime time.Time, schedule cron.Schedule) bool
```

This checks:
1. **isRunning**: `tp.cfg.IsRunning(ctx, dag)` → skip if running (also used for overlap policy on buffer items)
2. **alreadyFinished**: `latestStatus.StartedAt >= scheduledTime` → skip (dedup)
3. **skipIfSuccessful**: if `dag.SkipIfSuccessful` and status == Succeeded, check if success was in the previous schedule window → skip

For catchup buffer items, only the isRunning check applies (overlap policy). The alreadyFinished and skipIfSuccessful checks don't apply to catchup because the scheduled times are historical.

### drainEvents() — Channel consumption

```
drainEvents(ctx):
  loop:
    select event from channel (non-blocking):
      Added:   entries[name] = new entry, set watermark to now
      Updated: entries[name] = updated entry, recompute buffer if catchupWindow > 0
      Deleted: delete entries[name], delete buffers[name], delete watermark[name]
    default: break loop
```

Non-blocking drain: process all available events, then return. This means DAG changes are reflected at the START of each tick, before Plan() evaluates anything. At most 1-minute delay between file change and planner awareness.

### Event Handling Details

**DAG Added:**
- Create `plannerEntry` with parsed schedules
- Set watermark `DAGs[name] = {LastScheduledTime: now}` (RFC: "new DAGs get lastScheduledTime = now")
- No buffer created (brand new, nothing to catch up)

**DAG Updated:**
- Replace `plannerEntry` with new DAG definition
- Remove existing buffer (if any)
- If `catchupWindow > 0`: recompute missed intervals from existing watermark, create new buffer
- If `catchupWindow <= 0`: no buffer (just removed)
- Watermark entry preserved (continuous tracking)

**DAG Deleted:**
- Remove `plannerEntry`
- Remove buffer (if any)
- Remove watermark entry
- Mark watermark dirty

### Advance() — Watermark tracking

```
Advance(now):
  mu.Lock()
  watermarkState.LastTick = now
  for each run in lastPlanResult:
    watermarkState.DAGs[run.DAG.Name] = {LastScheduledTime: run.ScheduledTime}
  watermarkDirty = true
  mu.Unlock()
  lastPlanResult = nil
```

**Critical:** Watermarks advance for ALL planned runs — both live and catchup. This fixes the bug where live dispatches never updated per-DAG watermarks.

### Flush() — unchanged from current CatchupManager

Snapshot under read lock, write atomically. 5-second periodic flush + final flush on shutdown.

## Module 3: Scheduler (simplified orchestrator)

### Start() sequence

```
1. entryReader.Init()                    // load DAGs from disk
2. planner.Init(entryReader.DAGs())      // load watermarks, compute catchup
3. go entryReader.Start()                // watch filesystem, emit events on channel
4. go planner.StartFlusher()             // periodic watermark persistence
5. cronLoop()                            // main loop
```

The channel was passed to planner at construction time via `TickPlannerConfig.Events`.

### cronLoop()

```go
func (s *Scheduler) cronLoop(ctx context.Context, sig chan os.Signal) {
    tickTime := s.clock().Truncate(time.Minute)
    timer := time.NewTimer(0)
    defer timer.Stop()

    for {
        select {
        case <-ctx.Done(): return
        case <-sig: return
        case <-s.quit: return
        case <-timer.C:
            _ = timer.Stop()

            // Start schedules: unified plan + dispatch
            for _, run := range s.planner.Plan(ctx, tickTime) {
                s.dispatchRun(ctx, run)
            }
            s.planner.Advance(tickTime)

            // Stop/restart schedules: direct dispatch (unchanged)
            s.invokeStopRestartJobs(ctx, tickTime)

            tickTime = s.NextTick(tickTime)
            timer.Reset(tickTime.Sub(s.clock()))
        }
    }
}
```

### invokeStopRestartJobs()

Extracted from the current `invokeJobs()` — handles only stop and restart schedule types. Uses `entryReader.Next()` filtered to non-start types. Or better: the DAG Source provides schedule data and this method evaluates stop/restart crons directly.

Actually, simpler: keep `entryReader.Next()` but it now only returns stop/restart jobs (since start evaluation moved to planner). Or remove `Next()` entirely and have the scheduler evaluate stop/restart crons from `DAGs()`.

**Recommended:** Keep a `StopRestartJobs(ctx, now)` method on DAG Source that returns only stop/restart scheduled jobs. This is a focused, narrow method.

### dispatchRun()

Simple goroutine wrapper:

```go
func (s *Scheduler) dispatchRun(ctx context.Context, run PlannedRun) {
    go func() {
        defer func() { /* panic recovery */ }()
        err := s.dagExecutor.HandleJob(ctx, run.DAG, OPERATION_START, run.RunID, run.TriggerType)
        if err != nil {
            logger.Error(ctx, "Failed to dispatch run", ...)
        }
    }()
}
```

No guard checks here — Plan() already filtered. Pure dispatch.

## What Gets Reused (unchanged)

| Component | File | Reuse |
|-----------|------|-------|
| `ScheduleBuffer` | `schedule_buffer.go` | Reused inside TickPlanner, unchanged |
| `ComputeReplayFrom()` | `catchup.go` | Pure function, unchanged |
| `ComputeMissedIntervals()` | `catchup.go` | Pure function, unchanged |
| `SchedulerState` / `DAGWatermark` | `watermark.go` | Types unchanged |
| `WatermarkStore` | `watermark.go` | Interface unchanged |
| `filewatermark.Store` | `persis/filewatermark/store.go` | Implementation unchanged |
| `DAGExecutor` | `dag_executor.go` | Dispatch mechanism unchanged |
| `QueueProcessor` | `queue_processor.go` | Queue execution unchanged |
| `ZombieDetector` | `zombie_detector.go` | Dead process cleanup unchanged |
| `HealthServer` | `health.go` | Health endpoint unchanged |
| `filenotify` | `filenotify/` | File watching unchanged |
| Cron parsing | `core/spec/schedule.go` | `robfig/cron` usage unchanged |

## What Changes

| Today | After |
|-------|-------|
| `CatchupManager` (static after init) | `TickPlanner` (reactive, event-driven) |
| `EntryReader.Next()` evaluates all schedule types | DAG Source: no schedule evaluation. Planner: evaluates start schedules. Scheduler: evaluates stop/restart. |
| `invokeJobs()` mixes eval + routing + dispatch + error handling | `Plan()` returns actionable runs. Scheduler just dispatches. |
| `RouteToBuffer` bridges live → buffer | Eliminated. Single path. |
| `ProcessBuffers()` separate from live dispatch | Merged into `Plan()`. |
| Per-DAG watermarks only for catchup dispatches | Per-DAG watermarks for ALL start dispatches via `Advance()`. |
| Guards in `DAGRunJob.Start()` / `Ready()` | Guards in `TickPlanner.shouldRun()` |
| `DAGRunJob` handles start + stop + restart | `DAGRunJob` handles only stop + restart. Start goes through Planner. |

## Files Affected

| File | Action |
|------|--------|
| `catchup_manager.go` | **Rename/evolve** → `tick_planner.go`. Absorb schedule eval + guard logic. Add `Plan()`, `Advance()`, `drainEvents()`, `shouldRun()`. Remove `RouteToBuffer`, `ProcessBuffers` (merged into Plan). |
| `entryreader.go` | **Simplify.** Remove `Next()`. Add `Events() <-chan DAGChangeEvent`. Add `StopRestartJobs(ctx, now)` for non-start schedules. Emit events in `Start()`. |
| `scheduler.go` | **Simplify.** Replace `invokeJobs` with plan+dispatch loop. Wire event channel at construction. Remove `RouteToBuffer` coordination. |
| `dagrunjob.go` | **Simplify.** Remove `Start()`, `Ready()`, `skipIfSuccessful()`, `PrevExecTime()` (moved to Planner). Keep `Stop()`, `Restart()`, `GetDAG()`, `String()`. |
| `catchup.go` | **Unchanged.** Pure computation functions. |
| `schedule_buffer.go` | **Unchanged.** Reused inside Planner. |
| `watermark.go` | **Unchanged.** Types stay the same. |
| `schedule.go` | **Unchanged.** ScheduleType enum. |
| `dag_executor.go` | **Unchanged.** Dispatch mechanism. |
| `catchup_manager_test.go` | **Rename/rewrite** → `tick_planner_test.go`. Test Plan(), drainEvents(), shouldRun(), Advance(). |
| `catchup_test.go` | **Unchanged.** Pure function tests. |
| `schedule_buffer_test.go` | **Unchanged.** Buffer tests. |
| `manger_test.go` | **Update.** Adapt to new DAG Source interface. |
| External callers: `cmd/context.go`, `internal/test/scheduler.go`, `internal/intg/distr/fixtures_test.go` | **Update** constructor wiring. Pass event channel instead of separate catchup manager config. |

## Edge Cases

### DAG Lifecycle

1. **New DAG with catchupWindow:** Watermark set to `now`. No catchup buffer (no missed runs). Future live ticks dispatch normally. On restart, catchup uses the watermark.

2. **New DAG without catchupWindow:** Watermark set to `now`. No buffer. Normal scheduling. If user later adds catchupWindow, the watermark is already there.

3. **DAG deleted while buffer draining:** drainEvents() removes entry + buffer. Remaining buffered items are discarded. In-flight dispatches (already spawned goroutines) complete or fail independently.

4. **DAG updated: catchupWindow 0→6h:** Buffer created with missed runs computed from existing watermark. Catches up from last known dispatch time.

5. **DAG updated: catchupWindow 6h→0:** Buffer removed. Remaining catchup items discarded. Live scheduling continues normally.

6. **DAG updated: schedule changed:** Old buffer removed. New buffer computed from new schedule + existing watermark. Only intervals matching the NEW cron expression are replayed (intentional — RFC §Cron changes during downtime).

7. **DAG updated: overlapPolicy changed:** New buffer uses new policy. Takes effect immediately for subsequent Plan() calls.

8. **DAG renamed (file rename):** fsnotify emits Remove(old) + Create(new). Planner processes Delete(oldName) + Added(newDAG). Old watermark lost; new watermark starts at `now`. Acceptable for rare operation.

9. **DAG name changed in YAML:** File is the same but DAG name differs. fsnotify emits Write. Planner processes Updated(newName). If the name changed, entries[oldName] still exists (keyed by... wait, entries need to be keyed by DAG name, but the event needs to carry the old name too for cleanup).

   **Resolution:** `plannerEntry` is keyed by DAG name (from `dag.Name`). On update, if the name changed, we effectively have a delete of old + add of new. The DAG Source should detect name changes and emit Delete(oldName) + Added(newDAG). The DAG Source can do this by comparing `dag.Name` before and after the load.

10. **Bulk file copy (many DAGs added at once):** Channel buffer absorbs burst (256 capacity). drainEvents() processes all at next tick start. At most 1 minute delay.

11. **Channel full:** If 256+ events queue between ticks (very unlikely — would need 256 file changes in <1 minute), overflow events are dropped. The dropped DAGs will be picked up on next modification or restart. Log a warning.

### Scheduling

12. **Catchup + live both due for same DAG:** Catchup has priority (buffer items checked first in Plan). Live tick is deferred until buffer drains. This ensures chronological ordering.

13. **Multiple schedules on same DAG:** Each schedule evaluated independently in Plan(). If two schedules are both due, both produce candidates. Sorted by time, both returned if guards pass.

14. **Suspended DAG with catchup buffer:** Plan() checks suspension first. If suspended, skip entirely (no buffer processing either). When un-suspended, buffer resumes from where it left off.

15. **DAG with SkipIfSuccessful and catchup:** For catchup items, skipIfSuccessful does NOT apply (they represent historical missed runs that need to execute). Only isRunning (overlap policy) applies to catchup items.

16. **Guard check race:** Plan() checks isRunning at plan time. Between Plan() and dispatch, the DAG could start running. This is the same race that exists today (DAGRunJob checks at dispatch time too). Acceptable — worst case is a duplicate run, which is caught by the "already running" check in the executor.

### Watermark

17. **Advance called with no planned runs:** Updates only `LastTick`. Per-DAG watermarks unchanged. Dirty flag set (LastTick changed). Correct — no dispatch means no per-DAG advancement.

18. **Dispatch fails after Plan returned it:** Advance() still updates the per-DAG watermark. This matches RFC semantics: "watermark tracks dispatch, not execution completion." The failed dispatch is not retried on next restart. Users should configure step-level retries.

19. **Scheduler crashes between Plan and Advance:** Some runs dispatched but watermark not advanced. On restart, those runs may be re-dispatched. At-least-once semantics. Same behavior as current CatchupManager.

20. **New DAG added, scheduler crashes immediately:** Watermark set to `now` in drainEvents, but not flushed yet (flusher runs every 5s). On restart, watermark file doesn't have the new DAG. Init() treats it as new → watermark set to restart time. No incorrect catchup. Correct.

21. **Watermark file missing/corrupt:** Init() creates empty state. No catchup. Same as current behavior.

## Verification

1. **Unit tests:** `go test -v -race ./internal/service/scheduler/... -count=1`
2. **Integration tests:** `go test -v -race ./internal/intg/... -count=1`
3. **Full suite:** `make test`
4. **Lint:** `make lint`
5. **Manual scenarios:**
   - Start scheduler, create new DAG with catchupWindow. Verify watermark set. Stop/restart. Verify no erroneous catchup.
   - Start scheduler with existing DAGs. Delete a DAG. Verify no dispatch attempts for deleted DAG.
   - Start scheduler, modify a DAG's schedule. Verify new schedule takes effect.
   - Stop scheduler for 2 hours. Restart. Verify correct catchup for DAGs with catchupWindow.
   - Run with `-race` flag throughout. Zero race conditions.
