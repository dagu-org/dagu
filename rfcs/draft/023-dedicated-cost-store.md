# RFC 023: Dedicated LLM Cost Store

## Summary

Introduce a dedicated, append-only cost store that records every LLM cost event independently from DAG run data and session data. Costs are organized per-session and per-run in a separate directory, surviving cleanup of the originating sessions and runs. Supersedes RFC 021's aggregation approach (scanning session/run files) with a purpose-built store.

---

## Motivation

LLM cost data is currently scattered across two locations:

1. **Agent session files** — `Message.Cost` (per-message USD) and `SessionManager.totalCost` (in-memory accumulator, lost on restart). Stored in `{dataDir}/agent/sessions/{userID}/{sessionID}.json`.
2. **DAG run message files** — `LLMMessageMetadata.Cost` within `node.ChatMessages`, stored in `{dag-run-dir}/messages/{stepname}.json`.

This creates several problems:

- **No single source of truth.** Aggregating monthly costs requires scanning every session file and every DAG run's message files.
- **Lifecycle coupling.** When sessions are cleaned up (`enforceMaxSessionsLocked`) or DAG runs are purged, cost data vanishes.
- **Slow queries.** A "costs this month" query must deserialize every session and step message file in the time range.
- **No independent tracking.** Cost data cannot exist outside the context of the object that generated it.

### Use Cases

- An **administrator** views monthly cost breakdown by user, DAG, or model.
- A **developer** checks their own LLM spend across sessions and DAG runs.
- Cost data persists after session cleanup or DAG run purging for accurate historical reporting.

---

## Proposal

### 1. Storage Layout

```
{dataDir}/costs/
  sessions/
    {sessionID}.jsonl          # agent_chat cost entries
  runs/
    {dagRunID}.jsonl           # chat_step + agent_step cost entries
```

Each file is append-only JSONL — one JSON line per LLM API call. This means:

- **Per-session/run lookup** is a single file read.
- **Monthly aggregation** scans all files and filters by timestamp (acceptable at moderate scale).
- **Retention cleanup** deletes files whose last modification time exceeds the configured retention period.
- **Costs survive** deletion of the originating session or DAG run.

### 2. Cost Entry

```go
// internal/service/cost/types.go

type Source string

const (
    SourceAgentChat Source = "agent_chat"  // Interactive agent sessions
    SourceChatStep  Source = "chat_step"   // DAG chat executor
    SourceAgentStep Source = "agent_step"  // DAG agentstep executor
)

type CostEntry struct {
    ID               string    `json:"id"`               // UUID
    Timestamp        time.Time `json:"timestamp"`         // UTC
    Source           Source    `json:"source"`
    UserID           string    `json:"userId"`
    DAGName          string    `json:"dagName,omitempty"`
    DAGRunID         string    `json:"dagRunId,omitempty"`
    StepName         string    `json:"stepName,omitempty"`
    SessionID        string    `json:"sessionId,omitempty"`
    Provider         string    `json:"provider"`
    Model            string    `json:"model"`
    PromptTokens     int       `json:"promptTokens"`
    CompletionTokens int       `json:"completionTokens"`
    TotalTokens      int       `json:"totalTokens"`
    Cost             float64   `json:"cost"`              // USD
}
```

One entry per LLM API call — matches the natural granularity of `agent.Message.Usage` and `exec.LLMMessageMetadata`.

### 3. Store Interface

