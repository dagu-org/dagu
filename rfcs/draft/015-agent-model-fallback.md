# RFC 015: Agent Model Fallback

## Summary

Add a configurable model fallback priority to the agent settings. When the primary model fails (API error, rate limit, timeout), the agent automatically retries with the next model in the priority list. This applies to both the Web UI agent and the `type: agent` step (RFC 014).

---

## Motivation

Currently the agent configuration has a single `DefaultModelID`. If that model's API is unavailable — due to rate limits, outages, or transient errors — the agent fails entirely.

The `type: chat` step already supports model fallback via an ordered list of models in the YAML config. However, this capability does not exist at the agent level, where model configuration is managed globally through the Agent Settings UI.

A global fallback priority ensures:

- **Reliability**: Agent sessions and agent steps continue working when the primary model is down
- **Cost optimization**: Users can set an expensive model as primary and a cheaper model as fallback
- **Consistency**: One configuration governs both Web UI and step execution

---

## 1. Configuration

### Current Config

```go
type Config struct {
    Enabled        bool
    DefaultModelID string           // Single model
    ToolPolicy     ToolPolicyConfig
}
```

### Proposed Config

```go
type Config struct {
    Enabled        bool
    DefaultModelID string           // Primary model (unchanged, backwards compatible)
    FallbackModels []string         // Ordered list of fallback model IDs
    ToolPolicy     ToolPolicyConfig
}
```

`FallbackModels` is an ordered list of model IDs from the `ModelStore`. When the primary model (`DefaultModelID`) fails, the agent tries each fallback model in order until one succeeds or all are exhausted.

### Resolution Order

```
1. DefaultModelID        ← try first
2. FallbackModels[0]     ← first fallback
3. FallbackModels[1]     ← second fallback
...
N. FallbackModels[N-2]   ← last fallback
```

If all models fail, the error from the last attempted model is returned.

---

## 2. Agent Settings UI

The Agent Settings page adds a fallback configuration section below the default model selector:

### Model Priority List

A drag-and-drop ordered list showing the full resolution chain:

```
Model Priority
┌─────────────────────────────────────────┐
│ 1. Claude Sonnet 4 (default)        ★  │
│ 2. GPT-4o                           ⋮⋮ │
│ 3. Gemini Pro                       ⋮⋮ │
│                                         │
│ [+ Add fallback model]                  │
└─────────────────────────────────────────┘
```

- The default model is always position 1 (set via the existing default model selector)
- Fallback models are reorderable via drag-and-drop
- Each fallback can be removed individually
- Only models already configured in the Model Store can be added as fallbacks
- A model cannot appear more than once in the list

---

## 3. Fallback Behavior

### When to Fall Back

The agent falls back to the next model when a request fails with:

- HTTP 429 (rate limit)
- HTTP 500, 502, 503, 504 (server errors)
- Connection timeout or network errors
- Provider-specific transient errors

The agent does **not** fall back on:

- HTTP 400 (bad request — likely a prompt issue, not a model issue)
- HTTP 401/403 (authentication — next model likely has same issue if same provider)
- Context length exceeded (next model may have smaller context)
- Successful response with unexpected content

### Fallback Scope

Fallback applies **per LLM request**, not per session. Within a single agent session or step execution:

- Request 1: tries primary → fails → tries fallback A → succeeds
- Request 2: tries primary again → succeeds
- Request 3: tries primary → fails → tries fallback A → fails → tries fallback B → succeeds

Each new LLM request starts from the primary model. This avoids permanently degrading to a fallback when the primary model has a brief outage.

### Logging

Each fallback attempt is logged:

```
INFO  Attempting LLM request          model=claude-sonnet-4
WARN  LLM request failed              model=claude-sonnet-4 error="429 rate limited"
INFO  Falling back to next model      model=gpt-4o
INFO  Attempting LLM request          model=gpt-4o
```

---

## 4. Interaction with RFC 014 (Agent Step)

The agent step (`type: agent`) inherits the global fallback configuration automatically:

```yaml
steps:
  - name: analyze
    type: agent
    messages:
      - role: user
        content: "Analyze the logs"
```

This step uses `DefaultModelID` with the global `FallbackModels` chain.

When a step specifies `agent.model`, that model becomes the primary for that step, but the global fallback chain still applies for the remaining models:

```yaml
steps:
  - name: analyze
    type: agent
    agent:
      model: gpt-4o    # Override primary
    messages:
      - role: user
        content: "Analyze the logs"
```

Resolution for this step: `gpt-4o` → then `FallbackModels` (excluding `gpt-4o` if present).

---

## 5. API

### Get Config (existing endpoint, extended response)

```
GET /settings/agent

Response:
{
  "enabled": true,
  "defaultModelId": "claude-sonnet-4",
  "fallbackModels": ["gpt-4o", "gemini-pro"],
  "toolPolicy": { ... }
}
```

### Update Config (existing endpoint, extended body)

```
PATCH /settings/agent

{
  "fallbackModels": ["gpt-4o", "gemini-pro"]
}
```

Validation:
- All model IDs in `fallbackModels` must exist in the `ModelStore`
- No duplicates allowed
- `DefaultModelID` must not appear in `fallbackModels`

---

## 6. Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Inconsistent behavior across models | Log which model was used for each request; users can review in step logs or session history |
| Increased latency on fallback | Fail fast on non-retryable errors; only retry on transient failures |
| All fallbacks exhausted | Clear error message listing all attempted models and their errors |
| Stale fallback config (deleted model still in list) | Validate on save; skip missing models at runtime with a warning |
| Cost surprise from expensive fallback | Fallback order is explicit; cost tracking shows per-request model used |

---

## Key Files

| Purpose | File |
|---------|------|
| Agent config (add `FallbackModels`) | `internal/agent/store.go` |
| Agent config persistence | `internal/persis/fileagentconfig/` |
| Agent config API handler | `internal/service/frontend/api/v1/agent_config.go` |
| Provider cache (reuse for fallback providers) | `internal/agent/provider_cache.go` |
| Session manager (integrate fallback into loop) | `internal/agent/session.go` |
| Loop (add fallback retry logic) | `internal/agent/loop.go` |
| Chat step fallback (existing pattern to follow) | `internal/runtime/builtin/chat/executor.go` |
| Agent settings UI | `ui/src/pages/agent-settings/` |
| OpenAPI spec | `api/v1/api.yaml` |
