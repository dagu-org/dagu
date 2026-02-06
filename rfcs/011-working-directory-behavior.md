---
id: "011"
title: "Working Directory Behavior"
status: draft
---

# RFC 011: Working Directory Behavior

## Summary

Document the current working directory resolution behavior across the DAG execution lifecycle. This RFC serves as a reference for understanding how working directories are determined, inherited, and applied at each layer: DAG-level, step-level, sub-DAG, SSH, Docker containers, and script execution.

## Current Behavior

### Data Model

Working directory is represented at three levels in the data model:

| Layer | Go Field | YAML Field | JSON Field | Description |
|-------|----------|------------|------------|-------------|
| DAG | `DAG.WorkingDir` | `workingDir` | `workingDir` | Working directory for the entire DAG |
| Step | `Step.Dir` | `workingDir` | `dir` | Per-step working directory override |
| Container | `Container.WorkingDir` | `workingDir` | `WorkingDir` | Working directory inside a Docker container (no explicit `json:` tag — Go default) |

Note the naming inconsistency: the step's Go field is `Dir` (JSON: `dir`) while the YAML spec field is `WorkingDir` (YAML: `workingDir`). The spec-to-core mapping converts `spec.step.WorkingDir` to `core.Step.Dir`.

### Build-Time Resolution (DAG Loading)

When a DAG YAML file is loaded, the `buildWorkingDir` function (`internal/core/spec/dag.go:954-963`) resolves the DAG-level working directory with this priority:

1. **Explicit `workingDir` in YAML** — if the DAG file specifies a `workingDir` field
2. **`DefaultWorkingDir` from load options** — passed via `--default-working-dir` CLI flag (used for sub-DAG inheritance)
3. **Empty string** — allows inheritance from base config during the merge step

After the merge with base config, `loadDAG` applies a fallback if `WorkingDir` is still empty:

4. **Directory of the DAG file** — `filepath.Dir(dagFile)`
5. **Fallback** — current working directory (`os.Getwd()`), then user home directory

The `resolveWorkingDirPath` function (`internal/core/spec/dag.go:967-978`) determines what happens to the raw value:

- **Absolute paths** (`/foo/bar`): stored as-is
- **Home-relative paths** (`~/foo`): stored as-is (expanded at runtime)
- **Variable paths** (`$MY_DIR`, `${MY_DIR}`): stored as-is (expanded at runtime)
- **Relative paths** (`./scripts`, `../other`): resolved to absolute against the DAG file's directory at build time via `filepath.Join(filepath.Dir(dagFile), wd)`

Step-level working directory is NOT resolved at build time. The `buildStepWorkingDir` function (`internal/core/spec/step.go:403-405`) simply trims whitespace and stores the raw value.

### Runtime Resolution — Agent Initialization

When the Agent starts a DAG run, it evaluates the working directory before executing any steps (`internal/runtime/agent/agent.go:436-492`):

1. **`evaluateWorkingDir`** (`agent.go:1268-1294`):
   - If `dag.WorkingDir` is empty, returns immediately (no evaluation)
   - Expands environment variables using `EnvScope.Expand()` (or `os.ExpandEnv()` as fallback)
   - Resolves `~` prefix to user home directory
   - Stores result in `evaluatedWorkingDir` (does not mutate the original DAG)

2. **Directory creation**: `os.MkdirAll(evaluatedWorkingDir, 0o755)` — creates the directory if it doesn't exist

3. **Process-wide chdir**: `os.Chdir(evaluatedWorkingDir)` — changes the entire process's working directory

This `os.Chdir` call is significant: it changes the working directory for the entire Dagu process, not just for child commands. All subsequent operations in this process (including `os.Getwd()` calls in fallback paths) are affected.

### Runtime Resolution — Per-Step

When each step executes, `resolveWorkingDir` (`internal/runtime/env.go:101-133`) determines the step's working directory:

**Priority 1 — Step's `Dir` field** (if non-empty):
- Environment variables in `step.Dir` are expanded using DAG env vars first, then OS env vars (`expandStepDir`, `env.go:135-147`)
- The expanded path is then resolved (`resolveExpandedDir`, `env.go:149-173`):
  - Absolute paths and `~` paths: resolved via `fileutil.ResolvePath()`
  - Relative paths: joined with `dag.WorkingDir` via `filepath.Clean(filepath.Join(dag.WorkingDir, expandedDir))` (note: `filepath.Clean` is redundant here since `filepath.Join` already cleans)

