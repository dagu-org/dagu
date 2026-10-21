# RFC 003: Remove Workspaces

- **Status:** Proposed
- **Date:** 2026-10-09
- **Affected areas:** `workspace`, `auth`, `spec` (base config), `secret`,
  `profile`, notification routes, incident policies, wiki, `audit`, `api/v1`,
  `ui`, and `cmn/config`

## Summary

Remove workspaces in v3.0. The `workspace` label becomes an ordinary label.

v2.22 warns wherever workspaces are in use. v3.0 refuses to start while
workspace-scoped settings exist that would change how a workflow runs, and
turns workspace-restricted users and API keys into viewers. There is no
migration tool.

## Problem statement

Since v2.1.0, a workspace has scoped a group of DAGs across many subsystems:
access grants, base config, secrets, profiles, notification routes, incident
policies, wiki paths, and audit. The API spec mentions it about 270 times and
the UI in 85 files.

Dagu Cloud also uses "workspace" for the account that servers connect to.
Keeping both would give one word two levels in the same product.

## Goals

1. Remove the concept from the engine, API, and UI.
2. Never change silently which settings or secrets a workflow runs with.
3. No migration tool; the operator moves what remains, guided by exact
   messages.

## Non-goals

- A replacement grouping or per-group access model.
- `internal/runtime/workspacebundle`, which packs directories for distributed
  runs and is unrelated.

## What changes in v3.0

| Area | v2 | v3.0 | Risk if ignored | Guard |
| --- | --- | --- | --- | --- |
| `workspace` DAG label | Scopes the DAG | Ordinary label | None | None |
| Workspace base config (`<dags>/workspaces/<name>/base.yaml`) | Merged after the global base | Not read | Workflows lose env, defaults, SMTP | Refuse to start |
| Workspace secrets and profile entries (`/profiles/_workspaces/…`) | Override global values | Gone | A workflow resolves the global value of the same name: wrong credentials, silently | Refuse to start |
| Workspace notification routes and incident policies | Route alerts per workspace | Gone | Alerts stop or go elsewhere | Refuse to start |
| OIDC and proxy `workspace_mappings`, `default_workspace_access` | Grant workspace roles at login | Unknown keys | Logins map differently | Config error naming the key |
| Users and API keys with `workspace_access.all: false` | Global viewer plus roles per workspace | Global viewer on everything | They lose write access in their workspace and gain read access to every workflow, run, and log | Start; log each one; admin banner until acknowledged |
| Per-DAG wiki directory (`<wiki>/<workspace>/<dag>`) | Scoped by workspace | `<wiki>/<dag>` | The DAG no longer finds its pages | Move at startup when the target is free; otherwise log the conflict |
| Wiki pages under `<workspace>/` | Workspace-scoped | Ordinary folders | None | None |
| Audit `workspace` | Written and filterable | Kept in old entries; no longer written | None | None |
| API: `/workspaces*`, `/settings/workspaces/*`, `/notification-routes/workspaces/*`, `/incident-policies/workspaces/*`, `/profiles/_workspaces/*` | Present | Removed (`404`) | Clients break | Release notes |
| API: `?workspace=` on 19 paths; `workspaceAccess` on users and API keys | Present | Removed from the spec and ignored | None | None |
| UI: workspace selector, access editor, base config page | Present | Removed | None | None |

The DAG discovery skip for `workspaces/` (`persis/file/dag/discovery.go`) stays
in v3.0, so a leftover `base.yaml` never loads as a DAG while the guard
reports it.

### Refusal message

```text
dagu: workspaces were removed in v3.0, and these settings would no longer apply:
  base config      dags/workspaces/billing/base.yaml
  secret           DB_PASSWORD (workspace billing)
  incident policy  workspace billing
Move them into the DAG files or the global settings, then delete them.
```

The message lists every item, not the first one found.

## v2.22 deprecation

- A startup warning listing each item from the table that is in use.
- An admin banner with the same list.
- Release notes that point to the table above.
- No new workspace features.

## Compatibility and rollout

| Release | Ships |
| --- | --- |
| v2.22 | Warnings and banner. |
| v3.0 | Removal with the guards above. |

## Verification and acceptance criteria

1. A v2 data directory with no workspace settings starts on v3.0 unchanged.
2. Each refusal row in the table blocks startup, and the message lists every
   item.
3. A restricted user becomes a viewer of every workflow, is logged at startup,
   and appears in the admin banner.
4. A per-DAG wiki directory moves when the target is free and is reported
   when it is not.

## Alternatives considered

- **Keep workspaces and hide them in the UI.** The API, access model, and
  secret scoping keep their cost.
- **A migration tool.** Merging workspace base config, secrets, and routes
  into global settings requires choices only the operator can make, such as
  two workspaces holding different values for the same secret name.
- **Replace them with label-based or folder-based grants.** A new access model
  belongs in its own RFC, if one is needed.

## Consequences

- Upgrading to v3.0 needs a short manual step for installs that used
  workspace settings.
- Users previously limited to one workspace can read every workflow.
- "Workspace" is free to mean only the Dagu Cloud account.
