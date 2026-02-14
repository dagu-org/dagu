---
id: "014"
title: "Agent Step Type"
status: draft
---

# RFC 014: Agent Step Type

## Summary

Introduce a new `type: agent` step that runs the Tsumugi AI agent within a DAG workflow. This enables agentic LLM execution — multi-turn tool calling with built-in tools (bash, file I/O, web search, reasoning) — as a first-class workflow step, bringing the same capabilities currently available only through the Web UI into automated pipelines.

The existing `type: chat` step remains unchanged for simple, single-purpose LLM calls with DAG-based tools.

---

## Motivation

### Current State

The AI agent Tsumugi operates exclusively through the Web UI. Users interact via a chat modal, and the agent executes tools (bash commands, file reads/edits, web search) in real time with SSE-based streaming.

The existing `type: chat` step provides basic LLM interaction within DAGs, but it lacks the agent's core capabilities:

| Capability | Web UI Agent | `type: chat` |
|------------|:---:|:---:|
| Built-in tools (bash, read, patch) | Yes | No |
| Persistent memory (global + per-DAG) | Yes | No |
| Rich system prompt with environment context | Yes | No |
| Tool policy enforcement | Yes | No |
| Multi-turn agentic loop | Yes | Limited |
| DAG-based tool calling | No | Yes |

### Use Cases

- **Automated code review**: An agent step reads changed files, analyzes them, and writes a review summary to an output variable for downstream steps.
- **Infrastructure management**: An agent investigates system state via bash commands and takes corrective actions based on findings.
- **Report generation**: An agent reads data files, reasons about patterns, and generates a structured report written to disk.
- **Incident response**: A scheduled DAG triggers an agent step to check service health, read logs, diagnose issues, and patch configuration files.
- **DAG self-maintenance**: An agent step validates and updates other DAG definitions based on changing requirements.

---

## 1. DAG Schema

### Minimal Example

```yaml
steps:
  - name: analyze-logs
    type: agent
    messages:
      - role: user
        content: |
          Analyze the error logs at /var/log/app/errors.log from the last hour.
          Summarize the root causes and suggest fixes.
    output: ANALYSIS_RESULT
```

The agent uses the globally configured default model (set in Agent Settings via the Web UI). No per-step model configuration is needed.

### Full Example

```yaml
steps:
  - name: fix-config
    type: agent
    agent:
      model: claude-sonnet            # Optional: override the global default model
      tools:
        enabled:
          - bash
          - read
          - patch
          - think
          - web_search
        bash_policy:
          default_behavior: allow
          deny_behavior: hitl       # Pause for HITL approval when denied
          rules:
            - name: allow-read-commands
              pattern: "^(cat|head|tail|grep|find|ls)\\b"
              action: allow
            - name: deny-destructive
              pattern: "^(rm|chmod|chown|mkfs)\\b"
              action: deny
      memory:
        enabled: true               # Load global + DAG memory into context
      prompt: |
        Focus only on the config files in /etc/app/.
        Do not modify any files outside that directory.
      max_iterations: 30            # Max tool call rounds (default: 50)
      safe_mode: true               # Enable command approval via HITL
    messages:
      - role: user
        content: |
          The config at /etc/app/config.yaml has an invalid database_url.
          Read the file, fix the URL to point to ${DB_HOST}, and validate.
    output: FIX_RESULT
    timeout_sec: 600
```

---

## 2. Configuration Reference

### `agent` Block

