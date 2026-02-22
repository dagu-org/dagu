---
id: "021"
title: "LLM API Cost Tracking"
status: draft
---

# RFC 021: LLM API Cost Tracking

## Summary

Add a cost tracking feature that persists per-session LLM costs and exposes a monthly per-user cost summary via a new API endpoint and UI page. Covers agent chat sessions and DAG-embedded LLM steps (`chat` and `agentstep` executors). Depends on RFC 022 (Agent Step Cost Persistence) for executor-level cost data. Reuses existing session and DAG run data — no new database or storage backend required.

---

## Motivation

The agent already captures per-message cost (`Message.Cost`) and token usage (`Message.Usage`) in session JSON files. `SessionManager.totalCost` accumulates cost in-memory during a session, but this value is lost on restart. Additionally, the `chat` executor stores token usage metadata in DAG run message files but never computes a USD cost. There is currently no way to:

1. **View aggregated costs** — administrators cannot see how much LLM usage costs per user or per month.
2. **Attribute costs** — there is no breakdown of which users are driving LLM spend.
3. **Recover session cost** — restarting the server loses the in-memory `totalCost`; the only way to recover it is to re-sum all message costs.
4. **Track DAG step LLM costs** — the `chat` executor records token counts but not USD cost; the `agentstep` executor doesn't persist cost at all (only logs to stderr). See RFC 022 for executor-level changes.

### Use Cases

- An **administrator** views the monthly cost dashboard to monitor LLM spend across all cost sources (agent chat, chat steps, agent steps).
- A **manager** reviews cost trends to decide whether to adjust model selection or usage policies.
- A **developer** checks their own usage to stay aware of personal LLM consumption.

---

## Proposal

### 1. Persist `TotalCost` in Session Files

Add a `TotalCost` field to `SessionForStorage` and `Session`:

```go
// internal/persis/filesession/store.go
type SessionForStorage struct {
    // ... existing fields ...
    TotalCost float64 `json:"total_cost,omitempty"`
}

// internal/agent/types.go
type Session struct {
    // ... existing fields ...
    TotalCost float64 `json:"total_cost,omitempty"`
}
```

Update `AddMessage()` in the file session store to accumulate cost:

```go
func (s *Store) AddMessage(_ context.Context, sessionID string, msg *agent.Message) error {
    // ... existing logic ...
    stored.Messages = append(stored.Messages, *msg)
    if msg.Cost != nil {
        stored.TotalCost += *msg.Cost
    }
    // ... write file ...
}
```

Update `ToSession()` and `FromSession()` to include `TotalCost`.

**Backward compatibility:** When loading old session files where `TotalCost` is zero but messages have costs, compute `TotalCost` by summing `Message.Cost` values. This ensures existing sessions are handled correctly without migration.

### 2. Executor-Level Cost Persistence (RFC 022)

RFC 022 handles adding `Cost` to `LLMMessageMetadata`, implementing `ChatMessageHandler` on the `agentstep` executor, and populating cost in the `chat` executor. This RFC assumes that work is complete — per-message cost data is available in DAG run message files for both executor types.

### 3. New API Endpoint

```
GET /api/v1/agent/cost-summary?month=YYYY-MM&userId=optional
```

**Response schema:**

```yaml
AgentCostSummary:
  type: object
  properties:
    month:
      type: string
      description: "The queried month (YYYY-MM)"
    entries:
      type: array
      items:
        $ref: "#/components/schemas/AgentCostEntry"
    totalCost:
      type: number
      format: double

AgentCostEntry:
  type: object
  properties:
    userId:
      type: string
    sessionCount:
      type: integer
    totalTokens:
      type: integer
      description: "Sum of input + output tokens"
    totalCost:
      type: number
      format: double
```

**Implementation approach:**

