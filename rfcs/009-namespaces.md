---
id: "009"
title: "Namespaces"
status: draft
---

# RFC 009: Namespaces

## Summary

Introduce namespaces as a first-class isolation boundary in Dagu, enabling multiple teams or environments to share a single Dagu instance with full separation of DAGs, runs, configuration, and access control.

## Motivation

Currently, Dagu operates as a single-tenant system:

1. **All DAGs share one flat directory** — no isolation between teams, projects, or environments
2. **RBAC is global** — a user's role applies to every DAG in the instance
3. **Runs and logs are co-mingled** — no way to scope operational data to a team or project
4. **Configuration is instance-wide** — base configs, secrets providers, and queue settings apply globally

This makes it impractical to run a shared Dagu instance for multiple teams. Organizations must either deploy separate instances per team (operational overhead) or accept that all users see and can interact with all workflows (security/usability concern).

### Use Cases

- **Platform teams** hosting Dagu as a shared service for multiple product teams
- **Environment isolation** (dev / staging / prod) within a single deployment
- **Compliance boundaries** where certain workflows and their data must be isolated

## Proposal

### Namespace Concept

A namespace is a named isolation boundary that scopes:

- **DAGs** — each DAG belongs to exactly one namespace
- **DAG runs and history** — run data is stored and queried per namespace
- **Logs** — execution logs are partitioned by namespace
- **Queues** — queue definitions and state are namespace-local; the same queue name in different namespaces are independent
- **Suspend flags** — DAG suspend state is per-namespace
- **Webhooks** — webhook URLs include the namespace for routing
- **Configuration** — base config, secrets, and environment defaults can be set per namespace

Namespace names must match `[a-z0-9][a-z0-9-]*[a-z0-9]`, with a maximum length of 63 characters. The name `default` is reserved for the built-in namespace.

Every Dagu instance has a built-in `default` namespace for backward compatibility. Existing single-tenant deployments continue to work without changes.

### Directory-Based Storage

Namespaces map directly to the filesystem. Each namespace is a subdirectory under the DAGs root and data root:

```
# DAG definitions
~/.config/dagu/dags/
├── default/          # built-in namespace
│   ├── etl-daily.yaml
│   └── reports.yaml
├── team-alpha/
│   ├── ingest.yaml
│   └── transform.yaml
└── team-beta/
    └── analytics.yaml

# DAG runs and operational data
~/.local/share/dagu/data/
├── default/
│   ├── dag-runs/
│   ├── proc/
│   └── queue/
├── team-alpha/
│   ├── dag-runs/
│   ├── proc/
│   └── queue/
└── team-beta/
    └── ...
```

A namespace is created by creating its directory. No separate registration step is required.

### DAG Identity

A DAG is uniquely identified by the pair `(namespace, name)`. Within a single namespace, DAG names must be unique (same as today). Across namespaces, the same DAG name can exist independently.

API and CLI references use the format `namespace/dag-name` (e.g., `team-alpha/ingest`). Omitting the namespace implies `default`.

### Namespace-Scoped RBAC

Extend the existing role system to support per-namespace role bindings:

| Concept | Description |
|---------|-------------|
| **Global role** | Applies across all namespaces (current behavior) |
| **Namespace role** | Applies only within a specific namespace |
| **Namespace admin** | Can manage DAGs, runs, and users within their namespace |

A user can hold different roles in different namespaces. Global admin remains the superuser across all namespaces. A user with no explicit namespace binding has no access to that namespace (deny-by-default).

Each user gains a `namespaceRoles` mapping in addition to their existing global `role`. The existing role system (admin, manager, operator, viewer) is reused — no new role types are introduced.

Example: Alice has `viewer` as her global role, `admin` in `team-alpha`, and `operator` in `team-beta`. Bob has `admin` globally (superuser across all namespaces).

API keys are global-only in the initial implementation. Namespace-scoped API keys may be added later.

### API Changes

All existing API endpoints gain an optional `namespace` path prefix:

```
GET  /api/v1/namespaces                        # list namespaces
GET  /api/v1/namespaces/{ns}/dags              # list DAGs in namespace
POST /api/v1/namespaces/{ns}/dags              # create DAG in namespace
GET  /api/v1/namespaces/{ns}/dags/{name}       # get DAG
POST /api/v1/namespaces/{ns}/dags/{name}/runs  # trigger run
```

Requests without a namespace prefix operate on `default` for backward compatibility.

### CLI Changes

Existing commands accept namespace via the `namespace/dag-name` format or a `--namespace` flag:

```bash
dagu start team-alpha/ingest           # run DAG in namespace
dagu status team-alpha/ingest          # DAG status in namespace
dagu sync pull --namespace team-alpha  # sync specific namespace
dagu namespace list                    # list all namespaces
dagu namespace create staging          # create namespace
```

### UI Changes

- Namespace selector in the navigation bar
- Namespace-scoped views for DAGs, runs, and logs
- Admin panel for namespace management and role assignment

### Scheduler

The scheduler discovers and schedules DAGs across all namespaces. It scans namespace subdirectories under the DAGs root and watches each for file changes. Each scheduled DAG carries its namespace context through execution.

