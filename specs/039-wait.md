# Spec: Wait Actions

## Status

Implemented.

This spec defines conformance behavior for the built-in `wait.duration`,
`wait.until`, `wait.file`, and `wait.http` actions.

## Scope

This spec defines four actions that block a step until a condition becomes
true, then succeed:

- `wait.duration`: waits for a fixed length of time.
- `wait.until`: waits until an absolute point in time.
- `wait.file`: polls a filesystem path until it exists or stops existing.
- `wait.http`: polls an HTTP URL until it returns an expected status.

This spec covers:

- the `with` fields each action accepts, and their defaults
- how each action determines the condition is satisfied
- validation and runtime errors
- how these actions interact with a step's own `timeout_sec`

This spec does not define:

- signal delivery and abort behavior beyond what already applies to any step
  (see [Spec 017: Built-In Run Context](017-built-in-run-context.md))
- retry or `precondition` behavior
- authentication for `wait.http` requests

## Goal

Workflow authors pause a DAG-run until an external condition holds --
a fixed delay, a wall-clock deadline, a file appearing or disappearing, or
an HTTP endpoint becoming ready -- without writing a polling script by hand.

## Behavior

### wait.duration

`with.duration` (required) is a Go-style duration string (for example
`30s`, `5m`) and must be greater than zero. The step blocks for that long,
then succeeds.

### wait.until

`with.until` (required) is an RFC3339 timestamp. The step blocks until that
point in time, then succeeds. If the timestamp is already in the past when
the step starts, the step succeeds immediately rather than failing or
blocking.

### wait.file

`with.path` (required) is a file or directory path, resolved relative to
the step's working directory when not absolute. `with.state` selects the
condition:

- `exists` (default): succeeds once something exists at that path.
- `missing`: succeeds once nothing exists at that path.

The action polls at `with.poll_interval` (default `1s`).

### wait.http

`with.url` (required) must be an absolute `http://` or `https://` URL.
`with.method` defaults to `GET`. `with.headers` and `with.body` set the
request's headers and body. The action polls at `with.poll_interval`
(default `1s`), giving each individual request `with.request_timeout`
(default `10s`) to complete. A request that fails outright (connection
refused, timeout, and similar) counts as a non-matching poll rather than an
error and is retried on the next interval. The step succeeds once a
response's status code equals `with.status` (default `200`).

Config-schema validation for `with.url` (and `with.status`) runs before a
step's `with` values are resolved, so `with.url` must be a literal,
already-valid absolute URL in the DAG file; a `${...}`/`$VAR`-style
reference that would only become a valid URL after resolution fails
validation before the DAG ever runs.

`with.headers` and `with.body` are sent exactly as given, with no scheme
enforcement: a `with.url` using `http://` sends them in cleartext the same
as any other `http://` request. A workflow author who puts a credential in
`with.headers` is responsible for using an `https://` URL.

### Interaction with timeout_sec

All four actions poll or sleep against the step's own context, so a step
`timeout_sec` set on any of them cancels the wait once it elapses, the same
way it cancels any other step.

## Errors

### Validation

All of the following are rejected by DAG-build-time validation, before the
DAG starts running:

- `wait.duration`: `with.duration` is empty, is not a parseable duration, or
  is not greater than zero.
- `wait.until`: `with.until` is empty or is not a valid RFC3339 timestamp.
- `wait.file`: `with.path` is empty, or `with.state` is a value other than
  `exists` or `missing`.
- `wait.http`: `with.url` is empty or is not an absolute `http://`/`https://`
  URL, or `with.status` is outside `100`-`599`.

### Timeout and abort

All four actions block by sleeping or polling against the step's own
context (see "Interaction with timeout_sec" above), so a step `timeout_sec`
elapsing, or the DAG-run being stopped (`dagu stop` or an equivalent
cancellation), cancels the wait immediately with the same step-timeout or
abort result any other step gets; none of these actions has its own
distinct timeout or abort error shape.

### Cleanup

None of the four actions creates an external resource (a container, a job,
a connection held open past the request) that needs cleanup on completion,
failure, timeout, or abort. A `wait.http` request in flight when the step
is cancelled is simply abandoned; the action does not need to release
anything afterward.

## Related Specs

- Run context: [Spec 017: Built-In Run Context](017-built-in-run-context.md)
- Value resolution: [Spec 003: Value Resolution and Field Evaluation](003-value-resolution.md)

## Examples

Wait for a fixed delay:

```yaml
steps:
  - action: wait.duration
    with:
      duration: 30s
```

Wait for a file to appear, polling every 500ms:

```yaml
steps:
  - action: wait.file
    with:
      path: ready.flag
      poll_interval: 500ms
```

Wait for an HTTP health check to return 200:

```yaml
steps:
  - action: wait.http
    with:
      url: http://localhost:8080/healthz
      poll_interval: 1s
```
