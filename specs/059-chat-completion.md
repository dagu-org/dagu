# Spec: Chat Completion Action

## Status

Partially implemented.

## Scope

This spec covers `chat.completion` request construction, response output,
stream selection, model fallback, and the `with.tools` agentic tool-calling
loop, using a local OpenAI-compatible endpoint. No external model service is
required.

Provider-specific parameters, timeout, abort, and UI-facing tool sub-DAG
drill-down tracking belong to executor and lifecycle coverage.

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

### Tool calling

`with.tools` names other DAGs as callable functions. Each named DAG's
declared parameters become the tool's JSON Schema (offered to the model as
`tools` on every request), and its name becomes the tool name. A tool-calling
request always uses a non-streaming call, regardless of `with.stream`.

When the model's response includes tool calls, each one runs the
corresponding DAG as a sub-DAG-run, passing the model's JSON arguments as
DAG params. The sub-DAG's declared outputs (`output:`) are JSON-encoded and
returned to the model as the tool result content, in a `tool` role message
addressed to the tool call's ID. The loop then sends another request
including that tool result, repeating until a response has no tool calls.

A tool call naming a DAG outside `with.tools`, or whose sub-DAG-run itself
fails, is not fatal to the step: the result content reports the problem to
the model (`tool "<name>" not found`, or the execution error), and the loop
continues to request another response.

`with.max_tool_iterations` bounds how many such request/tool-call rounds
run, default 10. Reaching the limit does not fail the step: it is a second,
equally successful termination path alongside the model responding with no
further tool calls.

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

Tool calling, using another DAG in the same file as a tool:

```yaml
steps:
  - action: chat.completion
    with:
      provider: local
      model: local-model
      base_url: http://localhost:8080
      prompt: What is the weather in Paris?
      tools:
        - get-weather
      max_tool_iterations: 5
---
name: get-weather
params: CITY
steps:
  - command: fetch-weather.sh "$CITY"
    output: WEATHER_JSON
```
