# Spec: DuckDB and Action Bundles

## Status

Partially implemented.

Conformance accepts a `duckdb@v1` reference and exercises Dagu's action
input/output boundary with a local bundle. It does not download or run
DuckDB itself. Conformance does exercise the generic `source:<target>@version`
reference form's real git-clone transport, against a custom (non-official,
non-`dagucloud/*`) git-hosted action -- not just the local-directory
shortcut. SQL behavior and tool provisioning belong in action and executor
tests.

## Scope

This spec covers versioned action references and manifest input/output
validation, including:

- `source:./directory@version` and `source:file://...@version`, both of
  which resolve to a local directory tree directly, with no git operation
  at all
- `source:<git-url>@version`, where `<git-url>` is any git-clonable
  target that does *not* resolve to an existing local directory (an
  `http://`, `https://`, `ssh://`, `git://`, or SCP-like `user@host:path`
  URL, or a bare `owner/repo`/`name` official short name) -- this clones
  the repository for real, checks out the requested tag/branch/commit, and
  caches the result by its resolved commit SHA, regardless of whether the
  host is `github.com/dagucloud/*` or an arbitrary custom git remote

It does not define DuckDB's SQL dialect, tool installation, or any specific
git host's own availability or authentication requirements.

## Goal

Workflow authors can call reusable actions through a consistent input/output
contract, whether the action is one of Dagu's own official actions or a
custom action hosted in the workflow author's own git repository.

## Behavior

`dagu validate` accepts a versioned reference -- official
(`duckdb@v1`), local bundle (`source:./directory@version`), or a custom
git-hosted `source:<git-url>@version` -- without ever resolving it: it
does not fetch, clone, or otherwise reach the network for any reference
form, so validation completes regardless of whether the target host is
even reachable.

Action fields appear directly under `with`, such as `with.query` or
`with.message`. The action manifest validates that object against its `inputs`
schema. Successful action results are validated against its `outputs` schema
and emitted as JSON on stdout. This holds the same way for a custom
git-hosted action as for an official one: the manifest and workflow files
`dagu-action.yaml`/`dag:` name inside the cloned repository play the exact
same role regardless of where the repository came from.

For `source:<git-url>@version` specifically: the version is resolved to a
commit the same way for any git host -- `git ls-remote` against
`refs/tags/<version>` (preferring the peeled/annotated-tag commit),
`refs/tags/<version>` unpeeled, then `refs/heads/<version>`, falling back to
treating `<version>` itself as a full commit SHA. The resolved commit is
then checked out and cached under a directory keyed by the repository URL
and that commit SHA, so a later reference to the same repository and
version reuses the cached checkout rather than cloning again.

## Errors

A source reference without a version fails validation. Input and output schema
violations fail execution with diagnostics identifying the corresponding action
schema. Schema-library wording beyond that identification is not normative.

For `source:<git-url>@version` specifically:

- A version that names no tag, branch, or commit the repository has: the
  step fails with an error containing `did not match any file(s) known to
  git`.
- A repository the git host does not export, or does not exist: the step
  fails with an error containing `repository not exported` (or an
  equivalent transport-level failure) -- not a Dagu-specific message, since
  this is the underlying `git` command's own output.

## Examples

```yaml
steps:
  - action: duckdb@v1
    with:
      query: SELECT 42 AS answer
```

Reference a custom action hosted in the workflow author's own git
repository, not an official `dagucloud/*` one:

```yaml
steps:
  - action: "source:https://github.com/myorg/my-dagu-action.git@v1"
    with:
      message: hello
```
