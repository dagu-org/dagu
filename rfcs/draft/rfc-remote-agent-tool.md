# RFC 027: Remote Agent Tool

## Goal

Add `remote_agent` and `list_remote_nodes` agent tools that enable the local AI agent to discover remote Dagu nodes, delegate tasks, query information, and orchestrate workflows — without requiring the user to manually switch node context in the UI.

---

## Scope

| In Scope | Out of Scope |
|----------|--------------|
| Tool registration with `DefaultEnabled: false` | Multi-turn sessions (each call creates a new session) |
| Poll-until-done execution with exponential backoff | SSE streaming from remote sessions |
| Safe mode enforcement with automatic prompt rejection | Prompt forwarding to local user |
| Token-auth-only node filtering (`auth_type: token`) | Basic auth / no-auth node support |
| RBAC enforcement (execute permission required) | Per-node rate limiting |
| Audit logging of all remote agent invocations | Frontend UI components (tool viewer, session linking) |
| Delegate tool exclusion (prevent cascading remote calls) | Remote node auto-discovery |
| Agentstep integration via context-key wiring | Session cleanup or TTL |
| `list_remote_nodes` tool for node discovery | |
| Configurable per-node timeout | |
| Poll retry on transient GET failures | |
| Idempotent session creation via client-provided `sessionId` | |

---

## Solution

### Execution Flow

```mermaid
sequenceDiagram
    participant LA as Local Agent
    participant T as remote_agent Tool
    participant RN as Remote Node API

    LA->>T: remote_agent({ node: "production", message: "..." })
    T->>T: Generate sessionId (UUID v4), validate node, check RBAC
    T->>RN: POST /agent/sessions { sessionId, message, safeMode: true }
    Note over T,RN: sessionId enables idempotent retry; no model field — remote uses its default
    RN-->>T: 201 { sessionId, status: "accepted" }

    loop Poll (exponential backoff: 500ms → 5s cap)
        T->>RN: GET /agent/sessions/{id}
        RN-->>T: { sessionState: { working, hasPendingPrompt } }

        alt hasPendingPrompt == true
            T->>RN: POST /agent/sessions/{id}/respond { promptId, cancelled: true }
            Note over T: Record rejection, poll immediately
        end
    end

    RN-->>T: { working: false, messages: [...] }
    T->>T: Extract last assistant message, append rejection note, truncate
    T-->>LA: ToolOut { content: "<response>" }

    Note over T: On timeout or cancellation:<br/>POST /agent/sessions/{id}/cancel<br/>Error includes sessionId for debugging
```

### Tool Schema

```json
{
  "name": "remote_agent",
  "description": "Send a task to an AI agent on a remote Dagu node. The remote agent runs in safe mode (destructive commands are auto-rejected). Available nodes: <injected at factory time>",
  "parameters": {
    "type": "object",
    "properties": {
      "node": {
        "type": "string",
        "description": "Name of the remote node to communicate with.",
        "enum": ["<injected from config>"]
      },
      "message": {
        "type": "string",
        "description": "The task or question to send to the remote agent."
      }
    },
    "required": ["node", "message"]
  }
}
```

The `node` enum and tool description are populated at factory time from configured remote nodes, preventing the LLM from hallucinating node names.

### `list_remote_nodes` Tool Schema

```json
{
  "name": "list_remote_nodes",
  "description": "List available remote Dagu nodes that can be targeted by remote_agent.",
  "parameters": {
    "type": "object",
    "properties": {
      "name_filter": {
        "type": "string",
        "description": "Optional substring filter on node names. Omit to list all nodes."
      }
    },
    "required": []
  }
}
```

Returns a list of objects with `name` and `description` for each matching node. Reads from the resolver (config + store) — no network calls to remote nodes.

Like `remote_agent`, this tool requires execute permission and is hidden when no token-authenticated remote nodes are configured. It is excluded from delegate sub-agent tool sets.

### Polling Parameters

| Parameter | Value |
|-----------|-------|
| Initial interval | 500 ms |
| Backoff factor | 1.5× |
| Max interval | 5 s |
| Default timeout | 5 min (overridable per-node via config) |
| Max transient poll retries | 3 (per consecutive failure, resets on success) |
| HTTP connect timeout | 10 s |
| HTTP response timeout | 30 s |
| Response truncation limit | 10,000 chars |

HTTP connect and response timeouts are per-request limits, independent of the overall session timeout. They prevent individual requests from hanging indefinitely on an unresponsive node.

**Truncation strategy**: When a response exceeds 10,000 characters, the first 500 characters and last 9,500 characters are kept, joined by a `... [truncated N chars] ...` marker. This head+tail approach preserves the final answer (typically at the end of LLM output) while retaining initial context.

