---
id: "020"
title: "Agent Fallback via Model Array"
status: draft
supersedes: "015"
---

# RFC 020: Agent Fallback via Model Array

## Summary

Extend the `model` field to accept an ordered array of model IDs in both the agent config (chat modal) and the `type: agent` step. The first model is primary; the rest are tried in order on failure. No new `fallback` field — just a natural extension of the existing `model` field.

This mirrors the existing pattern in the `type: chat` executor, where `model` already accepts a string or array via `ModelValue`.

---

## Motivation

The agent feature (chat modal and agent step) currently accepts a single model. If that model's API is unavailable — rate limits, outages, timeouts — the agent fails entirely. The `type: chat` step already solves this with an ordered model array and sequential fallback. The agent should work the same way.

---

## Design

### 1. Agent Config (Chat Modal)

**Current:**

```go
// internal/agent/model_config.go
type Config struct {
    Enabled        bool
    DefaultModelID string           // single model ID
    ToolPolicy     ToolPolicyConfig
    EnabledSkills  []string
    SelectedSoulID string
}
```

**Proposed:**

```go
type Config struct {
    Enabled        bool
    DefaultModelID string           // primary model (backwards compatible)
    ModelIDs       []string         // ordered list: [primary, fallback1, fallback2, ...]
    ToolPolicy     ToolPolicyConfig
    EnabledSkills  []string
    SelectedSoulID string
}
```

**Resolution rule:** `ModelIDs` takes precedence when non-empty. If empty, fall back to `[DefaultModelID]`. This keeps backwards compatibility — existing configs with only `DefaultModelID` work unchanged.

```go
func (c *Config) ResolveModelIDs() []string {
    if len(c.ModelIDs) > 0 {
        return c.ModelIDs
    }
    if c.DefaultModelID != "" {
        return []string{c.DefaultModelID}
    }
    return nil
}
```

### 2. Agent Step Config

**Current:**

```go
// internal/core/step.go
type AgentStepConfig struct {
    Model         string  // single model ID
    // ...
}
```

**Proposed:**

```go
type AgentStepConfig struct {
    Model         string    // single model ID (backwards compatible)
    Models        []string  // ordered list: [primary, fallback1, ...]
    // ...
}
```

**Resolution rule:** Same pattern — `Models` takes precedence when non-empty, otherwise `[Model]`. If both are empty, inherit from global agent config.

**YAML:**

```yaml
# Single model (existing syntax, unchanged)
steps:
  - name: analyze
    type: agent
    agent:
      model: claude-sonnet-4

# Multiple models with fallback (new)
steps:
  - name: analyze
    type: agent
    agent:
      models:
        - claude-sonnet-4
        - gpt-4o
        - gemini-pro
```

### 3. Fallback Logic in the Loop

The `agent.Loop` currently takes a single `Provider` + `Model`. Change it to accept an ordered list of `(Provider, Model)` pairs and try them sequentially on each LLM request.

**Current `LoopConfig`:**

```go
type LoopConfig struct {
    Provider llm.Provider
    Model    string
    // ...
}
```

**Proposed `LoopConfig`:**

```go
type ModelSlot struct {
    Provider llm.Provider
    Model    string
    Name     string // human-readable name for logging
}

type LoopConfig struct {
    Models []ModelSlot // ordered: primary first, then fallbacks
    // ... (everything else unchanged)
}
```

**Fallback in `sendRequest`:**

```go
func (l *Loop) sendRequest(ctx context.Context) (*llm.ChatResponse, error) {
    history := l.copyHistory()
    messages := l.buildMessages(history)
    tools := l.buildToolDefinitions()

    var lastErr error
    for _, slot := range l.models {
        req := &llm.ChatRequest{
            Model:    slot.Model,
            Messages: messages,
            Tools:    tools,
        }

        l.setWorking(true)
        llmCtx, cancel := context.WithTimeout(ctx, llmRequestTimeout)
        resp, err := slot.Provider.Chat(llmCtx, req)
        cancel()

        if err == nil {
            l.accumulateUsage(resp.Usage)
            l.recordAssistantMessage(ctx, resp)
            return resp, nil
        }

        lastErr = err
        if !isFallbackEligible(err) {
            break // non-retryable error, don't try other models
        }

        l.logger.Warn("LLM request failed, trying next model",
            "model", slot.Name, "error", err)
    }

    l.recordErrorMessage(ctx, fmt.Sprintf("LLM request failed: %v", lastErr))
    l.setWorking(false)
    return nil, fmt.Errorf("LLM request failed: %w", lastErr)
}
```

Each new request starts from the primary model (no sticky fallback).

### 4. Fallback Eligibility

Fall back on transient/infrastructure errors only:

