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

Namespaces map directly to the filesystem. Each namespace is a subdirectory under the DAGs root and data root, using a **short internal ID** (4-char hex, first 4 characters of the SHA256 hex digest of the namespace name) as the directory name. This avoids filesystem path length issues — namespace names can be up to 63 characters, and combined with DAG names and the hierarchical run storage (`YYYY/MM/DD/dag-run_*/attempt_*/`), raw names could exceed OS path limits.

This approach is consistent with the existing pattern in the codebase where `SafeName()` + 4-char SHA256 suffix is used for DAG run directory names.

```
# DAG definitions
~/.config/dagu/dags/
├── 6a5f/             # "default" → sha256("default")[:4] = 6a5f
│   ├── etl-daily.yaml
│   └── reports.yaml
├── a1c3/             # "team-alpha" → sha256("team-alpha")[:4] = a1c3
│   ├── ingest.yaml
│   └── transform.yaml
└── e7b2/             # "team-beta" → sha256("team-beta")[:4] = e7b2
    └── analytics.yaml

# DAG runs and operational data
~/.local/share/dagu/data/
├── 6a5f/             # "default"
│   ├── dag-runs/
│   ├── proc/
│   └── queue/
├── a1c3/             # "team-alpha"
│   ├── dag-runs/
│   ├── proc/
│   └── queue/
└── e7b2/             # "team-beta"
    └── ...
```

#### Namespace Store

Namespace data follows the existing persistence pattern used by all other stores in the codebase:

**Port interface** (`internal/core/exec/namespace.go`):

```go
type NamespaceStore interface {
    Create(ctx context.Context, name string) (Namespace, error)
    Delete(ctx context.Context, name string) error
    Get(ctx context.Context, name string) (Namespace, error)
    List(ctx context.Context) ([]Namespace, error)
    Resolve(ctx context.Context, name string) (string, error) // name → short ID
}

type Namespace struct {
    Name        string           // Human-readable name (e.g., "team-alpha")
    ShortID     string           // 4-char hex directory name (e.g., "a1c3")
    CreatedAt   time.Time
    Description string           // Human-readable description
    BaseConfig  *core.DAG        // Namespace-level base config (same structure as global base.yaml)
    Defaults    NamespaceDefaults
    GitSync     NamespaceGitSync // Git sync settings (see Git Sync section)
}

type NamespaceDefaults struct {
    Queue      string
    WorkingDir string
}
```

**File-based implementation** (`internal/persis/filenamespace/store.go`):

```go
func New(baseDir string) exec.NamespaceStore {
    return &Store{baseDir: baseDir}
}
```

The store persists namespace data as JSON files inside its base directory — one file per namespace (`{shortID}.json`):

```
~/.local/share/dagu/data/namespaces/
├── 6a5f.json          # "default"
├── a1c3.json          # "team-alpha"
└── e7b2.json          # "team-beta"
```

Example (`a1c3.json`):

```json
{
  "name": "team-alpha",
  "shortID": "a1c3",
  "createdAt": "2025-03-01T10:30:00Z",
  "description": "Team Alpha workflows",
  "baseConfig": {
    "env": [
      {"name": "TEAM", "value": "alpha"}
    ],
    "logDir": "/logs/team-alpha",
    "histRetentionDays": 30
  },
  "defaults": {
    "queue": "team-alpha-queue",
    "workingDir": "/data/team-alpha"
  }
}
```

**Wired into `Context`** (`internal/cmd/context.go`) following the same pattern as `ProcStore`, `QueueStore`, etc.:

```go
type Context struct {
    // ... existing fields ...
    NamespaceStore exec.NamespaceStore
}

// In NewContext:
ns := filenamespace.New(cfg.Paths.NamespacesDir)
```

**Config path** (`internal/cmn/config/loader.go`) — a new `NamespacesDir` is added to the `Paths` struct and derived from `DataDir` in `finalizePaths()`, following the same derivation pattern as `ProcDir`, `QueueDir`, etc.

The `default` namespace uses a well-known fixed short ID assigned during initial setup or migration. Users never see the short ID — the API, CLI, and UI always use the human-readable namespace name. The short ID is an internal implementation detail for filesystem storage only.

In the unlikely event of a hash collision (two namespace names producing the same 4-char prefix), `Create` returns an error and the user must choose a different name.

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

#### Namespace Selector