```go
// internal/service/cost/store.go

type Store interface {
    // Record appends a cost entry to the appropriate file.
    // Writes to sessions/{sessionID}.jsonl or runs/{dagRunID}.jsonl.
    Record(ctx context.Context, entry *CostEntry) error

    // Summary scans cost files and returns aggregated totals.
    Summary(ctx context.Context, filter SummaryFilter) (*SummaryResult, error)

    // Close stops the background cleaner.
    Close() error
}

type SummaryGroupBy string

const (
    GroupByDay   SummaryGroupBy = "day"
    GroupByUser  SummaryGroupBy = "user"
    GroupByDAG   SummaryGroupBy = "dag"
    GroupByModel SummaryGroupBy = "model"
)

type SummaryFilter struct {
    StartTime time.Time       // Inclusive
    EndTime   time.Time       // Exclusive
    UserID    string          // empty = all
    DAGName   string          // empty = all
    GroupBy   SummaryGroupBy  // required
}

type SummaryBucket struct {
    Key              string  `json:"key"`
    TotalCost        float64 `json:"totalCost"`
    PromptTokens     int     `json:"promptTokens"`
    CompletionTokens int     `json:"completionTokens"`
    TotalTokens      int     `json:"totalTokens"`
    EntryCount       int     `json:"entryCount"`
}

type SummaryResult struct {
    Buckets   []SummaryBucket `json:"buckets"`
    TotalCost float64         `json:"totalCost"`
}
```

Two methods only: `Record` (append) and `Summary` (aggregate). GroupBy supports day, user, dag, model.

### 4. File-Based Implementation

New package `internal/persis/filecost/`, following the same conventions as `internal/persis/fileaudit/`:

- **Store struct**: `baseDir string`, `mu sync.Mutex`, `cleaner *cleaner`
- **Record**: Lock → determine file path from entry fields → `json.Marshal` → `os.OpenFile(O_APPEND|O_CREATE|O_WRONLY, 0640)` → write `data + '\n'`
- **File routing**: If `entry.SessionID != ""` → `sessions/{sessionID}.jsonl`, else → `runs/{dagRunID}.jsonl`
- **Summary**: `filepath.WalkDir` over both `sessions/` and `runs/`, scan each file line by line, filter by timestamp and other fields, aggregate into `map[string]*SummaryBucket` keyed by the GroupBy dimension
- **Cleaner**: Background goroutine (24h tick) walks both directories, deletes files with `ModTime` older than `cost_retention_days`. Modeled on `internal/persis/fileaudit/cleaner.go`.
- **No in-memory index**: Queries are time-range scans. At expected volume (hundreds of files, thousands of entries per file) this is fast enough. An index can be added later if needed.

### 5. Integration Points

#### 5a. Agent Chat Sessions

**File:** `internal/agent/session.go`, `createRecordMessageFunc()` (lines 586–613)

After cost calculation (lines 594–599), record to cost store:

```go
if cost > 0 && sm.costStore != nil {
    entry := &cost.CostEntry{
        Source: cost.SourceAgentChat, UserID: sm.userID, SessionID: sm.id,
        Provider: sm.provider, Model: sm.model, Cost: cost,
        PromptTokens: msg.Usage.PromptTokens,
        CompletionTokens: msg.Usage.CompletionTokens,
        TotalTokens: msg.Usage.TotalTokens,
    }
    if err := sm.costStore.Record(ctx, entry); err != nil {
        sm.logger.Warn("failed to record cost entry", "error", err)
    }
}
```

Add `costStore cost.Store` to `SessionManagerConfig` and `SessionManager`.

#### 5b. Agent Step Executor

**File:** `internal/runtime/builtin/agentstep/executor.go`, `RecordMessage` callback (line 230)

After `convertMessage()`, record assistant messages with cost. The cost store is obtained from the runtime context.

#### 5c. Chat Step Executor

**File:** `internal/runtime/builtin/chat/executor.go`, `createResponseMetadata()` (lines 757–768)

After building metadata with token counts, compute cost and record to cost store.

#### 5d. Server Wiring

**File:** `internal/service/frontend/server.go`

New `initCostStore(cfg)` function following the `initAuditService` pattern (lines 483–495):

```go
func initCostStore(cfg *config.Config) (*filecost.Store, error) {
    baseDir := filepath.Join(cfg.Paths.DataDir, "costs")
    return filecost.New(baseDir, cfg.Server.Cost.RetentionDays)
}
```

Thread to: `agent.APIConfig` → `SessionManager`, and runtime context for DAG step executors.

### 6. Configuration

```go
// internal/cmn/config/config.go
type CostConfig struct {
    RetentionDays int  // Default: 365; 0 = keep forever
}
```

