# Spec: DuckDB and Action Bundles

## Status

Partially implemented.

Conformance accepts a `duckdb@v1` reference and exercises Dagu's action
input/output boundary with local bundles and a loopback Git repository.
It does not download or run DuckDB. SQL behavior, tool provisioning, Git
server policy, and cache mechanics belong in action and executor tests.

## Scope

This spec covers versioned action references and manifest input/output
validation. `source:./directory@version` selects a local bundle;
`source:<git-url>@version` selects an action from a Git repository.

It does not define DuckDB's SQL dialect, tool installation, or a Git host's
availability and authentication requirements.

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

A Git source action executes the requested tag, branch, or commit using the
same manifest input/output contract as a local action.

## Errors

A source reference without a version fails validation. Input and output schema
violations fail execution with diagnostics identifying the corresponding action
schema. Schema-library wording beyond that identification is not normative.

An unavailable repository or revision fails execution with a diagnostic
identifying the action clone or checkout failure. Git's diagnostic wording
is not normative.

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
