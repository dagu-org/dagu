# Spec: Rclone Action

## Status

Implemented.

This spec defines conformance behavior for the official remote action
`rclone@v1` (`dagucloud/rclone`).

## Scope

Like `node-script@v1` ([Spec 060: Node Script Action](060-node-script.md)),
`python-script@v1` ([Spec 061: Python Script Action](061-python-script.md)),
`dbt@v1` ([Spec 062: DBT Action](062-dbt.md)), `ffmpeg@v1` ([Spec 064:
FFmpeg Action](064-ffmpeg.md)), and `github-cli@v1` ([Spec 065: GitHub CLI
Action](065-github-cli.md)), `rclone@v1` is not a built-in Go executor. It
is an official remote action: a git repository (`dagucloud/rclone`)
containing a `dagu-action.yaml` manifest and a DAG that runs a real
`rclone` binary the action provisions via the project's aqua-based tool
manager (`rclone/rclone@v1.74.1` and `nodejs/node@v22.21.1`; the action's
own wrapper is a Node.js script). Referencing `rclone@v1` resolves through
Dagu's generic remote-action mechanism, which clones the action's
repository and provisions its pinned tools on first use.

This spec covers:

- the required `with.command` (one of a fixed set of `rclone` subcommands)
  and the `with.source`/`with.destination` paths each subcommand needs,
  which the action's own wrapper -- not its input schema -- enforces
  based on the specific command
- that `sync`, `move`, `moveto`, `rmdir`, `delete`, and `purge` are treated
  as destructive: the wrapper refuses to run them unless `with.dryRun:
  true` or `with.allowDestructive: true` is set, again a wrapper-level
  check the input schema cannot express
- `with.configPath`, `with.workdir`, `with.checksum`, `with.fastList`,
  `with.verbose`, `with.stats`, `with.logLevel`, `with.transfers`,
  `with.checkers`, `with.include`/`with.exclude`/`with.filter`,
  `with.extraArgs`, `with.env`, and `with.maxOutputBytes`
- the result object: `ok`, `command`, `stdout`/`stderr`,
  `stdoutTruncated`/`stderrTruncated`, `durationMs`, `exitCode`, and
  `error`
- that, unlike `ffmpeg@v1`/`dbt@v1`/`github-cli@v1`, this action's `error`
  field is a plain string, and is populated for **every** failure --
  including a real `rclone` process failure, not only a wrapper-level
  validation error
- that this action has no `with.timeoutSeconds` field at all: the action's
  input schema forbids any property it does not declare
  (`additionalProperties: false`), so a step cannot bound how long
  `rclone` may run
- that a later step reads a result field as `${<step id>.outputs.<path>}`
  (a bare-step-id reference), the same form confirmed for `node-script@v1`,
  `python-script@v1`, `dbt@v1`, `ffmpeg@v1`, and `github-cli@v1`, and not
  via the strict `${steps.<step id>.outputs.<name>}` form
- that `dagu validate` never resolves a remote action reference: a
  `rclone@v1` step passes validate regardless of its `with:` content, and
  `with.command` being required is enforced only once the step actually
  runs

This spec does not define:

- the rclone action's own implementation, versioning, or release process --
  this spec treats `rclone@v1` as an external dependency and documents its
  observed contract
- `rclone`'s own command-line semantics, backend/remote configuration, or
  exit code conventions beyond how the action surfaces them
- the generic remote-action resolution mechanism itself, or the
  `${<step id>.outputs.<path>}` reference form's own resolution mechanism --
  both are covered in more depth by [Spec 060: Node Script
  Action](060-node-script.md) and [Spec 007: Value Resolution
  Steps](007-value-resolution-steps.md)
- performance or caching behavior of the action's own tool provisioning

## Goal

Workflow authors copy, sync, check, list, or manage files across any
rclone-supported storage backend as a Dagu step, without installing or
configuring `rclone` outside the workflow.

## Behavior

### Input

`with.command` is required and must be one of `copy`, `copyto`, `sync`,
`move`, `moveto`, `check`, `ls`, `lsf`, `lsd`, `tree`, `size`, `about`,
`mkdir`, `rmdir`, `delete`, `purge`, or `cat`. Each command has an arity the
wrapper enforces: `copy`, `copyto`, `sync`, `move`, `moveto`, and `check`
need both `with.source` and `with.destination`; every other command needs
only `with.source`. A command missing a path it needs fails with `source
is required for rclone <command>` or `destination is required for rclone
<command>` -- the input schema only requires `with.command` and cannot
make `with.source`/`with.destination` conditionally required based on its
value.

`with.dryRun` adds `--dry-run`; `with.allowDestructive` explicitly permits
a destructive command to run for real. `sync`, `move`, `moveto`, `rmdir`,
`delete`, and `purge` fail immediately (before `rclone` is ever invoked)
with `rclone <command> can delete or move data; set allowDestructive: true
or dryRun: true` unless one of those two is set -- this is, again, a
wrapper-level check the schema cannot express as a value constraint.