1. Add `ListUserIDs() []string` to the file session store (reads from the `byUser` in-memory index — no disk I/O).
2. Add a `GetCostSummary(ctx, month, userID)` method in `internal/agent/api.go` that:
   - Lists all user IDs (or filters to one if `userId` is specified).
   - For each user, lists sessions via `ListSessions`.
   - Filters sessions by `CreatedAt` month.
   - Sums `TotalCost` from the `Session` struct (with fallback: if `TotalCost` is zero, load messages and sum `Message.Cost`).
   - Aggregates token counts from `Message.Usage`.
   - Additionally scans DAG run message files for the queried month to include `chat` executor costs (see Section 2).
3. Add a handler in `internal/service/frontend/api/v1/agent_cost.go` wired to the Chi router.

**Cost source breakdown in response:**

The response includes a `source` field per entry to distinguish cost origins:

| Source | Description | Data Location |
|--------|-------------|---------------|
| `agent_chat` | Interactive agent chat sessions | Agent session files (`SessionStore`) |
| `chat_step` | DAG `chat` executor messages | DAG run message files (`{dag-run-dir}/messages/`) via `LLMMessageMetadata.Cost` (RFC 022) |
| `agent_step` | DAG `agentstep` executor messages | DAG run message files (same path as chat_step, via `ChatMessageHandler` — RFC 022) |

All three sources work identically in both local and shared-nothing worker modes. The `chat_step` and `agent_step` data flows through the existing `node.ChatMessages` → `DAGRunStatus` → `ReportStatus()` → `persistChatMessages()` pipeline. See RFC 022 for shared-nothing compatibility details.

### 4. Permission Model

Follows the audit log permission pattern:

| Role | Access |
|------|--------|
| Admin | All users' costs |
| Manager | All users' costs |
| Operator | Own costs only |
| Viewer | Own costs only |
| Developer | Own costs only |

The handler uses `requireManagerOrAbove` from the existing auth middleware. Non-manager users have the `userId` parameter forced to their own ID.

### 5. UI Page

A new page at route `/agent-cost` following the audit-logs page pattern:

- **Month picker** — defaults to current month, allows navigating previous months.
- **Table columns** — User, Sessions, Total Tokens, Total Cost (USD).
- **Totals row** — aggregate across all visible users.
- **Visibility** — shown in the sidebar under the "Operations" section, gated by `canViewAuditLogs` (reuses existing permission check).

**Files:**

| File | Change |
|------|--------|
| `ui/src/pages/agent-cost/index.tsx` | New page component |
| `ui/src/menu.tsx` | Add nav item under Operations |
| `ui/src/App.tsx` | Add route |
| `api/v1/api.yaml` | Add endpoint + schemas |

---

## Implementation Files

| File | Change |
|------|--------|
| `internal/agent/types.go` | Add `TotalCost` to `Session` |
| `internal/persis/filesession/store.go` | Add `TotalCost` to `SessionForStorage`, accumulate in `AddMessage`, backward-compat sum in `ToSession` |
| `internal/agent/store.go` | Add `ListUserIDs` to `SessionStore` interface |
| `internal/agent/api.go` | Add `GetCostSummary` method (aggregates sessions + DAG run messages) |
| `api/v1/api.yaml` | Add `/agent/cost-summary` endpoint + `AgentCostSummary` / `AgentCostEntry` schemas |
| `internal/service/frontend/api/v1/agent_cost.go` | New handler with permission check |
| `ui/src/pages/agent-cost/index.tsx` | New UI page |
| `ui/src/menu.tsx` | Add nav item |
| `ui/src/App.tsx` | Add route |

**Dependency:** RFC 022 covers executor-level changes (`messages.go`, `chat/executor.go`, `agentstep/executor.go`).

---

## Out of Scope

- **Budgets and alerts** — no spending limits or threshold notifications.
- **Real-time streaming** — cost summary is a point-in-time query, not SSE.
- **Export** — no CSV/JSON export of cost data.
- **Charts/graphs** — table only; visualization can be added later.
- **Per-model breakdown** — aggregated per-user, not split by model.
- **Cost forecasting** — no trend analysis or projection.
