# Spec: Docker Run Action

## Status

Partially implemented.

Conformance covers configuration acceptance and rejection without a Docker
daemon. Container execution, image pulls, and lifecycle behavior belong in
executor unit and integration tests.

## Scope

This spec defines the configuration boundary for `docker.run`.
It does not define Docker Engine behavior, registry authentication, container
cleanup, or the DAG-level `container` field.

## Goal

Workflow authors can validate container action configuration before running
workloads against a daemon.

## Behavior

- `with.image` selects an image for a new container.
- `with.container_name` can select an existing container. When supplied with
  `with.image`, it names the new container.
- At least one target is required.
- `with.exec` requires `with.container_name`.
- `with.command` accepts the command shapes defined by
  [Spec 014: Step Run Command](014-step-run-command.md).
- `dagu validate` accepts a valid image-based action without contacting a daemon.

## Errors

Missing target configuration fails validation or execution before a container
starts. The diagnostic identifies the missing target or required configuration.
Exec options without a container name fail validation with a diagnostic naming
`container_name`.

## Examples

```yaml
steps:
  - action: docker.run
    with:
      image: alpine:3
      command: echo hello
```
