# Spec: JQ Filter Action

## Status

Partially implemented.

## Scope

This spec covers basic `jq.filter` input selection, scalar output, and
configuration errors. The jq language, value-resolution permutations,
object formatting, and per-value error iteration belong to executor tests.

## Goal

Workflow authors can filter inline or file-backed JSON without installing
an external jq command.

## Behavior

`with.filter` supplies the query. `with.data` accepts inline data;
`with.input` names a JSON file. A `with.data` string beginning with
`file://` reads the named file. The `file://` shortcut applies only to
`with.data`; `with.input` is a filesystem path.

A scalar result is written as JSON followed by a newline. With
`with.raw: true`, strings are written without JSON quotes and a null result
writes exactly one newline.

## Errors

`dagu validate` rejects a missing `with.filter` and configurations that set
both `with.data` and `with.input`. It exits nonzero with an error identifying
the invalid configuration.

Filter-language errors, missing files, timeout, and abort behavior are
outside this conformance scope.

## Example

```yaml
steps:
  - action: jq.filter
    with:
      filter: .name
      data:
        name: World
      raw: true
```