The `agent` block is optional. When omitted entirely, the step uses all defaults (global default model, all tools enabled, no memory, no safe mode).

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | global default | Model ID from the globally configured agent models (Agent Settings). Overrides the default model for this step only. |
| `tools` | object | all enabled | Tool selection and policy. See [Section 3](#3-tools). |
| `memory` | object | `{ enabled: false }` | Memory configuration. See [Section 4](#4-memory). |
| `prompt` | string | — | Additional instructions appended to the built-in Tsumugi system prompt. Use this to constrain scope, set conventions, or provide task-specific context. |
| `max_iterations` | int | 50 | Maximum tool call rounds before the agent stops. |
| `safe_mode` | bool | true | Enable command approval. See [Section 5](#5-permission--security). |

### Model Resolution

The agent step uses the same model configuration managed through the Agent Settings page in the Web UI:

1. If `model` is specified, look up that model from the global `ModelStore`
2. If `model` is omitted, use the global default model (`ConfigStore.DefaultModelID`)
3. If no default model is configured, the step fails with a clear error

This ensures a single source of truth for model configuration (provider, API key, pricing, etc.) and avoids duplicating credentials across DAG files.

### `messages` Block

Uses the same schema as `type: chat` messages. Content supports variable substitution (`${VAR}`, `$1`, etc.).

```yaml
messages:
  - role: user
    content: "Analyze ${INPUT_FILE} and write results to ${OUTPUT_DIR}"
```

Messages from prior chat steps (via `ChatMessageHandler`) are passed as context, enabling multi-step agent chains.

### `output`

Standard step output capture. The agent's **final assistant response** (the last message with no tool calls) is written to the output variable.

---

## 3. Tools

### Available Tools

| Tool | Default | Description |
|------|---------|-------------|
| `bash` | enabled | Execute shell commands with timeout enforcement |
| `read` | enabled | Read file contents |
| `patch` | enabled | Create/edit files via unified diff |
| `think` | enabled | Internal reasoning (no side effects) |
| `read_schema` | enabled | DAG YAML schema reference |
| `web_search` | enabled | Search the web |

### Excluded Tools

| Tool | Reason | Alternative |
|------|--------|-------------|
| `navigate` | Requires Web UI — emits UIAction events with no consumer | N/A (no UI in step context) |
| `ask_user` | Requires Web UI for interactive prompts | Use HITL integration for approvals |

### Tool Selection

Users can restrict which tools are available:

```yaml
agent:
  tools:
    enabled:
      - bash
      - read
      - think
    # Only the listed tools are available; all others are disabled
```

When `tools.enabled` is omitted, all available tools (excluding `navigate` and `ask_user`) are enabled by default.

### Bash Policy

The bash policy follows the same schema as the Web UI agent's `BashPolicyConfig`:

```yaml
agent:
  tools:
    bash_policy:
      default_behavior: allow       # allow | deny
      deny_behavior: hitl           # hitl | block
      rules:
        - name: rule-name
          pattern: "regex"
          action: allow | deny
```

| `deny_behavior` | Effect when a command is denied |
|------------------|-------------------------------|
| `block` | Command is rejected; agent receives an error and can try alternatives |
| `hitl` | Step pauses and waits for human approval via the HITL mechanism |

---

## 4. Memory

When memory is enabled, the agent step loads persistent memory into its system prompt context:

```yaml
agent:
  memory:
    enabled: true
```

- **Global memory**: Loaded from `{memory_dir}/MEMORY.md`
- **Per-DAG memory**: Loaded from `{memory_dir}/dags/{dag_name}/MEMORY.md`

The agent can read and write memory files via the `read` and `patch` tools during execution. Changes persist across runs.

When `memory.enabled` is false (default), no memory is loaded and the agent operates statelessly.

---

## 5. Permission & Security

### Safe Mode and HITL Integration

In the Web UI, safe mode presents an approval dialog when the agent attempts dangerous commands. Since there is no UI in step context, the agent step integrates with the existing HITL (human-in-the-loop) mechanism:

```
Agent attempts denied bash command
        │
        ▼
  deny_behavior?
   ┌─────┴─────┐
   │            │
 block         hitl
   │            │
   ▼            ▼
 Reject     Pause step,
 command    emit HITL approval
   │        request and wait
   ▼            │
 Agent gets     ▼
 error msg   Human approves/rejects
             via Web UI or API
                │
                ▼
           Resume execution
```

When `deny_behavior: hitl`, the step pauses exactly like a `type: hitl` step — visible in the DAG run UI with an approval prompt showing the command and working directory.

When `safe_mode: false`, no approval is required and the agent runs autonomously with full trust. Safe mode is enabled by default.

### Bash Policy Enforcement

Bash rules are evaluated in order. The first matching rule determines the action. If no rule matches, `default_behavior` applies. This is identical to the Web UI agent's policy engine.

### Secret Masking

All messages sent to the LLM provider are passed through the secret masking layer. Environment variables marked as secrets (via `env:` with secret provider) are redacted before transmission.

### Role-Based Restrictions

In the step context, the agent runs with the permissions of the DAG execution context rather than a specific user session. Tool permission checks (`CanExecute`, `CanWrite`) should be configured via the tool policy rather than user roles.

---

## 6. Observability

### Step Logs

All agent activity is written to the step's stdout/stderr log files, which are viewable in the DAG run detail page:

| Log Entry | Content |
|-----------|---------|
| LLM request | Model, message count, tool count |
| Assistant response | Response content (or summary if large) |
| Tool call | Tool name, arguments (with secrets masked) |
| Tool result | Output (truncated if large), success/error status |
| Policy decision | Command, matched rule, action taken (allow/deny/hitl) |
| Iteration count | Current iteration / max iterations |

### Token Usage & Cost

Token usage is tracked per LLM request and accumulated across the step:

- **Per-request**: prompt tokens, completion tokens, total tokens
- **Per-step total**: cumulative tokens and estimated cost
- **Final summary**: Written to step log on completion

Cost is computed from the model's `input_cost_per_1m` and `output_cost_per_1m` pricing as configured in the global Agent Settings.

### Message History

The complete message history (user messages, assistant responses, tool calls, tool results) is structured data that can be:

- Written to the step's log output for debugging
- Captured via the step's `output` variable (final response only)
- Passed to subsequent `type: chat` or `type: agent` steps via `ChatMessageHandler` context

### Audit Integration

When the audit system is enabled, agent tool executions in steps generate audit entries consistent with the Web UI agent's audit trail:

| Audit Field | Value |
|-------------|-------|
| action | `bash_exec`, `file_read`, `file_patch`, etc. |
| dag_name | Parent DAG name |
| dag_run_id | Current run ID |
| step_name | Step name |
| details | Tool-specific details (command, file path, etc.) |

### DAG Run UI

In the DAG run detail view, agent steps display:

- Current status (running, waiting for HITL, completed, failed)
- Iteration progress (e.g., "Tool round 5/50")
- Final response content
- Expandable tool call history
- Token usage summary

---

## 7. Relationship to `type: chat`

The two step types serve different purposes and coexist:

| Aspect | `type: chat` | `type: agent` |
|--------|:---:|:---:|
| Purpose | Simple LLM calls | Agentic multi-tool workflows |
| Tools | DAG-based (other DAGs as tools) | Built-in (bash, read, patch, etc.) |
| Memory | None | Global + per-DAG |
| System prompt | Simple `system:` field | Built-in Tsumugi prompt (with optional `prompt` append) |
| Policy enforcement | None | Bash rules, tool restrictions |
| User interaction | None | HITL integration |
| Message passing | Yes (ChatMessageHandler) | Yes (ChatMessageHandler) |
| Model fallback | Yes (multiple models) | Future consideration |

Users who need simple "ask an LLM a question" functionality should continue using `type: chat`. Users who need the agent to take autonomous actions (run commands, edit files, search the web) should use `type: agent`.

---

## 8. Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Agent runs indefinitely with tool loops | `max_iterations` limit (default 50) + step `timeout_sec` |
| Unintended side effects from bash/patch | Tool policy with bash rules; `safe_mode: true` with HITL; explicit tool selection |
| Cost overruns from excessive LLM calls | Token usage logging; cost summary in step output; `max_iterations` cap |
| Secret leakage to LLM provider | Secret masking layer applied to all messages before transmission |
| Agent modifies files outside intended scope | Custom `prompt` to constrain behavior; bash policy rules |
| HITL approval blocks pipeline indefinitely | Standard HITL timeout mechanisms apply |

---

## 9. Out of Scope

- Inbox/notification integration (see RFC 012)
- Model fallback (multiple models with retry) — follow-up enhancement
- Streaming output to the DAG run UI in real time — initial version uses standard log output
- Agent-to-agent communication between steps — use output variables and message context passing
- Custom tool definitions (user-defined tools beyond the built-in set) — use `type: chat` with DAG-based tools for this

---

## Key Files

| Purpose | File |
|---------|------|
| Agent core (tools, loop, session, types) | `internal/agent/` |
| Agent system prompt template | `internal/agent/system_prompt.txt` |
| Existing chat step executor | `internal/runtime/builtin/chat/executor.go` |
| Executor interface & registry | `internal/runtime/executor/executor.go` |
| Step YAML parsing & transformers | `internal/core/spec/step.go` |
| Runtime step struct | `internal/core/step.go` |
| Executor capabilities | `internal/core/capabilities.go` |
| Node execution wrapper | `internal/runtime/node.go` |
| HITL step (pattern for HITL integration) | `internal/runtime/builtin/hitl/hitl.go` |
| LLM provider interface | `internal/llm/provider.go` |
| Agent research document | `RESEARCH_AGENT.md` |
