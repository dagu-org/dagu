# Spec: Chat Completion Action

## Status

Partially implemented.

## Scope

This spec covers `chat.completion` request construction, response output,
stream selection, and model fallback using a local OpenAI-compatible
endpoint. No external model service is required.

Provider-specific parameters, tool execution, timeout, abort, and tool
sub-DAG cleanup belong to executor and lifecycle coverage.

## Goal

Workflow authors can send a prompt or message history to a model and use
its response as step output.

## Behavior

A nonempty `with.prompt` becomes a user message. When both `with.prompt`
and `with.messages` are present, the prompt takes precedence. Otherwise,
`with.messages` provides the ordered message history. `with.system`, when
set, precedes the user messages.

Streaming is enabled by default. `with.stream: false` selects a
non-streaming request. Both modes write the response text to stdout,
followed by a newline.

A `with.model` array with multiple entries tries the models in order until
one succeeds. Requests for these fallback models disable streaming. A
single model does not activate fallback.

## Errors

`dagu validate` rejects missing prompt/message input and a configured
provider without a model. It exits nonzero with an error identifying the
invalid configuration.

When the first fallback model rejects a request and the next succeeds,
the step succeeds and writes the successful response. Exhausted fallback,
provider-specific errors, and lifecycle failures are outside this
conformance scope.

## Example

```yaml
steps:
  - action: chat.completion
    with:
      provider: local
      model: local-model
      base_url: http://localhost:8080
      prompt: Summarize the supplied text.
      stream: false
```