### Sub-DAG References

Cross-namespace sub-DAG calls are **not supported**. All `run` references resolve within the same namespace as the calling DAG. This keeps execution boundaries simple and avoids complex cross-namespace dependency graphs.

```yaml
steps:
  - name: call-local
    run: common-cleanup             # resolves within the same namespace
```

If teams need to share workflows, they should duplicate the DAG into each namespace or extract the shared logic into a script/binary invoked via a command step.

### Namespace Configuration

Each namespace can optionally include a `_namespace.yaml` configuration file:

```yaml
# team-alpha/_namespace.yaml
description: "Team Alpha workflows"
baseConfig: base.yaml              # namespace-level base config
defaults:
  queue: team-alpha-queue
  workingDir: /data/team-alpha
```

### Git Sync

Git sync becomes namespace-scoped. Each namespace can configure its own git repository, branch, and sync settings independently via `_namespace.yaml`:

```yaml
# team-alpha/_namespace.yaml
gitSync:
  enabled: true
  repository: "github.com/org/team-alpha-dags"
  branch: main
  auth:
    type: token
    token: "${TEAM_ALPHA_GIT_TOKEN}"
  autoSync:
    enabled: true
    interval: 300
```

Alternatively, a single repository can serve multiple namespaces by mapping subdirectories to namespaces using the `path` field:

```yaml
# Shared repo, different subdirectories per namespace
# team-alpha/_namespace.yaml
gitSync:
  repository: "github.com/org/all-dags"
  path: "team-alpha"           # sync only this subdirectory

# team-beta/_namespace.yaml
gitSync:
  repository: "github.com/org/all-dags"
  path: "team-beta"
```

Key behaviors:

- **Sync state is per-namespace** — each namespace maintains its own `state.json` under `{dataDir}/{namespace}/gitsync/`
- **Sync operations are namespace-scoped** — `dagu sync pull --namespace team-alpha` only pulls for that namespace
- **Global sync** — `dagu sync pull` (no namespace) syncs all namespaces that have git sync enabled
- **Permissions** — sync operations require `admin` or `manager` role within the target namespace
- **Conflict resolution** — conflicts are tracked and resolved per namespace independently

API endpoints gain namespace scoping:

```
POST /api/v1/namespaces/{ns}/sync/pull
POST /api/v1/namespaces/{ns}/sync/publish-all
GET  /api/v1/namespaces/{ns}/sync/status
```

The existing instance-level `gitSync` config in the global config file continues to work and applies to the `default` namespace only.

### AI Agent

The AI agent becomes namespace-aware. When a user interacts with the agent, the agent operates within the context of the user's currently selected namespace:

- **Tool permissions follow namespace RBAC** — the agent's `bash`, `patch`, and `read` tools are restricted to DAGs and files within the active namespace, enforced by the user's namespace role (not their global role)
- **DAG context is namespace-scoped** — when the agent references or operates on DAGs, it resolves names within the active namespace by default
- **No cross-namespace operations** — the agent cannot access DAGs or files outside the active namespace; users must start a new session to work in a different namespace
- **System prompt includes namespace context** — the LLM is informed of the active namespace, the user's role within it, and available DAGs scoped to that namespace

Example agent interaction:

```
User (namespace: team-alpha, role: manager):
  "Create a new DAG that runs the ETL pipeline daily"

Agent:
  → Creates DAG in team-alpha/ namespace
  → Tools enforced: can create/edit DAGs (manager role)
  → Cannot access DAGs outside team-alpha
```

Each session is locked to the namespace in which it was started. The agent cannot switch namespaces mid-session. Users must start a new session to work in a different namespace. Session history is visible across namespaces the user has access to, but each session is tagged with its originating namespace.

Audit logging (RFC 002) includes the namespace in every agent action record for traceability.

## Migration

Existing installations upgrade seamlessly:

1. All current DAGs are moved into `default/` subdirectory
2. All current run data is moved into `default/` subdirectory (dag-runs, proc, queue, suspend flags)
3. Git sync state is moved into `default/` subdirectory
4. Agent session history is tagged with the `default` namespace
5. Existing users receive their current global role unchanged
6. No configuration changes are required

An automatic migration runs on first startup after upgrade.

## Design Decisions

1. **Namespace deletion requires empty namespace** — a namespace must have all DAGs and run history removed before it can be deleted. No cascading deletion.
2. **No cross-namespace triggers** — DAGs cannot trigger or reference DAGs in other namespaces. All `run` references resolve within the same namespace. This avoids complex cross-namespace dependency graphs.
3. **Secrets are per-namespace by default** — secrets are configured in `baseConfig` which is already namespace-scoped. No additional mechanism needed.
4. **Git sync pushes independently per namespace** — when multiple namespaces share the same git repository, each namespace pushes independently. No coordination between namespaces.
5. **Agent sessions are locked to a single namespace** — the agent cannot switch namespaces mid-session. Users start a new session to work in a different namespace.

## Out of Scope

1. **Resource quotas** — namespace-level limits on concurrent runs, DAG count, or storage are not included in the initial implementation. May be revisited in a future RFC.