**Priority 2 — DAG's `WorkingDir`** (if step Dir is empty and DAG WorkingDir is non-empty):
- Expanded via `EnvScope.Expand()` or `os.ExpandEnv()`
- `~` prefix resolved after variable expansion

**Priority 3 — Fallback** (`fallbackWorkingDir`, `env.go:175-192`):
- Logs a warning
- Returns `os.Getwd()` (which, due to the agent's earlier `os.Chdir`, is the DAG's evaluated working directory)
- If that fails, returns `os.UserHomeDir()`

The resolved working directory is:
- Set as `Env.WorkingDir`
- Injected as the `PWD` environment variable for the step

### Command Execution

The command executor (`internal/runtime/builtin/command/command.go:228`) sets `cmd.Dir` on the `exec.Cmd` object:

```go
cmd.Dir = cfg.Dir
```

Before starting the command (`command.go:72-78`), if `cmd.Dir` is non-empty, the directory is created:

```go
if cmd.Dir != "" {
    if err := os.MkdirAll(cmd.Dir, 0750); err != nil { ... }
}
```

This uses the subprocess `Dir` field (not `os.Chdir`), so each step's working directory is isolated and does not affect the parent process or other steps.

### Script Execution

When a step has a `script` field, a temporary script file is created via `setupScript` (`internal/runtime/builtin/command/command_script.go:19-60`):

```go
file, err := os.CreateTemp(workDir, pattern)
```

The temporary script file is created **in the step's working directory** (the `workDir` parameter). If `workDir` is empty, `os.CreateTemp` falls back to the system temp directory. The temporary script file is removed after command execution completes (`command.go:52-54`).

### Sub-DAG Execution

When a step calls a sub-DAG (`internal/runtime/builtin/dag/dag.go:56-65`, `internal/runtime/executor/dag_runner.go:114-167`):

1. The parent step's resolved `WorkingDir` is captured from the runtime environment
2. The sub-DAG is launched as a separate `dagu start` process with:
   - `cmd.Dir = workDir` (subprocess working directory)
   - `--default-working-dir=<workDir>` flag (tells the child DAG loader to use this as default)
3. In the child DAG's loader, `WithDefaultWorkingDir` sets `LoadOptions.defaultWorkingDir`
4. The child DAG's `buildWorkingDir` uses this default only if the child has no explicit `workingDir` in its YAML

**Behavior:**
- Sub-DAG **with** explicit `workingDir`: uses its own value (overrides inherited)
- Sub-DAG **without** `workingDir`: inherits parent's working directory

### SSH Execution

SSH execution (`internal/runtime/builtin/ssh/ssh.go:172-203`) has special behavior:

- **Only uses step-level `Dir`** — DAG-level `WorkingDir` is intentionally ignored
- Rationale: DAG-level `workingDir` is for LOCAL execution and may not exist on the remote host
- If `step.Dir` is set, the SSH script prepends `cd <quoted-dir> || return 1` (the directory is shell-quoted via `cmdutil.ShellQuote()` for paths with spaces or special characters)
- If `step.Dir` is empty, the command runs in the SSH user's home directory

### Docker Container Execution

Docker containers have two distinct working directories:

1. **Host-side working directory**: the step's resolved `WorkingDir` — passed to `docker.LoadConfig(env.WorkingDir, ...)` where it is used solely for resolving relative volume mount paths (`parseVolumes`). It does NOT influence the container's working directory.
2. **Container-side working directory**: `Container.WorkingDir` — set as `container.Config.WorkingDir` in the Docker API, controls the `WORKDIR` inside the container

The container-side `workingDir` can be specified at either DAG level or step level in the container configuration.

### Dotenv File Resolution

Dotenv files (`internal/core/dag.go:356-373`) are resolved relative to the DAG's working directory:

```go
relativeTos := []string{d.WorkingDir}
if fileDir := filepath.Dir(d.Location); d.Location != "" && fileDir != d.WorkingDir {
    relativeTos = append(relativeTos, fileDir)
}
```

Resolution order:
1. Relative to `dag.WorkingDir`
2. Relative to the DAG file's directory (if different from WorkingDir)

Note: A default `.env` file is always prepended to the candidate list (`dag.go:368`), meaning `.env` in the working directory is always searched for regardless of whether it's explicitly listed in the YAML `dotenv` field.

### Base Config Inheritance (`base.yaml`)

When `workingDir` is set in a base config file (`base.yaml`), it serves as a global default for all DAGs:

- **Child DAGs without explicit `workingDir`** inherit the base value
- **Child DAGs with explicit `workingDir`** override the base value