A dropdown in the navigation bar lets the user switch the active namespace. The selector lists all namespaces the user has access to (retrieved from `GET /api/v1/namespaces`). Changing the selection reloads the current view (DAGs, runs, logs) scoped to the chosen namespace. A special "All Namespaces" option shows an aggregated view across every namespace the user can read.

#### Namespace Management Page

A dedicated settings page (`/namespaces`) where administrators can:

- **List** all namespaces with their description and status.
- **Create** a new namespace — prompts for name, description, and optional defaults.
- **Edit** an existing namespace — update description, default queue, default working directory, and git-sync settings (remote URL, branch, SSH key reference, sync interval).
- **Delete** a namespace — requires confirmation; only allowed when the namespace contains no DAGs.

All mutations go through the `NamespaceStore` API; there is no `_namespace.yaml` file to edit by hand.

#### Base Config Editor

Each namespace can carry a base configuration (shared `env`, `logDir`, `handlerOn`, etc.) that every DAG in the namespace inherits. The namespace settings page embeds a YAML editor — identical in style to the existing DAG spec editor — for viewing and editing this base config. Changes are validated server-side before being persisted to the `NamespaceStore`.

#### Namespace-Scoped Views

When a namespace is selected, every major view is filtered to that namespace:

- **DAGs list** — shows only DAGs belonging to the active namespace.
- **Run history** — displays runs for DAGs in the active namespace.
- **Log viewer** — scoped to log entries from the active namespace.

Breadcrumbs and page titles include the namespace name for clarity.

#### Admin Panel

The admin panel adds a **Role Assignment** tab per namespace where administrators can:

- Assign users or groups to roles (`viewer`, `editor`, `admin`) within a namespace.
- View the effective permission matrix for the selected namespace.

### Scheduler

The scheduler discovers and schedules DAGs across all namespaces. It scans namespace subdirectories under the DAGs root and watches each for file changes. When a DAG's schedule fires, the scheduler submits a task to the coordinator with the `namespace` field set to the namespace the DAG belongs to. This ensures the namespace context is available before the task reaches any worker.

### Worker

Workers poll the coordinator for tasks. Every task **must** carry a `namespace` field so the worker knows which namespace the DAG belongs to. Without this, the worker cannot resolve the correct DAG directory, data directory, base config, queue, or log path.

#### Proto Change

Add `namespace` to the `Task` message:

```proto
message Task {
  // ... existing fields ...
  string namespace = 16; // Namespace the DAG belongs to (required)
}
```

The coordinator **must** populate `namespace` before dispatching a task. A worker receiving a task with an empty `namespace` treats it as an error and rejects the task.

#### Subprocess Execution (Standard Mode)

`SubCmdBuilder.TaskStart()` adds a `--namespace` flag when building the subprocess command so the child `dagu start` process runs in the correct namespace context:

```go
// In TaskStart():
if task.Namespace != "" {
    args = append(args, fmt.Sprintf("--namespace=%s", task.Namespace))
}
```

The `start` command uses the namespace to:

1. **Resolve the DAG directory** — `{dagsRoot}/{shortID}/` instead of the flat root.
2. **Resolve the data directory** — `{dataRoot}/{shortID}/dag-runs/`, `proc/`, `queue/`.
3. **Load the namespace base config** — fetched from `NamespaceStore.Get()` and merged before DAG-level config, so namespace-level `env`, `logDir`, `handlerOn`, etc. are inherited.
4. **Select the correct queue** — defaults to the namespace's `Defaults.Queue` if the DAG does not specify one.

#### In-Process Execution (Shared-Nothing Mode)

The remote task handler receives the `namespace` from the `Task` proto and threads it through in the same way. It resolves paths and base config locally, then streams status and logs back to the coordinator tagged with the namespace.

#### Sub-DAG Propagation

When a running DAG spawns a sub-DAG via a `run` step, the agent propagates the parent's namespace to the child. The child task inherits the same `namespace` value — cross-namespace sub-DAG calls remain unsupported.

#### Local Socket Name

Each running DAG agent opens a Unix domain socket in `/tmp` for IPC (status queries and stop signals). The current `SockAddr()` function builds the socket name from the DAG's `Location` (file path) or `Name` + `dagRunID`:

```
/tmp/@dagu_<safeName>_<md5-hash>.sock
```

With namespaces, two DAGs with the same name in different namespaces (e.g., `team-alpha/ingest` and `team-beta/ingest`) could collide because the `safeName` portion is truncated to 32 characters and the 6-char MD5 hash only covers the name and run ID — not the namespace.

