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

`binary`, `prefix_args`, `flag_style`, and `option_flags` configure the startup
command. Other `with` options remain adapter CLI flags, except for the reserved
options described below. ACP carries prompts over the protocol, so ACP definitions
reject `prompt_mode`, `prompt_flag`, and `prompt_position`. `with.stdin` is appended
to the prompt after a blank line rather than written directly to the protocol pipe.

## Execution contract

- Each execution launches a subprocess, negotiates ACP v1, creates a new session,
  and sends one prompt. `session/new.cwd` is the absolute step working directory.
  The process inherits the step environment and resource limits.
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
- Timeout or stop sends `session/cancel`, then terminates the process tree and
  reclaims pipes and temporary output. Cancellation does not trigger fallback.

## Limitations

This version supports local stdio. Containerized ACP, HTTP/WebSocket transports,
session resume, interactive authentication, MCP server injection, and an interactive
permission UI are not supported. Authenticate adapters before running them.

The client advertises neither filesystem nor terminal callbacks. Agents can use
their own file and command tools. Agents that require client-provided filesystem
or terminal tools need additional client capabilities.

## Acceptance coverage

- `internal/runtime/builtin/harness/acp_test.go` uses an independent JSON-RPC peer
  to verify chunked output, permissions, negotiation, errors, cancellation, and fallback.
- `conformance/spec067_harness/acp_test.go` exercises the Dagu binary to verify YAML
  loading, the ACP subprocess, `with.stdin`, output capture, and downstream steps.
