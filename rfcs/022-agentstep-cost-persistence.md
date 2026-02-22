---
id: "022"
title: "Agent Step Cost Persistence"
status: draft
---

# RFC 022: Agent Step Cost Persistence

## Summary

Persist LLM cost and token usage data from `agentstep` executor runs into DAG run files by implementing the `ChatMessageHandler` interface. Also add a `Cost` field to `LLMMessageMetadata` so both `chat` and `agentstep` executors can record per-message USD cost.

## Motivation

The `agentstep` executor receives per-message cost (`agent.Message.Cost`) and token usage (`agent.Message.Usage`) via the `RecordMessage` callback, but only logs them to stderr. This data is lost after execution. The `chat` executor already persists messages via `ChatMessageHandler` → `node.ChatMessages` → DAG run files, but records token counts without USD cost (since `LLMMessageMetadata` has no `Cost` field).

RFC 021 (LLM Cost Tracking) needs per-message cost data from both executors to build the cost summary API. This RFC extracts the executor-level persistence changes as a standalone prerequisite.

## Proposal

### 1. Add `Cost` to `LLMMessageMetadata`

In `internal/core/exec/messages.go`:

```go
type LLMMessageMetadata struct {
    // ... existing fields ...
    Cost float64 `json:"cost,omitempty"` // USD cost for this API call
}
```

### 2. Implement `ChatMessageHandler` on `agentstep` Executor

In `internal/runtime/builtin/agentstep/executor.go`:

Add fields to `Executor`:

```go
type Executor struct {
    // ... existing fields ...
    contextMessages []exec.LLMMessage
    savedMessages   []exec.LLMMessage
}
```

Implement the interface (`internal/runtime/executor/executor.go:96`):

```go
func (e *Executor) SetContext(msgs []exec.LLMMessage) { e.contextMessages = msgs }
func (e *Executor) GetMessages() []exec.LLMMessage    { return e.savedMessages }
```

In the `RecordMessage` callback (line 215), convert `agent.Message` → `exec.LLMMessage`:

```go
RecordMessage: func(_ context.Context, msg agent.Message) {
    logMessage(stderr, msg)
    e.savedMessages = append(e.savedMessages, convertMessage(msg, modelCfg))
},
```

### 3. Message Type Mapping

| `agent.Message` field | `exec.LLMMessage` field |
|---|---|
| `Type` → `MessageTypeUser` | `Role` → `exec.RoleUser` |
| `Type` → `MessageTypeAssistant` | `Role` → `exec.RoleAssistant` |
| `Content` | `Content` |
| `ToolCalls` | `ToolCalls` (convert `llm.ToolCall` → `exec.ToolCall`) |
| `Usage.PromptTokens` | `Metadata.PromptTokens` |
| `Usage.CompletionTokens` | `Metadata.CompletionTokens` |
| `Usage.TotalTokens` | `Metadata.TotalTokens` |
| `Cost` | `Metadata.Cost` (new field) |
| model config (from `LoopConfig`) | `Metadata.Provider`, `Metadata.Model` |

### 4. Populate `Cost` in `chat` Executor

In `internal/runtime/builtin/chat/executor.go`, `createResponseMetadata()` (line 757) already builds `LLMMessageMetadata` with token counts. Once the `Cost` field exists, populate it from usage and model pricing.

## Shared-Nothing Worker Compatibility

No changes needed. `DAGRunStatusProto` is a JSON-serialized `DAGRunStatus` struct. `node.ChatMessages` is already part of this struct. The runtime's `ChatMessageHandler` capture (`internal/runtime/node.go:192-213`) already calls `SetContext` before execution and `GetMessages` after. Any executor implementing the interface gets persistence in both local and distributed modes automatically:

- Local: `node.ChatMessages` → `DAGRunStatus` → `WriteStepMessages()`
- Worker: `DAGRunStatus` → `ReportStatus()` gRPC → coordinator `persistChatMessages()`

## Referenced Files

| File | Change |
|------|--------|
| `internal/core/exec/messages.go:49` | Add `Cost` to `LLMMessageMetadata` |
| `internal/runtime/builtin/agentstep/executor.go:38-44` | Add `contextMessages`/`savedMessages` fields, implement `ChatMessageHandler` |
| `internal/runtime/builtin/agentstep/executor.go:215` | Convert `agent.Message` → `exec.LLMMessage` in `RecordMessage` callback |
| `internal/runtime/builtin/chat/executor.go:757-768` | Populate new `Cost` field in `createResponseMetadata()` |
| `internal/runtime/executor/executor.go:96` | `ChatMessageHandler` interface (no changes, already exists) |
| `internal/runtime/node.go:192-213` | Runtime capture logic (no changes, already works) |

## Out of Scope

- Cost aggregation, dashboard, or API endpoint (RFC 021)
- Changes to `agent.Loop` or `SessionManager` cost calculation
- Cost computation formula or model pricing tables
