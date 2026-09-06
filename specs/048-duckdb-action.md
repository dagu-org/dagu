# Spec: DuckDB and Action Bundles

## Status

Partially implemented.

Conformance accepts a `duckdb@v1` reference and exercises Dagu's action
input/output boundary with a local bundle. It does not download or run DuckDB.
SQL behavior and tool provisioning belong in action and executor tests.

## Scope

This spec covers versioned action references and manifest input/output validation.
It does not define DuckDB's SQL dialect, tool installation, Git transport, or
remote repository availability.

## Goal

Workflow authors can call reusable actions through a consistent input/output
contract.

## Behavior

`dagu validate` accepts an official versioned reference such as `duckdb@v1`
without fetching the action. Local bundles can be selected with
`source:./directory@version`.

Action fields appear directly under `with`, such as `with.query` or
`with.message`. The action manifest validates that object against its `inputs`
schema. Successful action results are validated against its `outputs` schema
and emitted as JSON on stdout.

## Errors

A source reference without a version fails validation. Input and output schema
violations fail execution with diagnostics identifying the corresponding action
schema. Schema-library wording beyond that identification is not normative.

## Examples

```yaml
steps:
  - action: duckdb@v1
    with:
      query: SELECT 42 AS answer
```
