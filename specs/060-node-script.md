# Spec 060: Node Script Action

## Status

Partial. Conformance covers Dagu's validation of the remote action reference.

## Scope

`node-script@v1` is an official remote action. This specification covers the
configuration boundary exposed by the Dagu CLI. Conformance tests do not
fetch the action or provision external runtimes.

Node.js execution, module imports, environment merging, timeout handling,
and result diagnostics belong to the action repository’s tests.

## Contract

- `dagu validate` accepts a versioned `node-script@v1` reference and its
  `with` mapping without fetching the remote action.
- The unversioned `node-script` name is rejected as an unknown action.
- Validation of remote action inputs requires the action's schema and occurs
  when the action runs. Successful CLI validation alone does not establish
  that those inputs are valid for the remote action.

## Example

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
