# Spec: FFmpeg Action

## Status

Implemented.

This spec defines conformance behavior for the official remote action
`ffmpeg@v1` (`dagucloud/ffmpeg`).

## Scope

Like `node-script@v1` ([Spec 060: Node Script Action](060-node-script.md)),
`python-script@v1` ([Spec 061: Python Script Action](061-python-script.md)),
and `dbt@v1` ([Spec 062: DBT Action](062-dbt.md)), `ffmpeg@v1` is not a
built-in Go executor. It is an official remote action: a git repository
(`dagucloud/ffmpeg`) containing a `dagu-action.yaml` manifest and a DAG that
runs a real `ffmpeg`/`ffprobe` binary the action provisions via the
project's aqua-based tool manager (`Tyrrrz/FFmpegBin@8.1` and
`nodejs/node@v22.21.1`; the action's own wrapper is a Node.js script).
Referencing `ffmpeg@v1` resolves through Dagu's generic remote-action
mechanism, which clones the action's repository and provisions its pinned
tools on first use.

This spec covers:

- the required `with.args` (a shell-style argument string, or a JSON array
  string for exact argv boundaries) and `with.command` (`ffmpeg` or
  `ffprobe`, defaulting to `ffmpeg`)
- the automation-safe options the action prepends to `with.args`:
  `-hide_banner` (unless `hideBanner: false`), and, for `ffmpeg` only,
  `-nostdin` (unless `nostdin: false`) and `-y`/`-n` (`with.overwrite`)
- `with.workdir`, `with.env` (a newline-delimited `KEY=value` string or a
  JSON object string -- not a native YAML mapping), `with.timeoutSeconds`,
  and `with.maxOutputBytes`
- the result object: `ok`, `exitCode`, `signal`, `command`, `args`,
  `stdout`/`stderr`, `durationMs`, `timedOut`, `truncated`, and `error`
- that bad shell quoting or a malformed `with.env` line -- caught by the
  action's own wrapper, not its input schema -- report a populated `error`
  object with `ok: false` and `exitCode: -1`, while a real `ffmpeg`/
  `ffprobe` process failure (a bad input file, exceeding
  `with.timeoutSeconds`) reports through the ordinary `exitCode`/`stdout`/
  `stderr` fields instead, with no `error` object
- that exceeding `with.timeoutSeconds` sends `SIGTERM`, and `ffmpeg` traps
  it and exits with its own conventional signal-exit code (`255`) rather
  than being killed outright, so `exitCode` is a real positive number even
  though `timedOut` is `true`
- that a later step reads a result field as `${<step id>.outputs.<path>}`
  (a bare-step-id reference), the same form confirmed for `node-script@v1`,
  `python-script@v1`, and `dbt@v1`, and not via the strict
  `${steps.<step id>.outputs.<name>}` form
- that `dagu validate` never resolves a remote action reference: an
  `ffmpeg@v1` step passes validate regardless of its `with:` content, and
  `with.args` being required is enforced only once the step actually runs

This spec does not define:

- the ffmpeg action's own implementation, versioning, or release process --
  this spec treats `ffmpeg@v1` as an external dependency and documents its
  observed contract
- `ffmpeg`/`ffprobe`'s own command-line semantics, codec behavior, or exit
  code conventions beyond how the action surfaces them
- the generic remote-action resolution mechanism itself, or the
  `${<step id>.outputs.<path>}` reference form's own resolution mechanism --
  both are covered in more depth by [Spec 060: Node Script
  Action](060-node-script.md) and [Spec 007: Value Resolution
  Steps](007-value-resolution-steps.md)
- performance or caching behavior of the action's own tool provisioning

## Goal

Workflow authors transcode, inspect, or otherwise process media files with
real `ffmpeg`/`ffprobe`, without baking FFmpeg into worker images or
managing its installation themselves.

## Behavior

### Input

`with.args` is required: a shell-style argument string (parsed by the
action's own wrapper, not a shell -- it supports single and double quotes
and backslash escapes) excluding the executable name. A string starting
with `[` is instead parsed as a JSON array, giving exact argv boundaries
without shell-style quoting ambiguity. `with.command` selects `ffmpeg` or
`ffprobe` and defaults to `ffmpeg`.

`with.hideBanner` (default `true`) and, for `ffmpeg` only, `with.nostdin`
(default `true`) and `with.overwrite` (default `false`) control automation-
safe options the action prepends to `with.args`, in this order:
`-hide_banner`, `-nostdin`, then `-y` (`overwrite: true`) or `-n`
(`overwrite: false`). `ffprobe` never gets `-nostdin` or `-y`/`-n` -- only
`-hide_banner` applies to both commands.

`with.workdir`, when set, is the working directory the `ffmpeg`/`ffprobe`
process runs in; a relative path in `with.args` resolves against it.
`with.env`, when set, is extra environment variables exposed to the
process, as a newline-delimited `KEY=value` string or a JSON object
string -- the action's own input schema types this field as a string, so a
native YAML mapping for `with.env` fails validation (`want "string"`),
unlike `dbt@v1`'s `with.env`, which is a plain object. `with.timeoutSeconds`
(default `3600`) bounds how long the process may run; `with.maxOutputBytes`
(default `1048576`) caps how much of the process's stdout and stderr are
captured for the result.

### Result

The step's stdout is one JSON object:

- `ok`: `true` when the process exited with code `0` before
  `with.timeoutSeconds` was reached.
