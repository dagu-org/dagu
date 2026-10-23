# RFC 003: Replace Workspaces with Projects

- **Status:** Proposed
- **Date:** 2026-10-09, revised 2026-10-23
- **Affected areas:** `workspace`, `auth`, `spec` (base config), `secret`,
  `profile`, notification routes, incident policies, wiki, `audit`,
  `gitsync`, `service/scheduler`, `service/coordinator`, `service/mcp`,
  `api/v1`, `proto`, `ui`, and `cmn/config`

## Summary

Replace workspaces with projects in v3.0. A project is a directory with the
same layout as `DAGU_HOME`. It holds its DAGs, runs, logs, base config,
secrets, profiles, notification routes, incident policies, and wiki. Users,
API keys, authentication, and workers stay server-wide. One server process
serves every project.

`dagu migrate workspaces` moves each v2 workspace into a project. v3.0 refuses
to start while unmigrated workspace items remain. Installs without workspaces
run unchanged as the default project.

## Problem statement

Since v2.1.0, a workspace has scoped a group of DAGs across many subsystems:
access grants, base config, secrets, profiles, notification routes, incident
policies, wiki paths, and audit. Membership comes from the `workspace` label
on each DAG, and every workspace shares one set of stores. That design has
three problems.

1. **Every subsystem filters.** Each store, query, and API path checks the
   label. The API spec mentions workspaces about 270 times and the UI in 85
   files.
2. **The boundary leaks.** Unlabelled DAGs are visible to every user. Child
   runs and distributed tasks must carry their parent's workspace to load the
   right base config (`base_config_workspace` on the coordinator task).
3. **The name is taken.** Dagu Cloud uses "workspace" for the account that
   servers connect to. Keeping both would give one word two levels in the same
   product.