### Remote Node Resolution

Remote nodes are resolved via the `Resolver`, which merges nodes from two sources:

1. **Config file** (`remote_nodes` YAML array) — static, read-only, loaded at startup
2. **Remote node store** (file-based, CRUD via API/UI) — dynamic, managed at runtime

Store nodes take precedence over config nodes on name collision.

Only nodes with `auth_type: token` are available to the tool. Nodes with `auth_type: none` or `auth_type: basic` are excluded. Since API tokens carry their own role (admin, manager, developer, etc.), RBAC on the remote node is enforced automatically — the remote node's auth middleware creates a synthetic user from the token's role and applies the same permission checks as for any other request. The LLM only sees node names, never credentials.

When no qualifying remote nodes exist (across both sources), the tool is hidden entirely (factory returns nil).

### Idempotent Session Creation

The `POST /agent/sessions` request is made idempotent via an optional client-provided `sessionId` field, following the same pattern as the webhook endpoint's `dagRunId` (see `webhooks.go`).

**Problem**: If a POST succeeds on the remote node but the response is lost (network blip), the tool retries and creates a duplicate orphaned session — the original session runs uncontrolled.

**Mechanism**: The tool generates a UUID v4 before the first POST attempt and sends it as `sessionId` in every attempt of the same logical request. The remote node uses this ID instead of generating its own.

**Server behavior**:

1. If `sessionId` is provided:
   a. Validate UUID format (reject non-UUID → 400)
   b. Check active sessions (`sync.Map`) for a session with that ID owned by the authenticated user
   c. Check the session store for a persisted session with that ID owned by the authenticated user
   d. If found and same user → return `201 { sessionId, status: "already_exists" }`
   e. If found but different user → return 400 (generic error, no information leakage)
2. If `sessionId` is absent → generate UUID v4 server-side (current behavior, fully backward compatible)
3. Create session with the determined ID → return `201 { sessionId, status: "accepted" }`

**Client behavior**: The tool always gets 201 with a `sessionId`, regardless of whether the session is new or pre-existing. It proceeds to poll either way. The retry is transparent — no special error-code branching needed.

**Why 201 (not 409)**: The webhook returns 409 because it's fire-and-forget — the caller doesn't need the resource ID back. The `remote_agent` tool needs the `sessionId` to proceed with polling. Returning 409 would force the tool to parse the error body for the ID — fragile and unnecessary. Returning 201 achieves true idempotency: same input → same output.

**Race condition handling**: Two concurrent POSTs with the same `sessionId` are resolved by `sync.Map.LoadOrStore`, which is atomic — exactly one caller wins and creates the session, the other sees the existing entry and returns `"already_exists"`. For persisted sessions (after server restart), the file-based session store's mutex (`filesession.Store.CreateSession`) provides the same guarantee.

**Backward compatibility**: `sessionId` is optional. Omitting it preserves current behavior (server generates UUID). The `CreateAgentSessionResponse` schema already has `sessionId` and `status` fields — only the new `"already_exists"` status value is added.

### Security Model

1. **Authentication** — API tokens from config file or remote node store; credentials never exposed to the LLM. Store-managed node credentials are encrypted at rest using AES-256-GCM.
2. **Safe mode** — Remote sessions always run with `safeMode: true`; command approval prompts are auto-rejected. Rejection reports include prompt type and a truncated summary (first 200 chars) — full prompt content is not forwarded to prevent leaking remote node context.
3. **Tool policy** — `DefaultEnabled: false`; operators must explicitly enable the tool.
4. **RBAC** — Requires execute permission (operator and above).
5. **Audit** — Both tools are audit-logged via the existing `OnAfterToolExec` hook. `remote_agent` uses action `remote_agent_exec` with details: `node`, `message_length`, `remote_session_id`. `list_remote_nodes` uses action `remote_nodes_list` with details: `name_filter`.
6. **Credential isolation** — Remote node credentials resolved via the `Resolver` at tool creation time, not accessible to the LLM. Store-managed credentials are encrypted at rest (AES-256-GCM) and decrypted only in memory.
7. **Delegate isolation** — `remote_agent` and `list_remote_nodes` are filtered from delegate sub-agent tool sets via the delegate tool's factory deny list, preventing cascading cross-node calls.
8. **Remote trust boundary** — Safe mode behavior is defined by the remote node's agent configuration. Operators should audit remote node agent configs to ensure safe mode policies meet expectations.
9. **Idempotency key scoping** — Client-provided `sessionId` is validated as UUID format and scoped to the authenticated user. A `sessionId` belonging to a different user is rejected with a generic 400 error, preventing session ID enumeration.