**Fix:** Include the namespace in the hash input so socket names are unique per namespace:

```go
func SockAddr(namespace, name, dagRunID string) string {
    hash := fmt.Sprintf("%x", md5.Sum([]byte(namespace+name+dagRunID)))[:hashLength]
    // ... rest unchanged ...
}
```

The namespace does **not** need to appear in the readable portion of the socket name (the `safeName` segment). Adding it to the MD5 input is sufficient to prevent collisions while keeping the socket path short. All call sites (`DAG.SockAddr()`, `DAG.SockAddrForSubDAGRun()`, and the manager) must pass the namespace through.

#### Worker Labels and Namespace Affinity

Workers can optionally restrict themselves to specific namespaces using labels:

```bash
dagu worker start --labels namespace=team-alpha
```

The coordinator matches `worker_selector` labels when dispatching, so teams can dedicate workers to their namespace. This is optional — workers without a namespace label accept tasks from any namespace.

### Namespace Context Propagation

Namespace must be available at every layer of the execution stack. The following changes thread it from the coordinator all the way down to stores and sockets.

#### 1. `core.DAG` — carry namespace on the DAG struct

```go
type DAG struct {
    Namespace string   // Namespace this DAG belongs to
    // ... existing fields ...
}
```

The `spec.Load()` pipeline does not set `Namespace` — the caller does, because namespace is determined by context (CLI flag, API path, coordinator task), not by the YAML file itself. This keeps DAG definitions portable across namespaces.

#### 2. `exec.Context` — runtime execution context

Add `Namespace` so every running step can access it:

```go
type Context struct {
    Namespace string          // Active namespace
    DAGRunID  string
    DAG       *core.DAG
    // ... existing fields ...
}
```