| Error | Fall back? |
|-------|-----------|
| HTTP 429 (rate limit) | Yes |
| HTTP 500, 502, 503, 504 | Yes |
| Connection timeout / network error | Yes |
| HTTP 400 (bad request) | No |
| HTTP 401/403 (auth) | No |
| Context length exceeded | No |

Reuse the existing `llm.APIError.Retryable` field — if retryable and all per-model retries are exhausted, fall back to next model.

### 5. Chat Modal API Integration

`resolveProvider` currently returns a single `(Provider, ModelConfig)`. Extend it to return a list:

```go
func (a *API) resolveProviders(ctx context.Context, modelIDs []string) ([]ModelSlot, *ModelConfig, error) {
    var slots []ModelSlot
    var primaryCfg *ModelConfig

    for i, id := range modelIDs {
        model, err := a.modelStore.GetByID(ctx, id)
        if err != nil {
            a.logger.Warn("Skipping unavailable model", "id", id, "error", err)
            continue
        }
        provider, _, err := a.providers.GetOrCreate(model.ToLLMConfig())
        if err != nil {
            a.logger.Warn("Skipping model, provider creation failed", "id", id, "error", err)
            continue
        }
        slots = append(slots, ModelSlot{
            Provider: provider,
            Model:    model.Model,
            Name:     model.Name,
        })
        if i == 0 || primaryCfg == nil {
            primaryCfg = model
        }
    }

    if len(slots) == 0 {
        return nil, nil, errors.New("no usable models configured")
    }
    return slots, primaryCfg, nil
}
```

`CreateSession` and `SendMessage` pass the full slot list to `SessionManager`, which passes it to the `Loop`.

### 6. Agent Step Integration

In `agentstep/executor.go`, resolve the model list from step config → global config:

```go
// Resolve ordered model IDs: step override → global config
modelIDs := resolveModelIDs(stepCfg, agentCfg)
if len(modelIDs) == 0 {
    return fmt.Errorf("no model configured")
}

var slots []agent.ModelSlot
for _, id := range modelIDs {
    cfg, err := modelStore.GetByID(ctx, id)
    if err != nil {
        logf(stderr, "Warning: model %q not found, skipping", id)
        continue
    }
    provider, err := agent.CreateLLMProvider(cfg.ToLLMConfig())
    if err != nil {
        logf(stderr, "Warning: provider for %q failed, skipping", id)
        continue
    }
    slots = append(slots, agent.ModelSlot{
        Provider: provider, Model: cfg.Model, Name: cfg.Name,
    })
}
```

### 7. Agent Settings UI

The model selector changes from a single dropdown to an ordered list:

```
Model Priority
┌─────────────────────────────────────────┐
│ 1. Claude Sonnet 4                  ⋮⋮  │
│ 2. GPT-4o                           ⋮⋮  │
│                                          │
│ [+ Add model]                            │
└──────────────────────────────────────────┘
```

- Drag-and-drop reordering
- First item is the primary model
- Models are selected from the existing model store
- No duplicates allowed
- Minimum 1 model required

### 8. API Changes

**Agent config response** (extend existing endpoint):

```json
{
  "enabled": true,
  "defaultModelId": "claude-sonnet-4",
  "modelIds": ["claude-sonnet-4", "gpt-4o"],
  "toolPolicy": {}
}
```

`defaultModelId` stays for backwards compat; `modelIds` is the source of truth when present.

---

## Logging

Each fallback attempt is logged to the step stderr (agent step) or session messages (chat modal):

```
[agent] Starting (models: [Claude Sonnet 4, GPT-4o], tools: 8)
[agent] LLM request failed (model: Claude Sonnet 4): 429 rate limited
[agent] Falling back to GPT-4o
[agent] LLM request succeeded (model: GPT-4o)
```

---

## Key Files

| Purpose | File |
|---------|------|
| Agent config (add `ModelIDs`) | `internal/agent/model_config.go` |
| Agent config persistence | `internal/persis/fileagentconfig/` |
| Agent config API handler | `internal/service/frontend/api/v1/agent_config.go` |
| Loop (multi-model fallback) | `internal/agent/loop.go` |
| Session manager (pass model slots) | `internal/agent/session.go` |
| Agent API (resolve multiple providers) | `internal/agent/api.go` |
| Provider cache (reuse for all models) | `internal/agent/provider_cache.go` |
| Agent step executor | `internal/runtime/builtin/agentstep/executor.go` |
| Agent step spec (add `Models` field) | `internal/core/spec/step.go`, `internal/core/step.go` |
| Chat step (existing pattern reference) | `internal/runtime/builtin/chat/executor.go` |
| Agent settings UI | `ui/src/pages/agent-settings/` |
| OpenAPI spec | `api/v1/api.yaml` |