### Configuration

Remote nodes can be defined in the server config file or managed dynamically via the settings UI/API. Config-file nodes use the existing `remote_nodes` array:

```yaml
remote_nodes:
  - name: "production"
    description: "Production environment"
    api_base_url: "https://prod.example.com/api/v1"
    auth_type: "token"
    auth_token: "${PROD_TOKEN}"
    timeout: 10m  # optional, default 5m
```

Store-managed nodes are created via the remote nodes settings UI or `POST /api/v1/remote-nodes`. Both sources are merged by the `Resolver` at runtime.

The tools are enabled via the existing agent tool policy:

```yaml
agent:
  toolPolicy:
    tools:
      remote_agent: true
      list_remote_nodes: true
```

### Error Handling

| Scenario | Behavior |
|----------|----------|
| Unknown node name | Return error with list of available nodes |
| Node unreachable | Return connection error (no credentials exposed) |
| Auth failure (401) | Return `"remote API error (HTTP 401): ..."` |
| Agent not configured (503) | Return `"remote API error (HTTP 503): ..."` |
| Session poll failure (transient) | Retry up to 3 times with backoff; return `"failed to poll session: ..."` with session ID after exhaustion |
| Remote agent error | Return the remote agent's error message with session ID |
| Timeout | Best-effort cancel remote session, return timeout error with session ID |
| Context cancelled (user stops) | Best-effort cancel remote session, return cancellation error with session ID |
| Auto-reject prompt fails | Log warning, continue polling |
| No output produced | Return `"Remote agent completed but produced no output."` |
| Role check fails (viewer) | Return `"Permission denied: remote_agent requires execute permission"` |
| Response > 10,000 chars | Head+tail truncation (see Polling Parameters) |
| Duplicate session (idempotent retry) | Return 201 with existing session ID and `status: "already_exists"` |
| Client `sessionId` not valid UUID | Return `"bad request: sessionId must be a valid UUID"` |

Auto-rejected prompts include the prompt type and a truncated summary (first 200 characters) in the response, so the local agent can understand what was rejected and adjust its approach. Full prompt content is not forwarded to avoid leaking sensitive context (file paths, environment variables) from the remote node's execution environment.

### Agentstep Integration

The `remote_agent` and `list_remote_nodes` tools are available in agent-type DAG steps, not just the interactive chat modal. The `Resolver` (which merges config-file and store-managed nodes) flows through the same context-key pattern as other agent dependencies (`WithSkillStore`/`GetSkillStore`, `WithConfigStore`/`GetConfigStore`, etc.):

1. Server startup creates the `Resolver` from config-file nodes and the remote node store
2. At the request/step boundary, the resolver is injected into `context.Context` via `WithRemoteNodeResolver`
3. The agentstep executor reads it via `GetRemoteNodeResolver`, queries for `auth_type: token` nodes, and populates `ToolConfig.RemoteNodes`
4. The tool factory uses `ToolConfig.RemoteNodes` to build the schema (node enum) and decide whether to return nil

This is consistent with how `SkillStore` flows: context key for cross-boundary injection, `ToolConfig` field for factory construction. The tools respect both the global tool policy and step-level `tools.enabled` overrides.

```yaml
steps:
  - name: check-production
    type: agent
    messages:
      - role: user
        content: "Check the last 5 runs of nightly-etl on production and report failures."

  - name: compare-envs
    type: agent
    agent:
      tools:
        enabled: [remote_agent, list_remote_nodes, read, bash, output]
    messages:
      - role: user
        content: "Compare the config of data-pipeline DAG on staging vs production."
    depends:
      - check-production
```

---

## Data Model

### ToolConfig Extension

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `RemoteNodes` | map (keyed by name) | `{}` | Available remote nodes for cross-node communication |

### Session Creation Request Extension

The `POST /agent/sessions` request body accepts an optional `sessionId` field for idempotent creation:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `sessionId` | string (UUID) | server-generated | Optional client-provided session ID. When provided and a session with this ID already exists for the authenticated user, the endpoint returns 201 with `status: "already_exists"`. Must match UUID format (`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`). |

### Remote Node Fields (used by tool)