`exec.NewContext()` receives the namespace from the agent. Steps use it when spawning sub-DAGs (to propagate the parent's namespace) and when resolving log paths.

#### 3. `agent.Options` — pass namespace into the agent

```go
type Options struct {
    Namespace string          // Namespace for this execution
    // ... existing fields (WorkerID, RootDAGRun, etc.) ...
}
```

The agent stores `opts.Namespace` and:

- Passes it to `exec.NewContext()` during `Run()`.
- Passes it to `dag.SockAddr()` when setting up the socket server.
- Includes it in the sub-command when dispatching sub-DAGs to the coordinator.

#### 4. `start` command — accept `--namespace` flag

```go
// In CmdStart():
cmd.Flags().StringVar(&namespaceFlag, "namespace", "default", "namespace for this DAG run")
```

`runStart()` uses the flag to:

1. **Resolve store paths** — constructs namespace-scoped base directories (`{dataRoot}/{shortID}/dag-runs/`, `{dataRoot}/{shortID}/proc/`, etc.) before initializing `DAGRunStore`, `ProcStore`, and `QueueStore`.
2. **Load namespace base config** — calls `NamespaceStore.Get()` and passes `ns.BaseConfig` to `spec.Load()` via `WithBaseConfig()`, replacing the global `base.yaml`.
3. **Set `DAG.Namespace`** — assigns the namespace on the loaded DAG struct.
4. **Pass to agent** — sets `agent.Options.Namespace`.

#### 5. Store scoping

Stores are already parameterized by a `baseDir`. Namespace scoping works by constructing namespace-specific base directories before creating stores — no changes to the store implementations themselves:

```go
// In runStart(), after resolving namespace short ID:
shortID, _ := namespaceStore.Resolve(ctx, namespaceFlag)

dagRunsDir := filepath.Join(cfg.Paths.DataDir, shortID, "dag-runs")
procDir    := filepath.Join(cfg.Paths.DataDir, shortID, "proc")
queueDir   := filepath.Join(cfg.Paths.DataDir, shortID, "queue")

drs := filedagrun.New(dagRunsDir)
ps  := fileproc.New(procDir)
qs  := filequeue.New(queueDir)
```

This keeps the store code unchanged — namespace isolation is achieved purely through directory layout.

#### 6. Full propagation chain

```
Coordinator (sets Task.Namespace)
  → Worker (reads Task.Namespace, passes --namespace flag)
    → start command (reads --namespace, resolves short ID)
      → NamespaceStore.Resolve() → short ID for directory paths
      → NamespaceStore.Get()     → base config for spec.Load()
      → Stores created with namespace-scoped base dirs
      → DAG.Namespace set on loaded DAG struct
      → agent.New(..., Options{Namespace: ns})
        → exec.Context{Namespace: ns}
          → Steps inherit namespace
          → Sub-DAG dispatch carries namespace
        → dag.SockAddr(namespace, ...) → unique socket
```

For CLI-initiated runs (`dagu start team-alpha/ingest`), the namespace is extracted from the `namespace/dag-name` format or the `--namespace` flag — no coordinator involved, but the same chain applies from the `start` command onward.

### Sub-DAG References

Cross-namespace sub-DAG calls are **not supported**. All `run` references resolve within the same namespace as the calling DAG. This keeps execution boundaries simple and avoids complex cross-namespace dependency graphs.

```yaml
steps:
  - name: call-local
    run: common-cleanup             # resolves within the same namespace
```

If teams need to share workflows, they should duplicate the DAG into each namespace or extract the shared logic into a script/binary invoked via a command step.

### Namespace Configuration

Namespace configuration is stored in the `NamespaceStore` as part of the `Namespace` struct. It is managed via the API and CLI — there is no separate configuration file to edit by hand.

```bash
dagu namespace create team-alpha \
  --description "Team Alpha workflows" \
  --default-queue team-alpha-queue \
  --default-working-dir /data/team-alpha

# Set base config from a YAML file (parsed and stored in the namespace JSON)
dagu namespace set-base-config team-alpha --from-file base.yaml
```

### Git Sync

Git sync becomes namespace-scoped. Each namespace can configure its own git repository, branch, and sync settings via the `GitSync` field in the `NamespaceStore`:

```json
{
  "name": "team-alpha",
  "shortID": "a1c3",
  "gitSync": {
    "enabled": true,
    "repository": "github.com/org/team-alpha-dags",
    "branch": "main",
    "auth": {
      "type": "token",
      "tokenEnv": "TEAM_ALPHA_GIT_TOKEN"
    },
    "autoSync": {
      "enabled": true,
      "interval": 300
    }
  }
}
```

Alternatively, a single repository can serve multiple namespaces by mapping subdirectories to namespaces using the `path` field:

```json
// team-alpha namespace
{ "gitSync": { "repository": "github.com/org/all-dags", "path": "team-alpha" } }

// team-beta namespace
{ "gitSync": { "repository": "github.com/org/all-dags", "path": "team-beta" } }
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
- **No cross-namespace operations** — the agent cannot access DAGs or files outside the active namespace; users must start a new conversation to work in a different namespace
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

Each conversation is locked to the namespace in which it was started. The agent cannot switch namespaces mid-conversation. Users must start a new conversation to work in a different namespace. Conversation history is visible across namespaces the user has access to, but each conversation is tagged with its originating namespace.

Audit logging (RFC 002) includes the namespace in every agent action record for traceability.

## Migration

Existing installations upgrade seamlessly:

1. Create the namespace registry file (`namespaces.json`) in the config root with the `default` namespace mapped to its well-known fixed short ID
2. All current DAGs are moved into the `default` namespace's short-ID subdirectory (e.g., `dags/6a5f/`)
3. All current run data is moved into the `default` namespace's short-ID subdirectory (dag-runs, proc, queue, suspend flags)
4. Git sync state is moved into the `default` namespace's short-ID subdirectory
5. Agent conversation history is tagged with the `default` namespace
6. Existing users receive their current global role unchanged
7. No configuration changes are required

An automatic migration runs on first startup after upgrade.

## Design Decisions

1. **Namespace deletion requires empty namespace** — a namespace must have all DAGs and run history removed before it can be deleted. No cascading deletion.
2. **No cross-namespace triggers** — DAGs cannot trigger or reference DAGs in other namespaces. All `run` references resolve within the same namespace. This avoids complex cross-namespace dependency graphs.
3. **Secrets are per-namespace by default** — secrets are configured in the namespace's `BaseConfig` stored in the `NamespaceStore`. No additional mechanism needed.
4. **Git sync pushes independently per namespace** — when multiple namespaces share the same git repository, each namespace pushes independently. No coordination between namespaces.
5. **Agent conversations are locked to a single namespace** — the agent cannot switch namespaces mid-conversation. Users start a new conversation to work in a different namespace.

## Out of Scope

1. **Resource quotas** — namespace-level limits on concurrent runs, DAG count, or storage are not included in the initial implementation. May be revisited in a future RFC.