- `exitCode`: the real process exit code, or `-1` when the process never
  produced one (a wrapper-level validation failure, or a failure to even
  start the process).
- `signal`: present only when the process was terminated by a signal the
  process itself did not handle (rare for `ffmpeg`; see below).
- `command`: `ffmpeg` or `ffprobe`.
- `args`: the full argv passed to the process, including the automation-
  safe options the action prepended.
- `stdout`/`stderr`: captured process output, truncated to
  `with.maxOutputBytes`.
- `durationMs`: wrapper-measured process duration.
- `timedOut`: `true` when `with.timeoutSeconds` was reached.
- `truncated`: `{stdout, stderr}` booleans, each `true` when that stream
  was cut off at `with.maxOutputBytes`.
- `error`: present only when the action's own wrapper rejects the resolved
  input (for example, unparseable shell quoting in `with.args`, or a
  `with.env` line not in `KEY=value` form) before ever invoking
  `ffmpeg`/`ffprobe`. It has `name` (for example, `ValidationError`) and
  `message`. This differs from `dbt@v1`, where the equivalent path is
  effectively unreachable through schema-valid input -- here, several of
  the wrapper's own checks (shell-quoting validity, `env` line syntax) go
  beyond what the input schema can express, so they are reachable in
  practice.

A real `ffmpeg`/`ffprobe` failure -- a bad input file, an unresolvable
codec, a profile mismatch -- reports through the ordinary `exitCode`/
`stdout`/`stderr` fields with `ok: false` and no `error` object: the
process started and ran, it just exited non-zero.

Exceeding `with.timeoutSeconds` sends the process `SIGTERM`; if it is still
running 5 seconds later, the action sends `SIGKILL`. In practice, `ffmpeg`
traps `SIGTERM` and exits on its own with its conventional signal-exit code
(`255`) rather than being killed outright, so a timed-out result typically
has `timedOut: true`, `ok: false`, and a real positive `exitCode` (`255`),
with no `signal` field -- `signal` is populated only when the process is
actually killed by an unhandled signal (for example, after the `SIGKILL`
grace period elapses).

### Downstream references

A later step reads a field of the result as `${<step id>.outputs.<path>}`
-- the step ID used directly as the reference's namespace. This matches
the behavior [Spec 060: Node Script Action](060-node-script.md), [Spec 061:
Python Script Action](061-python-script.md), and [Spec 062: DBT
Action](062-dbt.md) document, confirming it is a property of remote
action:-type steps generally, not specific to one action. The strict
`${steps.<step id>.outputs.<name>}` form documented for declared step
outputs (see [Spec 012: Step Outputs](012-step-outputs.md)) does not
resolve for an `ffmpeg@v1` step, for the same reason it does not for the
other remote actions above.

## Errors

### Validation

- An action reference missing its required `@version` suffix (for example,
  `ffmpeg` instead of `ffmpeg@v1`): an error containing `unknown action`.
  This is enforced generically for any action reference, not specifically
  for `ffmpeg`.

`dagu validate` does not resolve the remote action reference at all, so it
cannot check `ffmpeg@v1`'s own requirements (such as `with.args` being
required) -- a step missing `with.args` passes validate.

### Runtime

- `with.args` missing: an error containing `missing properties:
  ["args"]`, raised when the action resolves its own input schema against
  the step's `with:` content -- before `ffmpeg`/`ffprobe` is ever invoked.
- `with.command` set to a value other than `ffmpeg`/`ffprobe`: an error
  containing `does not equal any of`, raised by the same input-schema
  check.
- `with.env` given as a native YAML mapping rather than a string: an error
  containing `has type "object"` (the schema requires a string), raised by
  the same input-schema check.
- Unparseable shell quoting in `with.args` (for example, an unterminated
  quote), or a `with.env` line not in `KEY=value` form: the step fails (a
  non-zero exit) with a populated `error` object (`name: "ValidationError"`)
  in the JSON result, `ok: false`, and `exitCode: -1` -- the process is
  never started.
- A real `ffmpeg`/`ffprobe` failure, or exceeding `with.timeoutSeconds`:
  the step fails (a non-zero exit), with the diagnostic JSON result
  described above still written to stdout.

## Related Specs

- Node script action, DBT action, and Python script action, for the
  equivalent remote-action execution model and downstream-reference
  behavior this action shares: [Spec 060: Node Script Action](060-node-script.md),
  [Spec 061: Python Script Action](061-python-script.md), [Spec 062: DBT
  Action](062-dbt.md)
- Step outputs and reference syntax, for contrast with the bare-step-id
  reference form this action's result uses: [Spec 012: Step
  Outputs](012-step-outputs.md)

## Examples

Convert a media file and inspect it with `ffprobe`:

```yaml
steps:
  - id: convert
    action: ffmpeg@v1
    with:
      overwrite: true
      args: -i input.mov -c:v libx264 -c:a aac output.mp4

  - id: inspect
    depends: convert
    action: ffmpeg@v1
    with:
      command: ffprobe
      args: -v error -print_format json -show_format -show_streams output.mp4

  - id: print_probe
    depends: inspect
    run: echo "${inspect.outputs.stdout}"
```

Bound a run with a timeout:

```yaml
steps:
  - action: ffmpeg@v1
    with:
      timeoutSeconds: 30
      args: -i input.mov -c:v libx264 output.mp4
```