These fields are sourced from the `remotenode.RemoteNode` domain model (resolved from config file or store):

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Name` | string | — | Human-readable node name (used in tool enum) |
| `Description` | string | `""` | Optional node description (returned by `list_remote_nodes`) |
| `APIBaseURL` | string | — | Base URL for the remote node's REST API |
| `AuthType` | `AuthType` enum | `"none"` | Authentication method: `none`, `basic`, or `token`. Only `token` nodes are eligible. |
| `AuthToken` | string | — | Bearer token carrying its own RBAC role (used when `AuthType == "token"`) |
| `SkipTLSVerify` | bool | `false` | Skip TLS certificate verification |
| `Timeout` | duration | `5m` | Per-node session timeout, overrides the default 5-minute timeout. Stored as `Timeout` on `RemoteNode`. |

---

## Edge Cases & Tradeoffs

| Chosen | Considered | Why |
|--------|------------|-----|
| Polling with exponential backoff | SSE streaming | REST detail endpoint already returns all needed state (`working`, `hasPendingPrompt`); avoids SSE client complexity; 5s steady-state latency is acceptable for a tool that runs seconds to minutes |
| REST API | gRPC (coordinator protocol) | Aligns with existing remote node infrastructure; remote nodes are REST-based; no protocol changes needed |
| Separate `remote_agent` tool | Extend existing `delegate` tool | Delegate creates in-process sub-sessions; remote delegation is cross-process/cross-network — mixing the concerns would complicate delegate |
| Synchronous (wait for response) | Fire-and-forget | The local agent needs the remote response to inform its next actions |
| New session per call | Reusable sessions | Multi-turn session management adds lifecycle complexity (stale sessions, cleanup) without clear MVP value |
| Auto-reject pending prompts | Forward prompts to local user | Forwarding adds multi-turn complexity and latency; auto-rejection with reporting lets the local agent adjust its approach |
| Token-auth-only nodes (`auth_type: token`) | Support basic auth / no-auth nodes | Tokens carry their own RBAC role, enabling automatic permission enforcement; basic auth would require additional credential mapping |
| `DefaultEnabled: false` | `DefaultEnabled: true` | Cross-node agent communication is a powerful capability; explicit opt-in prevents accidental use |
| No rate limiting (MVP) | Per-node rate limits | LLM tool calling frequency is naturally bounded; rate limiting can be added later if abuse patterns emerge |
| Client-provided `sessionId` for idempotent POST | `Idempotency-Key` header with response cache | Follows existing `dagRunId` webhook pattern; no new storage infrastructure — the session store IS the dedup store; one field, one concept; backward compatible (field is optional) |
| Retry with exponential backoff (POST and GET) | No retry | Idempotent POST makes retries strictly safe — no orphaned duplicate sessions. Poll GETs retry up to 3 consecutive failures before giving up — avoids losing a running remote session due to a momentary network issue |

### Known Limitations

- **Orphaned remote sessions (poll-phase)** — If the local Dagu instance restarts **while polling** a remote session, the remote session continues running with no one consuming its output. Retry-induced orphans are eliminated by idempotent session creation (client-provided `sessionId`). Poll-phase orphans are mitigated by the remote node's count-based session retention (`session.max_per_user`, default 100) — orphaned sessions are eventually evicted as newer sessions are created. Full session cleanup is out of scope for MVP.
- **No health probing** — `list_remote_nodes` reads from the resolver (config + store) only; a node may be listed but unreachable. The agent discovers reachability when it calls `remote_agent` and receives a connection error.

---

## Definition of Done

- Sending a message to a valid remote node returns the remote agent's response.
- Sending a message to an invalid node name returns an error listing available nodes.
- Pending prompts on the remote session are automatically rejected; prompt type and truncated summary (first 200 chars) are reported.
- Responses exceeding 10,000 characters are truncated using head+tail strategy with a marker.
- Tool invocation times out (default 5 min, configurable per-node) and best-effort cancels the remote session.
- Cancelling the local chat best-effort cancels the remote session.
- Error messages include the remote session ID for debugging.
- Transient poll failures are retried up to 3 times before returning an error.
- Users without execute permission receive a permission denied error.
- Both tools are hidden when no token-authenticated remote nodes exist (across config and store).
- `remote_agent` and `list_remote_nodes` are excluded from delegate sub-agent tool sets.
- Both tools are audit-logged with actions `remote_agent_exec` and `remote_nodes_list`.
- `remote_agent_exec` audit details include `node`, `message`, and `remote_session_id`.
- Agent-type DAG steps can use both tools when remote nodes are present in context.
- Only `auth_type: token` nodes appear in the tools; `none` and `basic` auth nodes are filtered out.
- Both tools are disabled by default and must be explicitly enabled in tool policy.
- `list_remote_nodes` returns node names and descriptions, with optional name filter.
- `list_remote_nodes` reads from the resolver (config + store) only — no network calls to remote nodes.
- Retrying `POST /agent/sessions` with the same `sessionId` returns the existing session (201, `status: "already_exists"`) — no duplicate session created.
- Omitting `sessionId` preserves current behavior (server generates UUID).
- Invalid `sessionId` format returns 400.
