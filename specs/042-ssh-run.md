# Spec: SSH Run Action

## Status

Partially implemented.

Conformance covers action configuration without an SSH server. Authentication,
remote shell behavior, and cancellation belong in executor unit and integration
tests.

## Scope

This spec defines the configuration boundary for `ssh.run`.
It does not define SSH protocol behavior, host-key management, SFTP transfers,
or the lifetime of remote processes after disconnection.

## Goal

Workflow authors can validate an SSH command before connecting to its host.

## Behavior

`ssh.run` accepts connection fields under `with` and requires `with.command`.
The command follows [Spec 014: Step Run Command](014-step-run-command.md).
`dagu validate` accepts a configured SSH action without contacting the host or
requiring its private key to exist locally.

## Errors

Omitting `with.command` fails validation with a diagnostic naming that field.

## Examples

```yaml
steps:
  - action: ssh.run
    with:
      host: example.invalid
      user: deploy
      key: ~/.ssh/id_ed25519
      command: echo hello
```