Feedback in [#3288](https://github.com/dagucloud/dagu/issues/3288) shows that
multi-team installs depend on what workspaces provide: roles per group,
server-side visibility limits, and base config shared within a group. Removing
workspaces without a replacement would keep those installs on v2.

## Goals

1. Keep per-group roles, visibility, base config, secrets, notification
   routes, and incident policies.
2. Make the group a hard boundary: a DAG belongs to the project whose
   directory holds it.
3. Keep most subsystems unaware of projects. Only startup wiring, the API, MCP,
   and CLI edge, and authorization resolve a project.
4. Leave installs without workspaces unchanged.
5. Migrate workspaces with one reviewed command, and never change silently
   which settings or secrets a workflow runs with.
6. Leave "workspace" to mean only the Dagu Cloud account.

## Non-goals

- A process per project.
- Users or authentication configured per project.
- `dag.run` across projects.
- Spreading projects across several scheduler processes.
- Renaming `internal/runtime/workspacebundle`, which packs directories for
  distributed runs and is unrelated.

## Projects

### Layout

```text
$DAGU_HOME/
  config.yaml            server-wide settings
  base.yaml              global base config
  dags/ data/ logs/      the default project
  projects/
    weather/
      base.yaml          project base config
      dags/ data/ logs/
    sim/
      ...
```

- A project directory has the same layout as `DAGU_HOME`, without
  `config.yaml`.
- `paths.projects_dir` sets where projects live. The default is
  `$DAGU_HOME/projects`, or the XDG data directory when `DAGU_HOME` is unset.
- The default project is the existing home. Its paths, including any set in
  `config.yaml`, do not change.
- Project names match `^[A-Za-z0-9_-]+$`, as workspace names do. `default` is
  reserved.
- A new directory under `projects/` is a new project. Admins can also create
  one in the UI or API.

### Scope

| Scope | Contents |
| --- | --- |
| Server | `config.yaml`, global `base.yaml`, users, API keys, OIDC and proxy authentication, license, Dagu Cloud connection, coordinator and worker state, audit log, queues defined in `config.yaml` |
| Project | DAG files, project `base.yaml`, runs, logs, artifacts, DAG state, suspend flags, DAG settings, secrets, profiles, notification routes, incident policies, wiki, per-DAG webhooks, other queues |

### Base config

A DAG's base config is the global `base.yaml`, then its project's `base.yaml`,
then the DAG file. The order matches v2's global, workspace, DAG. A child
started with `dag.run` belongs to its parent's project, so it gets the same
base config without carrying provenance.

### Identity and references

- A DAG is identified by its project and name. Names are unique within a
  project.
- `dag.run` resolves its target within the caller's project. A target in
  another project is a load error.
- CLI commands take `--project` (`DAGU_PROJECT`). The default is `default`.
- API paths and MCP tools that address DAGs, runs, secrets, profiles, routes,
  policies, or wiki pages take `project`. The default is `default`, so
  existing clients keep working against the default project.
- `/projects` lists and creates projects. The `/workspaces*`,
  `/settings/workspaces/*`, `/notification-routes/workspaces/*`,
  `/incident-policies/workspaces/*`, and `/profiles/_workspaces/*` paths and
  the `?workspace=` parameter are removed.
- Local runs start their child process with `--project`. `DAGU_HOME` stays the
  server root, so the child still reads `config.yaml` and the global base.
- Audit entries record the project. Old entries keep `workspace`.

### Access

- A grant is a user or group, a project, and a role below `admin`. A
  server-wide role applies to every project.
- A user's role in a project is the higher of the server-wide role and the
  project grant.
- A user with neither gets `404` for the project's DAGs, runs, logs, and wiki
  pages. The project does not appear in lists or search.
- `admin` is server-wide only.
- OIDC and proxy `project_mappings` and `default_project_access` replace
  `workspace_mappings` and `default_workspace_access` with the same shape.
  v3.0 reads the old keys with a warning. v3.1 drops them.

### UI

The workspace selector becomes a project selector that lists the projects the
user can see. The access editor and the base config page work per project.

## Scheduler

One scheduler process serves every project.

```text
scheduler process   HA lock, health server, service registry, signals, license
 └─ supervisor      starts and stops units as projects appear and disappear
     ├─ unit: default
     ├─ unit: weather
     └─ unit: sim
          entry reader, tick planner, queue processor, retry scanner,
          zombie detector, notification and incident monitors
```

- A unit is the current per-home scheduler, built from one project's stores.
  It does not know about other projects.
- The HA lock, health server, and service registration move out of the unit
  and exist once per process. Kept per unit, they would give each project its
  own lock and collide on the health port.
- Queues defined in `config.yaml` are shared across units through one limiter,
  as they are shared across workspaces in v2. Other queues are per project.
- One file watcher covers `projects/` and routes events by path. A watcher per
  project would reach the Linux inotify instance limit (128 by default) on
  servers with many projects.

## Distributed execution

- Dispatch tasks, leases, and status reports carry the project. The
  coordinator records attempts in that project's run store.
- Tasks already carry base config content and the profile name. Both now come
  from the task's project. Profile and secret values resolve from the task's
  project.
- `base_config_workspace` is removed from the task.
- Workers stay shared and do not read project directories.

## Migration

`dagu migrate workspaces` prints a plan and changes nothing. With `--apply`,
it carries out the plan. Planning is read-only, so it can run against a live
v2 install before the upgrade. Apply it with every Dagu process for that
`DAGU_HOME` stopped.

| v2 | v3.0 |
| --- | --- |
| DAG labelled `workspace=X` | `projects/X/dags/`, with its runs, logs, artifacts, DAG state, suspend flags, and DAG settings |
| DAG without a `workspace` label | Stays in the default project |
| `<dags>/workspaces/X/base.yaml` | `projects/X/base.yaml` |
| Secrets and profile entries of `X` | Project `X`'s secrets and profiles |
| Notification routes and incident policies of `X` | Project `X`'s routes and policies |
| Per-DAG wiki directory `<wiki>/X/<dag>`, pages under `<wiki>/X/` | Project `X`'s wiki |
| User or API key with roles in `X` | Grants on project `X` |
| User or API key with `workspace_access.all: false` | Also `viewer` on the default project, which keeps v2 read access to unlabelled DAGs |
| `workspace_mappings`, `default_workspace_access` | Read as `project_mappings`, `default_project_access` until v3.1 |
| `workspace` label on a moved DAG | Kept as an ordinary label; the file is not rewritten |

The plan lists these conflicts, and `--apply` refuses while any remain:

- A `dag.run` target that would land in another project.
- A workspace named `default`, or two names that differ only in case.
- A queue not defined in `config.yaml` that DAGs in different projects share.
  Defining it in `config.yaml` keeps it shared.

### Refusal message

v3.0 refuses to start while anything the migration would move remains.

```text
dagu: workspaces became projects in v3.0, and these items are not migrated:
  workflows        12 DAGs labelled workspace=billing
  base config      dags/workspaces/billing/base.yaml
  secret           DB_PASSWORD (workspace billing)
  incident policy  workspace billing
Run `dagu migrate workspaces` to review the plan, then apply it with --apply.
```

The message lists every item, not the first one found.

## Compatibility and rollout

| Release | Ships |
| --- | --- |
| v2.22 | Warnings and an admin notice that list workspace settings in use. |
| Next v2 release | The warning and notice say that v3.0 moves workspaces into projects, and stop asking operators to move settings by hand. |
| v3.0 | Projects, `dagu migrate workspaces`, and the startup refusal. |
| v3.1 | Drops the `workspace_mappings` and `default_workspace_access` aliases. |

v2 gets no new workspace features.

## Verification and acceptance criteria

1. A v2 data directory with no workspaces starts on v3.0 unchanged, as the
   default project.
2. After migration, each DAG runs with the same base config, secrets,
   profile, notification routes, and incident policies as on v2.
3. After migration, each user and API key can see and do what it could on v2,
   and no more.
4. A user without a role in a project gets `404` for its DAGs, runs, logs, and
   wiki pages, and does not see the project in lists or search.
5. `dag.run` to a DAG in another project fails at load.
6. One scheduler process schedules every project. A new project directory
   starts scheduling without a restart.
7. A distributed run of a project DAG records its status in that project.
8. v3.0 refuses to start while workspace items remain, and the message lists
   every item.
9. Planning a migration changes nothing on disk.

## Alternatives considered

- **Remove workspaces without a replacement.** The first revision of this
  RFC. Multi-team installs would lose roles per group, visibility limits, and
  shared base config, and would stay on v2.
- **Rename workspaces to projects and keep the label model.** Fixes the name
  only. Every subsystem keeps filtering, and the boundary still leaks.
- **Label- or folder-based grants on one set of stores.** The same filtering
  in every subsystem under a new name.
- **One Dagu instance per group.** Works today, since `DAGU_HOME` already holds
  every path. Each group then needs its own scheduler, coordinator, workers,
  authentication setup, and URL, and no one sees across groups. It remains
  supported.
- **A scheduler process per project.** Multiplies locks, ports, and
  registrations. Runs already execute in their own processes, so isolation
  barely improves.
- **Migrate automatically at startup.** Moving DAG files and run history
  without review is hard to undo.

## Consequences

- Multi-team installs keep roles per group, visibility limits, and base
  config, with membership fixed by location.
- Upgrading an install with workspaces takes one reviewed command.
- Project-scoped API paths, MCP tools, and CLI commands gain a project
  parameter. Clients that omit it address the default project.
- `dag.run` cannot cross projects.
- The scheduler gains a supervisor, and project units reuse the current
  scheduling logic.
- "Workspace" means only the Dagu Cloud account.

## Open questions

1. **Git sync.** One repository per server with `projects/<name>/` mirrored
   under its root, or one repository per project. One repository keeps a v2
   install's setup and shows the migration as a single change to publish.
2. **A server-wide cap on concurrent runs** across projects, beyond queues
   defined in `config.yaml`.
