# Spec: Runtime Profiles

## Status

Partially implemented.

Conformance covers CRUD, enable/disable, and basic variable application at
the CLI level (`conformance/cli/profile_test.go`). This spec adds the layered
defaults precedence (global, workspace, and selected profile), webhook-header
profile selection, and how a selected profile's entries interact with a
DAG's own `env:` and `secrets:` fields.

## Scope

This spec covers how a runtime profile's variables and secrets are composed
into a DAG-run's environment, across three layers:

- Global defaults, set once for the whole server.
- Workspace defaults, set per workspace and applied only to runs whose DAG
  carries a matching `workspace` label.
- The explicitly selected profile, named by `--profile`, the `profile` field
  on `POST /dag-runs`, or (for a webhook-triggered run) the `X-Dagu-Profile`
  header.

It also covers precedence against a DAG's own `env:` and `secrets:` fields,
and masking of profile secret values.

It does not define the workspace-managed secret registry (`ref:`-style
secret entries, see spec 069), profile CRUD/enable-disable/basic variable
application (already covered by `conformance/cli/profile_test.go`), or
webhook HMAC signing.

## Goal

Workflow authors and operators can supply shared configuration and secrets
to DAG-runs from outside the DAG file, at different scopes (global,
workspace, or per-run), without editing every DAG that needs them.

## Behavior

A run's final environment composes, in order (later entries override
earlier ones with the same name):

1. Global default profile variables.
2. Global default profile secrets.
3. Workspace default profile variables and secrets, applied only when the
   DAG's `labels` include a `workspace` entry naming a workspace with
   configured defaults.
4. The DAG's own `env:` field.
5. The selected profile's variables.
6. The selected profile's secrets.
7. The DAG's own `secrets:` field.

A DAG's own `secrets:` field is therefore the final, highest-precedence
layer: it overrides a same-named entry from any profile layer. The selected
profile overrides the DAG's own `env:` field. Global and workspace defaults
are managed only through the HTTP API (`/profiles/_global/...` and
`/profiles/_workspaces/{name}/...`); no CLI command reaches them.

A webhook-triggered run selects a profile with the `X-Dagu-Profile` request
header, restricted to the profiles an admin allow-listed for that webhook
(`PUT /dags/{fileName}/webhook/profile-selection`). A run with no explicit
profile selection uses the webhook's configured default profile, if any.

A resolved profile secret is masked in run output exactly like a DAG-level
secret: the literal value is replaced with `*******` in the step's stdout
and stderr log files, the rendered status tree, and stored run status.

## Errors

A webhook request naming a profile outside its configured allow-list is
rejected with `403` and does not create a DAG-run. An unresolvable selected
profile (unknown name, or disabled) fails the run before any step starts.

## Examples

```yaml
labels:
  - workspace: acme
steps:
  - command: echo "$SHARED_VAR"
```

Selecting a profile for a local run:

```sh
dagu start --profile=deploy-prod my-dag.yaml
```
