# ACP harness providers

## Local stdio

Custom harness definitions select ACP with `protocol: acp`. Omitting `transport`
defaults to stdio. Omitting `protocol`, or setting it to `cli`, retains CLI behavior.
Install and authenticate the adapter before running the workflow.

```yaml
harnesses:
  coding-agent:
    protocol: acp
    binary: codex-acp
steps:
  - action: harness.run
    with:
      provider: coding-agent
      prompt: Review the project without modifying files.
    output: REVIEW
```

`binary`, `prefix_args`, `flag_style`, and `option_flags` configure local startup.
Other `with` options become adapter CLI flags. ACP definitions reject CLI prompt
placement options (`prompt_mode`, `prompt_flag`, `prompt_position`); `with.stdin`
is appended to the prompt after a blank line instead of sent to the protocol pipe.

## Remote WSS and Streamable HTTP

Remote definitions require `url` and an absolute `cwd` on the **agent host**.
They do not accept local startup settings (`binary`, `prefix_args`, `flag_style`,
`option_flags`). URL, cwd, and header values support workflow variables.

```yaml
harnesses:
  remote-agent:
    protocol: acp
    transport: http                 # websocket for ws:// or wss:// endpoints
    url: https://acp.example.com/acp # wss://acp.example.com/acp for secure WebSocket
    cwd: /srv/project
    headers:
      Authorization: Bearer ${ACP_TOKEN}
steps:
  - action: harness.run
    timeout_sec: 120
    with:
      provider: remote-agent
      prompt: Review the project without modifying files.
      # ca_file: /path/to/private-ca.pem
    output: REVIEW
```

Supply tokens through workflow environment/secrets settings. Remote steps accept
`provider`, `fallback`, `acp_permission`, and optional `ca_file`, not adapter CLI flags.
`ca_file` adds PEM roots alongside system certificates for this connection only;
relative paths resolve from the local step working directory. Certificate and
hostname verification remain enabled. Dagu never creates or translates remote cwd.

- **WebSocket:** accepts `ws://` and `wss://`, advertises `acp.v1`, and sends headers
  in the upgrade request. Bearer credentials are also offered as `bearer.<token>`
  for ACP UI-compatible bridges. Text frames may contain individual or newline-
  batched messages; outbound NDJSON delimiters are preserved for stdio bridges.
  Binary frames are rejected. A `$/ping` notification keeps idle connections alive.
- **HTTP:** accepts `http://` and `https://`, following the ACP SDK's experimental
  Streamable HTTP profile. Initialize returns JSON and `Acp-Connection-Id`.
  Subsequent POSTs return 202; responses, notifications, and permission requests
  arrive on connection/session SSE GETs. `Acp-Session-Id` routes session traffic,
  including callback responses. Cookies preserve affinity; DELETE releases the
  connection. This is ACP HTTP, not MCP's POST response-stream transport.

Remote requests reject redirects and omit credentials/response bodies from network
errors. Connection setup and POSTs have bounded deadlines. Use `timeout_sec` for
an overall turn deadline. Disconnects do not automatically reconnect or replay work.

## Shared behavior

All three transports use the same Go ACP SDK, protocol negotiation, session setup,
prompt handling, permissions, output capture, cancellation, and provider fallback.
Each execution creates a fresh session. Local processes inherit step environment,
working directory, and resource limits; remote agent processes remain server-owned.

Assistant text chunks become step stdout and can feed downstream output variables.
Protocol JSON, thoughts, and tool events are excluded. Only `end_turn` succeeds.
Failed attempts do not contribute text to successful fallback output.

`with.acp_permission` defaults to `deny`. `allow` selects only an offered `allow_once`
option; it never grants persistent permission. This policy governs ACP callbacks,
not a sandbox around the agent's own tools. Denials fail the attempt.

Timeout or stop sends `session/cancel` before interrupting the transport. Local
process trees are terminated; remote connections are closed and HTTP connections
are deleted. Remote agents must honor cancellation. Cancellation skips fallback.

Client filesystem/terminal callbacks, containerized ACP, session resume, interactive
authentication, and MCP injection are not included. Adapters authenticate beforehand.

## Verification

Package tests cover stdio, WSS certificate verification, HTTP cookies and scoped
SSE/callback routing, permission decisions, disconnects, cancellation, and cleanup.
Binary conformance tests exercise all three transports and downstream output use.
Interoperability acceptance also runs the real Dagu binary against Codex via stdio,
WSS, and the official TypeScript SDK's Streamable HTTP server profile.