```yaml
# config.yaml
cost:
  retention_days: 365
```

Config key: `cost.retention_days`, env: `COST_RETENTION_DAYS`, default: 365.

### 7. REST API

Add to `api/v1/api.yaml`:

```http
GET /api/v1/costs/summary?start=YYYY-MM-DDT00:00:00Z&end=YYYY-MM-DDT00:00:00Z&groupBy=day
```

**Parameters:**

| Param | Required | Description |
|-------|----------|-------------|
| `start` | Yes | Inclusive start (ISO 8601) |
| `end` | Yes | Exclusive end (ISO 8601) |
| `groupBy` | Yes | `day`, `user`, `dag`, or `model` |
| `userId` | No | Filter by user |
| `dagName` | No | Filter by DAG name |

**Response:**

```json
{
  "buckets": [
    {
      "key": "2026-02-01",
      "totalCost": 1.23,
      "promptTokens": 30000,
      "completionTokens": 15000,
      "totalTokens": 45000,
      "entryCount": 42
    }
  ],
  "totalCost": 15.67
}
```

**Handler:** `internal/service/frontend/api/v1/costs.go`

**RBAC:**

| Role | Access |
|------|--------|
| Admin | All cost data |
| Manager | All cost data |
| Operator | Own costs only (forced `userId` filter) |
| Developer | Own costs only |
| Viewer | No access |

---

## Relationship to RFC 021 and 022

- **RFC 022** (implemented): Added `Cost` to `LLMMessageMetadata` and `ChatMessageHandler` on the agentstep executor. No changes needed.
- **RFC 021**: This RFC **supersedes** RFC 021's aggregation approach. The cost summary API reads from the dedicated cost store instead of scanning session/run files. RFC 021's proposed `TotalCost` on `Session` remains useful for real-time session display but is not the source of truth for reporting.

---

## Implementation Files

### New Files

| File | Purpose |
|------|---------|
| `internal/service/cost/types.go` | `CostEntry`, `Source`, `SummaryFilter`, `SummaryResult`, `SummaryBucket` |
| `internal/service/cost/store.go` | `Store` interface |
| `internal/persis/filecost/store.go` | File-based store implementation |
| `internal/persis/filecost/cleaner.go` | Retention cleaner |
| `internal/persis/filecost/store_test.go` | Store tests |
| `internal/persis/filecost/cleaner_test.go` | Cleaner tests |
| `internal/service/frontend/api/v1/costs.go` | REST API handler |

### Modified Files

| File | Change |
|------|--------|
| `internal/cmn/config/config.go` | Add `CostConfig` struct |
| `internal/cmn/config/definition.go` | Add `Cost` definition field |
| `internal/cmn/config/loader.go` | Load `cost.retention_days`, set default |
| `internal/service/frontend/server.go` | Add `initCostStore()`, wire to agent API and runtime |
| `internal/agent/session.go` | Add `costStore` field, record in `createRecordMessageFunc()` |
| `internal/agent/api.go` | Thread `cost.Store` through `APIConfig` |
| `internal/runtime/builtin/agentstep/executor.go` | Record cost entries in `RecordMessage` callback |
| `internal/runtime/builtin/chat/executor.go` | Record cost entries in `createResponseMetadata()` |
| `api/v1/api.yaml` | Add `/costs/summary` endpoint and schemas |

### Implementation Order

1. Core types and store interface (`internal/service/cost/`)
2. File-based implementation and tests (`internal/persis/filecost/`)
3. Configuration (`CostConfig` in config loading)
4. Server wiring (`initCostStore()` in `server.go`)
5. Agent chat integration (cost store in `SessionManager`)
6. DAG step integration (cost store in agentstep + chat executors)
7. REST API (OpenAPI spec update + handler)

---

## Out of Scope

- **UI page** — cost dashboard can be added separately.
- **Budgets and alerts** — no spending limits or threshold notifications.
- **Per-message query API** — only summary aggregation for now.
- **Backfill CLI** — migrating historical cost data from session/run files.
- **Export** — no CSV/JSON export endpoint.