`with.checksum`, `with.fastList`, and `with.verbose` add `--checksum`,
`--fast-list`, and `-v`. `with.stats` and `with.logLevel` add `--stats
<value>` and `--log-level <value>` (`with.logLevel` must be one of
`DEBUG`, `INFO`, `NOTICE`, `ERROR`). `with.transfers`/`with.checkers` add
`--transfers`/`--checkers <value>`. `with.include`/`with.exclude`/
`with.filter` each add one `--include`/`--exclude`/`--filter <value>` per
array entry. `with.extraArgs` appends raw flags after the command and
before its path arguments. `with.configPath` adds `--config <value>`.
`with.workdir`, when set, is the working directory `rclone` runs in.
`with.env`, when set, is an object of extra environment variables for the
`rclone` process (a plain object, like `dbt@v1`'s `with.env`, not a string
like `ffmpeg@v1`'s). `with.maxOutputBytes` (default `1048576`) caps how
much of `rclone`'s stdout and stderr are captured into the result.

There is no `with.timeoutSeconds` field, and the action's input schema
sets `additionalProperties: false`, so a step cannot bound `rclone`'s
runtime at all -- a long-running `rclone` invocation runs until it exits
on its own.

### Result

The step's stdout is one JSON object:

- `ok`: `true` when `rclone` exited with code `0`.
- `command`: the `with.command` value that was run (not a full argv --
  unlike `ffmpeg@v1`/`dbt@v1`/`github-cli@v1`, this action's outputs
  schema exposes no argv/args field at all).
- `stdout`/`stderr`: text `rclone` wrote to stdout and stderr, each
  truncated to `with.maxOutputBytes`.
- `stdoutTruncated`/`stderrTruncated`: `true` when that stream was cut off
  at `with.maxOutputBytes`.
- `durationMs`: wrapper-measured process duration.
- `exitCode`: `rclone`'s real exit code, or a wrapper-forced `2` when the
  wrapper rejects the input or fails to start `rclone` at all.
- `error`: a plain string (not an object with `name`/`message`, unlike the
  other remote actions in this family). Unlike `ffmpeg@v1`/`dbt@v1`/
  `github-cli@v1`, this field is populated for every failure, not only a
  wrapper-level one: a missing required path, a destructive command
  without `allowDestructive`/`dryRun`, and a real non-zero `rclone` exit
  all set `error` (for a real `rclone` failure, to `rclone exited with
  code <n>`).

### Downstream references

A later step reads a field of the result as `${<step id>.outputs.<path>}`
-- the step ID used directly as the reference's namespace. This matches
the behavior [Spec 060: Node Script Action](060-node-script.md), [Spec
061: Python Script Action](061-python-script.md), [Spec 062: DBT
Action](062-dbt.md), [Spec 064: FFmpeg Action](064-ffmpeg.md), and [Spec
065: GitHub CLI Action](065-github-cli.md) document, confirming it is a
property of remote action:-type steps generally, not specific to one
action. The strict `${steps.<step id>.outputs.<name>}` form documented for
declared step outputs (see [Spec 012: Step Outputs](012-step-outputs.md))
does not resolve for a `rclone@v1` step, for the same reason it does not
for the other remote actions above.

## Errors

### Validation

- An action reference missing its required `@version` suffix (for example,
  `rclone` instead of `rclone@v1`): an error containing `unknown action`.
  This is enforced generically for any action reference, not specifically
  for `rclone`.
- Any `with:` field not declared by the input schema (there is no
  `with.timeoutSeconds`, for example): an error containing `unexpected
  additional properties`.

`dagu validate` does not resolve the remote action reference at all, so it
cannot check `rclone@v1`'s own requirements (such as `with.command` being
required, or `with.source`/`with.destination` being required for a
specific command) -- a step missing `with.command` passes validate.

### Runtime

- `with.command` missing: an error containing `missing properties:
  ["command"]`, raised when the action resolves its own input schema
  against the step's `with:` content -- before `rclone` is ever invoked.
- `with.command` set to an unsupported value: an error containing `does
  not equal any of`, raised by the same input-schema check.
- `with.source`/`with.destination` missing for a command that needs it, or
  a destructive command (`sync`, `move`, `moveto`, `rmdir`, `delete`,
  `purge`) without `with.allowDestructive`/`with.dryRun`: the step fails
  with `ok: false`, `exitCode: 2`, and a populated string `error` -- before
  `rclone` is ever invoked.
- A real `rclone` process failure: the step fails (a non-zero exit) with
  `rclone`'s own real `exitCode`, its own `stdout`/`stderr`, and `error`
  set to `rclone exited with code <n>`.

## Related Specs

- Node script action, Python script action, DBT action, FFmpeg action, and
  GitHub CLI action, for the equivalent remote-action execution model and
  downstream-reference behavior this action shares: [Spec 060: Node
  Script Action](060-node-script.md), [Spec 061: Python Script
  Action](061-python-script.md), [Spec 062: DBT Action](062-dbt.md),
  [Spec 064: FFmpeg Action](064-ffmpeg.md), [Spec 065: GitHub CLI
  Action](065-github-cli.md)
- Step outputs and reference syntax, for contrast with the bare-step-id
  reference form this action's result uses: [Spec 012: Step
  Outputs](012-step-outputs.md)

## Examples

Preview a sync, then run it for real:

```yaml
steps:
  - id: preview_sync
    action: rclone@v1
    with:
      command: sync
      source: /data/reports
      destination: backup:reports
      dryRun: true

  - id: run_sync
    depends: preview_sync
    action: rclone@v1
    with:
      command: sync
      source: /data/reports
      destination: backup:reports
      allowDestructive: true
```

List files and print the listing from a later step:

```yaml
steps:
  - id: list_files
    action: rclone@v1
    with:
      command: lsf
      source: /data/input

  - id: print
    depends: list_files
    run: echo "${list_files.outputs.stdout}"
```
