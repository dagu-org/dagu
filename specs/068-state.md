# Spec: State Executor

## Status

Implemented.

## Scope

Like the harness executor ([Spec 067: Harness Executor](067-harness.md)),
`state` is a built-in Go executor, not a remote action: it reads and
writes Dagu's own persistent state store directly, with no external
process and no network access at all.

This spec covers:

- the `action: state.get`/`state.set`/`state.delete`/`state.list`/
  `state.diff` step shapes and their `with:` fields: `scope`, `namespace`,
  `key`, `prefix`, `value`, `default`, `expected_version`, `create_only`,
  `required`, `update`, `limit`, `include_values`
- the four scopes state can live in -- `dag` (default), `root_dag`,
  `global`, and `custom` -- and, specifically, that `dag` scope namespaces
  state under the *current* DAG's own name (isolated per sub-dag), while
  `root_dag` scope namespaces it under the *root* dag-run's name, shared by
  every DAG in that run's own nested-call tree
- that state written with `scope: global` is durable across entirely
  separate `dagu start` invocations, not just across steps within one run
- optimistic concurrency (`with.expected_version`, `with.create_only`) and
  `state.diff`'s change-detection and conditional-write behavior
  (`with.update`)
- capturing JSON stdout through a declared `output: NAME` and consuming it
  in a downstream step
- that, unlike the remote actions in specs 060-066 and like the harness
  executor, `dagu validate` checks some of a state step's configuration
  (that `with.key` is present for `get`/`set`/`delete`/`diff`, and
  `with.value` is present for `set`/`diff`) without running anything --
  but scope-specific rules (a `custom` scope needing `with.namespace`,
  `with.key`'s character restrictions) surface only at run time

This spec does not define:

- the state store's own on-disk format, or how it composes with other
  persisted DAG-run state
- distributed/coordinator-mediated state access, or concurrent-writer
  behavior beyond what `expected_version`/`create_only` establish

## Goal

Workflow authors share small pieces of JSON state between steps, between
separate runs of the same DAG, or across a whole nested-call tree, without
standing up an external store (Redis, a database, a file on a shared
volume) for data that only needs to outlive one step.

## Behavior

### Operations

- `state.get` reads `with.key`. If the key exists, the result reports
  `found: true` with the entry's `value`, `version`, and `hash`. If it does
  not exist: with `with.required: true`, the step fails; otherwise the
  result reports `found: false`, with `value` set to `with.default` when
  given (omitted otherwise).
- `state.set` writes `with.value` at `with.key`. `with.create_only: true`
  fails if the key already exists. `with.expected_version` fails
  (a conflict) if the key's current version does not match. The result
  reports the new `version`, `hash`, and `created` (`true` only when this
  write created the key, that is, its resulting version is `1`).
- `state.delete` removes `with.key` and reports whether it existed
  (`deleted: true`/`false`) -- deleting a key that does not exist is not
  an error.
- `state.list` returns every entry in scope (optionally filtered by
  `with.prefix`, and capped at `with.limit`), each with its `key`,
  `version`, `hash`, `createdAt`, `updatedAt`, and, only when
  `with.include_values: true`, its `value`.
- `state.diff` compares `with.value` (normalized and hashed) against the
  entry currently stored at `with.key`. The result reports `changed`
  (`true` when the key did not exist yet, or its hash differs) and
  `foundPrevious`, with `previous`/`previousVersion` when a prior entry
  existed. When `changed` is `true` and `with.update` is not `false`
  (the default), it also writes `with.value` as the new entry -- exactly
  like `state.set` -- and the result's `version`/`hash` reflect that write.
  With `with.update: false`, no write happens: `version`/`hash` still
  describe whichever entry (previous or none) already existed.

### Scope

`with.scope` selects where a key lives, and defaults to `dag`:

- `dag`: namespaced under the *current* DAG's own name. A parent DAG and a
  child DAG it calls (`action: dag.run`) each get their own, isolated
  `dag`-scoped namespace -- the parent cannot see a key its child wrote
  under `dag` scope, and vice versa.
- `root_dag`: namespaced under the *root* dag-run's own DAG name --shared
  by every DAG in that run's nested-call tree, however deep. A child
  writing under `root_dag` scope is visible to its parent (and any
  sibling) reading the same key under `root_dag` scope.
- `global`: namespaced under `with.namespace`, or a fixed default
  namespace when `with.namespace` is not given. Not scoped to any DAG or
  run at all -- entries persist across entirely separate `dagu start`
  invocations, the same way a value in an external key-value store would.
- `custom`: namespaced under `with.namespace`, which is required for this
  scope.

### Result

A state step writes one JSON object to stdout, containing the fields listed
for its operation above. A declared `output: NAME` makes that JSON available
to downstream steps, following [Spec 012: Step Outputs](012-step-outputs.md).

## Errors

### Validation

`dagu validate` checks, without running anything:

- `with.key` missing for `state.get`/`state.set`/`state.delete`/
  `state.diff`: `key is required`.
- `with.value` missing for `state.set`/`state.diff`: `value is required
  for <operation>`.
- `with.limit` negative: `limit must be greater than or equal to zero`.

It does not check `with.scope: custom` requiring `with.namespace`, or
`with.key`/`with.namespace`'s character restrictions -- those are runtime
checks (below).

### Runtime

- `with.scope: custom` without `with.namespace`: `namespace is required
  for custom scope`.
- `with.key` or `with.namespace` containing a path separator, or equal to
  `.`/`..`, or (for `with.key`) starting or ending with `/`: `invalid key
  "<value>"` / `invalid namespace "<value>"`.
- `state.get` with `with.required: true` against a missing key: `dag
  state: not found`.
- `state.set`/`state.diff` with `with.create_only: true` against an
  existing key, or a stale `with.expected_version`: `dag state: conflict`.

## Related Specs

- Harness executor: [Spec 067: Harness Executor](067-harness.md)
- Step outputs and reference syntax, for the standard `output: NAME`
  mechanism this executor uses instead of a JSON outputs contract: [Spec
  012: Step Outputs](012-step-outputs.md)

## Examples

Share a value across separate runs with global scope:

```yaml
steps:
  - id: record_last_run
    action: state.set
    with:
      scope: global
      key: last_successful_run
      value: "${DAG_RUN_ID}"
```

Detect whether a value changed since the last run:

```yaml
steps:
  - id: check_config
    action: state.diff
    with:
      key: config_hash
      value: ${CONFIG_JSON}
    output: DIFF_RESULT

  - id: print_diff
    depends: check_config
    run: echo "${DIFF_RESULT}"
```
