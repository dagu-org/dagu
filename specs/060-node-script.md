# Spec 060: Node Script Action

## Status

Partial. Conformance covers Dagu's validation of the remote action reference.

## Scope

`node-script@v1` is an official remote action. This specification covers the
configuration boundary exposed by the Dagu CLI. Conformance tests do not
fetch the action or provision external runtimes.

Node.js execution, module imports, environment merging, timeout handling,
and result diagnostics belong to the action repository’s tests.

Remote input validation is outside this suite. Successful CLI validation
alone does not establish that inputs are valid for the remote action.

## Goal

Workflow authors can validate versioned remote action references without
fetching action code or provisioning a runtime.

## Behavior

`dagu validate` accepts a versioned `node-script@v1` reference and its
`with` mapping without fetching the remote action.

## Errors

The unversioned `node-script` name is rejected as an unknown action with a
nonzero exit code.

Runtime failures, timeout, abort, and cleanup are outside this configuration
conformance scope and belong to action and lifecycle tests.

## Examples

```yaml
steps:
  - action: node-script@v1
    with:
      script: "return input.value;"
      input:
        value: 42
```

## Conformance

`conformance/spec060_node_script/` checks a configured, versioned reference and
rejects the unversioned name. Both cases use `dagu validate` and require no
network access.
