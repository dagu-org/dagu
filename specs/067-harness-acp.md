# ACP support for harness providers

## Configuration

Set `protocol: acp` in a custom `harnesses` definition to connect to a local
Agent Client Protocol v1 agent or adapter. Omitting `protocol`, or setting it to
`cli`, preserves the existing CLI invocation behavior.

```yaml
harnesses:
  coding-agent:
    protocol: acp
    binary: codex-acp
    # Set prefix_args if the adapter requires additional startup arguments.

steps:
  - id: review
    action: harness.run
    with:
      provider: coding-agent
      prompt: Review the code in this directory and summarize its modules without modifying files.
    output: REVIEW
```

Install and authenticate the adapter before running the workflow. For example,
install the Codex adapter with `npm install -g @agentclientprotocol/codex-acp`.
Other agents can expose their own ACP mode or use a separate adapter. Dagu does
not require a predefined agent name and does not install adapters automatically.

### Remote WebSocket agents

Use `transport: websocket` to connect to the same type of `ws://` or `wss://`
endpoint supported by [ACP UI](https://github.com/formulahendry/acp-ui#remote-agents-over-websocket):

```yaml
harnesses:
  remote-agent:
    protocol: acp
    transport: websocket
    url: wss://acp.example.com/v1
    cwd: /srv/project
    headers:
      Authorization: Bearer ${ACP_TOKEN}

steps:
  - id: review
    action: harness.run
    timeout_sec: 120
    with:
      provider: remote-agent
      prompt: Review the project without changing files.
    output: REVIEW
```

Provide `ACP_TOKEN` through the workflow environment or secrets configuration.
`url`, header values, and `cwd` support workflow variable references. `cwd` is
required and must be an absolute path on the **agent host**. It is forwarded
unchanged: Dagu does not create that directory or translate it to the client OS.
Remote definitions must not contain `binary`, `prefix_args`, `option_flags`, or
`flag_style`. Adapter startup options belong on the remote server; remote steps
accept `provider`, `fallback`, and `acp_permission`, not local CLI flags.

The connection advertises the `acp.v1` WebSocket subprotocol. Headers are sent in
the HTTP upgrade request; a Bearer authorization value is also offered as the
`bearer.<token>` subprotocol for browser-compatible bridges. TLS certificates are
verified using the system trust store. Handshake redirects are rejected so
credentials are not forwarded to another endpoint. Connection errors omit the
URL, response body, and header values, which may contain credentials.

One text frame carries one JSON-RPC message, or a batch of newline-delimited
messages from a stdio bridge. Outgoing frames preserve the NDJSON trailing newline
so bridges can forward them directly to agent stdin. Binary frames are rejected. A `$/ping` notification
every 25 seconds keeps idle proxy connections alive. The handshake has a
15-second deadline; use `timeout_sec` to limit the full turn. Connections are not
automatically reconnected or prompts replayed after disconnects, since an agent
may already have performed work. Use the normal workflow retry or fallback policy.

To expose a local adapter, ACP UI recommends a stdio-to-WebSocket bridge such as
`@rebornix/stdio-to-ws`. For example, run the bridge on the agent host:

```sh
npx @rebornix/stdio-to-ws "codex-acp" --port 3000
```

Then connect Dagu to `ws://<agent-host>:3000` with the agent's working directory.
Configure authentication and TLS on the bridge or its reverse proxy as appropriate.
WebSocket is an additional transport, not ACP's draft Streamable HTTP protocol.

### Local stdio invocation

`binary`, `prefix_args`, `flag_style`, and `option_flags` configure the startup
command. Other `with` options remain adapter CLI flags, except for the reserved
options described below. ACP carries prompts over the protocol, so ACP definitions
reject `prompt_mode`, `prompt_flag`, and `prompt_position`. `with.stdin` is appended
to the prompt after a blank line rather than written directly to the protocol pipe.

## Execution contract

- Each execution opens its configured transport, negotiates ACP v1, creates a new
  session, and sends one prompt. Stdio subprocesses inherit the step working
  directory, environment, and resource limits. WebSocket sessions use the configured
  remote `cwd`; local environment variables are not copied to the remote process.
- The implementation uses `github.com/coder/acp-go-sdk`, a community Go SDK listed
  in the ACP documentation.
- Text from `agent_message_chunk` updates is concatenated in order into step
  stdout, which can be captured with `output` for downstream steps. Thoughts,
  tool calls, and protocol JSON are excluded. Agent stderr remains in the step log.
- Only `end_turn` succeeds. Protocol errors, disconnects, refusals, length limits,
  and denied permissions fail the attempt and follow the existing `fallback`
  ordering. Failed attempts do not contribute to successful step output.
- `with.acp_permission` defaults to `deny`. Setting it to `allow` selects an offered
  `allow_once` option; requests without that option remain denied. This policy
  governs ACP permission callbacks, not the subprocess sandbox or its own tools.
- Timeout or stop sends `session/cancel`, then terminates the local process tree
  or closes the remote connection. Pipes and temporary output are reclaimed.
  Cancellation does not trigger fallback. Remote agents must implement cancellation;
  Dagu cannot forcibly terminate a process on another host.

## Limitations

This version supports local stdio and remote WebSocket. Containerized ACP,
Streamable HTTP, session resume, interactive authentication, MCP server injection,
and an interactive permission UI are not supported. Authenticate adapters before running them.

The client advertises neither filesystem nor terminal callbacks. Agents can use
their own file and command tools. Agents that require client-provided filesystem
or terminal tools need additional client capabilities.

## Acceptance coverage

- `internal/runtime/builtin/harness/acp_test.go` uses an independent JSON-RPC peer
  to verify chunked output, permissions, negotiation, errors, cancellation, and fallback.
- `conformance/spec067_harness/acp_test.go` exercises the Dagu binary to verify YAML
  loading, the ACP subprocess, `with.stdin`, output capture, and downstream steps.

- `internal/runtime/builtin/harness/acp_websocket_test.go` verifies text framing,
  authentication, trusted and untrusted TLS, permissions, disconnects, cancellation,
  fallback, and credential-safe handshake failures.
