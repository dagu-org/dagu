# Spec: GitHub CLI Action

## Status

Implemented.

This spec defines conformance behavior for the official remote action
`github-cli@v1` (`dagucloud/github-cli`).

## Scope

Like `node-script@v1` ([Spec 060: Node Script Action](060-node-script.md)),
`python-script@v1` ([Spec 061: Python Script Action](061-python-script.md)),
`dbt@v1` ([Spec 062: DBT Action](062-dbt.md)), and `ffmpeg@v1` ([Spec 064:
FFmpeg Action](064-ffmpeg.md)), `github-cli@v1` is not a built-in Go
executor. It is an official remote action: a git repository
(`dagucloud/github-cli`) containing a `dagu-action.yaml` manifest and a DAG
that runs a real `gh` binary the action provisions via the project's
aqua-based tool manager (`cli/cli@v2.92.0` and `nodejs/node@v22.21.1`; the
action's own wrapper is a Node.js script). Referencing `github-cli@v1`
resolves through Dagu's generic remote-action mechanism, which clones the
action's repository and provisions its pinned tools on first use.

This spec covers:

- the required `with.args` (an array of strings passed to `gh` verbatim,
  with no shell parsing) and the optional `with.stdin`, `with.env`,
  `with.repo`, `with.host`, `with.workdir`, and `with.timeoutSeconds`
- that `with.repo` and `with.host` map to the `GH_REPO` and `GH_HOST`
  environment variables the real `gh` process reads, and `with.env` is
  merged on top of a fixed set of automation-safe defaults
  (`GH_PROMPT_DISABLED`, `GH_NO_UPDATE_NOTIFIER`, `GH_SPINNER_DISABLED`,
  `GH_TELEMETRY=false`) the action sets unless already present in the
  process environment
- the result object: `ok`, `exitCode`, `stdout`/`stderr`, `durationMs`,
  `ghVersion`, `timedOut`, and `error`
- that exceeding `with.timeoutSeconds` always reports `exitCode: 124`
  (forced by the wrapper, unlike `ffmpeg@v1`, which reports the real
  process's own signal-exit code) and, when the underlying process has no
  output of its own at the point it is killed, a synthesized
  `terminated by <signal>` `stderr` message
- that a real `gh` failure (missing authentication, a bad token, an
  unreachable host) reports through the ordinary `exitCode`/`stdout`/
  `stderr` fields with `ok: false` and no `error` object -- the process
  started and ran, it just exited non-zero
- that a later step reads a result field as `${<step id>.outputs.<path>}`
  (a bare-step-id reference), the same form confirmed for `node-script@v1`,
  `python-script@v1`, `dbt@v1`, and `ffmpeg@v1`, and not via the strict
  `${steps.<step id>.outputs.<name>}` form
- that `dagu validate` never resolves a remote action reference: a
  `github-cli@v1` step passes validate regardless of its `with:` content,
  and `with.args` being required is enforced only once the step actually
  runs

This spec does not define:

- the github-cli action's own implementation, versioning, or release
  process -- this spec treats `github-cli@v1` as an external dependency and
  documents its observed contract
- `gh`'s own command-line semantics, authentication model, or exit code
  conventions beyond how the action surfaces them
- the generic remote-action resolution mechanism itself, or the
  `${<step id>.outputs.<path>}` reference form's own resolution mechanism --
  both are covered in more depth by [Spec 060: Node Script
  Action](060-node-script.md) and [Spec 007: Value Resolution
  Steps](007-value-resolution-steps.md)
- performance or caching behavior of the action's own tool provisioning

## Goal

Workflow authors automate GitHub repository, issue, pull request, release,
or API operations through the real `gh` CLI, without installing or
authenticating it outside the workflow.

## Behavior

### Input

`with.args` is required: a non-empty array of non-empty strings passed to
`gh` as its argument vector, excluding the executable name -- unlike
`ffmpeg@v1`'s `with.args`, there is no shell-style string form and no
shell-style parsing to get wrong. `with.stdin`, when set, is text written to
`gh`'s stdin.

`with.env`, when set, is an object of extra environment variables for the
`gh` process (unlike `ffmpeg@v1`'s `with.env`, this one is a plain object,
not a string). `with.repo` (`[HOST/]OWNER/REPO`) and `with.host` map to the
`GH_REPO` and `GH_HOST` environment variables `gh` itself reads to select a
repository or host without an explicit `--repo`/`--hostname` flag on every
command. `with.workdir`, when set, is the working directory the `gh`
process runs in. `with.timeoutSeconds` (default `300`, maximum `1800`)
bounds how long the process may run.

The action always sets `GH_PROMPT_DISABLED=1`, `GH_NO_UPDATE_NOTIFIER=1`,
`GH_SPINNER_DISABLED=1`, and `GH_TELEMETRY=false` for the `gh` process,
unless the parent process environment already defines them -- these keep
`gh` from blocking on an interactive prompt or spinner in an unattended
run. `with.env` entries are applied after these defaults and after
`with.repo`/`with.host`, so they can override any of them.

### Result

The step's stdout is one JSON object:

- `ok`: `true` when `gh` exited with code `0` before `with.timeoutSeconds`
  was reached.
- `exitCode`: `gh`'s real exit code, `124` when `with.timeoutSeconds` was
  reached (regardless of how `gh` itself responded to being signaled), or
  `-1` when the process never started.
- `stdout`/`stderr`: text `gh` wrote to stdout and stderr. If the process
  was killed by a signal and produced no stderr of its own, this field is
  synthesized as `terminated by <signal>` instead of being left empty.
- `durationMs`: wrapper-measured process duration.
- `ghVersion`: the first line of `gh --version`, captured independently of
  `with.args` -- it is present even when the requested command itself
  fails.
- `timedOut`: `true` when `with.timeoutSeconds` was reached.
- `error`: present only when the action's own wrapper rejects the resolved
  input before ever invoking `gh` (for example, `with.timeoutSeconds` not
  an integer) or fails to start the process at all. As with `dbt@v1`, this
  is not reached in practice for a step that already passed the action's
  input schema: every wrapper-level check duplicates a constraint the
  schema already enforces (an array-of-strings `args`, a plain-object
  `env`, an in-range integer `timeoutSeconds`), so schema-valid input never
  reaches the wrapper's own validation code.

On `with.timeoutSeconds` being reached, the wrapper sends `SIGTERM`; if the
process is still running two seconds later, it sends `SIGKILL`. Unlike
`ffmpeg@v1` (which traps `SIGTERM` and exits with a real, positive exit
code), `gh` does not trap it and is actually killed by the signal -- but
the result's `exitCode` is still reported as `124` either way, since the
wrapper forces that value whenever `timedOut` is `true`.

A real `gh` failure -- missing authentication, an invalid token, an
unreachable host -- reports through the ordinary `exitCode`/`stdout`/
`stderr` fields with `ok: false` and no `error` object: the process started
and ran, it just exited non-zero. This includes `gh`'s own "not
authenticated" failure (exit code `4`), which every read or write command
in this spec's environment produces, since no `GH_TOKEN`/`GITHUB_TOKEN` is
ever configured -- `gh` refuses to attempt any API call at all without one,
regardless of whether the target data would otherwise be public.

### Downstream references

A later step reads a field of the result as `${<step id>.outputs.<path>}`
-- the step ID used directly as the reference's namespace. This matches
the behavior [Spec 060: Node Script Action](060-node-script.md), [Spec
061: Python Script Action](061-python-script.md), [Spec 062: DBT
Action](062-dbt.md), and [Spec 064: FFmpeg Action](064-ffmpeg.md) document,
confirming it is a property of remote action:-type steps generally, not
specific to one action. The strict `${steps.<step id>.outputs.<name>}`
form documented for declared step outputs (see [Spec 012: Step
Outputs](012-step-outputs.md)) -- and used in this action's own published
example -- does not resolve for a `github-cli@v1` step, for the same
reason it does not for the other remote actions above.

## Errors

### Validation

- An action reference missing its required `@version` suffix (for example,
  `github-cli` instead of `github-cli@v1`): an error containing `unknown
  action`. This is enforced generically for any action reference, not
  specifically for `github-cli`.

`dagu validate` does not resolve the remote action reference at all, so it
cannot check `github-cli@v1`'s own requirements (such as `with.args` being
required) -- a step missing `with.args` passes validate.

### Runtime

- `with.args` missing, or an empty array: an error containing `missing
  properties: ["args"]` or `less than 1`, raised when the action resolves
  its own input schema against the step's `with:` content -- before `gh`
  is ever invoked.
- `with.timeoutSeconds` outside `1`-`1800`: an error containing `greater
  than` or a comparable bound violation, raised by the same input-schema
  check.
- A real `gh` failure (missing authentication, a bad token, an unreachable
  host): the step fails (a non-zero exit), with the diagnostic JSON result
  described above still written to stdout, and no `error` object.
- Exceeding `with.timeoutSeconds`: the step fails (a non-zero exit) with
  `exitCode: 124` and `timedOut: true`.

## Related Specs

- Node script action, Python script action, DBT action, and FFmpeg action,
  for the equivalent remote-action execution model and downstream-reference
  behavior this action shares: [Spec 060: Node Script Action](060-node-script.md),
  [Spec 061: Python Script Action](061-python-script.md), [Spec 062: DBT
  Action](062-dbt.md), [Spec 064: FFmpeg Action](064-ffmpeg.md)
- Step outputs and reference syntax, for contrast with the bare-step-id
  reference form this action's result uses: [Spec 012: Step
  Outputs](012-step-outputs.md)

## Examples

Read a public repository's metadata and print it from a later step:

```yaml
steps:
  - id: repo
    action: github-cli@v1
    with:
      repo: dagucloud/dagu
      args: ["repo", "view", "--json", "name,description,url"]

  - id: print
    depends: repo
    run: echo "${repo.outputs.stdout}"
```

Pass a token for an authenticated call:

```yaml
secrets:
  - name: GH_TOKEN
    provider: env
    key: GH_TOKEN

steps:
  - id: latest_release
    action: github-cli@v1
    with:
      repo: dagucloud/dagu
      args: ["api", "repos/{owner}/{repo}/releases/latest", "--jq", ".tag_name"]
      env:
        GH_TOKEN: ${env.GH_TOKEN}
```
