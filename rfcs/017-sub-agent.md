---
id: "017"
title: "Sub-Agent Architecture"
status: draft
---

# RFC 017: Sub-Agent Architecture

## Summary

A `delegate` tool that lets the agent spawn focused child agents (sub-agents) for parallel sub-task execution. Each sub-agent runs in its own session, uses the same tools (minus `delegate`), and returns a summary to the parent. This is a prerequisite for skills, which will later define specialized sub-agent personas.

---

## Motivation

Currently:

1. **Single-threaded execution** — the agent processes everything sequentially in one loop. Complex tasks that naturally decompose into independent sub-tasks (e.g., "analyze configs, check resource limits, and review health checks") must be handled one at a time.
2. **No task delegation** — the agent cannot offload focused work to a separate context. Long tool-call chains for one sub-task pollute the main conversation context, reducing quality for subsequent work.
3. **No parallel work** — when multiple independent analyses are needed, the agent must complete each before starting the next, increasing latency.
4. **Prerequisite for skills** — skills (RFC 013) will define specialized sub-agent personas. Before skills can be activated at runtime, the sub-agent execution mechanism must exist.

### Use Cases

- A user asks the agent to "set up a complete CI/CD pipeline" — the agent delegates Dockerfile creation, GitHub Actions workflow, and deployment config to three parallel sub-agents.
- A user asks for a "comprehensive review of my DAGs" — the agent delegates review of each DAG to separate sub-agents, then synthesizes findings.
- A user asks to "migrate 5 DAGs from cron to event-driven" — each migration runs as a parallel sub-agent.

---

## Design

### Delegate Tool

A new tool named `delegate` is added to the agent's tool set.

**Input schema:**

```json
{
  "tasks": [
    {
      "task": "string (required) — description of the sub-task",
      "max_iterations": "integer (optional, default: 20) — max tool-call rounds"
    }
  ]
}
```

**Behavior:**

1. Creates a new sub-session linked to the parent session
2. Spawns a child `Loop` with the same provider, model, and system prompt
3. The child Loop receives the `task` as a user message
4. The child runs to completion (all tool-call rounds finish or max iterations reached)
5. Returns the last assistant response as a summary to the parent

**Tool result:** The tool result contains the sub-agent's summary text and a `delegate_id` that references the sub-session for on-demand history loading.

### Context and Tools

- **System prompt**: Sub-agent receives the same system prompt as the parent (including memory, environment info, etc.)
- **Conversation history**: Sub-agent does NOT receive the parent's conversation history. It starts fresh with only the system prompt and the delegated task.
- **Tools**: Sub-agent receives the same tools as the parent, minus `delegate`. This prevents recursion — only the root agent can delegate.

### Parallel Execution

When the LLM returns multiple `delegate` tool calls in a single response, they execute concurrently:

```text
Parent Loop
  ├─ LLM response: tool_calls: [
  │     delegate(tasks: [{task: "Analyze configs"}, {task: "Check resource limits"}, {task: "Review health checks"}])
  │  ]
  │
  ├─ Non-delegate tools execute sequentially (existing behavior)
  │
  ├─ Delegate tools execute in parallel:
  │   ┌──────────────┬──────────────┬──────────────┐
  │   │ Sub-agent A   │ Sub-agent B   │ Sub-agent C   │
  │   │ (sub-session) │ (sub-session) │ (sub-session) │
  │   └──────────────┴──────────────┴──────────────┘
  │   Parent waits for ALL to complete...
  │
  ├─ Returns 3 tool results (summaries) to LLM
  └─ Parent continues with combined results
```

**Limits:**
- Maximum 10 concurrent sub-agents per tool-call batch
- Excess delegate calls beyond 10 receive an error result
- Each sub-agent defaults to 20 max iterations (configurable per call)

### Sub-Session Storage

Each delegate invocation creates a separate session:

```go
Session{
    ID:              "<uuid>",
    UserID:          "<parent's user ID>",
    ParentSessionID: "<parent session ID>",  // Links to parent
    DelegateTask:    "<task description>",
}
```

- Sub-session messages are persisted via `SessionStore.AddMessage()`
- Sub-sessions are **excluded** from the main session listing (filtered by non-empty `ParentSessionID`)
- Sub-session history is loadable on demand via the existing `GET /api/v1/agent/sessions/{id}` endpoint

### Observability

**Parent session:**
- Tool call message shows `delegate` with the task description
- Tool result message carries a `delegate_id` field referencing the sub-session
- No real-time streaming of sub-agent messages — only status (running → finished)

**Frontend:**
- Delegate tool calls display as expandable blocks
- Collapsed: shows task description and summary
- Expanded: loads full sub-session history from API on demand

### Lifecycle

```text
1. Parent LLM returns delegate tool call(s)
2. For each delegate call:
   a. Generate sub-session ID (UUID)
   b. Create sub-session in SessionStore
   c. Build child Loop (same config, filtered tools)
   d. Queue task as user message
   e. Run child Loop.Go() — blocks until completion
   f. Capture last assistant response as summary
   g. Return ToolOut{Content: summary, DelegateID: subSessionID}
3. Parent records all tool results
4. Parent sends results to LLM for next response
```

---

## Configuration

No new configuration is required. The delegate tool is automatically available in the interactive chat agent. It is **not** available in DAG agent steps (`type: agent`), which explicitly list their tool set.

The `max_iterations` parameter on each delegate call controls sub-agent runtime. The global default is 20 iterations per sub-agent.

---

## Relationship to Existing Features

| Feature | Relationship |
|---------|-------------|
| **Agent Loop** | Sub-agent reuses the same `Loop` implementation |
| **Tools** | Sub-agent inherits parent's tools (minus delegate) |
| **Session Store** | Sub-sessions use the same storage mechanism |
| **Memory** | Sub-agent inherits memory via the shared system prompt |
| **DAG Agent Steps** | Delegate is NOT available in DAG agent steps |
| **Skills (future)** | Skills will customize sub-agent personas and tool sets |

---

## Future: Skills Integration

Once sub-agents are implemented, skills (RFC 013) can be activated by:

1. Loading skill knowledge into the sub-agent's system prompt
2. Filtering the sub-agent's tool set based on skill configuration
3. The `delegate` tool could accept an optional `skill` parameter to select a persona

This is explicitly out of scope for this RFC.

---

## Risks

1. **Cost amplification** — each sub-agent makes independent LLM calls. 10 parallel sub-agents could generate significant API costs. Mitigated by the max 10 limit and per-call iteration caps.
2. **Context isolation** — sub-agents don't see the parent's conversation, so they may duplicate work or miss relevant context. Mitigated by the task description serving as focused instructions.
3. **Error propagation** — a sub-agent failure returns an error result to the parent, which must decide how to proceed. The parent LLM handles this naturally.

---

## Out of Scope

- Recursive delegation (sub-agents spawning sub-agents)
- Skill-based persona customization for sub-agents
- Inter-sub-agent communication
- Sub-agent resource quotas beyond iteration limits