This works because `buildWorkingDir` returns `""` when no explicit `workingDir` is set (no YAML field, no `DefaultWorkingDir` option). When `mergo.Merge` runs with `WithOverride`, the child's empty `WorkingDir` does not overwrite the base's explicit value. The fallback chain (`filepath.Dir(dagFile)` → `os.Getwd()` → home) is applied post-merge in `loadDAG` only if `WorkingDir` is still empty after merging.

Previously, `buildWorkingDir` always returned a non-empty value via its fallback chain, which caused the child DAG's fallback value to overwrite the base's explicit `workingDir` during the merge step. This was fixed by moving the fallback logic to post-merge.

### Schema File Resolution

JSON schema files referenced in DAG parameters (`internal/core/spec/schema.go`) are resolved in order:
1. As-is (relative to CWD / env expansion)
2. Relative to `dag.WorkingDir`
3. Relative to the DAG file's directory

### Environment Variables

- `DAGU_WORK_DIR` — removed (was dead/orphaned: bound in config loader but never stored or read)
- `PWD` — set per-step to the resolved working directory (`env.go:80`)
- `DAGU_HOME` — determines the base directory for Dagu's data/config paths, but does not directly affect DAG working directory

## Resolution Summary

### Build Time (DAG Loading)

```
DAG workingDir (per-document, buildWorkingDir):
  explicit YAML value?     → store (resolve relative against DAG file dir)
  DefaultWorkingDir option? → use as-is
  otherwise                → return "" (allow base config inheritance)

Post-merge fallback (loadDAG, after merge + InitializeDefaults):
  WorkingDir still empty?
    DAG file exists?       → filepath.Dir(dagFile)
    fallback               → os.Getwd() or os.UserHomeDir()

Step workingDir:
  → stored as raw trimmed value (no resolution)
```

### Runtime (Agent Init)

```
Agent evaluateWorkingDir:
  dag.WorkingDir empty? → evaluateWorkingDir returns early (evaluatedWorkingDir stays "")
                          MkdirAll/Chdir still execute but are effectively no-ops
                          (in practice, buildWorkingDir fallbacks guarantee non-empty)
  otherwise            → expand env vars → resolve ~ → MkdirAll → os.Chdir
```

### Runtime (Per-Step)

```
Step working directory:
  step.Dir non-empty?
    → expand env vars (DAG env > OS env)
    → absolute or ~?  → resolve to absolute
    → relative?       → join with dag.WorkingDir
  dag.WorkingDir non-empty?
    → expand env vars → resolve ~
  fallback → os.Getwd() → os.UserHomeDir()

Result → cmd.Dir (subprocess isolation)
       → PWD env var
```

### Execution Type Specifics

| Execution Type | Working Directory Source | Mechanism |
|---------------|------------------------|-----------|
| Local command | Step Dir or DAG WorkingDir | `exec.Cmd.Dir` |
| Script | Step Dir or DAG WorkingDir | Temp file in workDir + `exec.Cmd.Dir` |
| Sub-DAG | Parent's WorkingDir (inherited) | `--default-working-dir` flag + `cmd.Dir` |
| SSH | Step Dir only | `cd <dir>` in script |
| Docker | Container.WorkingDir | Docker API `ContainerConfig.WorkingDir` |

## Design Decisions

1. **Process-wide chdir at agent init** — the agent calls `os.Chdir` once at initialization, changing the entire process's working directory. This is the only `os.Chdir` call in the runtime. Individual steps use `exec.Cmd.Dir` for subprocess isolation.

2. **Build-time vs runtime resolution** — relative paths in DAG-level `workingDir` are resolved to absolute at build time (against the DAG file directory). Variable references (`$VAR`, `~`) are deferred to runtime. Step-level `workingDir` is stored raw and fully resolved at runtime.

3. **Sub-DAG inheritance** — sub-DAGs without explicit `workingDir` inherit the parent's working directory via a CLI flag. Sub-DAGs with explicit `workingDir` override the inherited value.

4. **SSH ignores DAG-level workingDir** — by design, SSH execution only respects step-level `Dir` since the DAG's working directory may not exist on the remote host.

5. **Auto-creation** — working directories are auto-created (`os.MkdirAll`) both at the agent level (with `0o755` permissions) and at the command level (with `0o750` permissions). The agent uses `0o755` for the DAG-level directory (potentially shared/accessed by other processes) while the command executor uses `0o750` for per-step directories (more restrictive, no world-read).

6. **Naming inconsistency** — the step's Go struct uses `Dir` while the YAML spec uses `workingDir`. This is a historical artifact where the spec layer was refactored but the core struct retained the shorter name.
