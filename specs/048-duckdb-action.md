# Spec: DuckDB and Action Bundles

## Status

Implemented.

## Scope

This spec covers two things:

- versioned action references and manifest input/output validation, using
  a local bundle (`source:./directory@version`), with no network access
- the observed behavior of the official remote action `duckdb@v1`
  (`dagucloud/duckdb`) itself, resolved and run for real, the same way
  specs 060-067 cover their own remote actions or built-in executors

Like `node-script@v1` ([Spec 060: Node Script Action](060-node-script.md))
and the other remote actions in this family, `duckdb@v1` is not a built-in
Go executor: referencing it clones `https://github.com/dagucloud/duckdb`
at the given tag and provisions its own pinned `duckdb/duckdb@v1.5.2` tool
via the project's aqua-based tool manager on first use. Unlike those
actions' Node.js wrappers, `duckdb@v1`'s own `workflow.yaml` runs a plain
POSIX shell script that `exec`s the real `duckdb` CLI directly and captures
its stdout through Dagu's own declared `stdout: {outputs: {field:
result}}` mechanism ([Spec 012: Step Outputs](012-step-outputs.md)) --
there is no `ok`/`exitCode`/`error` JSON wrapper at all, just `{"result":
"<raw duckdb stdout>"}`.

This spec covers:

- `with.query` (required), `with.database` (optional; omitted or
  `:memory:` for a transient in-memory database), `with.workdir`,
  `with.format` (one of `json`, `csv`, `table`, `markdown`, `line`,
  `list`, `column`; default `json`), and `with.readonly`
- that `with.database`, when it names a real file path, persists across
  separate `duckdb@v1` invocations -- a later step with `with.readonly:
  true` sees an earlier step's writes to the same file
- that `with.query`'s text is not resolved by Dagu's own value resolution
  before it reaches the action (unlike `node-script@v1`'s `with.script`):
  a literal `$NAME`-shaped token in `with.query` survives unresolved into
  the query `duckdb` actually runs
- that a real `duckdb` SQL error reports through the ordinary
  `result`/exit-code channel, not a wrapper error object: `duckdb` (run
  with `-bail`) exits non-zero and writes its own error to stderr, while
  `result` reflects whatever `duckdb` wrote to stdout (nothing, for a
  failed query)
- that a later step reads a result field as `${<step id>.outputs.<path>}`
  (a bare-step-id reference), the same form confirmed for the other
  remote actions and the harness executor in this family, and not via the
  strict `${steps.<step id>.outputs.<name>}` form
- that `dagu validate` never resolves the remote action reference: a
  `duckdb@v1` step passes validate regardless of its `with:` content, and
  `with.query` being required is enforced only once the step actually runs

This spec does not define:

- DuckDB's own SQL dialect or CLI output format details beyond what the
  action surfaces (`with.format` selects one of `duckdb`'s own output
  modes; this spec does not define each mode's exact formatting)
- the duckdb action's own implementation, versioning, or release process --
  this spec treats `duckdb@v1` as an external dependency and documents its
  observed contract
- the generic remote-action resolution mechanism itself, covered in more
  depth by [Spec 060: Node Script Action](060-node-script.md)
- performance or caching behavior of the action's own tool provisioning

## Goal

Workflow authors can call reusable actions through a consistent input/output
contract, and specifically run real DuckDB SQL as a Dagu step without
installing DuckDB or managing its provisioning themselves.

## Behavior

`dagu validate` accepts an official versioned reference such as `duckdb@v1`
without fetching the action. Local bundles can be selected with
`source:./directory@version`.

Action fields appear directly under `with`, such as `with.query` or
`with.message`. The action manifest validates that object against its
`inputs` schema. Successful action results are validated against its
`outputs` schema and emitted as JSON on stdout.

For `duckdb@v1` specifically: the result's `result` field is `duckdb`'s own
raw stdout in the format `with.format` selected -- for the default `json`
format, that is `duckdb`'s own `-json` output (a JSON array of row
objects, as text), not a Dagu-specific envelope. A statement that produces
no rows (a `CREATE TABLE`/`INSERT`, for example) leaves `result` empty.
`with.workdir`, when set, is the directory `duckdb` runs in; it has no
effect on the query itself unless the query depends on the working
directory some other way (for example a relative `with.database` path).

## Errors

A source reference without a version fails validation. Input and output schema
violations fail execution with diagnostics identifying the corresponding action
schema. Schema-library wording beyond that identification is not normative.

For `duckdb@v1` specifically:

- `with.query` missing, or `with.format` set to a value other than the six
  named above: an error containing `missing properties: ["query"]` or
  `does not equal any of`, raised when the action resolves its own input
  schema against the step's `with:` content -- before `duckdb` is ever
  invoked.
- A real `duckdb` SQL error (a missing table, invalid syntax, and so on):
  the step fails (a non-zero exit) with `duckdb`'s own error text on
  stderr and an empty `result`, not a wrapper error object -- `duckdb`
  itself ran and failed.

## Examples

```yaml
steps:
  - action: duckdb@v1
    with:
      query: SELECT 42 AS answer
```

Persist data across steps and read it back read-only:

```yaml
steps:
  - id: seed
    action: duckdb@v1
    with:
      database: /data/warehouse.duckdb
      query: "CREATE TABLE t(id INTEGER, name VARCHAR); INSERT INTO t VALUES (1,'a');"

  - id: read
    depends: seed
    action: duckdb@v1
    with:
      database: /data/warehouse.duckdb
      readonly: true
      format: csv
      query: "SELECT * FROM t ORDER BY id;"

  - id: print
    depends: read
    run: echo "${read.outputs.result}"
```
